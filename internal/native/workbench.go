package native

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
	"github.com/oneo-me/onevra/source/shells/sidebar"
	"gocompare/internal/brand"
	"gocompare/internal/document"
	"gocompare/internal/i18n"
	"gocompare/internal/remote"
)

func readInput(ctx context.Context, path string, missing bool) (*document.File, error) {
	f, err := document.ReadDetected(ctx, path)
	if missing && os.IsNotExist(err) {
		return document.EmptyAt(path)
	}
	return f, err
}

func (a *workspaceState) selectKind(kind int) {
	if a.busy || kind == a.kind {
		return
	}
	a.guard(func() {
		a.ensureModeSessions()
		a.stopComparison()
		a.hunkFocus = 0
		a.pendingFocusHunk = 0
		a.compareRequested = false
		currentPaths := a.paths
		hasScratch := a.kind == kindScratch && a.scratchPanes[0] != nil && (a.scratchPanes[0].buffer.Len() > 0 || a.scratchPanes[1].buffer.Len() > 0)
		hadResult := a.panes[0] != nil || a.directoryRows != nil || a.mergeResult != nil || a.pictureResult != nil || a.mediaReady || a.infoReady || a.tableReady || hasScratch

		if hadResult || currentPaths[0] != "" || currentPaths[1] != "" {
			a.saveModeSession(a.kind)
		}

		a.kind = kind
		a.err = ""
		if !a.restoreModeSession(kind) {
			a.panes = [2]*pane{}
			a.directoryRows = nil
			a.syncPlan = nil
			a.dirMergeDecisions = nil
			a.clearMerge()
			a.clearPictures()
			a.clearMedia()
			a.clearVersion()
			a.clearTable()
			a.allowMissing = false
			a.directoryRoots = [2]string{}
			if kind == kindScratch {
				a.paths = [2]string{}
			} else if !hadResult {
				a.paths = currentPaths
			} else {
				a.paths = [2]string{}
			}
			if kind == kindScratch {
				a.ensureScratch()
				a.refreshComparison()
				a.message = i18n.T("status.scratchReady")
			} else {
				a.message = i18n.T("status.selectSource")
			}
		} else {
			if kind == kindScratch {
				a.paths = [2]string{}
				a.ensureScratch()
				a.refreshComparison()
			} else if kind == kindFile && a.panes[0] != nil {
				a.refreshComparison()
			}
		}
	})
}

func (a *workspaceState) navigateDiff(step int) {
	panes := a.activePanes()
	diff := a.activeDiff()
	if panes[0] == nil || a.compareRunning || a.compareErr != nil || !diff.ComparisonReady() {
		return
	}
	hunks := diff.Hunks()
	n := len(hunks)
	if n == 0 {
		return
	}
	i := -1
	if a.hunkFocus > 0 && a.hunkFocus <= n {
		i = (a.hunkFocus - 1 + step + n) % n
	} else {
		cur, _ := diff.CurrentHunk()
		if cur < 0 {
			if step > 0 {
				i = 0
			} else {
				i = n - 1
			}
		} else {
			i = (cur + step + n) % n
		}
	}
	a.selectHunk(i)
}

func (a *workspaceState) selectHunk(i int) {
	panes := a.activePanes()
	diff := a.activeDiff()
	if panes[0] == nil || a.compareRunning || a.compareErr != nil || !diff.ComparisonReady() {
		return
	}
	hunks := diff.Hunks()
	n := len(hunks)
	if i < 0 || i >= n {
		return
	}
	diff.GoToHunk(i)
	focusHunk(diff, hunks[i])
	a.hunkFocus = i + 1
	a.message = i18n.TArgs("status.hunkPosition", i+1, n)
}

// focusHunk 高亮差异块的全部行，让跳转位置一眼可见；光标只是一条细线，编辑器失焦时还不显示。
// 高亮只能落在一侧：优先新文本，纯删除的块在新文本里没有行，改为高亮旧文本。
func focusHunk(diff *editor.DiffState, h editor.DiffHunk) {
	r := editor.DiffLineRange{Side: editor.DiffNew, Start: h.NewStart, End: h.NewEnd - 1}
	if h.NewEnd <= h.NewStart {
		r = editor.DiffLineRange{Side: editor.DiffOld, Start: h.OldStart, End: h.OldEnd - 1}
	}
	diff.SetSelectedLines(r, r.End >= r.Start)
}

// hunkCounter 显示当前位置：跳转后为"第几处 / 共几处"，未跳转时只显示总数，没有差异时不显示。
func (a *workspaceState) hunkCounter(c *ui.Context, pal uiPalette) {
	total := len(a.activeDiff().Hunks())
	if total == 0 {
		return
	}
	label := i18n.TArgs("diff.hunkTotal", total)
	if a.hunkFocus > 0 && a.hunkFocus <= total {
		label = fmt.Sprintf("%d / %d", a.hunkFocus, total)
	}
	statusPill(c, label, pal.diffFg, pal.diffBg)
}

func (a *workspaceState) diffPrev() { a.navigateDiff(-1) }
func (a *workspaceState) diffNext() { a.navigateDiff(1) }

type modeItem struct {
	kind   int
	msgKey string
	icon   *ui.SVG
}

var modes = []modeItem{
	{kind: kindScratch, msgKey: "mode.scratch", icon: iconTextDiff},
	{kind: kindFile, msgKey: "mode.file", icon: iconFileDiff},
	{kind: kindDirectory, msgKey: "mode.directory", icon: iconFolderDiff},
	{kind: kindMerge, msgKey: "mode.merge", icon: iconGitMerge},
	{kind: kindTable, msgKey: "mode.table", icon: iconTable},
	{kind: kindPicture, msgKey: "mode.picture", icon: iconImageDiff},
	{kind: kindMedia, msgKey: "mode.media", icon: iconMediaTag},
	{kind: kindVersion, msgKey: "mode.version", icon: iconVersionReg},
}

func modeTitle(kind int) string {
	for _, m := range modes {
		if m.kind == kind {
			return i18n.T(m.msgKey)
		}
	}
	return i18n.T("mode.file")
}

func modeIcon(kind int) *ui.SVG {
	for _, m := range modes {
		if m.kind == kind {
			return m.icon
		}
	}
	return iconFileDiff
}

func (a *workspaceState) swapInputs() {
	if a.busy {
		return
	}
	a.guard(func() { a.paths[0], a.paths[1] = a.paths[1], a.paths[0]; a.load() })
}

func (a *workspaceState) shellOptions() sidebar.ShellOptions {
	return sidebar.ShellOptions{
		Title:        "GoCompare",
		Width:        1380,
		Height:       880,
		MinWidth:     900,
		MinHeight:    600,
		LeftWidth:    168,
		LeftMinWidth: 144,
		LeftMaxWidth: 260,
		StateKey:     "gocompare.main",
		Theme:        activeTheme,
		Left: func(c *ui.Context) {
			pal := paletteFor(c.Theme())
			ui.Column(c).Fill().Background(pal.sidebarBg).Children(func() {
				a.renderSidebar(c, pal, sidebar.HeaderHeight(c))
			})
		},
		ContentTitle: func(c *ui.Context) {
			pal := paletteFor(c.Theme())
			ui.Row(c).Fill().AlignItems(ui.Center).Gap(8).Children(func() {
				a.renderHeaderContents(c, pal)
			})
		},
		Content: func(c *ui.Context) {
			a.refreshComparison()
			pal := paletteFor(c.Theme())
			ui.Column(c).Fill().Padding(sidebar.HeaderHeight(c), 0, 0, 0).Background(pal.appBg).Children(func() {
				ui.Box(c).FillWidth().Height(1).Background(pal.cardBorder)
				a.renderWorkspaceMain(c, pal)
			})
			a.renderOverlaysAndShortcuts(c, pal)
		},
	}
}

func (a *workspaceState) renderSidebar(c *ui.Context, pal uiPalette, topPad float32) {
	ui.Column(c).Fill().Padding(topPad, 6, 10, 6).Gap(4).Children(func() {
		ui.Row(c).Padding(4, 6).Gap(8).Margin(0, 0, 6).AlignItems(ui.Center).Children(func() {
			if icon := a.appIcon(); icon != nil {
				ui.Box(c).Size(26, 26).Center().Radius(6).Background(pal.cardBg).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.08)).Children(func() {
					ui.Image(c, icon).Size(20, 20).Label(i18n.T("a11y.appIcon"))
				})
			}
			ui.Column(c).Gap(1).Children(func() {
				components.Text(c, "GoCompare").FontSize(14).Bold().TextColor(pal.sidebarText).SingleLine()
				components.Text(c, "Diff Workbench").FontSize(9).TextColor(pal.sidebarMuted).SingleLine()
			})
		})

		components.Sidebar(c).Padding(0, 2, 0, 2).Gap(6).Grow(1).Children(func() {
			components.SidebarSection(c, i18n.T("nav.modes"), func() {
				for _, m := range modes {
					m := m
					label := i18n.T(m.msgKey)
					item := components.SidebarItem(c, label, m.icon, a.kind == m.kind)
					item.Padding(5, 8).Gap(7).FontSize(12)
					if item.Clicked() {
						a.selectKind(m.kind)
					}
				}
			})

			ui.Box(c).Grow(1)
			ui.Box(c).Height(1).FillWidth().Margin(4, 4).Background(pal.sidebarBorder)

			components.SidebarSection(c, i18n.T("nav.session"), func() {
				openItem := components.SidebarItem(c, i18n.T("nav.openSession"), iconFolderOpen, false).
					Padding(5, 8).Gap(7).FontSize(11).Disabled(a.busy)
				if openItem.Clicked() {
					a.sessionDialog(false)
				}

				saveItem := components.SidebarItem(c, i18n.T("nav.saveSession"), iconSave, false).
					Padding(5, 8).Gap(7).FontSize(11).Disabled(a.busy || !a.canSaveSession())
				if saveItem.Clicked() {
					a.sessionDialog(true)
				}

				prefShortcut := "⌘,"
				if runtime.GOOS != "darwin" {
					prefShortcut = "Ctrl+,"
				}
				settingsItem := components.SidebarItemWithSlots(c, i18n.T("nav.settings"), a.settingsOpen,
					func(c *ui.Context, size float32) {
						components.Icon(c, iconSliders).Size(size, size).TextColor(c.Theme().Accent)
					},
					func(c *ui.Context, _ float32) {
						components.Text(c, prefShortcut).FontSize(10).TextColor(pal.sidebarMuted).SingleLine()
					},
				).Padding(5, 8).Gap(7).FontSize(11).Tooltip(i18n.T("nav.settings") + " (" + prefShortcut + ")")
				if settingsItem.Clicked() {
					a.settingsOpen = true
				}

				aboutItem := components.SidebarItem(c, i18n.T("nav.about"), iconInfo, a.aboutOpen).
					Padding(5, 8).Gap(7).FontSize(11)
				if aboutItem.Clicked() {
					a.aboutOpen = true
				}
			})
		})
	})
}

