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

func TestCreateAndCompare(t *testing.T) {
	dir := t.TempDir()
	nodes, err := Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := Encode(nodes)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	diffs := Compare(nodes, decoded)
	if len(diffs) != 0 {
		t.Fatalf("unexpected diffs: %+v", diffs)
	}

	left := []Node{{Path: "a.txt", Size: 10}, {Path: "b.txt", Size: 20}}
	right := []Node{{Path: "a.txt", Size: 15}, {Path: "c.txt", Size: 30}}
	diffs = Compare(left, right)
	if len(diffs) != 3 {
		t.Fatalf("expected 3 diffs, got %+v", diffs)
	}
}

func TestDecodeRejectsImpossibleCount(t *testing.T) {
	data := append(append([]byte(nil), Magic...), 0xff, 0xff, 0xff, 0xff)
	if _, err := Decode(data); err == nil {
		t.Fatal("unbounded count accepted")
	}
}
