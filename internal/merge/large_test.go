package merge

import (
	"fmt"
	"strings"
	"testing"
)

func TestMergeThreeAboveLineLimit(t *testing.T) {
	const lines = 4500
	var b strings.Builder
	for i := 0; i < lines; i++ {
		fmt.Fprintf(&b, "L%04d\n", i)
	}
	base := b.String()
	left := strings.Replace(base, "L0010\n", "LEFT\n", 1)
	right := strings.Replace(base, "L0020\n", "RIGHT\n", 1)
	want := strings.Replace(left, "L0020\n", "RIGHT\n", 1)
	got, err := MergeThree(base, left, right)
	if err != nil {
		t.Fatal(err)
	}
	if got.ConflictCount() != 0 || got.Text() != want {
		t.Fatalf("conflicts=%d equal=%v", got.ConflictCount(), got.Text() == want)
	}
	oneSide, err := MergeThree(base, left, base)
	if err != nil || oneSide.ConflictCount() != 0 || oneSide.Text() != left {
		t.Fatal("one-sided large merge did not keep the changed side")
	}
}

func TestMergeRejectsUnboundedInput(t *testing.T) {
	text := strings.Repeat("L\n", maxScalableLines+1)
	if _, err := MergeThree(text, text, text); err == nil {
		t.Fatal("expected line limit")
	}
}
