package merge

import (
	"fmt"

	"gocompare/internal/textcompare"
)

// maxMergeLines 是动态规划对齐表的行数上限。超过后改用线性空间的 Myers，直到 maxScalableLines。
const maxMergeLines = 4000

// maxScalableLines 是单份输入仍接受 Myers 对齐的行数上限。
const maxScalableLines = 100000

type hunk struct {
	a0, a1 int
	b0, b1 int
}

func diffHunks(a, b []string) ([]hunk, error) {
	if len(a) > maxScalableLines || len(b) > maxScalableLines {
		return nil, fmt.Errorf("merge input exceeds %d lines", maxScalableLines)
	}
	if len(a) > maxMergeLines || len(b) > maxMergeLines {
		return scalableHunks(a, b)
	}
	n, m := len(a), len(b)
	if n == 0 && m == 0 {
		return nil, nil
	}
	const (
		keep = byte(0)
		del  = byte(1)
		ins  = byte(2)
	)
	choice := make([]byte, (n+1)*(m+1))
	at := func(i, j int) int { return i*(m+1) + j }
	for j := 1; j <= m; j++ {
		choice[at(0, j)] = ins
	}
	for i := 1; i <= n; i++ {
		choice[at(i, 0)] = del
	}
	prev := make([]int, m+1)
	cur := make([]int, m+1)
	for i := 1; i <= n; i++ {
		for j := range cur {
			cur[j] = 0
		}
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				cur[j] = prev[j-1] + 1
				choice[at(i, j)] = keep
				continue
			}
			if prev[j] >= cur[j-1] {
				cur[j] = prev[j]
				choice[at(i, j)] = del
				continue
			}
			cur[j] = cur[j-1]
			choice[at(i, j)] = ins
		}
		prev, cur = cur, prev
	}
	type step struct {
		kind byte
		a, b int
	}
	i, j := n, m
	steps := make([]step, 0, n+m)
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && choice[at(i, j)] == keep:
			i--
			j--
			steps = append(steps, step{kind: keep, a: i, b: j})
		case j > 0 && (i == 0 || choice[at(i, j)] == ins):
			j--
			steps = append(steps, step{kind: ins, b: j, a: i})
		default:
			i--
			steps = append(steps, step{kind: del, a: i, b: j})
		}
	}
	var hunks []hunk
	var current *hunk
	flush := func() {
		if current != nil {
			hunks = append(hunks, *current)
			current = nil
		}
	}
	for k := len(steps) - 1; k >= 0; k-- {
		step := steps[k]
		if step.kind == keep {
			flush()
			continue
		}
		if current == nil {
			current = &hunk{a0: step.a, a1: step.a, b0: step.b, b1: step.b}
		}
		if step.kind == del {
			current.a1 = step.a + 1
		} else {
			current.b1 = step.b + 1
		}
	}
	flush()
	return hunks, nil
}

func scalableHunks(a, b []string) ([]hunk, error) {
	result, err := textcompare.DiffLines(a, b, textcompare.Myers)
	if err != nil {
		return nil, err
	}
	out := make([]hunk, len(result.Hunks))
	for i, h := range result.Hunks {
		out[i] = hunk{a0: h.LeftStart, a1: h.LeftEnd, b0: h.RightStart, b1: h.RightEnd}
	}
	return out, nil
}
