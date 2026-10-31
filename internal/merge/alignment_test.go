package merge

import "testing"

func TestAlignmentsKeepThreeSides(t *testing.T) {
	got, err := MergeThree("a\nb\n", "a\nleft\n", "a\nright\n")
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasBase() || got.ConflictCount() != 1 {
		t.Fatalf("base %v conflicts %d", got.HasBase(), got.ConflictCount())
	}
	rows := got.Alignments()
	if len(rows) != 2 {
		t.Fatalf("rows: %+v", rows)
	}
	if rows[0].Conflict || rows[0].Base != "a" || rows[0].Left != "a" || rows[0].Right != "a" || !rows[0].HasBase || !rows[0].HasLeft || !rows[0].HasRight {
		t.Fatalf("common: %+v", rows[0])
	}
	if !rows[1].Conflict || rows[1].ConflictIndex != 0 || rows[1].Base != "b" || rows[1].Left != "left" || rows[1].Right != "right" {
		t.Fatalf("conflict: %+v", rows[1])
	}
	taken, err := got.Take(0, ChoiceLeft)
	if err != nil || taken.Text() != "a\nleft\n" || taken.ConflictCount() != 0 {
		t.Fatalf("take: %q %d %v", taken.Text(), taken.ConflictCount(), err)
	}
	after := taken.Alignments()
	if len(after) != 2 || after[1].Conflict || after[1].Base != "b" || after[1].Left != "left" || after[1].Right != "right" {
		t.Fatalf("alignment changed after take: %+v", after)
	}
}

func TestAlignmentsShowUnchangedSideAndPadding(t *testing.T) {
	one, err := MergeThree("a\nb\nc\n", "a\nleft\nc\n", "a\nb\nc\n")
	if err != nil || one.ConflictCount() != 0 {
		t.Fatalf("one side: %v %d", err, one.ConflictCount())
	}
	rows := one.Alignments()
	if len(rows) != 3 || rows[1].Conflict || rows[1].Base != "b" || rows[1].Left != "left" || rows[1].Right != "b" {
		t.Fatalf("one side rows: %+v", rows)
	}
	padded, err := MergeThree("a\n", "a\nL1\nL2\n", "a\nR\n")
	if err != nil || padded.ConflictCount() != 1 {
		t.Fatalf("pad: %v %d %q", err, padded.ConflictCount(), padded.Text())
	}
	rows = padded.Alignments()
	if len(rows) != 3 {
		t.Fatalf("pad rows: %+v", rows)
	}
	if rows[1].Left != "L1" || rows[1].Right != "R" || rows[1].HasBase {
		t.Fatalf("first inserted: %+v", rows[1])
	}
	if rows[2].Left != "L2" || rows[2].HasLeft != true || rows[2].HasRight || rows[2].HasBase || !rows[2].Conflict {
		t.Fatalf("padding: %+v", rows[2])
	}
}

func TestTwoWayAlignmentHasNoBase(t *testing.T) {
	got, err := MergeTwo("a\nleft\n", "a\nright\n")
	if err != nil {
		t.Fatal(err)
	}
	if got.HasBase() {
		t.Fatal("two-way reported a base")
	}
	rows := got.Alignments()
	if len(rows) != 2 || rows[0].HasBase || rows[0].Left != "a" || rows[0].Right != "a" || rows[0].Conflict {
		t.Fatalf("common: %+v", rows)
	}
	if !rows[1].Conflict || rows[1].HasBase || rows[1].Left != "left" || rows[1].Right != "right" {
		t.Fatalf("conflict: %+v", rows)
	}
}
