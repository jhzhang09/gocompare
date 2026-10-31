package textcompare

import (
	"strings"
	"testing"
)

func TestRulesMatchRepresentativeInput(t *testing.T) {
	equal := []struct {
		name  string
		left  string
		right string
		rules Rules
	}{
		{"space", "a b\n", "ab\n", Rules{IgnoreSpace: true}},
		{"case", "Ä\nB\n", "ä\nb\n", Rules{IgnoreCase: true}},
		{"comment", "code\n// note\n", "code\n", Rules{IgnoreComments: true}},
		{"crlf", "a\r\nb\r\n", "a\nb\n", Rules{}},
		{"pattern", "a12b\n", "ab\n", Rules{IgnorePattern: `\d+`}},
		{"columns", "中文ab\n", "中\n", Rules{Columns: [][2]int{{0, 1}}}},
		{"replace", "foo\n", "bar\n", Rules{ReplaceFrom: "foo", ReplaceTo: "bar"}},
	}
	for _, tc := range equal {
		got, err := Compare(tc.left, tc.right, tc.rules, Myers, nil)
		if err != nil || !got.Equal || len(got.Hunks) != 0 {
			t.Fatalf("%s: %+v %v", tc.name, got, err)
		}
	}
	got, err := Compare("a b\nx\n", "ab\ny\n", Rules{IgnoreSpace: true}, Myers, nil)
	if err != nil || got.Equal || len(got.Hunks) != 1 || got.Hunks[0] != (Hunk{1, 2, 1, 2}) {
		t.Fatalf("partial space: %+v %v", got, err)
	}
	got, err = Compare("a // keep\n", "a\n", Rules{IgnoreComments: true}, Myers, nil)
	if err != nil || got.Equal {
		t.Fatal("inline comment must stay visible")
	}
	if _, err = Compare("a\n", "a\n", Rules{IgnorePattern: "("}, Myers, nil); err == nil {
		t.Fatal("invalid pattern accepted")
	}
}

func TestReplaceAll(t *testing.T) {
	got, n := ReplaceAll("a.a.a", ".", "-")
	if got != "a-a-a" || n != 2 {
		t.Fatalf("replace: %q %d", got, n)
	}
	got, n = ReplaceAll("abc", "", "x")
	if got != "abc" || n != 0 {
		t.Fatalf("empty find: %q %d", got, n)
	}
	got, n = ReplaceAll("aaa", "aa", "b")
	if got != "ba" || n != 1 {
		t.Fatalf("overlap: %q %d", got, n)
	}
}

func TestAlgorithmsReproduceRight(t *testing.T) {
	cases := [][2]string{
		{"a\nb\nc\n", "a\nx\nc\n"},
		{"a\nb\nc\nd\n", "a\nx\nc\ny\n"},
		{"a\nb\nc\n", "a\nb\nc\nx\n"},
		{"x\na\nb\nc\n", "a\nb\nc\n"},
		{"a\n", "b\n"},
		{"", "a\n"},
		{"a\n", ""},
		{"", ""},
		{"same\nUNIQUE\nother\n", "same\nother\n"},
		{"keep\nmove\ntail\n", "move\nkeep\ntail\n"},
	}
	for _, algorithm := range []Algorithm{Myers, Patience, Histogram} {
		for _, tc := range cases {
			got, err := Compare(tc[0], tc[1], Rules{}, algorithm, nil)
			if err != nil {
				t.Fatalf("%s: %v", algorithm, err)
			}
			if !sameLines(applyHunks(t, splitPlain(tc[0]), got.Hunks, splitPlain(tc[1])), splitPlain(tc[1])) {
				t.Fatalf("%s did not reproduce right\nleft %q\nright %q\nhunks %+v", algorithm, tc[0], tc[1], got.Hunks)
			}
		}
	}
	got, err := Compare("a\nb\nc\n", "a\nx\nc\n", Rules{}, Myers, nil)
	if err != nil || len(got.Hunks) != 1 || got.Hunks[0] != (Hunk{1, 2, 1, 2}) {
		t.Fatalf("myers replacement: %+v %v", got.Hunks, err)
	}
	got, err = Compare("a\nb\nc\nd\n", "a\nx\nc\ny\n", Rules{}, Myers, nil)
	if err != nil || len(got.Hunks) != 2 || got.Hunks[0] != (Hunk{1, 2, 1, 2}) || got.Hunks[1] != (Hunk{3, 4, 3, 4}) {
		t.Fatalf("myers kept unchanged line inside a hunk: %+v %v", got.Hunks, err)
	}
	if _, err = Compare("a\n", "a\n", Rules{}, "unknown", nil); err == nil {
		t.Fatal("unknown algorithm accepted")
	}
}

func TestPatienceKeepsUniqueLine(t *testing.T) {
	left := "A\nUNIQUE\nB\nC\n"
	right := "A\nX\nUNIQUE\nC\n"
	got, err := Compare(left, right, Rules{}, Patience, nil)
	if err != nil {
		t.Fatal(err)
	}
	if covers(got.Hunks, 1) {
		t.Fatalf("UNIQUE line is inside a hunk: %+v", got.Hunks)
	}
	if !sameLines(applyHunks(t, splitPlain(left), got.Hunks, splitPlain(right)), splitPlain(right)) {
		t.Fatal("patience apply failed")
	}
}

func TestHistogramPrefersEarlierCommonLine(t *testing.T) {
	// A、B、C 的出现次数都是 1+1。同分必须取更小的左侧下标 A，而不是 B。
	got, err := Compare("A\nB\nC\n", "B\nA\nC\n", Rules{}, Histogram, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []Hunk{{0, 0, 0, 1}, {1, 2, 2, 2}}
	if len(got.Hunks) != len(want) || got.Hunks[0] != want[0] || got.Hunks[1] != want[1] {
		t.Fatalf("histogram tie: %+v", got.Hunks)
	}
}

func TestAnchorAlignsAndReleaseMoves(t *testing.T) {
	left := "keep\nmove\ntail\n"
	right := "move\nkeep\ntail\n"
	anchored, err := Compare(left, right, Rules{}, Myers, []Anchor{{0, 1}})
	if err != nil {
		t.Fatal(err)
	}
	if covers(anchored.Hunks, 0) {
		t.Fatalf("anchored line is inside a hunk: %+v", anchored.Hunks)
	}
	free, err := Compare(left, right, Rules{}, Myers, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !covers(free.Hunks, 0) {
		t.Fatalf("releasing the anchor did not change alignment: %+v", free.Hunks)
	}
	if _, err = Compare(left, right, Rules{}, Myers, []Anchor{{1, 1}, {0, 2}}); err == nil {
		t.Fatal("out-of-order anchor accepted")
	}
}

func splitPlain(text string) []string {
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

func applyHunks(t *testing.T, left []string, hunks []Hunk, right []string) []string {
	t.Helper()
	var out []string
	at := 0
	for _, h := range hunks {
		if h.LeftStart < at || h.LeftEnd < h.LeftStart || h.LeftEnd > len(left) || h.RightStart < 0 || h.RightEnd < h.RightStart || h.RightEnd > len(right) {
			t.Fatalf("invalid hunk %+v at %d", h, at)
		}
		out = append(out, left[at:h.LeftStart]...)
		out = append(out, right[h.RightStart:h.RightEnd]...)
		at = h.LeftEnd
	}
	out = append(out, left[at:]...)
	return out
}

func sameLines(a, b []string) bool {
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

func covers(hunks []Hunk, line int) bool {
	for _, h := range hunks {
		if line >= h.LeftStart && line < h.LeftEnd {
			return true
		}
	}
	return false
}
