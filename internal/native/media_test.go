package native

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/egoist/mygo/ui"
)

func TestWorkspaceMediaTags(t *testing.T) {
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.bin")
	rightPath := filepath.Join(dir, "right.bin")
	writeWorkspaceID3(t, leftPath, "LeftSong", "甲", "LeftAlbum")
	writeWorkspaceID3(t, rightPath, "RightSong", "乙", "RightAlbum")
	beforeLeft, err := os.ReadFile(leftPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeRight, err := os.ReadFile(rightPath)
	if err != nil {
		t.Fatal(err)
	}
	a := &workspaceState{paths: [2]string{leftPath, rightPath}}
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()
	if err = tt.Click("媒体标签"); err != nil {
		t.Fatal(err)
	}
	if err = tt.Click("开始比较"); err != nil {
		t.Fatal(err)
	}
	if a.err != "" || a.kind != 4 || !a.mediaReady || a.panes[0] != nil {
		t.Fatalf("kind=%d ready=%v panes=%v err=%s", a.kind, a.mediaReady, a.panes[0] != nil, a.err)
	}
	if a.mediaLeft.Title != "LeftSong" || a.mediaLeft.Artist != "甲" || a.mediaLeft.Album != "LeftAlbum" {
		t.Fatalf("left %+v", a.mediaLeft)
	}
	if a.mediaRight.Title != "RightSong" || a.mediaRight.Artist != "乙" || a.mediaRight.Album != "RightAlbum" {
		t.Fatalf("right %+v", a.mediaRight)
	}
	for _, text := range []string{"左侧标题 LeftSong", "左侧艺术家 甲", "左侧专辑 LeftAlbum", "右侧标题 RightSong", "右侧艺术家 乙", "右侧专辑 RightAlbum", "不能播放"} {
		if !tt.HasText(text) {
			t.Fatalf("missing %q in %v", text, tt.Texts())
		}
	}
	if _, ok := tt.Find("播放"); ok {
		t.Fatal("media view has a play control")
	}
	tt.Advance(time.Second)
	if a.mediaLeft.Title != "LeftSong" || !tt.HasText("左侧标题 LeftSong") || !tt.HasText("不能播放") {
		t.Fatal("advancing time changed the tags")
	}
	if _, ok := tt.Find("播放"); ok {
		t.Fatal("advance revealed a play control")
	}
	afterLeft, err := os.ReadFile(leftPath)
	if err != nil || !bytes.Equal(beforeLeft, afterLeft) {
		t.Fatal("left bytes changed")
	}
	afterRight, err := os.ReadFile(rightPath)
	if err != nil || !bytes.Equal(beforeRight, afterRight) {
		t.Fatal("right bytes changed")
	}
}

func TestWorkspaceMediaRejectsText(t *testing.T) {
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "note.txt")
	rightPath := filepath.Join(dir, "other.txt")
	if err := os.WriteFile(leftPath, []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rightPath, []byte("world\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(leftPath)
	if err != nil {
		t.Fatal(err)
	}
	a := &workspaceState{paths: [2]string{leftPath, rightPath}}
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()
	if err = tt.Click("媒体标签"); err != nil {
		t.Fatal(err)
	}
	if err = tt.Click("开始比较"); err != nil {
		t.Fatal(err)
	}
	if a.kind != 4 || a.mediaReady || a.panes[0] != nil || a.err != "not an id3v2 header" {
		t.Fatalf("kind=%d ready=%v err=%s", a.kind, a.mediaReady, a.err)
	}
	if !tt.HasText("错误") || tt.HasText("左侧标题") || tt.HasText("左侧艺术家") || tt.HasText("左侧专辑") {
		t.Fatalf("texts %v err=%s", tt.Texts(), a.err)
	}
	if _, ok := tt.Find("播放"); ok {
		t.Fatal("rejected file shows a play control")
	}
	after, err := os.ReadFile(leftPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("rejected file changed")
	}
}

func writeWorkspaceID3(t *testing.T, path, title, artist, album string) {
	t.Helper()
	frames := append(workspaceTextFrame("TIT2", 0, append([]byte(title), 0)), workspaceTextFrame("TPE1", 1, workspaceUTF16(artist))...)
	frames = append(frames, workspaceTextFrame("TALB", 0, append([]byte(album), 0))...)
	data := make([]byte, 10+len(frames))
	copy(data[0:3], "ID3")
	data[3] = 3
	data[6] = byte(len(frames) >> 21 & 0x7f)
	data[7] = byte(len(frames) >> 14 & 0x7f)
	data[8] = byte(len(frames) >> 7 & 0x7f)
	data[9] = byte(len(frames) & 0x7f)
	copy(data[10:], frames)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func workspaceTextFrame(id string, enc byte, text []byte) []byte {
	payload := append([]byte{enc}, text...)
	frame := make([]byte, 10+len(payload))
	copy(frame[0:4], id)
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(payload)))
	copy(frame[10:], payload)
	return frame
}

func workspaceUTF16(s string) []byte {
	u := utf16.Encode([]rune(s))
	b := []byte{0xff, 0xfe}
	for _, v := range u {
		b = append(b, byte(v), byte(v>>8))
	}
	return append(b, 0, 0)
}
