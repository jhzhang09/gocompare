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
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"gocompare/internal/brand"
	"gocompare/internal/compare"
	"gocompare/internal/config"
	"gocompare/internal/dirmerge"
	"gocompare/internal/document"
	"gocompare/internal/i18n"
	"gocompare/internal/media"
	"gocompare/internal/merge"
	"gocompare/internal/operations"
	"gocompare/internal/pever"
	"gocompare/internal/picture"
	"gocompare/internal/regfile"
	"gocompare/internal/table"
	"gocompare/internal/textcompare"
	"gocompare/internal/updater"
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
	kindTable
)

type pane struct {
	file   *document.File
	buffer *editor.Buffer
	edit   editor.State
}

// editHistory 记录双侧编辑的先后顺序，配合各侧独立的 Buffer 撤销栈，
// 使公共工具栏能按实际修改顺序跨侧撤销/重做，同时各侧头部也能单独撤销/重做本侧。
// 栈中元素：0 表示左侧，1 表示右侧，2 表示两侧同时变更（如交换两侧、清空两侧、载入示例）。
type editHistory struct {
	buffers  [2]*editor.Buffer
	versions [2]uint64
	undo     []int
	redo     []int
}

func (h *editHistory) clone() editHistory {
	return editHistory{
		buffers:  h.buffers,
		versions: h.versions,
		undo:     append([]int(nil), h.undo...),
		redo:     append([]int(nil), h.redo...),
	}
}

func (h *editHistory) sync(panes [2]*pane) {
	if panes[0] == nil || panes[1] == nil {
		*h = editHistory{}
		return
	}
	b0, b1 := panes[0].buffer, panes[1].buffer
	if h.buffers[0] != b0 || h.buffers[1] != b1 {
		*h = editHistory{
			buffers:  [2]*editor.Buffer{b0, b1},
			versions: [2]uint64{b0.Version(), b1.Version()},
		}
		return
	}
	for side := 0; side < 2; side++ {
		v := h.buffers[side].Version()
		if v == h.versions[side] {
			continue
		}
		h.versions[side] = v
		if h.buffers[side].CanUndo() && !h.buffers[side].CanRedo() {
			h.undo = append(h.undo, side)
			h.removeSide(&h.redo, side)
		} else if h.buffers[side].CanRedo() {
			if h.popLastSide(&h.undo, side) {
				h.redo = append(h.redo, side)
			}
		}
	}
}

func (h *editHistory) recordEdit(panes [2]*pane, side int) {
	if panes[0] == nil || panes[1] == nil || side < 0 || side > 1 {
		return
	}
	v := panes[side].buffer.Version()
	if v == h.versions[side] {
		return
	}
	h.versions[side] = v
	h.undo = append(h.undo, side)
	h.removeSide(&h.redo, side)
}

func (h *editHistory) recordDualEdit(panes [2]*pane, fn func()) {
	if panes[0] == nil || panes[1] == nil {
		fn()
		return
	}
	h.sync(panes)
	v0, v1 := panes[0].buffer.Version(), panes[1].buffer.Version()
	fn()
	n0, n1 := panes[0].buffer.Version(), panes[1].buffer.Version()
	h.versions = [2]uint64{n0, n1}
	switch {
	case n0 != v0 && n1 != v1:
		h.undo = append(h.undo, 2)
		h.redo = nil
	case n0 != v0:
		h.undo = append(h.undo, 0)
		h.removeSide(&h.redo, 0)
	case n1 != v1:
		h.undo = append(h.undo, 1)
		h.removeSide(&h.redo, 1)
	}
}

func (h *editHistory) popLastSide(stack *[]int, side int) bool {
	s := *stack
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == side {
			*stack = append(s[:i], s[i+1:]...)
			return true
		}
		if s[i] == 2 {
			s[i] = 1 - side
			return true
		}
	}
	return false
}

