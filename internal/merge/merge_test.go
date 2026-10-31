package merge

import "testing"

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
