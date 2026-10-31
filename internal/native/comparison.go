package native

import (
	"context"

	"github.com/egoist/mygo/ui/editor"
	"gocompare/internal/i18n"
	"gocompare/internal/textcompare"
)

func (a *workspaceState) stopComparison() {
	a.compareGeneration++
	if a.compareCancel != nil {
		a.compareCancel()
		a.compareCancel = nil
	}
	a.compareRunning = false
}

// refreshComparison 只在输入版本改变时启动任务，不能在绘制回调中触发。
func (a *workspaceState) refreshComparison() {
	panes := a.activePanes()
	diff := a.activeDiff()
	if panes[0] == nil || !diff.ExternalComparison {
		return
	}
	buffers := [2]*editor.Buffer{panes[0].buffer, panes[1].buffer}
	versions := [2]uint64{buffers[0].Version(), buffers[1].Version()}
	anchors := append([]textcompare.Anchor(nil), a.anchors...)
	if a.ignoreBlankLines {
		// 锚点是原文行号，与去除空行后的行号不对应，只读副本上不使用。
		anchors = nil
	}
	if a.compareRequested && a.compareBuffers == buffers && a.compareVersions == versions && a.compareInline == diff.Inline && a.compareIgnoreSpace == a.ignoreSpace && sameAnchors(a.compareAnchors, anchors) {
		return
	}
	a.stopComparison()
	generation := a.compareGeneration
	a.compareRequested = true
	a.compareAnchors = anchors
	a.compareIgnoreSpace = a.ignoreSpace
	ignore := a.ignoreSpace
	a.compareBuffers = buffers
	a.compareVersions = versions
	a.compareInline = diff.Inline
	inline := diff.Inline
	a.compareRunning = true
	a.compareErr = nil
	request := editor.NewComparisonRequest(buffers[0], buffers[1])
	ctx, cancel := context.WithCancel(context.Background())
	a.compareCancel = cancel
	run := func() {
		defer cancel()
		points := make([][2]int, len(anchors))
		for i, p := range anchors {
			points[i] = [2]int{p.Left, p.Right}
		}
		result, err := request.ComputeRules(ctx, ignore, points, inline)
		if err == nil {
			err = ctx.Err()
		}
		a.publish(func() {
			if generation != a.compareGeneration {
				return
			}
			a.compareRunning = false
			a.compareCancel = nil
			if err != nil {
				a.compareErr = err
				return
			}
			activeDiff := a.activeDiff()
			if !activeDiff.ApplyComparison(result) {
				return
			}
			a.compareAnchors = anchors
			a.err = ""
			n := len(result.Hunks())
			if ignore {
				a.message = i18n.TArgs("status.ruleDiffs", n)
				if n == 0 {
					a.message = i18n.T("status.ruleSame")
				}
			} else {
				a.message = i18n.TArgs("status.compareDone", n)
				if len(anchors) > 0 {
					a.message = i18n.TArgs("status.aligned", len(anchors), n)
				}
			}
		})
	}

	// 无窗口测试同步调用相同结果协议，异步调度另有专门回归测试。
	if a.compareLaunch != nil {
		a.compareLaunch(run)
	} else if a.win == nil {
		run()
	} else {
		go run()
	}
}
func (a *workspaceState) recompute() { a.compareRequested = false; a.refreshComparison() }

func sameAnchors(a, b []textcompare.Anchor) bool {
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

// markAnchorSide 记录当前活动侧的光标行。侧必须与光标所在侧一致。
func (a *workspaceState) markAnchorSide(side editor.DiffSide) {
	if a.ignoreBlankLines {
		a.err = i18n.T("err.anchorBlank")
		return
	}
	if a.panes[0] == nil || !a.diff.ComparisonReady() {
		a.err = i18n.T("err.compareFirst")
		return
	}
	if a.diff.ActiveSide() != side {
		a.err = i18n.T("err.cursorSide")
		return
	}
	line := a.diff.CursorPos().Line
	if side == editor.DiffOld {
		a.anchorLeft, a.anchorLeftSet = line, true
	} else {
		a.anchorRight, a.anchorRightSet = line, true
	}
	a.err = ""
	left, right := i18n.T("anchor.unset"), i18n.T("anchor.unset")
	if a.anchorLeftSet {
		left = i18n.TArgs("anchor.line", a.anchorLeft+1)
	}
	if a.anchorRightSet {
		right = i18n.TArgs("anchor.line", a.anchorRight+1)
	}
	a.message = i18n.TArgs("anchor.marked", left, right)
}

// commitAnchor 把已标记的左右行追加为锚点，并按锚点重算差异。失败时不增加锚点。
func (a *workspaceState) commitAnchor() {
	if a.ignoreBlankLines {
		a.err = i18n.T("err.anchorBlank")
		return
	}
	if a.panes[0] == nil || !a.anchorLeftSet || !a.anchorRightSet {
		a.err = i18n.T("err.markBoth")
		return
	}
	next := append(append([]textcompare.Anchor(nil), a.anchors...), textcompare.Anchor{Left: a.anchorLeft, Right: a.anchorRight})
	leftMax, rightMax := a.panes[0].buffer.LineCount(), a.panes[1].buffer.LineCount()
	if a.panes[0].buffer.Snapshot().Len() == 0 {
		leftMax = 0
	}
	if a.panes[1].buffer.Snapshot().Len() == 0 {
		rightMax = 0
	}
	prevLeft, prevRight := 0, 0
	for _, p := range next {
		if p.Left < prevLeft || p.Right < prevRight || p.Left > leftMax || p.Right > rightMax {
			a.err = i18n.T("err.anchorOrder")
			return
		}
		prevLeft, prevRight = p.Left, p.Right
	}

	a.anchors = next
	a.err = ""
	a.recompute()
}

// releaseAnchors 去掉全部锚点，恢复普通行对齐。
func (a *workspaceState) releaseAnchors() {
	if len(a.anchors) == 0 {
		return
	}
	a.anchors = nil
	a.anchorLeftSet, a.anchorRightSet = false, false
	a.err = ""
	a.recompute()
}
