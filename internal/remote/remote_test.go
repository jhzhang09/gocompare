package remote

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseURL(t *testing.T) {
	cases := []struct {
		url      string
		wantType string
		wantPath string
	}{
		{"ftp://user:pass@localhost:21/dir/file.txt", "*remote.FTP", "dir/file.txt"},
		{"ftps://user:pass@localhost:990/file.txt", "*remote.FTP", "file.txt"},
		{"webdav://localhost:8080/remote.php", "*remote.WebDAV", "remote.php"},
		{"s3://key:sec@s3.amazonaws.com/bucket/obj", "*remote.S3", "bucket/obj"},
		{"dropbox://my-token@/folder/f.txt", "*remote.Dropbox", "folder/f.txt"},
		{"onedrive://token@/me/drive", "*remote.OneDrive", "me/drive"},
		{"svn://localhost/repo/trunk", "*remote.SVN", "repo/trunk"},
	}

	for _, tc := range cases {
		store, path, err := ParseURL(tc.url)
		if err != nil {
			t.Fatalf("ParseURL(%q) err: %v", tc.url, err)
		}
		if path != tc.wantPath {
			t.Errorf("ParseURL(%q) path = %q, want %q", tc.url, path, tc.wantPath)
		}
		if store == nil {
			t.Errorf("ParseURL(%q) returned nil store", tc.url)
		}
	}

	if _, _, err := ParseURL("invalid://foo"); err == nil {
		t.Error("expected error for invalid scheme")
	}
}

func TestFTPSURLServerName(t *testing.T) {
	s, _, err := ParseURL("ftps://user:pass@localhost:21/file")
	if err != nil {
		t.Fatal(err)
	}
	if s.(*FTP).TLS.ServerName != "localhost" || s.(*FTP).TLS.InsecureSkipVerify {
		t.Fatal("missing verified TLS hostname")
	}
}

func TestHTTPSSchemes(t *testing.T) {
	cases := map[string]string{
		"webdav+https://u:p@dav.example/a.txt": "https://dav.example",
		"s3+https://k:s@s3.example/b/k":        "https://s3.example",
		"svn+https://svn.example/repo/f":       "https://svn.example",
		"webdav://dav.example/a.txt":           "http://dav.example",
	}
	for raw, want := range cases {
		store, _, err := ParseURL(raw)
		if err != nil {
			t.Fatal(err)
		}
		var base string
		switch s := store.(type) {
		case *WebDAV:
			base = s.Base
		case *S3:
			base = s.Base
		case *SVN:
			base = s.Base
		}
		if base != want {
			t.Errorf("%s base = %q, want %q", raw, base, want)
		}
	}
}

func TestDropboxURLUsesContentHost(t *testing.T) {
	store, _, err := ParseURL("dropbox://tok@/f.txt")
	if err != nil {
		t.Fatal(err)
	}
	d := store.(*Dropbox)
	if d.ContentBase != "https://content.dropboxapi.com" || d.Base != "https://api.dropboxapi.com" {
		t.Fatalf("dropbox hosts = %q / %q", d.ContentBase, d.Base)
	}
}

func TestDropboxArgIsASCII(t *testing.T) {
	var header string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Get("Dropbox-API-Arg")
		w.Write([]byte("x"))
	}))
	defer srv.Close()
	if _, err := (&Dropbox{Base: srv.URL, Token: "t"}).Read(context.Background(), "目录/文件😀.txt"); err != nil {
		t.Fatal(err)
	}
	for _, r := range header {
		if r >= 0x80 {
			t.Fatalf("non-ASCII header %q", header)
		}
	}
	var arg struct{ Path string }
	if err := json.Unmarshal([]byte(header), &arg); err != nil || arg.Path != "/目录/文件😀.txt" {
		t.Fatalf("decoded arg %q %v", arg.Path, err)
	}
}

func TestOneDriveKeepsNestedPath(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.EscapedPath()
	}))
	defer srv.Close()
	if _, err := (&OneDrive{Base: srv.URL, Token: "t"}).Read(context.Background(), "dir/sub dir/f.txt"); err != nil {
		t.Fatal(err)
	}
	if got != "/v1.0/me/drive/root:/dir/sub%20dir/f.txt:/content" {
		t.Fatalf("path = %q", got)
	}
}

