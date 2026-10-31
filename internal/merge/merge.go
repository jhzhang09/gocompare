// Package merge 实现与窗口无关的两方和三方文本合并。
// 每行连同其换行符一起比较，因此文件末尾有无换行也是一处可合并、可冲突的修改；不调用外部 diff 或 Git。
package merge

import (
	"errors"
	"fmt"
	"strings"
)

// Choice 是冲突的一次决策。
type Choice string

const (
	ChoiceLeft  Choice = "left"
	ChoiceRight Choice = "right"
	ChoiceBoth  Choice = "both"
	ChoiceBase  Choice = "base"
)

// Conflict 是结果中的一个未解决区域，按输出中的出现顺序编号。
type Conflict struct {
	Index int
	Base  string
	Left  string
	Right string
}

// AlignRow 是三栏对齐中的一行。
// 某一侧没有对应内容时，对应的 Has 字段为 false，文本为空；这与该侧存在的空行不同。
// Conflict 为 true 时，这一行属于 ConflictIndex 指向的未决策冲突。
type AlignRow struct {
	Base          string
	Left          string
	Right         string
	HasBase       bool
	HasLeft       bool
	HasRight      bool
	Conflict      bool
	ConflictIndex int
}

// Result 保存可继续决策的合并结构。Text 含冲突标记，供编辑器直接显示。
type Result struct {
	chunks    []chunk
	conflicts int
	hasBase   bool
	LeftName  string
	RightName string
}

type chunk struct {
	lines    []string
	conflict bool
	aligned  bool
	base     []string
	left     []string
	right    []string
}

// MergeTwo 没有共同祖先。相同行保留，不同区域全部成为冲突。
func MergeTwo(left, right string) (Result, error) {
	leftLines := splitLines(left)
	rightLines := splitLines(right)
	hunks, err := diffHunks(leftLines, rightLines)
	if err != nil {
		return Result{}, err
	}
	result := Result{LeftName: "left", RightName: "right"}
	posL, posR := 0, 0
	for _, h := range hunks {
		if h.a0 > posL {
			n := h.a0 - posL
			leftPart := leftLines[posL:h.a0]
			rightPart := rightLines[posR : posR+n]
			result.addResolved(leftPart, nil, leftPart, rightPart)
			posR += n
			posL = h.a0
		}
		result.addConflict(nil, leftLines[h.a0:h.a1], rightLines[h.b0:h.b1])
		posL, posR = h.a1, h.b1
	}
	if posL < len(leftLines) {
		n := len(leftLines) - posL
		leftPart := leftLines[posL:]
		rightPart := rightLines[posR : posR+n]
		result.addResolved(leftPart, nil, leftPart, rightPart)
		posL += n
		posR += n
	}
	if posL != len(leftLines) || posR != len(rightLines) {
		return Result{}, errors.New("two-way merge did not consume both inputs")
	}
	return result, nil
}

