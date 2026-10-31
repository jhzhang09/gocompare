package operations

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMirrorPreviewIsReadOnly(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(left, "new"), []byte("x"), 0o600)
	os.MkdirAll(filepath.Join(right, "extra", "nested"), 0o700)
	os.WriteFile(filepath.Join(right, "extra", "nested", "old"), []byte("y"), 0o600)
	p, err := Preview(context.Background(), left, right, MirrorRight)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Actions) != 4 || p.Actions[0].Kind != "copy" || p.Actions[3].Path != "extra" {
		t.Fatalf("bad order: %+v", p.Actions)
	}
	if _, err = os.Stat(filepath.Join(right, "new")); !os.IsNotExist(err) {
		t.Fatal("preview copied file")
	}
	if _, err = os.Stat(filepath.Join(right, "extra", "nested", "old")); err != nil {
		t.Fatal("preview deleted file")
	}
}
func TestUpdateRespectsTimestamps(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	l, r := filepath.Join(left, "file"), filepath.Join(right, "file")
	os.WriteFile(l, []byte("left"), 0o600)
	os.WriteFile(r, []byte("right"), 0o600)
	older := time.Unix(1000, 0)
	newer := time.Unix(2000, 0)
	os.Chtimes(l, older, older)
	os.Chtimes(r, newer, newer)
	p, err := Preview(context.Background(), left, right, UpdateRight)
	if err != nil || len(p.Actions) != 0 {
		t.Fatalf("older overwrites newer: %+v %v", p, err)
	}
	p, err = Preview(context.Background(), left, right, UpdateBoth)
	if err != nil || len(p.Actions) != 1 || p.Actions[0].From != p.RightResolved {
		t.Fatalf("wrong direction: %+v %v", p, err)
	}
	os.Chtimes(l, newer, newer)
	if _, err = Preview(context.Background(), left, right, UpdateBoth); err == nil {
		t.Fatal("timestamp conflict silently resolved")
	}
}
func TestRejectNestedRoots(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	os.Mkdir(child, 0o700)
	if _, err := Preview(context.Background(), root, child, MirrorRight); err == nil {
		t.Fatal("nested roots accepted")
	}
}
