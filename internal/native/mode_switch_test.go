package native

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"gocompare/internal/compare"
	"gocompare/internal/document"
	"gocompare/internal/i18n"
)

func TestModeSwitchWithLoadedData(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	dir := t.TempDir()
	p1 := filepath.Join(dir, "left.txt")
	p2 := filepath.Join(dir, "right.txt")
	writeTestFile(t, p1, "left content\nline 2\n")
	writeTestFile(t, p2, "right content\nline 2\n")

	f1, err := document.Read(context.Background(), p1)
	if err != nil {
		t.Fatal(err)
	}
	f2, err := document.Read(context.Background(), p2)
	if err != nil {
		t.Fatal(err)
	}

	a := &workspaceState{
		optionsOpen: true,
		inlineDiff:  true,
		kind:        kindFile,
		paths:       [2]string{p1, p2},
		panes:       [2]*pane{newPane(f1), newPane(f2)},
	}
	a.ensureScratch()
	a.scratchPanes[0].buffer.Set("scratch left\n")
	a.scratchPanes[1].buffer.Set("scratch right\n")

	// Pre-populate directory session
	a.directoryRows = []compare.Entry{
		{Path: "file1.txt", Status: compare.Different},
		{Path: "file2.txt", Status: compare.Same},
	}
	a.saveModeSession(kindDirectory)
	a.directoryRows = nil // back to file mode state

	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()

	// Switch from File Compare to Directory Compare
	if err := tt.Click("目录比较"); err != nil {
		t.Fatalf("click 目录比较 failed: %v", err)
	}
	tt.Frame()

	// Switch from Directory Compare to Text Compare (Scratch)
	if err := tt.Click("文本比较"); err != nil {
		t.Fatalf("click 文本比较 failed: %v", err)
	}
	tt.Frame()

	// Switch from Text Compare to Directory Compare
	if err := tt.Click("目录比较"); err != nil {
		t.Fatalf("click 目录比较 from scratch failed: %v", err)
	}
	tt.Frame()

	// Switch from Directory Compare to File Compare
	if err := tt.Click("文件比较"); err != nil {
		t.Fatalf("click 文件比较 failed: %v", err)
	}
	tt.Frame()

	// Switch back to 文本比较
	if err := tt.Click("文本比较"); err != nil {
		t.Fatalf("click 文本比较 again failed: %v", err)
	}
	tt.Frame()
}

func TestModeSwitchAllModes(t *testing.T) {
	i18n.SetLanguage(i18n.ZhCN)
	a := &workspaceState{
		kind: kindFile,
	}
	a.ensureScratch()

	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()

	modeLabels := []string{
		"文本比较",
		"目录比较",
		"文件比较",
		"文本合并",
		"图片比较",
		"媒体标签",
		"版本与注册表",
		"表格比较",
	}

	// Repeatedly cycle through each mode twice
	for round := 0; round < 2; round++ {
		for _, label := range modeLabels {
			if err := tt.Click(label); err != nil {
				t.Fatalf("round %d click %s failed: %v", round, label, err)
			}
			tt.Frame()
		}
	}

	// Rapidly toggle between "文本比较" and "目录比较"
	for i := 0; i < 5; i++ {
		if err := tt.Click("文本比较"); err != nil {
			t.Fatalf("rapid toggle scratch %d failed: %v", i, err)
		}
		tt.Frame()
		if err := tt.Click("目录比较"); err != nil {
			t.Fatalf("rapid toggle dir %d failed: %v", i, err)
		}
		tt.Frame()
	}

	// Open options and toggle again
	a.optionsOpen = true
	tt.Frame()
	for i := 0; i < 3; i++ {
		if err := tt.Click("文本比较"); err != nil {
			t.Fatalf("options open toggle scratch %d failed: %v", i, err)
		}
		tt.Frame()
		if err := tt.Click("目录比较"); err != nil {
			t.Fatalf("options open toggle dir %d failed: %v", i, err)
		}
		tt.Frame()
		if err := tt.Click("文件比较"); err != nil {
			t.Fatalf("options open toggle file %d failed: %v", i, err)
		}
		tt.Frame()
	}
}
