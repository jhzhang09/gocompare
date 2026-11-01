package native

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"gocompare/internal/i18n"
)

func TestDiffNavigationHighlightsHunk(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	dir := t.TempDir()
	left, right := filepath.Join(dir, "l.txt"), filepath.Join(dir, "r.txt")
	os.WriteFile(left, []byte("a\nb\nc\nd\ne\nf\ng\nh\n"), 0o600)
	// 三处差异：第 2 行修改、第 5 行删除、末尾追加两行。
	os.WriteFile(right, []byte("a\nB\nc\nd\nf\ng\nh\ni\nj\n"), 0o600)
	a := &workspaceState{paths: [2]string{left, right}}
	a.load()
	tt := ui.NewTester(a.view, 1400, 880)
	tt.Frame()
	hunks := a.diff.Hunks()
	if len(hunks) != 3 {
		t.Fatalf("hunks %+v", hunks)
	}
	if !tt.HasText(i18n.TArgs("diff.hunkTotal", 3)) {
		t.Fatal("total count missing before navigating")
	}

	a.diffNext()
	sel, ok := a.diff.SelectedLines()
	if !ok || sel.Side != editor.DiffNew || sel.Start != hunks[0].NewStart || sel.End != hunks[0].NewEnd-1 || a.hunkFocus != 1 {
		t.Fatalf("first hunk focus %+v %v %d", sel, ok, a.hunkFocus)
	}
	tt.Frame()
	if !tt.HasText("1 / 3") || a.message != i18n.TArgs("status.hunkPosition", 1, 3) {
		t.Fatalf("position not shown: %q", a.message)
	}

	// 纯删除的块在新文本中没有行，改为高亮旧文本对应的行。
	a.diffNext()
	sel, ok = a.diff.SelectedLines()
	if !ok || sel.Side != editor.DiffOld || sel.Start != hunks[1].OldStart || a.hunkFocus != 2 {
		t.Fatalf("deletion focus %+v %v", sel, ok)
	}
	a.diffNext()
	a.diffNext() // 越过最后一处回到第一处
	if a.hunkFocus != 1 {
		t.Fatalf("wrap forward: %d", a.hunkFocus)
	}
	a.diffPrev()
	if a.hunkFocus != 3 {
		t.Fatalf("wrap backward: %d", a.hunkFocus)
	}

	// 内容变化后重新计算，旧的定位与高亮清除。
	a.panes[1].buffer.Set("a\nb\nc\nd\ne\nf\ng\nh\nz\n")
	a.refreshComparison()
	tt.Frame()
	if _, ok = a.diff.SelectedLines(); ok || a.hunkFocus != 0 {
		t.Fatal("stale focus kept after recompute")
	}
}