func TestS3SignsAWSEncodedPath(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.RequestURI
		io.Copy(io.Discard, r.Body)
	}))
	defer srv.Close()
	if _, err := (&S3{Base: srv.URL + "/bucket", AccessKey: "a", SecretKey: "s"}).Read(context.Background(), "dir/a b!(c)*'.txt"); err != nil {
		t.Fatal(err)
	}
	if got != "/bucket/dir/a%20b%21%28c%29%2A%27.txt" {
		t.Fatalf("request uri = %q", got)
	}
}

func TestWebDAVBasicAuthAndEscaping(t *testing.T) {
	var user, pass, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ = r.BasicAuth()
		path = r.URL.Path
	}))
	defer srv.Close()
	if _, err := (&WebDAV{Base: srv.URL, User: "u", Pass: "p"}).Read(context.Background(), "a #1?.txt"); err != nil {
		t.Fatal(err)
	}
	if user != "u" || pass != "p" || path != "/a #1?.txt" {
		t.Fatalf("auth %q/%q path %q", user, pass, path)
	}
}

func TestFTPRejectsCRLF(t *testing.T) {
	store, path, err := ParseURL("ftp://localhost:1/a%0D%0ADELE%20b")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Read(context.Background(), path); err == nil || !strings.Contains(err.Error(), "CR or LF") {
		t.Fatalf("CRLF path must be rejected before dialing: %v", err)
	}
}

func TestParsePASVReturnsPortOnly(t *testing.T) {
	port, err := parsePASV("227 Entering Passive Mode (10,0,0,9,4,1)")
	if err != nil || port != "1025" {
		t.Fatalf("port %q %v", port, err)
	}
	if _, err = parsePASV("227 (1,2,3,4,999,1)"); err == nil {
		t.Fatal("out-of-range PASV octet accepted")
	}
}

func TestParseSFTPURL(t *testing.T) {
	cases := []struct {
		raw, addr, user, pass, path string
	}{
		{"sftp://alice:pw@example.com/srv/a.txt", "example.com:22", "alice", "pw", "/srv/a.txt"},
		{"sftp://alice@example.com:2222/~/notes/a.txt", "example.com:2222", "alice", "", "notes/a.txt"},
		{"sftp://[::1]:2200/tmp/x", "[::1]:2200", "", "", "/tmp/x"},
	}
	for _, tc := range cases {
		store, path, err := ParseURL(tc.raw)
		if err != nil {
			t.Fatal(err)
		}
		s := store.(*SFTP)
		if s.Addr != tc.addr || s.User != tc.user || s.Pass != tc.pass || path != tc.path {
			t.Errorf("%s => %+v %q", tc.raw, s, path)
		}
	}
	if _, _, err := ParseURL("sftp://example.com/"); err == nil {
		t.Fatal("sftp URL without a path accepted")
	}
}

func TestParseSMBURL(t *testing.T) {
	cases := []struct {
		raw, addr, share, domain, user, pass, path string
	}{
		{"smb://bob:pw@nas/docs/dir/a.txt", "nas:445", "docs", "", "bob", "pw", "dir/a.txt"},
		{"smb://CORP;bob:pw@nas:1445/docs/a.txt", "nas:1445", "docs", "CORP", "bob", "pw", "a.txt"},
		{"smb://CORP%5Cbob:pw@nas/docs/a.txt", "nas:445", "docs", "CORP", "bob", "pw", "a.txt"},
		{"smb://nas/public/a.txt", "nas:445", "public", "", "", "", "a.txt"},
	}
	for _, tc := range cases {
		store, path, err := ParseURL(tc.raw)
		if err != nil {
			t.Fatal(err)
		}
		s := store.(*SMB)
		if s.Addr != tc.addr || s.Share != tc.share || s.Domain != tc.domain || s.User != tc.user || s.Pass != tc.pass || path != tc.path {
			t.Errorf("%s => %+v %q", tc.raw, s, path)
		}
	}
	for _, raw := range []string{"smb://nas/", "smb://nas/share", "smb://nas/share/"} {
		if _, _, err := ParseURL(raw); err == nil {
			t.Errorf("%s accepted without share and path", raw)
		}
	}
}

func TestSMBPath(t *testing.T) {
	if got := smbPath("/dir/sub/a.txt"); got != `dir\sub\a.txt` {
		t.Fatalf("smbPath = %q", got)
	}
}
