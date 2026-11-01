package compare

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"

	"gocompare/internal/remote"
)

// Source 是目录比较的一侧。相对路径一律使用 "/" 分隔。
type Source interface {
	// Scan 递归列出全部条目，按 opt 过滤并报告进度。
	Scan(ctx context.Context, opt ScanOptions) (map[string]*Metadata, error)
	// Open 打开普通文件以比较内容。
	Open(ctx context.Context, rel string) (io.ReadCloser, error)
	// Readlink 返回符号链接的目标。
	Readlink(ctx context.Context, rel string) (string, error)
}

// LocalSource 是本地目录。
func LocalSource(root string) Source { return localSource(root) }

type localSource string

func (s localSource) Scan(ctx context.Context, opt ScanOptions) (map[string]*Metadata, error) {
	return scan(ctx, string(s), opt)
}

func (s localSource) Open(ctx context.Context, rel string) (io.ReadCloser, error) {
	full := filepath.Join(string(s), filepath.FromSlash(rel))
	f, err := os.Open(full)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err == nil && !st.Mode().IsRegular() {
		err = fmt.Errorf("not a regular file: %s", full)
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

func (s localSource) Readlink(ctx context.Context, rel string) (string, error) {
	return os.Readlink(filepath.Join(string(s), filepath.FromSlash(rel)))
}

// RemoteSource 是远程目录，root 使用 remote.Lister 的路径写法。
// 远程扫描不跟随符号链接（LinkFollow 按 LinkRecord 处理），避免沿链接走出比较根。
func RemoteSource(conn remote.Conn, root string) Source {
	return &remoteSource{conn: conn, root: root}
}

type remoteSource struct {
	conn remote.Conn
	root string // Scan 后替换为服务端解析出的路径
}

func (s *remoteSource) Scan(ctx context.Context, opt ScanOptions) (map[string]*Metadata, error) {
	resolved, entries, err := s.conn.List(ctx, s.root)
	if err != nil {
		return nil, err
	}
	s.root = resolved
	result := map[string]*Metadata{}
	var walk func(dir, rel string, entries []remote.Entry) error
	walk = func(dir, rel string, entries []remote.Entry) error {
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
		for _, e := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			childRel := e.Name
			if rel != "" {
				childRel = rel + "/" + e.Name
			}
			if opt.Progress != nil {
				if err := opt.Progress(childRel); err != nil {
					return err
				}
			}
			include, err := opt.include(e.Name, childRel)
			if err != nil {
				return err
			}
			m := &Metadata{Directory: e.Dir && !e.Link, Symlink: e.Link, Size: e.Size, Mode: uint32(e.Mode), Modified: e.ModTime.UnixNano()}
			if e.ModTime.IsZero() {
				m.Modified = 0
			}
			if include || (opt.KeepParents && m.Directory) {
				result[childRel] = m
			}
			if !m.Directory {
				continue
			}
			child := path.Join(dir, e.Name)
			_, children, err := s.conn.List(ctx, child)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return err
				}
				m.Error = err.Error()
				continue
			}
			if err = walk(child, childRel, children); err != nil {
				return err
			}
		}
		return nil
	}
	if err = walk(resolved, "", entries); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *remoteSource) Open(ctx context.Context, rel string) (io.ReadCloser, error) {
	data, err := s.conn.Read(ctx, path.Join(s.root, rel))
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *remoteSource) Readlink(ctx context.Context, rel string) (string, error) {
	return s.conn.Readlink(ctx, path.Join(s.root, rel))
}

// equalSources 流式比较两侧同一相对路径的文件内容。
func equalSources(ctx context.Context, left, right Source, rel string) (bool, error) {
	a, err := left.Open(ctx, rel)
	if err != nil {
		return false, err
	}
	defer a.Close()
	b, err := right.Open(ctx, rel)
	if err != nil {
		return false, err
	}
	defer b.Close()
	return equalReaders(ctx, a, b)
}
