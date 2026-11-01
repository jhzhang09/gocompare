package native

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestConvertedDocumentEditorReadOnly(t *testing.T) {
	root := t.TempDir()
	left, right := filepath.Join(root, "left.docx"), filepath.Join(root, "right.docx")
	for _, path := range []string{left, right} {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		zw := zip.NewWriter(f)
		w, _ := zw.Create("word/document.xml")
		w.Write([]byte("<document><body><p><r><t>text</t></r></p></body></document>"))
		if err = zw.Close(); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	a := &workspaceState{paths: [2]string{left, right}}
	a.load()
	if a.panes[0] == nil || a.panes[1] == nil {
		t.Fatalf("load failed: %s", a.err)
	}
	tt := ui.NewTester(a.view, 1000, 700)
	for _, dark := range []bool{false, true} {
		tt.SetDark(dark)
		for mode := 0; mode <= 2; mode++ {
			a.mode = mode
			tt.Frame()
			if mode == 0 {
				if a.diff.Editable || a.diff.OldEditable {
					t.Fatal("converted comparison is editable")
				}
			} else if !a.panes[mode-1].edit.ReadOnly {
				t.Fatal("converted document editor is editable")
			}
		}
	}
}
