package document

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestConvertedDocumentCannotOverwrite(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sample.docx")
	original := writeTestDOCX(t, p, "original")
	f, e := ReadDetected(context.Background(), p)
	if e != nil {
		t.Fatal(e)
	}
	if !f.ReadOnly() {
		t.Fatal("converted document must be read-only")
	}
	ctx := context.Background()
	plain, err := EmptyAt(filepath.Join(t.TempDir(), "plain.txt"))
	if err != nil {
		t.Fatal(err)
	}
	attempts := []func() (*File, error){
		func() (*File, error) { return f.Save(ctx, "changed") },
		func() (*File, error) { return f.SaveAs(ctx, p, "changed") },
		func() (*File, error) { return f.SaveAsOverwrite(ctx, p, "changed", f) },
		func() (*File, error) { return plain.SaveAsOverwrite(ctx, p, "changed", f) },
	}
	for _, attempt := range attempts {
		if _, err := attempt(); err != ErrReadOnly {
			t.Fatalf("expected read-only error, got %v", err)
		}
	}
	b, err := os.ReadFile(p)
	if err != nil || string(b) != string(original) {
		t.Fatalf("original changed: %q %v", b, err)
	}
}

// writeTestDOCX 写出只含 word/document.xml 的最小 docx。
func writeTestDOCX(t *testing.T, path, text string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.Write([]byte("<document><body><p><r><t>" + text + "</t></r></p></body></document>")); err != nil {
		t.Fatal(err)
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestReadMergeableRejectsConvertedDocument(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sample.docx")
	writeTestDOCX(t, p, "text")
	if _, err := ReadMergeable(context.Background(), p); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("converted document must not be merged: %v", err)
	}
}
