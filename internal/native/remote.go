package native

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/egoist/mygo/ui"
	"gocompare/internal/document"
	"gocompare/internal/i18n"
	"gocompare/internal/remote"
)

// remoteTimeout 限制单次远程读写；超时后任务结束并报错，界面恢复可操作。
const remoteTimeout = 2 * time.Minute

// remoteInputError 记录失败的远程地址，界面据此决定是否询问密码。
type remoteInputError struct {
	raw string
	err error
}

func (e *remoteInputError) Error() string {
	return remote.Redact(e.raw) + ": " + friendlyRemoteError(e.err)
}
func (e *remoteInputError) Unwrap() error { return e.err }

// passwordPrompt 是等待用户输入的密码请求；retry 在提交后重新执行原操作。
type passwordPrompt struct {
	// passphrase 为真时询问的是 SFTP 私钥口令而不是登录密码。
	passphrase bool
	key        string
	label      string
	again      bool
	input      string
	retry      func()
	opened     bool
}

// hostKeyPrompt 是等待用户确认的未知 SSH 主机密钥。
type hostKeyPrompt struct {
	err    *remote.HostKeyError
	retry  func()
	opened bool
}

// remoteAuth 是界面线程复制出的凭据快照，后台任务只读不写；键为 remote.CredentialKey。
type remoteAuth struct {
	passwords   map[string]string
	identities  map[string]string
	passphrases map[string]string
}

// apply 为存储补上内存中的密码与 SFTP 私钥；URL 自带的密码优先。
func (r remoteAuth) apply(raw string, store remote.Store) {
	key := remote.CredentialKey(raw)
	if pass := r.passwords[key]; pass != "" && !remote.HasPassword(store) {
		remote.SetPassword(store, pass)
	}
	if s, ok := store.(*remote.SFTP); ok {
		if r.identities[key] != "" {
			s.IdentityFiles = []string{r.identities[key]}
		}
		s.Passphrase = r.passphrases[key]
	}
}

// openInput 读取比较输入：远程地址走 remote，其余按本地文件读取。
func openInput(ctx context.Context, path string, missing bool, auth remoteAuth) (*document.File, error) {
	if !remote.IsURL(path) {
		return readInput(ctx, path, missing)
	}
	store, rel, err := remote.ParseURL(path)
	if err != nil {
		return nil, err
	}
	auth.apply(path, store)
	src := remote.File{Store: store, Path: rel, Timeout: remoteTimeout}
	f, err := document.ReadRemote(ctx, remote.Redact(path), src)
	if missing && errors.Is(err, fs.ErrNotExist) {
		// 目录比较里只在一侧存在的文件：远程一侧给空白缓冲区，保存时创建。
		return document.EmptyRemote(remote.Redact(path), src), nil
	}
	if err != nil {
		return nil, &remoteInputError{raw: path, err: err}
	}
	return f, nil
}

// readInputBytes 读取用于比较的原始字节：远程地址从远端读取，本地文件从本地读取。
func readInputBytes(ctx context.Context, path string, max int64, auth remoteAuth) ([]byte, error) {
	if !remote.IsURL(path) {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		if max > 0 {
			data, err := io.ReadAll(io.LimitReader(f, max+1))
			if err != nil {
				return nil, err
			}
			if int64(len(data)) > max {
				return nil, fmt.Errorf("%s exceeds %d bytes limit", path, max)
			}
			return data, nil
		}
		return io.ReadAll(f)
	}
	store, rel, err := remote.ParseURL(path)
	if err != nil {
		return nil, err
	}
	auth.apply(path, store)
	src := remote.File{Store: store, Path: rel, Timeout: remoteTimeout}
	data, err := src.Read(ctx)
	if err != nil {
		return nil, &remoteInputError{raw: path, err: err}
	}
	if max > 0 && int64(len(data)) > max {
		return nil, fmt.Errorf("%s exceeds %d bytes limit", remote.Redact(path), max)
	}
	return data, nil
}

