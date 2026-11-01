package native

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"gocompare/internal/config"
	"gocompare/internal/i18n"
	"gocompare/internal/remote"
)

// listTimeout 限制一次连接加列目录；浏览时每进一层目录都会重新连接。
const listTimeout = 30 * time.Second

// remoteProtocol 是连接对话框里可浏览的协议；S3、Dropbox 等仍可在路径框直接输入地址。
type remoteProtocol struct {
	scheme string
	key    string // i18n 键
}

var remoteProtocols = []remoteProtocol{
	{"sftp", "remote.proto.sftp"},
	{"smb", "remote.proto.smb"},
	{"ftp", "remote.proto.ftp"},
	{"ftps", "remote.proto.ftps"},
	{"webdav", "remote.proto.webdav"},
	{"webdav+https", "remote.proto.webdavs"},
}

// remoteDialog 是"连接远程文件"对话框：第一步填写连接信息，第二步浏览并选择文件。
type remoteDialog struct {
	side   int
	opened bool
	browse bool
	// pickDir 为真时选择的是目录（目录比较），否则选择文件。
	pickDir bool

	protocol   string // 当前协议的显示名
	host       string
	port       string
	user       string
	domain     string
	share      string
	password   string
	identity   string
	passphrase string

	sshHosts  []remote.SSHHost
	sshChoice string
	recents   []config.RemoteBookmark

	ep  remote.Endpoint // 连接成功后固定下来的端点
	dir string
	// dirFor 记录 dir 属于哪个连接（Endpoint.Key）。表单改成别的主机或用户后，
	// 旧目录在新服务器上没有意义，连接时改从起始目录开始。
	dirFor   string
	entries  []remote.Entry
	list     ui.ListState
	selected string // 选中文件的完整路径
	manual   string // 手动输入的路径，浏览不可用时使用
	noList   bool

	loading bool
	err     string
	gen     uint64
	cancel  context.CancelFunc
}

func protocolLabels() []string {
	labels := make([]string, len(remoteProtocols))
	for i, p := range remoteProtocols {
		labels[i] = i18n.T(p.key)
	}
	return labels
}

func (d *remoteDialog) scheme() string {
	for _, p := range remoteProtocols {
		if i18n.T(p.key) == d.protocol {
			return p.scheme
		}
	}
	return "sftp"
}

func (d *remoteDialog) setScheme(scheme string) {
	for _, p := range remoteProtocols {
		if p.scheme == scheme {
			d.protocol = i18n.T(p.key)
		}
	}
}

// openRemoteDialog 打开连接对话框；路径框里已有远程地址时预先填好，便于换一个文件。
func (a *workspaceState) openRemoteDialog(side int) {
	if a.busy {
		return
	}
	d := &remoteDialog{side: side, opened: true, pickDir: a.kind == kindDirectory}
	d.setScheme("sftp")
	d.sshHosts, _ = remote.LoadSSHConfig("")
	if a.remotesFile != "" {
		d.recents = config.LoadRemotes(a.remotesFile)
	}
	if raw := a.paths[side]; remote.IsURL(raw) {
		d.fillFromURL(raw, d.pickDir)
	} else if len(d.recents) > 0 {
		d.fillFromBookmark(d.recents[0])
	}
	a.remoteDialog = d
}

func (d *remoteDialog) fillFromBookmark(b config.RemoteBookmark) {
	d.setScheme(b.Scheme)
	d.host, d.port, d.user, d.domain, d.share, d.identity = b.Host, b.Port, b.User, b.Domain, b.Share, b.IdentityFile
	d.dir = b.Dir
	d.dirFor = d.endpoint().Key()
	d.password, d.passphrase = "", ""
	d.err = ""
}

