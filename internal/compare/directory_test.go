package compare

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryComparison(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	write := func(root, path, text string) {
		t.Helper()
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, root := range []string{left, right} {
		write(root, "nested/equal", "same")
		write(root, "nested/change", root)
		os.Mkdir(filepath.Join(root, "empty"), 0o700)
	}
	write(left, "only-left", "x")
	write(right, "only-right", "y")
	write(left, "type", "file")
	os.Mkdir(filepath.Join(right, "type"), 0o700)
	rows, err := Directories(context.Background(), left, right)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Status{}
	for _, row := range rows {
		got[row.Path] = row.Status
	}
	for path, status := range map[string]Status{"nested": Different, "nested/equal": Same, "nested/change": Different, "empty": Same, "only-left": LeftOnly, "only-right": RightOnly, "type": TypeChanged} {
		if got[path] != status {
			t.Fatalf("%s: %s want %s", path, got[path], status)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = Directories(ctx, left, right); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestDirectorySymlinkNoTraversal(t *testing.T) {
	left, right, outside := t.TempDir(), t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(outside, "secret"), []byte("x"), 0o600)
	if err := os.Symlink(outside, filepath.Join(left, "link")); err != nil {
		t.Skipf("symlink not available: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(right, "link")); err != nil {
		t.Fatal(err)
	}
	rows, err := Directories(context.Background(), left, right)
	if err != nil || len(rows) != 1 || rows[0].Status != Same {
		t.Fatalf("unexpected traversal: %+v %v", rows, err)
	}
}