func (a *workspaceState) renderHeaderContents(c *ui.Context, pal uiPalette) {
	title := modeTitle(a.kind)
	ui.Row(c).Size(24, 24).Center().Radius(6).Background(pal.baseBg).Children(func() {
		components.Icon(c, modeIcon(a.kind)).Size(13, 13).TextColor(pal.baseAccent)
	})
	components.Text(c, title).FontSize(15).Bold().SingleLine()
	a.renderHeaderBadge(c, pal)
	ui.Box(c).Grow(1)

	if a.kind == kindFile && a.panes[0] != nil && a.directoryRoots[0] != "" {
		backBtn := toolbarBtn(c.Key("btn-back-folder"), i18n.T("action.backToFolder"), iconFolderOpen, components.ButtonSecondary, a.busy)
		backBtn.OnClick(func() {
			a.guard(func() {
				if a.restoreModeSession(1) {
					a.kind = kindDirectory
				} else {
					a.paths = a.directoryRoots
					a.kind = kindDirectory
					a.load()
				}
			})
		})
	}
	if a.busy || a.compareRunning {
		components.Spinner(c, components.SpinnerSpokes).Size(14, 14)
		cancelBtn := dangerBtn(c.Key("btn-cancel-task"), i18n.T("action.cancelTask"), nil, pal, false)
		cancelBtn.OnClick(func() {
			a.stopComparison()
			a.compareErr = context.Canceled
			if a.cancel != nil {
				a.cancel()
			}
		})
	}
	// 只有存在比较选项的模式才提供抽屉开关，避免打开一条空抽屉。
	if a.hasOptions() {
		variant := components.ButtonSecondary
		if a.optionsOpen {
			variant = components.ButtonToolbar
		}
		optBtn := toolbarBtn(c.Key("btn-toggle-options"), i18n.T("action.options"), iconSliders, variant, false)
		if a.optionsOpen {
			optBtn.Background(pal.baseBg).Border(1, pal.baseAccent.Alpha(0.35)).TextColor(pal.baseAccent)
		}
		optBtn.OnClick(func() { a.optionsOpen = !a.optionsOpen })
	}
}

func (a *workspaceState) renderWorkspaceMain(c *ui.Context, pal uiPalette) {
	t := c.Theme()
	if a.kind != kindScratch {
		// 双侧来源路径胶囊区（极简紧凑）
		ui.Row(c.Key("path-capsules")).Padding(6, 14).Gap(8).Background(pal.subtleSurface).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
			for side := 0; side < 2; side++ {
				side := side
				sideLabels := []string{i18n.T("pane.leftPath"), i18n.T("pane.rightPath")}
				sideRow := ui.Row(c.Key(side)).Grow(1).Basis(0).MinWidth(0).Height(34).Padding(0, 8).Gap(6).Radius(7).AlignItems(ui.Center)
				if sideRow.FileDragOver() {
					sideRow.Background(pal.baseBg).Border(1.5, pal.baseAccent)
				} else {
					sideRow.Background(pal.cardBg).Border(1, pal.cardBorder)
				}
				if dropped := sideRow.DroppedFiles(); len(dropped) > 0 {
					a.handleDrop(side, dropped)
				}
				sideRow.Children(func() {
					sideBadge(c, side, pal)
					if remote.IsURL(a.paths[side]) {
						statusPill(c, i18n.T("remote.badge"), pal.baseAccent, pal.baseBg)
					}
					components.TextBox(c.Key("input"), &a.paths[side], components.TextBoxOptions{}).
						Grow(1).MinWidth(0).Padding(3, 6).
						Background(ui.Transparent).Border(0, ui.Transparent).
						Placeholder(i18n.T("pane.pathPlaceholder")).
						Label(sideLabels[side]).
						Disabled(a.busy).
						OnSubmit(func() { a.guard(a.load) })
					compactBtn(c.Key("browse"), i18n.T("action.browse"), nil, components.ButtonSecondary, a.busy).
						OnClick(func() { a.browse(side) })
					// 所有双侧来源模式都支持远程来源：打开连接对话框选择，不必手输地址。
					compactBtn(c.Key("remote"), i18n.T("remote.open"), iconRemote, components.ButtonSecondary, a.busy).
						Tooltip(i18n.T("remote.openTooltip")).
						OnClick(func() { a.openRemoteDialog(side) })
				})
			}
		})
		// 文本合并的基准与输出路径和左右来源同属输入，常驻显示，不受比较选项开关影响。
		if a.kind == kindMerge {
			ui.Row(c.Key("merge-capsules")).Padding(6, 14).Gap(8).Background(pal.subtleSurface).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
				a.mergeInputsRow(c, pal)
			})
		}
	}

	// 操作命令工具栏（文本比较与各模式完全对齐）
	if a.kind == kindScratch {
		a.scratchToolbar(c, pal)
	} else {
		a.actionToolbar(c, pal)
	}

	// 比较选项抽屉：位于操作命令工具栏正下方，只在存在比较选项的模式显示。
	if a.optionsOpen && a.hasOptions() {
		a.optionsView(c)
	}
	if a.err != "" {
		ui.Row(c.Key("error-banner")).Padding(6, 16).Gap(8).AlignItems(ui.Center).Background(pal.dangerBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.dangerFg.Alpha(0.2)).Children(func() {
			components.Icon(c, iconAlert).Size(14, 14).TextColor(pal.dangerFg)
			components.Text(c, i18n.T("dialog.error")).Bold().FontSize(11).TextColor(pal.dangerFg)
			components.Text(c, a.err).Grow(1).MinWidth(0).FontSize(11).TextColor(pal.dangerFg)
			compactBtn(c.Key("close-err"), i18n.T("action.closeAlert"), nil, components.ButtonSecondary, false).
				OnClick(func() { a.err = "" })
		})
	}

	// 按模式区分主视图的控件身份，切换模式时各模式的控件状态互不复用。
	ui.Column(c.Key(fmt.Sprintf("view-%d", a.kind))).Grow(1).FillWidth().Children(func() {
		switch {
		case a.kind == kindScratch:
			a.scratchView(c)
		case a.kind == kindMerge:
			a.mergeView(c)
		case a.kind == kindPicture:
			a.pictureView(c)
		case a.kind == kindMedia:
			a.mediaView(c)
		case a.kind == kindVersion:
			a.versionView(c)
		case a.kind == kindTable:
			a.tableView(c)
		case a.kind == kindDirectory && a.directoryRows != nil:
			a.directoryView(c)
		case a.kind == kindFile && a.panes[0] != nil:
			a.textView(c)
		default:
			a.modeEmptyView(c, pal, a.kind)
		}
	})

	message := a.message
	if a.compareRunning {
		message = i18n.T("status.diffRunning")
	} else if a.compareErr != nil {
		message = i18n.T("status.diffStopped")
	}
	if a.busy {
		message = i18n.T("status.processing")
	}
	dotColor := pal.successFg
	if a.busy || a.compareRunning {
		dotColor = pal.diffFg
	} else if a.err != "" || a.compareErr != nil {
		dotColor = pal.dangerFg
	}
	ui.Row(c.Key("status-bar")).Height(26).Padding(0, 14).Gap(6).AlignItems(ui.Center).Background(pal.subtleSurface).BorderWidth(1, 0, 0, 0).BorderColor(pal.cardBorder).Children(func() {
		ui.Box(c).Size(5, 5).Radius(999).Background(dotColor)
		components.Text(c, message).Grow(1).MinWidth(0).FontSize(11).TextColor(t.TextMuted).SingleLine().Tooltip(message)
		components.Text(c, i18n.T("status.saveShortcut")).FontSize(10).TextColor(t.TextMuted)
	})
}

