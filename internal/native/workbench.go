package native

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"gocompare/internal/document"
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
		hadResult := a.panes[0] != nil || a.directoryRows != nil || a.mergeResult != nil || a.pictureResult != nil || a.mediaReady || a.infoReady
		a.stopComparison()
		a.compareRequested = false
		if hadResult {
			a.paths = [2]string{}
			if kind == 1 && a.panes[0] != nil {
				a.paths = [2]string{filepath.Dir(a.panes[0].file.Path), filepath.Dir(a.panes[1].file.Path)}
			}
		}
		a.kind = kind
		a.panes = [2]*pane{}
		a.directoryRows = nil
		a.syncPlan = nil
		a.clearMerge()
		a.clearPictures()
		a.clearMedia()
		a.clearVersion()
		a.allowMissing = false
		a.directoryRoots = [2]string{}
		a.err = ""
		a.message = "选择左右来源，然后开始比较"
	})
}

func (a *workspaceState) navigateDiff(step int) {
	if a.panes[0] == nil || a.compareRunning || a.compareErr != nil || !a.diff.ComparisonReady() {
		return
	}
	n := len(a.diff.Hunks())
	if n == 0 {
		return
	}
	i, _ := a.diff.CurrentHunk()
	if i < 0 {
		if step > 0 {
			i = 0
		} else {
			i = n - 1
		}
	} else {
		i = (i + step + n) % n
	}
	a.diff.GoToHunk(i)
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
	modeLabels := []string{"文件比较", "目录比较", "文本合并", "图片比较", "媒体标签", "版本与注册表"}

	ui.Row(c).Fill().Background(pal.appBg).Children(func() {
		// 左侧侧边栏（深度融合明暗主题）
		ui.Column(c).Width(160).FillHeight().Padding(12, 8).Gap(3).Background(pal.sidebarBg).BorderWidth(0, 1, 0, 0).BorderColor(pal.sidebarBorder).Children(func() {
			ui.Row(c).Padding(4, 6).Gap(8).Margin(0, 0, 12).Children(func() {
				if icon := a.appIcon(); icon != nil {
					ui.Box(c).Size(26, 26).Center().Radius(6).Background(pal.cardBg).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.08)).Children(func() {
						ui.Image(c, icon).Size(20, 20).Label("应用图标")
					})
				}
				ui.Column(c).Gap(1).Children(func() {
					ui.Text(c, "GoCompare").FontSize(14).Bold().TextColor(pal.sidebarText).SingleLine()
					ui.Text(c, "Diff Workbench").FontSize(9).TextColor(pal.sidebarMuted).SingleLine()
				})
			})

			ui.Text(c, "模式").Padding(2, 8).FontSize(10).Bold().TextColor(pal.sidebarMuted)
			for i, label := range modeLabels {
				i, label := i, label
				active := a.kind == i
				bg := ui.Transparent
				fg := pal.sidebarMuted
				btn := ui.Button(c, "").Label(label).FillWidth().Height(34).Padding(0, 8).Gap(7).Radius(6).Justify(ui.Start).BorderWidth(0).Background(bg)
				if active {
					btn.Background(pal.sidebarItemActive).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.06))
					fg = pal.sidebarText
				}
				btn.TextColor(fg)
				btn.OnClick(func() { a.selectKind(i) })
				btn.Children(func() {
					iconSvg := modeIcons[i]
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
			ui.Text(c, "会话").Padding(2, 8).FontSize(10).Bold().TextColor(pal.sidebarMuted)

			openBtn := ui.Button(c, "").Label("打开会话…").FillWidth().Height(30).Padding(0, 8).Gap(6).Radius(6).Justify(ui.Start).Background(pal.sidebarBtnBg).Border(1, pal.sidebarBtnBorder).TextColor(pal.sidebarText).Disabled(a.busy)
			openBtn.OnClick(func() { a.sessionDialog(false) })
			openBtn.Children(func() {
				ui.Icon(c, iconFolderOpen).Size(12, 12).TextColor(pal.sidebarMuted)
				ui.Text(c, "打开会话…").FontSize(11).SingleLine()
			})

			saveBtn := ui.Button(c, "").Label("保存会话…").FillWidth().Height(30).Padding(0, 8).Gap(6).Radius(6).Justify(ui.Start).Background(pal.sidebarBtnBg).Border(1, pal.sidebarBtnBorder).TextColor(pal.sidebarText).Disabled(a.busy || !a.canSaveSession())
			saveBtn.OnClick(func() { a.sessionDialog(true) })
			saveBtn.Children(func() {
				ui.Icon(c, iconSave).Size(12, 12).TextColor(pal.sidebarMuted)
				ui.Text(c, "保存会话…").FontSize(11).SingleLine()
			})
		})

		// 右侧主工作区
		ui.Column(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
			title := modeLabels[a.kind]
			// 顶部标题栏（沉浸式对齐设计）
			ui.Row(c).Height(44).Padding(0, 16).Gap(8).Background(pal.headerBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
				ui.Row(c).Size(24, 24).Center().Radius(6).Background(pal.baseBg).Children(func() {
					ui.Icon(c, modeIcons[a.kind]).Size(13, 13).TextColor(pal.baseAccent)
				})
				ui.Text(c, title).FontSize(15).Bold().SingleLine()
				a.renderHeaderBadge(c, pal)
				ui.Box(c).Grow(1)

				if a.kind == 0 && a.panes[0] != nil && a.directoryRoots[0] != "" {
					backBtn := ui.Button(c, "").Label("返回目录").Padding(4, 9).Gap(4).Radius(6).Disabled(a.busy)
					backBtn.OnClick(func() { a.guard(func() { a.paths = a.directoryRoots; a.kind = 1; a.load() }) })
					backBtn.Children(func() {
						ui.Icon(c, iconFolderOpen).Size(12, 12)
						ui.Text(c, "返回目录").FontSize(11).SingleLine()
					})
				}
				if a.kind == 2 {
					winBtn := ui.Button(c, "").Label("打开合并窗口").Padding(4, 9).Gap(4).Radius(6)
					winBtn.OnClick(a.openMergeWindow)
					winBtn.Children(func() {
						ui.Icon(c, iconExternal).Size(12, 12)
						ui.Text(c, "打开合并窗口").FontSize(11).SingleLine()
					})
				}
				if a.busy || a.compareRunning {
					ui.Spinner(c).Size(14, 14)
					cancelBtn := ui.Button(c, "取消任务").Padding(4, 8).Radius(6).Background(pal.dangerBg).Border(1, pal.dangerFg.Alpha(0.25)).TextColor(pal.dangerFg).FontSize(11)
					cancelBtn.OnClick(func() {
						a.stopComparison()
						a.compareErr = context.Canceled
						if a.cancel != nil {
							a.cancel()
						}
					})
				}
				optBtn := ui.Button(c, "").Label("比较选项").Padding(4, 9).Gap(4).Radius(6)
				if a.optionsOpen {
					optBtn.Background(pal.baseBg).Border(1, pal.baseAccent.Alpha(0.35)).TextColor(pal.baseAccent)
				}
				optBtn.OnClick(func() { a.optionsOpen = !a.optionsOpen })
				optBtn.Children(func() {
					ui.Icon(c, iconSliders).Size(12, 12)
					ui.Text(c, "比较选项").FontSize(11).SingleLine()
				})
			})

			// 双侧来源路径胶囊区（极简紧凑）
			ui.Row(c).Padding(6, 14).Gap(8).Background(pal.subtleSurface).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
				for side := 0; side < 2; side++ {
					side := side
					ui.Row(c).Grow(1).Basis(0).MinWidth(0).Height(34).Padding(0, 8).Gap(6).Radius(7).Background(pal.cardBg).Border(1, pal.cardBorder).Children(func() {
						sideBadge(c, side, pal)
						ui.TextInput(c.Key(side), &a.paths[side]).Grow(1).MinWidth(0).Placeholder("输入路径或浏览选择…").Label([]string{"左侧路径", "右侧路径"}[side]).Disabled(a.busy).OnSubmit(func() { a.guard(a.load) })
						ui.Button(c, "浏览…").Shrink(0).Padding(3, 8).Radius(5).FontSize(11).Disabled(a.busy).OnClick(func() { a.browse(side) })
					})
				}
			})

			// 操作命令工具栏（保持 900px 视口无缝容纳）
			ui.Row(c).Height(38).Padding(0, 14).Gap(6).Background(pal.headerBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
				startBtn := ui.PrimaryButton(c, "").Label("开始比较").Padding(4, 10).Gap(4).Radius(6).Disabled(a.busy)
				startBtn.OnClick(func() { a.guard(a.load) })
				startBtn.Children(func() {
					ui.Icon(c, iconCompare).Size(11, 11)
					ui.Text(c, "开始比较").FontSize(11).Bold().SingleLine()
				})

				swapBtn := ui.Button(c, "").Label("交换两侧").Padding(4, 9).Gap(4).Radius(6).Disabled(a.busy)
				swapBtn.OnClick(a.swapInputs)
				swapBtn.Children(func() {
					ui.Icon(c, iconSwap).Size(11, 11)
					ui.Text(c, "交换两侧").FontSize(11).SingleLine()
				})

				if a.kind == 0 && a.panes[0] != nil {
					ui.Divider(c).Height(16).Margin(0, 2)
					ready := a.diff.ComparisonReady() && !a.busy && !a.compareRunning && a.compareErr == nil
					prevBtn := ui.Button(c, "").Label("上一差异").Padding(4, 8).Gap(3).Radius(6).Disabled(!ready)
					prevBtn.OnClick(func() { a.navigateDiff(-1) })
					prevBtn.Children(func() {
						ui.Icon(c, iconChevronUp).Size(11, 11)
						ui.Text(c, "上一差异").FontSize(11).SingleLine()
					})

					nextBtn := ui.Button(c, "").Label("下一差异").Padding(4, 8).Gap(3).Radius(6).Disabled(!ready)
					nextBtn.OnClick(func() { a.navigateDiff(1) })
					nextBtn.Children(func() {
						ui.Icon(c, iconChevronDown).Size(11, 11)
						ui.Text(c, "下一差异").FontSize(11).SingleLine()
					})

					ui.Button(c, "← 应用到左侧").Padding(4, 8).Radius(6).FontSize(11).Disabled(!ready).OnClick(func() { a.apply(0) })
					ui.Button(c, "应用到右侧 →").Padding(4, 8).Radius(6).FontSize(11).Disabled(!ready).OnClick(func() { a.apply(1) })

					findBtn := ui.Button(c, "").Label("查找").Padding(4, 8).Gap(3).Radius(6)
					findBtn.OnClick(func() { a.diff.Run(editor.Find) })
					findBtn.Children(func() {
						ui.Icon(c, iconSearch).Size(11, 11)
						ui.Text(c, "查找").FontSize(11).SingleLine()
					})
				} else {
					ui.Box(c).Grow(1)
					a.renderToolbarMeta(c, pal)
				}
			})

			if a.optionsOpen {
				a.optionsView(c)
			}
			if a.err != "" {
				ui.Row(c).Padding(6, 16).Gap(8).Background(pal.dangerBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.dangerFg.Alpha(0.2)).Children(func() {
					ui.Icon(c, iconAlert).Size(14, 14).TextColor(pal.dangerFg)
					ui.Text(c, "错误").Bold().FontSize(11).TextColor(pal.dangerFg)
					ui.Text(c, a.err).Grow(1).MinWidth(0).FontSize(11).TextColor(pal.dangerFg)
					ui.Button(c, "关闭提示").Padding(2, 8).Radius(4).FontSize(10).OnClick(func() { a.err = "" })
				})
			}

			switch {
			case a.kind == 2:
				a.mergeView(c)
			case a.kind == 3:
				a.pictureView(c)
			case a.kind == 4:
				a.mediaView(c)
			case a.kind == 5:
				a.versionView(c)
			case a.kind == 1 && a.directoryRows != nil:
				a.directoryView(c)
			case a.kind == 0 && a.panes[0] != nil:
				a.textView(c)
			default:
				a.welcomeView(c, pal)
			}

			message := a.message
			if a.compareRunning {
				message = "正在后台计算差异，可继续编辑…"
			} else if a.compareErr != nil {
				message = "差异计算已停止，请重新计算"
			}
			if a.busy {
				message = "正在处理…"
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
				ui.Text(c, "⌘S 保存").FontSize(10).TextColor(t.TextMuted)
			})
		})
	})

	if a.confirmSync {
		choice := ui.AlertDialog(c, &a.confirmSync, "执行同步计划？", "预览中的复制和删除将修改目标目录；覆盖和删除的文件会先备份。", "取消", "执行")
		if choice == 1 {
			a.executeSync()
		}
	}
	if a.confirm {
		choice := ui.AlertDialog(c, &a.confirm, "放弃未保存的修改？", "可取消后先保存左右文件。", "取消", "放弃修改")
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
		if a.kind == 2 && a.mergeBuffer != nil && a.mergeBuffer.Modified() {
			a.saveMerge()
			return
		}
		if a.kind != 0 || a.panes[0] == nil {
			return
		}
		side := int(a.diff.ActiveSide())
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
	case 1:
		statusPill(c, "文件树与属性比对", pal.sameFg, pal.sameBg)
		if strings.TrimSpace(a.dirFilter) != "" {
			statusPill(c, "过滤 "+a.dirFilter, pal.baseAccent, pal.baseBg)
		}
		if a.compareMode {
			statusPill(c, "权限", pal.baseAccent, pal.baseBg)
		}
		if a.compareTime {
			statusPill(c, "时间", pal.baseAccent, pal.baseBg)
		}
	case 2:
		if strings.TrimSpace(a.mergeBase) != "" {
			statusPill(c, "三方合并", pal.baseAccent, pal.baseBg)
		} else {
			statusPill(c, "两方合并", pal.sameFg, pal.sameBg)
		}
	case 3:
		statusPill(c, "PNG · JPEG · GIF 像素级", pal.sameFg, pal.sameBg)
	case 4:
		statusPill(c, "ID3v2 元数据标签", pal.sameFg, pal.sameBg)
	case 5:
		statusPill(c, "PE FileVersion 与注册表", pal.sameFg, pal.sameBg)
	}
}

