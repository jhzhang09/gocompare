package formats_test

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"testing"
	"unicode/utf16"

	"gocompare/internal/doctext"
	"gocompare/internal/htmlview"
	"gocompare/internal/media"
	"gocompare/internal/pever"
	"gocompare/internal/regfile"
)

func TestID3Title(t *testing.T) {
	payload := append([]byte{0}, []byte("Hello")...)
	frame := make([]byte, 10+len(payload))
	copy(frame[0:4], "TIT2")
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(payload)))
	copy(frame[10:], payload)
	data := make([]byte, 10+len(frame))
	copy(data[0:3], "ID3")
	data[3] = 3
	putSynchsafe(data[6:10], len(frame))
	copy(data[10:], frame)
	got, err := media.ID3v2Title(data)
	if err != nil || got.Title != "Hello" || got.Artist != "" || got.Album != "" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestID3ArtistAndAlbum(t *testing.T) {
	title := textFrame("TIT2", 0, []byte("LeftSong\x00"))
	artist := textFrame("TPE1", 1, utf16LE("甲"))
	album := textFrame("TALB", 0, []byte("LeftAlbum\x00"))
	data := id3Tag(append(append(title, artist...), album...))
	got, err := media.ReadID3v2(data)
	if err != nil || got.Title != "LeftSong" || got.Artist != "甲" || got.Album != "LeftAlbum" {
		t.Fatalf("%+v %v", got, err)
	}
	viaTitle, err := media.ID3v2Title(data)
	if err != nil || viaTitle != got {
		t.Fatalf("title path %+v %v", viaTitle, err)
	}
	onlyArtist := id3Tag(textFrame("TPE1", 1, utf16LE("乙")))
	if _, err = media.ID3v2Title(onlyArtist); err == nil || err.Error() != "TIT2 frame not found" {
		t.Fatalf("missing TIT2: %v", err)
	}
	read, err := media.ReadID3v2(onlyArtist)
	if err != nil || read.Artist != "乙" || read.Title != "" {
		t.Fatalf("artist only %+v %v", read, err)
	}
}

func textFrame(id string, enc byte, text []byte) []byte {
	payload := append([]byte{enc}, text...)
	frame := make([]byte, 10+len(payload))
	copy(frame[0:4], id)
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(payload)))
	copy(frame[10:], payload)
	return frame
}

func id3Tag(frames []byte) []byte {
	data := make([]byte, 10+len(frames))
	copy(data[0:3], "ID3")
	data[3] = 3
	putSynchsafe(data[6:10], len(frames))
	copy(data[10:], frames)
	return data
}

func utf16LE(s string) []byte {
	u := utf16.Encode([]rune(s))
	b := []byte{0xff, 0xfe}
	for _, v := range u {
		b = append(b, byte(v), byte(v>>8))
	}
	return append(b, 0, 0)
}

func putSynchsafe(b []byte, n int) {
	b[0] = byte(n >> 21 & 0x7f)
	b[1] = byte(n >> 14 & 0x7f)
	b[2] = byte(n >> 7 & 0x7f)
	b[3] = byte(n & 0x7f)
}

func TestRegistryDiff(t *testing.T) {
	left := "Windows Registry Editor Version 5.00\n\n[HKEY_CURRENT_USER\\Software\\GoCompare]\n\"Name\"=\"One\"\n"
	right := "Windows Registry Editor Version 5.00\n\n[HKEY_CURRENT_USER\\Software\\GoCompare]\n\"Name\"=\"Two\"\n"
	a, err := regfile.Parse(left)
	if err != nil {
		t.Fatal(err)
	}
	b, err := regfile.Parse(right)
	if err != nil {
		t.Fatal(err)
	}
	diffs := regfile.Diff(a, b)
	if len(diffs) != 1 || diffs[0] != `HKEY_CURRENT_USER\Software\GoCompare\Name` {
		t.Fatalf("%v", diffs)
	}
}

func TestDocumentConversionAndHTMLBlocks(t *testing.T) {
	docx := zipBytes(t, map[string]string{
		"word/document.xml": `<document><body><p><r><t>Hello</t></r></p><p><r><t>Word</t></r></p></body></document>`,
	})
	text, err := doctext.DOCX(docx)
	if err != nil || text != "Hello\nWord" {
		t.Fatalf("docx: %q %v", text, err)
	}
	rtf, err := doctext.RTF([]byte("{\\rtf1 Hello \\par World \\'e9}"))
	if err != nil || rtf != "Hello World \u00e9" {
		t.Fatalf("rtf: %q %v", rtf, err)
	}
	pdfText, err := doctext.PDF([]byte("%PDF-1.1\n1 0 obj << /Length 15 >> stream\n(Hello PDF) Tj\nendstream\n"))
	if err != nil || pdfText != "Hello PDF" {
		t.Fatalf("pdf: %q %v", pdfText, err)
	}
	blocks := htmlview.Blocks("<h1>Title</h1><p>Body <b>x</b></p>")
	if len(blocks) != 2 || blocks[0].Text != "Title" || blocks[1].Text != "Body x" {
		t.Fatalf("blocks: %+v", blocks)
	}
	var report bytes.Buffer
	if err = htmlview.WriteReport(&report, "Report", blocks); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(report.Bytes(), []byte("Body x")) || bytes.Contains(report.Bytes(), []byte("webview")) {
		t.Fatalf("report: %s", report.Bytes())
	}
}

