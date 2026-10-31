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
		child.attachWindow(a)
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

func (a *workspaceState) attachWindow(parent *workspaceState) {
	a.win = mygo.NewWindow(mygo.WindowOptions{
		Title:     "GoCompare 合并",
		Width:     1200,
		Height:    800,
		MinWidth:  800,
		MinHeight: 520,
		Content:   ui.View(a.mergeWindowView),
	})
	a.win.OnClose(func(e *mygo.CloseEvent) {
		if a.busy {
			e.PreventDefault()
			return
		}
		if a.dirty() {
			e.PreventDefault()
			a.guard(func() { a.win.Destroy() })
		}
	})
	a.win.OnClosed(func() {
		if a.cancel != nil {
			a.cancel()
		}
		if parent != nil {
			for i, w := range parent.mergeWindows {
				if w == a {
					parent.mergeWindows = append(parent.mergeWindows[:i], parent.mergeWindows[i+1:]...)
					break
				}
			}
		}
	})
}

// mergeWindowView 是独立合并窗口的全部内容，不含文件比较工具栏。
func (a *workspaceState) mergeWindowView(c *ui.Context) {
	t := c.Theme()
	pal := paletteFor(t)
	ui.Column(c).Fill().Background(pal.appBg).Children(func() {
		ui.Row(c).Height(44).Padding(0, 16).Gap(8).Background(pal.headerBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
			ui.Row(c).Size(24, 24).Center().Radius(6).Background(pal.baseBg).Children(func() {
				ui.Icon(c, iconGitMerge).Size(13, 13).TextColor(pal.baseAccent)
			})
			ui.Text(c, "GoCompare 合并").FontSize(15).Bold().Label("独立合并窗口")
			statusPill(c, "独立输出缓冲区", pal.baseAccent, pal.baseBg)
		})
		if a.err != "" {
			ui.Row(c).Padding(6, 16).Gap(8).Background(pal.dangerBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.dangerFg.Alpha(0.2)).Children(func() {
				ui.Icon(c, iconAlert).Size(13, 13).TextColor(pal.dangerFg)
				ui.Text(c, a.err).Grow(1).FontSize(11).TextColor(pal.dangerFg).Label("错误")
			})
		}
		a.mergeView(c)
		message := a.message
		if a.busy {
			message = "正在处理文件…"
		}
		ui.Row(c).Height(26).Padding(0, 14).Gap(6).Background(pal.subtleSurface).BorderWidth(1, 0, 0, 0).BorderColor(pal.cardBorder).Children(func() {
			ui.Box(c).Size(5, 5).Radius(999).Background(pal.successFg)
			ui.Text(c, message).Grow(1).FontSize(11).TextColor(t.TextMuted).SingleLine()
		})
	})
	if a.confirm {
		choice := ui.AlertDialog(c, &a.confirm, "放弃未保存的修改？", "可取消后先保存合并输出。", "取消", "放弃修改")
		if choice == 1 {
			fn := a.pending
			a.pending = nil
			if fn != nil {
				fn()
			}
		} else if !a.confirm {
			a.pending = nil
		}
	}
	c.OnShortcut(ui.Cmd, ui.KeyS, func() {
		if a.mergeBuffer != nil && a.mergeBuffer.Modified() {
			a.saveMerge()
		}
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
				a.clearMerge()
				return
			}
			a.installMerge(lf, result)
		})
	})
}