// MergeThree 以 base 为祖先。只在一侧变化时采用该侧；两侧做出相同修改时保留该修改；其余区域成为冲突。
func MergeThree(base, left, right string) (Result, error) {
	baseLines := splitLines(base)
	leftLines := splitLines(left)
	rightLines := splitLines(right)
	leftHunks, err := diffHunks(baseLines, leftLines)
	if err != nil {
		return Result{}, err
	}
	rightHunks, err := diffHunks(baseLines, rightLines)
	if err != nil {
		return Result{}, err
	}
	result := Result{hasBase: true, LeftName: "left", RightName: "right"}
	basePos, leftPos, rightPos := 0, 0, 0
	i, j := 0, 0
	for i < len(leftHunks) || j < len(rightHunks) {
		next := len(baseLines)
		if i < len(leftHunks) && leftHunks[i].a0 < next {
			next = leftHunks[i].a0
		}
		if j < len(rightHunks) && rightHunks[j].a0 < next {
			next = rightHunks[j].a0
		}
		if next < basePos {
			return Result{}, errors.New("merge hunks are out of order")
		}
		if next > basePos {
			n := next - basePos
			part := baseLines[basePos:next]
			result.addResolved(part, part, leftLines[leftPos:leftPos+n], rightLines[rightPos:rightPos+n])
			leftPos += n
			rightPos += n
			basePos = next
		}
		// 重叠或相邻的两侧修改合成一个区域；未修改的一侧按基准行原样补齐。
		endBase := basePos
		endLeft, endRight := leftPos, rightPos
		leftBase, rightBase := basePos, basePos
		tookLeft, tookRight := false, false
		for {
			advanced := false
			if i < len(leftHunks) && leftHunks[i].a0 <= endBase {
				endLeft, leftBase = absorb(endLeft, leftBase, leftHunks[i])
				endBase = max(endBase, leftHunks[i].a1)
				i++
				tookLeft = true
				advanced = true
			}
			if j < len(rightHunks) && rightHunks[j].a0 <= endBase {
				endRight, rightBase = absorb(endRight, rightBase, rightHunks[j])
				endBase = max(endBase, rightHunks[j].a1)
				j++
				tookRight = true
				advanced = true
			}
			if !advanced {
				break
			}
		}
		if leftBase < endBase {
			endLeft += endBase - leftBase
		}
		if rightBase < endBase {
			endRight += endBase - rightBase
		}
		if endLeft > len(leftLines) || endRight > len(rightLines) || endBase > len(baseLines) {
			return Result{}, errors.New("merge region exceeds input")
		}
		baseText := baseLines[basePos:endBase]
		leftText := leftLines[leftPos:endLeft]
		rightText := rightLines[rightPos:endRight]
		switch {
		case !tookLeft:
			result.addResolved(rightText, baseText, leftText, rightText)
		case !tookRight:
			result.addResolved(leftText, baseText, leftText, rightText)
		case equalLines(leftText, rightText):
			result.addResolved(leftText, baseText, leftText, rightText)
		default:
			result.addConflict(baseText, leftText, rightText)
		}
		basePos, leftPos, rightPos = endBase, endLeft, endRight
	}
	if basePos < len(baseLines) {
		part := baseLines[basePos:]
		result.addResolved(part, part, leftLines[leftPos:], rightLines[rightPos:])
	}
	return result, nil
}

// absorb 把一侧差异块并入当前区域，并补上块前面仍与基准相同的行。
func absorb(sidePos, basePos int, h hunk) (int, int) {
	if h.a0 > basePos {
		sidePos += h.a0 - basePos
	}
	return h.b1, h.a1
}

func (r *Result) addConflict(base, left, right []string) {
	r.chunks = append(r.chunks, chunk{
		conflict: true,
		aligned:  true,
		base:     cloneLines(base),
		left:     cloneLines(left),
		right:    cloneLines(right),
	})
	r.conflicts++
}

// addResolved 保存已经可以输出的区域，同时留下三侧原文，供对齐视图使用。
// lines 是输出；base 在两方合并时为 nil。
func (r *Result) addResolved(lines, base, left, right []string) {
	r.chunks = append(r.chunks, chunk{
		aligned: true,
		lines:   cloneLines(lines),
		base:    cloneLines(base),
		left:    cloneLines(left),
		right:   cloneLines(right),
	})
}

func cloneLines(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}
	return append([]string(nil), lines...)
}

// ConflictCount 返回尚未决策的冲突数。
func (r Result) ConflictCount() int { return r.conflicts }

// HasBase 报告这次合并是否有共同祖先。两方合并没有基准列。
func (r Result) HasBase() bool { return r.hasBase }

