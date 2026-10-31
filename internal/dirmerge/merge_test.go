package dirmerge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, text := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func byPath(items []Decision) map[string]Decision {
	out := map[string]Decision{}
	for _, item := range items {
		out[item.Path] = item
	}
	return out
}

func TestTwoAndThreeWayDirectoryMerge(t *testing.T) {
	ctx := context.Background()
	left, right := t.TempDir(), t.TempDir()
	writeTree(t, left, map[string]string{"only.txt": "L\n", "both.txt": "a\n", "clash.txt": "left\n"})
	writeTree(t, right, map[string]string{"both.txt": "a\n", "clash.txt": "right\n", "new.txt": "R\n"})
	two, err := Two(ctx, left, right)
	if err != nil {
		t.Fatal(err)
	}
	got := byPath(two)
	if got["only.txt"].Kind != KindLeft || string(got["only.txt"].Data) != "L\n" {
		t.Fatalf("only: %+v", got["only.txt"])
	}
	if got["new.txt"].Kind != KindRight || string(got["new.txt"].Data) != "R\n" {
		t.Fatalf("new: %+v", got["new.txt"])
	}
	if got["clash.txt"].Kind != KindConflict || !bytes.Contains(got["clash.txt"].Data, []byte("<<<<<<< left")) {
		t.Fatalf("two-way conflict: %s", got["clash.txt"].Data)
	}
	base := t.TempDir()
	l := t.TempDir()
	r := t.TempDir()
	writeTree(t, base, map[string]string{
		"clean.txt": "a\nb\nc\n",
		"war.txt":   "a\nb\nc\n",
		"gone.txt":  "a\n",
		"edit.txt":  "a\n",
	})
	writeTree(t, l, map[string]string{
		"clean.txt": "L\nb\nc\n",
		"war.txt":   "a\nLEFT\nc\n",
	})
	writeTree(t, r, map[string]string{
		"clean.txt": "a\nb\nR\n",
		"war.txt":   "a\nRIGHT\nc\n",
		"gone.txt":  "a\n",
		"edit.txt":  "other\n",
	})
	three, err := Three(ctx, base, l, r)
	if err != nil {
		t.Fatal(err)
	}
	got = byPath(three)
	if got["clean.txt"].Kind != KindMerged || string(got["clean.txt"].Data) != "L\nb\nR\n" {
		t.Fatalf("clean merge: %s %s", got["clean.txt"].Kind, got["clean.txt"].Data)
	}
	if got["war.txt"].Kind != KindConflict || !bytes.Contains(got["war.txt"].Data, []byte("<<<<<<< left")) {
		t.Fatalf("text conflict: %s", got["war.txt"].Data)
	}
	if got["gone.txt"].Kind != KindDelete {
		t.Fatalf("delete: %+v", got["gone.txt"])
	}
	if got["edit.txt"].Kind != KindConflict {
		t.Fatalf("delete/modify: %+v", got["edit.txt"])
	}
	dest := t.TempDir()
	if err = Write(ctx, dest, three); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "clean.txt"))
	if err != nil || string(data) != "L\nb\nR\n" {
		t.Fatalf("materialized: %q %v", data, err)
	}
	if _, err = os.Stat(filepath.Join(dest, "gone.txt")); !os.IsNotExist(err) {
		t.Fatal("deleted file was created")
	}
}
