package native

import (
	"context"
	"fmt"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"gocompare/internal/document"
	"os"
	"path/filepath"
	"strings"
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
		hadResult := a.panes[0] != nil || a.directoryRows != nil
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
	ui.Row(c).Fill().Background(t.Background).Children(func() {
		ui.Column(c).Width(154).FillHeight().Padding(16, 12).Gap(6).Background(ui.Hex("#1b2c43")).Children(func() {
			ui.Row(c).Gap(6).Margin(0, 0, 22).Children(func() {
				if icon := a.appIcon(); icon != nil {
					ui.Image(c, icon).Size(25, 25).Label("应用图标")
				}
				ui.Text(c, "GoCompare").FontSize(16).Bold().TextColor(ui.Hex("#f0f4fa"))
			})
			for i, label := range []string{"文件比较", "目录比较", "文本合并", "图片比较", "媒体标签", "版本与注册表"} {
				i := i
				bg := ui.Transparent
				if a.kind == i {
					bg = ui.Hex("#334e71")
				}
				ui.Button(c, label).FillWidth().Height(36).Justify(ui.Start).BorderWidth(0).Background(bg).TextColor(ui.Hex("#e0e9f6")).OnClick(func() { a.selectKind(i) })
			}
			ui.Box(c).Grow(1)
			ui.Button(c, "打开会话…").FillWidth().Disabled(a.busy).OnClick(func() { a.sessionDialog(false) })
			ui.Button(c, "保存会话…").FillWidth().Disabled(a.busy || !a.canSaveSession()).OnClick(func() { a.sessionDialog(true) })
		})
		ui.Column(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
			title := []string{"文件比较", "目录比较", "文本合并", "图片比较", "媒体标签", "版本与注册表"}[a.kind]
			ui.Row(c).Height(50).Padding(0, 18).Gap(10).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				ui.Text(c, title).FontSize(18).Bold()
				ui.Box(c).Grow(1)
				if a.kind == 0 && a.panes[0] != nil && a.directoryRoots[0] != "" {
					ui.Button(c, "返回目录").Disabled(a.busy).OnClick(func() { a.guard(func() { a.paths = a.directoryRoots; a.kind = 1; a.load() }) })
				}
				if a.kind == 2 {
					ui.Button(c, "打开合并窗口").OnClick(a.openMergeWindow)
				}
				if a.busy || a.compareRunning {
					ui.Button(c, "取消任务").OnClick(func() {
						a.stopComparison()
						a.compareErr = context.Canceled
						if a.cancel != nil {
							a.cancel()
						}
					})
				}
				ui.Button(c, "比较选项").OnClick(func() { a.optionsOpen = !a.optionsOpen })
			})
			ui.Row(c).Padding(12, 18).Gap(12).Background(t.Surface).Children(func() {
				for side := 0; side < 2; side++ {
					side := side
					ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(5).Children(func() {
						ui.Text(c, []string{"左侧", "右侧"}[side]).Bold().FontSize(12).TextColor(t.TextMuted)
						ui.Row(c).Gap(6).Children(func() {
							ui.TextInput(c.Key(side), &a.paths[side]).Grow(1).MinWidth(0).Placeholder("输入路径或浏览选择").Label([]string{"左侧路径", "右侧路径"}[side]).Disabled(a.busy).OnSubmit(func() { a.guard(a.load) })
							ui.Button(c, "浏览…").Disabled(a.busy).OnClick(func() { a.browse(side) })
						})
					})
				}
			})
			ui.Row(c).Padding(8, 18).Gap(8).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				ui.PrimaryButton(c, "开始比较").Disabled(a.busy).OnClick(func() { a.guard(a.load) })
				ui.Button(c, "交换两侧").Disabled(a.busy).OnClick(a.swapInputs)
				if a.kind == 0 && a.panes[0] != nil {
					ready := a.diff.ComparisonReady() && !a.busy && !a.compareRunning && a.compareErr == nil
					ui.Button(c, "上一差异").Disabled(!ready).OnClick(func() { a.navigateDiff(-1) })
					ui.Button(c, "下一差异").Disabled(!ready).OnClick(func() { a.navigateDiff(1) })
					ui.Button(c, "← 应用到左侧").Disabled(!ready).OnClick(func() { a.apply(0) })
					ui.Button(c, "应用到右侧 →").Disabled(!ready).OnClick(func() { a.apply(1) })
					ui.Button(c, "查找").OnClick(func() { a.diff.Run(editor.Find) })
				}
			})
			if a.optionsOpen {
				a.optionsView(c)
			}
			if a.err != "" {
				ui.Row(c).Padding(8, 18).Gap(8).Children(func() {
					ui.Text(c, "错误").Bold().TextColor(ui.Hex("#b5472e"))
					ui.Text(c, a.err).Grow(1).FontSize(12)
					ui.Button(c, "关闭提示").OnClick(func() { a.err = "" })
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
				ui.Column(c).Grow(1).FillWidth().Center().Gap(12).Children(func() {
					ui.Text(c, "选择两侧来源，查看每一处变化").FontSize(24).Bold()
					ui.Text(c, "浏览选择文件或目录，也可以粘贴路径后按 Enter。").TextColor(t.TextMuted)
					ui.Text(c, "差异对齐 · 双向编辑 · 安全保存").FontSize(12).TextColor(t.TextMuted)
				})
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
			ui.Text(c, message).FillWidth().Padding(6, 18).FontSize(11).TextColor(t.TextMuted).Background(t.Surface).SingleLine().Tooltip(message)
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
		if a.mode > 0 {
			a.save(a.mode - 1)
		} else if a.panes[0] != nil {
			a.save(int(a.diff.ActiveSide()))
		}
	})
}

