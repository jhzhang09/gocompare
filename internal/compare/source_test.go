package compare

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"gocompare/internal/remote"
)

// memConn 是内存中的远程目录树，键为绝对路径；记录读取次数以确认大小不同时不读内容。
type memConn struct {
	files map[string]string
	reads int
}

func (m *memConn) List(_ context.Context, dir string) (string, []remote.Entry, error) {
	if dir == "" {
		dir = "/srv"
	}
	prefix := strings.TrimSuffix(dir, "/") + "/"
	seen := map[string]bool{}
	var out []remote.Entry
	found := false
	for p, body := range m.files {
		if !strings.HasPrefix(p, prefix) {
			continue
		}
		found = true
		rest := strings.TrimPrefix(p, prefix)
		name, _, nested := strings.Cut(rest, "/")
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, remote.Entry{Name: name, Dir: nested, Size: int64(len(body)), ModTime: time.Unix(1, 0)})
	}
	if !found {
		return "", nil, os.ErrNotExist
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return dir, out, nil
}

func (m *memConn) Read(_ context.Context, p string) ([]byte, error) {
	m.reads++
	body, ok := m.files[p]
	if !ok {
		return nil, os.ErrNotExist
	}
	return []byte(body), nil
}

func (m *memConn) Readlink(context.Context, string) (string, error) {
	return "", errors.New("no links")
}

func (m *memConn) Close() error { return nil }

func TestCompareLocalWithRemote(t *testing.T) {
	local := t.TempDir()
	os.MkdirAll(filepath.Join(local, "conf"), 0o700)
	os.WriteFile(filepath.Join(local, "same.txt"), []byte("same"), 0o600)
	os.WriteFile(filepath.Join(local, "edit.txt"), []byte("aaaa"), 0o600)
	os.WriteFile(filepath.Join(local, "size.txt"), []byte("short"), 0o600)
	os.WriteFile(filepath.Join(local, "conf", "app.conf"), []byte("x=1"), 0o600)
	os.WriteFile(filepath.Join(local, "local-only.txt"), []byte("l"), 0o600)
	conn := &memConn{files: map[string]string{
		"/srv/same.txt":        "same",
		"/srv/edit.txt":        "bbbb",
		"/srv/size.txt":        "much longer",
		"/srv/conf/app.conf":   "x=1",
		"/srv/remote-only.txt": "r",
	}}
	rows, err := CompareSources(context.Background(), LocalSource(local), RemoteSource(conn, ""), ScanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Status{}
	for _, r := range rows {
		got[r.Path] = r.Status
	}
	want := map[string]Status{
		"same.txt": Same, "edit.txt": Different, "size.txt": Different, "conf": Same, "conf/app.conf": Same,
		"local-only.txt": LeftOnly, "remote-only.txt": RightOnly,
	}
	for p, st := range want {
		if got[p] != st {
			t.Errorf("%s = %s, want %s", p, got[p], st)
		}
	}
	// 只有大小相同的三个文件需要读远程内容；大小不同直接判定。
	if conn.reads != 3 {
		t.Fatalf("remote reads %d, want 3", conn.reads)
	}
}

func TestRemoteSourceFilterAndMissingRoot(t *testing.T) {
	conn := &memConn{files: map[string]string{"/srv/a/x.go": "1", "/srv/a/y.txt": "2", "/srv/b.go": "3"}}
	got, err := RemoteSource(conn, "/srv").Scan(context.Background(), ScanOptions{Filter: "*.go", KeepParents: true})
	if err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if strings.Join(keys, ",") != "a,a/x.go,b.go" || !got["a"].Directory {
		t.Fatalf("filtered %v", keys)
	}
	if _, err = RemoteSource(conn, "/nope").Scan(context.Background(), ScanOptions{}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing root: %v", err)
	}
}