func (a *workspaceState) clearMerge() {
	a.mergeResult = nil
	a.mergeRows = nil
	a.mergeAlign = ui.ListState{}
	a.mergeBuffer = nil
	a.mergeEdit = editor.State{}
	a.mergeLeft = nil
	a.mergeConflict = 0
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
	pal := paletteFor(t)
	ui.Row(c).Padding(6, 14).Gap(8).Background(pal.cardBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		sideBadge(c, 2, pal)
		ui.TextInput(c.Key("merge-base"), &a.mergeBase).Grow(1).Placeholder("基准共同祖先文件（可留空）").Disabled(a.busy).Label("基准文件，可留空")
		ui.TextInput(c.Key("merge-output"), &a.mergeOutput).Grow(1).Placeholder("合并结果输出路径…").Disabled(a.busy).Label("输出路径")
		saveBtn := ui.PrimaryButton(c, "").Label("保存输出").Padding(4, 10).Gap(4).Radius(5).Disabled(a.busy || a.mergeBuffer == nil)
		saveBtn.OnClick(a.saveMerge)
		saveBtn.Children(func() {
			ui.Icon(c, iconSave).Size(11, 11)
			ui.Text(c, "保存输出").FontSize(11).Bold().SingleLine()
		})
	})
	if a.mergeResult == nil || a.mergeBuffer == nil {
		ui.Column(c).Grow(1).FillWidth().Center().Padding(24).Children(func() {
			ui.Column(c).MaxWidth(480).Padding(24, 28).Gap(10).Radius(12).Background(pal.cardBg).Border(1, pal.cardBorder).Center().Children(func() {
				ui.Row(c).Size(32, 32).Center().Radius(8).Background(pal.baseBg).Children(func() {
					ui.Icon(c, iconGitMerge).Size(16, 16).TextColor(pal.baseAccent)
				})
				ui.Text(c, "文本合并").FontSize(20).Bold()
				ui.Text(c, "填写左右文件。有共同祖先时再填写基准文件，然后开始比较。").FontSize(12).TextColor(t.TextMuted)
			})
		})
		return
	}
	count := a.mergeResult.ConflictCount()
	ui.Row(c).Height(34).Padding(0, 14).Gap(5).Background(pal.subtleSurface).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		ui.Button(c, "采用左侧").Padding(3, 8).Radius(5).FontSize(11).Disabled(a.busy || count == 0).OnClick(func() { a.takeConflict(merge.ChoiceLeft) })
		ui.Button(c, "采用右侧").Padding(3, 8).Radius(5).FontSize(11).Disabled(a.busy || count == 0).OnClick(func() { a.takeConflict(merge.ChoiceRight) })
		ui.Button(c, "采用双方").Padding(3, 8).Radius(5).FontSize(11).Disabled(a.busy || count == 0).OnClick(func() { a.takeConflict(merge.ChoiceBoth) })
		ui.Button(c, "采用基准").Padding(3, 8).Radius(5).FontSize(11).Disabled(a.busy || count == 0 || a.mergeBase == "").OnClick(func() { a.takeConflict(merge.ChoiceBase) })
		ui.Divider(c).Height(14)
		ui.Button(c, "上一冲突").Padding(3, 7).Radius(5).FontSize(11).Disabled(a.busy || count == 0).OnClick(func() {
			if a.mergeConflict > 0 {
				a.mergeConflict--
			}
		})
		ui.Button(c, "下一冲突").Padding(3, 7).Radius(5).FontSize(11).Disabled(a.busy || count == 0).OnClick(func() {
			if a.mergeConflict+1 < count {
				a.mergeConflict++
			}
		})
		ui.Box(c).Grow(1)
		label := "没有冲突"
		fg, bg := pal.successFg, pal.successBg
		if count > 0 {
			label = fmt.Sprintf("冲突 %d / %d", a.mergeConflict+1, count)
			fg, bg = pal.dangerFg, pal.dangerBg
		}
		statusPill(c, label, fg, bg)
	})
	a.alignmentView(c)
	ui.Row(c).Height(24).Padding(0, 14).Gap(6).Background(pal.subtleSurface).BorderWidth(1, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		ui.Text(c, "合并结果输出").FontSize(10).Bold().TextColor(t.TextMuted)
	})
	a.mergeEdit.ReadOnly = a.busy
	editor.View(c.Key("merge-output-view"), &a.mergeEdit).Grow(1).FillWidth().MinHeight(80)
}

// alignmentView 在同一屏显示基准、左侧和右侧的对齐行。输出编辑器仍在下方。
func (a *workspaceState) alignmentView(c *ui.Context) {
	t := c.Theme()
	pal := paletteFor(t)
	rows := a.mergeRows
	showBase := a.mergeResult != nil && a.mergeResult.HasBase()
	ui.Row(c).Height(26).Padding(0, 14).Gap(8).Background(pal.cardBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		if showBase {
			ui.Row(c).Grow(1).Basis(0).Gap(5).Children(func() {
				sideBadge(c, 2, pal)
				ui.Text(c, "基准").Bold().FontSize(11).Label("对齐列 基准")
			})
		}
		ui.Row(c).Grow(1).Basis(0).Gap(5).Children(func() {
			sideBadge(c, 0, pal)
			ui.Text(c, "左侧").Bold().FontSize(11).Label("对齐列 左侧")
		})
		ui.Row(c).Grow(1).Basis(0).Gap(5).Children(func() {
			sideBadge(c, 1, pal)
			ui.Text(c, "右侧").Bold().FontSize(11).Label("对齐列 右侧")
		})
	})
	if len(rows) == 0 {
		ui.Text(c, "没有可对齐的行").Padding(6, 14).TextColor(t.TextMuted)
		return
	}
	ui.List(c, &a.mergeAlign, len(rows)).Grow(2).FillWidth().MinHeight(110).Rows(func(row ui.ListRow) {
		item := rows[row.Index]
		line := ui.Row(row.Context).Padding(2, 14).Gap(8).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder.Alpha(0.3))
		if item.Conflict {
			bg, activeBg := ui.Hex("#fef2f2"), ui.Hex("#fee2e2")
			if t.Dark {
				bg, activeBg = ui.Hex("#3d221e"), ui.Hex("#592b25")
			}
			line.Background(bg)
			if item.ConflictIndex == a.mergeConflict {
				line.Background(activeBg)
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
	ui.Text(c, shown).Grow(1).Basis(0).Shrink(1).MinWidth(0).SingleLine().FontSize(12).Label(alignCellLabel(side, index, text, present))
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
