// Package document 负责本地与远程文本的保真读取及带外部修改检测的保存。
package document

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"gocompare/internal/doctext"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// MaxBytes 是当前原型的读取上限，防止在 UI 比较路径中处理无界输入。
const MaxBytes = 16 << 20

// ErrChanged 表示文件在加载后已被其他程序修改。
var ErrChanged = errors.New("file changed on disk")

// ErrReadOnly 表示不可逆转换得到的文档禁止保存。
var ErrReadOnly = errors.New("converted document is read-only")

// File 保留文本与原始文件信息；Text 使用 LF，保存时还原编码、BOM 和换行。
// Encoding 为空时按 utf-8 处理。
type File struct {
	Path      string
	Text      string
	Encoding  string
	BOM       bool
	CRLF      bool
	converted bool
	digest    [32]byte
	info      os.FileInfo
	// remote 非空表示远程文件：Path 只是不含密码的显示地址，读写都经由 remote。
	remote Remote
	// remoteMissing 表示远程一侧原本不存在（目录比较的单侧文件）；保存时要求它仍不存在，然后创建。
	remoteMissing bool
}

// Remote 是单个远程文件的读写入口，连接、认证与超时由实现方负责。
type Remote interface {
	Read(ctx context.Context) ([]byte, error)
	Write(ctx context.Context, data []byte) error
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
	f, err := detected(full, data)
	if err != nil {
		return nil, err
	}
	f.info = info
	return f, nil
}

// ReadRemote 读取远程文件，编码识别与转换规则同 ReadDetected。
// label 用于显示、标题和会话保存，调用方必须先去掉其中的密码。
func ReadRemote(ctx context.Context, label string, src Remote) (*File, error) {
	data, err := readRemote(ctx, src)
	if err != nil {
		return nil, err
	}
	f, err := detected(label, data)
	if err != nil {
		return nil, err
	}
	f.remote = src
	return f, nil
}

// EmptyRemote 为目录比较中远程缺失的一侧建立空白缓冲区；保存时若远程已出现同名文件则返回 ErrChanged。
func EmptyRemote(label string, src Remote) *File {
	return &File{Path: label, Encoding: "utf-8", remote: src, remoteMissing: true}
}

func readRemote(ctx context.Context, src Remote) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := src.Read(ctx)
	if err != nil {
		return nil, err
	}
	if len(data) > MaxBytes {
		return nil, fmt.Errorf("file exceeds %d MiB limit", MaxBytes>>20)
	}
	return data, nil
}

// detected 按内容识别编码并解码；调用方补上本地文件信息或远程来源。
func detected(full string, data []byte) (*File, error) {
	// RTF 本身是文本，保持按源码读取和编辑的既有行为；只有 docx、PDF 这类无法按文本读取的格式才自动转换为只读文本。
	if doctext.Supports(full, data) && !doctext.IsRTF(full, data) {
		text, docErr := doctext.Convert(full, data)
		if docErr == nil {
			return &File{Path: full, Text: text, converted: true, Encoding: "utf-8", BOM: false, CRLF: false, digest: sha256.Sum256(data)}, nil
		}
	}
	encoding := detectEncoding(data)
	if encoding == "" {
		return nil, errors.New("unsupported encoding or binary file; UTF-8 required")
	}
	text, bom, crlf, err := decodeBytes(data, encoding)
	if err != nil {
		return nil, err
	}
	return &File{Path: full, Text: text, Encoding: encoding, BOM: bom, CRLF: crlf, digest: sha256.Sum256(data)}, nil
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
// 返回新的基线，不修改调用者持有的 File。转换文档返回 ErrReadOnly。
func (d *File) Save(ctx context.Context, text string) (*File, error) {
	if d.remote != nil {
		return d.saveRemote(ctx, text)
	}
	if d.Missing() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, err := d.encoded(text); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(d.Path), 0o755); err != nil {
			return nil, err
		}
		return d.SaveAs(ctx, d.Path, text)
	}
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

// saveRemote 写回远程文件。写入前重新读取并核对基线，远程内容已被他人修改时返回 ErrChanged。
// 远程协议没有比较并交换语义，核对与写入之间仍有短暂竞争窗口。
func (d *File) saveRemote(ctx context.Context, text string) (*File, error) {
	data, err := d.encoded(text)
	if err != nil {
		return nil, err
	}
	if _, err = d.unchangedBytes(ctx); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if err = d.remote.Write(ctx, data); err != nil {
		return nil, err
	}
	return &File{Path: d.Path, Text: text, Encoding: d.Encoding, BOM: d.BOM, CRLF: d.CRLF, digest: sha256.Sum256(data), remote: d.remote}, nil
}

func (d *File) unchanged(ctx context.Context) error {
	_, err := d.unchangedBytes(ctx)
	return err
}

// unchangedBytes 核对磁盘内容仍与基线一致，并返回这份原始字节。
func (d *File) unchangedBytes(ctx context.Context) ([]byte, error) {
	if d.remote != nil {
		data, err := readRemote(ctx, d.remote)
		if d.remoteMissing {
			if errors.Is(err, fs.ErrNotExist) {
				return nil, nil
			}
			if err == nil {
				return nil, ErrChanged
			}
			return nil, err
		}
		if err != nil {
			return nil, err
		}
		if sha256.Sum256(data) != d.digest {
			return nil, ErrChanged
		}
		return data, nil
	}
	_, data, info, err := readRaw(ctx, d.Path)
	if err != nil {
		return nil, err
	}
	if sha256.Sum256(data) != d.digest || !os.SameFile(info, d.info) || info.Mode() != d.info.Mode() {
		return nil, ErrChanged
	}
	return data, nil
}

// EmptyAt 为目录比较的缺失一侧建立空白缓冲区；保存时必须创建新文件。
func EmptyAt(path string) (*File, error) {
	full, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	return &File{Path: full, Encoding: "utf-8"}, nil
}
func (d *File) Missing() bool { return d.info == nil && d.remote == nil }

// IsRemote 表示文件来自远程来源。
func (d *File) IsRemote() bool { return d.remote != nil }

// ReadMergeable 读取用于合并的文本。转换文档返回 ErrReadOnly：
// 提取文本与源格式不对应，合并结果既不能写回源格式，也会把提取文本混进其他文本文件。
func ReadMergeable(ctx context.Context, path string) (*File, error) {
	f, err := ReadDetected(ctx, path)
	if err != nil {
		return nil, err
	}
	if f.ReadOnly() {
		return nil, fmt.Errorf("%s: %w", f.Path, ErrReadOnly)
	}
	return f, nil
}

// ReadOnly 表示文档是否由不可逆转换生成。
func (d *File) ReadOnly() bool { return d.converted }
