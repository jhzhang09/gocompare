package compare

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
)

// Status 区分相同、不同、单侧存在和错误；读取失败不能标为相同。
type Status string

const (
	Same        Status = "same"
	Different   Status = "different"
	LeftOnly    Status = "left_only"
	RightOnly   Status = "right_only"
	TypeChanged Status = "type_changed"
	Failed      Status = "error"
)

// LinkPolicy 决定符号链接是记录目标，还是显式跟随。
// 零值与 LinkRecord 都不跟随，避免默认走出比较根。
type LinkPolicy string

const (
	LinkRecord LinkPolicy = "record"
	LinkFollow LinkPolicy = "follow"
)

// ScanOptions 控制过滤、链接、属性规则和渐进回调。
// Progress 在发现每个相对路径时调用，父目录早于其子项；返回的错误会停止扫描。
type ScanOptions struct {
	Filter      string
	Links       LinkPolicy
	CompareMode bool
	CompareTime bool
	Progress    func(rel string) error
}

// Metadata 保留目录条目的类型和大小；符号链接默认不隐式解引用。
type Metadata struct {
	Directory bool   `json:"directory"`
	Symlink   bool   `json:"symlink"`
	Size      int64  `json:"size"`
	Mode      uint32 `json:"mode,omitempty"`
	Modified  int64  `json:"modified,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Entry 是按相对路径对齐的目录比较行。
type Entry struct {
	Path   string    `json:"path"`
	Left   *Metadata `json:"left,omitempty"`
	Right  *Metadata `json:"right,omitempty"`
	Status Status    `json:"status"`
	Error  string    `json:"error,omitempty"`
}

// Directories 递归比较目录，不跟随符号链接，按确定的相对路径顺序输出。
// 子项错误保留为 Failed；取消与根目录错误直接返回。
func Directories(ctx context.Context, left, right string) ([]Entry, error) {
	return CompareDirectories(ctx, left, right, ScanOptions{})
}

// CompareDirectories 在 Directories 的结果上应用过滤、链接策略和属性规则。
func CompareDirectories(ctx context.Context, left, right string, opt ScanOptions) ([]Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if opt.Links != "" && opt.Links != LinkRecord && opt.Links != LinkFollow {
		return nil, fmt.Errorf("unknown link policy: %s", opt.Links)
	}
	a, err := scan(ctx, left, opt)
	if err != nil {
		return nil, err
	}
	b, err := scan(ctx, right, opt)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(a)+len(b))
	for path := range a {
		paths = append(paths, path)
	}
	for path := range b {
		if _, ok := a[path]; !ok {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	result := make([]Entry, 0, len(paths))
	for _, path := range paths {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		x, y := a[path], b[path]
		row := Entry{Path: path, Left: x, Right: y, Status: Same}
		switch {
		case x != nil && x.Error != "":
			row.Status = Failed
			row.Error = x.Error
		case y != nil && y.Error != "":
			row.Status = Failed
			row.Error = y.Error
		case x == nil:
			row.Status = RightOnly
		case y == nil:
			row.Status = LeftOnly
		case x.Directory != y.Directory || x.Symlink != y.Symlink:
			row.Status = TypeChanged
		case x.Directory:
			if attributeDiffers(opt, x, y) {
				row.Status = Different
			}
		case x.Symlink:
			var aa, bb string
			aa, err = os.Readlink(filepath.Join(left, filepath.FromSlash(path)))
			if err == nil {
				bb, err = os.Readlink(filepath.Join(right, filepath.FromSlash(path)))
			}
			if err == nil && aa != bb {
				row.Status = Different
			}
		case x.Size != y.Size:
			row.Status = Different
		case attributeDiffers(opt, x, y):
			row.Status = Different
		default:
			var equal bool
			equal, err = EqualFiles(ctx, filepath.Join(left, filepath.FromSlash(path)), filepath.Join(right, filepath.FromSlash(path)))
			if err == nil && !equal {
				row.Status = Different
			}
		}
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			row.Status = Failed
			row.Error = err.Error()
			err = nil
		}
		result = append(result, row)
	}
	// 子项差异向共同父目录汇总；单侧目录保留单侧状态，错误优先于差异。
	byPath := map[string]int{}
	for i := range result {
		byPath[result[i].Path] = i
	}
	for i := len(result) - 1; i >= 0; i-- {
		row := result[i]
		if row.Status == Same {
			continue
		}
		for parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(row.Path))); parent != "."; parent = filepath.ToSlash(filepath.Dir(filepath.FromSlash(parent))) {
			index, ok := byPath[parent]
			if !ok {
				continue
			}
			p := &result[index]
			if p.Status == Same {
				p.Status = Different
			}
			if row.Status == Failed {
				p.Status = Failed
				p.Error = "descendant could not be compared"
			}
		}
	}
	return result, nil
}

func attributeDiffers(opt ScanOptions, x, y *Metadata) bool {
	if opt.CompareMode && x.Mode != y.Mode {
		return true
	}
	return opt.CompareTime && x.Modified != y.Modified
}

func scan(ctx context.Context, root string, opt ScanOptions) (map[string]*Metadata, error) {
	st, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", root)
	}
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	result := map[string]*Metadata{}
	seen := map[string]bool{rootReal: true}
	var walk func(dir, rel string) error
	walk = func(dir, rel string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, item := range entries {
			if err = ctx.Err(); err != nil {
				return err
			}
			name := item.Name()
			childRel := name
			if rel != "" {
				childRel = rel + "/" + name
			}
			full := filepath.Join(dir, name)
			if opt.Progress != nil {
				if err = opt.Progress(childRel); err != nil {
					return err
				}
			}
			if err = ctx.Err(); err != nil {
				return err
			}
			include := true
			if opt.Filter != "" {
				include, err = path.Match(opt.Filter, childRel)
				if err != nil {
					return err
				}
			}
			info, infoErr := os.Lstat(full)
			m := &Metadata{}
			if infoErr != nil {
				m.Error = infoErr.Error()
			} else {
				m.Directory = info.IsDir()
				m.Symlink = info.Mode()&os.ModeSymlink != 0
				m.Size = info.Size()
				m.Mode = uint32(info.Mode().Perm())
				m.Modified = info.ModTime().UnixNano()
			}
			if include {
				result[childRel] = m
			}
			if m.Error != "" {
				continue
			}
			if m.Symlink {
				if opt.Links != LinkFollow {
					continue
				}
				if err = followLink(ctx, rootReal, full, childRel, m, seen, walk); err != nil {
					return err
				}
				continue
			}
			if m.Directory {
				if err = walk(full, childRel); err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						return err
					}
					m.Error = err.Error()
				}
			}
		}
		return nil
	}
	if err = walk(root, ""); err != nil {
		return nil, err
	}
	return result, nil
}

func followLink(ctx context.Context, rootReal, full, childRel string, m *Metadata, seen map[string]bool, walk func(string, string) error) error {
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		m.Error = err.Error()
		return nil
	}
	relOut, err := filepath.Rel(rootReal, resolved)
	if err != nil || (relOut != "." && !filepath.IsLocal(relOut)) {
		m.Error = "symlink escapes root"
		return nil
	}
	if seen[resolved] {
		m.Error = "symlink cycle"
		return nil
	}
	st, err := os.Stat(resolved)
	if err != nil {
		m.Error = err.Error()
		return nil
	}
	m.Symlink = false
	m.Directory = st.IsDir()
	m.Size = st.Size()
	m.Mode = uint32(st.Mode().Perm())
	m.Modified = st.ModTime().UnixNano()
	if !st.IsDir() {
		return nil
	}
	seen[resolved] = true
	err = walk(resolved, childRel)
	delete(seen, resolved)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		m.Error = err.Error()
	}
	return ctx.Err()
}
