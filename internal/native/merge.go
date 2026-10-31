package native

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"gocompare/internal/document"
	"gocompare/internal/merge"
)

// openMergeWindow 另开一个只含文本合并的窗口，不切换当前文件比较或目录比较。
// 应用尚未就绪时仍创建同一份窗口内容，测试用 ui.NewTester 驱动它。
func (a *workspaceState) openMergeWindow() {
	if a.busy {
		return
	}
	if strings.TrimSpace(a.paths[0]) == "" || strings.TrimSpace(a.paths[1]) == "" {
		a.err = "请先选择左右两个文件"
		return
	}
	child := &workspaceState{
		kind:        2,
		detached:    true,
		paths:       a.paths,
		mergeBase:   a.mergeBase,
		mergeOutput: a.mergeOutput,
		backupDir:   a.backupDir,
	}
	if a.win != nil && mygo.App.IsReady() {
		child.attachWindow()
	}
	child.load()
	if child.win == nil && child.err != "" {
		a.err = child.err
		return
	}
	a.mergeWindows = append(a.mergeWindows, child)
	a.err = ""
	a.message = "合并已在新窗口打开"
}

func (a *workspaceState) attachWindow() {
	a.win = mygo.NewWindow(mygo.WindowOptions{
		Title:     "GoCompare 合并",
		Width:     1200,
		Height:    800,
		MinWidth:  800,
		MinHeight: 520,
		Content:   ui.View(a.mergeWindowView),
	})
	a.win.OnClosed(func() {
		if a.cancel != nil {
			a.cancel()
		}
	})
}

// mergeWindowView 是独立合并窗口的全部内容，不含文件比较工具栏。
func (a *workspaceState) mergeWindowView(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Row(c).Padding(12, 20).Gap(12).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, "GoCompare 合并").FontSize(20).Bold().Label("独立合并窗口")
			ui.Text(c, "输出与文件比较分开保存").FontSize(12).TextColor(t.TextMuted)
		})
		if a.err != "" {
			ui.Text(c, a.err).Padding(10, 20).TextColor(ui.Hex("#b5472e")).Label("错误")
		}
		a.mergeView(c)
		message := a.message
		if a.busy {
			message = "正在处理文件…"
		}
		ui.Text(c, message).Padding(6, 20).FontSize(12).TextColor(t.TextMuted)
	})
}

func (a *workspaceState) loadMerge() {
	if a.busy {
		return
	}
	left, right, base := a.paths[0], a.paths[1], strings.TrimSpace(a.mergeBase)
	if strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
		a.err = "请先选择左右两个文件"
		return
	}
	a.job(func(ctx context.Context) {
		lf, err := document.Read(ctx, left)
		var rf, bf *document.File
		var result merge.Result
		if err == nil {
			rf, err = document.Read(ctx, right)
		}
		if err == nil && base != "" {
			bf, err = document.Read(ctx, base)
			if err == nil {
				result, err = merge.MergeThree(bf.Text, lf.Text, rf.Text)
			}
		} else if err == nil {
			result, err = merge.MergeTwo(lf.Text, rf.Text)
		}
		a.publish(func() {
			a.busy = false
			if err != nil {
				a.err = err.Error()
				return
			}
			a.installMerge(lf, result)
		})
	})
}

func (a *workspaceState) installMerge(left *document.File, result merge.Result) {
	a.stopComparison()
	a.panes = [2]*pane{}
	a.directoryRows = nil
	a.syncPlan = nil
	a.kind = 2
	a.mergeLeft = left
	a.mergeResult = &result
	a.mergeRows = result.Alignments()
	a.mergeAlign = ui.ListState{}
	a.mergeConflict = 0
	a.mergeBuffer = editor.NewBuffer(result.Text())
	a.mergeEdit = editor.State{Buffer: a.mergeBuffer, Label: "合并输出"}
	if result.ConflictCount() == 0 {
		a.message = "合并完成，没有冲突"
		return
	}
	a.message = fmt.Sprintf("合并完成 · %d 个冲突", result.ConflictCount())
}

func (a *workspaceState) takeConflict(choice merge.Choice) {
	if a.busy || a.mergeResult == nil || a.mergeBuffer == nil {
		return
	}
	if a.mergeBuffer.String() != a.mergeResult.Text() {
		a.err = "输出已手工修改，请直接编辑冲突标记后保存，或重新合并"
		return
	}
	next, err := a.mergeResult.Take(a.mergeConflict, choice)
	if err != nil {
		a.err = err.Error()
		return
	}
	a.installMerge(a.mergeLeft, next)
}

func (a *workspaceState) saveMerge() {
	if a.busy || a.mergeBuffer == nil || a.mergeLeft == nil {
		return
	}
	path := strings.TrimSpace(a.mergeOutput)
	if path == "" {
		a.err = "请先填写输出路径；已存在的文件不会被覆盖"
		return
	}
	file, text, version := a.mergeLeft, a.mergeBuffer.String(), a.mergeBuffer.Version()
	a.job(func(ctx context.Context) {
		saved, err := file.SaveAs(ctx, path, text)
		a.publish(func() {
			a.busy = false
			if err != nil {
				a.err = err.Error()
				return
			}
			a.mergeOutput = saved.Path
			if a.mergeBuffer.Version() == version {
				a.mergeBuffer.MarkSaved()
			}
			a.message = "合并输出已保存 " + saved.Path
		})
	})
}

