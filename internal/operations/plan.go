// Package operations 负责生成可检查的文件操作计划，执行层不隐藏在比较视图中。
package operations

import (
	"context"
	"fmt"
	"gocompare/internal/compare"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Mode 描述更新和镜像方向；双向更新不传播删除。
type Mode string

const (
	UpdateRight Mode = "update-right"
	UpdateLeft  Mode = "update-left"
	UpdateBoth  Mode = "update-both"
	MirrorRight Mode = "mirror-right"
	MirrorLeft  Mode = "mirror-left"
)

// Action 描述一个待确认操作，以及预览时源和目标的版本。
type Action struct {
	Kind          string  `json:"kind"`
	Path          string  `json:"path"`
	From          string  `json:"from,omitempty"`
	To            string  `json:"to"`
	Reason        string  `json:"reason"`
	SourceVersion Version `json:"source_version"`
	TargetVersion Version `json:"target_version"`
}

// Plan 是两个来源目录之间的操作预览。
type Plan struct {
	Mode          Mode     `json:"mode"`
	Left          string   `json:"left"`
	Right         string   `json:"right"`
	Actions       []Action `json:"actions"`
	LeftResolved  string   `json:"left_resolved"`
	RightResolved string   `json:"right_resolved"`
}

// Preview 重新比较后生成计划。遇到读取错误、链接或类型冲突时拒绝生成可执行预览。
func Preview(ctx context.Context, left, right string, mode Mode) (Plan, error) {
	p := Plan{Mode: mode, Actions: []Action{}}
	if mode != UpdateRight && mode != UpdateLeft && mode != UpdateBoth && mode != MirrorRight && mode != MirrorLeft {
		return p, fmt.Errorf("unknown sync mode: %s", mode)
	}
	var err error
	p.Left, err = filepath.Abs(left)
	if err != nil {
		return p, err
	}
	p.Right, err = filepath.Abs(right)
	if err != nil {
		return p, err
	}
	a, err := filepath.EvalSymlinks(p.Left)
	if err != nil {
		return p, err
	}
	b, err := filepath.EvalSymlinks(p.Right)
	if err != nil {
		return p, err
	}
	p.LeftResolved = a
	p.RightResolved = b
	p.Left = a
	p.Right = b
	// 父子目录或同一目录之间的同步会让输出反过来改变扫描来源。
	nested := func(root, path string) bool {
		rel, err := filepath.Rel(root, path)
		return err == nil && (rel == "." || filepath.IsLocal(rel))
	}
	if nested(a, b) || nested(b, a) {
		return p, fmt.Errorf("sync roots must be separate directories")
	}
	rows, err := compare.Directories(ctx, p.Left, p.Right)
	if err != nil {
		return p, err
	}
	add := func(kind, path, from, to, reason string) {
		p.Actions = append(p.Actions, Action{Kind: kind, Path: path, From: from, To: to, Reason: reason})
	}
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return Plan{}, err
		}
		if row.Status == compare.Same {
			continue
		}
		if row.Status == compare.Failed || row.Status == compare.TypeChanged {
			return Plan{}, fmt.Errorf("cannot plan %s: %s", row.Path, row.Status)
		}
		if (row.Left != nil && row.Left.Symlink) || (row.Right != nil && row.Right.Symlink) {
			return Plan{}, fmt.Errorf("symlink handling needs an explicit policy: %s", row.Path)
		}
		lpath, rpath := filepath.Join(p.Left, filepath.FromSlash(row.Path)), filepath.Join(p.Right, filepath.FromSlash(row.Path))
		if row.Left != nil && row.Right != nil && row.Left.Directory {
			continue
		}
		sourceLeft := mode == UpdateRight || mode == MirrorRight
		if mode == UpdateBoth {
			sourceLeft = row.Right == nil
			if row.Left != nil && row.Right != nil {
				l, err := os.Stat(lpath)
				if err != nil {
					return Plan{}, err
				}
				r, err := os.Stat(rpath)
				if err != nil {
					return Plan{}, err
				}
				if l.ModTime().Equal(r.ModTime()) {
					return Plan{}, fmt.Errorf("different content with equal timestamps: %s", row.Path)
				}
				sourceLeft = l.ModTime().After(r.ModTime())
			}
		}
		src, dst := row.Left, row.Right
		from, to, rootFrom, rootTo := lpath, rpath, p.Left, p.Right
		if !sourceLeft {
			src, dst = dst, src
			from, to, rootFrom, rootTo = to, from, rootTo, rootFrom
		}
		mirror := mode == MirrorRight || mode == MirrorLeft
		if src == nil {
			if mirror {
				kind := "delete"
				if dst.Directory {
					kind = "remove-directory"
				}
				add(kind, row.Path, "", rootTo, "only in destination")
			}
			continue
		}
		if src.Directory {
			if dst == nil {
				add("create-directory", row.Path, "", rootTo, "only in source")
			}
			continue
		}
		if !mirror && dst != nil {
			l, err := os.Stat(from)
			if err != nil {
				return Plan{}, err
			}
			r, err := os.Stat(to)
			if err != nil {
				return Plan{}, err
			}
			if !l.ModTime().After(r.ModTime()) {
				if l.ModTime().Equal(r.ModTime()) {
					return Plan{}, fmt.Errorf("different content with equal timestamps: %s", row.Path)
				}
				continue
			}
		}
		add("copy", row.Path, rootFrom, rootTo, "new or changed source")
	}
	// 先建父目录、再复制文件，删除按子项优先；排序确定，方便审阅与后续执行。
	rank := func(a Action) int {
		switch a.Kind {
		case "create-directory":
			return 0
		case "copy":
			return 1
		case "delete":
			return 2
		default:
			return 3
		}
	}
	sort.SliceStable(p.Actions, func(i, j int) bool {
		x, y := p.Actions[i], p.Actions[j]
		if rank(x) != rank(y) {
			return rank(x) < rank(y)
		}
		if x.Kind == "remove-directory" {
			dx, dy := strings.Count(x.Path, "/"), strings.Count(y.Path, "/")
			if dx != dy {
				return dx > dy
			}
		}
		return x.Path < y.Path
	})
	for i := range p.Actions {
		item := &p.Actions[i]
		target, err := safePath(item.To, item.Path)
		if err != nil {
			return Plan{}, err
		}
		item.TargetVersion, err = fingerprint(ctx, target)
		if err != nil {
			return Plan{}, err
		}
		if item.Kind == "copy" {
			source, err := safePath(item.From, item.Path)
			if err != nil {
				return Plan{}, err
			}
			item.SourceVersion, err = fingerprint(ctx, source)
			if err != nil {
				return Plan{}, err
			}
		}
	}
	return p, nil
}
