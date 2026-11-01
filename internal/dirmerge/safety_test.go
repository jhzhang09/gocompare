package dirmerge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteRejectsOutputSymlink(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "out")
	outside := filepath.Join(root, "outside")
	if err := os.Mkdir(dest, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "keep"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dest, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	e := Write(context.Background(), dest, []Decision{{Path: "linked/keep", Kind: KindLeft, Data: []byte("overwritten")}})
	b, err := os.ReadFile(filepath.Join(outside, "keep"))
	if err != nil {
		t.Fatal(err)
	}
	if e == nil || string(b) != "original" {
		t.Fatal("wrote outside dest through existing symlink")
	}
}

func TestWriteRequiresEmptyDestination(t *testing.T) {
	dest := t.TempDir()
	writeTree(t, dest, map[string]string{"keep": "original"})
	if err := Write(context.Background(), dest, []Decision{{Path: "keep", Kind: KindLeft, Data: []byte("replacement")}}); err == nil {
		t.Fatal("nonempty destination accepted")
	}
	data, err := os.ReadFile(filepath.Join(dest, "keep"))
	if err != nil || string(data) != "original" {
		t.Fatalf("existing file changed: %q %v", data, err)
	}
}
