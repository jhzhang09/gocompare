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
	optionsOpen        bool
	ignoreSpace        bool
	compareIgnoreSpace bool
	closedDirs         map[string]bool
	dirSearch          string
	directoryOptions   compare.ScanOptions
	allowMissing       bool
	win                *mygo.Window
	icon               *ui.Bitmap
	backupDir          string
	panes              [2]*pane
	paths              [2]string
	kind               int
	directoryRoots     [2]string
	directoryRows      []compare.Entry
	directoryList      ui.ListState
	onlyDifferences    bool
	dirFilter          string
	compareMode        bool
	compareTime        bool
	followLinks        bool
	scanSeen           []string
	scanPath           string
	syncChoice         string
	syncPlan           *operations.Plan

	confirmSync    bool
	syncGen        uint64
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
	if a.kind == 2 && a.mergeBuffer != nil && a.mergeBuffer.Modified() {
		return true
	}
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
	allowMissing := a.allowMissing
	a.allowMissing = false
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
		l, le := os.Stat(paths[0])
		r, re := os.Stat(paths[1])
		if le == nil && re == nil && l.IsDir() && r.IsDir() {
			a.publish(func() { a.busy = false; a.kind = 1; a.loadDirectory(paths) })
			return
		}
		left, err := readInput(ctx, paths[0], allowMissing)
		var right *document.File
		if err == nil {
			right, err = readInput(ctx, paths[1], allowMissing)
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
	a.panes[0].edit.Label = "左侧编辑器"
	a.panes[1].edit.Label = "右侧编辑器"
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
		if a.backupDir != "" && !file.Missing() {
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
	if a.busy || a.compareRunning || a.compareErr != nil || a.panes[0] == nil || !a.diff.ComparisonReady() {
		return
	}
	hunks := a.diff.Hunks()
	i, inside := a.diff.CurrentHunk()
	if len(hunks) > 0 && (!inside || i < 0) {
		i = 0
		a.diff.GoToHunk(0)
		inside = true
	}
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