// openInputTemp 为需要本地文件句柄或 seek 的模块准备输入。
// 如果是远程地址，将内容下载到临时文件，返回临时文件路径与清理函数。
func openInputTemp(ctx context.Context, path string, max int64, auth remoteAuth) (string, func(), error) {
	if !remote.IsURL(path) {
		return path, func() {}, nil
	}
	data, err := readInputBytes(ctx, path, max, auth)
	if err != nil {
		return "", func() {}, err
	}
	ext := filepath.Ext(path)
	tmp, err := os.CreateTemp("", "gocompare-remote-*"+ext)
	if err != nil {
		return "", func() {}, err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return "", func() {}, err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return "", func() {}, err
	}
	return name, func() { _ = os.Remove(name) }, nil
}

// takeURLPasswords 把输入框里 URL 携带的密码移入内存凭据表，并从输入框中抹掉，
// 避免密码留在界面、窗口标题或保存的会话里。返回凭据快照供后台任务使用。
func (a *workspaceState) takeURLPasswords() remoteAuth {
	if a.remoteSecrets == nil {
		a.remoteSecrets = map[string]string{}
	}
	for i, p := range a.paths {
		if !remote.IsURL(p) {
			continue
		}
		if pass := remote.PasswordOf(p); pass != "" {
			a.remoteSecrets[remote.CredentialKey(p)] = pass
			a.paths[i] = remote.Redact(p)
		}
	}
	return remoteAuth{passwords: maps.Clone(a.remoteSecrets), identities: maps.Clone(a.remoteIdentities), passphrases: maps.Clone(a.remotePassphrases)}
}

// handleRemoteError 把需要用户介入的远程错误转成对话框；返回 false 时调用方按普通错误显示。
func (a *workspaceState) handleRemoteError(err error, retry func()) bool {
	var hostKey *remote.HostKeyError
	if errors.As(err, &hostKey) {
		a.hostKeyPrompt = &hostKeyPrompt{err: hostKey, retry: retry, opened: true}
		return true
	}
	var input *remoteInputError
	if !errors.As(err, &input) {
		return false
	}
	if errors.Is(err, remote.ErrKeyPassphrase) || errors.Is(err, remote.ErrKeyPassphraseWrong) {
		key := remote.CredentialKey(input.raw)
		a.passwordPrompt = &passwordPrompt{passphrase: true, key: key, label: remote.Redact(input.raw), again: errors.Is(err, remote.ErrKeyPassphraseWrong), retry: retry, opened: true}
		return true
	}
	if !errors.Is(err, remote.ErrAuth) {
		return false
	}
	store, _, perr := remote.ParseURL(input.raw)
	if perr != nil || !remote.AcceptsPassword(store) {
		return false
	}
	key := remote.CredentialKey(input.raw)
	_, tried := a.remoteSecrets[key]
	a.passwordPrompt = &passwordPrompt{key: key, label: remote.Redact(input.raw), again: tried || remote.HasPassword(store), retry: retry, opened: true}
	return true
}

func (a *workspaceState) submitPassword() {
	p := a.passwordPrompt
	if p == nil {
		return
	}
	a.passwordPrompt = nil
	if p.passphrase {
		if a.remotePassphrases == nil {
			a.remotePassphrases = map[string]string{}
		}
		a.remotePassphrases[p.key] = p.input
	} else {
		if a.remoteSecrets == nil {
			a.remoteSecrets = map[string]string{}
		}
		a.remoteSecrets[p.key] = p.input
	}
	if p.retry != nil {
		p.retry()
	}
}

func (a *workspaceState) trustHostKey() {
	p := a.hostKeyPrompt
	if p == nil {
		return
	}
	a.hostKeyPrompt = nil
	if err := p.err.Trust(); err != nil {
		a.err = err.Error()
		return
	}
	if p.retry != nil {
		p.retry()
	}
}

