package native

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"gocompare/internal/i18n"
)

func TestHandleDropSingleFileLeftAndRight(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	dir := t.TempDir()
	f1 := filepath.Join(dir, "f1.txt")
	f2 := filepath.Join(dir, "f2.txt")
	os.WriteFile(f1, []byte("left\n"), 0o600)
	os.WriteFile(f2, []byte("right\n"), 0o600)

	a := &workspaceState{kind: kindFile}
	a.handleDrop(0, []string{f1})
	if a.paths[0] != f1 || a.paths[1] != "" {
		t.Fatalf("expected paths[0]=%q, paths[1]='', got %v", f1, a.paths)
	}
	if a.panes[0] != nil {
		t.Fatal("comparison should not start with only one path filled")
	}

	a.handleDrop(1, []string{f2})
	if a.paths[0] != f1 || a.paths[1] != f2 {
		t.Fatalf("expected paths=%v, got %v", []string{f1, f2}, a.paths)
	}
	if a.panes[0] == nil || a.panes[1] == nil {
		t.Fatal("comparison should start when second path is dropped")
	}
}

func TestHandleDropMultipleFiles(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	f2 := filepath.Join(dir, "b.txt")
	os.WriteFile(f1, []byte("aaa\n"), 0o600)
	os.WriteFile(f2, []byte("bbb\n"), 0o600)

	a := &workspaceState{kind: kindFile}
	// Drop 2 files at once onto side 0
	a.handleDrop(0, []string{f1, f2})
	if a.paths[0] != f1 || a.paths[1] != f2 {
		t.Fatalf("expected paths to be filled from multi-drop, got %v", a.paths)
	}
	if a.panes[0] == nil || a.panes[1] == nil {
		t.Fatal("comparison should start immediately upon dropping two files")
	}
}

func TestHandleDropFolderAutoSwitchesToDirectoryMode(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	os.WriteFile(filepath.Join(dir1, "x.txt"), []byte("x"), 0o600)
	os.WriteFile(filepath.Join(dir2, "x.txt"), []byte("x"), 0o600)

	a := &workspaceState{kind: kindFile}
	// Dropping a folder into file mode switches mode to directory compare
	a.handleDrop(0, []string{dir1})
	if a.kind != kindDirectory {
		t.Fatalf("expected kindDirectory (%d), got %d", kindDirectory, a.kind)
	}
	if a.paths[0] != dir1 {
		t.Fatalf("expected paths[0]=%q, got %q", dir1, a.paths[0])
	}

	a.handleDrop(1, []string{dir2})
	if a.paths[1] != dir2 {
		t.Fatalf("expected paths[1]=%q, got %q", dir2, a.paths[1])
	}
	if a.directoryRows == nil {
		t.Fatal("directory comparison should start when both folders are set")
	}
}

func TestHandleDropOnEmptyView(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	dir := t.TempDir()
	f1 := filepath.Join(dir, "left.txt")
	f2 := filepath.Join(dir, "right.txt")
	os.WriteFile(f1, []byte("1\n"), 0o600)
	os.WriteFile(f2, []byte("2\n"), 0o600)

	// Drop two files onto empty view (side = -1)
	a := &workspaceState{kind: kindFile}
	a.handleDrop(-1, []string{f1, f2})
	if a.paths[0] != f1 || a.paths[1] != f2 {
		t.Fatalf("expected both paths filled on empty view drop, got %v", a.paths)
	}
	if a.panes[0] == nil {
		t.Fatal("comparison should start")
	}

	// Drop single file onto empty view twice
	a2 := &workspaceState{kind: kindFile}
	a2.handleDrop(-1, []string{f1})
	if a2.paths[0] != f1 || a2.paths[1] != "" {
		t.Fatalf("expected first drop filled left, got %v", a2.paths)
	}
	a2.handleDrop(-1, []string{f2})
	if a2.paths[0] != f1 || a2.paths[1] != f2 {
		t.Fatalf("expected second drop filled right, got %v", a2.paths)
	}
	if a2.panes[0] == nil {
		t.Fatal("comparison should start after second drop")
	}
}

func TestHandleDropTableMode(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	dir := t.TempDir()
	c1 := filepath.Join(dir, "t1.csv")
	c2 := filepath.Join(dir, "t2.csv")
	os.WriteFile(c1, []byte("a,b\n1,2\n"), 0o600)
	os.WriteFile(c2, []byte("a,b\n1,3\n"), 0o600)

	a := &workspaceState{kind: kindTable}
	a.handleDrop(0, []string{c1, c2})
	if !a.tableReady {
		t.Fatal("table comparison should be ready after drop")
	}
	if len(a.tableDiffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(a.tableDiffs))
	}
}

func TestUITesterDropTarget(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	dir := t.TempDir()
	f1 := filepath.Join(dir, "left.txt")
	f2 := filepath.Join(dir, "right.txt")
	os.WriteFile(f1, []byte("L\n"), 0o600)
	os.WriteFile(f2, []byte("R\n"), 0o600)

	a := &workspaceState{kind: kindFile}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()

	// Verify empty view accepts dropped files
	// Drop in center of window where modeEmptyView is located (e.g. 600, 400)
	taken := tt.DropFiles(600, 400, f1, f2)
	if !taken {
		t.Fatal("empty view should take dropped files")
	}
	tt.Frame()
	if a.paths[0] != f1 || a.paths[1] != f2 {
		t.Fatalf("expected paths filled via DropFiles, got %v", a.paths)
	}
	if a.panes[0] == nil {
		t.Fatal("comparison should load after dropping two files")
	}
}