func (a *workspaceState) renderOverlaysAndShortcuts(c *ui.Context, pal uiPalette) {
	a.settingsModal(c, pal)
	a.aboutModal(c, pal)
	a.remoteDialogView(c, pal)
	a.remoteDialogs(c, pal)

	switch components.AlertDialog(c.Key("confirm-sync"), &a.confirmSync, components.AlertDialogOptions{
		Title:        i18n.T("dialog.syncTitle"),
		Message:      i18n.T("dialog.syncDesc"),
		CancelLabel:  i18n.T("dialog.cancel"),
		ConfirmLabel: i18n.T("dialog.execute"),
	}) {
	case components.DialogConfirmed:
		a.executeSync()
	}

	switch components.AlertDialog(c.Key("confirm-discard"), &a.confirm, components.AlertDialogOptions{
		Title:        i18n.T("dialog.discardTitle"),
		Message:      i18n.T("dialog.discardDesc"),
		CancelLabel:  i18n.T("dialog.cancel"),
		ConfirmLabel: i18n.T("dialog.discard"),
	}) {
	case components.DialogConfirmed:
		fn := a.pending
		a.pending = nil
		if fn != nil {
			fn()
		}
	case components.DialogCancelled:
		a.pending = nil
	}

	c.OnShortcut(ui.Cmd, ui.KeyComma, func() {
		a.settingsOpen = !a.settingsOpen
	})
	c.OnShortcut(ui.Cmd, ui.KeyS, func() {
		if a.kind == kindMerge && a.mergeBuffer != nil && a.mergeBuffer.Modified() {
			a.saveMerge()
			return
		}
		if a.kind != kindFile || a.panes[0] == nil {
			return
		}
		side := int(a.activeDiff().ActiveSide())
		if a.mode > 0 {
			side = a.mode - 1
		}
		if a.panes[side] != nil && a.panes[side].buffer.Modified() {
			a.save(side)
		}
	})
	c.OnShortcut(ui.Cmd, ui.KeyZ, func() {
		if a.kind == kindFile || a.kind == kindScratch {
			a.undo()
		}
	})
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyZ, func() {
		if a.kind == kindFile || a.kind == kindScratch {
			a.redo()
		}
	})
}

func (a *workspaceState) view(c *ui.Context) {
	t := applyOnevraTheme(c)
	a.refreshComparison()
	pal := paletteFor(t)

	ui.Row(c).Fill().Background(pal.appBg).Children(func() {
		// 左侧侧边栏（Onevra Sidebar 导航组件）
		ui.Column(c).Width(160).FillHeight().Background(pal.sidebarBg).BorderWidth(0, 1, 0, 0).BorderColor(pal.sidebarBorder).Children(func() {
			a.renderSidebar(c, pal, 10)
		})

		// 右侧主工作区
		ui.Column(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
			ui.Row(c.Key("header-bar")).Height(44).Padding(0, 16).Gap(8).AlignItems(ui.Center).Background(pal.headerBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
				a.renderHeaderContents(c, pal)
			})
			a.renderWorkspaceMain(c, pal)
		})
	})

	a.renderOverlaysAndShortcuts(c, pal)
}

func (a *workspaceState) renderToolbarMeta(c *ui.Context, pal uiPalette) {
	switch a.kind {
	case kindDirectory:
		statusPill(c, i18n.T("meta.dir"), pal.sameFg, pal.sameBg)
		if strings.TrimSpace(a.dirFilter) != "" {
			statusPill(c, i18n.TArgs("dir.filterPill", a.dirFilter), pal.baseAccent, pal.baseBg)
		}
		if a.compareMode {
			statusPill(c, i18n.T("meta.permission"), pal.baseAccent, pal.baseBg)
		}
		if a.compareTime {
			statusPill(c, i18n.T("meta.time"), pal.baseAccent, pal.baseBg)
		}
	case kindMerge:
		if strings.TrimSpace(a.mergeBase) != "" {
			statusPill(c, i18n.T("meta.merge3"), pal.baseAccent, pal.baseBg)
		} else {
			statusPill(c, i18n.T("meta.merge2"), pal.sameFg, pal.sameBg)
		}
	case kindPicture:
		statusPill(c, i18n.T("meta.picture"), pal.sameFg, pal.sameBg)
	case kindMedia:
		statusPill(c, i18n.T("meta.media"), pal.sameFg, pal.sameBg)
	case kindVersion:
		statusPill(c, i18n.T("meta.version"), pal.sameFg, pal.sameBg)
	}
}

func (a *workspaceState) renderHeaderBadge(c *ui.Context, pal uiPalette) {
	switch {
	case a.kind == kindScratch:
		diff := a.activeDiff()
		if diff.ComparisonReady() {
			n := len(diff.Hunks())
			if n == 0 {
				statusPill(c, i18n.T("badge.same"), pal.successFg, pal.successBg)
			} else {
				statusPill(c, i18n.TArgs("badge.diffCount", n), pal.diffFg, pal.diffBg)
			}
		}
	case a.kind == kindFile && a.panes[0] != nil && a.activeDiff().ComparisonReady():
		n := len(a.activeDiff().Hunks())
		if n == 0 {
			statusPill(c, i18n.T("badge.same"), pal.successFg, pal.successBg)
		} else {
			statusPill(c, i18n.TArgs("badge.diffCount", n), pal.diffFg, pal.diffBg)
		}
	case a.kind == kindDirectory && a.directoryRows != nil:
		statusPill(c, i18n.TArgs("badge.itemCount", len(a.directoryRows)), pal.baseAccent, pal.baseBg)
	case a.kind == kindMerge && a.mergeResult != nil:
		n := a.mergeResult.ConflictCount()
		if n == 0 {
			statusPill(c, i18n.T("badge.noConflict"), pal.successFg, pal.successBg)
		} else {
			statusPill(c, i18n.TArgs("badge.conflictCount", n), pal.dangerFg, pal.dangerBg)
		}
	}
}

func (a *workspaceState) canReset() bool {
	if strings.TrimSpace(a.paths[0]) != "" || strings.TrimSpace(a.paths[1]) != "" {
		return true
	}
	if a.err != "" {
		return true
	}
	switch a.kind {
	case kindFile:
		return a.panes[0] != nil || a.panes[1] != nil
	case kindDirectory:
		return a.directoryRows != nil || strings.TrimSpace(a.dirFilter) != ""
	case kindMerge:
		return a.mergeResult != nil || strings.TrimSpace(a.mergeBase) != "" || strings.TrimSpace(a.mergeOutput) != ""
	case kindPicture:
		return a.pictureLeft != nil || a.pictureRight != nil
	case kindMedia:
		return a.mediaReady
	case kindVersion:
		return a.infoReady
	case kindTable:
		return a.tableReady
	case kindScratch:
		return a.scratchPanes[0] != nil && (a.scratchPanes[0].buffer.Len() > 0 || a.scratchPanes[1].buffer.Len() > 0)
	}
	return false
}

func (a *workspaceState) resetCurrentMode() {
	if a.busy {
		return
	}
	a.paths = [2]string{"", ""}
	a.err = ""
	a.message = ""
	a.confirm = false

	switch a.kind {
	case kindFile:
		a.stopComparison()
		a.panes = [2]*pane{nil, nil}
		a.diff = editor.DiffState{}
		a.history = editHistory{}
		a.anchors = nil
		a.anchorLeftSet = false
		a.anchorRightSet = false
	case kindDirectory:
		a.directoryRows = nil
		a.directoryRoots = [2]string{"", ""}
		a.syncPlan = nil
		a.dirMergeDecisions = nil
		a.syncChoice = ""
		a.dirFilter = ""
		a.directoryNote = ""
	case kindMerge:
		a.clearMerge()
		a.mergeBase = ""
		a.mergeOutput = ""
	case kindPicture:
		a.clearPictures()
		a.pictureMode = pictureSide
		a.pictureTol = 2
		a.pictureZoom = pictureZoomFit
	case kindMedia:
		a.clearMedia()
	case kindVersion:
		a.clearVersion()
	case kindTable:
		a.clearTable()
	case kindScratch:
		// 重建面板和差异状态；只清空差异状态会让比较视图失去缓冲区，无法继续输入。
		a.stopComparison()
		a.scratchPanes = [2]*pane{}
		a.ensureScratch()
		a.refreshComparison()
	}
	if a.modeSessions != nil {
		delete(a.modeSessions, a.kind)
	}
	a.message = i18n.T("status.resetDone")
}