func (a *workspaceState) remoteDialogs(c *ui.Context, pal uiPalette) {
	t := c.Theme()
	if p := a.passwordPrompt; p != nil {
		ui.Modal(c, &p.opened, func() {
			ui.Column(c).Width(440).Padding(20, 22).Gap(12).Radius(12).Background(pal.cardBg).Border(1, pal.cardBorder).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.22)).Children(func() {
				title, field, rejected := "remote.passwordTitle", "remote.passwordPlaceholder", "remote.passwordRejected"
				if p.passphrase {
					title, field, rejected = "remote.passphraseTitle", "remote.passphrase", "remote.errKeyPassphraseWrong"
				}
				ui.Text(c, i18n.T(title)).FontSize(15).Bold()
				ui.Text(c, p.label).FontSize(12).TextColor(t.TextMuted).Selectable()
				if p.again {
					ui.Text(c, i18n.T(rejected)).FontSize(12).TextColor(pal.dangerFg)
				}
				ui.TextInput(c.Key("remote-password"), &p.input).Password().FillWidth().Placeholder(i18n.T(field)).Label(i18n.T(field)).OnSubmit(a.submitPassword)
				ui.Text(c, i18n.T("remote.passwordHint")).FontSize(11).TextColor(t.TextMuted)
				ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
					ui.Button(c, i18n.T("dialog.cancel")).Padding(5, 14).Radius(6).FontSize(12).OnClick(func() { a.passwordPrompt = nil })
					ui.PrimaryButton(c, i18n.T("remote.connect")).Padding(5, 14).Radius(6).FontSize(12).OnClick(a.submitPassword)
				})
			})
		})
		if !p.opened {
			a.passwordPrompt = nil
		}
	}
	if p := a.hostKeyPrompt; p != nil {
		ui.Modal(c, &p.opened, func() {
			ui.Column(c).Width(500).Padding(20, 22).Gap(12).Radius(12).Background(pal.cardBg).Border(1, pal.cardBorder).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.22)).Children(func() {
				ui.Text(c, i18n.T("remote.hostKeyTitle")).FontSize(15).Bold()
				ui.Text(c, i18n.TArgs("remote.hostKeyDesc", p.err.Host)).FontSize(12)
				ui.Text(c, p.err.KeyType+"  "+p.err.Fingerprint).FontSize(12).Bold().Selectable()
				ui.Text(c, i18n.TArgs("remote.hostKeyFile", p.err.KnownHosts)).FontSize(11).TextColor(t.TextMuted).Selectable()
				ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
					ui.Button(c, i18n.T("dialog.cancel")).Padding(5, 14).Radius(6).FontSize(12).OnClick(func() { a.hostKeyPrompt = nil })
					ui.PrimaryButton(c, i18n.T("remote.trustConnect")).Padding(5, 14).Radius(6).FontSize(12).OnClick(a.trustHostKey)
				})
			})
		})
		if !p.opened {
			a.hostKeyPrompt = nil
		}
	}
}

// friendlyRemoteError 把常见连接与认证错误转成界面文案，其余保留原始信息以便排查。
func friendlyRemoteError(err error) string {
	var dnsErr *net.DNSError
	switch {
	case errors.Is(err, remote.ErrHostKeyChanged):
		// 附上原始信息：其中的指纹是与管理员核对的依据。
		return i18n.T("remote.errHostKeyChanged") + "\n" + err.Error()
	case errors.Is(err, remote.ErrKeyPassphrase):
		return i18n.T("remote.errKeyPassphrase")
	case errors.Is(err, remote.ErrKeyPassphraseWrong):
		return i18n.T("remote.errKeyPassphraseWrong")
	case strings.Contains(err.Error(), "read identity") && errors.Is(err, fs.ErrNotExist):
		return i18n.T("remote.errKeyMissing")
	case errors.Is(err, fs.ErrNotExist):
		return i18n.T("remote.errNotFound")
	case strings.Contains(err.Error(), "parse identity"):
		return i18n.T("remote.errKeyInvalid")
	case errors.Is(err, remote.ErrAuth):
		return i18n.T("remote.errAuth")
	case errors.Is(err, context.DeadlineExceeded):
		return i18n.T("remote.errTimeout")
	case errors.Is(err, syscall.ECONNREFUSED):
		return i18n.T("remote.errRefused")
	case errors.As(err, &dnsErr):
		return i18n.T("remote.errNoHost")
	}
	return err.Error()
}
