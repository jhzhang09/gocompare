package acl

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectAndCompare(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "f1.txt")
	f2 := filepath.Join(dir, "f2.txt")
	f3 := filepath.Join(dir, "f3.txt")

	if err := os.WriteFile(f1, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f3, []byte("hello"), 0o755); err != nil {
		t.Fatal(err)
	}

	info1, err := Inspect(f1)
	if IsWindows() {
		if !errors.Is(err, ErrUnsupported) || info1 == nil || info1.Supported || len(info1.Entries) != 0 {
			t.Fatalf("fabricated ACL: %+v %v", info1, err)
		}
		return
	}
	if err != nil {
		t.Fatalf("Inspect f1: %v", err)
	}
	info2, err := Inspect(f2)
	if err != nil {
		t.Fatalf("Inspect f2: %v", err)
	}
	info3, err := Inspect(f3)
	if err != nil {
		t.Fatalf("Inspect f3: %v", err)
	}

	// 1. Same file modes should match
	diffs := Compare(info1, info2)
	if len(diffs) != 0 {
		t.Fatalf("expected no diffs between f1 and f2, got: %v", diffs)
	}
	if FormatDiffs(diffs) != "ACL and permissions match" {
		t.Fatalf("unexpected format: %s", FormatDiffs(diffs))
	}

	// 2. Different permissions should show diff
	diffs13 := Compare(info1, info3)
	if len(diffs13) == 0 {
		t.Fatal("expected diffs between f1 (0644) and f3 (0755)")
	}
	// owner、group、others 三项权限都不同（0644 vs 0755），每项差异只报告一次。
	if len(diffs13) != 3 {
		t.Fatalf("expected one diff per changed entry, got: %v", diffs13)
	}
	formatted := FormatDiffs(diffs13)
	if !strings.Contains(formatted, "differs") {
		t.Fatalf("expected formatted text to contain differs, got: %s", formatted)
	}

	// 3. Compare with nil
	if diffsNil := Compare(info1, nil); len(diffsNil) == 0 {
		t.Fatal("expected diffs for nil input")
	}

	// 4. Non-existent file
	if _, err := Inspect(filepath.Join(dir, "missing.txt")); err == nil {
		t.Fatal("expected error on missing file")
	}
}

func TestUnsupportedACLDoesNotMatch(t *testing.T) {
	if len(Compare(&Info{}, &Info{})) == 0 {
		t.Fatal("unsupported ACL reported equal")
	}
}
