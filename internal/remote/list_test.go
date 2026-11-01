package remote

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSFTPList(t *testing.T) {
	srv := startSSHServer(t, nil)
	os.Mkdir(filepath.Join(srv.root, "beta"), 0o700)
	os.WriteFile(filepath.Join(srv.root, "Alpha.txt"), []byte("12345"), 0o600)
	os.WriteFile(filepath.Join(srv.root, "zeta.txt"), nil, 0o600)
	client := &SFTP{Addr: srv.addr, User: "u", Pass: "p", KnownHosts: []string{knownHostsFile(t, srv.addr, srv.hostKey.PublicKey())}}
	dir, entries, err := client.List(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if dir == "" || len(entries) != 3 {
		t.Fatalf("dir %q entries %+v", dir, entries)
	}
	// 目录在前，文件按名称不区分大小写排序。
	if !entries[0].Dir || entries[0].Name != "beta" || entries[1].Name != "Alpha.txt" || entries[1].Size != 5 || entries[2].Name != "zeta.txt" {
		t.Fatalf("order %+v", entries)
	}
	// 相对路径按服务端工作目录解析，避免依赖各平台绝对路径写法。
	sub, children, err := client.List(context.Background(), "beta")
	if err != nil || len(children) != 0 || !strings.HasSuffix(sub, "/beta") {
		t.Fatalf("sub %q %+v %v", sub, children, err)
	}
}

func TestWebDAVList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" || r.Header.Get("Depth") != "1" || r.URL.Path != "/docs/" {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		w.WriteHeader(207)
		w.Write([]byte(`<?xml version="1.0"?><d:multistatus xmlns:d="DAV:">
<d:response><d:href>/docs/</d:href><d:propstat><d:prop><d:resourcetype><d:collection/></d:resourcetype></d:prop></d:propstat></d:response>
<d:response><d:href>/docs/sub%20dir/</d:href><d:propstat><d:prop><d:resourcetype><d:collection/></d:resourcetype></d:prop></d:propstat></d:response>
<d:response><d:href>/docs/a.txt</d:href><d:propstat><d:prop><d:resourcetype/><d:getcontentlength>42</d:getcontentlength><d:getlastmodified>Sun, 01 Nov 2026 10:00:00 GMT</d:getlastmodified></d:prop></d:propstat></d:response>
</d:multistatus>`))
	}))
	defer srv.Close()
	dir, entries, err := (&WebDAV{Base: srv.URL}).List(context.Background(), "docs")
	if err != nil {
		t.Fatal(err)
	}
	if dir != "/docs" || len(entries) != 2 || entries[0].Name != "sub dir" || !entries[0].Dir || entries[1].Name != "a.txt" || entries[1].Size != 42 || entries[1].ModTime.IsZero() {
		t.Fatalf("dir %q entries %+v", dir, entries)
	}
}

func TestParseMLSD(t *testing.T) {
	data := []byte("type=cdir;modify=20260101000000; .\r\ntype=pdir; ..\r\ntype=file;size=10;modify=20261101120000; b.txt\r\ntype=dir;modify=20261101120000; A dir\r\n")
	entries := parseMLSD(data)
	if len(entries) != 2 || entries[0].Name != "A dir" || !entries[0].Dir || entries[1].Name != "b.txt" || entries[1].Size != 10 || entries[1].ModTime.IsZero() {
		t.Fatalf("%+v", entries)
	}
}

func TestEndpointURL(t *testing.T) {
	cases := []struct {
		e    Endpoint
		p    string
		want string
	}{
		{Endpoint{Scheme: "sftp", Host: "web1", Port: "22", User: "alice"}, "/etc/app.conf", "sftp://alice@web1/etc/app.conf"},
		{Endpoint{Scheme: "sftp", Host: "web1", Port: "2222"}, "/srv/a b.txt", "sftp://web1:2222/srv/a%20b.txt"},
		{Endpoint{Scheme: "smb", Host: "nas", User: "bob", Domain: "CORP", Share: "docs"}, "dir/a.txt", "smb://CORP;bob@nas/docs/dir/a.txt"},
		{Endpoint{Scheme: "ftp", Host: "ftp.example", User: "u"}, "pub/x", "ftp://u@ftp.example/pub/x"},
		{Endpoint{Scheme: "webdav+https", Host: "dav", Port: "8443"}, "/a.txt", "webdav+https://dav:8443/a.txt"},
		{Endpoint{Scheme: "sftp", Host: "::1", Port: "2200"}, "/x", "sftp://[::1]:2200/x"},
	}
	for _, tc := range cases {
		got := tc.e.URL(tc.p)
		if got != tc.want {
			t.Errorf("%+v %q = %q, want %q", tc.e, tc.p, got, tc.want)
		}
		// 拼出的地址必须能被 ParseURL 还原成同一目标。
		if _, _, err := ParseURL(got); err != nil {
			t.Errorf("%s does not parse: %v", got, err)
		}
	}
	if (Endpoint{Scheme: "smb", Host: "nas"}).URL("") != "smb://nas" {
		t.Fatal("bare smb endpoint")
	}
	if _, err := ParseBrowseURL("smb://nas"); err != nil {
		t.Fatalf("browse url: %v", err)
	}
}

