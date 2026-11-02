package native

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/egoist/mygo/ui"
	"gocompare/internal/compare"
	"gocompare/internal/config"
	"gocompare/internal/i18n"
	"gocompare/internal/remote"
)

// davServer 是要求 Basic 认证 u/p 的 WebDAV 测试服务端。
type davServer struct {
	mu    sync.Mutex
	files map[string]string
}

func startDAV(t *testing.T, files map[string]string) (*davServer, string) {
	t.Helper()
	d := &davServer{files: files}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, pass, ok := r.BasicAuth(); !ok || user != "u" || pass != "p" {
			http.Error(w, "auth", http.StatusUnauthorized)
			return
		}
		d.mu.Lock()
		defer d.mu.Unlock()
		switch r.Method {
		case http.MethodGet:
			body, ok := d.files[r.URL.Path]
			if !ok {
				http.NotFound(w, r)
				return
			}
			io.WriteString(w, body)
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			d.files[r.URL.Path] = string(body)
		case "PROPFIND":
			// 列出 r.URL.Path 下的直接子项：文件直接列出，更深的路径归为子目录。
			dir := strings.TrimSuffix(r.URL.Path, "/") + "/"
			exists := dir == "/"
			for name := range d.files {
				exists = exists || strings.HasPrefix(name, dir)
			}
			if !exists {
				http.NotFound(w, r)
				return
			}
			var b strings.Builder
			b.WriteString(`<?xml version="1.0"?><d:multistatus xmlns:d="DAV:"><d:response><d:href>` + dir + `</d:href><d:propstat><d:prop><d:resourcetype><d:collection/></d:resourcetype></d:prop></d:propstat></d:response>`)
			seen := map[string]bool{}
			for name, body := range d.files {
				if !strings.HasPrefix(name, dir) {
					continue
				}
				rest := strings.TrimPrefix(name, dir)
				if sub, _, nested := strings.Cut(rest, "/"); nested {
					if !seen[sub] {
						seen[sub] = true
						b.WriteString(`<d:response><d:href>` + dir + sub + `/</d:href><d:propstat><d:prop><d:resourcetype><d:collection/></d:resourcetype></d:prop></d:propstat></d:response>`)
					}
					continue
				}
				fmt.Fprintf(&b, `<d:response><d:href>%s%s</d:href><d:propstat><d:prop><d:resourcetype/><d:getcontentlength>%d</d:getcontentlength></d:prop></d:propstat></d:response>`, dir, rest, len(body))
			}
			b.WriteString(`</d:multistatus>`)
			w.WriteHeader(207)
			io.WriteString(w, b.String())
		}
	}))
	t.Cleanup(srv.Close)
	return d, strings.Replace(srv.URL, "http://", "webdav://", 1)
}

func TestRemoteCompareAsksPasswordAndSaves(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	dav, base := startDAV(t, map[string]string{"/a.txt": "remote\n"})
	local := filepath.Join(t.TempDir(), "a.txt")
	os.WriteFile(local, []byte("local\n"), 0o600)
	remoteURL := strings.Replace(base, "webdav://", "webdav://u@", 1) + "/a.txt"

	a := &workspaceState{paths: [2]string{remoteURL, local}}
	a.load()
	if a.passwordPrompt == nil || a.panes[0] != nil {
		t.Fatalf("expected password prompt, err=%q", a.err)
	}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if !tt.HasText(i18n.T("remote.passwordTitle")) {
		t.Fatal("password dialog not shown")
	}
	a.passwordPrompt.input = "p"
	a.submitPassword()
	if a.panes[0] == nil || !a.panes[0].file.IsRemote() || a.panes[0].file.Text != "remote\n" {
		t.Fatalf("remote file not loaded: %q", a.err)
	}

	a.panes[0].buffer.Set("edited\n")
	a.save(0)
	if a.err != "" {
		t.Fatal(a.err)
	}
	dav.mu.Lock()
	got := dav.files["/a.txt"]
	dav.mu.Unlock()
	if got != "edited\n" {
		t.Fatalf("remote content %q", got)
	}
	if strings.Contains(a.paths[0], ":p@") {
		t.Fatalf("password leaked into path field: %q", a.paths[0])
	}
}

