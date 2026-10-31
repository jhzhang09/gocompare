package native

import (
	"context"
	"errors"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"gocompare/internal/document"
	"math/rand/v2"
	"strings"
	"sync/atomic"
	"testing"
)

func TestExternalComparisonRejectsObsoleteResults(t *testing.T) {
	before, after := editor.NewBuffer("a\nb\n"), editor.NewBuffer("a\nc\n")
	d := editor.DiffState{Old: before, New: after, ExternalComparison: true}
	if d.ComparisonReady() {
		t.Fatal("pending treated as equal")
	}
	request := editor.NewComparisonRequest(before, after)
	result, err := request.Compute(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	after.Edit(editor.Edit{Text: "new\n"})
	if d.ApplyComparison(result) {
		t.Fatal("accepted old version")
	}
	current, err := editor.NewComparisonRequest(before, after).Compute(context.Background())
	if err != nil || !d.ApplyComparison(current) {
		t.Fatalf("current rejected: %v", err)
	}
	d.New = editor.NewBuffer("a\nc\n")
	if d.ApplyComparison(result) {
		t.Fatal("accepted another buffer with same version")
	}
}

type cancelOnCheck struct {
	context.Context
	cancel context.CancelFunc
	checks atomic.Int32
	limit  int32
}

func (c *cancelOnCheck) Err() error {
	if c.checks.Add(1) >= c.limit {
		c.cancel()
	}
	return c.Context.Err()
}
func TestComparisonCancellationInsideAlgorithm(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	checked := &cancelOnCheck{Context: ctx, cancel: cancel, limit: 8}
	a, b := editor.NewBuffer(strings.Repeat("a\nb\nc\n", 20000)), editor.NewBuffer(strings.Repeat("c\na\nb\n", 20000))
	_, err := editor.NewComparisonRequest(a, b).Compute(checked)
	if !errors.Is(err, context.Canceled) || checked.checks.Load() < checked.limit {
		t.Fatalf("cancellation not observed: %v", err)
	}
	d := editor.DiffState{Old: a, New: b, ExternalComparison: true}
	if d.ComparisonReady() {
		t.Fatal("cancelled computation became ready")
	}
}

func TestComparisonResultsMatchSynchronousModel(t *testing.T) {
	random := rand.New(rand.NewPCG(7, 19))
	pieces := []string{"a\n", "重复\n", "🙂\n", "\n", "last"}
	for range 100 {
		var a, b strings.Builder
		for range random.IntN(30) {
			a.WriteString(pieces[random.IntN(len(pieces))])
		}
		for range random.IntN(30) {
			b.WriteString(pieces[random.IntN(len(pieces))])
		}
		before, after := editor.NewBuffer(a.String()), editor.NewBuffer(b.String())
		sync := editor.DiffState{Old: before, New: after}
		async := editor.DiffState{Old: before, New: after, ExternalComparison: true}
		result, err := editor.NewComparisonRequest(before, after).Compute(context.Background())
		if err != nil || !async.ApplyComparison(result) {
			t.Fatalf("result rejected: %v", err)
		}
		x, y := sync.Hunks(), async.Hunks()
		if len(x) != len(y) {
			t.Fatalf("hunks differ: %+v %+v", x, y)
		}
		for i := range x {
			if x[i] != y[i] {
				t.Fatalf("hunk differs: %+v %+v", x, y)
			}
		}
	}
}

func TestComparisonSchedulerRejectsOutOfOrderCompletion(t *testing.T) {
	a := testWorkspace(t)
	var queue []func()
	a.compareLaunch = func(fn func()) { queue = append(queue, fn) }
	a.recompute()
	if !a.compareRunning || len(queue) != 1 {
		t.Fatal("job not queued")
	}
	a.panes[0].buffer.Edit(editor.Edit{Text: "first\n"})
	a.refreshComparison()
	a.panes[1].buffer.Edit(editor.Edit{Text: "second\n"})
	a.refreshComparison()
	if len(queue) != 3 {
		t.Fatal("new versions did not supersede jobs")
	}
	queue[2]()
	if !a.diff.ComparisonReady() || a.compareRunning {
		t.Fatal("latest result not applied")
	}
	expected := a.diff.Hunks()
	queue[0]()
	queue[1]()
	got := a.diff.Hunks()
	if len(got) != len(expected) || !a.diff.ComparisonReady() {
		t.Fatal("old completion replaced latest")
	}
	// 新会话使用不同缓冲区，即使版本相同也不能接收旧结果。
	a.recompute()
	pending := queue[len(queue)-1]
	a.stopComparison()
	a.panes = [2]*pane{}
	pending()
	if a.compareRunning {
		t.Fatal("closed view retained job")
	}
}

func TestPendingComparisonDoesNotApplyOrShowOldMarkers(t *testing.T) {
	a := testWorkspace(t)
	var queued func()
	a.compareLaunch = func(fn func()) { queued = fn }
	a.panes[0].buffer.Edit(editor.Edit{Text: "pending\n"})
	a.refreshComparison()
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()
	before := a.panes[1].buffer.String()
	a.apply(1)
	if a.panes[1].buffer.String() != before {
		t.Fatal("applied stale hunks")
	}
	if a.diff.ComparisonReady() {
		t.Fatal("pending marked ready")
	}
	if !tt.HasText("正在后台计算") {
		t.Fatal("missing pending status")
	}
	queued()
	tt.Frame()
	if !a.diff.ComparisonReady() {
		t.Fatal("completed result missing")
	}
}

func BenchmarkPendingComparisonFrame(b *testing.B) {
	a := &workspaceState{compareLaunch: func(func()) {}}
	text := strings.Repeat("same line\n", 250000)
	a.install(&document.File{Path: "left.txt", Text: text}, &document.File{Path: "right.txt", Text: text + "changed\n"})
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		tt.Frame()
	}
}

func TestPatchViewClearsExternalEditPolicy(t *testing.T) {
	d := editor.DiffState{Old: editor.NewBuffer("old"), New: editor.NewBuffer("new"), Editable: true, OldEditable: true, ExternalComparison: true}
	patches, err := editor.ParsePatch("--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-old\n+new\n")
	if err != nil {
		t.Fatal(err)
	}
	d.SetPatch(patches[0])
	if d.Editable || d.OldEditable || d.ExternalComparison || !d.ComparisonReady() {
		t.Fatal("patch retained editable or pending state")
	}
	if len(d.Hunks()) != 1 {
		t.Fatal("patch hunks lost")
	}
}