func (a *workspaceState) renderHeaderBadge(c *ui.Context, pal uiPalette) {
	switch {
	case a.kind == 0 && a.panes[0] != nil && a.diff.ComparisonReady():
		n := len(a.diff.Hunks())
		if n == 0 {
			statusPill(c, "相同", pal.successFg, pal.successBg)
		} else {
			statusPill(c, fmt.Sprintf("%d 处差异", n), pal.diffFg, pal.diffBg)
		}
	case a.kind == 1 && a.directoryRows != nil:
		statusPill(c, fmt.Sprintf("%d 项", len(a.directoryRows)), pal.baseAccent, pal.baseBg)
	case a.kind == 2 && a.mergeResult != nil:
		n := a.mergeResult.ConflictCount()
		if n == 0 {
			statusPill(c, "无冲突", pal.successFg, pal.successBg)
		} else {
			statusPill(c, fmt.Sprintf("%d 处冲突", n), pal.dangerFg, pal.dangerBg)
		}
	}
}

func (a *workspaceState) welcomeView(c *ui.Context, pal uiPalette) {
	t := c.Theme()
	ui.Column(c).Grow(1).FillWidth().Center().Padding(24).Children(func() {
		ui.Column(c).MaxWidth(600).Padding(24, 28).Gap(10).Radius(12).Background(pal.cardBg).Border(1, pal.cardBorder).Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.04)).Center().Children(func() {
			ui.Row(c).Gap(6).Children(func() {
				statusPill(c, "差异对齐 · 双向编辑 · 安全保存", pal.baseAccent, pal.baseBg)
			})
			ui.Text(c, "选择两侧来源，查看每一处变化").FontSize(20).Bold()
			ui.Text(c, "浏览选择文件或目录，也可以粘贴路径后按 Enter。").FontSize(12).TextColor(t.TextMuted)

			ui.Row(c).FillWidth().Gap(8).Margin(8, 0, 0).Children(func() {
				cards := []struct {
					kind int
					head string
					desc string
				}{
					{0, "双栏代码比较", "行内词级对比与实时双向编辑"},
					{1, "目录结构比对", "递归比对与带备份安全同步"},
					{2, "文本合并解决", "三列行对齐与一键采纳冲突"},
				}
				for _, card := range cards {
					card := card
					box := ui.Column(c).Grow(1).Basis(0).Padding(10, 12).Gap(4).Radius(8).Background(pal.subtleSurface).Border(1, pal.cardBorder).Cursor(ui.CursorPointer)
					box.OnClick(func() { a.selectKind(card.kind) })
					box.Children(func() {
						ui.Row(c).Gap(5).Children(func() {
							ui.Icon(c, modeIcons[card.kind]).Size(13, 13).TextColor(pal.baseAccent)
							ui.Text(c, card.head).FontSize(12).Bold().SingleLine()
						})
						ui.Text(c, card.desc).FontSize(10).TextColor(t.TextMuted)
					})
				}
			})
		})
	})
}

