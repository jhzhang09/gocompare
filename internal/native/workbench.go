package native

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"gocompare/internal/document"
	"gocompare/internal/i18n"
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
		a.compareRequested = false
		currentPaths := a.paths
		hadResult := a.panes[0] != nil || a.directoryRows != nil || a.mergeResult != nil || a.pictureResult != nil || a.mediaReady || a.infoReady

		if hadResult || currentPaths[0] != "" || currentPaths[1] != "" {
			a.saveModeSession(a.kind)
		}

		a.kind = kind
		a.err = ""
		if !a.restoreModeSession(kind) {
			a.panes = [2]*pane{}
			a.directoryRows = nil
			a.syncPlan = nil
			a.clearMerge()
			a.clearPictures()
			a.clearMedia()
			a.clearVersion()
			a.allowMissing = false
			a.directoryRoots = [2]string{}
			if !hadResult {
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
	n := len(diff.Hunks())
	if n == 0 {
		return
	}
	i, _ := diff.CurrentHunk()
	if i < 0 {
		if step > 0 {
			i = 0
		} else {
			i = n - 1
		}
	} else {
		i = (i + step + n) % n
	}
	diff.GoToHunk(i)
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

func (a *workspaceState) view(c *ui.Context) {
	a.refreshComparison()
	t := c.Theme()
	pal := paletteFor(t)

	ui.Row(c).Fill().Background(pal.appBg).Children(func() {
		// 左侧侧边栏（深度融合明暗主题）
		ui.Column(c).Width(160).FillHeight().Padding(12, 8).Gap(3).Background(pal.sidebarBg).BorderWidth(0, 1, 0, 0).BorderColor(pal.sidebarBorder).Children(func() {
			ui.Row(c).Padding(4, 6).Gap(8).Margin(0, 0, 12).Children(func() {
				if icon := a.appIcon(); icon != nil {
					ui.Box(c).Size(26, 26).Center().Radius(6).Background(pal.cardBg).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.08)).Children(func() {
						ui.Image(c, icon).Size(20, 20).Label(i18n.T("a11y.appIcon"))
					})
				}
				ui.Column(c).Gap(1).Children(func() {
					ui.Text(c, "GoCompare").FontSize(14).Bold().TextColor(pal.sidebarText).SingleLine()
					ui.Text(c, "Diff Workbench").FontSize(9).TextColor(pal.sidebarMuted).SingleLine()
				})
			})

			ui.Text(c, i18n.T("nav.modes")).Padding(2, 8).FontSize(10).Bold().TextColor(pal.sidebarMuted)
			for _, m := range modes {
				m := m
				active := a.kind == m.kind
				bg := ui.Transparent
				fg := pal.sidebarMuted
				label := i18n.T(m.msgKey)
				btn := ui.Button(c, "").Label(label).FillWidth().Height(34).Padding(0, 8).Gap(7).Radius(6).Justify(ui.Start).BorderWidth(0).Background(bg)
				if active {
					btn.Background(pal.sidebarItemActive).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.06))
					fg = pal.sidebarText
				}
				btn.TextColor(fg)
				btn.OnClick(func() { a.selectKind(m.kind) })
				btn.Children(func() {
					iconSvg := m.icon
					iconCol := pal.sidebarMuted
					if active {
						iconCol = pal.baseAccent
					}
					ui.Icon(c, iconSvg).Size(14, 14).TextColor(iconCol)
					txt := ui.Text(c, label).FontSize(12).SingleLine()
					if active {
						txt.Bold().TextColor(pal.sidebarText)
					}
				})
			}

			ui.Box(c).Grow(1)
			ui.Box(c).Height(1).FillWidth().Margin(6, 4).Background(pal.sidebarBorder)
			ui.Text(c, i18n.T("nav.session")).Padding(2, 8).FontSize(10).Bold().TextColor(pal.sidebarMuted)

			openBtn := ui.Button(c, "").Label(i18n.T("nav.openSession")).FillWidth().Height(30).Padding(0, 8).Gap(6).Radius(6).Justify(ui.Start).Background(pal.sidebarBtnBg).Border(1, pal.sidebarBtnBorder).TextColor(pal.sidebarText).Disabled(a.busy)
			openBtn.OnClick(func() { a.sessionDialog(false) })
			openBtn.Children(func() {
				ui.Icon(c, iconFolderOpen).Size(12, 12).TextColor(pal.sidebarMuted)
				ui.Text(c, i18n.T("nav.openSession")).FontSize(11).SingleLine()
			})

			saveBtn := ui.Button(c, "").Label(i18n.T("nav.saveSession")).FillWidth().Height(30).Padding(0, 8).Gap(6).Radius(6).Justify(ui.Start).Background(pal.sidebarBtnBg).Border(1, pal.sidebarBtnBorder).TextColor(pal.sidebarText).Disabled(a.busy || !a.canSaveSession())
			saveBtn.OnClick(func() { a.sessionDialog(true) })
			saveBtn.Children(func() {
				ui.Icon(c, iconSave).Size(12, 12).TextColor(pal.sidebarMuted)
				ui.Text(c, i18n.T("nav.saveSession")).FontSize(11).SingleLine()
			})

			settingsBtn := ui.Button(c, "").Label(i18n.T("nav.settings")).FillWidth().Height(30).Padding(0, 8).Gap(6).Radius(6).Justify(ui.Start).Background(pal.sidebarBtnBg).Border(1, pal.sidebarBtnBorder).TextColor(pal.sidebarText)
			settingsBtn.OnClick(func() { a.settingsOpen = true })
			settingsBtn.Children(func() {
				ui.Icon(c, iconSliders).Size(12, 12).TextColor(pal.sidebarMuted)
				ui.Text(c, i18n.T("nav.settings")).FontSize(11).SingleLine()
			})
		})

		// 右侧主工作区
		ui.Column(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
			title := modeTitle(a.kind)
			// 顶部标题栏（沉浸式对齐设计）
			ui.Row(c).Height(44).Padding(0, 16).Gap(8).Background(pal.headerBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
				ui.Row(c).Size(24, 24).Center().Radius(6).Background(pal.baseBg).Children(func() {
					ui.Icon(c, modeIcon(a.kind)).Size(13, 13).TextColor(pal.baseAccent)
				})
				ui.Text(c, title).FontSize(15).Bold().SingleLine()
				a.renderHeaderBadge(c, pal)
				ui.Box(c).Grow(1)

				if a.kind == kindFile && a.panes[0] != nil && a.directoryRoots[0] != "" {
					backBtn := ui.Button(c, "").Label(i18n.T("action.backToFolder")).Padding(4, 9).Gap(4).Radius(6).Disabled(a.busy)
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
					backBtn.Children(func() {
						ui.Icon(c, iconFolderOpen).Size(12, 12)
						ui.Text(c, i18n.T("action.backToFolder")).FontSize(11).SingleLine()
					})
				}
				if a.kind == kindMerge {
					winBtn := ui.Button(c, "").Label(i18n.T("action.openMerge")).Padding(4, 9).Gap(4).Radius(6)
					winBtn.OnClick(a.openMergeWindow)
					winBtn.Children(func() {
						ui.Icon(c, iconExternal).Size(12, 12)
						ui.Text(c, i18n.T("action.openMerge")).FontSize(11).SingleLine()
					})
				}
				if a.busy || a.compareRunning {
					ui.Spinner(c).Size(14, 14)
					cancelBtn := ui.Button(c, i18n.T("action.cancelTask")).Padding(4, 8).Radius(6).Background(pal.dangerBg).Border(1, pal.dangerFg.Alpha(0.25)).TextColor(pal.dangerFg).FontSize(11)
					cancelBtn.OnClick(func() {
						a.stopComparison()
						a.compareErr = context.Canceled
						if a.cancel != nil {
							a.cancel()
						}
					})
				}
				optBtn := ui.Button(c, "").Label(i18n.T("action.options")).Padding(4, 9).Gap(4).Radius(6)
				if a.optionsOpen {
					optBtn.Background(pal.baseBg).Border(1, pal.baseAccent.Alpha(0.35)).TextColor(pal.baseAccent)
				}
				optBtn.OnClick(func() { a.optionsOpen = !a.optionsOpen })
				optBtn.Children(func() {
					ui.Icon(c, iconSliders).Size(12, 12)
					ui.Text(c, i18n.T("action.options")).FontSize(11).SingleLine()
				})
			})

			if a.kind != kindScratch {
				// 双侧来源路径胶囊区（极简紧凑）
				ui.Row(c).Padding(6, 14).Gap(8).Background(pal.subtleSurface).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
					for side := 0; side < 2; side++ {
						side := side
						sideLabels := []string{i18n.T("pane.leftPath"), i18n.T("pane.rightPath")}
						ui.Row(c).Grow(1).Basis(0).MinWidth(0).Height(34).Padding(0, 8).Gap(6).Radius(7).Background(pal.cardBg).Border(1, pal.cardBorder).Children(func() {
							sideBadge(c, side, pal)
							ui.TextInput(c.Key(side), &a.paths[side]).Grow(1).MinWidth(0).Placeholder(i18n.T("pane.pathPlaceholder")).Label(sideLabels[side]).Disabled(a.busy).OnSubmit(func() { a.guard(a.load) })
							ui.Button(c, i18n.T("action.browse")).Shrink(0).Padding(3, 8).Radius(5).FontSize(11).Disabled(a.busy).OnClick(func() { a.browse(side) })
						})
					}
				})
			}

			// 操作命令工具栏（文本比较与各模式完全对齐）
			if a.kind == kindScratch {
				a.scratchToolbar(c, pal)
			} else {
				a.actionToolbar(c, pal)
			}

			// 比较选项抽屉：无论何种模式，都严格位于操作命令工具栏正下方！
			if a.optionsOpen {
				a.optionsView(c)
			}
			if a.err != "" {
				ui.Row(c).Padding(6, 16).Gap(8).Background(pal.dangerBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.dangerFg.Alpha(0.2)).Children(func() {
					ui.Icon(c, iconAlert).Size(14, 14).TextColor(pal.dangerFg)
					ui.Text(c, i18n.T("dialog.error")).Bold().FontSize(11).TextColor(pal.dangerFg)
					ui.Text(c, a.err).Grow(1).MinWidth(0).FontSize(11).TextColor(pal.dangerFg)
					ui.Button(c, i18n.T("action.closeAlert")).Padding(2, 8).Radius(4).FontSize(10).OnClick(func() { a.err = "" })
				})
			}

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
			case a.kind == kindDirectory && a.directoryRows != nil:
				a.directoryView(c)
			case a.kind == kindFile && a.panes[0] != nil:
				a.textView(c)
			default:
				a.modeEmptyView(c, pal, a.kind)
			}

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
			ui.Row(c).Height(26).Padding(0, 14).Gap(6).Background(pal.subtleSurface).BorderWidth(1, 0, 0, 0).BorderColor(pal.cardBorder).Children(func() {
				ui.Box(c).Size(5, 5).Radius(999).Background(dotColor)
				ui.Text(c, message).Grow(1).MinWidth(0).FontSize(11).TextColor(t.TextMuted).SingleLine().Tooltip(message)
				ui.Text(c, i18n.T("status.saveShortcut")).FontSize(10).TextColor(t.TextMuted)
			})
		})
	})

	if a.settingsOpen {
		a.settingsModal(c, pal)
	}

	if a.confirmSync {
		choice := ui.AlertDialog(c, &a.confirmSync, i18n.T("dialog.syncTitle"), i18n.T("dialog.syncDesc"), i18n.T("dialog.cancel"), i18n.T("dialog.execute"))
		if choice == 1 {
			a.executeSync()
		}
	}
	if a.confirm {
		choice := ui.AlertDialog(c, &a.confirm, i18n.T("dialog.discardTitle"), i18n.T("dialog.discardDesc"), i18n.T("dialog.cancel"), i18n.T("dialog.discard"))
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
		a.anchors = nil
		a.anchorLeftSet = false
		a.anchorRightSet = false
	case kindDirectory:
		a.directoryRows = nil
		a.directoryRoots = [2]string{"", ""}
		a.syncPlan = nil
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
	case kindScratch:
		// 重建面板和差异状态；只清空差异状态会让比较视图失去缓冲区，无法继续输入。
		a.stopComparison()
		a.scratchPanes = [2]*pane{}
		a.ensureScratch()
		a.refreshComparison()
	}
	if a.modeSessions != nil {
		a.modeSessions[a.kind] = nil
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
	default: // 0: 文件比较
		tagKey, titleKey, descKey = "intro.fileTag", "intro.fileTitle", "intro.fileDesc"
		f1TKey, f1DKey = "intro.fileFeat1Title", "intro.fileFeat1Desc"
		f2TKey, f2DKey = "intro.fileFeat2Title", "intro.fileFeat2Desc"
		f3TKey, f3DKey = "intro.fileFeat3Title", "intro.fileFeat3Desc"
	}

	ui.Column(c).Grow(1).FillWidth().Center().Padding(24).Children(func() {
		ui.Column(c).MaxWidth(620).Padding(24, 28).Gap(10).Radius(12).Background(pal.cardBg).Border(1, pal.cardBorder).Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.04)).Center().Children(func() {
			ui.Row(c).Size(36, 36).Center().Radius(10).Background(pal.baseBg).Border(1, pal.baseAccent.Alpha(0.2)).Children(func() {
				ui.Icon(c, icon).Size(18, 18).TextColor(pal.baseAccent)
			})
			statusPill(c, i18n.T(tagKey), pal.baseAccent, pal.baseBg)
			ui.Text(c, i18n.T(titleKey)).FontSize(20).Bold()
			ui.Text(c, i18n.T(descKey)).FontSize(12).TextColor(t.TextMuted)

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
	ui.Row(c).Height(38).Padding(0, 14).Gap(6).Background(pal.headerBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		startBtn := ui.PrimaryButton(c, "").Label(i18n.T("action.compare")).Padding(4, 10).Gap(4).Radius(6).Disabled(a.busy)
		startBtn.OnClick(func() { a.guard(a.load) })
		startBtn.Children(func() {
			ui.Icon(c, iconCompare).Size(11, 11)
			ui.Text(c, i18n.T("action.compare")).FontSize(11).Bold().SingleLine()
		})

		swapBtn := ui.Button(c, "").Label(i18n.T("action.swap")).Padding(4, 9).Gap(4).Radius(6).Disabled(a.busy)
		swapBtn.OnClick(a.swapInputs)
		swapBtn.Children(func() {
			ui.Icon(c, iconSwap).Size(11, 11)
			ui.Text(c, i18n.T("action.swap")).FontSize(11).SingleLine()
		})

		resetBtn := ui.Button(c, "").Label(i18n.T("action.reset")).Padding(4, 9).Gap(4).Radius(6).Disabled(a.busy || !a.canReset())
		resetBtn.OnClick(a.resetCurrentMode)
		resetBtn.Children(func() {
			ui.Icon(c, iconReset).Size(11, 11)
			ui.Text(c, i18n.T("action.reset")).FontSize(11).SingleLine()
		})

		if a.kind == kindFile && a.panes[0] != nil {
			ui.Divider(c).Height(16).Margin(0, 2)
			ready := a.activeDiff().ComparisonReady() && !a.busy && !a.compareRunning && a.compareErr == nil
			prevBtn := ui.Button(c, "").Label(i18n.T("action.prevDiff")).Padding(4, 8).Gap(3).Radius(6).Disabled(!ready)
			prevBtn.OnClick(func() { a.navigateDiff(-1) })
			prevBtn.Children(func() {
				ui.Icon(c, iconChevronUp).Size(11, 11)
				ui.Text(c, i18n.T("action.prevDiff")).FontSize(11).SingleLine()
			})

			nextBtn := ui.Button(c, "").Label(i18n.T("action.nextDiff")).Padding(4, 8).Gap(3).Radius(6).Disabled(!ready)
			nextBtn.OnClick(func() { a.navigateDiff(1) })
			nextBtn.Children(func() {
				ui.Icon(c, iconChevronDown).Size(11, 11)
				ui.Text(c, i18n.T("action.nextDiff")).FontSize(11).SingleLine()
			})

			ui.Button(c, i18n.T("action.applyLeft")).Padding(4, 8).Radius(6).FontSize(11).Disabled(!ready || a.ignoreBlankLines).OnClick(func() { a.apply(0) })
			ui.Button(c, i18n.T("action.applyRight")).Padding(4, 8).Radius(6).FontSize(11).Disabled(!ready || a.ignoreBlankLines).OnClick(func() { a.apply(1) })

			findBtn := ui.Button(c, "").Label(i18n.T("action.find")).Padding(4, 8).Gap(3).Radius(6)
			findBtn.OnClick(func() { a.activeDiff().Run(editor.Find) })
			findBtn.Children(func() {
				ui.Icon(c, iconSearch).Size(11, 11)
				ui.Text(c, i18n.T("action.find")).FontSize(11).SingleLine()
			})
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

	ui.Row(c).Height(38).Padding(0, 14).Gap(6).Background(pal.cardBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		resetBtn := ui.Button(c, "").Label(i18n.T("action.reset")).Padding(4, 8).Gap(3).Radius(6).Disabled(a.busy || !a.canReset())
		resetBtn.OnClick(a.resetCurrentMode)
		resetBtn.Children(func() {
			ui.Icon(c, iconReset).Size(11, 11)
			ui.Text(c, i18n.T("action.reset")).FontSize(11).SingleLine()
		})

		clearBtn := ui.Button(c, i18n.T("action.clear")).Padding(4, 8).Radius(6).FontSize(11).Disabled(leftLen == 0 && rightLen == 0)
		clearBtn.OnClick(func() {
			leftBuf.Set("")
			rightBuf.Set("")
			a.refreshComparison()
			a.message = i18n.T("status.clearedBoth")
		})

		sampleBtn := ui.Button(c, i18n.T("action.sample")).Padding(4, 8).Radius(6).FontSize(11)
		sampleBtn.OnClick(func() {
			leftBuf.Set("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, GoCompare!\")\n\titems := []string{\"apple\", \"banana\", \"cherry\"}\n\tfor i, item := range items {\n\t\tfmt.Printf(\"%d: %s\\n\", i, item)\n\t}\n}\n")
			rightBuf.Set("package main\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nfunc main() {\n\tfmt.Println(\"Hello, GoCompare v2!\")\n\titems := []string{\"apple\", \"blueberry\", \"cherry\", \"date\"}\n\tfor i, item := range items {\n\t\tfmt.Printf(\"[%d] %s\\n\", i+1, strings.ToUpper(item))\n\t}\n}\n")
			a.refreshComparison()
			a.message = i18n.T("status.sampleLoaded")
		})

		swapBtn := ui.Button(c, "").Label(i18n.T("action.swap")).Padding(4, 8).Gap(3).Radius(6).Disabled(a.busy)
		swapBtn.OnClick(func() {
			lt := leftBuf.String()
			rt := rightBuf.String()
			leftBuf.Set(rt)
			rightBuf.Set(lt)
			a.refreshComparison()
			a.message = i18n.T("status.swappedSides")
		})
		swapBtn.Children(func() {
			ui.Icon(c, iconSwap).Size(11, 11)
			ui.Text(c, i18n.T("action.swap")).FontSize(11).SingleLine()
		})

		ui.Divider(c).Height(16).Margin(0, 2)

		diff := a.activeDiff()
		ready := diff.ComparisonReady() && !a.busy && !a.compareRunning && a.compareErr == nil
		hasHunks := ready && len(diff.Hunks()) > 0

		prevBtn := ui.Button(c, "").Label(i18n.T("action.prevDiff")).Padding(4, 8).Gap(3).Radius(6).Disabled(!hasHunks)
		prevBtn.OnClick(a.diffPrev)
		prevBtn.Children(func() {
			ui.Icon(c, iconChevronUp).Size(11, 11)
			ui.Text(c, i18n.T("action.prevDiff")).FontSize(11).SingleLine()
		})

		nextBtn := ui.Button(c, "").Label(i18n.T("action.nextDiff")).Padding(4, 8).Gap(3).Radius(6).Disabled(!hasHunks)
		nextBtn.OnClick(a.diffNext)
		nextBtn.Children(func() {
			ui.Icon(c, iconChevronDown).Size(11, 11)
			ui.Text(c, i18n.T("action.nextDiff")).FontSize(11).SingleLine()
		})

		ui.Button(c, i18n.T("action.applyLeft")).Padding(4, 8).Radius(6).FontSize(11).Disabled(!ready || a.ignoreBlankLines).OnClick(func() { a.apply(0) })
		ui.Button(c, i18n.T("action.applyRight")).Padding(4, 8).Radius(6).FontSize(11).Disabled(!ready || a.ignoreBlankLines).OnClick(func() { a.apply(1) })

		findBtn := ui.Button(c, "").Label(i18n.T("action.find")).Padding(4, 8).Gap(3).Radius(6)
		findBtn.OnClick(func() { a.activeDiff().Run(editor.Find) })
		findBtn.Children(func() {
			ui.Icon(c, iconSearch).Size(11, 11)
			ui.Text(c, i18n.T("action.find")).FontSize(11).SingleLine()
		})
	})
}

func (a *workspaceState) optionsView(c *ui.Context) {
	t := c.Theme()
	pal := paletteFor(t)
	ui.Column(c).Padding(6, 14).Gap(6).Background(pal.subtleSurface).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		ui.Row(c).Gap(8).Wrap().Children(func() {
			winBtn := ui.Button(c, "").Label(i18n.T("action.openMerge")).Padding(4, 9).Gap(4).Radius(6).Disabled(a.busy)
			winBtn.OnClick(a.openMergeWindow)
			winBtn.Children(func() {
				ui.Icon(c, iconExternal).Size(11, 11)
				ui.Text(c, i18n.T("action.openMerge")).FontSize(11).SingleLine()
			})

			if a.kind == kindDirectory {
				ui.Divider(c).Height(16)
				ui.TextInput(c.Key("dir-filter"), &a.dirFilter).Label(i18n.T("option.dirFilter")).Placeholder(i18n.T("option.dirFilterPlaceholder")).Grow(1).MinWidth(160).Disabled(a.busy)
				ui.Checkbox(c, &a.compareMode, i18n.T("option.compareMode")).Disabled(a.busy)
				ui.Checkbox(c, &a.compareTime, i18n.T("option.compareTime")).Disabled(a.busy)
				ui.Checkbox(c, &a.followLinks, i18n.T("option.followLinks")).Disabled(a.busy)
			}
			if (a.kind == kindFile && a.panes[0] != nil) || a.kind == kindScratch {
				ui.Divider(c).Height(16)
				if a.kind == kindFile {
					ui.Segmented(c, &a.mode, i18n.T("option.modeCompare"), i18n.T("option.modeEditLeft"), i18n.T("option.modeEditRight"))
				}
				ui.Checkbox(c, &a.ignoreSpace, i18n.T("option.ignoreSpace")).OnChange(a.refreshComparison)
				ui.Checkbox(c, &a.ignoreBlankLines, i18n.T("option.ignoreBlankLines")).OnChange(func() { a.setIgnoreBlankLines(a.ignoreBlankLines) })
				ui.Checkbox(c, &a.wrap, i18n.T("option.wrap"))
				ui.Checkbox(c, &a.inlineDiff, i18n.T("option.inlineDiff")).OnChange(func() { a.setInlineDiff(a.inlineDiff) })
				ui.Button(c, i18n.T("action.recompute")).Padding(4, 8).Radius(6).FontSize(11).OnClick(a.recompute)
			}
		})
		if a.kind == kindFile && a.panes[0] != nil {
			ui.Row(c).Gap(6).Children(func() {
				ui.Icon(c, iconAnchor).Size(12, 12).TextColor(pal.baseAccent)
				ui.Text(c, i18n.T("option.anchors")).FontSize(11).Bold().TextColor(t.TextMuted)
				ui.Button(c, i18n.T("option.markLeft")).Padding(3, 8).Radius(5).FontSize(11).OnClick(func() { a.markAnchorSide(editor.DiffOld) })
				ui.Button(c, i18n.T("option.markRight")).Padding(3, 8).Radius(5).FontSize(11).OnClick(func() { a.markAnchorSide(editor.DiffNew) })
				ui.Button(c, i18n.T("option.alignHere")).Padding(3, 8).Radius(5).FontSize(11).OnClick(a.commitAnchor)
				ui.Button(c, i18n.T("option.clearAnchors")).Padding(3, 8).Radius(5).FontSize(11).OnClick(a.releaseAnchors)
				if len(a.anchors) > 0 {
					statusPill(c, i18n.TArgs("option.anchorCount", len(a.anchors)), pal.baseAccent, pal.baseBg)
				}
			})
		}
	})
}

