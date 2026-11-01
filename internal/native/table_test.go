package native

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestWorkspaceTableCompare(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left.csv")
	right := filepath.Join(dir, "right.csv")
	if err := os.WriteFile(left, []byte("name,age\nAlice,30\nBob,25\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("name,age\nAlice,30\nBob,26\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	a := &workspaceState{}
	a.selectKind(kindTable)
	if a.kind != kindTable {
		t.Fatalf("expected kindTable, got %d", a.kind)
	}

	// Verify empty view renders cleanly
	tt := ui.NewTester(a.view, 1000, 700)
	tt.SetDark(false)
	tt.Frame()

	// Load table files
	a.paths = [2]string{left, right}
	a.loadTable()

	if !a.tableReady {
		t.Fatal("tableReady is false")
	}
	if len(a.tableDiffs) != 1 {
		t.Fatalf("expected 1 diff, got %d (%+v)", len(a.tableDiffs), a.tableDiffs)
	}

	// Re-render with diffs
	tt.Frame()

	// Reset mode
	a.resetCurrentMode()
	if a.tableReady || len(a.tableDiffs) != 0 || a.paths[0] != "" {
		t.Fatalf("reset failed: ready=%v diffs=%v paths=%v", a.tableReady, a.tableDiffs, a.paths)
	}
}

func TestReadTableRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.csv")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	err = f.Truncate((32 << 20) + 1)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if data, err := readTableFile(path); err == nil || data != nil {
		t.Fatal("oversized table was silently truncated")
	}
}
