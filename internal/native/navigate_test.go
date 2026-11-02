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

func TestApplyPreservesFocusAndCenterGutterActions(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	dir := t.TempDir()
	left, right := filepath.Join(dir, "l.txt"), filepath.Join(dir, "r.txt")
	os.WriteFile(left, []byte("same1\nleftA\nsame2\nleftB\nsame3\nleftC\n"), 0o600)
	os.WriteFile(right, []byte("same1\nrightA\nsame2\nrightB\nsame3\nrightC\n"), 0o600)

	a := &workspaceState{paths: [2]string{left, right}}
	a.load()
	tt := ui.NewTester(a.view, 1400, 880)
	tt.Frame()
	tt.SetDark(true)
	tt.SetDark(false)

	bands := a.diff.HunkBands()
	if len(bands) != 3 {
		t.Fatalf("expected 3 hunk bands, got %d", len(bands))
	}
	for i, b := range bands {
		if b.Height <= 0 {
			t.Fatalf("band %d height <= 0: %+v", i, b)
		}
		if i > 0 && b.Top <= bands[i-1].Top {
			t.Fatalf("band %d top %v not after %v", i, b.Top, bands[i-1].Top)
		}
	}

	// 通过中间差异列点击聚焦第 2 处差异（索引 1）。
	a.selectHunk(1)
	tt.Frame()
	if a.hunkFocus != 2 || !tt.HasText("2 / 3") {
		t.Fatalf("expected focus on hunk 2/3, got %d", a.hunkFocus)
	}

	// 点击中间列的“应用到右侧 #2”按钮，应仅应用第 2 处差异，并自动将焦点保持到下一处（原第 3 处，现为 2/2）。
	rightBtn2, ok := tt.Find(i18n.T("action.applyRight") + " #2")
	if !ok {
		t.Fatal("missing center gutter right apply button #2")
	}
	tt.ClickAt(rightBtn2.X+rightBtn2.W/2, rightBtn2.Y+rightBtn2.H/2)
	tt.Frame()

	hunks := a.diff.Hunks()
	if len(hunks) != 2 {
		t.Fatalf("expected 2 hunks remaining, got %d", len(hunks))
	}
	if a.hunkFocus != 2 || !tt.HasText("2 / 2") {
		t.Fatalf("focus lost after applying hunk 2: focus=%d", a.hunkFocus)
	}
	sel, hasSel := a.diff.SelectedLines()
	if !hasSel || sel.Start != hunks[1].NewStart {
		t.Fatalf("expected highlight on new hunk[1], got %+v %v", sel, hasSel)
	}

	// 再次直接调用 apply(0)（应用到左侧），应作用于当前聚焦的第 2 处（原第 3 处），而不是跳回第 1 处。
	a.apply(0)
	tt.Frame()
	hunks = a.diff.Hunks()
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk remaining, got %d", len(hunks))
	}
	if a.hunkFocus != 1 || !tt.HasText("1 / 1") {
		t.Fatalf("expected focus clamped to remaining hunk 1/1, got %d", a.hunkFocus)
	}
	if gotL := a.panes[0].buffer.String(); gotL != "same1\nleftA\nsame2\nleftB\nsame3\nrightC\n" {
		t.Fatalf("unexpected left buffer: %q", gotL)
	}
	if gotR := a.panes[1].buffer.String(); gotR != "same1\nrightA\nsame2\nleftB\nsame3\nrightC\n" {
		t.Fatalf("unexpected right buffer: %q", gotR)
	}

	// 应用最后一处差异后，两边完全一致，焦点与高亮清除。
	a.apply(1)
	tt.Frame()
	if len(a.diff.Hunks()) != 0 || a.hunkFocus != 0 {
		t.Fatalf("expected 0 hunks and focus=0, got hunks=%d focus=%d", len(a.diff.Hunks()), a.hunkFocus)
	}
	if _, hasSel = a.diff.SelectedLines(); hasSel {
		t.Fatal("expected selection cleared when no hunks remain")
	}
}