func (a *workspaceState) settingsModal(c *ui.Context, pal uiPalette) {
	t := c.Theme()
	ui.Modal(c, &a.settingsOpen, func() {
		ui.Column(c).Width(420).Padding(20, 22).Gap(14).Radius(12).Background(pal.cardBg).Border(1, pal.cardBorder).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.22)).Children(func() {
			ui.Row(c).FillWidth().Children(func() {
				ui.Row(c).Gap(6).Children(func() {
					ui.Icon(c, iconSliders).Size(16, 16).TextColor(pal.baseAccent)
					ui.Text(c, i18n.T("settings.title")).FontSize(15).Bold()
				})
				ui.Box(c).Grow(1)
				closeBtn := ui.Button(c, "✕").Padding(2, 6).Radius(4).FontSize(11)
				closeBtn.OnClick(func() { a.settingsOpen = false })
			})

			ui.Box(c).Height(1).FillWidth().Background(pal.cardBorder)

			ui.Column(c).FillWidth().Gap(6).Children(func() {
				ui.Text(c, i18n.T("settings.language")).FontSize(12).Bold().TextColor(pal.sidebarText)
				ui.Text(c, i18n.T("settings.languageHint")).FontSize(11).TextColor(t.TextMuted)
				seg := ui.Segmented(c, &a.settingsLang, i18n.T("settings.langZhCN"), i18n.T("settings.langZhTW"), i18n.T("settings.langEn"))
				seg.OnChange(func() {
					a.applySettings(a.settingsLang, a.settingsTheme)
				})
			})

			ui.Column(c).FillWidth().Gap(6).Children(func() {
				ui.Text(c, i18n.T("settings.appearance")).FontSize(12).Bold().TextColor(pal.sidebarText)
				ui.Text(c, i18n.T("settings.themeHint")).FontSize(11).TextColor(t.TextMuted)
				seg := ui.Segmented(c, &a.settingsTheme, i18n.T("settings.themeLight"), i18n.T("settings.themeDark"), i18n.T("settings.themeSystem"))
				seg.OnChange(func() {
					a.applySettings(a.settingsLang, a.settingsTheme)
				})
			})

			ui.Box(c).Height(1).FillWidth().Background(pal.cardBorder)

			ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
				doneBtn := ui.PrimaryButton(c, i18n.T("settings.close")).Padding(5, 16).Radius(6).FontSize(12)
				doneBtn.OnClick(func() { a.settingsOpen = false })
			})
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
			ui.Row(c).Grow(1).Basis(0).MinWidth(0).Height(36).Padding(0, 12).Gap(8).Background(pal.cardBg).Children(func() {
				sideBadge(c, side, pal)
				if parent != "" && parent != "." && parent != "/" {
					ui.Text(c, parent+"/").Shrink(0).FontSize(11).TextColor(t.TextMuted).SingleLine()
				}
				ui.Text(c, name).Bold().FontSize(12).Grow(1).MinWidth(0).SingleLine().Tooltip(p.file.Path)
				if isDirty {
					statusPill(c, i18n.T("pane.unsaved"), pal.diffFg, pal.diffBg)
				}
				if a.ignoreBlankLines {
					statusPill(c, i18n.T("pane.blankIgnored"), pal.baseAccent, pal.baseBg)
				}
				saveLabel := []string{i18n.T("pane.saveLeft"), i18n.T("pane.saveRight")}[side]
				saveBtn := ui.Button(c, "").Label(saveLabel).Shrink(0).Padding(3, 8).Gap(4).Radius(5).Disabled(a.busy || !isDirty)
				saveBtn.OnClick(func() { a.save(side) })
				saveBtn.Children(func() {
					ui.Icon(c, iconSave).Size(11, 11)
					ui.Text(c, saveLabel).FontSize(11).SingleLine()
				})
				ui.Button(c, i18n.T("pane.saveAs")).Shrink(0).Padding(3, 8).Radius(5).FontSize(11).Disabled(a.busy).OnClick(func() { a.saveAs(side) })
			})
		}
	})
	if a.mode == 0 {
		a.comparisonView(c, "comparison")
	} else {
		p := a.panes[a.mode-1]
		p.edit.SoftWrap = a.wrap
		p.edit.ReadOnly = a.busy
		editor.View(c.Key(a.mode), &p.edit).Grow(1).FillWidth()
	}
	ui.Row(c).Height(26).Padding(0, 12).Gap(12).Background(pal.cardBg).BorderWidth(1, 0, 0, 0).BorderColor(pal.cardBorder).Children(func() {
		for _, p := range a.panes {
			ending := "LF"
			if p.file.CRLF {
				ending = "CRLF"
			}
			ui.Row(c).Grow(1).Basis(0).MinWidth(0).Gap(6).Children(func() {
				ui.Icon(c, iconFile).Size(11, 11).TextColor(t.TextMuted)
				ui.Text(c, i18n.TArgs("pane.fileInfo", strings.ToUpper(p.file.Encoding), ending, p.buffer.LineCount())).Grow(1).FontSize(11).TextColor(t.TextMuted).SingleLine()
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
	ui.Row(c).Gap(1).Background(pal.cardBorder).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		// 左侧头部
		ui.Row(c).Grow(1).Basis(0).MinWidth(0).Height(38).Padding(0, 12).Gap(8).Background(pal.cardBg).Children(func() {
			sideBadge(c, 0, pal)
			ui.Text(c, i18n.T("pane.leftText")).Bold().FontSize(12).TextColor(pal.sidebarText).SingleLine()
			stat := i18n.TArgs("scratch.stat", leftLines, leftLen)
			ui.Text(c, stat).FontSize(11).TextColor(t.TextMuted).SingleLine()
			ui.Box(c).Grow(1)

			pasteBtn := ui.Button(c, "").Label(i18n.T("pane.pasteLeft")).Padding(3, 8).Gap(4).Radius(5).Background(pal.subtleSurface).Border(1, pal.cardBorder)
			pasteBtn.OnClick(func() {
				clip := c.Services().ReadClipboard()
				if clip != "" {
					leftBuf.Set(clip)
					a.refreshComparison()
					a.message = i18n.TArgs("scratch.pastedLeft", len(clip))
				} else {
					a.message = i18n.T("scratch.clipboardEmpty")
				}
			})
			pasteBtn.Children(func() {
				ui.Text(c, i18n.T("pane.paste")).FontSize(11).SingleLine()
			})

			copyBtn := ui.Button(c, i18n.T("pane.copy")).Padding(3, 8).Radius(5).FontSize(11).Disabled(leftLen == 0)
			copyBtn.OnClick(func() {
				c.Services().WriteClipboard(leftBuf.String())
				a.message = i18n.T("scratch.copiedLeft")
			})

			clearLeftBtn := ui.Button(c, i18n.T("pane.clear")).Padding(3, 8).Radius(5).FontSize(11).Disabled(leftLen == 0)
			clearLeftBtn.OnClick(func() {
				leftBuf.Set("")
				a.refreshComparison()
			})

			saveLeftBtn := ui.Button(c, i18n.T("pane.saveAs")).Padding(3, 8).Radius(5).FontSize(11).Disabled(leftLen == 0)
			saveLeftBtn.OnClick(func() { a.saveScratchAs(0) })
		})

		// 右侧头部
		ui.Row(c).Grow(1).Basis(0).MinWidth(0).Height(38).Padding(0, 12).Gap(8).Background(pal.cardBg).Children(func() {
			sideBadge(c, 1, pal)
			ui.Text(c, i18n.T("pane.rightText")).Bold().FontSize(12).TextColor(pal.sidebarText).SingleLine()
			stat := i18n.TArgs("scratch.stat", rightLines, rightLen)
			ui.Text(c, stat).FontSize(11).TextColor(t.TextMuted).SingleLine()
			ui.Box(c).Grow(1)

			pasteBtn := ui.Button(c, "").Label(i18n.T("pane.pasteRight")).Padding(3, 8).Gap(4).Radius(5).Background(pal.subtleSurface).Border(1, pal.cardBorder)
			pasteBtn.OnClick(func() {
				clip := c.Services().ReadClipboard()
				if clip != "" {
					rightBuf.Set(clip)
					a.refreshComparison()
					a.message = i18n.TArgs("scratch.pastedRight", len(clip))
				} else {
					a.message = i18n.T("scratch.clipboardEmpty")
				}
			})
			pasteBtn.Children(func() {
				ui.Text(c, i18n.T("pane.paste")).FontSize(11).SingleLine()
			})

			copyBtn := ui.Button(c, i18n.T("pane.copy")).Padding(3, 8).Radius(5).FontSize(11).Disabled(rightLen == 0)
			copyBtn.OnClick(func() {
				c.Services().WriteClipboard(rightBuf.String())
				a.message = i18n.T("scratch.copiedRight")
			})

			clearRightBtn := ui.Button(c, i18n.T("pane.clear")).Padding(3, 8).Radius(5).FontSize(11).Disabled(rightLen == 0)
			clearRightBtn.OnClick(func() {
				rightBuf.Set("")
				a.refreshComparison()
			})

			saveRightBtn := ui.Button(c, i18n.T("pane.saveAs")).Padding(3, 8).Radius(5).FontSize(11).Disabled(rightLen == 0)
			saveRightBtn.OnClick(func() { a.saveScratchAs(1) })
		})
	})

	if leftLen == 0 && rightLen == 0 {
		ui.Row(c).Height(32).Padding(0, 14).Gap(8).Background(pal.subtleSurface).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
			ui.Text(c, i18n.T("pane.scratchTip")).FontSize(11).TextColor(t.TextMuted).SingleLine()
		})
	}

	// 核心双栏 DiffView
	a.comparisonView(c, "scratch_comparison")
}

// comparisonView 绘制当前比较视图；忽略空行时使用独立键和只读副本，避免与可编辑视图共享控件状态。
func (a *workspaceState) comparisonView(c *ui.Context, key string) {
	diff := a.activeDiff()
	editable := !a.busy && !a.ignoreBlankLines
	if a.ignoreBlankLines {
		key += "_blank"
	}
	diff.Wrap = a.wrap
	diff.Editable = editable
	diff.OldEditable = editable
	editor.DiffView(c.Key(key), diff).Grow(1).FillWidth()
}
