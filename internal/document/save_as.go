package document

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// encoded 校验编辑文本，并恢复原文档的编码标记和换行。
func (d *File) encoded(text string) ([]byte, error) {
	if !utf8.ValidString(text) || strings.ContainsRune(text, 0) || strings.ContainsRune(text, '\r') {
		return nil, errors.New("text must be UTF-8 with LF line endings")
	}
	if d.CRLF {
		text = strings.ReplaceAll(text, "\n", "\r\n")
	}
	data, err := encodeText(text, d.Encoding, d.BOM)
	if err != nil {
		return nil, err
	}
	if len(data) > MaxBytes {
		return nil, fmt.Errorf("file exceeds %d MiB limit", MaxBytes>>20)
	}
	return data, nil
}

// SaveAs 创建新的文件，保留源文档格式。目标存在时返回 os.ErrExist，绝不静默覆盖。
func (d *File) SaveAs(ctx context.Context, path, text string) (*File, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := d.encoded(text)
	if err != nil {
		return nil, err
	}
	full, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	temp, err := os.CreateTemp(filepath.Dir(full), ".gocompare-save-*")
	if err != nil {
		return nil, err
	}
	name := temp.Name()
	defer os.Remove(name)
	if _, err = temp.Write(data); err == nil {
		mode := os.FileMode(0o600)
		if d.info != nil {
			mode = d.info.Mode().Perm()
		}
		err = temp.Chmod(mode)
	}
	if err == nil {
		err = temp.Sync()
	}
	closeErr := temp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	// 同目录硬链接发布保证目标不存在；不支持硬链接的文件系统明确报错。
	if err = os.Link(name, full); err != nil {
		return nil, err
	}
	info, err := os.Stat(full)
	if err != nil {
		return nil, fmt.Errorf("saved but cannot stat file: %w", err)
	}
	return &File{Path: full, Text: text, Encoding: d.Encoding, BOM: d.BOM, CRLF: d.CRLF, digest: sha256.Sum256(data), info: info}, nil
}

// SaveAsOverwrite 把文本写到已有或新的路径。
// 目标不存在时 baseline 必须为 nil，并创建文件。
// 目标已存在时必须提供该目标的读取基线；磁盘内容或身份与基线不符时返回 ErrChanged，且不覆盖。
func (d *File) SaveAsOverwrite(ctx context.Context, path, text string, destBaseline *File) (*File, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	full, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	info, statErr := os.Lstat(full)
	if statErr != nil && !os.IsNotExist(statErr) {
		return nil, statErr
	}
	if os.IsNotExist(statErr) {
		if destBaseline != nil {
			return nil, errors.New("baseline provided for a missing file")
		}
		return d.SaveAs(ctx, full, text)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file: %s", full)
	}
	if destBaseline == nil {
		return nil, fmt.Errorf("refusing to overwrite %s without a baseline", full)
	}
	basePath, err := filepath.Abs(destBaseline.Path)
	if err != nil {
		return nil, err
	}
	if basePath != full {
		return nil, fmt.Errorf("baseline path does not match destination")
	}
	if err = destBaseline.unchanged(ctx); err != nil {
		return nil, err
	}
	data, err := d.encoded(text)
	if err != nil {
		return nil, err
	}
	temp, err := os.CreateTemp(filepath.Dir(full), ".gocompare-save-*")
	if err != nil {
		return nil, err
	}
	name := temp.Name()
	defer os.Remove(name)
	if _, err = temp.Write(data); err == nil {
		err = temp.Chmod(destBaseline.info.Mode().Perm())
	}
	if err == nil {
		err = temp.Sync()
	}
	closeErr := temp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if err = destBaseline.unchanged(ctx); err != nil {
		return nil, err
	}
	if err = os.Rename(name, full); err != nil {
		return nil, err
	}
	saved, err := os.Stat(full)
	if err != nil {
		return nil, fmt.Errorf("saved but cannot stat file: %w", err)
	}
	return &File{Path: full, Text: text, Encoding: d.Encoding, BOM: d.BOM, CRLF: d.CRLF, digest: sha256.Sum256(data), info: saved}, nil
}

// Backup 将原始内容备份到指定目录，返回备份路径。内容变化时拒绝备份旧基线。
func (d *File) Backup(ctx context.Context, dir string) (string, error) {
	if err := d.unchanged(ctx); err != nil {
		return "", err
	}
	data, err := d.encoded(d.Text)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, fmt.Sprintf("%x-*.bak", sha256.Sum256([]byte(d.Path))))
	if err != nil {
		return "", err
	}
	name := f.Name()
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}