func TestRemoteURLPasswordIsRemovedFromInput(t *testing.T) {
	_, base := startDAV(t, map[string]string{"/a.txt": "x\n", "/b.txt": "y\n"})
	withPass := strings.Replace(base, "webdav://", "webdav://u:p@", 1)
	a := &workspaceState{paths: [2]string{withPass + "/a.txt", withPass + "/b.txt"}}
	a.load()
	if a.panes[0] == nil {
		t.Fatalf("load failed: %q", a.err)
	}
	for _, p := range a.paths {
		if remote.PasswordOf(p) != "" {
			t.Fatalf("password left in path %q", p)
		}
	}
	// 凭据留在内存，重新加载不再询问。
	a.load()
	if a.passwordPrompt != nil || a.err != "" {
		t.Fatalf("reload asked again: %q", a.err)
	}
}

func TestRemoteWrongPasswordAsksAgain(t *testing.T) {
	_, base := startDAV(t, map[string]string{"/a.txt": "x\n"})
	u := strings.Replace(base, "webdav://", "webdav://u:bad@", 1) + "/a.txt"
	a := &workspaceState{paths: [2]string{u, u}}
	a.load()
	if a.passwordPrompt == nil || !a.passwordPrompt.again {
		t.Fatalf("rejected password must prompt again: %+v %q", a.passwordPrompt, a.err)
	}
}

func TestHostKeyDialogShowsFingerprint(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	a := &workspaceState{}
	if !a.handleRemoteError(&remote.HostKeyError{Host: "[h]:2222", Fingerprint: "SHA256:abc", KeyType: "ssh-ed25519", KnownHosts: "/tmp/kh"}, nil) {
		t.Fatal("host key error not handled")
	}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if !tt.HasText("ssh-ed25519  SHA256:abc") || !tt.HasText(i18n.T("remote.trustConnect")) {
		t.Fatal("host key dialog missing fingerprint or trust button")
	}
}

// fillDAV 把对话框填成指向测试 WebDAV 服务端的连接。
func fillDAV(t *testing.T, d *remoteDialog, base, pass string) {
	t.Helper()
	hostPort := strings.TrimPrefix(base, "webdav://")
	host, port, _ := strings.Cut(hostPort, ":")
	d.setScheme("webdav")
	d.host, d.port, d.user, d.password = host, port, "u", pass
}

func TestRemoteDialogBrowseAndChoose(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	_, base := startDAV(t, map[string]string{"/a.txt": "remote\n", "/conf/app.conf": "x=1\n"})
	local := filepath.Join(t.TempDir(), "a.txt")
	os.WriteFile(local, []byte("local\n"), 0o600)
	a := &workspaceState{kind: kindFile, paths: [2]string{"", local}, remotesFile: filepath.Join(t.TempDir(), "remotes.json")}

	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if !tt.HasText(i18n.T("remote.open")) {
		t.Fatal("remote button missing from the path bar")
	}
	a.openRemoteDialog(0)
	d := a.remoteDialog
	fillDAV(t, d, base, "p")
	tt.Frame()
	if !tt.HasText(i18n.T("remote.dialogTitleLeft")) || !tt.HasText(i18n.T("remote.host")) {
		t.Fatal("connection form not shown")
	}
	a.remoteConnect()
	if !d.browse || d.err != "" || len(d.entries) != 2 || !d.entries[0].Dir || d.entries[0].Name != "conf" || d.entries[1].Name != "a.txt" {
		t.Fatalf("browse %v err %q entries %+v", d.browse, d.err, d.entries)
	}
	tt.Frame()
	if !tt.HasText("a.txt") || !tt.HasText(i18n.T("remote.choose")) {
		t.Fatal("file list not shown")
	}

	a.remoteOpenEntry(d.entries[0])
	if d.dir != "/conf" || len(d.entries) != 1 || d.entries[0].Name != "app.conf" {
		t.Fatalf("enter folder: %q %+v", d.dir, d.entries)
	}
	a.remoteUp()
	if d.dir != "/" {
		t.Fatalf("up: %q", d.dir)
	}
	a.remoteOpenEntry(d.entries[1])
	if d.selected != "/a.txt" {
		t.Fatalf("selected %q", d.selected)
	}
	a.remoteChoose()
	if a.remoteDialog != nil {
		t.Fatal("dialog still open after choosing")
	}
	if remote.PasswordOf(a.paths[0]) != "" || !strings.HasSuffix(a.paths[0], "/a.txt") || !strings.HasPrefix(a.paths[0], "webdav://u@") {
		t.Fatalf("path %q", a.paths[0])
	}
	// 另一侧已有路径，选定后直接加载，凭据来自对话框，不再询问。
	if a.panes[0] == nil || !a.panes[0].file.IsRemote() || a.passwordPrompt != nil {
		t.Fatalf("not loaded: %q", a.err)
	}
	// 连接被记住（不含密码），再次打开对话框自动填好。
	data, _ := os.ReadFile(a.remotesFile)
	if !strings.Contains(string(data), `"host": "127.0.0.1"`) || strings.Contains(string(data), `"p"`) {
		t.Fatalf("bookmark %s", data)
	}
	a.openRemoteDialog(0)
	if a.remoteDialog.host != "127.0.0.1" || a.remoteDialog.scheme() != "webdav" || a.remoteDialog.dir != "/" {
		t.Fatalf("prefill %+v", a.remoteDialog)
	}
}

