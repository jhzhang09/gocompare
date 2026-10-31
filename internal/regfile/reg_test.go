package regfile

import (
	"encoding/binary"
	"strings"
	"testing"
	"unicode/utf16"
)

func utf16le(text string) []byte {
	out := []byte{0xff, 0xfe}
	for _, u := range utf16.Encode([]rune(text)) {
		out = binary.LittleEndian.AppendUint16(out, u)
	}
	return out
}

// 注册表编辑器的默认导出：UTF-16LE、CRLF、长 hex 值用反斜杠折行，值名可含 = 与转义引号。
func TestParseRegeditExport(t *testing.T) {
	export := strings.Join([]string{
		"Windows Registry Editor Version 5.00",
		"",
		`[HKEY_CURRENT_USER\Software\GoCompare]`,
		`@="default"`,
		`"a=b"="c"`,
		`"say \"hi\""="x"`,
		`"Bin"=hex:00,01,02,\`,
		`  03,04`,
		"",
	}, "\r\n")
	text, err := Decode(utf16le(export))
	if err != nil {
		t.Fatal(err)
	}
	values, err := Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"": `"default"`, "a=b": `"c"`, `say "hi"`: `"x"`, "Bin": "hex:00,01,02,03,04"}
	if len(values) != len(want) {
		t.Fatalf("values %+v", values)
	}
	for _, v := range values {
		if want[v.Name] != v.Data {
			t.Fatalf("%q = %q, want %q", v.Name, v.Data, want[v.Name])
		}
	}
	if _, err = Decode([]byte{0x4d, 0x5a, 0x90, 0xff, 0xfe}); err == nil {
		t.Fatal("binary data must not decode as registry text")
	}
	if text, err = Decode([]byte("\xef\xbb\xbfWindows Registry Editor Version 5.00\n")); err != nil || !strings.HasPrefix(text, "Windows") {
		t.Fatalf("UTF-8 BOM: %q %v", text, err)
	}
}