// Alignments 返回基准、左侧和右侧按行对齐的副本。
// 决策冲突不会改写这些原文；较短的一侧用 Has 字段标成没有这一行。
func (r Result) Alignments() []AlignRow {
	out := make([]AlignRow, 0)
	conflict := 0
	for _, item := range r.chunks {
		base, left, right := item.base, item.left, item.right
		if !item.aligned {
			left, right = item.lines, item.lines
			if r.hasBase {
				base = item.lines
			}
		}
		n := len(base)
		if len(left) > n {
			n = len(left)
		}
		if len(right) > n {
			n = len(right)
		}
		for i := 0; i < n; i++ {
			row := AlignRow{ConflictIndex: -1}
			if i < len(base) {
				row.Base = displayLine(base[i])
				row.HasBase = true
			}
			if i < len(left) {
				row.Left = displayLine(left[i])
				row.HasLeft = true
			}
			if i < len(right) {
				row.Right = displayLine(right[i])
				row.HasRight = true
			}
			if item.conflict {
				row.Conflict = true
				row.ConflictIndex = conflict
			}
			out = append(out, row)
		}
		if item.conflict {
			conflict++
		}
	}
	return out
}

// Conflicts 返回冲突副本。文本使用与 Text 相同的换行规则。
func (r Result) Conflicts() []Conflict {
	out := make([]Conflict, 0, r.conflicts)
	index := 0
	for _, chunk := range r.chunks {
		if !chunk.conflict {
			continue
		}
		out = append(out, Conflict{
			Index: index,
			Base:  strings.TrimSuffix(strings.Join(chunk.base, ""), "\n"),
			Left:  strings.TrimSuffix(strings.Join(chunk.left, ""), "\n"),
			Right: strings.TrimSuffix(strings.Join(chunk.right, ""), "\n"),
		})
		index++
	}
	return out
}

// Text 渲染稳定文本和冲突标记。标记只存在于显示文本，不写回输入文件。
// 冲突中某侧的末行没有换行时，在标记前补一个换行，保证标记独占一行。
func (r Result) Text() string {
	var b strings.Builder
	write := func(lines []string) {
		for _, line := range lines {
			if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
				b.WriteByte('\n')
			}
			b.WriteString(line)
		}
	}
	marker := func(text string) { write([]string{text + "\n"}) }
	for _, chunk := range r.chunks {
		if !chunk.conflict {
			write(chunk.lines)
			continue
		}
		marker("<<<<<<< " + r.LeftName)
		write(chunk.left)
		if r.hasBase {
			marker("||||||| base")
			write(chunk.base)
		}
		marker("=======")
		write(chunk.right)
		marker(">>>>>>> " + r.RightName)
	}
	return b.String()
}

// Take 用一次决策替换指定冲突。已手工改写 Text 后不能再调用，调用方需先比较文本。
func (r Result) Take(index int, choice Choice) (Result, error) {
	if index < 0 || index >= r.conflicts {
		return Result{}, fmt.Errorf("conflict index out of range")
	}
	next := r
	next.chunks = append([]chunk(nil), r.chunks...)
	seen := 0
	for i, item := range next.chunks {
		if !item.conflict {
			continue
		}
		if seen != index {
			seen++
			continue
		}
		var lines []string
		switch choice {
		case ChoiceLeft:
			lines = item.left
		case ChoiceRight:
			lines = item.right
		case ChoiceBase:
			if !r.hasBase {
				return Result{}, errors.New("two-way merge has no base")
			}
			lines = item.base
		case ChoiceBoth:
			lines = append(append([]string(nil), item.left...), item.right...)
		default:
			return Result{}, fmt.Errorf("unknown merge choice: %s", choice)
		}
		next.chunks[i] = chunk{
			lines:   cloneLines(lines),
			base:    item.base,
			left:    item.left,
			right:   item.right,
			aligned: item.aligned,
		}
		next.conflicts--
		return next, nil
	}
	return Result{}, errors.New("conflict not found")
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// splitLines 按行切分并保留每行的换行符；只有最后一行可能没有换行符。
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func displayLine(line string) string { return strings.TrimSuffix(line, "\n") }
