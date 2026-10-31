package operations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestExecuteMirrorWithBackups(t *testing.T) {
	ctx := context.Background()
	left, right := t.TempDir(), t.TempDir()
	os.Mkdir(filepath.Join(left, "new-dir"), 0o700)
	os.WriteFile(filepath.Join(left, "new-dir", "a"), []byte("new"), 0o600)
	os.WriteFile(filepath.Join(right, "old"), []byte("old"), 0o600)
	p, err := Preview(ctx, left, right, MirrorRight)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Execute(ctx, p, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Completed) != 3 || report.Pending != nil {
		t.Fatalf("bad report: %+v", report)
	}
	data, err := os.ReadFile(filepath.Join(right, "new-dir", "a"))
	if err != nil || string(data) != "new" {
		t.Fatalf("copy: %q %v", data, err)
	}
	if _, err = os.Stat(filepath.Join(right, "old")); !os.IsNotExist(err) {
		t.Fatal("orphan not deleted")
	}
	found := false
	for _, record := range report.Completed {
		if record.Action.Kind == "delete" {
			data, err = os.ReadFile(record.Backup)
			if err != nil || string(data) != "old" {
				t.Fatalf("backup: %q %v", data, err)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("delete missing from journal")
	}
	if _, err = os.Stat(report.Journal); err != nil {
		t.Fatal(err)
	}
}
func TestExecuteRejectsStalePlanBeforeChanges(t *testing.T) {
	ctx := context.Background()
	left, right := t.TempDir(), t.TempDir()
	for _, name := range []string{"a", "b"} {
		os.WriteFile(filepath.Join(left, name), []byte("original"), 0o600)
	}
	p, err := Preview(ctx, left, right, MirrorRight)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(left, "b"), []byte("changed"), 0o600)
	report, err := Execute(ctx, p, t.TempDir())
	if !errors.Is(err, ErrChanged) || len(report.Completed) != 0 {
		t.Fatalf("stale plan: %+v %v", report, err)
	}
	if _, err = os.Stat(filepath.Join(right, "a")); !os.IsNotExist(err) {
		t.Fatal("preflight wrote first file")
	}
}
func TestExecuteRejectsSymlinkAndCancellation(t *testing.T) {
	ctx := context.Background()
	left, right, outside := t.TempDir(), t.TempDir(), t.TempDir()
	os.Mkdir(filepath.Join(left, "nested"), 0o700)
	os.WriteFile(filepath.Join(left, "nested", "file"), []byte("x"), 0o600)
	os.Mkdir(filepath.Join(right, "nested"), 0o700)
	p, err := Preview(ctx, left, right, MirrorRight)
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = Execute(canceled, p, t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	os.Remove(filepath.Join(right, "nested"))
	if err = os.Symlink(outside, filepath.Join(right, "nested")); err != nil {
		t.Skipf("symlink not available: %v", err)
	}
	if _, err = Execute(ctx, p, t.TempDir()); err == nil {
		t.Fatal("followed replaced parent")
	}
	if _, err = os.Stat(filepath.Join(outside, "file")); !os.IsNotExist(err) {
		t.Fatal("wrote outside root")
	}
}

func TestRejectBackupInsideRoot(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(left, "a"), []byte("a"), 0o600)
	p, err := Preview(context.Background(), left, right, MirrorRight)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Execute(context.Background(), p, filepath.Join(left, "backups")); err == nil {
		t.Fatal("backup inside root allowed")
	}
	if _, err = os.Stat(filepath.Join(left, "backups")); !os.IsNotExist(err) {
		t.Fatal("invalid backup path was created")
	}
}