func (a *workspaceState) optionsView(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Padding(8, 18).Gap(6).Background(t.Surface).Children(func() {
		ui.Button(c, "打开合并窗口").Disabled(a.busy).OnClick(a.openMergeWindow)
		if a.kind == 1 {
			ui.Row(c).Gap(8).Children(func() {
				ui.TextInput(c.Key("dir-filter"), &a.dirFilter).Label("目录过滤").Placeholder("*.go 或 nested/*.txt").Grow(1).Disabled(a.busy)
				ui.Checkbox(c, &a.compareMode, "比较权限").Disabled(a.busy)
				ui.Checkbox(c, &a.compareTime, "比较时间").Disabled(a.busy)
				ui.Checkbox(c, &a.followLinks, "跟随链接").Disabled(a.busy)
			})
		}
		if a.kind == 0 && a.panes[0] != nil {
			ui.Row(c).Gap(8).Children(func() {
				ui.Segmented(c, &a.mode, "比较", "编辑左侧", "编辑右侧")
				ui.Checkbox(c, &a.ignoreSpace, "忽略空白").OnChange(a.recompute)
				ui.Checkbox(c, &a.wrap, "自动换行")
				ui.Button(c, "重新计算").OnClick(a.recompute)
			})
			ui.Row(c).Gap(8).Children(func() {
				ui.Button(c, "标记左侧").OnClick(func() { a.markAnchorSide(editor.DiffOld) })
				ui.Button(c, "标记右侧").OnClick(func() { a.markAnchorSide(editor.DiffNew) })
				ui.Button(c, "在此对齐").OnClick(a.commitAnchor)
				ui.Button(c, "解除对齐").OnClick(a.releaseAnchors)
			})
		}
	})
}
func (a *workspaceState) textView(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Gap(1).Background(t.Border).Children(func() {
		for side, p := range a.panes {
			side, p := side, p
			ui.Row(c).Grow(1).Basis(0).MinWidth(0).Padding(8, 12).Gap(8).Background(t.Surface).Children(func() {
				name := filepath.Base(p.file.Path)
				if p.file.Missing() {
					name += "（不存在）"
				}
				if p.buffer.Modified() {
					name += " •"
				}
				ui.Text(c, name).Bold().Grow(1).SingleLine().Tooltip(p.file.Path)
				ui.Button(c, []string{"保存左侧", "保存右侧"}[side]).Disabled(a.busy || !p.buffer.Modified()).OnClick(func() { a.save(side) })
				ui.Button(c, "另存为…").Disabled(a.busy).OnClick(func() { a.saveAs(side) })
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
	ui.Row(c).Padding(5, 14).Gap(12).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
		for _, p := range a.panes {
			ending := "LF"
			if p.file.CRLF {
				ending = "CRLF"
			}
			ui.Text(c, fmt.Sprintf("%s / %s · %d 行", strings.ToUpper(p.file.Encoding), ending, p.buffer.LineCount())).Grow(1).FontSize(11).TextColor(t.TextMuted)
		}
	})
}
