package compare

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanProgressFilterAndAttributes(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	write := func(root, rel, text string, mode os.FileMode) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), mode); err != nil {
			t.Fatal(err)
		}
	}
	write(left, "nested/a.txt", "same", 0o600)
	write(right, "nested/a.txt", "same", 0o640)
	write(left, "nested/b.bin", "x", 0o600)
	write(right, "nested/b.bin", "y", 0o600)
	past := time.Unix(1_700_000_000, 0)
	future := past.Add(time.Hour)
	if err := os.Chtimes(filepath.Join(left, "nested", "a.txt"), past, past); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(right, "nested", "a.txt"), future, future); err != nil {
		t.Fatal(err)
	}
	var seen []string
	rows, err := CompareDirectories(context.Background(), left, right, ScanOptions{
		Progress: func(rel string) error {
			seen = append(seen, rel)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	parent, child := -1, -1
	for i, rel := range seen {
		if rel == "nested" {
			parent = i
		}
		if rel == "nested/a.txt" {
			child = i
		}
	}
	if parent < 0 || child < 0 || parent > child {
		t.Fatalf("progress order: %v", seen)
	}
	status := map[string]Status{}
	for _, row := range rows {
		status[row.Path] = row.Status
	}
	if status["nested/a.txt"] != Same {
		t.Fatalf("mode and time are ignored by default: %s", status["nested/a.txt"])
	}
	rows, err = CompareDirectories(context.Background(), left, right, ScanOptions{CompareMode: true})
	if err != nil {
		t.Fatal(err)
	}
	status = map[string]Status{}
	for _, row := range rows {
		status[row.Path] = row.Status
	}
	if status["nested/a.txt"] != Different {
		t.Fatal("mode rule did not see the permission change")
	}
	rows, err = CompareDirectories(context.Background(), left, right, ScanOptions{CompareTime: true, Filter: "nested/*.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Path != "nested/a.txt" || rows[0].Status != Different {
		t.Fatalf("filter: %+v", rows)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var prefix []string
	_, err = CompareDirectories(ctx, left, right, ScanOptions{Progress: func(rel string) error {
		prefix = append(prefix, rel)
		cancel()
		return nil
	}})
	if !errors.Is(err, context.Canceled) || len(prefix) == 0 {
		t.Fatalf("cancel prefix: %v %v", prefix, err)
	}
}

func TestLinkFollowRejectsEscapeAndCycle(t *testing.T) {
	left, right, outside := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("hidden"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(left, "out")); err != nil {
		t.Skipf("symlink not available: %v", err)
	}
	if err := os.Mkdir(filepath.Join(right, "out"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(right, "out", "secret"), []byte("visible"), 0o600); err != nil {
		t.Fatal(err)
	}
	rows, err := CompareDirectories(context.Background(), left, right, ScanOptions{Links: LinkFollow})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Path == "out/secret" && row.Left != nil {
			t.Fatalf("followed an escaping symlink: %+v", rows)
		}
		if row.Path == "out" && (row.Left == nil || row.Left.Error == "") {
			t.Fatalf("escape was not reported: %+v", row)
		}
	}
	cycle := t.TempDir()
	other := t.TempDir()
	if err := os.Symlink(cycle, filepath.Join(cycle, "loop")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(other, "loop"), 0o700); err != nil {
		t.Fatal(err)
	}
	rows, err = CompareDirectories(context.Background(), cycle, other, ScanOptions{Links: LinkFollow})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range rows {
		if row.Path == "loop" && row.Left != nil && row.Left.Error != "" {
			found = true
		}
		if len(row.Path) > 12 {
			t.Fatalf("cycle expanded: %s", row.Path)
		}
	}
	if !found {
		t.Fatalf("cycle not reported: %+v", rows)
	}
	inside, plain := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(inside, "file"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(inside, "file"), filepath.Join(inside, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plain, "file"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plain, "link"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	rows, err = Directories(context.Background(), inside, plain)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Path == "link" && row.Status != TypeChanged {
			t.Fatalf("default policy followed a symlink: %+v", row)
		}
	}
	rows, err = CompareDirectories(context.Background(), inside, plain, ScanOptions{Links: LinkFollow})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Path == "link" && row.Status != Same {
			t.Fatalf("followed file symlink: %+v", row)
		}
	}
}