func TestPEFileVersion(t *testing.T) {
	data := peWithVersion("1.2.3.4")
	got, err := pever.FileVersion(data)
	if err != nil || got != "1.2.3.4" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestPEFileVersionAlignedString(t *testing.T) {
	data := peWithVersionBlob(alignedVersionString("1.2.3.4"))
	got, err := pever.FileVersion(data)
	if err != nil || got != "1.2.3.4" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestPEWithoutRTVersion(t *testing.T) {
	// 模拟类似 NSIS 安装包的 PE：有 .rsrc 节和 RT_ICON (3)，但没有 RT_VERSION (16)
	const blobAt = 88
	blob := []byte{1, 2, 3, 4}
	sec := make([]byte, blobAt+len(blob))
	putDir(sec, 0, 3, 24, true)
	putDir(sec, 24, 1, 48, true)
	putDir(sec, 48, 0, 72, false)
	const sectionVA = 0x1000
	binary.LittleEndian.PutUint32(sec[72:], uint32(sectionVA+blobAt))
	binary.LittleEndian.PutUint32(sec[76:], uint32(len(blob)))
	copy(sec[blobAt:], blob)
	file := make([]byte, 0x80+len(sec))
	file[0], file[1] = 'M', 'Z'
	binary.LittleEndian.PutUint32(file[0x3c:], 0x40)
	copy(file[0x40:], []byte{'P', 'E', 0, 0})
	binary.LittleEndian.PutUint16(file[0x40+4:], 0xaa64) // arm64
	binary.LittleEndian.PutUint16(file[0x40+4+2:], 1)
	off := 0x40 + 4 + 20
	copy(file[off:off+8], []byte(".rsrc"))
	binary.LittleEndian.PutUint32(file[off+8:], uint32(len(sec)))
	binary.LittleEndian.PutUint32(file[off+12:], sectionVA)
	binary.LittleEndian.PutUint32(file[off+16:], uint32(len(sec)))
	binary.LittleEndian.PutUint32(file[off+20:], 0x80)
	copy(file[0x80:], sec)

	got, err := pever.FileVersion(file)
	if err != nil {
		t.Fatalf("unexpected error for PE without RT_VERSION: %v", err)
	}
	if !bytes.HasPrefix([]byte(got), []byte("PE/arm64 (")) {
		t.Fatalf("expected PE/arm64 fallback, got %q", got)
	}
}

func peWithVersion(version string) []byte {
	return peWithVersionBlob(append(utf16z("FileVersion"), utf16z(version)...))
}

// alignedVersionString 是 DWORD 对齐的 String：6 字节头、UTF-16 键、填充 0、值。
// wValueLength 按字计算，并包含结尾 NUL。
func alignedVersionString(version string) []byte {
	key := utf16z("FileVersion")
	value := utf16z(version)
	body := make([]byte, 6+len(key))
	copy(body[6:], key)
	for len(body)%4 != 0 {
		body = append(body, 0)
	}
	body = append(body, value...)
	binary.LittleEndian.PutUint16(body[0:], uint16(len(body)))
	binary.LittleEndian.PutUint16(body[2:], uint16(len(value)/2))
	binary.LittleEndian.PutUint16(body[4:], 1)
	return body
}

func peWithVersionBlob(blob []byte) []byte {
	// 三级目录各占 24 字节（头 16 + 一条 8 字节项），数据入口 16 字节必须落在目录项之后、版本块之前。
	const blobAt = 88
	sec := make([]byte, blobAt+len(blob))
	putDir(sec, 0, 16, 24, true)
	putDir(sec, 24, 1, 48, true)
	putDir(sec, 48, 0, 72, false)
	const sectionVA = 0x1000
	binary.LittleEndian.PutUint32(sec[72:], uint32(sectionVA+blobAt))
	binary.LittleEndian.PutUint32(sec[76:], uint32(len(blob)))
	copy(sec[blobAt:], blob)
	file := make([]byte, 0x80+len(sec))
	file[0], file[1] = 'M', 'Z'
	binary.LittleEndian.PutUint32(file[0x3c:], 0x40)
	copy(file[0x40:], []byte{'P', 'E', 0, 0})
	binary.LittleEndian.PutUint16(file[0x40+4+2:], 1)
	off := 0x40 + 4 + 20
	copy(file[off:off+8], []byte(".rsrc"))
	binary.LittleEndian.PutUint32(file[off+8:], uint32(len(sec)))
	binary.LittleEndian.PutUint32(file[off+12:], sectionVA)
	binary.LittleEndian.PutUint32(file[off+16:], uint32(len(sec)))
	binary.LittleEndian.PutUint32(file[off+20:], 0x80)
	copy(file[0x80:], sec)
	return file
}

func putDir(b []byte, off, id, target int, sub bool) {
	binary.LittleEndian.PutUint16(b[off+14:], 1)
	binary.LittleEndian.PutUint32(b[off+16:], uint32(id))
	rel := uint32(target)
	if sub {
		rel |= 0x80000000
	}
	binary.LittleEndian.PutUint32(b[off+20:], rel)
}

func utf16z(s string) []byte {
	u := utf16.Encode([]rune(s))
	out := make([]byte, (len(u)+1)*2)
	for i, v := range u {
		binary.LittleEndian.PutUint16(out[i*2:], v)
	}
	return out
}

func zipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
