package integrate

import (
	"runtime"
	"testing"
)

func TestPlatformActionsStayHiddenOffWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		if len(PlatformActions()) == 0 {
			t.Fatal("windows actions empty")
		}
		return
	}
	if PlatformActions() != nil {
		t.Fatalf("non-windows actions: %v", PlatformActions())
	}
}

func TestLaunchArgs(t *testing.T) {
	if got := FileManagerArgs("L", "R"); len(got) != 2 || got[0] != "L" || got[1] != "R" {
		t.Fatalf("file manager: %v", got)
	}
	if got := DifftoolArgs("L", "R"); len(got) != 3 || got[0] != "difftool" {
		t.Fatalf("difftool: %v", got)
	}
	got := MergetoolArgs("B", "L", "R", "O")
	want := []string{"mergetool", "--base", "B", "--output", "O", "L", "R"}
	if len(got) != len(want) {
		t.Fatalf("%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%v", got)
		}
	}
}
