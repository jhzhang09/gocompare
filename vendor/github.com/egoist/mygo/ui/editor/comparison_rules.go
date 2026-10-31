package editor

import (
	"context"
	"errors"
	"github.com/egoist/mygo/internal/diff"
	"strings"
	"unicode"
)

// ComputeRules 在原始快照的行坐标中应用空白规则及单调锚点。
// 结果保留缓冲区身份与版本；不把规范化内容写入缓冲区。
func (r ComparisonRequest) ComputeRules(ctx context.Context, ignoreSpace bool, anchors [][2]int, mode DiffInline) (Comparison, error) {
	if r.old == nil || r.new == nil {
		return Comparison{}, errors.New("invalid comparison request")
	}
	if mode > DiffInlineNone {
		return Comparison{}, errors.New("invalid inline mode")
	}
	if err := ctx.Err(); err != nil {
		return Comparison{}, err
	}
	old, new := r.before.t, r.after.t
	a, b := ropeLines(old, 0, old.LineCount()), ropeLines(new, 0, new.LineCount())
	if ignoreSpace {
		for _, lines := range [][]string{a, b} {
			for i, s := range lines {
				if err := ctx.Err(); err != nil {
					return Comparison{}, err
				}
				lines[i] = strings.Map(func(c rune) rune {
					if unicode.IsSpace(c) {
						return -1
					}
					return c
				}, s)
			}
		}
	}
	points := append(append([][2]int(nil), anchors...), [2]int{len(a), len(b)})
	left, right := 0, 0
	var hunks []diff.Hunk
	for _, point := range points {
		if point[0] < left || point[1] < right || point[0] > len(a) || point[1] > len(b) {
			return Comparison{}, errors.New("anchor out of order or out of range")
		}
		part, err := diff.LinesContext(ctx, a[left:point[0]], b[right:point[1]])
		if err != nil {
			return Comparison{}, err
		}
		for _, h := range part {
			h.A0 += left
			h.A1 += left
			h.B0 += right
			h.B1 += right
			hunks = append(hunks, h)
		}
		left, right = point[0], point[1]
	}
	model := diffModel{oldText: old, newText: new, hunks: hunks}
	for i := range hunks {
		if _, err := model.wordsOfContext(ctx, i, mode); err != nil {
			return Comparison{}, err
		}
	}
	return Comparison{request: r, hunks: hunks, inline: mode, words: model.words}, nil
}