func TestRemoteDialogWrongPassword(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	_, base := startDAV(t, map[string]string{"/a.txt": "x"})
	a := &workspaceState{kind: kindFile}
	a.openRemoteDialog(1)
	fillDAV(t, a.remoteDialog, base, "bad")
	a.remoteConnect()
	if a.remoteDialog.browse || a.remoteDialog.err != i18n.T("remote.errAuth") {
		t.Fatalf("err %q browse %v", a.remoteDialog.err, a.remoteDialog.browse)
	}
	if a.remoteDialog.loading {
		t.Fatal("still loading after failure")
	}
}

func TestRemoteDialogDarkAndSMBForm(t *testing.T) {
	i18n.SetLanguage(i18n.EnUS)
	defer i18n.SetLanguage(i18n.ZhCN)
	a := &workspaceState{kind: kindFile}
	a.openRemoteDialog(0)
	a.remoteDialog.setScheme("smb")
	tt := ui.NewTester(a.view, 1200, 800)
	tt.SetDark(true)
	tt.Frame()
	if !tt.HasText(i18n.T("remote.share")) || !tt.HasText(i18n.T("remote.domain")) {
		t.Fatal("SMB fields missing")
	}
}

func TestRemotePathHelpers(t *testing.T) {
	if child("sftp", "/home/a", "x") != "/home/a/x" || child("smb", "", "x") != "x" || child("ftp", "pub", "x") != "pub/x" {
		t.Fatal("child")
	}
	if parentDir("sftp", "/home/a/x") != "/home/a" || parentDir("sftp", "/x") != "/" || parentDir("smb", "x") != "" || parentDir("ftp", "a/b") != "a" {
		t.Fatal("parentDir")
	}
	d := &remoteDialog{}
	d.fillFromURL("smb://CORP;bob@nas:1445/docs/dir/a.txt", false)
	if d.scheme() != "smb" || d.host != "nas" || d.port != "1445" || d.user != "bob" || d.domain != "CORP" || d.share != "docs" || d.dir != "dir" {
		t.Fatalf("fillFromURL %+v", d)
	}
	d = &remoteDialog{ep: remote.Endpoint{Scheme: "smb", Host: "nas", Share: "docs"}, dir: "a/b"}
	items, _ := d.crumbs()
	if strings.Join(items, ">") != "nas>docs>a>b" {
		t.Fatalf("crumbs %v", items)
	}
}

