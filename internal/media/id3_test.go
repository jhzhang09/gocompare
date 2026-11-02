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

func TestFLACWAVAndID3v1(t *testing.T) {
	dir := t.TempDir()

	// 1. FLAC with Vorbis Comment
	var vc bytes.Buffer
	binary.Write(&vc, binary.LittleEndian, uint32(4))
	vc.WriteString("test")
	binary.Write(&vc, binary.LittleEndian, uint32(3))
	for _, entry := range []string{"TITLE=FlacSong", "ARTIST=FlacArtist", "DATE=2026"} {
		binary.Write(&vc, binary.LittleEndian, uint32(len(entry)))
		vc.WriteString(entry)
	}
	flacData := []byte("fLaC")
	blk := vc.Bytes()
	flacData = append(flacData, 0x84, byte(len(blk)>>16), byte(len(blk)>>8), byte(len(blk)))
	flacData = append(flacData, blk...)
	flacPath := filepath.Join(dir, "test.flac")
	if err := os.WriteFile(flacPath, flacData, 0o600); err != nil {
		t.Fatal(err)
	}
	flacTags, err := Open(context.Background(), flacPath)
	if err != nil || flacTags.Title != "FlacSong" || flacTags.Artist != "FlacArtist" || flacTags.Year != "2026" || flacTags.Format != "FLAC" {
		t.Fatalf("flac: %+v %v", flacTags, err)
	}

	// 2. ID3v1 at end of file
	v1 := make([]byte, 256)
	tail := v1[128:]
	copy(tail[0:3], "TAG")
	copy(tail[3:33], "V1Title")
	copy(tail[33:63], "V1Artist")
	copy(tail[63:93], "V1Album")
	copy(tail[93:97], "1999")
	tail[125] = 0
	tail[126] = 5
	v1Path := filepath.Join(dir, "v1.mp3")
	if err := os.WriteFile(v1Path, v1, 0o600); err != nil {
		t.Fatal(err)
	}
	v1Tags, err := Open(context.Background(), v1Path)
	if err != nil || v1Tags.Title != "V1Title" || v1Tags.Artist != "V1Artist" || v1Tags.Year != "1999" || v1Tags.Track != "5" {
		t.Fatalf("id3v1: %+v %v", v1Tags, err)
	}
}
