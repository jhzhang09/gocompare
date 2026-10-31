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
