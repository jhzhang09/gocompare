package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"
)

func TestZipNestedAndEmptyDirectory(t *testing.T) {
	inner, err := Write("zip", []Entry{{Name: "b.txt", Data: []byte("world")}})
	if err != nil {
		t.Fatal(err)
	}
	outer, err := Write("zip", []Entry{
		{Name: "empty/", Dir: true},
		{Name: "zero", Data: []byte{}},
		{Name: "a.txt", Data: []byte("hello")},
		{Name: "inner.zip", Data: inner},
	})
	if err != nil {
		t.Fatal(err)
	}
	ents, err := Read("zip", outer)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Entry{}
	for _, e := range ents {
		got[e.Name] = e
	}
	dir, ok := got["empty/"]
	if !ok || !dir.Dir {
		t.Fatalf("empty dir: %+v", got)
	}
	zero, ok := got["zero"]
	if !ok || zero.Dir || len(zero.Data) != 0 {
		t.Fatalf("empty file: %+v", zero)
	}
	if string(got["a.txt"].Data) != "hello" {
		t.Fatalf("a.txt: %q", got["a.txt"].Data)
	}
	nested, err := Read("zip", got["inner.zip"].Data)
	if err != nil || len(nested) != 1 || nested[0].Name != "b.txt" || string(nested[0].Data) != "world" {
		t.Fatalf("nested: %+v %v", nested, err)
	}
}

func TestTarGzReadOnlyAnd7zRejected(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "dir/", Typeflag: tar.TypeDir, Mode: 0o755}); err != nil {
		t.Fatal(err)
	}
	body := []byte("hello")
	if err := tw.WriteHeader(&tar.Header{Name: "a.txt", Mode: 0o600, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	ents, err := Read("tar.gz", buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 2 || !ents[0].Dir || ents[1].Name != "a.txt" || string(ents[1].Data) != "hello" {
		t.Fatalf("%+v", ents)
	}
	if _, err = Write("tar.gz", nil); err == nil || err.Error() != "tar.gz is read-only" {
		t.Fatalf("tar.gz write: %v", err)
	}
	if _, err = Read("7z", []byte("7z")); err == nil || err.Error() != "7z is not supported" {
		t.Fatalf("7z: %v", err)
	}
}

func TestArchiveRejectsOversizedEntry(t *testing.T) {
	huge := make([]byte, (32<<20)+1)
	z, err := Write("zip", []Entry{{Name: "huge.bin", Data: huge}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Read("zip", z); err == nil || err.Error() != "archive entry exceeds 32 MiB" {
		t.Fatalf("expected oversized zip entry error, got %v", err)
	}
}
