package native

import (
	"context"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"gocompare/internal/document"
	"gocompare/internal/i18n"
	"gocompare/internal/merge"
)

// openMergeWindow 另开一个只含文本合并的窗口，不切换当前文件比较或目录比较。
func (a *workspaceState) openMergeWindow() {
	if a.busy {
		return
	}
	if strings.TrimSpace(a.paths[0]) == "" || strings.TrimSpace(a.paths[1]) == "" {
		a.err = i18n.T("err.selectBothFiles")
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
	a.message = i18n.T("merge.opened")
}

func (a *workspaceState) attachWindow(parent *workspaceState) {
	a.win = mygo.NewWindow(mygo.WindowOptions{
		Title:     i18n.T("merge.windowTitle"),
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
			ui.Text(c, i18n.T("merge.windowTitle")).FontSize(15).Bold().Label(i18n.T("merge.windowLabel"))
			statusPill(c, i18n.T("merge.ownBuffer"), pal.baseAccent, pal.baseBg)
		})
		if a.err != "" {
			ui.Row(c).Padding(6, 16).Gap(8).Background(pal.dangerBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.dangerFg.Alpha(0.2)).Children(func() {
				ui.Icon(c, iconAlert).Size(13, 13).TextColor(pal.dangerFg)
				ui.Text(c, a.err).Grow(1).FontSize(11).TextColor(pal.dangerFg).Label(i18n.T("a11y.error"))
			})
		}
		a.mergeView(c)
		message := a.message
		if a.busy {
			message = i18n.T("status.processingFiles")
		}
		ui.Row(c).Height(26).Padding(0, 14).Gap(6).Background(pal.subtleSurface).BorderWidth(1, 0, 0, 0).BorderColor(pal.cardBorder).Children(func() {
			ui.Box(c).Size(5, 5).Radius(999).Background(pal.successFg)
			ui.Text(c, message).Grow(1).FontSize(11).TextColor(t.TextMuted).SingleLine()
		})
	})
	if a.confirm {
		choice := ui.AlertDialog(c, &a.confirm, i18n.T("dialog.discardTitle"), i18n.T("merge.discardDesc"), i18n.T("dialog.cancel"), i18n.T("dialog.discard"))
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
		a.err = i18n.T("err.selectBothFiles")
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
	a.kind = kindMerge
	a.mergeLeft = left
	a.mergeResult = &result
	a.mergeRows = result.Alignments()
	a.mergeAlign = ui.ListState{}
	a.mergeConflict = 0
	a.mergeBuffer = editor.NewBuffer(result.Text())
	a.mergeEdit = editor.State{Buffer: a.mergeBuffer, Label: i18n.T("merge.output")}
	if result.ConflictCount() == 0 {
		a.message = i18n.T("merge.doneClean")
		return
	}
	a.message = i18n.TArgs("merge.doneConflicts", result.ConflictCount())
}

func (a *workspaceState) takeConflict(choice merge.Choice) {
	if a.busy || a.mergeResult == nil || a.mergeBuffer == nil {
		return
	}
	if a.mergeBuffer.String() != a.mergeResult.Text() {
		a.err = i18n.T("merge.editedManually")
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
		a.err = i18n.T("merge.needOutput")
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
			a.message = i18n.TArgs("merge.saved", saved.Path)
		})
	})
}