func (a *workspaceState) modeEmptyView(c *ui.Context, pal uiPalette, kind int) {
	t := c.Theme()
	icon := modeIcon(kind)
	var (
		tagKey         string
		titleKey       string
		descKey        string
		f1TKey, f1DKey string
		f2TKey, f2DKey string
		f3TKey, f3DKey string
	)
	switch kind {
	case kindDirectory:
		tagKey, titleKey, descKey = "intro.dirTag", "intro.dirTitle", "intro.dirDesc"
		f1TKey, f1DKey = "intro.dirFeat1Title", "intro.dirFeat1Desc"
		f2TKey, f2DKey = "intro.dirFeat2Title", "intro.dirFeat2Desc"
		f3TKey, f3DKey = "intro.dirFeat3Title", "intro.dirFeat3Desc"
	case kindMerge:
		tagKey, titleKey, descKey = "intro.mergeTag", "intro.mergeTitle", "intro.mergeDesc"
		f1TKey, f1DKey = "intro.mergeFeat1Title", "intro.mergeFeat1Desc"
		f2TKey, f2DKey = "intro.mergeFeat2Title", "intro.mergeFeat2Desc"
		f3TKey, f3DKey = "intro.mergeFeat3Title", "intro.mergeFeat3Desc"
	case kindPicture:
		tagKey, titleKey, descKey = "intro.pictureTag", "intro.pictureTitle", "intro.pictureDesc"
		f1TKey, f1DKey = "intro.pictureFeat1Title", "intro.pictureFeat1Desc"
		f2TKey, f2DKey = "intro.pictureFeat2Title", "intro.pictureFeat2Desc"
		f3TKey, f3DKey = "intro.pictureFeat3Title", "intro.pictureFeat3Desc"
	case kindMedia:
		tagKey, titleKey, descKey = "intro.mediaTag", "intro.mediaTitle", "intro.mediaDesc"
		f1TKey, f1DKey = "intro.mediaFeat1Title", "intro.mediaFeat1Desc"
		f2TKey, f2DKey = "intro.mediaFeat2Title", "intro.mediaFeat2Desc"
		f3TKey, f3DKey = "intro.mediaFeat3Title", "intro.mediaFeat3Desc"
	case kindVersion:
		tagKey, titleKey, descKey = "intro.versionTag", "intro.versionTitle", "intro.versionDesc"
		f1TKey, f1DKey = "intro.versionFeat1Title", "intro.versionFeat1Desc"
		f2TKey, f2DKey = "intro.versionFeat2Title", "intro.versionFeat2Desc"
		f3TKey, f3DKey = "intro.versionFeat3Title", "intro.versionFeat3Desc"
	case kindTable:
		tagKey, titleKey, descKey = "intro.tableTag", "intro.tableTitle", "intro.tableDesc"
		f1TKey, f1DKey = "intro.tableFeat1Title", "intro.tableFeat1Desc"
		f2TKey, f2DKey = "intro.tableFeat2Title", "intro.tableFeat2Desc"
		f3TKey, f3DKey = "intro.tableFeat3Title", "intro.tableFeat3Desc"
	default: // 0: 文件比较
		tagKey, titleKey, descKey = "intro.fileTag", "intro.fileTitle", "intro.fileDesc"
		f1TKey, f1DKey = "intro.fileFeat1Title", "intro.fileFeat1Desc"
		f2TKey, f2DKey = "intro.fileFeat2Title", "intro.fileFeat2Desc"
		f3TKey, f3DKey = "intro.fileFeat3Title", "intro.fileFeat3Desc"
	}

	emptyCol := ui.Column(c.Key("empty-col")).Grow(1).FillWidth().Center().Padding(24)
	isDrag := emptyCol.FileDragOver()
	if isDrag {
		emptyCol.Background(pal.baseBg.Alpha(0.2))
	}
	if dropped := emptyCol.DroppedFiles(); len(dropped) > 0 {
		a.handleDrop(-1, dropped)
	}
	emptyCol.Children(func() {
		card := ui.Column(c).MaxWidth(620).Padding(24, 28).Gap(10).Radius(12)
		if isDrag {
			card.Background(pal.cardBg).Border(2, pal.baseAccent).Shadow(0, 4, 16, 0, pal.baseAccent.Alpha(0.2))
		} else {
			card.Background(pal.cardBg).Border(1, pal.cardBorder).Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.04))
		}
		card.Center().Children(func() {
			ui.Row(c).Size(36, 36).Center().Radius(10).Background(pal.baseBg).Border(1, pal.baseAccent.Alpha(0.2)).Children(func() {
				ui.Icon(c, icon).Size(18, 18).TextColor(pal.baseAccent)
			})
			statusPill(c, i18n.T(tagKey), pal.baseAccent, pal.baseBg)
			ui.Text(c, i18n.T(titleKey)).FontSize(20).Bold()
			ui.Text(c, i18n.T(descKey)).FontSize(12).TextColor(t.TextMuted)

			ui.Row(c).Margin(2, 0, 0).Gap(6).Center().Children(func() {
				ui.Icon(c, iconDrop).Size(13, 13).TextColor(pal.baseAccent)
				ui.Text(c, i18n.T("pane.dropPrompt")).FontSize(11).TextColor(pal.baseAccent)
			})

			ui.Row(c).FillWidth().Gap(8).Margin(8, 0, 0).Children(func() {
				feats := []struct{ t, d string }{
					{i18n.T(f1TKey), i18n.T(f1DKey)},
					{i18n.T(f2TKey), i18n.T(f2DKey)},
					{i18n.T(f3TKey), i18n.T(f3DKey)},
				}
				for _, f := range feats {
					box := ui.Column(c).Grow(1).Basis(0).Padding(10, 12).Gap(4).Radius(8).Background(pal.subtleSurface).Border(1, pal.cardBorder)
					box.Children(func() {
						ui.Text(c, f.t).FontSize(11).Bold().TextColor(pal.sidebarText).SingleLine()
						ui.Text(c, f.d).FontSize(10).TextColor(t.TextMuted)
					})
				}
			})

			if kind == kindFile {
				ui.Row(c).Margin(6, 0, 0).Gap(8).Center().Children(func() {
					ui.Text(c, i18n.T("intro.fileScratchHint")).FontSize(11).TextColor(t.TextMuted)
					scratchBtn := ui.Button(c, i18n.T("intro.toScratch")).Padding(3, 10).Radius(6).FontSize(11).Background(pal.baseBg).Border(1, pal.baseAccent.Alpha(0.2)).TextColor(pal.baseAccent)
					scratchBtn.OnClick(func() { a.selectKind(kindScratch) })
				})
			}
		})
	})
}

func (a *workspaceState) actionToolbar(c *ui.Context, pal uiPalette) {
	ui.Row(c.Key("act-toolbar-row")).Wrap().MinHeight(38).Padding(5, 14).Gap(6).AlignItems(ui.Center).Background(pal.headerBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		toolbarBtn(c.Key("act-compare"), i18n.T("action.compare"), iconCompare, components.ButtonPrimary, a.busy).
			OnClick(func() { a.guard(a.load) })

		toolbarBtn(c.Key("act-swap"), i18n.T("action.swap"), iconSwap, components.ButtonSecondary, a.busy).
			OnClick(a.swapInputs)

		toolbarBtn(c.Key("act-reset"), i18n.T("action.reset"), iconReset, components.ButtonSecondary, a.busy || !a.canReset()).
			OnClick(a.resetCurrentMode)

		// 独立合并窗口按左右文本文件逐行合并，只在文件比较和文本合并提供，位置固定在重置之后。
		if a.canOpenMerge() {
			ui.Divider(c).Height(16).Margin(0, 2)
			toolbarBtn(c.Key("act-open-merge"), i18n.T("action.openMerge"), iconExternal, components.ButtonSecondary, a.busy).
				OnClick(a.openMergeWindow)
		}

		if a.kind == kindFile && a.panes[0] != nil {
			ui.Divider(c).Height(16).Margin(0, 2)
			ready := a.activeDiff().ComparisonReady() && !a.busy && !a.compareRunning && a.compareErr == nil
			if ready {
				a.hunkCounter(c, pal)
			}
			toolbarBtn(c.Key("act-prev-diff"), i18n.T("action.prevDiff"), iconChevronUp, components.ButtonSecondary, !ready).
				OnClick(func() { a.navigateDiff(-1) })

			toolbarBtn(c.Key("act-next-diff"), i18n.T("action.nextDiff"), iconChevronDown, components.ButtonSecondary, !ready).
				OnClick(func() { a.navigateDiff(1) })

			toolbarBtn(c.Key("act-apply-left"), i18n.T("action.applyLeft"), nil, components.ButtonSecondary, !ready || a.ignoreBlankLines).
				OnClick(func() { a.apply(0) })
			toolbarBtn(c.Key("act-apply-right"), i18n.T("action.applyRight"), nil, components.ButtonSecondary, !ready || a.ignoreBlankLines).
				OnClick(func() { a.apply(1) })
			a.renderToolbarUndoRedo(c, "act")

			toolbarBtn(c.Key("act-find"), i18n.T("action.find"), iconSearch, components.ButtonSecondary, false).
				OnClick(func() { a.activeDiff().Run(editor.Find) })
		} else {
			ui.Box(c).Grow(1)
			a.renderToolbarMeta(c, pal)
		}
	})
}

func (a *workspaceState) scratchToolbar(c *ui.Context, pal uiPalette) {
	a.ensureScratch()
	leftBuf := a.scratchPanes[0].buffer
	rightBuf := a.scratchPanes[1].buffer
	leftLen := leftBuf.Len()
	rightLen := rightBuf.Len()

	ui.Row(c.Key("scratch-toolbar-row")).Wrap().MinHeight(38).Padding(5, 14).Gap(6).AlignItems(ui.Center).Background(pal.cardBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		toolbarBtn(c.Key("scratch-reset"), i18n.T("action.reset"), iconReset, components.ButtonSecondary, a.busy || !a.canReset()).
			OnClick(a.resetCurrentMode)

		toolbarBtn(c.Key("scratch-clear"), i18n.T("action.clear"), nil, components.ButtonSecondary, leftLen == 0 && rightLen == 0).
			OnClick(func() {
				a.scratchHistory.recordDualEdit(a.scratchPanes, func() {
					leftBuf.Set("")
					rightBuf.Set("")
				})
				a.refreshComparison()
				a.message = i18n.T("status.clearedBoth")
			})

		toolbarBtn(c.Key("scratch-sample"), i18n.T("action.sample"), lucide.Sparkles(), components.ButtonSecondary, false).
			OnClick(func() {
				const sampleLeft = "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, GoCompare!\")\n\titems := []string{\"apple\", \"banana\", \"cherry\"}\n\tfor i, item := range items {\n\t\tfmt.Printf(\"%d: %s\\n\", i, item)\n\t}\n}\n"
				const sampleRight = "package main\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nfunc main() {\n\tfmt.Println(\"Hello, GoCompare v2!\")\n\titems := []string{\"apple\", \"blueberry\", \"cherry\", \"date\"}\n\tfor i, item := range items {\n\t\tfmt.Printf(\"[%d] %s\\n\", i+1, strings.ToUpper(item))\n\t}\n}\n"
				leftBuf.Reset(sampleLeft)
				rightBuf.Reset(sampleRight)
				a.scratchHistory.setBaseline(a.scratchPanes, 0, sampleLeft)
				a.scratchHistory.setBaseline(a.scratchPanes, 1, sampleRight)
				a.scratchHistory.undo, a.scratchHistory.redo = nil, nil
				a.scratchHistory.applied = [2][]editor.DiffLineRange{}
				a.hunkFocus = 0
				a.pendingFocusHunk = 0
				a.recompute()
				a.message = i18n.T("status.sampleLoaded")
			})

		toolbarBtn(c.Key("scratch-swap"), i18n.T("action.swap"), iconSwap, components.ButtonSecondary, a.busy).
			OnClick(func() {
				lt := leftBuf.String()
				rt := rightBuf.String()
				a.scratchHistory.recordDualEdit(a.scratchPanes, func() {
					leftBuf.Set(rt)
					rightBuf.Set(lt)
				})
				a.refreshComparison()
				a.message = i18n.T("status.swappedSides")
			})

		ui.Divider(c).Height(16).Margin(0, 2)

		diff := a.activeDiff()
		ready := diff.ComparisonReady() && !a.busy && !a.compareRunning && a.compareErr == nil
		hasHunks := ready && len(diff.Hunks()) > 0

		if hasHunks {
			a.hunkCounter(c, pal)
		}
		toolbarBtn(c.Key("scratch-prev"), i18n.T("action.prevDiff"), iconChevronUp, components.ButtonSecondary, !hasHunks).
			OnClick(a.diffPrev)

		toolbarBtn(c.Key("scratch-next"), i18n.T("action.nextDiff"), iconChevronDown, components.ButtonSecondary, !hasHunks).
			OnClick(a.diffNext)

		toolbarBtn(c.Key("scratch-apply-left"), i18n.T("action.applyLeft"), nil, components.ButtonSecondary, !ready || a.ignoreBlankLines).
			OnClick(func() { a.apply(0) })
		toolbarBtn(c.Key("scratch-apply-right"), i18n.T("action.applyRight"), nil, components.ButtonSecondary, !ready || a.ignoreBlankLines).
			OnClick(func() { a.apply(1) })
		a.renderToolbarUndoRedo(c, "scratch")

		toolbarBtn(c.Key("scratch-find"), i18n.T("action.find"), iconSearch, components.ButtonSecondary, false).
			OnClick(func() { a.activeDiff().Run(editor.Find) })
	})
}

