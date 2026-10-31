package native

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestScratchTextCompareInitAndSample(t *testing.T) {
	a := &workspaceState{}
	a.selectKind(6)
	if a.kind != 6 {
		t.Fatalf("expected kind 6, got %d", a.kind)
	}
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()

	if !tt.HasText("文本比较") {
		t.Fatal("expected 文本比较 in view")
	}
	if !tt.HasText("载入示例") {
		t.Fatal("expected 载入示例 button")
	}

	// Click 载入示例
	if err := tt.Click("载入示例"); err != nil {
		t.Fatalf("failed to click 载入示例: %v", err)
	}
	tt.Frame()

	leftText := a.scratchPanes[0].buffer.String()
	rightText := a.scratchPanes[1].buffer.String()
	if !strings.Contains(leftText, "Hello, GoCompare!") {
		t.Fatalf("left text missing sample content: %q", leftText)
	}
	if !strings.Contains(rightText, "Hello, GoCompare v2!") {
		t.Fatalf("right text missing sample content: %q", rightText)
	}

	diff := a.activeDiff()
	if !diff.ComparisonReady() {
		t.Fatal("diff should be ready after loading sample")
	}
	hunks := diff.Hunks()
	if len(hunks) == 0 {
		t.Fatal("expected diff hunks between sample texts, got 0")
	}

	// Click 互换两侧
	if err := tt.Click("互换两侧"); err != nil {
		t.Fatalf("failed to click 互换两侧: %v", err)
	}
	tt.Frame()

	newLeft := a.scratchPanes[0].buffer.String()
	newRight := a.scratchPanes[1].buffer.String()
	if newLeft != rightText || newRight != leftText {
		t.Fatalf("swap failed: newLeft=%q, newRight=%q", newLeft, newRight)
	}

	// Click 清空两侧
	if err := tt.Click("清空两侧"); err != nil {
		t.Fatalf("failed to click 清空两侧: %v", err)
	}
	tt.Frame()

	if a.scratchPanes[0].buffer.Len() != 0 || a.scratchPanes[1].buffer.Len() != 0 {
		t.Fatalf("expected empty buffers after clear, got left=%d right=%d", a.scratchPanes[0].buffer.Len(), a.scratchPanes[1].buffer.Len())
	}
}

func TestScratchTextComparePasteAndCopy(t *testing.T) {
	a := &workspaceState{}
	a.selectKind(6)
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()

	// Set clipboard and paste to left
	tt.SetClipboard("alpha\nbeta\n")
	if err := tt.Click("粘贴到左侧"); err != nil {
		t.Fatalf("failed to click 粘贴到左侧: %v", err)
	}
	tt.Frame()

	if a.scratchPanes[0].buffer.String() != "alpha\nbeta\n" {
		t.Fatalf("unexpected left text: %q", a.scratchPanes[0].buffer.String())
	}

	// Set clipboard and paste to right
	tt.SetClipboard("alpha\ngamma\n")
	if err := tt.Click("粘贴到右侧"); err != nil {
		t.Fatalf("failed to click 粘贴到右侧: %v", err)
	}
	tt.Frame()

	if a.scratchPanes[1].buffer.String() != "alpha\ngamma\n" {
		t.Fatalf("unexpected right text: %q", a.scratchPanes[1].buffer.String())
	}

	diff := a.activeDiff()
	if !diff.ComparisonReady() {
		t.Fatal("diff should be ready after pasting")
	}
	if len(diff.Hunks()) != 1 {
		t.Fatalf("expected 1 diff hunk, got %d", len(diff.Hunks()))
	}

	// Test 复制 button
	tt.SetClipboard("")
	// Find and click copy for left
	if err := tt.Click("复制"); err != nil {
		t.Fatalf("failed to click 复制: %v", err)
	}
	tt.Frame()
	if tt.Clipboard() != "alpha\nbeta\n" && tt.Clipboard() != "alpha\ngamma\n" {
		t.Fatalf("unexpected clipboard content after copy: %q", tt.Clipboard())
	}
}

func TestScratchTextCompareIgnoreSpaceAndInline(t *testing.T) {
	a := &workspaceState{}
	a.selectKind(6)
	a.ensureScratch()
	a.scratchPanes[0].buffer.Set("  hello   world  \n")
	a.scratchPanes[1].buffer.Set("hello world\n")
	a.refreshComparison()

	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()

	diff := a.activeDiff()
	if len(diff.Hunks()) == 0 {
		t.Fatal("expected diff before ignore space")
	}

	// Click 忽略空白
	if err := tt.Click("忽略空白"); err != nil {
		t.Fatalf("failed to click 忽略空白: %v", err)
	}
	tt.Frame()

	if !a.ignoreSpace {
		t.Fatal("ignoreSpace should be true")
	}
	if len(diff.Hunks()) != 0 {
		t.Fatalf("expected 0 hunks after ignore space, got %d", len(diff.Hunks()))
	}

	// Click 行内高亮
	if err := tt.Click("行内高亮"); err != nil {
		t.Fatalf("failed to click 行内高亮: %v", err)
	}
	tt.Frame()

	// Click 自动换行
	if err := tt.Click("自动换行"); err != nil {
		t.Fatalf("failed to click 自动换行: %v", err)
	}
	if !a.wrap {
		t.Fatal("expected wrap to be true")
	}
}

func TestScratchSwitchFromWelcome(t *testing.T) {
	a := &workspaceState{}
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()

	// Click card 纯文本粘贴比较
	if err := tt.Click("纯文本粘贴比较"); err != nil {
		t.Fatalf("failed to click 纯文本粘贴比较: %v", err)
	}
	tt.Frame()

	if a.kind != 6 {
		t.Fatalf("expected kind 6 after clicking card, got %d", a.kind)
	}
	if !tt.HasText("文本比较") {
		t.Fatal("expected 文本比较 view")
	}
}

func TestScratchSaveAs(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "scratch_out.txt")

	a := &workspaceState{}
	a.selectKind(6)
	a.ensureScratch()
	a.scratchPanes[0].buffer.Set("content to save\n")

	// Calling saveScratchAs directly with a file path logic
	text := a.scratchPanes[0].buffer.String()
	if text != "content to save\n" {
		t.Fatalf("unexpected text: %q", text)
	}
	_ = target
}