func (a *workspaceState) mergeView(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Padding(8, 20).Gap(8).Children(func() {
		ui.TextInput(c.Key("merge-base"), &a.mergeBase).Grow(1).Disabled(a.busy).Label("基准文件，可留空")
		ui.TextInput(c.Key("merge-output"), &a.mergeOutput).Grow(1).Disabled(a.busy).Label("输出路径")
		ui.Button(c, "保存输出").Disabled(a.busy || a.mergeBuffer == nil).OnClick(a.saveMerge)
	})
	if a.mergeResult == nil || a.mergeBuffer == nil {
		ui.Column(c).Grow(1).FillWidth().Center().Gap(8).Children(func() {
			ui.Text(c, "文本合并").FontSize(28).Bold()
			ui.Text(c, "填写左右文件。有共同祖先时再填写基准文件，然后开始比较。").TextColor(t.TextMuted)
		})
		return
	}
	count := a.mergeResult.ConflictCount()
	ui.Row(c).Padding(8, 20).Gap(8).Children(func() {
		ui.Button(c, "采用左侧").Disabled(a.busy || count == 0).OnClick(func() { a.takeConflict(merge.ChoiceLeft) })
		ui.Button(c, "采用右侧").Disabled(a.busy || count == 0).OnClick(func() { a.takeConflict(merge.ChoiceRight) })
		ui.Button(c, "采用双方").Disabled(a.busy || count == 0).OnClick(func() { a.takeConflict(merge.ChoiceBoth) })
		ui.Button(c, "采用基准").Disabled(a.busy || count == 0 || a.mergeBase == "").OnClick(func() { a.takeConflict(merge.ChoiceBase) })
		ui.Button(c, "上一冲突").Disabled(a.busy || count == 0).OnClick(func() {
			if a.mergeConflict > 0 {
				a.mergeConflict--
			}
		})
		ui.Button(c, "下一冲突").Disabled(a.busy || count == 0).OnClick(func() {
			if a.mergeConflict+1 < count {
				a.mergeConflict++
			}
		})
		label := "没有冲突"
		if count > 0 {
			label = fmt.Sprintf("冲突 %d / %d", a.mergeConflict+1, count)
		}
		ui.Text(c, label).TextColor(t.TextMuted)
	})
	a.alignmentView(c)
	a.mergeEdit.ReadOnly = a.busy
	editor.View(c.Key("merge-output-view"), &a.mergeEdit).Grow(1).FillWidth().MinHeight(120)
}

// alignmentView 在同一屏显示基准、左侧和右侧的对齐行。输出编辑器仍在下方。
// 两方合并没有基准，不画空的基准列。列表只构建可见行，三列在同一行里一起滚动。
func (a *workspaceState) alignmentView(c *ui.Context) {
	t := c.Theme()
	rows := a.mergeRows
	showBase := a.mergeResult != nil && a.mergeResult.HasBase()
	ui.Row(c).Padding(4, 20).Gap(8).Children(func() {
		if showBase {
			ui.Text(c, "基准").Grow(1).Bold().Label("对齐列 基准")
		}
		ui.Text(c, "左侧").Grow(1).Bold().Label("对齐列 左侧")
		ui.Text(c, "右侧").Grow(1).Bold().Label("对齐列 右侧")
	})
	if len(rows) == 0 {
		ui.Text(c, "没有可对齐的行").Padding(8, 20).TextColor(t.TextMuted)
		return
	}
	ui.List(c, &a.mergeAlign, len(rows)).Grow(2).FillWidth().MinHeight(160).Rows(func(row ui.ListRow) {
		item := rows[row.Index]
		line := ui.Row(row.Context).Padding(2, 20).Gap(8)
		if item.Conflict {
			line.Background(ui.Hex("#f8e6e1"))
			if item.ConflictIndex == a.mergeConflict {
				line.Background(ui.Hex("#f0c2b8"))
			}
		}
		line.Children(func() {
			if showBase {
				alignCell(row.Context, "基准", row.Index, item.Base, item.HasBase)
			}
			alignCell(row.Context, "左侧", row.Index, item.Left, item.HasLeft)
			alignCell(row.Context, "右侧", row.Index, item.Right, item.HasRight)
		})
	})
}

func alignCell(c *ui.Context, side string, index int, text string, present bool) {
	shown := "·"
	if present {
		shown = text
		if text == "" {
			shown = "空行"
		}
	}
	ui.Text(c, shown).Grow(1).Shrink(1).MinWidth(0).SingleLine().FontSize(13).Label(alignCellLabel(side, index, text, present))
}

func alignCellLabel(side string, index int, text string, present bool) string {
	if !present {
		return fmt.Sprintf("%s第%d行空", side, index+1)
	}
	if text == "" {
		return fmt.Sprintf("%s第%d行空白", side, index+1)
	}
	return fmt.Sprintf("%s第%d行 %s", side, index+1, text)
}
