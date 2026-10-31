package compare

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEqualFiles(t *testing.T) {
	for _, pair := range [][2]string{{"", ""}, {"same", "same"}, {"same", "different"}, {"a\r\n", "a\n"}, {strings.Repeat("a", 65536), strings.Repeat("a", 65536)}, {strings.Repeat("a", 65536), strings.Repeat("a", 65536) + "b"}} {
		dir := t.TempDir()
		a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
		os.WriteFile(a, []byte(pair[0]), 0o600)
		os.WriteFile(b, []byte(pair[1]), 0o600)
		got, err := EqualFiles(context.Background(), a, b)
		if err != nil || got != (pair[0] == pair[1]) {
			t.Fatalf("equal=%v err=%v", got, err)
		}
	}
}
func TestEqualFilesErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := EqualFiles(context.Background(), dir, dir); err == nil {
		t.Fatal("accepted directory")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := EqualFiles(ctx, "missing", "missing"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
