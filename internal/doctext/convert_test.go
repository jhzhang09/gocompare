package doctext

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"testing"
)

func TestPDFCompressedAndHex(t *testing.T) {
	var compressedStream bytes.Buffer
	zw := zlib.NewWriter(&compressedStream)
	zw.Write([]byte("(Compressed PDF Content) Tj <48656c6c6f> Tj"))
	zw.Close()

	pdfData := []byte("%PDF-1.4\n1 0 obj\n<< /Length " + string(rune(compressedStream.Len())) + " /Filter /FlateDecode >>\nstream\n" + compressedStream.String() + "\nendstream\nendobj\n")
	text, err := PDF(pdfData)
	if err != nil {
		t.Fatalf("pdf error: %v", err)
	}
	if text != "Compressed PDF ContentHello" {
		t.Fatalf("got %q, want Compressed PDF ContentHello", text)
	}
}

func TestConvertSupports(t *testing.T) {
	if !Supports("doc.docx", nil) || !Supports("test.pdf", nil) || !Supports("file.rtf", nil) {
		t.Fatal("Supports failed for extensions")
	}

	rtf := []byte("{\\rtf1 Hello \\par World}")
	text, err := Convert("sample.rtf", rtf)
	if err != nil || text != "Hello World" {
		t.Fatalf("rtf convert failed: %q %v", text, err)
	}

	docxData := buildTestDOCX(t, "Document Body Text")
	text, err = Convert("sample.docx", docxData)
	if err != nil || text != "Document Body Text" {
		t.Fatalf("docx convert failed: %q %v", text, err)
	}
}

func buildTestDOCX(t *testing.T, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	xmlData := `<document><body><p><r><t>` + content + `</t></r></p></body></document>`
	if _, err := w.Write([]byte(xmlData)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func flate(t *testing.T, s string) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
	zw.Close()
	return buf.String()
}

func TestPDFSkipsNonContentStreams(t *testing.T) {
	font := flate(t, "(GARBAGE) Tj <414243> Tj")
	image := flate(t, "(IMG) Tj")
	content := flate(t, "BT /F1 12 Tf [(Real) -250 (Text)] TJ (x) 0 0 Td ET")
	pdf := "%PDF-1.7\n" +
		"1 0 obj << /Length1 900 /Filter /FlateDecode >>\nstream\n" + font + "\nendstream\nendobj\n" +
		"2 0 obj << /Type /XObject /Subtype /Image /Filter /FlateDecode >>\nstream\n" + image + "\nendstream\nendobj\n" +
		"3 0 obj << /Length 15 /Filter /FlateDecode >>\nstream\n" + content + "\nendstream\nendobj\n"
	text, err := PDF([]byte(pdf))
	if err != nil {
		t.Fatal(err)
	}
	// "x" 后面跟的是 Td 而不是文本操作符，不能输出。
	if text != "RealText" {
		t.Fatalf("got %q, want RealText", text)
	}
}

func TestPDFStringDecoding(t *testing.T) {
	// 转义、嵌套括号、八进制 Latin-1；含控制字节的 CID 十六进制串被丢弃。
	pdf := "%PDF-1.4\n1 0 obj << /Length 9 >>\nstream\n" +
		`(a\(b\) (c) \351) Tj <0001 0002> Tj <4F 4B> Tj` + "\nendstream\n"
	text, err := PDF([]byte(pdf))
	if err != nil {
		t.Fatal(err)
	}
	if text != "a(b) (c) éOK" {
		t.Fatalf("got %q", text)
	}
}
