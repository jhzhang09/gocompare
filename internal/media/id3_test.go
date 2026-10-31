package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func putSize(b []byte, n int, syncsafe bool) {
	if syncsafe {
		b[0], b[1], b[2], b[3] = byte(n>>21&0x7f), byte(n>>14&0x7f), byte(n>>7&0x7f), byte(n&0x7f)
		return
	}
	binary.BigEndian.PutUint32(b, uint32(n))
}

func frame(version byte, id string, body []byte) []byte {
	h := make([]byte, 10, 10+len(body))
	copy(h, id)
	putSize(h[4:8], len(body), version == 4)
	return append(h, body...)
}

func tag(version, flags byte, body ...[]byte) []byte {
	all := bytes.Join(body, nil)
	h := []byte{'I', 'D', '3', version, 0, flags, 0, 0, 0, 0}
	putSize(h[6:10], len(all), true)
	return append(h, all...)
}

func TestReadID3v24AndEncodings(t *testing.T) {
	// v2.4 帧大小是同步安全整数：300 字节的封面在标题之前也不能读错位置。
	v24 := tag(4, 0, frame(4, "APIC", make([]byte, 300)), frame(4, "TIT2", append([]byte{3}, "歌"...)))
	if got, err := ReadID3v2(v24); err != nil || got.Title != "歌" {
		t.Fatalf("v2.4: %+v %v", got, err)
	}
	cases := map[string][]byte{
		"Hi":   {1, 0xfe, 0xff, 0, 'H', 0, 'i'},
		"Yo":   {2, 0, 'Y', 0, 'o'},
		"café": {0, 'c', 'a', 'f', 0xe9, 0},
	}
	for want, payload := range cases {
		if got, err := ReadID3v2(tag(3, 0, frame(3, "TIT2", payload))); err != nil || got.Title != want {
			t.Fatalf("want %q got %+v %v", want, got, err)
		}
	}
	// v2.3 扩展头：大小字段不含自身 4 字节。
	ext := append([]byte{0, 0, 0, 6}, make([]byte, 6)...)
	if got, err := ReadID3v2(tag(3, 0x40, ext, frame(3, "TIT2", []byte{3, 'E'}))); err != nil || got.Title != "E" {
		t.Fatalf("extended header: %+v %v", got, err)
	}
	if _, err := ReadID3v2(tag(2, 0, nil)); err == nil {
		t.Fatal("v2.2 must be reported as unsupported")
	}
}

// 只读开头的标签，超过 16 MiB 的音频文件照样能读出标题。
func TestOpenReadsOnlyTheTag(t *testing.T) {
	path := filepath.Join(t.TempDir(), "long.mp3")
	data := append(tag(3, 0, frame(3, "TIT2", []byte{3, 'L', 'o', 'n', 'g'})), make([]byte, 17<<20)...)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := Open(context.Background(), path); err != nil || got.Title != "Long" {
		t.Fatalf("%+v %v", got, err)
	}
}
