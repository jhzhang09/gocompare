package merge

import (
	"math/rand"
	"strings"
	"testing"
)

func TestDiffHunks(t *testing.T) {
	h, err := diffHunks([]string{"a", "b", "c"}, []string{"a", "x", "c"})
	if err != nil || len(h) != 1 || h[0] != (hunk{1, 2, 1, 2}) {
		t.Fatalf("%+v %v", h, err)
	}
	h, err = diffHunks([]string{"a", "b"}, []string{"a"})
	if err != nil || len(h) != 1 || h[0] != (hunk{1, 2, 1, 1}) {
		t.Fatalf("delete %+v %v", h, err)
	}
	h, err = diffHunks(nil, []string{"a"})
	if err != nil || len(h) != 1 || h[0] != (hunk{0, 0, 0, 1}) {
		t.Fatalf("insert %+v %v", h, err)
	}
	h, err = diffHunks([]string{"a", "b", "c"}, []string{"a", "b", "c"})
	if err != nil || len(h) != 0 {
		t.Fatalf("same %+v %v", h, err)
	}
}

func TestMergeThree(t *testing.T) {
	got, err := MergeThree("a\nb\nc\n", "a\nleft\nc\n", "a\nb\nc\n")
	if err != nil || got.ConflictCount() != 0 || got.Text() != "a\nleft\nc\n" {
		t.Fatalf("left only: %q %d %v", got.Text(), got.ConflictCount(), err)
	}
	got, err = MergeThree("a\nb\nc\n", "a\nb\nc\n", "a\nright\nc\n")
	if err != nil || got.ConflictCount() != 0 || got.Text() != "a\nright\nc\n" {
		t.Fatalf("right only: %q %v", got.Text(), err)
	}
	got, err = MergeThree("a\nb\nc\n", "a\nsame\nc\n", "a\nsame\nc\n")
	if err != nil || got.ConflictCount() != 0 || got.Text() != "a\nsame\nc\n" {
		t.Fatalf("same edit: %q %v", got.Text(), err)
	}
	got, err = MergeThree("a\nb\nc\n", "a\nleft\nc\n", "a\nright\nc\n")
	if err != nil || got.ConflictCount() != 1 || !contains(got.Text(), "<<<<<<< left\nleft\n||||||| base\nb\n=======\nright\n>>>>>>> right\n") {
		t.Fatalf("conflict: %q %v", got.Text(), err)
	}
	taken, err := got.Take(0, ChoiceLeft)
	if err != nil || taken.ConflictCount() != 0 || taken.Text() != "a\nleft\nc\n" {
		t.Fatalf("take left: %q %v", taken.Text(), err)
	}
	got, err = MergeThree("a\nb\nc\nd\n", "a\nL\nc\nd\n", "a\nb\nc\nR\n")
	if err != nil || got.ConflictCount() != 0 || got.Text() != "a\nL\nc\nR\n" {
		t.Fatalf("apart: %q %d %v", got.Text(), got.ConflictCount(), err)
	}
	got, err = MergeThree("a\nb\nc\nd\n", "a\nL\nb\nc\nd\n", "a\nb\nR\nd\n")
	if err != nil || got.ConflictCount() != 0 || got.Text() != "a\nL\nb\nR\nd\n" {
		t.Fatalf("apart insert: %q %d %v", got.Text(), got.ConflictCount(), err)
	}
	got, err = MergeThree("a\nb\nc\nd\n", "a\nL1\nL2\nd\n", "a\nb\nR\nd\n")
	if err != nil || got.ConflictCount() != 1 {
		t.Fatalf("overlap: %q %d %v", got.Text(), got.ConflictCount(), err)
	}
	baseTaken, err := got.Take(0, ChoiceBase)
	if err != nil || baseTaken.Text() != "a\nb\nc\nd\n" {
		t.Fatalf("base: %q %v", baseTaken.Text(), err)
	}
}

