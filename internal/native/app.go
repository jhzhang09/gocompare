// Package native 实现 GoCompare 的 MyGo 原生工作区。
package native

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"gocompare/internal/brand"
	"gocompare/internal/compare"
	"gocompare/internal/document"
	"gocompare/internal/integrate"
	"gocompare/internal/media"
	"gocompare/internal/merge"
	"gocompare/internal/operations"
	"gocompare/internal/textcompare"
)

type pane struct {
	file   *document.File
	buffer *editor.Buffer
	edit   editor.State
}

func newPane(f *document.File) *pane {
	b := editor.NewBuffer(f.Text)
	b.SetLanguage(editor.LanguageFor(f.Path))
	return &pane{file: f, buffer: b, edit: editor.State{Buffer: b, Label: filepath.Base(f.Path)}}
}

type workspaceState struct {
	win             *mygo.Window
	icon            *ui.Bitmap
	backupDir       string
	panes           [2]*pane
	paths           [2]string
	kind            int
	directoryRoots  [2]string
	directoryRows   []compare.Entry
	directoryList   ui.ListState
	onlyDifferences bool
	dirFilter       string
	compareMode     bool
	compareTime     bool
	followLinks     bool
	scanSeen        []string
	scanPath        string
	syncChoice      string
	syncPlan        *operations.Plan

	confirmSync    bool
	lastJournal    string
	syncRolledBack bool
	directoryNote  string

	mergeBase     string
	mergeOutput   string
	mergeResult   *merge.Result
	mergeRows     []merge.AlignRow
	mergeAlign    ui.ListState
	mergeBuffer   *editor.Buffer
	mergeEdit     editor.State
	mergeLeft     *document.File
	mergeConflict int
	mergeWindows  []*workspaceState
	detached      bool

	pictureMode   int
	pictureTol    uint8
	pictureGen    uint64
	pictureLeft   image.Image
	pictureRight  image.Image
	pictureResult *image.RGBA
	pictureBitmap *ui.Bitmap
	pictureNote   string

	mediaGen   uint64
	mediaLeft  media.Tags
	mediaRight media.Tags
	mediaReady bool

	infoGen   uint64
	infoKind  string
	infoLeft  string
	infoRight string
	infoDiffs []string
	infoReady bool

	compareGeneration uint64
	compareRequested  bool
	compareBuffers    [2]*editor.Buffer
	compareVersions   [2]uint64
	compareRunning    bool
	compareCancel     context.CancelFunc
	compareErr        error

	compareLaunch  func(func())
	compareInline  editor.DiffInline
	compareAnchors []textcompare.Anchor

	anchors        []textcompare.Anchor
	anchorLeft     int
	anchorRight    int
	anchorLeftSet  bool
	anchorRightSet bool

	diff    editor.DiffState
	mode    int
	wrap    bool
	busy    bool
	message string
	err     string
	pending func()
	confirm bool
	cancel  context.CancelFunc
}

// Run 启动原生工作区；支持零参数或两个普通文件路径。
func Run(args []string) error {
	version, err := brand.Framework()
	if err != nil {
		return err
	}
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Printf("GoCompare (%s)\nUsage: gocompare [left-file right-file] | --directories left-dir right-dir\n", version)
		return nil
	}
	directory := len(args) == 3 && args[0] == "--directories"
	if directory {
		args = args[1:]
	}
	if len(args) != 0 && len(args) != 2 {
		return fmt.Errorf("expected zero or two file paths")
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	if custom := os.Getenv("GOCOMPARE_DATA_DIR"); custom != "" {
		configDir = custom
	}
	mygo.App.SetName("GoCompare")
	mygo.App.SetVersion(version)
	var iconErr error
	mygo.App.WhenReady(func() {
		// darwin 的 Window.SetIcon 不改变图标。Dock.SetIcon 才会把应用图标换成这张 PNG。
		if iconErr = mygo.App.Dock.SetIcon(brand.IconPNG()); iconErr != nil {
			mygo.App.Quit()
			return
		}
		a := &workspaceState{message: "选择两个文件开始比较", backupDir: filepath.Join(configDir, "GoCompare", "backups")}
		a.win = mygo.NewWindow(mygo.WindowOptions{Title: "GoCompare", Width: 1380, Height: 880, MinWidth: 900, MinHeight: 600, Content: ui.View(a.view)})
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
			a.stopComparison()
			if a.cancel != nil {
				a.cancel()
			}
		})
		if len(args) == 2 {
			a.paths = [2]string{args[0], args[1]}
			if directory {
				a.kind = 1
			}
			a.load()
		}
	})
	if err = mygo.App.Run(); err != nil {
		return err
	}
	return iconErr
}

