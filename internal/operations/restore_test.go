package operations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type cancelAfterCreate struct {
	context.Context
	path string
}

func (c cancelAfterCreate) Err() error {
	if err := c.Context.Err(); err != nil {
		return err
	}
	if _, err := os.Stat(c.path); err == nil {
		return context.Canceled
	}
	return nil
}

func TestExecuteRollsBackCompletedActions(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(left, "a"), []byte("A"), 0o600)
	os.WriteFile(filepath.Join(left, "b"), []byte("B"), 0o600)
	plan, err := Preview(context.Background(), left, right, MirrorRight)
	if err != nil {
		t.Fatal(err)
	}
	ctx := cancelAfterCreate{Context: context.Background(), path: filepath.Join(right, "a")}
	report, err := Execute(ctx, plan, t.TempDir())
	if !errors.Is(err, context.Canceled) || !report.RolledBack || len(report.Completed) != 0 {
		t.Fatalf("rollback: %+v %v", report, err)
	}
	if _, statErr := os.Stat(filepath.Join(right, "a")); !os.IsNotExist(statErr) {
		t.Fatal("rolled back file still exists")
	}
	if _, statErr := os.Stat(filepath.Join(right, "b")); !os.IsNotExist(statErr) {
		t.Fatal("later file was copied")
	}
}

func TestRestoreUndoMirror(t *testing.T) {
	ctx := context.Background()
	left, right := t.TempDir(), t.TempDir()
	os.Mkdir(filepath.Join(left, "new-dir"), 0o700)
	os.WriteFile(filepath.Join(left, "new-dir", "a"), []byte("new"), 0o600)
	os.WriteFile(filepath.Join(right, "old"), []byte("old"), 0o600)
	plan, err := Preview(ctx, left, right, MirrorRight)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Execute(ctx, plan, t.TempDir())
	if err != nil || report.RolledBack {
		t.Fatalf("execute: %+v %v", report, err)
	}
	restored, err := Restore(ctx, report.Journal)
	if err != nil || !restored.RolledBack || len(restored.Completed) != 0 {
		t.Fatalf("restore: %+v %v", restored, err)
	}
	data, err := os.ReadFile(filepath.Join(right, "old"))
	if err != nil || string(data) != "old" {
		t.Fatalf("old: %q %v", data, err)
	}
	if _, err = os.Stat(filepath.Join(right, "new-dir")); !os.IsNotExist(err) {
		t.Fatal("copied directory remains")
	}
	if _, err = Restore(ctx, report.Journal); err == nil {
		t.Fatal("second restore accepted")
	}
}

func TestRestoreRefusesRecreatedPath(t *testing.T) {
	ctx := context.Background()
	left, right := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(left, "a"), []byte("new"), 0o600)
	os.WriteFile(filepath.Join(right, "old"), []byte("old"), 0o600)
	plan, err := Preview(ctx, left, right, MirrorRight)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Execute(ctx, plan, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(right, "old"), []byte("recreated"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = Restore(ctx, report.Journal); err == nil {
		t.Fatal("restore overwrote recreated file")
	}
	data, err := os.ReadFile(filepath.Join(right, "old"))
	if err != nil || string(data) != "recreated" {
		t.Fatalf("recreated: %q %v", data, err)
	}
}

// 同步之后用户又改过的文件不能被撤销覆盖或删除。
func TestRestoreRefusesFilesEditedAfterSync(t *testing.T) {
	ctx := context.Background()
	for _, existing := range []bool{true, false} {
		left, right := t.TempDir(), t.TempDir()
		target := filepath.Join(right, "a.txt")
		if err := os.WriteFile(filepath.Join(left, "a.txt"), []byte("from-left"), 0o644); err != nil {
			t.Fatal(err)
		}
		if existing {
			if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
				t.Fatal(err)
			}
			old := time.Now().Add(-time.Hour)
			os.Chtimes(target, old, old)
		}
		plan, err := Preview(ctx, left, right, UpdateRight)
		if err != nil {
			t.Fatal(err)
		}
		report, err := Execute(ctx, plan, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(target, []byte("user edit"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err = Restore(ctx, report.Journal); err == nil {
			t.Fatalf("existing=%v: restore must refuse an edited file", existing)
		}
		if data, _ := os.ReadFile(target); string(data) != "user edit" {
			t.Fatalf("existing=%v: user edit lost: %q", existing, data)
		}
	}
}
