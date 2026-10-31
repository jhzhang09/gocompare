// Package native 实现 GoCompare 的 MyGo 原生工作区。
package native

import (
	"context"
	"errors"
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
	"gocompare/internal/config"
	"gocompare/internal/document"
	"gocompare/internal/i18n"
	"gocompare/internal/media"
	"gocompare/internal/merge"
	"gocompare/internal/operations"
	"gocompare/internal/picture"
	"gocompare/internal/textcompare"
)

// 工作区模式编号，作为模式会话的键；新增模式只能追加，不能重排。
const (
	kindFile = iota
	kindDirectory
	kindMerge
	kindPicture
	kindMedia
	kindVersion
	kindScratch
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

type modeSession struct {
	paths          [2]string
	panes          [2]*pane
	diff           editor.DiffState
	anchors        []textcompare.Anchor
	anchorLeft     int
	anchorRight    int
	anchorLeftSet  bool
	anchorRightSet bool
	mode           int

	directoryRoots   [2]string
	directoryRows    []compare.Entry
	directoryList    ui.ListState
	syncPlan         *operations.Plan
	dirFilter        string
	compareMode      bool
	compareTime      bool
	followLinks      bool
	closedDirs       map[string]bool
	directoryOptions compare.ScanOptions

	mergeBase   string
	mergeOutput string
	mergeResult *merge.Result
	mergeRows   []merge.AlignRow
	mergeAlign  ui.ListState
	mergeBuffer *editor.Buffer
	mergeEdit   editor.State
	mergeLeft   *document.File
	mergeSaved  *document.File

	pictureLeft    image.Image
	pictureRight   image.Image
	pictureResult  *image.RGBA
	pictureBitmap  *ui.Bitmap
	pictureMode    int
	pictureTol     uint8
	pictureZoom    int
	pictureNote    string
	pictureSummary picture.Summary
	pictureFlips   [2]pictureFlip

	mediaLeft  media.Tags
	mediaRight media.Tags
	mediaReady bool

	infoKind  string
	infoLeft  string
	infoRight string
	infoDiffs []string
	infoReady bool

	message string
}

type workspaceState struct {
	optionsOpen        bool
	settingsOpen       bool
	settingsLang       int
	settingsTheme      int
	ignoreSpace        bool
	compareIgnoreSpace bool
	ignoreBlankLines   bool
	// 忽略空行时比较视图换成去除空行的只读副本，用户缓冲区和保存基线保持原样。
	blankPanes       [2]*pane
	blankSources     [2]*editor.Buffer
	blankVersions    [2]uint64
	blankDiff        editor.DiffState
	inlineDiff       bool
	modeSessions     map[int]*modeSession
	closedDirs       map[string]bool
	dirSearch        string
	directoryOptions compare.ScanOptions
	allowMissing     bool
	win              *mygo.Window
	icon             *ui.Bitmap
	backupDir        string
	panes            [2]*pane
	paths            [2]string
	kind             int
	directoryRoots   [2]string
	directoryRows    []compare.Entry
	directoryList    ui.ListState
	onlyDifferences  bool
	dirFilter        string
	compareMode      bool
	compareTime      bool
	followLinks      bool
	scanSeen         []string
	scanPath         string
	syncChoice       string
	syncPlan         *operations.Plan

	confirmSync    bool
	syncGen        uint64
	lastJournal    string
	syncRolledBack bool
	directoryNote  string

	mergeBase   string
	mergeOutput string
	mergeResult *merge.Result
	mergeRows   []merge.AlignRow
	mergeAlign  ui.ListState
	mergeBuffer *editor.Buffer
	mergeEdit   editor.State
	mergeLeft   *document.File
	// mergeSaved 是本窗口上次写出的合并输出，只有它可以被再次保存覆盖。
	mergeSaved    *document.File
	mergeConflict int
	mergeWindows  []*workspaceState
	detached      bool

	pictureMode   int
	pictureTol    uint8
	pictureZoom   int
	pictureGen    uint64
	pictureLeft   image.Image
	pictureRight  image.Image
	pictureResult *image.RGBA
	pictureBitmap *ui.Bitmap
	pictureNote   string
	// pictureSummary 与画布一起在后台算好，绘制时只读取。
	pictureSummary   picture.Summary
	pictureFlips     [2]pictureFlip
	pictureTolSlider float64
	pictureRerender  bool

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

	scratchPanes [2]*pane
	scratchDiff  editor.DiffState

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
	if len(args) == 1 && args[0] == "--version" {
		fmt.Printf("GoCompare %s\nMyGo %s\n", brand.AppVersion(), version)
		return nil
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
	cfg := config.Load()
	i18n.SetLanguage(i18n.Language(cfg.Language))
	switch cfg.Theme {
	case "light":
		mygo.Theme.SetSource(mygo.ThemeLight)
	case "dark":
		mygo.Theme.SetSource(mygo.ThemeDark)
	default:
		mygo.Theme.SetSource(mygo.ThemeSystem)
	}

	langIdx := 0
	switch cfg.Language {
	case "zh-TW":
		langIdx = 1
	case "en":
		langIdx = 2
	}
	themeIdx := 2
	switch cfg.Theme {
	case "light":
		themeIdx = 0
	case "dark":
		themeIdx = 1
	}

	mygo.App.SetName("GoCompare")
	mygo.App.SetVersion(brand.AppVersion())
	var iconErr error
	mygo.App.WhenReady(func() {
		// darwin 的 Window.SetIcon 不改变图标。Dock.SetIcon 才会把应用图标换成这张 PNG。
		if iconErr = mygo.App.Dock.SetIcon(brand.IconPNG()); iconErr != nil {
			mygo.App.Quit()
			return
		}
		a := &workspaceState{
			optionsOpen:   true,
			inlineDiff:    true,
			settingsLang:  langIdx,
			settingsTheme: themeIdx,
			message:       i18n.T("status.selectSource"),
			backupDir:     filepath.Join(configDir, "GoCompare", "backups"),
		}
		a.win = mygo.NewWindow(mygo.WindowOptions{Title: "GoCompare", Width: 1380, Height: 880, MinWidth: 900, MinHeight: 600, Content: ui.View(a.view)})
		a.win.OnClose(func(e *mygo.CloseEvent) {
			if a.busy {
				e.PreventDefault()
				a.message = i18n.T("status.closeBusy")
				return
			}
			if a.dirty() || a.scratchUnsaved() {
				e.PreventDefault()
				a.pending = func() { a.win.Destroy() }
				a.confirm = true
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
				a.kind = kindDirectory
			}
			a.load()
		} else {
			a.kind = kindScratch
			a.ensureScratch()
			a.message = i18n.T("status.scratchReady")
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
	if a.kind == kindMerge && a.mergeBuffer != nil && a.mergeBuffer.Modified() {
		return true
	}
	for _, p := range a.panes {
		if p != nil && p.buffer.Modified() {
			return true
		}
	}
	return false
}

// scratchUnsaved 判断纯文本是否有未另存的内容。纯文本没有文件基线，只在关窗时提示，
// 切换文件或目录不会影响它。
func (a *workspaceState) scratchUnsaved() bool {
	for _, p := range a.scratchPanes {
		if p != nil && p.buffer.Len() > 0 && p.buffer.Modified() {
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
		a.err = i18n.T("err.selectBothFiles")
		return
	}
	if a.kind == kindDirectory {
		a.loadDirectory(paths)
		return
	}
	if a.kind == kindMerge {
		a.loadMerge()
		return
	}
	if a.kind == kindPicture {
		a.loadPictures()
		return
	}
	if a.kind == kindMedia {
		a.loadMedia()
		return
	}
	if a.kind == kindVersion {
		a.loadVersion()
		return
	}
	a.job(func(ctx context.Context) {
		l, le := os.Stat(paths[0])
		r, re := os.Stat(paths[1])
		if le == nil && re == nil && l.IsDir() && r.IsDir() {
			a.publish(func() { a.busy = false; a.kind = kindDirectory; a.loadDirectory(paths) })
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
	a.panes[0].edit.Label = i18n.T("a11y.leftEditor")
	a.panes[1].edit.Label = i18n.T("a11y.rightEditor")
	a.kind = kindFile
	a.directoryRows = nil
	a.paths = [2]string{left.Path, right.Path}
	a.diff = editor.DiffState{Old: a.panes[0].buffer, New: a.panes[1].buffer, ExpandUnchanged: true, Editable: true, OldEditable: true, ExternalComparison: true}
	a.anchors = nil
	a.anchorLeftSet, a.anchorRightSet = false, false
	a.stopComparison()
	a.compareRequested = false
	a.refreshComparison()
	a.mode = 0
	a.message = i18n.T("status.filesLoaded")
	a.updateTitle()
}
func (a *workspaceState) browse(side int) {
	if a.busy {
		return
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: i18n.T([]string{"dialog.pickLeft", "dialog.pickRight"}[side]), Directory: a.kind == kindDirectory})
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
			a.message = i18n.TArgs("status.saved", filepath.Base(f.Path))
		})
	})
}
func (a *workspaceState) apply(side int) {
	if a.ignoreBlankLines {
		a.message = i18n.T("status.blankReadOnly")
		return
	}
	diff := a.activeDiff()
	panes := a.activePanes()
	if a.busy || a.compareRunning || a.compareErr != nil || panes[0] == nil || !diff.ComparisonReady() {
		return
	}
	hunks := diff.Hunks()
	i, inside := diff.CurrentHunk()
	if len(hunks) > 0 && (!inside || i < 0) {
		i = 0
		diff.GoToHunk(0)
		inside = true
	}
	if !inside || i < 0 || i >= len(hunks) {
		a.message = i18n.T("status.selectHunk")
		return
	}
	// Revert 将左侧块复制到右侧；Accept 将右侧块复制到左侧，仅改变内存缓冲区。
	if side == 1 {
		diff.Revert(i)
	} else {
		diff.Accept(i)
	}
	a.message = i18n.T("status.hunkApplied")
}

func (a *workspaceState) ensureScratch() {
	if a.scratchPanes[0] != nil {
		return
	}
	left := &document.File{Path: i18n.T("scratch.leftName"), Text: "", Encoding: "utf-8"}
	right := &document.File{Path: i18n.T("scratch.rightName"), Text: "", Encoding: "utf-8"}
	a.scratchPanes = [2]*pane{newPane(left), newPane(right)}
	a.scratchPanes[0].edit.Label = i18n.T("a11y.leftScratchEditor")
	a.scratchPanes[1].edit.Label = i18n.T("a11y.rightScratchEditor")
	a.scratchDiff = editor.DiffState{
		Old:                a.scratchPanes[0].buffer,
		New:                a.scratchPanes[1].buffer,
		ExpandUnchanged:    true,
		Editable:           true,
		OldEditable:        true,
		ExternalComparison: true,
	}
}

// basePanes 返回当前模式下用户可编辑的面板与差异状态。
func (a *workspaceState) basePanes() ([2]*pane, *editor.DiffState) {
	if a.kind == kindScratch {
		a.ensureScratch()
		return a.scratchPanes, &a.scratchDiff
	}
	return a.panes, &a.diff
}

// activePanes 返回比较视图正在显示的面板；忽略空行时是只读副本。
func (a *workspaceState) activePanes() [2]*pane {
	panes, diff := a.basePanes()
	if a.ignoreBlankLines && a.ensureBlankView(panes, diff) {
		return a.blankPanes
	}
	return panes
}

// activeDiff 返回比较视图正在显示的差异状态；忽略空行时是只读副本。
func (a *workspaceState) activeDiff() *editor.DiffState {
	panes, diff := a.basePanes()
	if a.ignoreBlankLines && a.ensureBlankView(panes, diff) {
		return &a.blankDiff
	}
	return diff
}

// ensureBlankView 按源缓冲区身份和版本重建去除空行的只读副本。
// 源面板缺失时返回 false。重新加载文件或在单栏编辑后版本变化，副本随之重建，不会残留旧内容。
func (a *workspaceState) ensureBlankView(src [2]*pane, base *editor.DiffState) bool {
	if src[0] == nil || src[1] == nil {
		return false
	}
	sources := [2]*editor.Buffer{src[0].buffer, src[1].buffer}
	versions := [2]uint64{sources[0].Version(), sources[1].Version()}
	if a.blankPanes[0] != nil && a.blankSources == sources && a.blankVersions == versions {
		return true
	}
	for i, p := range src {
		text, _ := stripBlankLines(p.buffer.String())
		view := newPane(&document.File{Path: p.file.Path, Text: text, Encoding: p.file.Encoding})
		view.edit.Label = p.edit.Label
		view.edit.ReadOnly = true
		a.blankPanes[i] = view
	}
	a.blankSources, a.blankVersions = sources, versions
	a.blankDiff = editor.DiffState{Old: a.blankPanes[0].buffer, New: a.blankPanes[1].buffer, ExpandUnchanged: true, ExternalComparison: true, Inline: base.Inline}
	return true
}

func (a *workspaceState) clearBlankView() {
	a.blankPanes = [2]*pane{}
	a.blankSources = [2]*editor.Buffer{}
	a.blankVersions = [2]uint64{}
	a.blankDiff = editor.DiffState{}
}

func (a *workspaceState) saveScratchAs(side int) {
	if a.busy || a.scratchPanes[side] == nil {
		return
	}
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent: a.win,
		Title:  i18n.T([]string{"dialog.saveLeftText", "dialog.saveRightText"}[side]),
	})
	if err != nil {
		a.err = err.Error()
		return
	}
	if path != "" {
		a.saveScratchAsPath(side, path)
	}
}

// saveScratchAsPath 在后台以 UTF-8 新建文件，与文件比较的另存为一致：目标已存在时不覆盖。
func (a *workspaceState) saveScratchAsPath(side int, path string) {
	if a.busy || a.scratchPanes[side] == nil {
		return
	}
	p := a.scratchPanes[side]
	text := strings.ReplaceAll(p.buffer.String(), "\r\n", "\n")
	version := p.buffer.Version()
	file := &document.File{Encoding: "utf-8"}
	a.job(func(ctx context.Context) {
		f, err := file.SaveAs(ctx, path, text)
		a.publish(func() {
			a.busy = false
			if err != nil {
				if errors.Is(err, os.ErrExist) {
					a.err = i18n.T("err.targetExists")
				} else {
					a.err = err.Error()
				}
				return
			}
			if p.buffer.Version() == version {
				p.buffer.MarkSaved()
			}
			a.message = i18n.TArgs("status.savedTo", filepath.Base(f.Path))
		})
	})
}

func stripBlankLines(text string) (string, int) {
	if text == "" {
		return "", 0
	}
	hasTrailingNL := strings.HasSuffix(text, "\n")
	lines := strings.Split(text, "\n")
	var kept []string
	removed := 0
	for i, line := range lines {
		if i == len(lines)-1 && line == "" && hasTrailingNL {
			continue
		}
		if strings.TrimSpace(line) == "" {
			removed++
			continue
		}
		kept = append(kept, line)
	}
	result := strings.Join(kept, "\n")
	if hasTrailingNL && len(kept) > 0 {
		result += "\n"
	}
	return result, removed
}

// setIgnoreBlankLines 只切换比较视图，不改写用户缓冲区。
func (a *workspaceState) setIgnoreBlankLines(enabled bool) {
	a.ignoreBlankLines = enabled
	a.clearBlankView()
	panes, _ := a.basePanes()
	if panes[0] == nil || panes[1] == nil {
		return
	}
	a.refreshComparison()
	if enabled {
		_, remLeft := stripBlankLines(panes[0].buffer.String())
		_, remRight := stripBlankLines(panes[1].buffer.String())
		a.message = i18n.TArgs("status.blankIgnored", remLeft, remRight)
	} else {
		a.message = i18n.T("status.blankRestored")
	}
}

func (a *workspaceState) toggleIgnoreBlankLines() {
	a.setIgnoreBlankLines(!a.ignoreBlankLines)
}

func (a *workspaceState) setInlineDiff(enabled bool) {
	a.inlineDiff = enabled
	inline := editor.DiffInlineNone
	if enabled {
		inline = editor.DiffInlineWordAlt
	}
	_, base := a.basePanes()
	base.Inline = inline
	a.activeDiff().Inline = inline
	a.refreshComparison()
}

func (a *workspaceState) ensureModeSessions() {
	if a.modeSessions == nil {
		a.modeSessions = make(map[int]*modeSession)
	}
}

func (a *workspaceState) saveModeSession(kind int) {
	a.ensureModeSessions()
	var closed map[string]bool
	if a.closedDirs != nil {
		closed = make(map[string]bool, len(a.closedDirs))
		for k, v := range a.closedDirs {
			closed[k] = v
		}
	}
	anchors := append([]textcompare.Anchor(nil), a.anchors...)
	a.modeSessions[kind] = &modeSession{
		paths:            a.paths,
		panes:            a.panes,
		diff:             a.diff,
		anchors:          anchors,
		anchorLeft:       a.anchorLeft,
		anchorRight:      a.anchorRight,
		anchorLeftSet:    a.anchorLeftSet,
		anchorRightSet:   a.anchorRightSet,
		mode:             a.mode,
		directoryRoots:   a.directoryRoots,
		directoryRows:    a.directoryRows,
		directoryList:    a.directoryList,
		syncPlan:         a.syncPlan,
		dirFilter:        a.dirFilter,
		compareMode:      a.compareMode,
		compareTime:      a.compareTime,
		followLinks:      a.followLinks,
		closedDirs:       closed,
		directoryOptions: a.directoryOptions,
		mergeBase:        a.mergeBase,
		mergeOutput:      a.mergeOutput,
		mergeResult:      a.mergeResult,
		mergeRows:        a.mergeRows,
		mergeAlign:       a.mergeAlign,
		mergeBuffer:      a.mergeBuffer,
		mergeEdit:        a.mergeEdit,
		mergeLeft:        a.mergeLeft,
		mergeSaved:       a.mergeSaved,
		pictureLeft:      a.pictureLeft,
		pictureRight:     a.pictureRight,
		pictureResult:    a.pictureResult,
		pictureBitmap:    a.pictureBitmap,
		pictureMode:      a.pictureMode,
		pictureTol:       a.pictureTol,
		pictureZoom:      a.pictureZoom,
		pictureNote:      a.pictureNote,
		pictureSummary:   a.pictureSummary,
		pictureFlips:     a.pictureFlips,
		mediaLeft:        a.mediaLeft,
		mediaRight:       a.mediaRight,
		mediaReady:       a.mediaReady,
		infoKind:         a.infoKind,
		infoLeft:         a.infoLeft,
		infoRight:        a.infoRight,
		infoDiffs:        append([]string(nil), a.infoDiffs...),
		infoReady:        a.infoReady,
		message:          a.message,
	}
}

func (a *workspaceState) restoreModeSession(kind int) bool {
	a.ensureModeSessions()
	sess, ok := a.modeSessions[kind]
	if !ok {
		return false
	}
	a.paths = sess.paths
	a.panes = sess.panes
	a.diff = sess.diff
	a.anchors = sess.anchors
	a.anchorLeft = sess.anchorLeft
	a.anchorRight = sess.anchorRight
	a.anchorLeftSet = sess.anchorLeftSet
	a.anchorRightSet = sess.anchorRightSet
	a.mode = sess.mode
	a.directoryRoots = sess.directoryRoots
	a.directoryRows = sess.directoryRows
	a.directoryList = sess.directoryList
	a.syncPlan = sess.syncPlan
	a.dirFilter = sess.dirFilter
	a.compareMode = sess.compareMode
	a.compareTime = sess.compareTime
	a.followLinks = sess.followLinks
	a.closedDirs = sess.closedDirs
	a.directoryOptions = sess.directoryOptions
	a.mergeBase = sess.mergeBase
	a.mergeOutput = sess.mergeOutput
	a.mergeResult = sess.mergeResult
	a.mergeRows = sess.mergeRows
	a.mergeAlign = sess.mergeAlign
	a.mergeBuffer = sess.mergeBuffer
	a.mergeEdit = sess.mergeEdit
	a.mergeLeft = sess.mergeLeft
	a.mergeSaved = sess.mergeSaved
	a.pictureLeft = sess.pictureLeft
	a.pictureRight = sess.pictureRight
	a.pictureResult = sess.pictureResult
	a.pictureBitmap = sess.pictureBitmap
	a.pictureMode = sess.pictureMode
	a.pictureTol = sess.pictureTol
	a.pictureZoom = sess.pictureZoom
	a.pictureNote = sess.pictureNote
	a.pictureSummary = sess.pictureSummary
	a.pictureFlips = sess.pictureFlips
	a.mediaLeft = sess.mediaLeft
	a.mediaRight = sess.mediaRight
	a.mediaReady = sess.mediaReady
	a.infoKind = sess.infoKind
	a.infoLeft = sess.infoLeft
	a.infoRight = sess.infoRight
	a.infoDiffs = sess.infoDiffs
	a.infoReady = sess.infoReady
	if sess.message != "" {
		a.message = sess.message
	}
	return true
}

func (a *workspaceState) applySettings(langIdx, themeIdx int) {
	a.settingsLang = langIdx
	a.settingsTheme = themeIdx
	langs := []i18n.Language{i18n.ZhCN, i18n.ZhTW, i18n.EnUS}
	if langIdx >= 0 && langIdx < len(langs) {
		i18n.SetLanguage(langs[langIdx])
	}
	themeSources := []mygo.ThemeSource{mygo.ThemeLight, mygo.ThemeDark, mygo.ThemeSystem}
	themeStr := []string{"light", "dark", "system"}
	if themeIdx >= 0 && themeIdx < len(themeSources) {
		if mygo.App.IsReady() {
			mygo.Theme.SetSource(themeSources[themeIdx])
		}
	}
	_ = config.Save(config.Settings{
		Language: string(i18n.CurrentLanguage()),
		Theme:    themeStr[themeIdx],
	})
}
