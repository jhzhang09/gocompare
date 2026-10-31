// Package document 负责本地文本的保真读取及带外部修改检测的保存。
package document

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// MaxBytes 是当前原型的读取上限，防止在 UI 比较路径中处理无界输入。
const MaxBytes = 16 << 20

// ErrChanged 表示文件在加载后已被其他程序修改。
var ErrChanged = errors.New("file changed on disk")

// File 保留文本与原始文件信息；Text 使用 LF，保存时还原编码、BOM 和换行。
// Encoding 为空时按 utf-8 处理。
type File struct {
	Path     string
	Text     string
	Encoding string
	BOM      bool
	CRLF     bool
	digest   [32]byte
	info     os.FileInfo
}

// Read 读取普通 UTF-8 文件；其他已声明编码使用 ReadEncoded。
func Read(ctx context.Context, path string) (*File, error) {
	return ReadEncoded(ctx, path, "utf-8")
}

// ReadDetected 读取普通文件，并按内容选择编码。
// 顺序是 UTF-16 BOM、UTF-8 BOM 或合法 UTF-8、可原样往返的 GBK。
// Latin-1 必须用 ReadEncoded 显式指定。超过 MaxBytes 的文件拒绝读取。
func ReadDetected(ctx context.Context, path string) (*File, error) {
	full, data, info, err := readRaw(ctx, path)
	if err != nil {
		return nil, err
	}
	encoding := detectEncoding(data)
	if encoding == "" {
		return nil, errors.New("unsupported encoding or binary file; UTF-8 required")
	}
	text, bom, crlf, err := decodeBytes(data, encoding)
	if err != nil {
		return nil, err
	}
	return &File{Path: full, Text: text, Encoding: encoding, BOM: bom, CRLF: crlf, digest: sha256.Sum256(data), info: info}, nil
}

// ReadEncoded 按声明的编码读取普通文件。
// 支持 utf-8、utf-16le、utf-16be、gbk 和 latin1。逻辑文本统一为 LF。
func ReadEncoded(ctx context.Context, path, encoding string) (*File, error) {
	if encoding == "" {
		encoding = "utf-8"
	}
	full, data, info, err := readRaw(ctx, path)
	if err != nil {
		return nil, err
	}
	text, bom, crlf, err := decodeBytes(data, encoding)
	if err != nil {
		return nil, err
	}
	return &File{Path: full, Text: text, Encoding: encoding, BOM: bom, CRLF: crlf, digest: sha256.Sum256(data), info: info}, nil
}

func readRaw(ctx context.Context, path string) (string, []byte, os.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return "", nil, nil, err
	}
	full, err := filepath.Abs(path)
	if err != nil {
		return "", nil, nil, err
	}
	info, err := os.Lstat(full)
	if err != nil {
		return "", nil, nil, err
	}
	if !info.Mode().IsRegular() {
		return "", nil, nil, fmt.Errorf("not a regular file: %s", full)
	}
	f, err := os.Open(full)
	if err != nil {
		return "", nil, nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil {
		return "", nil, nil, err
	}
	if len(data) > MaxBytes {
		return "", nil, nil, fmt.Errorf("file exceeds %d MiB limit", MaxBytes>>20)
	}
	if err = ctx.Err(); err != nil {
		return "", nil, nil, err
	}
	st, err := f.Stat()
	if err != nil {
		return "", nil, nil, err
	}
	if !os.SameFile(info, st) {
		return "", nil, nil, ErrChanged
	}
	return full, data, st, nil
}

// Save 通过同目录临时文件替换原文件并保留其权限；外部变化时拒绝覆盖。
// 返回新的基线，不修改调用者持有的 File。
func (d *File) Save(ctx context.Context, text string) (*File, error) {
	data, err := d.encoded(text)
	if err != nil {
		return nil, err
	}
	if err := d.unchanged(ctx); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(d.Path), ".gocompare-save-*")
	if err != nil {
		return nil, err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Chmod(d.info.Mode().Perm())
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	if err = d.unchanged(ctx); err != nil {
		return nil, err
	}
	if err = os.Rename(name, d.Path); err != nil {
		return nil, err
	}
	// 替换已经完成，直接使用已写入内容建立基线，避免把后续读取失败误报为未保存。
	info, err := os.Stat(d.Path)
	if err != nil {
		return nil, fmt.Errorf("saved but cannot stat file: %w", err)
	}
	return &File{Path: d.Path, Text: text, Encoding: d.Encoding, BOM: d.BOM, CRLF: d.CRLF, digest: sha256.Sum256(data), info: info}, nil
}

func (d *File) unchanged(ctx context.Context) error {
	_, data, info, err := readRaw(ctx, d.Path)
	if err != nil {
		return err
	}
	if sha256.Sum256(data) != d.digest || !os.SameFile(info, d.info) || info.Mode() != d.info.Mode() {
		return ErrChanged
	}
	return nil
}
