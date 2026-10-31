// Package textcompare 提供与窗口无关的文本规则、算法和手工对齐。
package textcompare

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Algorithm 选择行对齐策略。三种算法都必须能还原右侧文本。
type Algorithm string

const (
	Myers     Algorithm = "myers"
	Patience  Algorithm = "patience"
	Histogram Algorithm = "histogram"
)

// Rules 在比较前规范化文本。零值表示逐行精确比较。
type Rules struct {
	IgnoreSpace    bool
	IgnoreCase     bool
	IgnoreComments bool
	CommentPrefix  string
	IgnorePattern  string
	Columns        [][2]int
	ReplaceFrom    string
	ReplaceTo      string
}

// Anchor 强制把左右两侧的逻辑行视为同一位置，索引针对规范化后的行。
type Anchor struct {
	Left  int
	Right int
}

// Hunk 是左侧 [LeftStart, LeftEnd) 被右侧 [RightStart, RightEnd) 替换的范围。
type Hunk struct {
	LeftStart  int
	LeftEnd    int
	RightStart int
	RightEnd   int
}

// Result 是规则规范化之后的差异。Equal 表示没有差异块。
type Result struct {
	Hunks []Hunk
	Equal bool
}

// Compare 按规则规范化后分段比较。锚点必须递增，且落在规范化行范围内。
func Compare(left, right string, rules Rules, algorithm Algorithm, anchors []Anchor) (Result, error) {
	a, err := normalize(left, rules)
	if err != nil {
		return Result{}, err
	}
	b, err := normalize(right, rules)
	if err != nil {
		return Result{}, err
	}
	if algorithm == "" {
		algorithm = Myers
	}
	var hunks []Hunk
	prevL, prevR := 0, 0
	points := append([]Anchor(nil), anchors...)
	points = append(points, Anchor{len(a), len(b)})
	for _, point := range points {
		if point.Left < prevL || point.Right < prevR || point.Left > len(a) || point.Right > len(b) {
			return Result{}, fmt.Errorf("anchor out of order: %+v", point)
		}
		part, err := diffLines(a[prevL:point.Left], b[prevR:point.Right], algorithm)
		if err != nil {
			return Result{}, err
		}
		for _, h := range part {
			hunks = append(hunks, Hunk{h.LeftStart + prevL, h.LeftEnd + prevL, h.RightStart + prevR, h.RightEnd + prevR})
		}
		prevL, prevR = point.Left, point.Right
	}
	return Result{Hunks: hunks, Equal: len(hunks) == 0}, nil
}

// DiffLines 比较已经分行的文本，不再做规则规范化和换行折叠。
// 合并大文件时用它保持行号与调用方的行切片一致。
func DiffLines(a, b []string, algorithm Algorithm) (Result, error) {
	if algorithm == "" {
		algorithm = Myers
	}
	hunks, err := diffLines(a, b, algorithm)
	if err != nil {
		return Result{}, err
	}
	return Result{Hunks: hunks, Equal: len(hunks) == 0}, nil
}

// ReplaceAll 替换全部非空查找串，返回新文本和替换次数。
func ReplaceAll(text, find, repl string) (string, int) {
	if find == "" {
		return text, 0
	}
	return strings.ReplaceAll(text, find, repl), strings.Count(text, find)
}

func normalize(text string, rules Rules) ([]string, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if rules.ReplaceFrom != "" {
		text = strings.ReplaceAll(text, rules.ReplaceFrom, rules.ReplaceTo)
	}
	var pattern *regexp.Regexp
	if rules.IgnorePattern != "" {
		var err error
		pattern, err = regexp.Compile(rules.IgnorePattern)
		if err != nil {
			return nil, fmt.Errorf("ignore pattern: %w", err)
		}
	}
	prefix := rules.CommentPrefix
	if rules.IgnoreComments && prefix == "" {
		prefix = "//"
	}
	raw := split(text)
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		if rules.IgnoreComments && strings.HasPrefix(strings.TrimLeftFunc(line, unicode.IsSpace), prefix) {
			continue
		}
		if pattern != nil {
			line = pattern.ReplaceAllString(line, "")
		}
		if len(rules.Columns) > 0 {
			line = columns(line, rules.Columns)
		}
		if rules.IgnoreCase {
			line = strings.ToLower(line)
		}
		if rules.IgnoreSpace {
			line = stripSpace(line)
		}
		out = append(out, line)
	}
	return out, nil
}

func split(text string) []string {
	if text == "" {
		return nil
	}
	newline := strings.HasSuffix(text, "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if !newline {
		return lines
	}
	return lines
}

func columns(line string, spans [][2]int) string {
	runes := []rune(line)
	var b strings.Builder
	for _, span := range spans {
		start, end := span[0], span[1]
		if start < 0 {
			start = 0
		}
		if end > len(runes) {
			end = len(runes)
		}
		if start < end {
			b.WriteString(string(runes[start:end]))
		}
	}
	return b.String()
}

func stripSpace(line string) string {
	var b strings.Builder
	for _, r := range line {
		if !unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