func (a *workspaceState) appIcon() *ui.Bitmap {
	if a.icon != nil {
		return a.icon
	}
	icon, err := ui.DecodeBitmap(brand.IconPNG())
	if err != nil {
		if a.err == "" {
			a.err = err.Error()
		}
		return nil
	}
	a.icon = icon
	return icon
}

func (a *workspaceState) frameworkVersion() string {
	version, err := brand.Framework()
	if err != nil {
		if a.err == "" {
			a.err = err.Error()
		}
		return ""
	}
	return version
}

func (a *workspaceState) dirty() bool {
	for _, p := range a.panes {
		if p != nil && p.buffer.Modified() {
			return true
		}
	}
	return false
}
func (a *workspaceState) guard(fn func()) {
	if a.dirty() {
		a.pending = fn
		a.confirm = true
		return
	}
	fn()
}
func (a *workspaceState) publish(fn func()) {
	if a.win == nil {
		fn()
	} else {
		a.win.Update(fn)
	}
}
func (a *workspaceState) job(fn func(context.Context)) {
	a.busy = true
	a.err = ""
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	run := func() { defer cancel(); fn(ctx) }
	if a.win == nil {
		run()
	} else {
		go run()
	}
}
func (a *workspaceState) load() {
	if a.busy {
		return
	}
	paths := a.paths
	if strings.TrimSpace(paths[0]) == "" || strings.TrimSpace(paths[1]) == "" {
		a.err = "请先选择左右两个文件"
		return
	}
	if a.kind == 1 {
		a.loadDirectory(paths)
		return
	}
	if a.kind == 2 {
		a.loadMerge()
		return
	}
	if a.kind == 3 {
		a.loadPictures()
		return
	}
	if a.kind == 4 {
		a.loadMedia()
		return
	}
	if a.kind == 5 {
		a.loadVersion()
		return
	}
	a.job(func(ctx context.Context) {
		left, err := document.ReadDetected(ctx, paths[0])
		var right *document.File
		if err == nil {
			right, err = document.ReadDetected(ctx, paths[1])
		}
		a.publish(func() {
			a.busy = false
			if err != nil {
				a.err = err.Error()
				return
			}
			a.install(left, right)
		})
	})
}
func (a *workspaceState) install(left, right *document.File) {
	a.panes = [2]*pane{newPane(left), newPane(right)}
	a.kind = 0
	a.directoryRows = nil
	a.paths = [2]string{left.Path, right.Path}
	a.diff = editor.DiffState{Old: a.panes[0].buffer, New: a.panes[1].buffer, ExpandUnchanged: true, Editable: true, OldEditable: true, ExternalComparison: true}
	a.anchors = nil
	a.anchorLeftSet, a.anchorRightSet = false, false
	a.stopComparison()
	a.compareRequested = false
	a.refreshComparison()
	a.mode = 0
	a.message = "已加载 · 左右内容均可直接编辑，保存后写入文件"
	a.updateTitle()
}
func (a *workspaceState) browse(side int) {
	if a.busy {
		return
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: []string{"选择左侧来源", "选择右侧来源"}[side], Directory: a.kind == 1})
	if err != nil {
		a.err = err.Error()
		return
	}
	if len(paths) > 0 {
		a.paths[side] = paths[0]
	}
}
func (a *workspaceState) save(side int) {
	if a.busy || a.panes[side] == nil {
		return
	}
	p := a.panes[side]
	text := p.buffer.String()
	version := p.buffer.Version()
	file := p.file
	a.job(func(ctx context.Context) {
		var f *document.File
		var err error
		if a.backupDir != "" {
			_, err = file.Backup(ctx, a.backupDir)
		}
		if err == nil {
			f, err = file.Save(ctx, text)
		}
		a.publish(func() {
			a.busy = false
			if err != nil {
				a.err = err.Error()
				return
			}
			p.file = f
			if p.buffer.Version() == version {
				p.buffer.MarkSaved()
			}
			a.message = "已保存 " + filepath.Base(f.Path)
		})
	})
}
func (a *workspaceState) apply(side int) {
	if a.busy || a.panes[0] == nil || !a.diff.ComparisonReady() {
		return
	}
	hunks := a.diff.Hunks()
	i, inside := a.diff.CurrentHunk()
	if !inside || i < 0 || i >= len(hunks) {
		a.message = "先在比较视图中选择一个差异块"
		return
	}
	// Revert 将左侧块复制到右侧；Accept 将右侧块复制到左侧，仅改变内存缓冲区。
	if side == 1 {
		a.diff.Revert(i)
	} else {
		a.diff.Accept(i)
	}
	a.message = "差异块已应用，保存后写入文件"
}

