package operations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPauseAndResumeSameJournal(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(left, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(left, "b.txt"), []byte("b"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := Preview(context.Background(), left, right, MirrorRight)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Actions) < 2 {
		t.Fatalf("actions: %+v", plan.Actions)
	}
	backup := t.TempDir()
	paused, err := ExecutePauseBefore(context.Background(), plan, backup, 1)
	if !errors.Is(err, ErrPaused) {
		t.Fatalf("pause: %v", err)
	}
	if !paused.Paused || paused.Next != 1 || paused.Plan == nil || len(paused.Completed) != 1 {
		t.Fatalf("paused report: paused %v next %d completed %d", paused.Paused, paused.Next, len(paused.Completed))
	}
	if _, err = os.Stat(filepath.Join(right, plan.Actions[0].Path)); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(right, plan.Actions[1].Path)); !os.IsNotExist(err) {
		t.Fatal("paused sync wrote the next file")
	}
	resumed, err := Resume(context.Background(), paused.Journal)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Journal != paused.Journal || resumed.Paused || len(resumed.Completed) != len(plan.Actions) {
		t.Fatalf("resume journal: %s paused %v completed %d", resumed.Journal, resumed.Paused, len(resumed.Completed))
	}
	for _, action := range plan.Actions {
		data, err := os.ReadFile(filepath.Join(right, filepath.FromSlash(action.Path)))
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(filepath.Join(left, filepath.FromSlash(action.Path)))
		if err != nil || string(data) != string(want) {
			t.Fatalf("%s: %q", action.Path, data)
		}
	}
	if _, err = Resume(context.Background(), paused.Journal); err == nil {
		t.Fatal("resumed a finished journal")
	}
}
