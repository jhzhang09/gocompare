package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "comparison.gocompare")
	s := Session{Version: 1, Left: filepath.Join(dir, "a.txt"), Right: filepath.Join(dir, "b.txt"), Wrap: true}
	if err := Write(path, s); err != nil {
		t.Fatal(err)
	}
	got, err := Read(path)
	if err != nil || got != s {
		t.Fatalf("%+v %v", got, err)
	}
	if err = os.WriteFile(path, []byte(`{"version":1,"left":"a","right":"b"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = Read(path)
	if err != nil || got.Left != filepath.Join(dir, "a") {
		t.Fatalf("relative: %+v %v", got, err)
	}
}
func TestSessionRejectInvalid(t *testing.T) {
	for _, data := range []string{`{`, `{"version":2,"left":"a","right":"b"}`, `{"version":1,"left":"a"}`, `{"version":1,"left":"a","right":"b"} {}`, `{"version":1,"left":"a","right":"b"}` + strings.Repeat(" ", 1<<20)} {
		path := filepath.Join(t.TempDir(), "bad.gocompare")
		os.WriteFile(path, []byte(data), 0o600)
		if _, err := Read(path); err == nil {
			t.Fatalf("accepted invalid session of length %d", len(data))
		}
	}
}

// 相对的基准和输出路径按写入时的工作目录解析，读回后不能改指会话目录下的同名文件。
func TestWriteResolvesOptionalPaths(t *testing.T) {
	dir := t.TempDir()
	sessions := filepath.Join(dir, "sessions")
	if err := os.Mkdir(sessions, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	path := filepath.Join(sessions, "s.gocompare")
	if err := Write(path, Session{Kind: "merge", Version: 1, Left: "l.txt", Right: "r.txt", Base: "base.txt", Output: "out.txt"}); err != nil {
		t.Fatal(err)
	}
	s, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	if s.Base != filepath.Join(wd, "base.txt") || s.Output != filepath.Join(wd, "out.txt") {
		t.Fatalf("base %s output %s, want under %s", s.Base, s.Output, wd)
	}
}
