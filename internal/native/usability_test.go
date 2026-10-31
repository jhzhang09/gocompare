package native

import (
	"context"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"gocompare/internal/compare"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestIgnoreSpaceUpdatesRealHunksAndPersists(t *testing.T) {
	a := testWorkspace(t)
	a.panes[0].buffer.Set("a b\n")
	a.panes[1].buffer.Set("ab\n")
	a.recompute()
	if len(a.diff.Hunks()) == 0 {
		t.Fatal("fixture has no difference")
	}
	a.compareIgnoringSpace()
	if !a.ignoreSpace || len(a.diff.Hunks()) != 0 {
		t.Fatal("rule changed label but not actual diff")
	}
	a.panes[0].buffer.Set("a  b\n")
	a.refreshComparison()
	if len(a.diff.Hunks()) != 0 {
		t.Fatal("rule did not survive editing")
	}
	a.compareIgnoringSpace()
	if len(a.diff.Hunks()) == 0 {
		t.Fatal("cannot return to exact comparison")
	}
}
func TestSwitchKindProtectsDirtyBuffersAndClearsOldView(t *testing.T) {
	a := testWorkspace(t)
	a.panes[0].buffer.Edit(editor.Edit{Text: "unsaved"})
	a.selectKind(1)
	if a.kind != 0 || !a.confirm || a.panes[0] == nil {
		t.Fatal("changed view before discard confirmation")
	}
	a.confirm = false
	a.pending = nil
	a.panes[0].buffer.MarkSaved()
	a.selectKind(1)
	if a.kind != 1 || a.panes[0] != nil || a.directoryRows != nil {
		t.Fatal("old file result leaked into folder view")
	}
}
func TestOrphanOpenCopyAndSave(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	os.Mkdir(filepath.Join(left, "nested"), 0o700)
	original := []byte("仅左侧\n")
	os.WriteFile(filepath.Join(left, "nested", "only.txt"), original, 0o600)
	a := &workspaceState{kind: 1, paths: [2]string{left, right}}
	a.load()
	a.openDirectoryEntry(directoryRow(a, "nested/only.txt"))
	if a.err != "" || a.panes[1] == nil || !a.panes[1].file.Missing() {
		t.Fatalf("orphan not opened: %s", a.err)
	}
	a.apply(1)
	a.save(1)
	if a.err != "" {
		t.Fatal(a.err)
	}
	data, err := os.ReadFile(filepath.Join(right, "nested", "only.txt"))
	if err != nil || string(data) != string(original) {
		t.Fatalf("missing counterpart not created: %q %v", data, err)
	}
}
func TestDirectoryCollapseAndFilteredSyncBoundary(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	for _, root := range []string{left, right} {
		os.Mkdir(filepath.Join(root, "nested"), 0o700)
		os.WriteFile(filepath.Join(root, "nested", "a.txt"), []byte(root), 0o600)
		os.WriteFile(filepath.Join(root, "nested", "excluded.bin"), []byte(root), 0o600)
	}
	a := &workspaceState{kind: 1, paths: [2]string{left, right}, dirFilter: "*.txt"}
	a.load()
	if len(a.directoryRows) != 2 || directoryRow(a, "nested/a.txt").Path == "" {
		t.Fatalf("recursive glob: %+v", a.directoryRows)
	}
	a.openDirectoryEntry(directoryRow(a, "nested"))
	if len(a.visibleDirectoryRows()) != 1 {
		t.Fatal("collapse did not hide descendants")
	}
	a.openDirectoryEntry(directoryRow(a, "nested"))
	if len(a.visibleDirectoryRows()) != 2 {
		t.Fatal("expand failed")
	}
	a.syncChoice = "镜像到右侧"
	a.previewSync()
	if a.syncPlan != nil || a.err == "" {
		t.Fatal("sync exceeded the filtered comparison scope")
	}
}
func TestAnchoredRequestCoalescesAndRejectsStaleResult(t *testing.T) {
	a := testWorkspace(t)
	a.anchorLeftSet = true
	a.anchorRightSet = true
	a.anchorLeft = 1
	a.anchorRight = 1
	a.commitAnchor()
	var queue []func()
	a.compareLaunch = func(fn func()) { queue = append(queue, fn) }
	a.recompute()
	a.refreshComparison()
	a.refreshComparison()
	if len(queue) != 1 {
		t.Fatalf("same anchored request repeatedly cancelled: %d", len(queue))
	}
	a.panes[0].buffer.Edit(editor.Edit{Text: "new\n"})
	queue[0]()
	if a.diff.ComparisonReady() {
		t.Fatal("stale anchored hunks applied to new text")
	}
}
func TestWorkbenchLayouts(t *testing.T) {
	a := testWorkspace(t)
	for _, width := range []float32{900, 1380} {
		tt := ui.NewTester(a.view, int(width), 800)
		tt.Frame()
		for _, label := range []string{"开始比较", "交换两侧", "上一差异", "下一差异", "查找", "保存左侧", "保存右侧"} {
			r, ok := tt.Find(label)
			if !ok || r.X < 0 || r.X+r.W > width+1 {
				t.Fatalf("%s outside %v viewport: %+v", label, width, r)
			}
		}
	}
	left, right := t.TempDir(), t.TempDir()
	for _, root := range []string{left, right} {
		os.Mkdir(filepath.Join(root, "src"), 0o700)
		os.WriteFile(filepath.Join(root, "src", "config.go"), []byte(root), 0o600)
		os.WriteFile(filepath.Join(root, "README.md"), []byte("same"), 0o600)
	}
	os.WriteFile(filepath.Join(left, "local.env"), []byte("example"), 0o600)
	rows, err := compare.Directories(context.Background(), left, right)
	if err != nil {
		t.Fatal(err)
	}
	a.installDirectory([2]string{left, right}, rows)
	for _, dark := range []bool{false, true} {
		tt := ui.NewTester(a.view, 1380, 800)
		tt.SetDark(dark)
		tt.Frame()
		if dir := os.Getenv("GOCOMPARE_SNAPSHOTS"); dir != "" {
			os.MkdirAll(dir, 0o755)
			name := "directory-light.png"
			if dark {
				name = "directory-dark.png"
			}
			f, err := os.Create(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, tt.Image())
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestCopyHunkPreservesEmptyAndFinalNewline(t *testing.T) {
	for _, content := range []string{"text", "text\n", "\n", "text\n\n", "中文🙂\n"} {
		for _, right := range []bool{false, true} {
			old, new := editor.NewBuffer(content), editor.NewBuffer("")
			if !right {
				old, new = new, old
			}
			d := editor.DiffState{Old: old, New: new}
			if right {
				d.Revert(0)
			} else {
				d.Accept(0)
			}
			if old.String() != new.String() || old.String() != content {
				t.Fatalf("copy right=%v %q → %q / %q", right, content, old.String(), new.String())
			}
		}
	}
}

func TestSessionRetainsComparisonRules(t *testing.T) {
	a := testWorkspace(t)
	a.ignoreSpace = true
	session := filepath.Join(t.TempDir(), "rules.gocompare")
	a.saveSession(session)
	a.ignoreSpace = false
	a.openSession(session)
	if a.err != "" || !a.ignoreSpace {
		t.Fatalf("text rules lost: %s", a.err)
	}
	left, right := t.TempDir(), t.TempDir()
	for _, dir := range []string{left, right} {
		os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o600)
		os.WriteFile(filepath.Join(dir, "b.bin"), []byte("b"), 0o600)
	}
	a.paths = [2]string{left, right}
	a.kind = 1
	a.dirFilter = "*.txt"
	a.load()
	a.saveSession(session)
	a.dirFilter = ""
	a.openSession(session)
	if a.err != "" || a.dirFilter != "*.txt" || len(a.directoryRows) != 1 || a.directoryOptions.Filter != "*.txt" {
		t.Fatalf("directory rules lost: %s %+v", a.err, a.directoryRows)
	}
}