// fillFromURL 从现有地址还原连接信息：文件地址取所在目录，目录地址取目录本身。
func (d *remoteDialog) fillFromURL(raw string, isDir bool) {
	parse := remote.ParseURL
	if isDir {
		parse = remote.ParseDirURL
	}
	store, p, err := parse(raw)
	if err != nil {
		return
	}
	ep, ok := endpointOf(raw, store)
	if !ok {
		return
	}
	d.setScheme(ep.Scheme)
	d.host, d.port, d.user, d.domain, d.share = ep.Host, ep.Port, ep.User, ep.Domain, ep.Share
	d.dir = p
	if !isDir {
		d.dir = parentDir(ep.Scheme, p)
	}
	d.dirFor = d.endpoint().Key()
}

// endpointOf 只支持对话框可浏览的协议。
func endpointOf(raw string, store remote.Store) (remote.Endpoint, bool) {
	scheme, _, _ := strings.Cut(raw, "://")
	ep := remote.Endpoint{Scheme: strings.ToLower(scheme)}
	var addr string
	switch s := store.(type) {
	case *remote.SFTP:
		addr, ep.User = s.Addr, s.User
	case *remote.SMB:
		addr, ep.User, ep.Domain, ep.Share = s.Addr, s.User, s.Domain, s.Share
	case *remote.FTP:
		addr, ep.User = s.Addr, s.User
	case *remote.WebDAV:
		addr = strings.TrimPrefix(strings.TrimPrefix(s.Base, "https://"), "http://")
		ep.User = s.User
	default:
		return ep, false
	}
	host, port, err := splitHostPort(addr)
	if err != nil {
		return ep, false
	}
	ep.Host, ep.Port = host, port
	if ep.Port == remote.DefaultPort(ep.Scheme) {
		ep.Port = ""
	}
	return ep, true
}

func splitHostPort(addr string) (string, string, error) {
	if !strings.Contains(addr, ":") || strings.HasSuffix(addr, "]") {
		return strings.Trim(addr, "[]"), "", nil
	}
	i := strings.LastIndex(addr, ":")
	return strings.Trim(addr[:i], "[]"), addr[i+1:], nil
}