// hasOptions 报告当前模式是否有比较选项；没有选项的模式不显示抽屉和开关。
func (a *workspaceState) hasOptions() bool {
	return a.kind == kindScratch || a.kind == kindFile || a.kind == kindDirectory
}

// canOpenMerge 报告当前模式的左右来源能否送入独立合并窗口：只有文本文件来源可以。
func (a *workspaceState) canOpenMerge() bool {
	return a.kind == kindFile || a.kind == kindMerge
}

func (a *workspaceState) optionsView(c *ui.Context) {
	t := c.Theme()
	pal := paletteFor(t)
	ui.Column(c.Key("options-col")).Padding(6, 14).Gap(6).Background(pal.subtleSurface).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		ui.Row(c.Key("options-controls-row")).Gap(10).AlignItems(ui.Center).Wrap().Children(func() {
			if a.kind == kindDirectory {
				components.TextBox(c.Key("opt-dir-filter"), &a.dirFilter, components.TextBoxOptions{}).
					Label(i18n.T("option.dirFilter")).
					Placeholder(i18n.T("option.dirFilterPlaceholder")).
					Grow(1).MinWidth(160).Padding(4, 8).
					Disabled(a.busy)
				components.Checkbox(c.Key("opt-dir-mode"), &a.compareMode, i18n.T("option.compareMode")).Disabled(a.busy)
				components.Checkbox(c.Key("opt-dir-time"), &a.compareTime, i18n.T("option.compareTime")).Disabled(a.busy)
				components.Checkbox(c.Key("opt-dir-links"), &a.followLinks, i18n.T("option.followLinks")).Disabled(a.busy)
			}
			if a.kind == kindFile || a.kind == kindScratch {
				if a.kind == kindFile {
					components.Tabs(c.Key("opt-file-mode"), &a.mode, components.TabsSegmented,
						i18n.T("option.modeCompare"),
						i18n.T("option.modeEditLeft"),
						i18n.T("option.modeEditRight"),
					)
				}
				components.Checkbox(c.Key("opt-text-space"), &a.ignoreSpace, i18n.T("option.ignoreSpace")).OnChange(a.refreshComparison)
				components.Checkbox(c.Key("opt-text-blank"), &a.ignoreBlankLines, i18n.T("option.ignoreBlankLines")).OnChange(func() { a.setIgnoreBlankLines(a.ignoreBlankLines) })
				components.Checkbox(c.Key("opt-text-wrap"), &a.wrap, i18n.T("option.wrap"))
				components.Checkbox(c.Key("opt-text-inline"), &a.inlineDiff, i18n.T("option.inlineDiff")).OnChange(func() { a.setInlineDiff(a.inlineDiff) })
				compactBtn(c.Key("opt-text-recompute"), i18n.T("action.recompute"), iconSync, components.ButtonSecondary, a.kind == kindFile && a.panes[0] == nil).
					OnClick(a.recompute)
			}
		})
		if a.kind == kindFile && a.panes[0] != nil {
			ui.Row(c.Key("opt-anchors-row")).Gap(6).AlignItems(ui.Center).Children(func() {
				components.Icon(c, iconAnchor).Size(12, 12).TextColor(pal.baseAccent)
				components.Text(c, i18n.T("option.anchors")).FontSize(11).Bold().TextColor(t.TextMuted)
				compactBtn(c.Key("mark-left"), i18n.T("option.markLeft"), nil, components.ButtonSecondary, false).
					OnClick(func() { a.markAnchorSide(editor.DiffOld) })
				compactBtn(c.Key("mark-right"), i18n.T("option.markRight"), nil, components.ButtonSecondary, false).
					OnClick(func() { a.markAnchorSide(editor.DiffNew) })
				compactBtn(c.Key("align-here"), i18n.T("option.alignHere"), nil, components.ButtonSecondary, false).
					OnClick(a.commitAnchor)
				compactBtn(c.Key("clear-anchors"), i18n.T("option.clearAnchors"), nil, components.ButtonSecondary, false).
					OnClick(a.releaseAnchors)
				if len(a.anchors) > 0 {
					statusPill(c, i18n.TArgs("option.anchorCount", len(a.anchors)), pal.baseAccent, pal.baseBg)
				}
			})
		}
	})
}

func (a *workspaceState) openExternalURL(url string) {
	if !mygo.App.IsReady() {
		return
	}
	if err := mygo.Shell.OpenExternal(url); err != nil {
		a.err = err.Error()
	}
}

