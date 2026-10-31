// Package archive 按格式读写压缩包。zip 可读写并支持嵌套；tar.gz 只读；7z 明确不支持。
package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Entry 是归档中的一个成员。目录没有内容；空文件的 Data 长度为 0 且 Dir 为 false。
type Entry struct {
	Name string
	Dir  bool
	Data []byte
}

// Read 按声明的格式读取。format 取 zip、tar.gz 或 7z。
func Read(format string, data []byte) ([]Entry, error) {
	switch format {
	case "zip":
		return readZip(data)
	case "tar.gz":
		return readTarGz(data)
	case "7z":
		return nil, errors.New("7z is not supported")
	default:
		return nil, fmt.Errorf("unknown archive format %s", format)
	}
}

// Write 按声明的格式写入。tar.gz 与 7z 会返回明确错误，不会静默写成别的格式。
func Write(format string, entries []Entry) ([]byte, error) {
	switch format {
	case "zip":
		return writeZip(entries)
	case "tar.gz":
		return nil, errors.New("tar.gz is read-only")
	case "7z":
		return nil, errors.New("7z is not supported")
	default:
		return nil, fmt.Errorf("unknown archive format %s", format)
	}
}

func writeZip(entries []Entry) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		name := e.Name
		if e.Dir && !strings.HasSuffix(name, "/") {
			name += "/"
		}
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if e.Dir {
			h.Method = zip.Store
			h.SetMode(os.ModeDir | 0o755)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			return nil, err
		}
		if !e.Dir {
			if _, err = w.Write(e.Data); err != nil {
				return nil, err
			}
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

const (
	maxEntryBytes   = 32 << 20
	maxArchiveBytes = 128 << 20
)

func readZip(data []byte) ([]Entry, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	var out []Entry
	var total int
	for _, f := range zr.File {
		dir := f.FileInfo().IsDir() || strings.HasSuffix(f.Name, "/")
		e := Entry{Name: f.Name, Dir: dir}
		if !dir {
			r, err := f.Open()
			if err != nil {
				return nil, err
			}
			e.Data, err = io.ReadAll(io.LimitReader(r, maxEntryBytes+1))
			r.Close()
			if err != nil {
				return nil, err
			}
			if len(e.Data) > maxEntryBytes {
				return nil, errors.New("archive entry exceeds 32 MiB")
			}
			total += len(e.Data)
			if total > maxArchiveBytes {
				return nil, errors.New("archive total size exceeds 128 MiB")
			}
		}
		out = append(out, e)
	}
	return out, nil
}

func readTarGz(data []byte) ([]Entry, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var out []Entry
	var total int
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		e := Entry{Name: h.Name, Dir: h.Typeflag == tar.TypeDir || strings.HasSuffix(h.Name, "/")}
		if !e.Dir {
			e.Data, err = io.ReadAll(io.LimitReader(tr, maxEntryBytes+1))
			if err != nil {
				return nil, err
			}
			if len(e.Data) > maxEntryBytes {
				return nil, errors.New("archive entry exceeds 32 MiB")
			}
			total += len(e.Data)
			if total > maxArchiveBytes {
				return nil, errors.New("archive total size exceeds 128 MiB")
			}
		}
		out = append(out, e)
	}
}