// endpoint 由表单生成端点；SMB 共享名可留空，连接后先列出共享。
func (d *remoteDialog) endpoint() remote.Endpoint {
	ep := remote.Endpoint{Scheme: d.scheme(), Host: strings.TrimSpace(d.host), Port: strings.TrimSpace(d.port), User: strings.TrimSpace(d.user)}
	if ep.Scheme == "smb" {
		ep.Domain = strings.TrimSpace(d.domain)
		ep.Share = strings.Trim(strings.TrimSpace(d.share), `/\`)
	}
	return ep
}

// connect 校验表单后列出起始目录；成功即进入浏览步骤。
func (a *workspaceState) remoteConnect() {
	d := a.remoteDialog
	if d == nil || d.loading {
		return
	}
	ep := d.endpoint()
	if ep.Host == "" {
		d.err = i18n.T("remote.errHost")
		return
	}
	d.ep = ep
	if d.password != "" {
		if a.remoteSecrets == nil {
			a.remoteSecrets = map[string]string{}
		}
		a.remoteSecrets[ep.Key()] = d.password
	}
	if a.remoteIdentities == nil {
		a.remoteIdentities = map[string]string{}
	}
	if a.remotePassphrases == nil {
		a.remotePassphrases = map[string]string{}
	}
	if id := strings.TrimSpace(d.identity); id != "" && ep.Scheme == "sftp" {
		a.remoteIdentities[ep.Key()] = id
	} else {
		delete(a.remoteIdentities, ep.Key())
	}
	if d.passphrase != "" && ep.Scheme == "sftp" {
		a.remotePassphrases[ep.Key()] = d.passphrase
	} else {
		delete(a.remotePassphrases, ep.Key())
	}
	start := d.dir
	if d.dirFor != ep.Key() {
		start = ""
	}
	a.remoteList(start)
}

// remoteList 在后台列出 dir，结果按代次丢弃过期响应，避免快速点击时目录错乱。
func (a *workspaceState) remoteList(dir string) {
	d := a.remoteDialog
	if d == nil {
		return
	}
	raw := d.ep.URL("")
	store, err := remote.ParseBrowseURL(raw)
	if err != nil {
		d.err = err.Error()
		return
	}
	key := d.ep.Key()
	auth := remoteAuth{
		passwords:   map[string]string{key: a.remoteSecrets[key]},
		identities:  map[string]string{key: a.remoteIdentities[key]},
		passphrases: map[string]string{key: a.remotePassphrases[key]},
	}
	auth.apply(raw, store)
	lister, ok := store.(remote.Lister)
	if !ok {
		d.browse, d.noList = true, true
		return
	}
	if d.cancel != nil {
		d.cancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), listTimeout)
	d.cancel = cancel
	d.gen++
	gen := d.gen
	d.loading, d.err = true, ""
	run := func() {
		defer cancel()
		resolved, entries, err := lister.List(ctx, dir)
		a.publish(func() {
			if a.remoteDialog != d || d.gen != gen {
				return
			}
			d.loading = false
			if err != nil {
				a.remoteListFailed(d, dir, err)
				return
			}
			d.browse, d.noList = true, false
			d.dir, d.entries, d.selected = resolved, entries, ""
			d.list = ui.ListState{}
		})
	}
	if a.win == nil {
		run()
	} else {
		go run()
	}
}

func (a *workspaceState) remoteListFailed(d *remoteDialog, dir string, err error) {
	// 首次连接时记住的目录可能已被删除或改名：退回起始目录，而不是让用户卡在报错上。
	if !d.browse && dir != "" && errors.Is(err, fs.ErrNotExist) {
		a.remoteList("")
		return
	}
	var hostKey *remote.HostKeyError
	switch {
	case errors.As(err, &hostKey):
		a.hostKeyPrompt = &hostKeyPrompt{err: hostKey, retry: func() { a.remoteList(dir) }, opened: true}
	case errors.Is(err, remote.ErrListUnsupported):
		d.browse, d.noList = true, true
		d.manual = dir
	case errors.Is(err, remote.ErrAuth) && d.ep.Scheme == "sftp" && d.password == "":
		d.browse = false
		d.err = i18n.T("remote.errSSHAuth")
	default:
		// 认证、私钥、网络问题都回到表单，方便直接修改后重试。
		d.browse = false
		d.err = friendlyRemoteError(err)
	}
}

func (a *workspaceState) closeRemoteDialog() {
	if d := a.remoteDialog; d != nil && d.cancel != nil {
		d.cancel()
	}
	a.remoteDialog = nil
}

// remoteChoose 把选中的文件填入路径框，记住这次连接，然后直接加载比较（另一侧已有路径时）。
func (a *workspaceState) remoteChoose() {
	d := a.remoteDialog
	if d == nil {
		return
	}
	p := d.selected
	if d.pickDir {
		p = d.dir
	}
	if d.noList {
		p = strings.TrimSpace(d.manual)
	}
	if !d.canChoose() {
		return
	}
	a.paths[d.side] = d.ep.URL(p)
	dir := p
	if !d.pickDir {
		dir = parentDir(d.ep.Scheme, p)
	}
	if a.remotesFile != "" {
		_, _ = config.AddRemote(a.remotesFile, config.RemoteBookmark{
			Scheme: d.ep.Scheme, Host: d.ep.Host, Port: d.ep.Port, User: d.ep.User, Domain: d.ep.Domain, Share: d.ep.Share,
			Dir: dir, IdentityFile: a.remoteIdentities[d.ep.Key()],
		})
	}
	a.closeRemoteDialog()
	if strings.TrimSpace(a.paths[1-d.side]) != "" {
		a.guard(a.load)
	}
}

// child 与 parentDir 按协议的路径写法拼接：SFTP、WebDAV 用绝对路径，FTP、SMB 相对起点。
func child(scheme, dir, name string) string {
	p := path.Join(dir, name)
	if scheme == "ftp" || scheme == "ftps" || scheme == "smb" {
		return strings.TrimLeft(p, "/")
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

func parentDir(scheme, p string) string {
	parent := path.Dir(strings.TrimRight(p, "/"))
	if parent == "." {
		parent = ""
	}
	if parent == "" && scheme != "ftp" && scheme != "ftps" && scheme != "smb" {
		parent = "/"
	}
	return parent
}

// crumbs 返回面包屑文字和每一项对应的目录；SMB 第一项回到共享列表。
func (d *remoteDialog) crumbs() ([]string, []func(a *workspaceState)) {
	parts := strings.FieldsFunc(d.dir, func(r rune) bool { return r == '/' })
	var items []string
	var jumps []func(a *workspaceState)
	scheme := d.ep.Scheme
	switch scheme {
	case "smb":
		items = append(items, d.ep.Host)
		jumps = append(jumps, func(a *workspaceState) { d.ep.Share = ""; a.remoteList("") })
		if d.ep.Share != "" {
			items = append(items, d.ep.Share)
			jumps = append(jumps, func(a *workspaceState) { a.remoteList("") })
		}
	case "ftp", "ftps":
		items = append(items, "~")
		jumps = append(jumps, func(a *workspaceState) { a.remoteList("") })
	default:
		items = append(items, "/")
		jumps = append(jumps, func(a *workspaceState) { a.remoteList("/") })
	}
	for i, part := range parts {
		target := strings.Join(parts[:i+1], "/")
		if scheme != "smb" && scheme != "ftp" && scheme != "ftps" {
			target = "/" + target
		}
		items = append(items, part)
		jumps = append(jumps, func(a *workspaceState) { a.remoteList(target) })
	}
	return items, jumps
}

// canChoose 报告「打开 / 选择此目录」是否可用。目录模式下 SMB 必须先进入某个共享。
func (d *remoteDialog) canChoose() bool {
	switch {
	case d.loading:
		return false
	case d.noList:
		return strings.TrimSpace(d.manual) != ""
	case d.pickDir:
		return d.browse && !(d.ep.Scheme == "smb" && d.ep.Share == "")
	}
	return d.selected != ""
}

func (a *workspaceState) remoteOpenEntry(e remote.Entry) {
	d := a.remoteDialog
	if d == nil || d.loading {
		return
	}
	if d.ep.Scheme == "smb" && d.ep.Share == "" {
		d.ep.Share = e.Name
		a.remoteList("")
		return
	}
	p := child(d.ep.Scheme, d.dir, e.Name)
	if e.Dir {
		a.remoteList(p)
		return
	}
	if !d.pickDir {
		d.selected = p
	}
}

func (a *workspaceState) remoteUp() {
	d := a.remoteDialog
	if d == nil {
		return
	}
	if d.ep.Scheme == "smb" && d.dir == "" {
		d.ep.Share = ""
		a.remoteList("")
		return
	}
	a.remoteList(parentDir(d.ep.Scheme, d.dir))
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

func (a *workspaceState) remoteDialogView(c *ui.Context, pal uiPalette) {
	d := a.remoteDialog
	if d == nil {
		return
	}
	ui.Modal(c, &d.opened, func() {
		card := ui.Column(c).Width(620).Padding(20, 22).Gap(12).Radius(12).Background(pal.cardBg).Border(1, pal.cardBorder).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.22))
		if d.browse {
			// 浏览页固定高度，目录切换时对话框不跳动；表单页按内容自适应。
			card.Height(560)
		}
		card.Children(func() {
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Icon(c, iconFolderOpen).Size(16, 16).TextColor(pal.baseAccent)
				titles := []string{"remote.dialogTitleLeft", "remote.dialogTitleRight"}
				if d.pickDir {
					titles = []string{"remote.dirTitleLeft", "remote.dirTitleRight"}
				}
				ui.Text(c, i18n.T(titles[d.side])).FontSize(15).Bold()
				ui.Box(c).Grow(1)
				ui.Button(c, "✕").Label(i18n.T("dialog.cancel")).Padding(2, 6).Radius(4).FontSize(11).OnClick(a.closeRemoteDialog)
			})
			if d.browse {
				a.remoteBrowseView(c, pal, d)
			} else {
				a.remoteFormView(c, pal, d)
			}
		})
	})
	if !d.opened {
		a.closeRemoteDialog()
	}
}

func (a *workspaceState) remoteFormView(c *ui.Context, pal uiPalette, d *remoteDialog) {
	t := c.Theme()
	scheme := d.scheme()
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		if len(d.recents) > 0 {
			ui.Text(c, i18n.T("remote.recent")).FontSize(12).Bold().TextColor(pal.sidebarText)
			ui.Row(c).FillWidth().Gap(6).Wrap().Children(func() {
				for _, b := range d.recents {
					b := b
					label := b.Scheme + "  " + bookmarkTitle(b)
					ui.Button(c, label).Padding(3, 8).Radius(5).FontSize(11).OnClick(func() { d.fillFromBookmark(b); a.remoteConnect() })
				}
			})
			ui.Box(c).Height(1).FillWidth().Background(pal.cardBorder)
		}
		formRow(c, i18n.T("remote.protocol"), func() {
			sel := ui.Select(c, &d.protocol, protocolLabels()).Grow(1)
			if sel.Changed() {
				d.port, d.err = "", ""
			}
		})
		// 只有存在 ~/.ssh/config 时才出现；这是可选的快捷导入，所有字段都可以手动填写。
		if scheme == "sftp" && len(d.sshHosts) > 0 {
			formRow(c, i18n.T("remote.sshConfig"), func() {
				options := []string{i18n.T("remote.sshConfigNone")}
				for _, h := range d.sshHosts {
					options = append(options, h.Alias)
				}
				if d.sshChoice == "" {
					d.sshChoice = options[0]
				}
				if ui.Select(c, &d.sshChoice, options).Grow(1).Changed() {
					for _, h := range d.sshHosts {
						if h.Alias == d.sshChoice {
							d.host, d.port, d.user, d.identity = h.HostName, h.Port, h.User, h.IdentityFile
						}
					}
				}
			})
		}
		formRow(c, i18n.T("remote.host"), func() {
			ui.TextInput(c.Key("remote-host"), &d.host).Grow(1).MinWidth(0).Placeholder(i18n.T("remote.hostPlaceholder")).Label(i18n.T("remote.host")).OnSubmit(a.remoteConnect)
			ui.Text(c, i18n.T("remote.port")).FontSize(12).TextColor(t.TextMuted)
			ui.TextInput(c.Key("remote-port"), &d.port).Width(70).Placeholder(remote.DefaultPort(scheme)).Label(i18n.T("remote.port")).OnSubmit(a.remoteConnect)
		})
		formRow(c, i18n.T("remote.user"), func() {
			ui.TextInput(c.Key("remote-user"), &d.user).Grow(1).MinWidth(0).Placeholder(i18n.T("remote.userPlaceholder")).Label(i18n.T("remote.user")).OnSubmit(a.remoteConnect)
		})
		if scheme == "smb" {
			formRow(c, i18n.T("remote.domain"), func() {
				ui.TextInput(c.Key("remote-domain"), &d.domain).Grow(1).MinWidth(0).Placeholder(i18n.T("remote.domainPlaceholder")).Label(i18n.T("remote.domain")).OnSubmit(a.remoteConnect)
			})
			formRow(c, i18n.T("remote.share"), func() {
				ui.TextInput(c.Key("remote-share"), &d.share).Grow(1).MinWidth(0).Placeholder(i18n.T("remote.sharePlaceholder")).Label(i18n.T("remote.share")).OnSubmit(a.remoteConnect)
			})
		}
		formRow(c, i18n.T("remote.passwordPlaceholder"), func() {
			placeholder := ""
			if scheme == "sftp" {
				placeholder = i18n.T("remote.sftpPasswordPlaceholder")
			}
			ui.TextInput(c.Key("remote-pass"), &d.password).Password().Grow(1).MinWidth(0).Placeholder(placeholder).Label(i18n.T("remote.passwordPlaceholder")).OnSubmit(a.remoteConnect)
		})
		if scheme == "sftp" {
			formRow(c, i18n.T("remote.identity"), func() {
				ui.TextInput(c.Key("remote-identity"), &d.identity).Grow(1).MinWidth(0).Placeholder(i18n.T("remote.identityPlaceholder")).Label(i18n.T("remote.identity"))
				ui.Button(c, i18n.T("action.browse")).Padding(3, 8).Radius(5).FontSize(11).OnClick(func() {
					paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: i18n.T("remote.identity")})
					if err == nil && len(paths) > 0 {
						d.identity = paths[0]
					}
				})
			})
			formRow(c, i18n.T("remote.passphrase"), func() {
				ui.TextInput(c.Key("remote-passphrase"), &d.passphrase).Password().Grow(1).MinWidth(0).Placeholder(i18n.T("remote.passphrasePlaceholder")).Label(i18n.T("remote.passphrase")).OnSubmit(a.remoteConnect)
			})
			ui.Text(c, i18n.T("remote.sftpHint")).FontSize(11).TextColor(t.TextMuted)
		} else {
			ui.Text(c, i18n.T("remote.passwordHint")).FontSize(11).TextColor(t.TextMuted)
		}
		if d.err != "" {
			ui.Text(c, d.err).FontSize(12).TextColor(pal.dangerFg).Selectable()
		}
		ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
			ui.Button(c, i18n.T("dialog.cancel")).Padding(5, 14).Radius(6).FontSize(12).OnClick(a.closeRemoteDialog)
			label := i18n.T("remote.connect")
			if d.loading {
				label = i18n.T("remote.connecting")
			}
			ui.PrimaryButton(c, label).Padding(5, 14).Radius(6).FontSize(12).Disabled(d.loading || strings.TrimSpace(d.host) == "").OnClick(a.remoteConnect)
		})
	})
}

func bookmarkTitle(b config.RemoteBookmark) string {
	s := b.Host
	if b.User != "" {
		s = b.User + "@" + s
	}
	if b.Port != "" {
		s += ":" + b.Port
	}
	if b.Share != "" {
		s += "/" + b.Share
	}
	return s
}

// formRow 是左侧固定宽度标签、右侧输入的一行。
func formRow(c *ui.Context, label string, fn func()) {
	ui.Row(c).FillWidth().Gap(8).Children(func() {
		ui.Text(c, label).Width(86).FontSize(12).SingleLine()
		fn()
	})
}

func (a *workspaceState) remoteBrowseView(c *ui.Context, pal uiPalette, d *remoteDialog) {
	t := c.Theme()
	ui.Row(c).FillWidth().Gap(8).Children(func() {
		ui.Text(c, d.ep.Scheme+"://"+bookmarkTitle(config.RemoteBookmark{Host: d.ep.Host, User: d.ep.User, Port: d.ep.Port})).FontSize(12).Bold().Grow(1).MinWidth(0).SingleLine()
		ui.Button(c, i18n.T("remote.changeConnection")).Padding(3, 8).Radius(5).FontSize(11).OnClick(func() {
			if d.cancel != nil {
				d.cancel()
			}
			d.browse, d.loading, d.err = false, false, ""
		})
	})
	if d.noList {
		ui.Column(c).Grow(1).FillWidth().Gap(8).Children(func() {
			ui.Text(c, i18n.T("remote.noList")).FontSize(12).TextColor(t.TextMuted)
			ui.TextInput(c.Key("remote-manual"), &d.manual).FillWidth().Placeholder(i18n.T("remote.manualPlaceholder")).Label(i18n.T("remote.manualPlaceholder")).OnSubmit(a.remoteChoose)
		})
	} else {
		items, jumps := d.crumbs()
		ui.Row(c).FillWidth().Gap(6).Children(func() {
			ui.Button(c, "↑").Label(i18n.T("remote.up")).Tooltip(i18n.T("remote.up")).Padding(2, 8).Radius(5).FontSize(12).Disabled(d.loading || len(items) <= 1).OnClick(a.remoteUp)
			chosen := len(items) - 1
			if ui.Breadcrumbs(c, items, &chosen).Label(i18n.T("remote.location")).Grow(1).MinWidth(0).Changed() && !d.loading {
				jumps[chosen](a)
			}
			if d.loading {
				ui.Spinner(c).Size(14, 14)
			}
		})
		entries := d.entries
		ui.Column(c).Grow(1).FillWidth().Radius(6).Border(1, pal.cardBorder).Children(func() {
			if len(entries) == 0 && !d.loading {
				ui.Column(c).Grow(1).FillWidth().Center().Children(func() {
					ui.Text(c, i18n.T("remote.emptyDir")).FontSize(12).TextColor(t.TextMuted)
				})
				return
			}
			ui.List(c, &d.list, len(entries)).Grow(1).FillWidth().ItemKey(func(i int) any { return entries[i].Name }).Rows(func(row ui.ListRow) {
				e := entries[row.Index]
				rc := row.Context
				full := child(d.ep.Scheme, d.dir, e.Name)
				rowBg := ui.Transparent
				if !e.Dir && full == d.selected {
					rowBg = pal.baseBg
				} else if row.Index%2 == 1 {
					rowBg = pal.subtleSurface.Alpha(0.35)
				}
				ui.Row(rc).Height(30).Padding(0, 10).Gap(8).Background(rowBg).Children(func() {
					icon, color := iconFile, t.TextMuted
					if e.Dir {
						icon, color = iconFolderOpen, pal.baseAccent
					}
					ui.Icon(rc, icon).Size(13, 13).TextColor(color)
					// 选择目录时文件只作参考，不可点选。
					btn := ui.Button(rc, e.Name).Label(e.Name).Grow(1).MinWidth(0).Padding(2, 4).Justify(ui.Start).BorderWidth(0).Background(ui.Transparent).Disabled(d.loading || (d.pickDir && !e.Dir))
					if e.Dir {
						btn.Bold()
					}
					btn.OnClick(func() { a.remoteOpenEntry(e) })
					if !e.Dir && !d.pickDir && btn.DoubleClicked() {
						d.selected = full
						a.remoteChoose()
					}
					if !e.Dir {
						ui.Text(rc, humanSize(e.Size)).Width(72).FontSize(11).TextColor(t.TextMuted).SingleLine()
					}
					if !e.ModTime.IsZero() {
						ui.Text(rc, e.ModTime.Local().Format("2006-01-02 15:04")).Width(110).FontSize(11).TextColor(t.TextMuted).SingleLine()
					}
				})
			})
		})
	}
	if d.err != "" {
		ui.Text(c, d.err).FontSize(12).TextColor(pal.dangerFg).Selectable()
	}
	ui.Row(c).FillWidth().Gap(8).Children(func() {
		shown := d.selected
		if d.pickDir {
			shown = d.ep.URL(d.dir)
		}
		if shown == "" {
			shown = i18n.T("remote.selectHint")
		}
		ui.Text(c, shown).FontSize(11).TextColor(t.TextMuted).Grow(1).MinWidth(0).SingleLine().Tooltip(shown)
		ui.Button(c, i18n.T("dialog.cancel")).Padding(5, 14).Radius(6).FontSize(12).OnClick(a.closeRemoteDialog)
		choose := i18n.T("remote.choose")
		if d.pickDir {
			choose = i18n.T("remote.chooseDir")
		}
		ui.PrimaryButton(c, choose).Padding(5, 14).Radius(6).FontSize(12).Disabled(!d.canChoose()).OnClick(a.remoteChoose)
	})
}