func (h *editHistory) removeSide(stack *[]int, side int) {
	s := *stack
	out := s[:0]
	for _, item := range s {
		if item == side {
			continue
		}
		if item == 2 {
			out = append(out, 1-side)
			continue
		}
		out = append(out, item)
	}
	*stack = out
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
	history        editHistory
	anchors        []textcompare.Anchor
	anchorLeft     int
	anchorRight    int
	anchorLeftSet  bool
	anchorRightSet bool
	mode           int

	directoryRoots    [2]string
	directoryRows     []compare.Entry
	directoryList     ui.ListState
	syncPlan          *operations.Plan
	dirMergeDecisions []dirmerge.Decision
	dirFilter         string
	compareMode       bool
	compareTime       bool
	followLinks       bool
	closedDirs        map[string]bool
	directoryOptions  compare.ScanOptions

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

	infoKind     string
	infoLeft     string
	infoRight    string
	infoLeftPE   pever.Info
	infoRightPE  pever.Info
	infoDiffs    []string
	infoRegDiffs []regfile.DiffEntry
	infoRegPage  int
	infoReady    bool

	tableSheets        [2][]table.Sheet
	tableDiffs         []table.Diff
	tableReady         bool
	tableSelectedSheet string
	tablePage          int

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
	blankPanes        [2]*pane
	blankSources      [2]*editor.Buffer
	blankVersions     [2]uint64
	blankDiff         editor.DiffState
	inlineDiff        bool
	modeSessions      map[int]*modeSession
	closedDirs        map[string]bool
	dirSearch         string
	directoryOptions  compare.ScanOptions
	allowMissing      bool
	win               *mygo.Window
	icon              *ui.Bitmap
	backupDir         string
	panes             [2]*pane
	paths             [2]string
	kind              int
	directoryRoots    [2]string
	directoryRows     []compare.Entry
	directoryList     ui.ListState
	onlyDifferences   bool
	dirFilter         string
	compareMode       bool
	compareTime       bool
	followLinks       bool
	scanSeen          []string
	scanPath          string
	syncChoice        string
	syncPlan          *operations.Plan
	dirMergeDecisions []dirmerge.Decision

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

	infoGen      uint64
	infoKind     string
	infoLeft     string
	infoRight    string
	infoLeftPE   pever.Info
	infoRightPE  pever.Info
	infoDiffs    []string
	infoRegDiffs []regfile.DiffEntry
	infoRegPage  int
	infoReady    bool

	tableGen           uint64
	tableSheets        [2][]table.Sheet
	tableDiffs         []table.Diff
	tableReady         bool
	tableSelectedSheet string
	tablePage          int

	// hunkFocus 是上一/下一差异定位到的差异块序号（从 1 开始），0 表示尚未定位；零值即可用。
	hunkFocus int
	// pendingFocusHunk 记录应用差异块后待自动聚焦的差异块序号（从 1 开始），避免重算后失焦或跳回首处。
	pendingFocusHunk int

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

	scratchPanes   [2]*pane
	scratchDiff    editor.DiffState
	scratchHistory editHistory
	history        editHistory

	updateChecking bool
	updateStatus   string
	updateURL      string

	// remoteSecrets 只在内存中保存远程密码，键为 remote.CredentialKey；不写入会话或设置。
	remoteSecrets map[string]string
	// remoteIdentities 记录连接对话框里为 SFTP 选的私钥文件，同样只在内存中。
	remoteIdentities map[string]string
	// remotePassphrases 是 SFTP 私钥口令，只在内存中。
	remotePassphrases map[string]string
	passwordPrompt    *passwordPrompt
	hostKeyPrompt     *hostKeyPrompt
	remoteDialog      *remoteDialog
	// remotesFile 是最近连接的保存位置；为空时不读写（测试默认如此）。
	remotesFile string

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
			remotesFile:   filepath.Join(configDir, "GoCompare", "remotes.json"),
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

// checkForUpdates 独立于 a.job 运行：更新检查可以和比较任务并行，
// 不能覆盖当前任务的 cancel，也不能在任务未完成时把 busy 复位。
func (a *workspaceState) checkForUpdates(customURL string) {
	if a.updateChecking {
		return
	}
	a.updateChecking = true
	a.updateStatus = i18n.T("settings.checkingUpdate")
	a.updateURL = ""
	run := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		res, err := updater.Check(ctx, customURL, brand.AppVersion(), updater.ProductApp)
		a.publish(func() {
			a.updateChecking = false
			if err != nil {
				a.updateStatus = i18n.T("settings.updateFailed") + ": " + err.Error()
				return
			}
			if res.HasUpdate {
				a.updateStatus = i18n.TArgs("settings.updateAvailable", res.LatestVersion)
				a.updateURL = res.DownloadURL
			} else {
				a.updateStatus = i18n.T("settings.updateLatest")
			}
		})
	}
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
	auth := a.takeURLPasswords()
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
	if a.kind == kindTable {
		a.loadTable()
		return
	}
	a.job(func(ctx context.Context) {
		l, le := os.Stat(paths[0])
		r, re := os.Stat(paths[1])
		if le == nil && re == nil && l.IsDir() && r.IsDir() {
			a.publish(func() { a.busy = false; a.kind = kindDirectory; a.loadDirectory(paths) })
			return
		}
		left, err := openInput(ctx, paths[0], allowMissing, auth)
		var right *document.File
		if err == nil {
			right, err = openInput(ctx, paths[1], allowMissing, auth)
		}
		a.publish(func() {
			a.busy = false
			if err != nil {
				if !a.handleRemoteError(err, a.load) {
					a.err = err.Error()
				}
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
	a.history = editHistory{}
	a.history.sync(a.panes)
	a.anchors = nil
	a.anchorLeftSet, a.anchorRightSet = false, false
	a.hunkFocus = 0
	a.pendingFocusHunk = 0
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

// handleDrop 处理从外部拖入的文件或目录，自动填充到输入框并按需触发比较。
func (a *workspaceState) handleDrop(side int, files []string) {
	if a.busy || len(files) == 0 {
		return
	}
	if len(files) >= 2 {
		// 一次性拖入多个文件或文件夹：分别填充左右两侧并触发加载
		a.paths[0] = files[0]
		a.paths[1] = files[1]
		a.guard(a.load)
		return
	}
	first := files[0]
	if side == 0 || side == 1 {
		a.paths[side] = first
		if fi, err := os.Stat(first); err == nil {
			if fi.IsDir() && a.kind == kindFile && strings.TrimSpace(a.paths[1-side]) == "" {
				a.kind = kindDirectory
			} else if !fi.IsDir() && a.kind == kindDirectory && strings.TrimSpace(a.paths[1-side]) == "" {
				a.kind = kindFile
			}
		}
		if strings.TrimSpace(a.paths[1-side]) != "" {
			a.guard(a.load)
		}
		return
	}
	// 未明确指定侧（例如拖入整体空白引导区域）：若左侧为空填左侧，否则填右侧
	if strings.TrimSpace(a.paths[0]) == "" {
		a.paths[0] = first
		if fi, err := os.Stat(first); err == nil && fi.IsDir() && a.kind == kindFile && strings.TrimSpace(a.paths[1]) == "" {
			a.kind = kindDirectory
		}
		if strings.TrimSpace(a.paths[1]) != "" {
			a.guard(a.load)
		}
	} else if strings.TrimSpace(a.paths[1]) == "" {
		a.paths[1] = first
		a.guard(a.load)
	} else {
		a.paths[0] = first
		a.guard(a.load)
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
	if panes[side].file != nil && panes[side].file.ReadOnly() {
		a.err = document.ErrReadOnly.Error()
		return
	}
	hunks := diff.Hunks()
	if len(hunks) == 0 {
		a.message = i18n.T("status.selectHunk")
		return
	}
	i := -1
	if a.hunkFocus > 0 && a.hunkFocus <= len(hunks) {
		i = a.hunkFocus - 1
	} else if cur, inside := diff.CurrentHunk(); inside && cur >= 0 && cur < len(hunks) {
		i = cur
	} else {
		i = 0
	}
	a.applyHunk(i, side)
}

func (a *workspaceState) applyHunk(i int, side int) {
	if a.ignoreBlankLines {
		a.message = i18n.T("status.blankReadOnly")
		return
	}
	diff := a.activeDiff()
	panes := a.activePanes()
	if a.busy || a.compareRunning || a.compareErr != nil || panes[0] == nil || !diff.ComparisonReady() {
		return
	}
	if side < 0 || side > 1 || panes[side] == nil {
		return
	}
	if panes[side].file != nil && panes[side].file.ReadOnly() {
		a.err = document.ErrReadOnly.Error()
		return
	}
	hunks := diff.Hunks()
	if i < 0 || i >= len(hunks) {
		a.message = i18n.T("status.selectHunk")
		return
	}
	a.pendingFocusHunk = i + 1
	h := a.activeHistory()
	h.sync(panes)
	// Revert 将左侧块复制到右侧；Accept 将右侧块复制到左侧，仅改变内存缓冲区。
	if side == 1 {
		diff.Revert(i)
	} else {
		diff.Accept(i)
	}
	h.recordEdit(panes, side)
	a.refreshComparison()
	a.message = i18n.T("status.hunkApplied")
}

func (a *workspaceState) activeHistory() *editHistory {
	if a.kind == kindScratch {
		return &a.scratchHistory
	}
	return &a.history
}

func (a *workspaceState) canUndo() bool {
	panes, _ := a.basePanes()
	if panes[0] == nil || panes[1] == nil {
		return false
	}
	return panes[0].buffer.CanUndo() || panes[1].buffer.CanUndo()
}

func (a *workspaceState) canRedo() bool {
	panes, _ := a.basePanes()
	if panes[0] == nil || panes[1] == nil {
		return false
	}
	return panes[0].buffer.CanRedo() || panes[1].buffer.CanRedo()
}

func (a *workspaceState) undoSide(side int) {
	if a.ignoreBlankLines {
		a.message = i18n.T("status.blankReadOnly")
		return
	}
	panes, _ := a.basePanes()
	if a.busy || side < 0 || side > 1 || panes[side] == nil {
		return
	}
	if panes[side].file != nil && panes[side].file.ReadOnly() {
		a.err = document.ErrReadOnly.Error()
		return
	}
	h := a.activeHistory()
	h.sync(panes)
	if !panes[side].buffer.Undo() {
		return
	}
	h.versions[side] = panes[side].buffer.Version()
	h.popLastSide(&h.undo, side)
	h.redo = append(h.redo, side)
	a.refreshComparison()
	a.message = i18n.T([]string{"status.undoneLeft", "status.undoneRight"}[side])
}

func (a *workspaceState) redoSide(side int) {
	if a.ignoreBlankLines {
		a.message = i18n.T("status.blankReadOnly")
		return
	}
	panes, _ := a.basePanes()
	if a.busy || side < 0 || side > 1 || panes[side] == nil {
		return
	}
	if panes[side].file != nil && panes[side].file.ReadOnly() {
		a.err = document.ErrReadOnly.Error()
		return
	}
	h := a.activeHistory()
	h.sync(panes)
	if !panes[side].buffer.Redo() {
		return
	}
	h.versions[side] = panes[side].buffer.Version()
	h.popLastSide(&h.redo, side)
	h.undo = append(h.undo, side)
	a.refreshComparison()
	a.message = i18n.T([]string{"status.redoneLeft", "status.redoneRight"}[side])
}

func (a *workspaceState) undo() {
	if a.ignoreBlankLines {
		a.message = i18n.T("status.blankReadOnly")
		return
	}
	panes, diff := a.basePanes()
	if a.busy || panes[0] == nil || panes[1] == nil {
		return
	}
	if a.kind == kindFile && a.mode > 0 {
		a.undoSide(a.mode - 1)
		return
	}
	h := a.activeHistory()
	h.sync(panes)
	for len(h.undo) > 0 {
		last := h.undo[len(h.undo)-1]
		if last == 2 {
			h.undo = h.undo[:len(h.undo)-1]
			u0 := panes[0].buffer.CanUndo() && (panes[0].file == nil || !panes[0].file.ReadOnly())
			u1 := panes[1].buffer.CanUndo() && (panes[1].file == nil || !panes[1].file.ReadOnly())
			if u0 && u1 {
				panes[0].buffer.Undo()
				panes[1].buffer.Undo()
				h.versions[0] = panes[0].buffer.Version()
				h.versions[1] = panes[1].buffer.Version()
				h.redo = append(h.redo, 2)
				a.refreshComparison()
				a.message = i18n.T("status.undone")
				return
			}
			if u0 {
				h.undo = append(h.undo, 0)
				a.undoSide(0)
				return
			}
			if u1 {
				h.undo = append(h.undo, 1)
				a.undoSide(1)
				return
			}
			continue
		}
		if last >= 0 && last <= 1 && panes[last].buffer.CanUndo() {
			a.undoSide(last)
			return
		}
		h.undo = h.undo[:len(h.undo)-1]
	}
	active := int(diff.ActiveSide())
	if panes[active].buffer.CanUndo() {
		a.undoSide(active)
	} else if panes[1-active].buffer.CanUndo() {
		a.undoSide(1 - active)
	}
}

func (a *workspaceState) redo() {
	if a.ignoreBlankLines {
		a.message = i18n.T("status.blankReadOnly")
		return
	}
	panes, diff := a.basePanes()
	if a.busy || panes[0] == nil || panes[1] == nil {
		return
	}
	if a.kind == kindFile && a.mode > 0 {
		a.redoSide(a.mode - 1)
		return
	}
	h := a.activeHistory()
	h.sync(panes)
	for len(h.redo) > 0 {
		last := h.redo[len(h.redo)-1]
		if last == 2 {
			h.redo = h.redo[:len(h.redo)-1]
			r0 := panes[0].buffer.CanRedo() && (panes[0].file == nil || !panes[0].file.ReadOnly())
			r1 := panes[1].buffer.CanRedo() && (panes[1].file == nil || !panes[1].file.ReadOnly())
			if r0 && r1 {
				panes[0].buffer.Redo()
				panes[1].buffer.Redo()
				h.versions[0] = panes[0].buffer.Version()
				h.versions[1] = panes[1].buffer.Version()
				h.undo = append(h.undo, 2)
				a.refreshComparison()
				a.message = i18n.T("status.redone")
				return
			}
			if r0 {
				h.redo = append(h.redo, 0)
				a.redoSide(0)
				return
			}
			if r1 {
				h.redo = append(h.redo, 1)
				a.redoSide(1)
				return
			}
			continue
		}
		if last >= 0 && last <= 1 && panes[last].buffer.CanRedo() {
			a.redoSide(last)
			return
		}
		h.redo = h.redo[:len(h.redo)-1]
	}
	active := int(diff.ActiveSide())
	if panes[active].buffer.CanRedo() {
		a.redoSide(active)
	} else if panes[1-active].buffer.CanRedo() {
		a.redoSide(1 - active)
	}
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
	a.scratchHistory = editHistory{}
	a.scratchHistory.sync(a.scratchPanes)
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
		paths:              a.paths,
		panes:              a.panes,
		diff:               a.diff,
		history:            a.history.clone(),
		anchors:            anchors,
		anchorLeft:         a.anchorLeft,
		anchorRight:        a.anchorRight,
		anchorLeftSet:      a.anchorLeftSet,
		anchorRightSet:     a.anchorRightSet,
		mode:               a.mode,
		directoryRoots:     a.directoryRoots,
		directoryRows:      a.directoryRows,
		directoryList:      a.directoryList,
		syncPlan:           a.syncPlan,
		dirMergeDecisions:  a.dirMergeDecisions,
		dirFilter:          a.dirFilter,
		compareMode:        a.compareMode,
		compareTime:        a.compareTime,
		followLinks:        a.followLinks,
		closedDirs:         closed,
		directoryOptions:   a.directoryOptions,
		mergeBase:          a.mergeBase,
		mergeOutput:        a.mergeOutput,
		mergeResult:        a.mergeResult,
		mergeRows:          a.mergeRows,
		mergeAlign:         a.mergeAlign,
		mergeBuffer:        a.mergeBuffer,
		mergeEdit:          a.mergeEdit,
		mergeLeft:          a.mergeLeft,
		mergeSaved:         a.mergeSaved,
		pictureLeft:        a.pictureLeft,
		pictureRight:       a.pictureRight,
		pictureResult:      a.pictureResult,
		pictureBitmap:      a.pictureBitmap,
		pictureMode:        a.pictureMode,
		pictureTol:         a.pictureTol,
		pictureZoom:        a.pictureZoom,
		pictureNote:        a.pictureNote,
		pictureSummary:     a.pictureSummary,
		pictureFlips:       a.pictureFlips,
		mediaLeft:          a.mediaLeft,
		mediaRight:         a.mediaRight,
		mediaReady:         a.mediaReady,
		infoKind:           a.infoKind,
		infoLeft:           a.infoLeft,
		infoRight:          a.infoRight,
		infoLeftPE:         a.infoLeftPE,
		infoRightPE:        a.infoRightPE,
		infoDiffs:          append([]string(nil), a.infoDiffs...),
		infoRegDiffs:       append([]regfile.DiffEntry(nil), a.infoRegDiffs...),
		infoRegPage:        a.infoRegPage,
		infoReady:          a.infoReady,
		tableSheets:        a.tableSheets,
		tableDiffs:         a.tableDiffs,
		tableReady:         a.tableReady,
		tableSelectedSheet: a.tableSelectedSheet,
		tablePage:          a.tablePage,
		message:            a.message,
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
	a.history = sess.history.clone()
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
	a.dirMergeDecisions = sess.dirMergeDecisions
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
	a.infoLeftPE = sess.infoLeftPE
	a.infoRightPE = sess.infoRightPE
	a.infoDiffs = sess.infoDiffs
	a.infoRegDiffs = sess.infoRegDiffs
	a.infoRegPage = sess.infoRegPage
	a.infoReady = sess.infoReady
	a.tableSheets = sess.tableSheets
	a.tableDiffs = sess.tableDiffs
	a.tableReady = sess.tableReady
	a.tableSelectedSheet = sess.tableSelectedSheet
	a.tablePage = sess.tablePage
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
