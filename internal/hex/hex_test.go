package hex

import (
	"bytes"
	"testing"
)

func TestDiffAndEditableBuffer(t *testing.T) {
	left := bytes.Repeat([]byte{1}, 20)
	right := append([]byte(nil), left...)
	right[16] = 2
	chunks := Diff(left, right, 16)
	if len(chunks) != 1 || chunks[0].Offset != 16 || len(chunks[0].Left) != 4 || chunks[0].Right[0] != 2 {
		t.Fatalf("chunks: %+v", chunks)
	}
	buf := New([]byte("abcdef"))
	if buf.Search([]byte("cd")) != 2 {
		t.Fatal(buf.Search([]byte("cd")))
	}
	buf.Insert(2, []byte("XY"))
	if !bytes.Equal(buf.Bytes(), []byte("abXYcdef")) {
		t.Fatalf("insert: %q", buf.Bytes())
	}
	buf.Delete(2, 2)
	if !bytes.Equal(buf.Bytes(), []byte("abcdef")) {
		t.Fatalf("delete: %q", buf.Bytes())
	}
	if !buf.Undo() || !bytes.Equal(buf.Bytes(), []byte("abXYcdef")) {
		t.Fatalf("undo: %q", buf.Bytes())
	}
	if buf.Search(nil) != -1 {
		t.Fatal("empty search")
	}
}
