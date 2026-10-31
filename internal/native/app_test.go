package native

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"gocompare/internal/document"
	"gocompare/internal/integrate"
)

func testWorkspace(t *testing.T) *workspaceState {
	t.Helper()
	dir := t.TempDir()
	a := &workspaceState{paths: [2]string{filepath.Join(dir, "left.go"), filepath.Join(dir, "right.go")}}
	for i, p := range a.paths {
		data := []string{"package main\n\nfunc main() {\n println(\"Hello\")\n}\n", "package main\n\nfunc main() {\n println(\"你好，GoCompare\")\n}\n"}[i]
		if err := os.WriteFile(p, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	a.load()
	if a.err != "" {
		t.Fatal(a.err)
	}
	return a
}
func TestWorkspaceLifecycle(t *testing.T) {
	a := testWorkspace(t)
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()
	a.diff.GoToHunk(0)
	a.apply(1)
	tt.Frame()
	if a.panes[1].buffer.String() != a.panes[0].buffer.String() || !a.dirty() {
		t.Fatal("copy to right failed")
	}
	a.save(1)
	if a.err != "" || a.dirty() {
		t.Fatalf("save: %s", a.err)
	}
	for _, mode := range []int{1, 2, 0, 1, 0} {
		a.mode = mode
		tt.Frame()
		tt.Frame()
	}
	a.panes[0].buffer.Edit(editor.Edit{Text: "// 修改\n"})
	a.guard(a.load)
	if !a.confirm || a.pending == nil {
		t.Fatal("missing discard confirmation")
	}
	a.confirm = false
	a.pending = nil
	// 外部修改必须保留在磁盘上，并在界面报告错误。
	os.WriteFile(a.paths[0], []byte("external"), 0o600)
	a.save(0)
	if a.err == "" {
		t.Fatal("missing conflict error")
	}
	f, err := document.Read(context.Background(), a.paths[0])
	if err != nil || f.Text != "external" {
		t.Fatalf("external file overwritten: %v", err)
	}
}
func TestWorkspaceScreenshots(t *testing.T) {
	a := testWorkspace(t)
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()
	if dir := os.Getenv("GOCOMPARE_SNAPSHOTS"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(filepath.Join(dir, "compare.png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err = png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEditingWithKeyboardAndUndo(t *testing.T) {
	a := testWorkspace(t)
	a.optionsOpen = true
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()
	for side, label := range []string{"编辑左侧", "编辑右侧"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		p := a.panes[side]
		before := p.buffer.String()
		if err := tt.Click([]string{"左侧编辑器", "右侧编辑器"}[side]); err != nil {
			t.Fatal(err)
		}
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type("新的内容\n")
		if got := p.buffer.String(); got != "新的内容\n" {
			t.Fatalf("side %d typing: %q", side, got)
		}
		tt.Key(ui.Cmd, ui.KeyZ)
		if got := p.buffer.String(); got != before {
			t.Fatalf("side %d undo: %q", side, got)
		}
		if err := tt.Click("比较"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	tt.SetDark(true)
	tt.Frame()
	if !tt.HasText("GoCompare") {
		t.Fatal("workspace missing after theme switch")
	}
}

// 在对齐视图中直接输入，验证焦点侧、撤销、保存目标及页面重建。
func TestSimultaneousEditing(t *testing.T) {
	a := testWorkspace(t)
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()
	for side := 0; side < 2; side++ {
		// 比较区从工具栏下方开始；两侧始终保留原始左右顺序。
		if err := tt.Click([]string{"Old text", "New text"}[side]); err != nil {
			t.Fatal(err)
		}
		before := a.panes[side].buffer.String()
		other := a.panes[1-side].buffer.String()
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type("新的内容\n\n末行")
		if got := a.panes[side].buffer.String(); got != "新的内容\n\n末行" {
			t.Fatalf("side %d typing: %q", side, got)
		}
		if a.panes[1-side].buffer.String() != other {
			t.Fatal("edited wrong side")
		}
		if got := int(a.diff.ActiveSide()); got != side {
			t.Fatalf("active side=%d want %d", got, side)
		}
		tt.Key(ui.Cmd, ui.KeyZ)
		if got := a.panes[side].buffer.String(); got != before {
			t.Fatalf("side %d undo: %q", side, got)
		}
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type("已保存\n")
		tt.Key(ui.Cmd, ui.KeyS)
		if a.err != "" {
			t.Fatal(a.err)
		}
		f, err := document.Read(context.Background(), a.paths[side])
		if err != nil || f.Text != "已保存\n" {
			t.Fatalf("side %d save: %v", side, err)
		}
		if a.panes[1-side].buffer.String() != other {
			t.Fatal("saved wrong side")
		}
	}
	a.wrap = true
	tt.Frame()
	tt.Frame()
	a.mode = 1
	tt.Frame()
	a.mode = 0
	tt.Frame()
	tt.Frame()
	if !a.diff.OldEditable || !a.diff.Editable {
		t.Fatal("editable state lost on rebuild")
	}
}

// 上游默认仍禁止左侧编辑，应用忙碌时两侧均禁止编辑。
func TestDiffEditPolicy(t *testing.T) {
	for _, allow := range []bool{false, true} {
		d := editor.DiffState{Old: editor.NewBuffer("left"), New: editor.NewBuffer("right"), Editable: true, OldEditable: allow}
		tt := ui.NewTester(func(c *ui.Context) { editor.DiffView(c, &d).Fill() }, 800, 400)
		tt.Frame()
		tt.ClickAt(90, 12)
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type("changed")
		expected := "left"
		if allow {
			expected = "changed"
		}
		if d.Old.String() != expected {
			t.Fatalf("OldEditable=%v: %q", allow, d.Old.String())
		}
	}
	a := testWorkspace(t)
	a.busy = true
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()
	for side := 0; side < 2; side++ {
		before := a.panes[side].buffer.String()
		if err := tt.Click([]string{"Old text", "New text"}[side]); err != nil {
			t.Fatal(err)
		}
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type("blocked")
		if a.panes[side].buffer.String() != before {
			t.Fatal("edited during I/O")
		}
	}
}

func TestDualEditAcrossAlignmentChanges(t *testing.T) {
	d := editor.DiffState{Old: editor.NewBuffer("header\nleft\ntail\n"), New: editor.NewBuffer("header\nright\ntail\n"), Editable: true, OldEditable: true, ExpandUnchanged: true, Wrap: true}
	tt := ui.NewTester(func(c *ui.Context) { editor.DiffView(c, &d).Fill() }, 800, 400)
	tt.Frame()
	for side, label := range []string{"Old text", "New text"} {
		content := "共同标题\n" + strings.Repeat("很长的中文内容和 emoji 🙂 ", 40) + "\n新增一行\n共同结尾\n"
		if side == 1 {
			content = "共同标题\n右侧不同\n共同结尾\n"
		}
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type(content)
		for range 3 {
			tt.Frame()
		}
		buffer := d.Old
		if side == 1 {
			buffer = d.New
		}
		if buffer.String() != content {
			t.Fatal("content lost after alignment rebuild")
		}
		// 所有差异块按顺序替换后，必须能够从左侧精确重建右侧文本。
		old, new := d.Old.Snapshot(), d.New.Snapshot()
		var result strings.Builder
		offset := 0
		for _, h := range d.Hunks() {
			start, end := old.Offset(editor.Pos{Line: h.OldStart}), old.Offset(editor.Pos{Line: h.OldEnd})
			result.WriteString(old.Slice(offset, start))
			result.WriteString(new.Slice(new.Offset(editor.Pos{Line: h.NewStart}), new.Offset(editor.Pos{Line: h.NewEnd})))
			offset = end
		}
		result.WriteString(old.Slice(offset, old.Len()))
		if result.String() != new.String() {
			t.Fatal("invalid hunks after editing")
		}
	}
	d.Wrap = false
	tt.Frame()
	d.Wrap = true
	tt.Frame()
	tt.Frame()
}

func TestSessionAndSaveAs(t *testing.T) {
	a := testWorkspace(t)
	dir := t.TempDir()
	session := filepath.Join(dir, "test.gocompare")
	a.wrap = true
	a.saveSession(session)
	if a.err != "" {
		t.Fatal(a.err)
	}
	original := a.paths
	a.panes[0].buffer.Edit(editor.Edit{Text: "修改"})
	target := filepath.Join(dir, "new.go")
	a.saveAsPath(0, target)
	if a.err != "" || a.paths[0] != target || a.panes[0].buffer.Modified() {
		t.Fatalf("save as: %s", a.err)
	}
	if a.panes[0].edit.Label != "左侧编辑器" {
		t.Fatal("editor identity not updated")
	}
	a.openSession(session)
	if a.err != "" || a.paths != original || !a.wrap {
		t.Fatalf("session: %s", a.err)
	}
	// 加载失败不会丢弃当前工作区。
	a.openSession(filepath.Join(dir, "missing"))
	if a.err == "" || a.paths != original {
		t.Fatal("failed session replaced workspace")
	}
	a.backupDir = filepath.Join(dir, "backups")
	a.panes[1].buffer.Edit(editor.Edit{Text: "备份"})
	a.save(1)
	if a.err != "" {
		t.Fatal(a.err)
	}
	entries, err := os.ReadDir(a.backupDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("backup missing: %v", err)
	}
}

func TestDirectoryWorkspaceAndSession(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	for _, dir := range []string{left, right} {
		os.WriteFile(filepath.Join(dir, "same.txt"), []byte("same"), 0o600)
	}
	os.WriteFile(filepath.Join(left, "orphan.txt"), []byte("left"), 0o600)
	a := &workspaceState{kind: 1, paths: [2]string{left, right}}
	a.load()
	if a.err != "" || len(a.directoryRows) != 2 {
		t.Fatalf("load: %s", a.err)
	}
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()
	tt.Frame()
	session := filepath.Join(t.TempDir(), "directory.gocompare")
	a.saveSession(session)
	if a.err != "" {
		t.Fatal(a.err)
	}
	if err := tt.Click("same.txt"); err != nil {
		t.Fatal(err)
	}
	if a.panes[0] == nil || a.directoryRows != nil {
		t.Fatal("file pair did not open")
	}
	if err := tt.Click("返回目录"); err != nil {
		t.Fatal(err)
	}
	if len(a.directoryRows) != 2 {
		t.Fatal("parent directory lost")
	}
	if err := tt.Click("仅显示差异"); err != nil {
		t.Fatal(err)
	}
	// 「最近扫描」会带上最后走过的路径，不能用子串判断这一行是否还在列表里。
	if _, ok := tt.Find("same.txt"); ok {
		t.Fatal("same file not filtered")
	}
	if _, ok := tt.Find("orphan.txt"); !ok {
		t.Fatal("difference row hidden")
	}
	a.openSession(session)
	if a.err != "" || len(a.directoryRows) != 2 || a.directoryRoots != [2]string{left, right} {
		t.Fatalf("restore: %s", a.err)
	}
}
func TestBackupFailureDoesNotSave(t *testing.T) {
	a := testWorkspace(t)
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	os.WriteFile(blocked, []byte("x"), 0o600)
	a.backupDir = blocked
	original, _ := os.ReadFile(a.paths[0])
	a.panes[0].buffer.Edit(editor.Edit{Text: "new"})
	a.save(0)
	if a.err == "" || !a.panes[0].buffer.Modified() {
		t.Fatal("backup failure was ignored")
	}
	got, _ := os.ReadFile(a.paths[0])
	if string(got) != string(original) {
		t.Fatal("file saved without backup")
	}
}

func TestSyncPreviewAndExecution(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(left, "new.txt"), []byte("new"), 0o600)
	os.WriteFile(filepath.Join(right, "old.txt"), []byte("old"), 0o600)
	a := &workspaceState{kind: 1, paths: [2]string{left, right}, backupDir: t.TempDir()}
	a.load()
	a.syncChoice = "镜像到右侧"
	a.previewSync()
	if a.err != "" || a.syncPlan == nil || len(a.syncPlan.Actions) != 2 {
		t.Fatalf("preview: %s", a.err)
	}
	if _, err := os.Stat(filepath.Join(right, "new.txt")); !os.IsNotExist(err) {
		t.Fatal("preview modified destination")
	}
	a.optionsOpen = true
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()
	if err := tt.Click("执行同步…"); err != nil {
		t.Fatal(err)
	}
	if !a.confirmSync {
		t.Fatal("execution missing confirmation")
	}
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	if a.confirmSync {
		t.Fatal("cancel did not close dialog")
	}
	if _, err := os.Stat(filepath.Join(right, "old.txt")); err != nil {
		t.Fatal("cancel modified files")
	}
	a.executeSync()
	if a.err != "" {
		t.Fatal(a.err)
	}
	data, err := os.ReadFile(filepath.Join(right, "new.txt"))
	if err != nil || string(data) != "new" {
		t.Fatalf("sync copy: %q %v", data, err)
	}
	if _, err = os.Stat(filepath.Join(right, "old.txt")); !os.IsNotExist(err) {
		t.Fatal("mirror did not remove orphan")
	}
	for _, row := range a.directoryRows {
		if row.Status != "same" {
			t.Fatalf("stale comparison after sync: %+v", row)
		}
	}
	tt = ui.NewTester(a.view, 1380, 880)
	tt.Frame()
	if err = tt.Click("撤销上次同步"); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(right, "old.txt")); err != nil {
		t.Fatal("undo did not restore orphan")
	}
	if a.message != "已按操作记录撤销同步" {
		t.Fatalf("undo message: %s", a.message)
	}
	if _, err = os.Stat(filepath.Join(right, "new.txt")); !os.IsNotExist(err) {
		t.Fatal("undo left copied file")
	}
}

func TestTextMergeWorkspace(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.txt")
	left := filepath.Join(dir, "left.txt")
	right := filepath.Join(dir, "right.txt")
	output := filepath.Join(dir, "out.txt")
	os.WriteFile(base, []byte("a\nb\n"), 0o600)
	os.WriteFile(left, []byte("a\nleft\n"), 0o600)
	os.WriteFile(right, []byte("a\nright\n"), 0o600)
	a := &workspaceState{kind: 2, paths: [2]string{left, right}, mergeBase: base, mergeOutput: output}
	a.load()
	if a.err != "" || a.mergeResult == nil || a.mergeResult.ConflictCount() != 1 {
		t.Fatalf("merge: %s", a.err)
	}
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()
	if !tt.HasText("冲突 1 / 1") {
		t.Fatal("conflict status missing")
	}
	if err := tt.Click("采用左侧"); err != nil {
		t.Fatal(err)
	}
	if a.mergeResult.ConflictCount() != 0 || a.mergeBuffer.String() != "a\nleft\n" {
		t.Fatalf("take: %q", a.mergeBuffer.String())
	}
	a.saveMerge()
	data, err := os.ReadFile(output)
	if err != nil || string(data) != "a\nleft\n" {
		t.Fatalf("saved: %q %v", data, err)
	}
	session := filepath.Join(dir, "merge.gocompare")
	a.saveSession(session)
	b := &workspaceState{}
	b.openSession(session)
	if b.err != "" || b.kind != 2 || b.mergeResult == nil || b.mergeResult.ConflictCount() != 1 {
		t.Fatalf("session: kind %d err %s", b.kind, b.err)
	}
}

func TestThreeColumnMergeAlignment(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.txt")
	left := filepath.Join(dir, "left.txt")
	right := filepath.Join(dir, "right.txt")
	if err := os.WriteFile(base, []byte("a\nb\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(left, []byte("a\nleft\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("a\nright\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	beforeBase, _ := os.ReadFile(base)
	beforeLeft, _ := os.ReadFile(left)
	beforeRight, _ := os.ReadFile(right)
	a := &workspaceState{kind: 2, paths: [2]string{left, right}, mergeBase: base}
	a.load()
	if a.err != "" || a.mergeResult == nil || !a.mergeResult.HasBase() || len(a.mergeRows) != 2 {
		t.Fatalf("merge: err %s rows %d", a.err, len(a.mergeRows))
	}
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()
	baseCell, okB := tt.Find("基准第2行 b")
	leftCell, okL := tt.Find("左侧第2行 left")
	rightCell, okR := tt.Find("右侧第2行 right")
	commonL, okC := tt.Find("左侧第1行 a")
	if !okB || !okL || !okR || !okC || !tt.HasText("对齐列 基准") {
		t.Fatalf("columns missing: base %v left %v right %v common %v", okB, okL, okR, okC)
	}
	if !overlapY(baseCell, leftCell) || !overlapY(leftCell, rightCell) || !(baseCell.X < leftCell.X && leftCell.X < rightCell.X) || !(commonL.Y < leftCell.Y) {
		t.Fatalf("alignment: base %+v left %+v right %+v common %+v", baseCell, leftCell, rightCell, commonL)
	}
	if err := tt.Click("采用左侧"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.mergeBuffer.String() != "a\nleft\n" || a.mergeResult.ConflictCount() != 0 {
		t.Fatalf("take: %q conflicts %d", a.mergeBuffer.String(), a.mergeResult.ConflictCount())
	}
	keptBase, okB2 := tt.Find("基准第2行 b")
	keptLeft, okL2 := tt.Find("左侧第2行 left")
	keptRight, okR2 := tt.Find("右侧第2行 right")
	if !okB2 || !okL2 || !okR2 || !overlapY(keptBase, keptLeft) || !overlapY(keptLeft, keptRight) {
		t.Fatal("resolved merge dropped an aligned source line")
	}
	afterBase, _ := os.ReadFile(base)
	afterLeft, _ := os.ReadFile(left)
	afterRight, _ := os.ReadFile(right)
	if string(afterBase) != string(beforeBase) || string(afterLeft) != string(beforeLeft) || string(afterRight) != string(beforeRight) {
		t.Fatal("alignment wrote an input file")
	}
}

func TestThreeColumnMergeScrollsTogether(t *testing.T) {
	dir := t.TempDir()
	var base, left, right strings.Builder
	for i := 0; i < 400; i++ {
		line := fmt.Sprintf("b%03d\n", i)
		base.WriteString(line)
		if i == 300 {
			left.WriteString("left-300\n")
			right.WriteString("right-300\n")
			continue
		}
		left.WriteString(line)
		right.WriteString(line)
	}
	basePath := filepath.Join(dir, "base.txt")
	leftPath := filepath.Join(dir, "left.txt")
	rightPath := filepath.Join(dir, "right.txt")
	os.WriteFile(basePath, []byte(base.String()), 0o600)
	os.WriteFile(leftPath, []byte(left.String()), 0o600)
	os.WriteFile(rightPath, []byte(right.String()), 0o600)
	a := &workspaceState{kind: 2, paths: [2]string{leftPath, rightPath}, mergeBase: basePath}
	a.load()
	if a.err != "" || len(a.mergeRows) != 400 || !a.mergeRows[300].Conflict || a.mergeRows[300].Left != "left-300" || a.mergeRows[300].Right != "right-300" || a.mergeRows[300].Base != "b300" {
		t.Fatalf("rows %d err %s", len(a.mergeRows), a.err)
	}
	tt := ui.NewTester(a.view, 1100, 520)
	tt.Frame()
	if tt.HasText("左侧第301行 left-300") {
		t.Fatal("off-screen alignment row was built")
	}
	a.mergeAlign.ScrollTo(300, ui.Start)
	tt.Frame()
	baseCell, okB := tt.Find("基准第301行 b300")
	leftCell, okL := tt.Find("左侧第301行 left-300")
	rightCell, okR := tt.Find("右侧第301行 right-300")
	if !okB || !okL || !okR || !overlapY(baseCell, leftCell) || !overlapY(leftCell, rightCell) || !(baseCell.X < leftCell.X && leftCell.X < rightCell.X) {
		t.Fatalf("scrolled: base %v %+v left %v %+v right %v %+v", okB, baseCell, okL, leftCell, okR, rightCell)
	}
}

func overlapY(a, b ui.Rect) bool {
	return a.Y < b.Y+b.H && b.Y < a.Y+a.H
}

func TestIndependentMergeWindow(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.txt")
	left := filepath.Join(dir, "left.txt")
	right := filepath.Join(dir, "right.txt")
	output := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(base, []byte("a\nb\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(left, []byte("a\nleft\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("a\nright\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	parent := &workspaceState{paths: [2]string{left, right}, mergeBase: base, mergeOutput: output}
	parent.load()
	if parent.err != "" || parent.kind != 0 || parent.panes[0] == nil {
		t.Fatalf("compare: kind %d err %s", parent.kind, parent.err)
	}
	beforeLeft, _ := os.ReadFile(left)
	beforeRight, _ := os.ReadFile(right)
	beforeBase, _ := os.ReadFile(base)
	parent.optionsOpen = true
	tt := ui.NewTester(parent.view, 1380, 880)
	tt.Frame()
	if !tt.HasText("忽略空白") || tt.HasText("基准第2行 b") || tt.HasText("独立合并窗口") {
		t.Fatal("file compare already showed the merge window")
	}
	if err := tt.Click("打开合并窗口"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if parent.kind != 0 || parent.mergeResult != nil || len(parent.mergeWindows) != 1 || parent.message != "合并已在新窗口打开" {
		t.Fatalf("parent changed: kind %d windows %d message %s", parent.kind, len(parent.mergeWindows), parent.message)
	}
	if !tt.HasText("忽略空白") || tt.HasText("独立合并窗口") || tt.HasText("基准第2行 b") {
		t.Fatal("opening the merge window replaced the file comparison")
	}
	child := parent.mergeWindows[0]
	if !child.detached || child.win != nil || child.mergeResult == nil || child.mergeResult.ConflictCount() != 1 {
		t.Fatalf("child: detached %v win %v err %s", child.detached, child.win != nil, child.err)
	}
	mt := ui.NewTester(child.mergeWindowView, 1200, 800)
	mt.Frame()
	if !mt.HasText("独立合并窗口") || mt.HasText("忽略空白") || mt.HasText("标记左侧") {
		t.Fatal("merge window reused the file-compare toolbar")
	}
	baseCell, okB := mt.Find("基准第2行 b")
	leftCell, okL := mt.Find("左侧第2行 left")
	rightCell, okR := mt.Find("右侧第2行 right")
	if !okB || !okL || !okR || !overlapY(baseCell, leftCell) || !overlapY(leftCell, rightCell) || !(baseCell.X < leftCell.X && leftCell.X < rightCell.X) {
		t.Fatalf("window alignment: base %v %+v left %v %+v right %v %+v", okB, baseCell, okL, leftCell, okR, rightCell)
	}
	if err := mt.Click("采用左侧"); err != nil {
		t.Fatal(err)
	}
	mt.Frame()
	if child.mergeBuffer.String() != "a\nleft\n" || child.mergeResult.ConflictCount() != 0 || parent.mergeResult != nil {
		t.Fatalf("decision leaked: child %q parent result %v", child.mergeBuffer.String(), parent.mergeResult != nil)
	}
	if err := mt.Click("保存输出"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil || string(data) != "a\nleft\n" {
		t.Fatalf("output: %q %v", data, err)
	}
	afterLeft, _ := os.ReadFile(left)
	afterRight, _ := os.ReadFile(right)
	afterBase, _ := os.ReadFile(base)
	if string(afterLeft) != string(beforeLeft) || string(afterRight) != string(beforeRight) || string(afterBase) != string(beforeBase) {
		t.Fatal("merge window wrote an input")
	}
	if parent.panes[0].buffer.String() != "a\nleft\n" || parent.kind != 0 {
		t.Fatal("parent comparison buffer changed")
	}

	two := &workspaceState{paths: [2]string{left, right}}
	two.optionsOpen = true
	wt := ui.NewTester(two.view, 1100, 700)
	wt.Frame()
	if err := wt.Click("打开合并窗口"); err != nil {
		t.Fatal(err)
	}
	if len(two.mergeWindows) != 1 || two.mergeWindows[0].mergeResult.HasBase() {
		t.Fatalf("two-way window: %d", len(two.mergeWindows))
	}
	twoView := ui.NewTester(two.mergeWindows[0].mergeWindowView, 1100, 700)
	twoView.Frame()
	if !twoView.HasText("独立合并窗口") || twoView.HasText("对齐列 基准") || !twoView.HasText("左侧第2行 left") || !twoView.HasText("右侧第2行 right") {
		t.Fatal("two-way window showed a base column")
	}
}

func TestIndependentMergeWindowRejects(t *testing.T) {
	empty := &workspaceState{}
	empty.optionsOpen = true
	tt := ui.NewTester(empty.view, 1000, 600)
	tt.Frame()
	if err := tt.Click("打开合并窗口"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if empty.err != "请先选择左右两个文件" || len(empty.mergeWindows) != 0 || !tt.HasText("错误") {
		t.Fatalf("empty: %s windows %d", empty.err, len(empty.mergeWindows))
	}
	dir := t.TempDir()
	left := filepath.Join(dir, "left.bin")
	right := filepath.Join(dir, "right.txt")
	if err := os.WriteFile(left, []byte{0xff, 0x00}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := &workspaceState{paths: [2]string{left, right}}
	binary.optionsOpen = true
	bt := ui.NewTester(binary.view, 1000, 600)
	bt.Frame()
	if err := bt.Click("打开合并窗口"); err != nil {
		t.Fatal(err)
	}
	bt.Frame()
	if len(binary.mergeWindows) != 0 || !strings.Contains(binary.err, "UTF-8 required") || !bt.HasText("错误") {
		t.Fatalf("binary: windows %d err %s", len(binary.mergeWindows), binary.err)
	}
}

func TestIgnoreSpaceRuleLeavesFiles(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left.txt")
	right := filepath.Join(dir, "right.txt")
	if err := os.WriteFile(left, []byte("a b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("ab\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := &workspaceState{paths: [2]string{left, right}}
	a.load()
	if a.err != "" {
		t.Fatal(a.err)
	}
	beforeLeft, _ := os.ReadFile(left)
	beforeRight, _ := os.ReadFile(right)
	a.optionsOpen = true
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()
	if err := tt.Click("忽略空白"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.message != "按规则相同" || !tt.HasText("按规则相同") {
		t.Fatalf("message: %s", a.message)
	}
	afterLeft, _ := os.ReadFile(left)
	afterRight, _ := os.ReadFile(right)
	if string(afterLeft) != string(beforeLeft) || string(afterRight) != string(beforeRight) {
		t.Fatal("ignore-space wrote the files")
	}
}

func TestAnchorAlignsInTheView(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left.txt")
	right := filepath.Join(dir, "right.txt")
	if err := os.WriteFile(left, []byte("keep\nmove\ntail\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("move\nkeep\ntail\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := &workspaceState{paths: [2]string{left, right}}
	a.load()
	if a.err != "" {
		t.Fatal(a.err)
	}
	if a.panes[0].buffer.Line(0) != "keep" || a.panes[1].buffer.Line(1) != "keep" {
		t.Fatalf("lines: %q %q", a.panes[0].buffer.Line(0), a.panes[1].buffer.Line(1))
	}
	beforeLeft, _ := os.ReadFile(left)
	beforeRight, _ := os.ReadFile(right)
	a.optionsOpen = true
	tt := ui.NewTester(a.view, 1600, 880)
	tt.Frame()
	if !hunkCovers(a.diff.Hunks(), 0) {
		t.Fatalf("unaligned first line was not a change: %+v", a.diff.Hunks())
	}
	if err := tt.Click("Old text"); err != nil {
		t.Fatal(err)
	}
	a.diff.Run(editor.MoveDocStart)
	if err := tt.Click("标记左侧"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("New text"); err != nil {
		t.Fatal(err)
	}
	a.diff.Run(editor.MoveDocStart)
	a.diff.Run(editor.MoveDown)
	if err := tt.Click("标记右侧"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.anchorLeft != 0 || a.anchorRight != 1 || !tt.HasText("已标记左侧第 1 行、右侧第 2 行") {
		t.Fatalf("marks: left %d right %d message %s err %s", a.anchorLeft, a.anchorRight, a.message, a.err)
	}
	if err := tt.Click("在此对齐"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.err != "" || hunkCovers(a.diff.Hunks(), 0) || !tt.HasText("已对齐 1 处") {
		t.Fatalf("anchored: err %s hunks %+v message %s", a.err, a.diff.Hunks(), a.message)
	}
	if err := tt.Click("Old text"); err != nil {
		t.Fatal(err)
	}
	a.diff.Run(editor.MoveDocStart)
	a.diff.Run(editor.MoveDown)
	if err := tt.Click("标记左侧"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("New text"); err != nil {
		t.Fatal(err)
	}
	a.diff.Run(editor.MoveDocStart)
	if err := tt.Click("标记右侧"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("在此对齐"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !strings.Contains(a.err, "对齐位置必须按顺序") || hunkCovers(a.diff.Hunks(), 0) || !tt.HasText("错误") {
		t.Fatalf("out of order: err %q hunks %+v", a.err, a.diff.Hunks())
	}
	if err := tt.Click("解除对齐"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(a.anchors) != 0 || !hunkCovers(a.diff.Hunks(), 0) || !tt.HasText("比较完成") {
		t.Fatalf("released: anchors %d hunks %+v message %s", len(a.anchors), a.diff.Hunks(), a.message)
	}
	afterLeft, _ := os.ReadFile(left)
	afterRight, _ := os.ReadFile(right)
	if string(afterLeft) != string(beforeLeft) || string(afterRight) != string(beforeRight) {
		t.Fatal("alignment wrote the files")
	}
}

func TestNavigateAndApplyBothDirections(t *testing.T) {
	applyVia := func(t *testing.T, button, wantLeft, wantRight string) {
		t.Helper()
		dir := t.TempDir()
		left := filepath.Join(dir, "left.txt")
		right := filepath.Join(dir, "right.txt")
		const leftText = "same\nleft only\n"
		const rightText = "same\nright only\n"
		if err := os.WriteFile(left, []byte(leftText), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(right, []byte(rightText), 0o600); err != nil {
			t.Fatal(err)
		}
		a := &workspaceState{paths: [2]string{left, right}}
		a.load()
		if a.err != "" {
			t.Fatal(a.err)
		}
		tt := ui.NewTester(a.view, 1600, 880)
		tt.Frame()
		if err := tt.Click("下一差异"); err != nil {
			t.Fatal(err)
		}
		if err := tt.Click("上一差异"); err != nil {
			t.Fatal(err)
		}
		if err := tt.Click("下一差异"); err != nil {
			t.Fatal(err)
		}
		if err := tt.Click(button); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if a.panes[0].buffer.String() != wantLeft || a.panes[1].buffer.String() != wantRight {
			t.Fatalf("%s: left %q right %q message %s err %s", button, a.panes[0].buffer.String(), a.panes[1].buffer.String(), a.message, a.err)
		}
		gotLeft, _ := os.ReadFile(left)
		gotRight, _ := os.ReadFile(right)
		if string(gotLeft) != leftText || string(gotRight) != rightText {
			t.Fatal("apply wrote the files before save")
		}
	}
	applyVia(t, "应用到右侧 →", "same\nleft only\n", "same\nleft only\n")
	applyVia(t, "← 应用到左侧", "same\nright only\n", "same\nright only\n")
}

func hunkCovers(hunks []editor.DiffHunk, line int) bool {
	for _, h := range hunks {
		if line >= h.OldStart && line < h.OldEnd {
			return true
		}
	}
	return false
}

func TestSyntaxInlineAndDetectedText(t *testing.T) {
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.go")
	rightPath := filepath.Join(dir, "right.go")
	left := append([]byte("package main\n\nfunc main() {\n\treturn alpha\n}\n// "), 0xd6, 0xd0, 0xce, 0xc4, '\n')
	right := append([]byte("package main\n\nfunc main() {\n\treturn beta\n}\n// "), 0xd6, 0xd0, 0xce, 0xc4, '\n')
	if err := os.WriteFile(leftPath, left, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rightPath, right, 0o600); err != nil {
		t.Fatal(err)
	}
	a := &workspaceState{paths: [2]string{leftPath, rightPath}}
	a.load()
	if a.err != "" {
		t.Fatal(a.err)
	}
	if a.panes[0].file.Encoding != "gbk" || !strings.Contains(a.panes[0].buffer.String(), "中文") || !strings.Contains(a.panes[1].buffer.String(), "中文") {
		t.Fatalf("detected: %s %q", a.panes[0].file.Encoding, a.panes[0].buffer.String())
	}
	lang := a.panes[0].buffer.Language()
	if lang == nil || lang.Name != "Go" || lang.Tokenizer == nil {
		t.Fatal("go language was not attached")
	}
	funcLine := a.panes[0].buffer.Line(2)
	tokens, _ := lang.Tokenizer.Tokenize(funcLine, 0, nil)
	if kindAt(tokens, funcLine, "func") != editor.TokenKeyword {
		t.Fatalf("func token in %q: %+v", funcLine, tokens)
	}
	comment := a.panes[0].buffer.Line(5)
	tokens, _ = lang.Tokenizer.Tokenize(comment, 0, nil)
	if kindAt(tokens, comment, "//") != editor.TokenComment {
		t.Fatalf("comment token in %q: %+v", comment, tokens)
	}
	hunks := a.diff.Hunks()
	if len(hunks) != 1 || hunks[0] != (editor.DiffHunk{OldStart: 3, OldEnd: 4, NewStart: 3, NewEnd: 4}) {
		t.Fatalf("hunks %+v", hunks)
	}
	tt := ui.NewTester(a.view, 1200, 480)
	tt.Frame()
	if !tt.HasText("GBK / LF") {
		t.Fatalf("encoding label missing: %v", tt.Texts())
	}
	// 先切到无行内高亮并完成一次重算，再切回默认行内模式。两次画面的状态文字相同，像素差只来自行内高亮。
	a.diff.Inline = editor.DiffInlineNone
	tt.Frame()
	off := cloneRGBA(tt.Image())
	a.diff.Inline = editor.DiffInlineWordAlt
	tt.Frame()
	leftN, leftW, leftH := diffSpan(tt.Image(), off, 0, off.Bounds().Dx()/2)
	rightN, rightW, rightH := diffSpan(tt.Image(), off, off.Bounds().Dx()/2, off.Bounds().Dx())
	limitW, limitH := off.Bounds().Dx()/6, 40
	if leftN == 0 || rightN == 0 || leftW > limitW || rightW > limitW || leftH > limitH || rightH > limitH {
		t.Fatalf("inline highlight left %dx%d (%d) right %dx%d (%d)", leftW, leftH, leftN, rightW, rightH, rightN)
	}
}

func kindAt(tokens []editor.Token, line, word string) editor.TokenKind {
	start := strings.Index(line, word)
	if start < 0 {
		return 255
	}
	end := start + len(word)
	from := 0
	for _, tok := range tokens {
		if start >= from && end <= tok.End {
			return tok.Kind
		}
		from = tok.End
	}
	return 255
}

func cloneRGBA(src *image.RGBA) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	copy(dst.Pix, src.Pix)
	return dst
}

func diffSpan(a, b *image.RGBA, x0, x1 int) (n, w, h int) {
	if !a.Bounds().Eq(b.Bounds()) {
		return 0, 0, 0
	}
	minX, minY := x1, a.Bounds().Max.Y
	maxX, maxY := x0-1, -1
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := x0; x < x1; x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				n++
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if n == 0 {
		return 0, 0, 0
	}
	return n, maxX - minX + 1, maxY - minY + 1
}

func TestNonWindowsHidesLiveRegistry(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows lists platform actions")
	}
	if integrate.PlatformActions() != nil {
		t.Fatal("platform actions")
	}
	a := &workspaceState{}
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()
	err := tt.Click("实时注册表")
	if err == nil || !strings.Contains(err.Error(), "实时注册表") {
		t.Fatalf("live registry visible: %v", err)
	}
	if tt.HasText("实时注册表") || tt.HasText("MSSCCI") {
		t.Fatal("windows-only action is shown")
	}
}
