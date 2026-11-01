// Package dirmerge 把两方或三方目录合并成可检查的文件决策，文本文件复用 internal/merge。
package dirmerge

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"gocompare/internal/merge"
)

const (
	maxFileBytes = 16 << 20
	// maxTreeBytes 限制单棵目录树读入内存的总量；预览会持有全部决策内容，大目录必须明确报错。
	maxTreeBytes = 256 << 20
)

// Decision 是一个相对路径的合并结果。Delete 不携带内容；文本冲突的 Data 带有合并标记。
// 二进制冲突和删除/修改冲突无法用标记表示：Data 为 nil，Reason 说明原因，
// Left/Right 保存两侧原始内容（nil 表示该侧不存在），写出时分别落为 <path>.left 与 <path>.right。
type Decision struct {
	Path   string
	Kind   string
	Data   []byte
	Reason string
	Left   []byte
	Right  []byte
}

const (
	KindLeft     = "left"
	KindRight    = "right"
	KindDelete   = "delete"
	KindMerged   = "merged"
	KindConflict = "conflict"
)

// Two 没有共同祖先。单侧文件直接采用；两侧不同的文本文件做两方合并。
func Two(ctx context.Context, left, right string) ([]Decision, error) {
	a, err := collect(ctx, left)
	if err != nil {
		return nil, err
	}
	b, err := collect(ctx, right)
	if err != nil {
		return nil, err
	}
	var out []Decision
	for _, p := range union(a, b) {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		l, lok := a[p]
		r, rok := b[p]
		switch {
		case lok && !rok:
			out = append(out, Decision{Path: p, Kind: KindLeft, Data: l})
		case rok && !lok:
			out = append(out, Decision{Path: p, Kind: KindRight, Data: r})
		case bytes.Equal(l, r):
			out = append(out, Decision{Path: p, Kind: KindLeft, Data: l})
		default:
			item, err := merged(nil, l, r)
			if err != nil {
				return nil, err
			}
			item.Path = p
			out = append(out, item)
		}
	}
	return out, nil
}

// Three 以 base 为祖先。单侧修改自动采用，删除对修改是冲突，两侧文本修改复用三方合并。
func Three(ctx context.Context, base, left, right string) ([]Decision, error) {
	b, err := collect(ctx, base)
	if err != nil {
		return nil, err
	}
	l, err := collect(ctx, left)
	if err != nil {
		return nil, err
	}
	r, err := collect(ctx, right)
	if err != nil {
		return nil, err
	}
	var out []Decision
	for _, p := range union(b, l, r) {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		baseData, bok := b[p]
		leftData, lok := l[p]
		rightData, rok := r[p]
		switch {
		case lok && rok && bytes.Equal(leftData, rightData):
			out = append(out, Decision{Path: p, Kind: KindLeft, Data: leftData})
		case bok && lok && !rok && bytes.Equal(baseData, leftData):
			out = append(out, Decision{Path: p, Kind: KindDelete})
		case bok && rok && !lok && bytes.Equal(baseData, rightData):
			out = append(out, Decision{Path: p, Kind: KindDelete})
		case !lok && !rok:
			out = append(out, Decision{Path: p, Kind: KindDelete})
		case bok && lok && bytes.Equal(baseData, leftData) && rok:
			out = append(out, Decision{Path: p, Kind: KindRight, Data: rightData})
		case bok && rok && bytes.Equal(baseData, rightData) && lok:
			out = append(out, Decision{Path: p, Kind: KindLeft, Data: leftData})
		case !bok && lok && !rok:
			out = append(out, Decision{Path: p, Kind: KindLeft, Data: leftData})
		case !bok && rok && !lok:
			out = append(out, Decision{Path: p, Kind: KindRight, Data: rightData})
		case bok && (!lok || !rok):
			out = append(out, Decision{Path: p, Kind: KindConflict, Reason: "delete/modify conflict", Left: leftData, Right: rightData})
		default:
			item, err := merged(baseData, leftData, rightData)
			if err != nil {
				return nil, err
			}
			item.Path = p
			out = append(out, item)
		}
	}
	return out, nil
}

// Write 把决策写到空目录。文本冲突写下带标记的文本；二进制与删除/修改冲突把存在的一侧
// 写成 <path>.left、<path>.right，不生成占位内容；删除不会创建文件。
func Write(ctx context.Context, dest string, decisions []Decision) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(dest)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("destination must be a directory, not a symlink")
	}
	root, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer root.Close()
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	entries, err := dir.Readdirnames(1)
	dir.Close()
	if err != nil && err != io.EOF {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("destination must be empty")
	}
	for _, item := range decisions {
		if err := ctx.Err(); err != nil {
			return err
		}
		if item.Kind == KindDelete {
			continue
		}
		rel := filepath.FromSlash(item.Path)
		if !filepath.IsLocal(rel) || filepath.Clean(rel) == "." {
			return fmt.Errorf("invalid decision path: %s", item.Path)
		}
		if item.Kind == KindConflict && item.Data == nil {
			if item.Left != nil {
				if err := writeNew(root, rel+".left", item.Left); err != nil {
					return err
				}
			}
			if item.Right != nil {
				if err := writeNew(root, rel+".right", item.Right); err != nil {
					return err
				}
			}
			continue
		}
		if err := writeNew(root, rel, item.Data); err != nil {
			return err
		}
	}
	return nil
}

// writeNew 在 root 内创建新文件；O_EXCL 保证不会覆盖已写出的文件或经由符号链接写出。
func writeNew(root *os.Root, rel string, data []byte) error {
	if err := root.MkdirAll(filepath.Dir(rel), 0o700); err != nil {
		return err
	}
	f, err := root.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

func merged(base, left, right []byte) (Decision, error) {
	if bytes.IndexByte(left, 0) >= 0 || bytes.IndexByte(right, 0) >= 0 || (base != nil && bytes.IndexByte(base, 0) >= 0) {
		return Decision{Kind: KindConflict, Reason: "binary conflict", Left: left, Right: right}, nil
	}
	var result merge.Result
	var err error
	if base == nil {
		result, err = merge.MergeTwo(string(left), string(right))
	} else {
		result, err = merge.MergeThree(string(base), string(left), string(right))
	}
	if err != nil {
		return Decision{}, err
	}
	kind := KindMerged
	if result.ConflictCount() > 0 {
		kind = KindConflict
	}
	return Decision{Kind: kind, Data: []byte(result.Text())}, nil
}

func collect(ctx context.Context, root string) (map[string][]byte, error) {
	out := map[string][]byte{}
	if root == "" {
		return out, nil
	}
	total := 0
	err := filepath.WalkDir(root, func(path string, item os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if item.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink requires an explicit policy: %s", rel)
		}
		if item.IsDir() {
			return nil
		}
		if !item.Type().IsRegular() {
			return fmt.Errorf("unsupported file type: %s", rel)
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(f, maxFileBytes+1))
		f.Close()
		if err != nil {
			return err
		}
		if len(data) > maxFileBytes {
			return fmt.Errorf("file exceeds 16 MiB limit: %s", rel)
		}
		total += len(data)
		if total > maxTreeBytes {
			return fmt.Errorf("directory %s exceeds 256 MiB merge limit", root)
		}
		out[rel] = data
		return nil
	})
	return out, err
}

func union(sets ...map[string][]byte) []string {
	seen := map[string]bool{}
	var paths []string
	for _, set := range sets {
		for p := range set {
			if !seen[p] {
				seen[p] = true
				paths = append(paths, p)
			}
		}
	}
	sort.Strings(paths)
	return paths
}