func TestKeyPassphrasePromptAndMessages(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	retried := false
	a := &workspaceState{}
	err := &remoteInputError{raw: "sftp://u@h/x", err: fmt.Errorf("%w: /k", remote.ErrKeyPassphrase)}
	if !a.handleRemoteError(err, func() { retried = true }) || a.passwordPrompt == nil || !a.passwordPrompt.passphrase {
		t.Fatalf("passphrase prompt not opened: %+v", a.passwordPrompt)
	}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if !tt.HasText(i18n.T("remote.passphraseTitle")) {
		t.Fatal("passphrase dialog title missing")
	}
	a.passwordPrompt.input = "secret"
	a.submitPassword()
	if !retried || a.remotePassphrases[remote.CredentialKey("sftp://u@h/x")] != "secret" || a.remoteSecrets[remote.CredentialKey("sftp://u@h/x")] != "" {
		t.Fatalf("passphrase stored wrong: %v %v", a.remotePassphrases, a.remoteSecrets)
	}

	cases := map[error]string{
		fmt.Errorf("%w: /k", remote.ErrKeyPassphrase):      "remote.errKeyPassphrase",
		fmt.Errorf("%w: /k", remote.ErrKeyPassphraseWrong): "remote.errKeyPassphraseWrong",
		fmt.Errorf("read identity /k: %w", os.ErrNotExist): "remote.errKeyMissing",
		fmt.Errorf("parse identity /k: bad"):               "remote.errKeyInvalid",
		fmt.Errorf("%w: x", remote.ErrAuth):                "remote.errAuth",
	}
	changed := fmt.Errorf("%w: h (got ecdsa SHA256:abc)", remote.ErrHostKeyChanged)
	if got := friendlyRemoteError(changed); !strings.HasPrefix(got, i18n.T("remote.errHostKeyChanged")) || !strings.Contains(got, "SHA256:abc") {
		t.Errorf("host key changed message %q", got)
	}
	for e, key := range cases {
		if got := friendlyRemoteError(e); got != i18n.T(key) {
			t.Errorf("%v => %q, want %s", e, got, key)
		}
	}
}

func TestRemoteDialogShowsPassphraseField(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	a := &workspaceState{kind: kindFile}
	a.openRemoteDialog(0)
	d := a.remoteDialog
	d.host = "h"
	d.ep = d.endpoint()
	a.remoteListFailed(d, "", fmt.Errorf("%w: /k", remote.ErrKeyPassphrase))
	if d.browse || d.err != i18n.T("remote.errKeyPassphrase") {
		t.Fatalf("err %q", d.err)
	}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if !tt.HasText(i18n.T("remote.passphrase")) || !tt.HasText(i18n.T("remote.errKeyPassphrase")) {
		t.Fatal("passphrase field or error missing")
	}
}

func TestRemoteDialogStartDirFollowsConnection(t *testing.T) {
	_, base := startDAV(t, map[string]string{"/a.txt": "x", "/conf/app.conf": "y"})
	hostPort := strings.TrimPrefix(base, "webdav://")
	host, port, _ := strings.Cut(hostPort, ":")
	a := &workspaceState{kind: kindFile}
	a.openRemoteDialog(0)
	d := a.remoteDialog
	// 预填自另一台服务器的最近连接，带着那台服务器上的目录。
	d.fillFromBookmark(config.RemoteBookmark{Scheme: "webdav", Host: "other.example", User: "u", Dir: "/home/ykt/gone"})
	// 用户改填当前服务器（相当于从 SSH 配置导入另一台主机）。
	d.host, d.port, d.password = host, port, "p"
	a.remoteConnect()
	if !d.browse || d.err != "" || d.dir != "/" {
		t.Fatalf("stale start dir reused: dir %q err %q", d.dir, d.err)
	}

	// 同一服务器上记住的目录已被删除：自动退回起始目录。
	a.openRemoteDialog(0)
	d = a.remoteDialog
	d.fillFromBookmark(config.RemoteBookmark{Scheme: "webdav", Host: host, Port: port, User: "u", Dir: "/conf"})
	d.password = "p"
	a.remoteConnect()
	if d.dir != "/conf" || d.err != "" {
		t.Fatalf("remembered dir not used: %q %q", d.dir, d.err)
	}
	a.openRemoteDialog(0)
	d = a.remoteDialog
	d.fillFromBookmark(config.RemoteBookmark{Scheme: "webdav", Host: host, Port: port, User: "u", Dir: "/deleted"})
	d.password = "p"
	a.remoteConnect()
	if !d.browse || d.dir != "/" || d.err != "" {
		t.Fatalf("missing remembered dir must fall back to root: %q %q", d.dir, d.err)
	}
}

