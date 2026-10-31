package document

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDeclaredEncodingsRoundTrip(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name     string
		encoding string
		text     string
		raw      []byte
		bom      bool
		crlf     bool
	}{
		{
			name:     "utf-16le",
			encoding: "utf-16le",
			text:     "Ab\n",
			raw:      []byte{0xff, 0xfe, 0x41, 0x00, 0x62, 0x00, 0x0a, 0x00},
			bom:      true,
		},
		{
			name:     "utf-16be",
			encoding: "utf-16be",
			text:     "Ab\n",
			raw:      []byte{0xfe, 0xff, 0x00, 0x41, 0x00, 0x62, 0x00, 0x0a},
			bom:      true,
		},
		{
			name:     "utf-16le-crlf",
			encoding: "utf-16le",
			text:     "A\n",
			raw:      []byte{0x41, 0x00, 0x0d, 0x00, 0x0a, 0x00},
			crlf:     true,
		},
		{
			name:     "gbk",
			encoding: "gbk",
			text:     "中文\n",
			raw:      []byte{0xd6, 0xd0, 0xce, 0xc4, 0x0a},
		},
		{
			name:     "latin1",
			encoding: "latin1",
			text:     "caf\u00e9\n",
			raw:      []byte{'c', 'a', 'f', 0xe9, '\n'},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sample")
			if err := os.WriteFile(path, tc.raw, 0o600); err != nil {
				t.Fatal(err)
			}
			f, err := ReadEncoded(ctx, path, tc.encoding)
			if err != nil {
				t.Fatal(err)
			}
			if f.Text != tc.text || f.Encoding != tc.encoding || f.BOM != tc.bom || f.CRLF != tc.crlf {
				t.Fatalf("decoded %+v", f)
			}
			f, err = f.Save(ctx, f.Text)
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != string(tc.raw) {
				t.Fatalf("round trip bytes: %x %v", got, err)
			}
			edited := "Z\n"
			f, err = f.Save(ctx, edited)
			if err != nil {
				t.Fatal(err)
			}
			again, err := ReadEncoded(ctx, path, tc.encoding)
			if err != nil || again.Text != edited {
				t.Fatalf("edited: %q %v", again.Text, err)
			}
			if err = os.WriteFile(path, []byte("hacked"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err = again.Save(ctx, "nope\n"); !errors.Is(err, ErrChanged) {
				t.Fatalf("external change: %v", err)
			}
			got, _ = os.ReadFile(path)
			if string(got) != "hacked" {
				t.Fatalf("external overwritten: %q", got)
			}
		})
	}
	if _, err := ReadEncoded(ctx, filepath.Join(t.TempDir(), "missing"), "koi8-r"); err == nil {
		t.Fatal("unknown encoding accepted")
	}
}

func TestReadDetectedChoosesEncodingAndRejectsLimits(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	write := func(name string, raw []byte) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	f, err := ReadDetected(ctx, write("gbk.txt", []byte{0xd6, 0xd0, 0xce, 0xc4, 0x0a}))
	if err != nil || f.Text != "中文\n" || f.Encoding != "gbk" {
		t.Fatalf("gbk: %+v %v", f, err)
	}
	f, err = ReadDetected(ctx, write("u16.txt", []byte{0xff, 0xfe, 0x41, 0x00, 0x62, 0x00, 0x0a, 0x00}))
	if err != nil || f.Text != "Ab\n" || f.Encoding != "utf-16le" || !f.BOM {
		t.Fatalf("utf16: %+v %v", f, err)
	}
	f, err = ReadDetected(ctx, write("u8.txt", []byte("中文\n")))
	if err != nil || f.Text != "中文\n" || f.Encoding != "utf-8" {
		t.Fatalf("utf8: %+v %v", f, err)
	}
	if _, err = ReadDetected(ctx, write("lat.txt", []byte{'c', 'a', 'f', 0xe9, '\n'})); err == nil {
		t.Fatal("latin1 guessed")
	}
	if _, err = ReadDetected(ctx, write("bin.txt", []byte{'a', 0, 'b'})); err == nil {
		t.Fatal("nul accepted")
	}
	if _, err = ReadDetected(ctx, write("big.txt", bytes.Repeat([]byte{'a'}, MaxBytes+1))); err == nil {
		t.Fatal("oversize accepted")
	}
	f, err = ReadDetected(ctx, write("exact.txt", bytes.Repeat([]byte{'a'}, MaxBytes)))
	if err != nil || len(f.Text) != MaxBytes || f.Encoding != "utf-8" {
		t.Fatalf("exact: %+v %v", f, err)
	}
}

func TestSaveAsOverwriteRefusesExternalChange(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "source")
	destPath := filepath.Join(dir, "dest")
	if err := os.WriteFile(srcPath, []byte("source\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destPath, []byte("dest\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	src, err := Read(ctx, srcPath)
	if err != nil {
		t.Fatal(err)
	}
	dest, err := Read(ctx, destPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = src.SaveAsOverwrite(ctx, destPath, "nope\n", nil); err == nil {
		t.Fatal("overwrite without baseline")
	}
	data, _ := os.ReadFile(destPath)
	if string(data) != "dest\n" {
		t.Fatal("unbaselined overwrite changed the file")
	}
	saved, err := src.SaveAsOverwrite(ctx, destPath, "replaced\n", dest)
	if err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(destPath)
	if string(data) != "replaced\n" || saved.Text != "replaced\n" {
		t.Fatalf("overwrite: %q", data)
	}
	if err = os.WriteFile(destPath, []byte("hacked\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err = src.SaveAsOverwrite(ctx, destPath, "again\n", saved); !errors.Is(err, ErrChanged) {
		t.Fatalf("stale overwrite: %v", err)
	}
	data, _ = os.ReadFile(destPath)
	if string(data) != "hacked\n" {
		t.Fatalf("external overwritten: %q", data)
	}
	fresh := filepath.Join(dir, "fresh")
	created, err := src.SaveAsOverwrite(ctx, fresh, "fresh\n", nil)
	if err != nil || created.Text != "fresh\n" {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(fresh)
	if string(data) != "fresh\n" {
		t.Fatalf("create: %q", data)
	}
	if _, err = src.SaveAs(ctx, fresh, "other\n"); !errors.Is(err, os.ErrExist) {
		t.Fatalf("SaveAs overwrite allowed: %v", err)
	}
}