func (a *workspaceState) mergeView(c *ui.Context) {
	t := c.Theme()
	pal := paletteFor(t)
	ui.Row(c).Padding(6, 14).Gap(8).Background(pal.cardBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		sideBadge(c, 2, pal)
		ui.TextInput(c.Key("merge-base"), &a.mergeBase).Grow(1).Placeholder(i18n.T("merge.basePlaceholder")).Disabled(a.busy).Label(i18n.T("merge.baseLabel"))
		ui.TextInput(c.Key("merge-output"), &a.mergeOutput).Grow(1).Placeholder(i18n.T("merge.outputPlaceholder")).Disabled(a.busy).Label(i18n.T("merge.outputLabel"))
		saveBtn := ui.PrimaryButton(c, "").Label(i18n.T("merge.save")).Padding(4, 10).Gap(4).Radius(5).Disabled(a.busy || a.mergeBuffer == nil)
		saveBtn.OnClick(a.saveMerge)
		saveBtn.Children(func() {
			ui.Icon(c, iconSave).Size(11, 11)
			ui.Text(c, i18n.T("merge.save")).FontSize(11).Bold().SingleLine()
		})
	})
	if a.mergeResult == nil || a.mergeBuffer == nil {
		if a.err == "" {
			a.modeEmptyView(c, pal, 2)
		}
		return
	}
	count := a.mergeResult.ConflictCount()
	ui.Row(c).Height(34).Padding(0, 14).Gap(5).Background(pal.subtleSurface).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		ui.Button(c, i18n.T("merge.takeLeft")).Padding(3, 8).Radius(5).FontSize(11).Disabled(a.busy || count == 0).OnClick(func() { a.takeConflict(merge.ChoiceLeft) })
		ui.Button(c, i18n.T("merge.takeRight")).Padding(3, 8).Radius(5).FontSize(11).Disabled(a.busy || count == 0).OnClick(func() { a.takeConflict(merge.ChoiceRight) })
		ui.Button(c, i18n.T("merge.takeBoth")).Padding(3, 8).Radius(5).FontSize(11).Disabled(a.busy || count == 0).OnClick(func() { a.takeConflict(merge.ChoiceBoth) })
		ui.Button(c, i18n.T("merge.takeBase")).Padding(3, 8).Radius(5).FontSize(11).Disabled(a.busy || count == 0 || a.mergeBase == "").OnClick(func() { a.takeConflict(merge.ChoiceBase) })
		ui.Divider(c).Height(14)
		ui.Button(c, i18n.T("merge.prevConflict")).Padding(3, 7).Radius(5).FontSize(11).Disabled(a.busy || count == 0).OnClick(func() {
			if a.mergeConflict > 0 {
				a.mergeConflict--
			}
		})
		ui.Button(c, i18n.T("merge.nextConflict")).Padding(3, 7).Radius(5).FontSize(11).Disabled(a.busy || count == 0).OnClick(func() {
			if a.mergeConflict+1 < count {
				a.mergeConflict++
			}
		})
		ui.Box(c).Grow(1)
		label := i18n.T("merge.noConflict")
		fg, bg := pal.successFg, pal.successBg
		if count > 0 {
			label = i18n.TArgs("merge.conflictPos", a.mergeConflict+1, count)
			fg, bg = pal.dangerFg, pal.dangerBg
		}
		statusPill(c, label, fg, bg)
	})
	a.alignmentView(c)
	ui.Row(c).Height(24).Padding(0, 14).Gap(6).Background(pal.subtleSurface).BorderWidth(1, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		ui.Text(c, i18n.T("merge.outputHeader")).FontSize(10).Bold().TextColor(t.TextMuted)
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
				ui.Text(c, i18n.T("merge.colBase")).Bold().FontSize(11).Label(i18n.TArgs("merge.colLabel", i18n.T("merge.colBase")))
			})
		}
		ui.Row(c).Grow(1).Basis(0).Gap(5).Children(func() {
			sideBadge(c, 0, pal)
			ui.Text(c, i18n.T("merge.colLeft")).Bold().FontSize(11).Label(i18n.TArgs("merge.colLabel", i18n.T("merge.colLeft")))
		})
		ui.Row(c).Grow(1).Basis(0).Gap(5).Children(func() {
			sideBadge(c, 1, pal)
			ui.Text(c, i18n.T("merge.colRight")).Bold().FontSize(11).Label(i18n.TArgs("merge.colLabel", i18n.T("merge.colRight")))
		})
	})
	if len(rows) == 0 {
		ui.Text(c, i18n.T("merge.noRows")).Padding(6, 14).TextColor(t.TextMuted)
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
				alignCell(row.Context, i18n.T("merge.colBase"), row.Index, item.Base, item.HasBase)
			}
			alignCell(row.Context, i18n.T("merge.colLeft"), row.Index, item.Left, item.HasLeft)
			alignCell(row.Context, i18n.T("merge.colRight"), row.Index, item.Right, item.HasRight)
		})
	})
}

func alignCell(c *ui.Context, side string, index int, text string, present bool) {
	shown := "·"
	if present {
		shown = text
		if text == "" {
			shown = i18n.T("merge.emptyLine")
		}
	}
	ui.Text(c, shown).Grow(1).Basis(0).Shrink(1).MinWidth(0).SingleLine().FontSize(12).Label(alignCellLabel(side, index, text, present))
}

func alignCellLabel(side string, index int, text string, present bool) string {
	if !present {
		return i18n.TArgs("merge.cellMissing", side, index+1)
	}
	if text == "" {
		return i18n.TArgs("merge.cellBlank", side, index+1)
	}
	return i18n.TArgs("merge.cellText", side, index+1, text)
}