func TestRemoteDirectoryCompare(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	dav, base := startDAV(t, map[string]string{
		"/same.txt":      "same\n",
		"/edit.txt":      "remote\n",
		"/conf/app.conf": "x=1\n",
		"/only-dav.txt":  "r\n",
	})
	local := t.TempDir()
	os.MkdirAll(filepath.Join(local, "conf"), 0o700)
	os.WriteFile(filepath.Join(local, "same.txt"), []byte("same\n"), 0o600)
	os.WriteFile(filepath.Join(local, "edit.txt"), []byte("local!\n"), 0o600)
	os.WriteFile(filepath.Join(local, "conf", "app.conf"), []byte("x=1\n"), 0o600)
	os.WriteFile(filepath.Join(local, "only-local.txt"), []byte("l\n"), 0o600)

	root := strings.Replace(base, "webdav://", "webdav://u@", 1) + "/"
	a := &workspaceState{kind: kindDirectory, paths: [2]string{local, root}}
	a.load()
	if a.passwordPrompt == nil {
		t.Fatalf("expected password prompt in folder mode, err=%q", a.err)
	}
	a.passwordPrompt.input = "p"
	a.submitPassword()
	if a.directoryRows == nil {
		t.Fatalf("folder compare not loaded: %q", a.err)
	}
	got := map[string]compare.Status{}
	for _, r := range a.directoryRows {
		got[r.Path] = r.Status
	}
	want := map[string]compare.Status{"same.txt": compare.Same, "edit.txt": compare.Different, "conf/app.conf": compare.Same, "only-dav.txt": compare.RightOnly, "only-local.txt": compare.LeftOnly}
	for p, st := range want {
		if got[p] != st {
			t.Errorf("%s = %s, want %s", p, got[p], st)
		}
	}
	if !a.remoteDirectory() {
		t.Fatal("remote root not detected")
	}
	a.previewSync()
	a.previewDirMerge()
	if a.syncPlan != nil || a.dirMergeDecisions != nil {
		t.Fatal("local-only operations ran against a remote folder")
	}
	tt := ui.NewTester(a.view, 1400, 880)
	tt.Frame()

	// 打开只在本地存在的文件：远程一侧是空白缓冲区，保存时在远程创建。
	var onlyLocal compare.Entry
	for _, r := range a.directoryRows {
		if r.Path == "only-local.txt" {
			onlyLocal = r
		}
	}
	a.openDirectoryEntry(onlyLocal)
	if a.panes[1] == nil || !a.panes[1].file.IsRemote() || a.panes[1].file.Text != "" {
		t.Fatalf("remote side of a left-only file: %q", a.err)
	}
	if !strings.HasSuffix(a.paths[1], "/only-local.txt") || remote.PasswordOf(a.paths[1]) != "" {
		t.Fatalf("remote path %q", a.paths[1])
	}
	a.panes[1].buffer.Set("created remotely\n")
	a.save(1)
	dav.mu.Lock()
	created := dav.files["/only-local.txt"]
	dav.mu.Unlock()
	if a.err != "" || created != "created remotely\n" {
		t.Fatalf("remote create: %q %q", created, a.err)
	}
}