func (a *workspaceState) settingsModal(c *ui.Context, pal uiPalette) {
	t := c.Theme()
	modKey := "⌘"
	shiftKey := "⇧⌘"
	if runtime.GOOS != "darwin" {
		modKey = "Ctrl+"
		shiftKey = "Ctrl+Shift+"
	}
	components.Dialog(c.Key("settings-dialog"), &a.settingsOpen, components.DialogOptions{
		Label: i18n.T("settings.title"),
	}, func(panel ui.Element) {
		panel.Width(560).MaxWidth(560)
		if panel.OverlayShortcut(ui.Cmd, ui.KeyComma) {
			a.settingsOpen = false
		}
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				components.Icon(c, iconSliders).Size(16, 16).TextColor(pal.baseAccent)
				components.Text(c, i18n.T("settings.title")).FontSize(15).Bold()
				statusPill(c, modKey+",", pal.sameFg, pal.sameBg)
			})
			ui.Box(c).Grow(1)
			if components.IconButton(c, lucide.X(), "✕", components.IconButtonPlain).Clicked() {
				a.settingsOpen = false
			}
		})

		ui.Box(c).Height(1).FillWidth().Background(pal.cardBorder)

		// 语言设置
		ui.Column(c).FillWidth().Gap(5).Children(func() {
			components.Text(c, i18n.T("settings.language")).FontSize(12).Bold().TextColor(pal.sidebarText)
			components.Text(c, i18n.T("settings.languageHint")).FontSize(11).TextColor(t.TextMuted)
			langTabs := components.Tabs(c.Key("settings-lang-tabs"), &a.settingsLang, components.TabsSegmented,
				i18n.T("settings.langZhCN"),
				i18n.T("settings.langZhTW"),
				i18n.T("settings.langEn"),
			)
			if langTabs.List.Changed() {
				a.applySettings(a.settingsLang, a.settingsTheme)
			}
		})

		// 日间/夜间/系统外观
		ui.Column(c).FillWidth().Gap(5).Children(func() {
			components.Text(c, i18n.T("settings.appearance")).FontSize(12).Bold().TextColor(pal.sidebarText)
			components.Text(c, i18n.T("settings.themeHint")).FontSize(11).TextColor(t.TextMuted)
			themeTabs := components.Tabs(c.Key("settings-theme-tabs"), &a.settingsTheme, components.TabsSegmented,
				i18n.T("settings.themeLight"),
				i18n.T("settings.themeDark"),
				i18n.T("settings.themeSystem"),
			)
			if themeTabs.List.Changed() {
				a.applySettings(a.settingsLang, a.settingsTheme)
			}
		})

		// 主题配色方案
		ui.Column(c).FillWidth().Gap(6).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				components.Icon(c, iconPalette).Size(13, 13).TextColor(pal.baseAccent)
				components.Text(c, i18n.T("settings.palette")).FontSize(12).Bold().TextColor(pal.sidebarText)
			})
			components.Text(c, i18n.T("settings.paletteHint")).FontSize(11).TextColor(t.TextMuted)
			paletteNames := []string{
				i18n.T("settings.paletteClassic"),
				i18n.T("settings.paletteEmerald"),
				i18n.T("settings.paletteViolet"),
				i18n.T("settings.paletteGraphite"),
				i18n.T("settings.paletteAmber"),
			}
			ui.Row(c.Key("settings-palette-row")).FillWidth().Gap(6).Wrap().Children(func() {
				for idx, pName := range paletteNames {
					idx, pName := idx, pName
					selected := a.settingsPalette == idx
					dotColor := palettePreviewColor(idx)
					bg := pal.subtleSurface
					borderCol := pal.cardBorder
					borderW := float32(1)
					if selected {
						bg = pal.baseBg
						borderCol = pal.baseAccent
						borderW = 1.5
					}
					btn := ui.ButtonBase(c.Key(fmt.Sprintf("palette-%d", idx))).
						Label(pName).
						Grow(1).Shrink(0).Padding(6, 8).Gap(5).Radius(7).
						AlignItems(ui.Center).Justify(ui.Center).
						Background(bg).Border(borderW, borderCol).
						Cursor(ui.CursorPointer)
					btn.Children(func() {
						ui.Box(c).Size(10, 10).Radius(999).Background(dotColor).Shrink(0)
						txt := components.Text(c, pName).FontSize(11).Shrink(0)
						if selected {
							txt.Bold().TextColor(pal.baseAccent)
						} else {
							txt.TextColor(pal.sidebarText)
						}
					})
					if btn.Clicked() {
						a.applyPalette(idx)
					}
				}
			})
		})

		ui.Box(c).Height(1).FillWidth().Background(pal.cardBorder)

		// 快捷键一览
		ui.Column(c).FillWidth().Gap(5).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				components.Icon(c, iconKeyboard).Size(13, 13).TextColor(pal.baseAccent)
				components.Text(c, i18n.T("settings.shortcuts")).FontSize(12).Bold().TextColor(pal.sidebarText)
			})
			shortcuts := [][2]string{
				{i18n.T("settings.shortcutPreferences"), modKey + ","},
				{i18n.T("settings.shortcutSave"), modKey + "S"},
				{i18n.T("settings.shortcutUndoRedo"), modKey + "Z / " + shiftKey + "Z"},
				{i18n.T("settings.shortcutFind"), modKey + "F"},
			}
			ui.Row(c).FillWidth().Gap(6).Wrap().Children(func() {
				for _, sc := range shortcuts {
					sc := sc
					ui.Row(c).Grow(1).Basis(0).MinWidth(180).Padding(4, 8).Radius(6).AlignItems(ui.Center).Background(pal.subtleSurface).Border(1, pal.cardBorder).Children(func() {
						components.Text(c, sc[0]).Grow(1).FontSize(11).TextColor(t.TextMuted).SingleLine()
						statusPill(c, sc[1], pal.sidebarText, pal.cardBg)
					})
				}
			})
		})

		ui.Box(c).Height(1).FillWidth().Background(pal.cardBorder)

		// 软件更新与作者/GitHub 信息
		ui.Column(c).FillWidth().Gap(6).Children(func() {
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				components.Text(c, i18n.T("settings.updateSection")).FontSize(12).Bold().TextColor(pal.sidebarText)
				components.Text(c, i18n.TArgs("settings.currentVersion", brand.AppVersion())).FontSize(11).TextColor(t.TextMuted)
				ui.Box(c).Grow(1)
				compactBtn(c.Key("check-update"), i18n.T("settings.checkUpdate"), iconSync, components.ButtonSecondary, a.updateChecking).
					OnClick(func() { a.checkForUpdates("") })
			})
			if a.updateStatus != "" {
				components.Text(c, a.updateStatus).FontSize(11).TextColor(pal.baseAccent)
			}
			if a.updateURL != "" {
				components.Text(c, i18n.TArgs("settings.downloadURL", a.updateURL)).FontSize(11).TextColor(t.TextMuted)
			}

			// 作者与 GitHub 卡片
			ui.Row(c).FillWidth().Padding(8, 10).Gap(8).Radius(7).AlignItems(ui.Center).Background(pal.subtleSurface).Border(1, pal.cardBorder).Children(func() {
				components.Icon(c, iconUser).Size(14, 14).TextColor(pal.baseAccent)
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						components.Text(c, i18n.T("about.author")+": "+brand.Author).FontSize(11).Bold().TextColor(pal.sidebarText)
						statusPill(c, brand.License, pal.sameFg, pal.sameBg)
					})
					components.Text(c, brand.RepoURL).FontSize(10).TextColor(t.TextMuted).SingleLine().Selectable()
				})
				compactBtn(c.Key("settings-open-github"), "GitHub", iconExternal, components.ButtonSecondary, false).
					Tooltip(brand.RepoURL).
					OnClick(func() { a.openExternalURL(brand.RepoURL) })
				compactBtn(c.Key("settings-open-about"), i18n.T("nav.about"), iconInfo, components.ButtonSecondary, false).
					OnClick(func() {
						a.settingsOpen = false
						a.aboutOpen = true
					})
			})
		})

		ui.Box(c).Height(1).FillWidth().Background(pal.cardBorder)

		ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
			if components.Button(c.Key("settings-done"), i18n.T("settings.close"), components.ButtonPrimary).Clicked() {
				a.settingsOpen = false
			}
		})
	})
}

func (a *workspaceState) aboutModal(c *ui.Context, pal uiPalette) {
	t := c.Theme()
	components.Dialog(c.Key("about-dialog"), &a.aboutOpen, components.DialogOptions{
		Label: i18n.T("about.title"),
	}, func(panel ui.Element) {
		panel.Width(420)
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				components.Icon(c, iconInfo).Size(16, 16).TextColor(pal.baseAccent)
				components.Text(c, i18n.T("about.title")).FontSize(15).Bold()
			})
			ui.Box(c).Grow(1)
			if components.IconButton(c, lucide.X(), "✕", components.IconButtonPlain).Clicked() {
				a.aboutOpen = false
			}
		})

		ui.Box(c).Height(1).FillWidth().Background(pal.cardBorder)

		// 顶部品牌标识
		ui.Row(c).FillWidth().Padding(6, 4).Gap(12).AlignItems(ui.Center).Children(func() {
			if icon := a.appIcon(); icon != nil {
				ui.Box(c).Size(48, 48).Center().Radius(12).Background(pal.subtleSurface).Border(1, pal.cardBorder).Children(func() {
					ui.Image(c, icon).Size(36, 36).Label(i18n.T("a11y.appIcon"))
				})
			}
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					components.Text(c, "GoCompare").FontSize(18).Bold().TextColor(pal.sidebarText)
					statusPill(c, "v"+brand.AppVersion(), pal.baseAccent, pal.baseBg)
				})
				components.Text(c, i18n.T("about.desc")).FontSize(11).TextColor(t.TextMuted)
			})
		})

		// 作者与开源信息列表
		ui.Column(c).FillWidth().Padding(10, 12).Gap(8).Radius(8).Background(pal.subtleSurface).Border(1, pal.cardBorder).Children(func() {
			// 作者
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				components.Text(c, i18n.T("about.author")).Width(90).FontSize(11).TextColor(t.TextMuted)
				components.Text(c, brand.Author).Grow(1).FontSize(12).Bold().TextColor(pal.sidebarText).Selectable()
			})
			ui.Box(c).Height(1).FillWidth().Background(pal.cardBorder.Alpha(0.6))
			// GitHub 地址
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				components.Text(c, i18n.T("about.github")).Width(90).FontSize(11).TextColor(t.TextMuted)
				components.Text(c, brand.RepoURL).Grow(1).MinWidth(0).FontSize(11).TextColor(pal.baseAccent).SingleLine().Selectable()
				compactBtn(c.Key("about-open-github"), i18n.T("about.openGithub"), iconExternal, components.ButtonSecondary, false).
					OnClick(func() { a.openExternalURL(brand.RepoURL) })
			})
			ui.Box(c).Height(1).FillWidth().Background(pal.cardBorder.Alpha(0.6))
			// 开源协议
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				components.Text(c, i18n.T("about.license")).Width(90).FontSize(11).TextColor(t.TextMuted)
				components.Text(c, brand.License).Grow(1).FontSize(11).TextColor(pal.sidebarText)
			})
			if fw := a.frameworkVersion(); fw != "" {
				ui.Box(c).Height(1).FillWidth().Background(pal.cardBorder.Alpha(0.6))
				ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
					components.Text(c, i18n.T("about.framework")).Width(90).FontSize(11).TextColor(t.TextMuted)
					components.Text(c, "MyGo "+fw).Grow(1).FontSize(11).TextColor(pal.sidebarText)
				})
			}
		})

		if a.updateStatus != "" {
			components.Text(c, a.updateStatus).FontSize(11).TextColor(pal.baseAccent)
		}

		ui.Box(c).Height(1).FillWidth().Background(pal.cardBorder)

		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			compactBtn(c.Key("about-check-update"), i18n.T("settings.checkUpdate"), iconSync, components.ButtonSecondary, a.updateChecking).
				OnClick(func() { a.checkForUpdates("") })
			ui.Box(c).Grow(1)
			if components.Button(c.Key("about-done"), i18n.T("settings.close"), components.ButtonPrimary).Clicked() {
				a.aboutOpen = false
			}
		})
	})
}