func (a *workspaceState) view(c *ui.Context) {
	a.refreshComparison()
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Row(c).Padding(12, 20).Gap(12).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			if icon := a.appIcon(); icon != nil {
				ui.Image(c, icon).Width(28).Height(28).Label("应用图标")
			}
			ui.Text(c, "GoCompare").FontSize(20).Bold()
			ui.Text(c, "MyGo "+a.frameworkVersion()).FontSize(12).TextColor(t.TextMuted)
			ui.Segmented(c, &a.kind, "文件比较", "目录比较", "文本合并", "图片比较", "媒体标签", "版本与注册表").Disabled(a.busy)
			ui.Button(c, "打开合并窗口").Disabled(a.busy).OnClick(a.openMergeWindow)
			ui.Box(c).Grow(1)
			ui.Button(c, "取消任务").Disabled(!a.busy && !a.compareRunning).OnClick(func() {
				a.stopComparison()
				a.compareErr = context.Canceled
				if a.cancel != nil {
					a.cancel()
				}
			})
			ui.Button(c, "打开会话…").Disabled(a.busy).OnClick(func() { a.sessionDialog(false) })
			ui.Button(c, "保存会话…").Disabled(a.busy || !a.canSaveSession()).OnClick(func() { a.sessionDialog(true) })
			for _, action := range integrate.PlatformActions() {
				ui.Button(c, action).Disabled(true)
			}
		})
		ui.Row(c).Padding(12, 20).Gap(12).Children(func() {
			for side := 0; side < 2; side++ {
				side := side
				ui.Column(c).Grow(1).MinWidth(0).Gap(6).Children(func() {
					ui.Text(c, []string{"左侧来源", "右侧来源"}[side]).FontSize(12).Bold()
					ui.Row(c).Gap(6).Children(func() {
						ui.TextInput(c.Key(side), &a.paths[side]).Grow(1).Disabled(a.busy).Label([]string{"左侧路径", "右侧路径"}[side])
						ui.Button(c, "浏览…").Disabled(a.busy).OnClick(func() { a.browse(side) })
					})
				})
			}
		})
		ui.Row(c).Padding(8, 20).Gap(8).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.PrimaryButton(c, "开始比较").Disabled(a.busy).OnClick(func() { a.guard(a.load) })
			if a.kind == 1 {
				ui.TextInput(c.Key("dir-filter"), &a.dirFilter).Label("目录过滤").Placeholder("nested/*.txt").MinWidth(160).Disabled(a.busy)
				ui.Checkbox(c, &a.compareMode, "比较权限").Disabled(a.busy)
				ui.Checkbox(c, &a.compareTime, "比较时间").Disabled(a.busy)
				ui.Checkbox(c, &a.followLinks, "跟随链接").Disabled(a.busy)
			}
			if a.kind == 0 && a.panes[0] != nil {
				if a.directoryRoots[0] != "" {
					ui.Button(c, "返回目录").Disabled(a.busy).OnClick(func() { a.guard(func() { a.paths = a.directoryRoots; a.kind = 1; a.load() }) })
				}
				ui.Segmented(c, &a.mode, "比较", "编辑左侧", "编辑右侧").Disabled(a.busy)
				ui.Button(c, "重新计算").Disabled(a.busy).OnClick(a.recompute)
				ui.Button(c, "忽略空白").Disabled(a.busy).OnClick(a.compareIgnoringSpace)
				ui.Button(c, "标记左侧").Disabled(a.busy || a.mode != 0).OnClick(func() { a.markAnchorSide(editor.DiffOld) })
				ui.Button(c, "标记右侧").Disabled(a.busy || a.mode != 0).OnClick(func() { a.markAnchorSide(editor.DiffNew) })
				ui.Button(c, "在此对齐").Disabled(a.busy || a.mode != 0).OnClick(a.commitAnchor)
				ui.Button(c, "解除对齐").Disabled(a.busy || a.mode != 0 || len(a.anchors) == 0).OnClick(a.releaseAnchors)
				ui.Button(c, "上一差异").Disabled(a.busy || a.mode != 0 || !a.diff.ComparisonReady()).OnClick(func() { i, _ := a.diff.CurrentHunk(); a.diff.GoToHunk(max(0, i-1)) })
				ui.Button(c, "下一差异").Disabled(a.busy || a.mode != 0 || !a.diff.ComparisonReady()).OnClick(func() {
					i, _ := a.diff.CurrentHunk()
					n := len(a.diff.Hunks())
					if n > 0 {
						a.diff.GoToHunk(min(n-1, i+1))
					}
				})
				ui.Box(c).Grow(1)
				ui.Checkbox(c, &a.wrap, "自动换行")
			}
		})
		if a.err != "" {
			ui.Text(c, a.err).Padding(10, 20).TextColor(ui.Hex("#b5472e")).Label("错误")
		}
		if a.kind == 2 {
			a.mergeView(c)
		} else if a.kind == 3 {
			a.pictureView(c)
		} else if a.kind == 4 {
			a.mediaView(c)
		} else if a.kind == 5 {
			a.versionView(c)
		} else if a.directoryRows != nil {
			a.directoryView(c)
		} else if a.panes[0] == nil {
			ui.Column(c).Grow(1).FillWidth().Center().Gap(12).Children(func() {
				ui.Text(c, "看清每一处变化").FontSize(30).Bold()
				ui.Text(c, "选择两个文本文件，比较内容并逐块应用修改。支持 UTF-8、UTF-16 BOM 和可识别的 GBK。").TextColor(t.TextMuted)
				ui.Text(c, "支持同屏双侧编辑、撤销、CRLF 与 UTF-8 BOM 保留。").FontSize(13).TextColor(t.TextMuted)
			})
		} else {
			if a.mode == 0 {
				ui.Row(c).Padding(6, 20).Gap(8).Children(func() {
					ui.Button(c, "← 应用到左侧").Disabled(a.busy || !a.diff.ComparisonReady()).OnClick(func() { a.apply(0) })
					ui.Button(c, "应用到右侧 →").Disabled(a.busy || !a.diff.ComparisonReady()).OnClick(func() { a.apply(1) })
				})
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
			ui.Row(c).Padding(8, 20).Gap(12).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
				for side, p := range a.panes {
					side, p := side, p
					label := []string{"保存左侧", "保存右侧"}[side]
					if p.buffer.Modified() {
						label += " •"
					}
					ui.Button(c, label).Disabled(a.busy || !p.buffer.Modified()).OnClick(func() { a.save(side) })
					ending := "LF"
					if p.file.CRLF {
						ending = "CRLF"
					}
					encoding := strings.ToUpper(p.file.Encoding)
					if encoding == "" {
						encoding = "UTF-8"
					}
					if p.file.BOM {
						encoding += " BOM"
					}
					ui.Button(c, "另存为…").Disabled(a.busy).OnClick(func() { a.saveAs(side) })
					ui.Text(c, encoding+" / "+ending).FontSize(11).TextColor(t.TextMuted)
				}
			})
		}
		message := a.message
		if a.compareRunning {
			message += " · 正在后台计算差异…"
		} else if a.compareErr != nil {
			message += " · 差异计算已停止，可重新计算"
		}
		if a.busy {
			message = "正在处理文件…"
		}
		ui.Text(c, message).Padding(6, 20).FontSize(12).TextColor(t.TextMuted)
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