func TestLoadSSHConfig(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config")
	os.WriteFile(p, []byte(`
# comment
Host web1 web1-alias
    HostName 10.0.0.5
    User deploy
Host db
    HostName=db.internal
    Port 2222
    IdentityFile ~/.ssh/db_key
Host *.corp !bad
    User ignored
Match host foo
    User matched
Host *
    User fallback
    IdentityFile ~/.ssh/id_ed25519
`), 0o600)
	hosts, err := LoadSSHConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 3 {
		t.Fatalf("hosts %+v", hosts)
	}
	home, _ := os.UserHomeDir()
	web1, db := hosts[0], hosts[2]
	if web1.Alias != "web1" || web1.HostName != "10.0.0.5" || web1.User != "deploy" || web1.IdentityFile != filepath.Join(home, ".ssh", "id_ed25519") {
		t.Fatalf("web1 %+v", web1)
	}
	if hosts[1].Alias != "web1-alias" || hosts[1].HostName != "10.0.0.5" {
		t.Fatalf("alias %+v", hosts[1])
	}
	if db.HostName != "db.internal" || db.Port != "2222" || db.User != "fallback" || db.IdentityFile != filepath.Join(home, ".ssh", "db_key") {
		t.Fatalf("db %+v", db)
	}
	if missing, err := LoadSSHConfig(filepath.Join(t.TempDir(), "none")); err != nil || missing != nil {
		t.Fatalf("missing config: %v %v", missing, err)
	}
}

func TestSFTPConnReusesOneConnection(t *testing.T) {
	srv := startSSHServer(t, nil)
	os.MkdirAll(filepath.Join(srv.root, "sub"), 0o700)
	os.WriteFile(filepath.Join(srv.root, "sub", "f.txt"), []byte("body"), 0o600)
	store := &SFTP{Addr: srv.addr, User: "u", Pass: "p", KnownHosts: []string{knownHostsFile(t, srv.addr, srv.hostKey.PublicKey())}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, err := Connect(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	root, entries, err := conn.List(ctx, "")
	if err != nil || len(entries) != 1 || !entries[0].Dir {
		t.Fatalf("root %q %+v %v", root, entries, err)
	}
	sub, files, err := conn.List(ctx, "sub") // 相对服务端工作目录，避免依赖平台绝对路径写法
	if err != nil || len(files) != 1 || files[0].Name != "f.txt" {
		t.Fatalf("sub %q %+v %v", sub, files, err)
	}
	data, err := conn.Read(ctx, sub+"/f.txt")
	if err != nil || string(data) != "body" {
		t.Fatalf("read %q %v", data, err)
	}
	// 取消后连接被关闭，后续操作立即失败而不是挂起。
	cancel()
	if _, _, err = conn.List(context.Background(), root); err == nil {
		t.Fatal("connection still usable after its context was canceled")
	}
}

func TestDirURLHelpers(t *testing.T) {
	if s, p, err := ParseDirURL("sftp://h/"); err != nil || p != "/" || s == nil {
		t.Fatalf("sftp root: %q %v", p, err)
	}
	if _, p, err := ParseDirURL("sftp://h"); err != nil || p != "" {
		t.Fatalf("sftp home: %q %v", p, err)
	}
	if _, _, err := ParseDirURL("smb://nas"); err == nil {
		t.Fatal("smb dir without share accepted")
	}
	if got := JoinPath("sftp://u@h/srv/app", "conf/a b.txt"); got != "sftp://u@h/srv/app/conf/a%20b.txt" {
		t.Fatalf("join %q", got)
	}
	if got := JoinPath("smb://nas/docs", "x.txt"); got != "smb://nas/docs/x.txt" {
		t.Fatalf("join smb %q", got)
	}
	if got := (Endpoint{Scheme: "sftp", Host: "h"}).URL("/"); got != "sftp://h/" {
		t.Fatalf("root endpoint %q", got)
	}
}