func (a *workspaceState) textView(c *ui.Context) {
	t := c.Theme()
	pal := paletteFor(t)
	ui.Row(c).Gap(1).Background(pal.cardBorder).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		for side, p := range a.panes {
			side, p := side, p
			name := filepath.Base(p.file.Path)
			if p.file.Missing() {
				name += i18n.T("pane.missing")
			}
			isDirty := p.buffer.Modified()
			if isDirty {
				name += " •"
			}
			parent := filepath.Base(filepath.Dir(p.file.Path))
			sideRow := ui.Row(c.Key(fmt.Sprintf("text-pane-header-%d", side))).Grow(1).Basis(0).MinWidth(0).Height(36).Padding(0, 12).Gap(8).AlignItems(ui.Center)
			if sideRow.FileDragOver() {
				sideRow.Background(pal.baseBg).Border(1.5, pal.baseAccent)
			} else {
				sideRow.Background(pal.cardBg)
			}
			if dropped := sideRow.DroppedFiles(); len(dropped) > 0 {
				a.handleDrop(side, dropped)
			}
			sideRow.Children(func() {
				sideBadge(c, side, pal)
				if parent != "" && parent != "." && parent != "/" {
					components.Text(c, parent+"/").Shrink(0).FontSize(11).TextColor(t.TextMuted).SingleLine()
				}
				components.Text(c, name).Bold().FontSize(12).Grow(1).MinWidth(0).SingleLine().Tooltip(p.file.Path)
				if isDirty {
					statusPill(c, i18n.T("pane.unsaved"), pal.diffFg, pal.diffBg)
				}
				if a.ignoreBlankLines {
					statusPill(c, i18n.T("pane.blankIgnored"), pal.baseAccent, pal.baseBg)
				}
				a.renderPaneUndoRedo(c, side)
				saveLabel := []string{i18n.T("pane.saveLeft"), i18n.T("pane.saveRight")}[side]
				compactBtn(c.Key("pane-save"), saveLabel, iconSave, components.ButtonSecondary, a.busy || !isDirty || p.file.ReadOnly()).
					OnClick(func() { a.save(side) })
				compactBtn(c.Key("pane-save-as"), i18n.T("pane.saveAs"), nil, components.ButtonSecondary, a.busy || p.file.ReadOnly()).
					OnClick(func() { a.saveAs(side) })
			})
		}
	})
	if a.mode == 0 {
		a.comparisonView(c, "comparison")
	} else {
		side := a.mode - 1
		a.syncAppliedDecorations(&a.diff, pal)
		p := a.panes[side]
		p.edit.SoftWrap = a.wrap
		p.edit.ReadOnly = a.busy || (p.file != nil && p.file.ReadOnly())
		p.edit.LineBackgrounds = a.diff.Backgrounds[side]
		p.edit.GutterMarks = a.diff.GutterMarks[side]
		p.edit.DiffBase = a.activeHistory().baseBuf[side]
		editor.View(c.Key(a.mode), &p.edit).Grow(1).FillWidth()
	}
	ui.Row(c).Height(26).Padding(0, 12).Gap(12).AlignItems(ui.Center).Background(pal.cardBg).BorderWidth(1, 0, 0, 0).BorderColor(pal.cardBorder).Children(func() {
		for _, p := range a.panes {
			ending := "LF"
			if p.file.CRLF {
				ending = "CRLF"
			}
			ui.Row(c).Grow(1).Basis(0).MinWidth(0).Gap(6).AlignItems(ui.Center).Children(func() {
				components.Icon(c, iconFile).Size(11, 11).TextColor(t.TextMuted)
				components.Text(c, i18n.TArgs("pane.fileInfo", strings.ToUpper(p.file.Encoding), ending, p.buffer.LineCount())).Grow(1).FontSize(11).TextColor(t.TextMuted).SingleLine()
				if lang := p.buffer.Language(); lang != nil && lang.Name != "" {
					statusPill(c, lang.Name, pal.sameFg, pal.sameBg)
				}
			})
		}
	})
}

func (a *workspaceState) scratchView(c *ui.Context) {
	t := c.Theme()
	pal := paletteFor(t)
	a.ensureScratch()

	leftBuf := a.scratchPanes[0].buffer
	rightBuf := a.scratchPanes[1].buffer
	leftLen := leftBuf.Len()
	rightLen := rightBuf.Len()
	leftLines := leftBuf.LineCount()
	rightLines := rightBuf.LineCount()

	// 双栏头部（左侧与右侧）
	ui.Row(c.Key("scratch-header-row")).Gap(1).Background(pal.cardBorder).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		// 左侧头部
		ui.Row(c.Key("scratch-header-left")).Grow(1).Basis(0).MinWidth(0).Height(38).Padding(0, 12).Gap(8).AlignItems(ui.Center).Background(pal.cardBg).Children(func() {
			sideBadge(c, 0, pal)
			components.Text(c, i18n.T("pane.leftText")).Bold().FontSize(12).TextColor(pal.sidebarText).SingleLine()
			stat := i18n.TArgs("scratch.stat", leftLines, leftLen)
			components.Text(c, stat).FontSize(11).TextColor(t.TextMuted).SingleLine()
			ui.Box(c).Grow(1)
			a.renderPaneUndoRedo(c, 0)

			pasteBtn := compactBtn(c.Key("paste-left"), i18n.T("pane.paste"), nil, components.ButtonSecondary, false).
				Label(i18n.T("pane.pasteLeft"))
			pasteBtn.OnClick(func() {
				clip := c.Services().ReadClipboard()
				if clip != "" {
					leftBuf.Reset(clip)
					a.scratchHistory.setBaseline(a.scratchPanes, 0, clip)
					a.scratchHistory.removeSide(&a.scratchHistory.undo, 0)
					a.scratchHistory.removeSide(&a.scratchHistory.redo, 0)
					a.scratchHistory.applied[0] = nil
					a.recompute()
					a.message = i18n.TArgs("scratch.pastedLeft", len(clip))
				} else {
					a.message = i18n.T("scratch.clipboardEmpty")
				}
			})

			compactBtn(c.Key("copy-left"), i18n.T("pane.copy"), nil, components.ButtonSecondary, leftLen == 0).
				OnClick(func() {
					c.Services().WriteClipboard(leftBuf.String())
					a.message = i18n.T("scratch.copiedLeft")
				})

			compactBtn(c.Key("clear-left"), i18n.T("pane.clear"), nil, components.ButtonSecondary, leftLen == 0).
				OnClick(func() {
					a.scratchHistory.sync(a.scratchPanes)
					leftBuf.Set("")
					a.scratchHistory.recordEdit(a.scratchPanes, 0)
					a.refreshComparison()
				})

			compactBtn(c.Key("save-left"), i18n.T("pane.saveAs"), nil, components.ButtonSecondary, leftLen == 0).
				OnClick(func() { a.saveScratchAs(0) })
		})

		// 右侧头部
		ui.Row(c.Key("scratch-header-right")).Grow(1).Basis(0).MinWidth(0).Height(38).Padding(0, 12).Gap(8).AlignItems(ui.Center).Background(pal.cardBg).Children(func() {
			sideBadge(c, 1, pal)
			components.Text(c, i18n.T("pane.rightText")).Bold().FontSize(12).TextColor(pal.sidebarText).SingleLine()
			stat := i18n.TArgs("scratch.stat", rightLines, rightLen)
			components.Text(c, stat).FontSize(11).TextColor(t.TextMuted).SingleLine()
			ui.Box(c).Grow(1)
			a.renderPaneUndoRedo(c, 1)

			pasteBtn := compactBtn(c.Key("paste-right"), i18n.T("pane.paste"), nil, components.ButtonSecondary, false).
				Label(i18n.T("pane.pasteRight"))
			pasteBtn.OnClick(func() {
				clip := c.Services().ReadClipboard()
				if clip != "" {
					rightBuf.Reset(clip)
					a.scratchHistory.setBaseline(a.scratchPanes, 1, clip)
					a.scratchHistory.removeSide(&a.scratchHistory.undo, 1)
					a.scratchHistory.removeSide(&a.scratchHistory.redo, 1)
					a.scratchHistory.applied[1] = nil
					a.recompute()
					a.message = i18n.TArgs("scratch.pastedRight", len(clip))
				} else {
					a.message = i18n.T("scratch.clipboardEmpty")
				}
			})

			compactBtn(c.Key("copy-right"), i18n.T("pane.copy"), nil, components.ButtonSecondary, rightLen == 0).
				OnClick(func() {
					c.Services().WriteClipboard(rightBuf.String())
					a.message = i18n.T("scratch.copiedRight")
				})

			compactBtn(c.Key("clear-right"), i18n.T("pane.clear"), nil, components.ButtonSecondary, rightLen == 0).
				OnClick(func() {
					a.scratchHistory.sync(a.scratchPanes)
					rightBuf.Set("")
					a.scratchHistory.recordEdit(a.scratchPanes, 1)
					a.refreshComparison()
				})

			compactBtn(c.Key("save-right"), i18n.T("pane.saveAs"), nil, components.ButtonSecondary, rightLen == 0).
				OnClick(func() { a.saveScratchAs(1) })
		})
	})

	if leftLen == 0 && rightLen == 0 {
		ui.Row(c).Height(32).Padding(0, 14).Gap(8).AlignItems(ui.Center).Background(pal.subtleSurface).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
			components.Text(c, i18n.T("pane.scratchTip")).FontSize(11).TextColor(t.TextMuted).SingleLine()
		})
	}

	// 核心双栏 DiffView
	a.comparisonView(c, "scratch_comparison")
}

// comparisonView 绘制当前比较视图；忽略空行时使用独立键和只读副本，避免与可编辑视图共享控件状态。
func (a *workspaceState) comparisonView(c *ui.Context, key string) {
	diff := a.activeDiff()
	pal := paletteFor(c.Theme())
	editable := !a.busy && !a.ignoreBlankLines
	if a.ignoreBlankLines {
		key += "_blank"
	}
	diff.Wrap = a.wrap
	panes := a.activePanes()
	diff.Editable = editable && (panes[1] == nil || panes[1].file == nil || !panes[1].file.ReadOnly())
	diff.OldEditable = editable && (panes[0] == nil || panes[0].file == nil || !panes[0].file.ReadOnly())
	a.syncAppliedDecorations(diff, pal)
	diff.CenterWidth = 40
	diff.RenderCenter = func(c *ui.Context, col ui.Element) {
		a.renderDiffCenter(c, col, diff, panes, editable)
	}
	editor.DiffView(c.Key(key), diff).Grow(1).FillWidth()
}

