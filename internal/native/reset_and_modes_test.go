package native

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"gocompare/internal/i18n"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestModeSpecificEmptyViews(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	a := &workspaceState{}
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()

	// 1. Initial mode is File Compare (kind 0)
	if !tt.HasText("双栏代码/文本 · 词级高亮对比") {
		t.Fatalf("expected file compare intro tag, got texts: %v", tt.Texts())
	}
	if !tt.HasText("行内词级对比") || !tt.HasText("双向安全编辑") {
		t.Fatalf("expected file compare feature highlights")
	}
	// Must NOT have directory compare intro tag
	if tt.HasText("目录树结构 · 递归比对与安全同步") {
		t.Fatalf("file compare should not show directory compare tag")
	}

	// 2. Switch to Directory Compare (kind 1)
	if err := tt.Click("目录比较"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("目录树结构 · 递归比对与安全同步") {
		t.Fatalf("expected directory compare intro tag, got texts: %v", tt.Texts())
	}
	if !tt.HasText("全树递归扫描") || !tt.HasText("安全同步与回滚") {
		t.Fatalf("expected directory compare feature highlights")
	}
	// Must NOT have file compare intro tag
	if tt.HasText("双栏代码/文本 · 词级高亮对比") {
		t.Fatalf("directory compare should not show file compare tag")
	}

	// 3. Switch to Text Merge (kind 2)
	if err := tt.Click("文本合并"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("三方/两方合并 · 智能冲突决策") {
		t.Fatalf("expected merge intro tag, got texts: %v", tt.Texts())
	}

	// 4. Switch to Picture Compare (kind 3)
	if err := tt.Click("图片比较"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("像素级图像比对 · 视觉差异可视化") {
		t.Fatalf("expected picture intro tag, got texts: %v", tt.Texts())
	}

	// 5. Switch to Media Tags (kind 4)
	if err := tt.Click("媒体标签"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("音频 ID3v2 标签 · 元数据结构比对") {
		t.Fatalf("expected media intro tag, got texts: %v", tt.Texts())
	}

	// 6. Switch to Version & Registry (kind 5)
	if err := tt.Click("版本与注册表"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("PE 文件版本资源与注册表导出对比") {
		t.Fatalf("expected version intro tag, got texts: %v", tt.Texts())
	}
}

func TestResetButtonInAllModes(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	dir := t.TempDir()

	// 1. Test File Compare Reset
	p1 := filepath.Join(dir, "f1.txt")
	p2 := filepath.Join(dir, "f2.txt")
	writeTestFile(t, p1, "left file content\n")
	writeTestFile(t, p2, "right file content\n")

	a := &workspaceState{paths: [2]string{p1, p2}}
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()

	if err := tt.Click("开始比较"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.panes[0] == nil || a.panes[1] == nil {
		t.Fatal("file compare not loaded")
	}

	// Click 重置
	if err := tt.Click("重置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	if a.panes[0] != nil || a.panes[1] != nil {
		t.Fatalf("expected panes cleared after reset")
	}
	if a.paths[0] != "" || a.paths[1] != "" {
		t.Fatalf("expected paths cleared after reset")
	}
	if !tt.HasText("已重置当前比较") {
		t.Fatalf("expected '已重置当前比较' status, got texts: %v", tt.Texts())
	}
	if !tt.HasText("双栏代码/文本 · 词级高亮对比") {
		t.Fatalf("expected file compare empty guide after reset")
	}

	// 2. Test Directory Compare Reset
	d1 := filepath.Join(dir, "dirA")
	d2 := filepath.Join(dir, "dirB")
	writeTestFile(t, filepath.Join(d1, "a.txt"), "left")
	writeTestFile(t, filepath.Join(d2, "a.txt"), "right")

	a.paths = [2]string{d1, d2}
	if err := tt.Click("目录比较"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("开始比较"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.directoryRows == nil {
		t.Fatal("directory compare not loaded")
	}

	// Click 重置 in directory compare
	if err := tt.Click("重置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.directoryRows != nil {
		t.Fatalf("expected directoryRows cleared after reset")
	}
	if a.paths[0] != "" || a.paths[1] != "" {
		t.Fatalf("expected directory paths cleared after reset")
	}
	if !tt.HasText("目录树结构 · 递归比对与安全同步") {
		t.Fatalf("expected directory compare empty guide after reset")
	}

	// 3. Test Scratch Text Compare Reset
	if err := tt.Click("文本比较"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.scratchPanes[0].buffer.Set("custom scratch left")
	a.scratchPanes[1].buffer.Set("custom scratch right")
	a.refreshComparison()
	tt.Frame()

	if a.scratchPanes[0].buffer.String() != "custom scratch left" {
		t.Fatal("scratch text not set")
	}

	// Click 重置 in scratch text compare
	if err := tt.Click("重置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	if a.scratchPanes[0].buffer.Len() != 0 || a.scratchPanes[1].buffer.Len() != 0 {
		t.Fatalf("expected scratch text buffers cleared after reset")
	}
	if !tt.HasText("已重置当前比较") {
		t.Fatalf("expected '已重置当前比较' after scratch reset")
	}

	// 4. Test Picture Compare Reset
	pic1 := filepath.Join(dir, "p1.png")
	pic2 := filepath.Join(dir, "p2.png")
	writeTestFile(t, pic1, "dummy")
	writeTestFile(t, pic2, "dummy")
	a.paths = [2]string{pic1, pic2}
	if err := tt.Click("图片比较"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// Click reset
	if err := tt.Click("重置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.paths[0] != "" || a.paths[1] != "" {
		t.Fatalf("expected picture paths cleared")
	}
	if !tt.HasText("像素级图像比对 · 视觉差异可视化") {
		t.Fatalf("expected picture empty view after reset")
	}
}

func TestScratchResetKeepsEditableComparison(t *testing.T) {
	a := &workspaceState{}
	a.selectKind(kindScratch)
	a.ensureScratch()
	a.scratchPanes[0].buffer.Set("x\n")
	a.resetCurrentMode()
	d := a.activeDiff()
	if d.Old == nil || d.New == nil || !d.ExternalComparison || d.Old != a.scratchPanes[0].buffer {
		t.Fatal("scratch comparison lost its buffers after reset")
	}
	if a.scratchPanes[0].buffer.Len() != 0 || a.scratchUnsaved() {
		t.Fatal("reset must leave empty, clean scratch panes")
	}
}