func TestMergeTwo(t *testing.T) {
	got, err := MergeTwo("a\nb\n", "a\nb\n")
	if err != nil || got.ConflictCount() != 0 || got.Text() != "a\nb\n" {
		t.Fatalf("same: %q %v", got.Text(), err)
	}
	got, err = MergeTwo("a\nleft\n", "a\nright\n")
	if err != nil || got.ConflictCount() != 1 || contains(got.Text(), "|||||||") {
		t.Fatalf("two-way: %q %v", got.Text(), err)
	}
	both, err := got.Take(0, ChoiceBoth)
	if err != nil || both.Text() != "a\nleft\nright\n" {
		t.Fatalf("both: %q %v", both.Text(), err)
	}
	if _, err = got.Take(0, ChoiceBase); err == nil {
		t.Fatal("two-way base accepted")
	}
}

func TestMergePreservesMissingNewline(t *testing.T) {
	got, err := MergeThree("a", "a", "a")
	if err != nil || got.Text() != "a" {
		t.Fatalf("%q %v", got.Text(), err)
	}
}

func contains(s, part string) bool {
	return len(s) >= len(part) && (s == part || len(part) == 0 || (func() bool { return stringIndex(s, part) >= 0 })())
}

func stringIndex(s, part string) int {
	for i := 0; i+len(part) <= len(s); i++ {
		if s[i:i+len(part)] == part {
			return i
		}
	}
	return -1
}

// 文件末尾的换行与普通行一样参与三方判断：只有一侧改动时采用该侧，删空文件得到空文本。
func TestMergeTrailingNewlineAndEmpty(t *testing.T) {
	cases := []struct{ base, left, right, want string }{
		{"a\nb\n", "a\nb", "a\nb\n", "a\nb"},
		{"a\nb", "a\nb\n", "a\nb", "a\nb\n"},
		{"d\nd\n", "", "d\nd\n", ""},
		{"a\nb\n", "a\nb\n", "a\nc", "a\nc"},
	}
	for _, c := range cases {
		got, err := MergeThree(c.base, c.left, c.right)
		if err != nil || got.ConflictCount() != 0 || got.Text() != c.want {
			t.Fatalf("%q %q %q -> %q conflicts=%d err=%v, want %q", c.base, c.left, c.right, got.Text(), got.ConflictCount(), err, c.want)
		}
	}
	two, err := MergeTwo("a\nb", "a\nb\n")
	if err != nil || two.ConflictCount() != 1 {
		t.Fatalf("two-way trailing newline difference must conflict: %q %d", two.Text(), two.ConflictCount())
	}
	// 冲突一侧末行没有换行时，标记仍独占一行。
	if want := "a\n<<<<<<< left\nb\n=======\nb\n>>>>>>> right\n"; two.Text() != want {
		t.Fatalf("markers: %q", two.Text())
	}
	taken, _ := two.Take(0, ChoiceLeft)
	if taken.Text() != "a\nb" {
		t.Fatalf("take left keeps missing newline: %q", taken.Text())
	}
}

// 随机检查：只有一侧修改时结果等于该侧，且没有冲突。
func TestMergeOneSidedProperty(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	gen := func(n int) string {
		var b strings.Builder
		for i := 0; i < n; i++ {
			b.WriteByte(byte('a' + r.Intn(3)))
			if i < n-1 || r.Intn(2) == 0 {
				b.WriteByte('\n')
			}
		}
		return b.String()
	}
	for i := 0; i < 20000; i++ {
		base, side := gen(r.Intn(6)), gen(r.Intn(6))
		for _, c := range [][3]string{{base, side, base}, {base, base, side}, {base, side, side}} {
			got, err := MergeThree(c[0], c[1], c[2])
			if err != nil || got.ConflictCount() != 0 || got.Text() != side {
				t.Fatalf("%q %q %q -> %q conflicts=%d err=%v", c[0], c[1], c[2], got.Text(), got.ConflictCount(), err)
			}
		}
	}
}