// syncAppliedDecorations 将各侧相对于初始基线已应用/已修改的行区间同步为编辑器背景色与行号槽标记。
func (a *workspaceState) syncAppliedDecorations(diff *editor.DiffState, pal uiPalette) {
	if a.ignoreBlankLines {
		diff.Backgrounds = [2][]editor.LineBackground{}
		diff.GutterMarks = [2][]editor.GutterMark{}
		return
	}
	h := a.activeHistory()
	bgCol := pal.baseAccent.Alpha(0.14)
	markCol := pal.baseAccent
	for side := 0; side < 2; side++ {
		ranges := h.applied[side]
		if len(ranges) == 0 {
			diff.Backgrounds[side] = nil
			diff.GutterMarks[side] = nil
			continue
		}
		bgs := make([]editor.LineBackground, 0, len(ranges))
		var marks []editor.GutterMark
		for _, r := range ranges {
			bgs = append(bgs, editor.LineBackground{
				First: r.Start,
				Last:  r.End,
				Color: bgCol,
			})
			for line := r.Start; line <= r.End; line++ {
				marks = append(marks, editor.GutterMark{
					Line:  line,
					Color: markCol,
				})
			}
		}
		diff.Backgrounds[side] = bgs
		diff.GutterMarks[side] = marks
	}
}

// renderDiffCenter 在双栏文本比较中间绘制差异块连接与焦点列，支持点击聚焦、就地向左右两侧应用差异块，并标出已应用的修改位置。
func (a *workspaceState) renderDiffCenter(c *ui.Context, col ui.Element, diff *editor.DiffState, panes [2]*pane, editable bool) {
	t := c.Theme()
	pal := paletteFor(t)
	col.Background(pal.subtleSurface).BorderWidth(0, 1, 0, 1).BorderColor(pal.cardBorder)
	if a.compareRunning || a.compareErr != nil || !diff.ComparisonReady() {
		return
	}
	bands := diff.HunkBands()
	var appliedRanges []editor.DiffLineRange
	if !a.ignoreBlankLines {
		h := a.activeHistory()
		appliedRanges = append(appliedRanges, h.applied[0]...)
		appliedRanges = append(appliedRanges, h.applied[1]...)
	}
	appliedBands := diff.LineBands(appliedRanges)
	if len(bands) == 0 && len(appliedBands) == 0 {
		return
	}
	leftCanApply := editable && (panes[0] == nil || panes[0].file == nil || !panes[0].file.ReadOnly())
	rightCanApply := editable && (panes[1] == nil || panes[1].file == nil || !panes[1].file.ReadOnly())
	total := len(bands)
	col.Children(func() {
		for idx, ab := range appliedBands {
			ab := ab
			h := max(ab.Height, float32(18))
			if ab.Top+h < -100 || ab.Top > 4000 {
				continue
			}
			isLeft := ab.Range.Side == editor.DiffOld
			tip := i18n.T("diff.appliedRight")
			accent := pal.rightAccent
			bg := pal.rightBg.Alpha(0.85)
			if isLeft {
				tip = i18n.T("diff.appliedLeft")
				accent = pal.leftAccent
				bg = pal.leftBg.Alpha(0.85)
			}
			badge := ui.Row(c.Key(fmt.Sprintf("diff-applied-%d-%d", ab.Range.Side, idx))).
				Label(tip).
				Absolute().Left(2).Right(2).Top(ab.Top).Height(h).
				Center().Gap(1).Radius(4).
				Background(bg).Border(1, accent.Alpha(0.45)).
				Tooltip(tip).Cursor(ui.CursorPointer)
			badge.OnClick(func() { diff.SetSelectedLines(ab.Range, true) })
			badge.Children(func() {
				if isLeft {
					ui.Icon(c, iconChevronLeft).Size(9, 9).TextColor(accent)
					ui.Icon(c, iconCheck).Size(9, 9).TextColor(accent)
				} else {
					ui.Icon(c, iconCheck).Size(9, 9).TextColor(accent)
					ui.Icon(c, iconChevronRight).Size(9, 9).TextColor(accent)
				}
			})
		}
		for _, band := range bands {
			band := band
			h := max(band.Height, float32(20))
			if band.Top+h < -100 || band.Top > 4000 {
				continue
			}
			focused := a.hunkFocus == band.Index+1
			bg := pal.diffBg.Alpha(0.55)
			borderCol := pal.diffFg.Alpha(0.35)
			borderW := float32(1)
			if focused {
				bg = pal.diffBg
				borderCol = pal.diffFg
				borderW = 1.5
			}
			posTip := i18n.TArgs("status.hunkPosition", band.Index+1, total)
			item := ui.Row(c.Key(fmt.Sprintf("diff-band-%d", band.Index))).
				Absolute().Left(1).Right(1).Top(band.Top).Height(h).
				Center().Gap(2).Radius(4).
				Background(bg).Border(borderW, borderCol).
				Tooltip(posTip).Cursor(ui.CursorPointer)
			item.OnClick(func() { a.selectHunk(band.Index) })
			item.Children(func() {
				if leftCanApply || rightCanApply {
					btnBg := pal.cardBg.Alpha(0.88)
					if focused {
						btnBg = pal.cardBg
					}
					leftLabel := fmt.Sprintf("%s #%d", i18n.T("action.applyLeft"), band.Index+1)
					leftBtn := ui.Button(c.Key(fmt.Sprintf("diff-band-left-%d", band.Index)), "").
						Label(leftLabel).Tooltip(i18n.T("action.applyLeft")).
						Size(16, 16).Padding(0).Radius(3).
						Background(btnBg).Border(1, borderCol).
						Disabled(!leftCanApply)
					leftBtn.OnClick(func() { a.applyHunk(band.Index, 0) })
					leftBtn.Children(func() {
						ui.Icon(c, iconChevronLeft).Size(10, 10).TextColor(pal.leftAccent)
					})

					rightLabel := fmt.Sprintf("%s #%d", i18n.T("action.applyRight"), band.Index+1)
					rightBtn := ui.Button(c.Key(fmt.Sprintf("diff-band-right-%d", band.Index)), "").
						Label(rightLabel).Tooltip(i18n.T("action.applyRight")).
						Size(16, 16).Padding(0).Radius(3).
						Background(btnBg).Border(1, borderCol).
						Disabled(!rightCanApply)
					rightBtn.OnClick(func() { a.applyHunk(band.Index, 1) })
					rightBtn.Children(func() {
						ui.Icon(c, iconChevronRight).Size(10, 10).TextColor(pal.rightAccent)
					})
				}
			})
		}
	})
}

// renderToolbarUndoRedo 在公共工具栏按需渲染跨侧撤销、重做与恢复按钮；无修改时不占位。
func (a *workspaceState) renderToolbarUndoRedo(c *ui.Context, prefix string) {
	disabled := a.busy || a.ignoreBlankLines
	if a.canUndo() {
		toolbarBtn(c.Key(prefix+"-undo"), i18n.T("action.undo"), iconUndo, components.ButtonSecondary, disabled).
			OnClick(a.undo)
	}
	if a.canRedo() {
		toolbarBtn(c.Key(prefix+"-redo"), i18n.T("action.redo"), iconRedo, components.ButtonSecondary, disabled).
			OnClick(a.redo)
	}
	if a.canRestore() {
		toolbarBtn(c.Key(prefix+"-restore"), i18n.T("action.restore"), iconReset, components.ButtonSecondary, disabled).
			OnClick(a.restoreAll)
	}
}

// renderPaneUndoRedo 在左右分栏头部按需渲染本侧已改标记、独立撤销、重做与恢复按钮；本侧无修改时不显示。
func (a *workspaceState) renderPaneUndoRedo(c *ui.Context, side int) {
	panes, _ := a.basePanes()
	if side < 0 || side > 1 || panes[side] == nil {
		return
	}
	p := panes[side]
	pal := paletteFor(c.Theme())
	if !a.ignoreBlankLines {
		if n := len(a.activeHistory().applied[side]); n > 0 {
			statusPill(c, i18n.TArgs("pane.appliedCount", n), pal.baseAccent, pal.baseBg)
		}
	}
	disabled := a.busy || a.ignoreBlankLines || (p.file != nil && p.file.ReadOnly())
	if p.buffer.CanUndo() {
		label := i18n.T([]string{"pane.undoLeft", "pane.undoRight"}[side])
		compactBtn(c.Key("pane-undo"), i18n.T("action.undo"), iconUndo, components.ButtonSecondary, disabled).
			Label(label).Tooltip(label).
			OnClick(func() { a.undoSide(side) })
	}
	if p.buffer.CanRedo() {
		label := i18n.T([]string{"pane.redoLeft", "pane.redoRight"}[side])
		compactBtn(c.Key("pane-redo"), i18n.T("action.redo"), iconRedo, components.ButtonSecondary, disabled).
			Label(label).Tooltip(label).
			OnClick(func() { a.redoSide(side) })
	}
	if a.canRestoreSide(side) {
		label := i18n.T([]string{"pane.restoreLeft", "pane.restoreRight"}[side])
		compactBtn(c.Key("pane-restore"), i18n.T("action.restore"), iconReset, components.ButtonSecondary, disabled).
			Label(label).Tooltip(label).
			OnClick(func() { a.restoreSide(side) })
	}
}
