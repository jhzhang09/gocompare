package document

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// memRemote 是内存中的远程文件，记录写入次数；missing 为真时表示文件不存在。
type memRemote struct {
	data    []byte
	writes  int
	missing bool
}

func (m *memRemote) Read(context.Context) ([]byte, error) {
	if m.missing {
		return nil, fs.ErrNotExist
	}
	return append([]byte(nil), m.data...), nil
}

func (m *memRemote) Write(_ context.Context, data []byte) error {
	m.data = append([]byte(nil), data...)
	m.missing = false
	m.writes++
	return nil
}

func TestRemoteRoundTripKeepsEncoding(t *testing.T) {
	ctx := context.Background()
	src := &memRemote{data: []byte("\xef\xbb\xbfline1\r\nline2\r\n")}
	f, err := ReadRemote(ctx, "sftp://alice@host/a.txt", src)
	if err != nil {
		t.Fatal(err)
	}
	if f.Text != "line1\nline2\n" || !f.BOM || !f.CRLF || f.Missing() || !f.IsRemote() || f.ReadOnly() {
		t.Fatalf("decoded %+v", f)
	}
	saved, err := f.Save(ctx, "line1\nchanged\n")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(src.data, []byte("\xef\xbb\xbfline1\r\nchanged\r\n")) {
		t.Fatalf("remote bytes %q", src.data)
	}
	// 新基线可以继续保存。
	if _, err = saved.Save(ctx, "again\n"); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteSaveRejectsExternalChange(t *testing.T) {
	ctx := context.Background()
	src := &memRemote{data: []byte("v1\n")}
	f, err := ReadRemote(ctx, "smb://nas/share/a.txt", src)
	if err != nil {
		t.Fatal(err)
	}
	src.data = []byte("someone else\n")
	if _, err = f.Save(ctx, "mine\n"); !errors.Is(err, ErrChanged) {
		t.Fatalf("want ErrChanged, got %v", err)
	}
	if src.writes != 0 || string(src.data) != "someone else\n" {
		t.Fatalf("remote overwritten: %q (%d writes)", src.data, src.writes)
	}
}

func TestRemoteBackupAndSaveAsLocal(t *testing.T) {
	ctx := context.Background()
	src := &memRemote{data: []byte("remote body\n")}
	f, err := ReadRemote(ctx, "webdav://dav/a.txt", src)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	backup, err := f.Backup(ctx, filepath.Join(dir, "bak"))
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(backup); string(data) != "remote body\n" {
		t.Fatalf("backup %q", data)
	}
	local := filepath.Join(dir, "copy.txt")
	if _, err = f.SaveAs(ctx, local, "local copy\n"); err != nil {
		t.Fatal(err)
	}
	if src.writes != 0 {
		t.Fatal("save as a local file must not write the remote source")
	}
	if _, err = (&File{Path: local}).SaveAsOverwrite(ctx, local, "x", f); err == nil {
		t.Fatal("remote baseline accepted for a local overwrite")
	}
}

func TestRemoteRejectsOversizedAndConvertsDocx(t *testing.T) {
	ctx := context.Background()
	if _, err := ReadRemote(ctx, "s3://h/b/k", &memRemote{data: make([]byte, MaxBytes+1)}); err == nil {
		t.Fatal("oversized remote file accepted")
	}
	p := filepath.Join(t.TempDir(), "d.docx")
	docx := writeTestDOCX(t, p, "remote doc")
	f, err := ReadRemote(ctx, "sftp://h/d.docx", &memRemote{data: docx})
	if err != nil || f.Text != "remote doc" || !f.ReadOnly() {
		t.Fatalf("docx %+v %v", f, err)
	}
	if _, err = f.Save(ctx, "x"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("converted remote doc saved: %v", err)
	}
}

func TestEmptyRemoteCreatesOnlyIfStillMissing(t *testing.T) {
	ctx := context.Background()
	src := &memRemote{missing: true}
	f := EmptyRemote("sftp://h/new.txt", src)
	if f.Missing() || !f.IsRemote() || f.Text != "" {
		t.Fatalf("empty remote %+v", f)
	}
	saved, err := f.Save(ctx, "created\n")
	if err != nil || string(src.data) != "created\n" {
		t.Fatalf("create: %q %v", src.data, err)
	}
	if _, err = saved.Save(ctx, "again\n"); err != nil {
		t.Fatalf("baseline after create: %v", err)
	}
	// 另一个人先创建了同名文件：拒绝覆盖。
	other := &memRemote{missing: true}
	g := EmptyRemote("sftp://h/x.txt", other)
	other.missing, other.data = false, []byte("theirs")
	if _, err = g.Save(ctx, "mine"); !errors.Is(err, ErrChanged) || string(other.data) != "theirs" {
		t.Fatalf("overwrote concurrent file: %q %v", other.data, err)
	}
}
