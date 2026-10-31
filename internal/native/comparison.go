package native

import (
	"context"
	"fmt"

	"github.com/egoist/mygo/ui/editor"
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
	if a.panes[0] == nil || !a.diff.ExternalComparison {
		return
	}
	buffers := [2]*editor.Buffer{a.panes[0].buffer, a.panes[1].buffer}
	versions := [2]uint64{buffers[0].Version(), buffers[1].Version()}
	anchors := append([]textcompare.Anchor(nil), a.anchors...)
	if a.compareRequested && a.compareBuffers == buffers && a.compareVersions == versions && a.compareInline == a.diff.Inline && sameAnchors(a.compareAnchors, anchors) {
		return
	}
	a.stopComparison()
	generation := a.compareGeneration
	a.compareRequested = true
	a.compareBuffers = buffers
	a.compareVersions = versions
	a.compareInline = a.diff.Inline
	inline := a.diff.Inline
	a.compareRunning = true
	a.compareErr = nil
	leftText, rightText := buffers[0].String(), buffers[1].String()
	request := editor.NewComparisonRequest(buffers[0], buffers[1])
	ctx, cancel := context.WithCancel(context.Background())
	a.compareCancel = cancel
	run := func() {
		defer cancel()
		var aligned []editor.DiffHunk
		var alignedErr error
		if len(anchors) > 0 {
			result, err := textcompare.Compare(leftText, rightText, textcompare.Rules{}, textcompare.Myers, anchors)
			alignedErr = err
			if err == nil {
				aligned = make([]editor.DiffHunk, len(result.Hunks))
				for i, h := range result.Hunks {
					aligned[i] = editor.DiffHunk{OldStart: h.LeftStart, OldEnd: h.LeftEnd, NewStart: h.RightStart, NewEnd: h.RightEnd}
				}
			}
		}
		result, err := request.Compute(ctx, inline)
		a.publish(func() {
			if generation != a.compareGeneration {
				return
			}
			a.compareRunning = false
			a.compareCancel = nil
			if len(anchors) > 0 {
				if alignedErr != nil {
					a.compareErr = alignedErr
					a.err = alignedErr.Error()
					return
				}
				if !a.diff.ApplyHunks(aligned) {
					return
				}
				a.compareAnchors = anchors
				a.err = ""
				a.message = fmt.Sprintf("已对齐 %d 处 · %d 个差异块", len(anchors), len(aligned))
				return
			}
			if err != nil {
				a.compareErr = err
				return
			}
			if !a.diff.ApplyComparison(result) {
				return
			}
			a.compareAnchors = nil
			a.message = fmt.Sprintf("比较完成 · %d 个差异块", len(result.Hunks()))
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
	if a.panes[0] == nil || !a.diff.ComparisonReady() {
		a.err = "先完成一次比较"
		return
	}
	if a.diff.ActiveSide() != side {
		a.err = "先在这一侧放置光标"
		return
	}
	line := a.diff.CursorPos().Line
	if side == editor.DiffOld {
		a.anchorLeft, a.anchorLeftSet = line, true
	} else {
		a.anchorRight, a.anchorRightSet = line, true
	}
	a.err = ""
	left, right := "未标记", "未标记"
	if a.anchorLeftSet {
		left = fmt.Sprintf("第 %d 行", a.anchorLeft+1)
	}
	if a.anchorRightSet {
		right = fmt.Sprintf("第 %d 行", a.anchorRight+1)
	}
	a.message = "已标记左侧" + left + "、右侧" + right
}

// commitAnchor 把已标记的左右行追加为锚点，并按锚点重算差异。失败时不增加锚点。
func (a *workspaceState) commitAnchor() {
	if a.panes[0] == nil || !a.anchorLeftSet || !a.anchorRightSet {
		a.err = "先标记左右两行"
		return
	}
	next := append(append([]textcompare.Anchor(nil), a.anchors...), textcompare.Anchor{Left: a.anchorLeft, Right: a.anchorRight})
	if _, err := textcompare.Compare(a.panes[0].buffer.String(), a.panes[1].buffer.String(), textcompare.Rules{}, textcompare.Myers, next); err != nil {
		a.err = err.Error()
		return
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
