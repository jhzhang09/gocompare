package snapshot

import (
	"bytes"
	"testing"
)

func TestEmptyDirectoryIsNotAnEmptyFile(t *testing.T) {
	dirSnap, err := Encode([]Node{{Path: "empty", Dir: true}})
	if err != nil {
		t.Fatal(err)
	}
	fileSnap, err := Encode([]Node{{Path: "empty", Dir: false, Size: 0}})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(dirSnap, fileSnap) {
		t.Fatal("empty directory encoded like an empty file")
	}
	if len(dirSnap) == 0 || !bytes.HasPrefix(dirSnap, Magic) {
		t.Fatalf("snapshot looks like an empty file: %q", dirSnap)
	}
	if _, err = Decode(nil); err == nil {
		t.Fatal("decoded empty input")
	}
	if _, err = Decode([]byte{}); err == nil {
		t.Fatal("decoded empty file")
	}
	got, err := Decode(dirSnap)
	if err != nil || len(got) != 1 || !got[0].Dir || got[0].Path != "empty" {
		t.Fatalf("%+v %v", got, err)
	}
	files, err := Decode(fileSnap)
	if err != nil || len(files) != 1 || files[0].Dir || files[0].Size != 0 {
		t.Fatalf("%+v %v", files, err)
	}
	bare, err := Encode(nil)
	if err != nil || len(bare) == 0 || !bytes.HasPrefix(bare, Magic) {
		t.Fatalf("empty snapshot: %q %v", bare, err)
	}
}