func (a *workspaceState) optionsView(c *ui.Context) {
	t := c.Theme()
	pal := paletteFor(t)
	ui.Column(c).Padding(6, 14).Gap(6).Background(pal.subtleSurface).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		ui.Row(c).Gap(8).Wrap().Children(func() {
			winBtn := ui.Button(c, "").Label("打开合并窗口").Padding(4, 9).Gap(4).Radius(6).Disabled(a.busy)
			winBtn.OnClick(a.openMergeWindow)
			winBtn.Children(func() {
				ui.Icon(c, iconExternal).Size(11, 11)
				ui.Text(c, "打开合并窗口").FontSize(11).SingleLine()
			})

			if a.kind == 1 {
				ui.Divider(c).Height(16)
				ui.TextInput(c.Key("dir-filter"), &a.dirFilter).Label("目录过滤").Placeholder("*.go 或 nested/*.txt").Grow(1).MinWidth(160).Disabled(a.busy)
				ui.Checkbox(c, &a.compareMode, "比较权限").Disabled(a.busy)
				ui.Checkbox(c, &a.compareTime, "比较时间").Disabled(a.busy)
				ui.Checkbox(c, &a.followLinks, "跟随链接").Disabled(a.busy)
			}
			if a.kind == 0 && a.panes[0] != nil {
				ui.Divider(c).Height(16)
				ui.Segmented(c, &a.mode, "比较", "编辑左侧", "编辑右侧")
				ui.Checkbox(c, &a.ignoreSpace, "忽略空白").OnChange(a.recompute)
				ui.Checkbox(c, &a.wrap, "自动换行")
				ui.Button(c, "重新计算").Padding(4, 8).Radius(6).FontSize(11).OnClick(a.recompute)
			}
		})
		if a.kind == 0 && a.panes[0] != nil {
			ui.Row(c).Gap(6).Children(func() {
				ui.Icon(c, iconAnchor).Size(12, 12).TextColor(pal.baseAccent)
				ui.Text(c, "行对齐锚点").FontSize(11).Bold().TextColor(t.TextMuted)
				ui.Button(c, "标记左侧").Padding(3, 8).Radius(5).FontSize(11).OnClick(func() { a.markAnchorSide(editor.DiffOld) })
				ui.Button(c, "标记右侧").Padding(3, 8).Radius(5).FontSize(11).OnClick(func() { a.markAnchorSide(editor.DiffNew) })
				ui.Button(c, "在此对齐").Padding(3, 8).Radius(5).FontSize(11).OnClick(a.commitAnchor)
				ui.Button(c, "解除对齐").Padding(3, 8).Radius(5).FontSize(11).OnClick(a.releaseAnchors)
				if len(a.anchors) > 0 {
					statusPill(c, fmt.Sprintf("%d 个锚点", len(a.anchors)), pal.baseAccent, pal.baseBg)
				}
			})
		}
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
				name += "（不存在）"
			}
			if p.buffer.Modified() {
				name += " •"
			}
			parent := filepath.Base(filepath.Dir(p.file.Path))
			ui.Row(c).Grow(1).Basis(0).MinWidth(0).Height(36).Padding(0, 12).Gap(8).Background(pal.cardBg).Children(func() {
				sideBadge(c, side, pal)
				if parent != "" && parent != "." && parent != "/" {
					ui.Text(c, parent+"/").Shrink(0).FontSize(11).TextColor(t.TextMuted).SingleLine()
				}
				ui.Text(c, name).Bold().FontSize(12).Grow(1).MinWidth(0).SingleLine().Tooltip(p.file.Path)
				if p.buffer.Modified() {
					statusPill(c, "未保存", pal.diffFg, pal.diffBg)
				}
				saveLabel := []string{"保存左侧", "保存右侧"}[side]
				saveBtn := ui.Button(c, "").Label(saveLabel).Shrink(0).Padding(3, 8).Gap(4).Radius(5).Disabled(a.busy || !p.buffer.Modified())
				saveBtn.OnClick(func() { a.save(side) })
				saveBtn.Children(func() {
					ui.Icon(c, iconSave).Size(11, 11)
					ui.Text(c, saveLabel).FontSize(11).SingleLine()
				})
				ui.Button(c, "另存为…").Shrink(0).Padding(3, 8).Radius(5).FontSize(11).Disabled(a.busy).OnClick(func() { a.saveAs(side) })
			})
		}
	})
	if a.mode == 0 {
		a.diff.Wrap = a.wrap
		a.diff.Editable = !a.busy
		a.diff.OldEditable = !a.busy
		editor.DiffView(c.Key("comparison"), &a.diff).Grow(1).FillWidth()
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
				ui.Text(c, fmt.Sprintf("%s / %s · %d 行", strings.ToUpper(p.file.Encoding), ending, p.buffer.LineCount())).Grow(1).FontSize(11).TextColor(t.TextMuted).SingleLine()
				if lang := p.buffer.Language(); lang != nil && lang.Name != "" {
					statusPill(c, lang.Name, pal.sameFg, pal.sameBg)
				}
			})
		}
	})
}
