package document

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSavePreservesFormatAndRejectsExternalChanges(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "sample.txt")
	original := []byte("\xef\xbb\xbf你好\r\nworld\r\n")
	if err := os.WriteFile(path, original, 0o640); err != nil {
		t.Fatal(err)
	}
	f, err := Read(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if f.Text != "你好\nworld\n" || !f.BOM || !f.CRLF {
		t.Fatalf("unexpected document: %+v", f)
	}
	f, err = f.Save(ctx, "你好\nchanged\n")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "\xef\xbb\xbf你好\r\nchanged\r\n" {
		t.Fatalf("format lost: %q", data)
	}
	if err = os.WriteFile(path, []byte("external"), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err = f.Save(ctx, "overwrite"); !errors.Is(err, ErrChanged) {
		t.Fatalf("want external change error, got %v", err)
	}
	data, _ = os.ReadFile(path)
	if string(data) != "external" {
		t.Fatal("external content overwritten")
	}
}
func TestRejectUnsupportedInput(t *testing.T) {
	for _, text := range []string{"a\r\nb\n", "a\rb", "a\x00b", "\xff\xfe"} {
		path := filepath.Join(t.TempDir(), "input")
		os.WriteFile(path, []byte(text), 0o600)
		if _, err := Read(context.Background(), path); err == nil {
			t.Fatalf("accepted %q", text)
		}
	}
}
func TestCancellationAndNoFinalNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input")
	os.WriteFile(path, []byte("last"), 0o600)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Read(ctx, path); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	f, err := Read(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Save(ctx, "lost"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err = f.Save(context.Background(), "changed"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "changed" {
		t.Fatalf("unexpected final newline: %q", data)
	}
}
func TestRejectReplacedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input")
	os.WriteFile(path, []byte("same"), 0o600)
	f, err := Read(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(filepath.Dir(path), "replacement")
	os.WriteFile(replacement, []byte("same"), 0o600)
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	if _, err = f.Save(context.Background(), "overwrite"); !errors.Is(err, ErrChanged) {
		t.Fatalf("replacement not detected: %v", err)
	}
}

func TestSaveAsAndBackup(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "source")
	original := "\xef\xbb\xbf原文\r\n"
	os.WriteFile(path, []byte(original), 0o600)
	f, err := Read(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	backup, err := f.Backup(ctx, filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(backup)
	if err != nil || string(data) != original {
		t.Fatalf("backup: %q %v", data, err)
	}
	target := filepath.Join(dir, "new")
	saved, err := f.SaveAs(ctx, target, "修改\n")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Path != target || !saved.BOM || !saved.CRLF {
		t.Fatal("format lost")
	}
	data, _ = os.ReadFile(target)
	if string(data) != "\xef\xbb\xbf修改\r\n" {
		t.Fatalf("save as: %q", data)
	}
	if _, err = f.SaveAs(ctx, target, "overwrite"); !errors.Is(err, os.ErrExist) {
		t.Fatalf("overwrite allowed: %v", err)
	}
	data, _ = os.ReadFile(path)
	if string(data) != original {
		t.Fatal("source modified")
	}
	os.WriteFile(path, []byte("external"), 0o600)
	if _, err = f.Backup(ctx, filepath.Join(dir, "backups")); !errors.Is(err, ErrChanged) {
		t.Fatalf("stale backup allowed: %v", err)
	}
}