func TestRemoteDialogPicksFolder(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	_, base := startDAV(t, map[string]string{"/conf/app.conf": "x=1\n", "/a.txt": "a"})
	local := t.TempDir()
	os.WriteFile(filepath.Join(local, "app.conf"), []byte("x=1\n"), 0o600)
	a := &workspaceState{kind: kindDirectory, paths: [2]string{local, ""}}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if !tt.HasText(i18n.T("remote.open")) {
		t.Fatal("remote button missing in folder mode")
	}
	a.openRemoteDialog(1)
	d := a.remoteDialog
	if !d.pickDir {
		t.Fatal("folder mode must pick folders")
	}
	fillDAV(t, d, base, "p")
	a.remoteConnect()
	tt.Frame()
	if !tt.HasText(i18n.T("remote.dirTitleRight")) || !tt.HasText(i18n.T("remote.chooseDir")) {
		t.Fatal("folder picker labels missing")
	}
	// 文件不能被选中，只能进入目录。
	a.remoteOpenEntry(remote.Entry{Name: "a.txt"})
	if d.selected != "" {
		t.Fatal("file selected in folder mode")
	}
	a.remoteOpenEntry(remote.Entry{Name: "conf", Dir: true})
	if d.dir != "/conf" || !d.canChoose() {
		t.Fatalf("enter folder: %q", d.dir)
	}
	a.remoteChoose()
	if !strings.HasSuffix(a.paths[1], "/conf") || a.directoryRows == nil {
		t.Fatalf("folder not chosen/loaded: %q %q", a.paths[1], a.err)
	}
	for _, r := range a.directoryRows {
		if r.Path == "app.conf" && r.Status != compare.Same {
			t.Fatalf("app.conf %s", r.Status)
		}
	}
}

func TestRemoteButtonPresentInAllModes(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	modes := []int{kindFile, kindDirectory, kindMerge, kindPicture, kindMedia, kindVersion, kindTable}
	for _, m := range modes {
		a := &workspaceState{kind: m}
		tt := ui.NewTester(a.view, 1200, 800)
		tt.Frame()
		if !tt.HasText(i18n.T("remote.open")) {
			t.Fatalf("remote button missing in mode %d", m)
		}
	}
	// Scratchpad mode has no path capsules
	scratch := &workspaceState{kind: kindScratch}
	ttScratch := ui.NewTester(scratch.view, 1200, 800)
	ttScratch.Frame()
	if ttScratch.HasText(i18n.T("remote.open")) {
		t.Fatal("remote button should not be in scratch mode")
	}
}

func TestRemoteMergeAndTableCompare(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	_, base := startDAV(t, map[string]string{
		"/m1.txt": "hello\nworld\n",
		"/t1.csv": "col1,col2\n1,2\n",
	})
	localMerge := filepath.Join(t.TempDir(), "m2.txt")
	os.WriteFile(localMerge, []byte("hello\nthere\n"), 0o600)
	localTable := filepath.Join(t.TempDir(), "t2.csv")
	os.WriteFile(localTable, []byte("col1,col2\n1,3\n"), 0o600)

	// Merge with remote left
	remoteMergeURL := strings.Replace(base, "webdav://", "webdav://u@", 1) + "/m1.txt"
	aMerge := &workspaceState{
		kind:          kindMerge,
		paths:         [2]string{remoteMergeURL, localMerge},
		remoteSecrets: map[string]string{remote.CredentialKey(remoteMergeURL): "p"},
	}
	aMerge.loadMerge()
	if aMerge.err != "" {
		t.Fatalf("loadMerge remote err: %s", aMerge.err)
	}
	if aMerge.mergeResult == nil {
		t.Fatalf("loadMerge remote failed to produce merge result, prompt=%v", aMerge.passwordPrompt != nil)
	}

	// Table with remote left
	remoteTableURL := strings.Replace(base, "webdav://", "webdav://u@", 1) + "/t1.csv"
	aTable := &workspaceState{
		kind:          kindTable,
		paths:         [2]string{remoteTableURL, localTable},
		remoteSecrets: map[string]string{remote.CredentialKey(remoteTableURL): "p"},
	}
	aTable.loadTable()
	if aTable.err != "" {
		t.Fatalf("loadTable remote err: %s", aTable.err)
	}
	if !aTable.tableReady || len(aTable.tableDiffs) == 0 {
		t.Fatalf("loadTable remote ready=%v diffs=%d", aTable.tableReady, len(aTable.tableDiffs))
	}
}
