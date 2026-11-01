package native

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"gocompare/internal/compare"
	"gocompare/internal/dirmerge"
	"gocompare/internal/i18n"
	"gocompare/internal/operations"
	"gocompare/internal/remote"
)

func (a *workspaceState) loadDirectory(paths [2]string) {
	a.scanSeen = nil
	a.scanPath = ""
	filter, mode, mtime, follow := a.dirFilter, a.compareMode, a.compareTime, a.followLinks
	auth := a.takeURLPasswords()
	paths = a.paths // 密码已从地址中移出
	a.job(func(ctx context.Context) {
		lastProgress := time.Time{}
		links := compare.LinkRecord
		if follow {
			links = compare.LinkFollow
		}
		var sources [2]compare.Source
		var err error
		for i := range paths {
			var done func()
			if sources[i], done, err = openSource(ctx, paths[i], auth); err != nil {
				break
			}
			defer done()
		}
		var rows []compare.Entry
		if err == nil {
			rows, err = compare.CompareSources(ctx, sources[0], sources[1], compare.ScanOptions{
				Filter:      filter,
				KeepParents: true,
				Links:       links,
				CompareMode: mode,
				CompareTime: mtime,
				Progress: func(rel string) error {
					if err := ctx.Err(); err != nil {
						return err
					}
					if a.win != nil && time.Since(lastProgress) < 50*time.Millisecond {
						return nil
					}
					lastProgress = time.Now()
					// 进度限流，避免扫描大目录淹没 UI 队列。
					a.publish(func() {
						a.scanPath = rel
						if a.win == nil {
							a.scanSeen = append(a.scanSeen, rel)
						}
					})
					return nil
				},
			})
		}
		a.publish(func() {
			a.busy = false
			if err != nil {
				if !a.handleRemoteError(err, a.load) {
					a.err = err.Error()
				}
				return
			}
			a.installDirectory(paths, rows)
			a.directoryOptions = compare.ScanOptions{Filter: filter, CompareMode: mode, CompareTime: mtime, Links: links}
		})
	})
}

func (a *workspaceState) installDirectory(paths [2]string, rows []compare.Entry) {
	if paths != a.directoryRoots {
		a.closedDirs = nil
		a.dirSearch = ""
	}
	a.directoryOptions = compare.ScanOptions{}
	a.stopComparison()
	a.compareRequested = false
	a.panes = [2]*pane{}
	a.syncPlan = nil
	a.dirMergeDecisions = nil
	if a.syncChoice == "" {
		a.syncChoice = i18n.T(syncModes[0].key)
	}
	a.directoryRows = rows
	a.directoryRoots = paths
	a.paths = paths
	a.kind = kindDirectory
	a.directoryList = ui.ListState{}
	if a.directoryNote != "" {
		a.message = a.directoryNote
		a.directoryNote = ""
	} else {
		a.message = i18n.TArgs("dir.done", len(rows))
	}
	if a.win != nil {
		a.win.SetTitle(i18n.T("mode.directory") + " — GoCompare")
	}
}

func (a *workspaceState) openDirectoryEntry(entry compare.Entry) {
	if a.busy {
		return
	}
	if (entry.Left != nil && entry.Left.Directory) || (entry.Right != nil && entry.Right.Directory) {
		if a.closedDirs == nil {
			a.closedDirs = map[string]bool{}
		}
		a.closedDirs[entry.Path] = !a.closedDirs[entry.Path]
		return
	}
	if entry.Status == compare.Failed || entry.Status == compare.TypeChanged || (entry.Left != nil && entry.Left.Symlink) || (entry.Right != nil && entry.Right.Symlink) {
		a.err = entry.Error
		if a.err == "" {
			a.err = i18n.T("err.notRegular")
		}
		return
	}
	a.saveModeSession(kindDirectory)
	for i, root := range a.directoryRoots {
		if remote.IsURL(root) {
			a.paths[i] = remote.JoinPath(root, entry.Path)
		} else {
			a.paths[i] = filepath.Join(root, filepath.FromSlash(entry.Path))
		}
	}
	a.allowMissing = true
	a.kind = kindFile
	a.load()
}

func (a *workspaceState) visibleDirectoryRows() []compare.Entry {
	rows := make([]compare.Entry, 0, len(a.directoryRows))
	for _, r := range a.directoryRows {
		if a.onlyDifferences && r.Status == compare.Same {
			continue
		}
		if a.dirSearch != "" && !strings.Contains(strings.ToLower(r.Path), strings.ToLower(a.dirSearch)) {
			continue
		}
		hidden := false
		if a.dirSearch == "" {
			for parent := filepath.ToSlash(filepath.Dir(r.Path)); parent != "."; parent = filepath.ToSlash(filepath.Dir(parent)) {
				if a.closedDirs[parent] {
					hidden = true
					break
				}
			}
		}
		if !hidden {
			rows = append(rows, r)
		}
	}
	return rows
}

func (a *workspaceState) directoryView(c *ui.Context) {
	if a.syncPlan != nil {
		a.syncPreviewView(c)
		return
	}
	if a.dirMergeDecisions != nil {
		a.dirMergePreviewView(c)
		return
	}
	t := c.Theme()
	pal := paletteFor(t)
	rows := a.visibleDirectoryRows()
	changed := 0
	for _, r := range rows {
		if r.Status != compare.Same {
			changed++
		}
	}

	// 搜索过滤与同步操作栏
	ui.Row(c).Height(38).Padding(0, 14).Gap(8).Background(pal.cardBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		ui.Icon(c, iconSearch).Size(13, 13).TextColor(t.TextMuted)
		ui.TextInput(c.Key("search-files"), &a.dirSearch).Placeholder(i18n.T("dir.filterPlaceholder")).Label(i18n.T("dir.searchLabel")).Grow(1).MinWidth(130)
		ui.Checkbox(c, &a.onlyDifferences, i18n.T("dir.onlyDiff"))
		statusPill(c, i18n.TArgs("dir.items", len(rows)), pal.baseAccent, pal.baseBg)
		if changed > 0 {
			statusPill(c, i18n.TArgs("dir.diffCount", changed), pal.diffFg, pal.diffBg)
		} else if len(rows) > 0 {
			statusPill(c, i18n.T("dir.allSame"), pal.successFg, pal.successBg)
		}

		ui.Divider(c).Height(16)
		// 同步、目录合并与撤销直接读写本地文件系统，任一侧是远程目录时不可用。
		localOnly := a.busy || a.remoteDirectory()
		localTip := ""
		if a.remoteDirectory() {
			localTip = i18n.T("dir.remoteLocalOnly")
		}
		a.normalizeSyncChoice()
		ui.Select(c, &a.syncChoice, syncOptions()).Disabled(localOnly)
		syncBtn := ui.Button(c, "").Label(i18n.T("sync.preview")).Padding(3, 8).Gap(4).Radius(5).Disabled(localOnly).Tooltip(localTip)
		syncBtn.OnClick(a.previewSync)
		syncBtn.Children(func() {
			ui.Icon(c, iconSync).Size(11, 11)
			ui.Text(c, i18n.T("sync.preview")).FontSize(11).SingleLine()
		})
		dirMergeBtn := ui.Button(c, "").Label(i18n.T("dirmerge.action")).Padding(3, 8).Gap(4).Radius(5).Disabled(localOnly).Tooltip(localTip)
		dirMergeBtn.OnClick(a.previewDirMerge)
		dirMergeBtn.Children(func() {
			ui.Icon(c, iconGitMerge).Size(11, 11)
			ui.Text(c, i18n.T("dirmerge.action")).FontSize(11).SingleLine()
		})
		if a.lastJournal != "" && !a.syncRolledBack {
			undoBtn := ui.Button(c, "").Label(i18n.T("sync.undo")).Padding(3, 8).Gap(4).Radius(5).Disabled(localOnly)
			undoBtn.OnClick(a.undoSync)
			undoBtn.Children(func() {
				ui.Icon(c, iconUndo).Size(11, 11)
				ui.Text(c, i18n.T("sync.undo")).FontSize(11).SingleLine()
			})
		}
		if a.lastJournal != "" {
			ui.Button(c, i18n.T("sync.journal")).Padding(3, 8).Radius(5).FontSize(11).OnClick(func() {
				if err := mygo.Shell.OpenPath(a.lastJournal); err != nil {
					a.err = err.Error()
				}
			})
		}
	})

	// 表头
	ui.Row(c).Height(30).Padding(0, 14).Gap(10).Background(pal.subtleSurface).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		ui.Row(c).Grow(1).Basis(0).Gap(6).Children(func() {
			sideBadge(c, 0, pal)
			ui.Text(c, i18n.T("dir.leftHeader")).FontSize(11).Bold().TextColor(t.TextMuted)
		})
		ui.Row(c).Width(88).Center().Children(func() {
			ui.Text(c, i18n.T("dir.resultHeader")).FontSize(11).Bold().TextColor(t.TextMuted)
		})
		ui.Row(c).Grow(1).Basis(0).Gap(6).Children(func() {
			sideBadge(c, 1, pal)
			ui.Text(c, i18n.T("dir.rightHeader")).FontSize(11).Bold().TextColor(t.TextMuted)
		})
	})

	if len(rows) == 0 {
		ui.Column(c).Grow(1).FillWidth().Center().Gap(6).Children(func() {
			ui.Text(c, i18n.T("dir.emptyTitle")).FontSize(16).Bold()
			ui.Text(c, i18n.T("dir.emptyDesc")).FontSize(12).TextColor(t.TextMuted)
		})
		return
	}

	ui.List(c, &a.directoryList, len(rows)).Grow(1).FillWidth().ItemKey(func(i int) any { return rows[i].Path }).Rows(func(row ui.ListRow) {
		entry := rows[row.Index]
		rc := row.Context
		fg, bg := pal.sameFg, pal.sameBg
		switch entry.Status {
		case compare.Different:
			fg, bg = pal.diffFg, pal.diffBg
		case compare.LeftOnly:
			fg, bg = pal.leftAccent, pal.leftBg
		case compare.RightOnly:
			fg, bg = pal.rightAccent, pal.rightBg
		case compare.Failed, compare.TypeChanged:
			fg, bg = pal.dangerFg, pal.dangerBg
		}
		rowBg := ui.Transparent
		if row.Index%2 == 1 {
			rowBg = pal.subtleSurface.Alpha(0.35)
		}
		ui.Row(rc).Height(34).Padding(0, 14).Gap(10).Background(rowBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder.Alpha(0.5)).Children(func() {
			for side, m := range []*compare.Metadata{entry.Left, entry.Right} {
				if side == 1 {
					ui.Row(rc).Width(88).Center().Children(func() {
						statusPill(rc, statusLabel(entry.Status), fg, bg)
					})
				}
				ui.Row(rc).Grow(1).Basis(0).MinWidth(0).Gap(5).Children(func() {
					if m == nil {
						ui.Text(rc, "—").Grow(1).TextColor(t.TextMuted)
						return
					}
					prefix := "· "
					icon := iconFile
					iconColor := t.TextMuted
					if m.Directory {
						icon = iconFolderOpen
						iconColor = pal.baseAccent
						prefix = "▾ "
						if a.closedDirs[entry.Path] {
							prefix = "▸ "
						}
					}
					indent := float32(strings.Count(entry.Path, "/") * 12)
					ui.Text(rc, prefix).Width(10).Margin(0, 0, 0, indent).FontSize(11).TextColor(fg)
					ui.Icon(rc, icon).Size(13, 13).TextColor(iconColor)
					btn := ui.Button(rc, filepath.Base(entry.Path)).Label(entry.Path).Grow(1).MinWidth(0).Padding(2, 4).Justify(ui.Start).BorderWidth(0).Background(ui.Transparent).Tooltip(entry.Path + "\n" + entry.Error).Disabled(a.busy)
					if m.Directory {
						btn.Bold()
					}
					btn.OnClick(func() { a.openDirectoryEntry(entry) })
					ui.Text(rc, metadataLabel(m)).Width(64).FontSize(10).TextColor(t.TextMuted).SingleLine()
				})
			}
		})
	})
}

func metadataLabel(m *compare.Metadata) string {
	if m == nil {
		return "—"
	}
	if m.Error != "" {
		return i18n.T("dir.readFailed")
	}
	if m.Symlink {
		return i18n.T("dir.symlink")
	}
	if m.Directory {
		return i18n.T("dir.directory")
	}
	return fmt.Sprintf("%d B", m.Size)
}

func statusLabel(s compare.Status) string {
	key, ok := map[compare.Status]string{compare.Same: "dir.same", compare.Different: "dir.different", compare.LeftOnly: "dir.leftOnly", compare.RightOnly: "dir.rightOnly", compare.TypeChanged: "dir.typeChanged", compare.Failed: "dir.readFailed"}[s]
	if !ok {
		return ""
	}
	return i18n.T(key)
}

// syncModes 是同步下拉框的选项顺序。下拉框绑定显示文案，执行时按文案反查模式；
// 切换语言前保存的文案也能识别，不会把选择悄悄变成默认值。
var syncModes = []struct {
	key  string
	mode operations.Mode
}{
	{"sync.updateRight", operations.UpdateRight},
	{"sync.updateLeft", operations.UpdateLeft},
	{"sync.updateBoth", operations.UpdateBoth},
	{"sync.mirrorRight", operations.MirrorRight},
	{"sync.mirrorLeft", operations.MirrorLeft},
}

func syncOptions() []string {
	out := make([]string, len(syncModes))
	for i, m := range syncModes {
		out[i] = i18n.T(m.key)
	}
	return out
}

// syncModeOf 返回文案对应的模式；未知文案返回空模式，由 Preview 报错，不默认执行任何方向。
func syncModeOf(choice string) operations.Mode {
	for _, m := range syncModes {
		if i18n.Matches(m.key, choice) {
			return m.mode
		}
	}
	return ""
}

// normalizeSyncChoice 把其他语言的已选文案换成当前语言，下拉框才能显示正确选中项。
func (a *workspaceState) normalizeSyncChoice() {
	for _, m := range syncModes {
		if i18n.Matches(m.key, a.syncChoice) {
			a.syncChoice = i18n.T(m.key)
			return
		}
	}
}

func (a *workspaceState) previewSync() {
	if a.busy || a.remoteDirectory() {
		return
	}
	if a.directoryOptions.Filter != "" || a.directoryOptions.CompareMode || a.directoryOptions.CompareTime || a.directoryOptions.Links == compare.LinkFollow {
		a.err = i18n.T("err.syncRules")
		return
	}
	roots := a.directoryRoots
	mode := syncModeOf(a.syncChoice)
	a.job(func(ctx context.Context) {
		plan, err := operations.Preview(ctx, roots[0], roots[1], mode)
		a.publish(func() {
			a.busy = false
			if err != nil {
				a.err = err.Error()
				return
			}
			a.syncPlan = &plan
			a.directoryList = ui.ListState{}
			a.message = i18n.T("sync.previewReady")
		})
	})
}

func (a *workspaceState) syncPreviewView(c *ui.Context) {
	t := c.Theme()
	pal := paletteFor(t)
	plan := a.syncPlan
	ui.Row(c).Height(38).Padding(0, 16).Gap(10).Background(pal.cardBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		ui.Button(c, i18n.T("sync.back")).Padding(4, 9).Radius(5).FontSize(11).OnClick(func() { a.syncPlan = nil; a.directoryList = ui.ListState{} })
		statusPill(c, i18n.TArgs("sync.pending", len(plan.Actions)), pal.diffFg, pal.diffBg)
		ui.Text(c, i18n.T("sync.backupNote")).Grow(1).FontSize(11).TextColor(t.TextMuted).SingleLine()
		ui.PrimaryButton(c, i18n.T("sync.execute")).Padding(4, 10).Radius(5).FontSize(11).Disabled(a.busy || len(plan.Actions) == 0).OnClick(func() { a.confirmSync = true })
	})
	if len(plan.Actions) == 0 {
		ui.Column(c).Grow(1).FillWidth().Center().Gap(6).Children(func() {
			ui.Icon(c, iconCheck).Size(22, 22).TextColor(pal.successFg)
			ui.Text(c, i18n.T("sync.nothing")).FontSize(15).Bold()
		})
		return
	}
	ui.List(c, &a.directoryList, len(plan.Actions)).Grow(1).FillWidth().ItemKey(func(i int) any { item := plan.Actions[i]; return item.Kind + ":" + item.Path }).Rows(func(row ui.ListRow) {
		item := plan.Actions[row.Index]
		rc := row.Context
		actionText := i18n.T(map[string]string{"copy": "sync.copy", "delete": "sync.delete", "create-directory": "sync.mkdir", "remove-directory": "sync.rmdir"}[item.Kind])
		fg, bg := pal.baseAccent, pal.baseBg
		if item.Kind == "delete" || item.Kind == "remove-directory" {
			fg, bg = pal.dangerFg, pal.dangerBg
		} else if item.Kind == "create-directory" {
			fg, bg = pal.successFg, pal.successBg
		}
		ui.Row(rc).Height(34).Padding(0, 16).Gap(12).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder.Alpha(0.4)).Children(func() {
			ui.Row(rc).Width(80).Children(func() {
				statusPill(rc, actionText, fg, bg)
			})
			ui.Text(rc, item.Path).Grow(1).Bold().FontSize(12).SingleLine()
			if item.To != "" {
				ui.Text(rc, "→").FontSize(11).TextColor(t.TextMuted)
				ui.Text(rc, item.To).FontSize(11).TextColor(t.TextMuted).SingleLine()
			}
		})
	})
}

func (a *workspaceState) undoSync() {
	if a.busy || a.lastJournal == "" || a.syncRolledBack {
		return
	}
	journal := a.lastJournal
	roots := a.directoryRoots
	a.job(func(ctx context.Context) {
		_, err := operations.Restore(ctx, journal)
		a.publish(func() {
			a.busy = false
			if err != nil {
				a.err = err.Error()
				return
			}
			a.syncRolledBack = true
			a.directoryNote = i18n.T("sync.undone")
			if roots[0] != "" {
				a.loadDirectory(roots)
			} else {
				a.message = a.directoryNote
				a.directoryNote = ""
			}
		})
	})
}

func (a *workspaceState) executeSync() {
	if a.busy || a.syncPlan == nil {
		return
	}
	plan := *a.syncPlan
	backup := a.backupDir
	if backup == "" {
		a.err = i18n.T("err.noBackupDir")
		return
	}
	a.syncGen++
	runBackup := filepath.Join(backup, fmt.Sprintf("run-%d-%d", time.Now().UnixNano(), a.syncGen))
	a.job(func(ctx context.Context) {
		report, err := operations.Execute(ctx, plan, runBackup)
		a.publish(func() {
			a.busy = false
			a.lastJournal = report.Journal
			if err != nil {
				a.syncRolledBack = report.RolledBack
				if report.RolledBack {
					a.err = i18n.TArgs("sync.stoppedRolledBack", err, len(report.Undone))
					a.directoryNote = a.err
				} else {
					a.err = i18n.TArgs("sync.stoppedPartial", err, len(report.Completed), report.Journal)
				}
				a.loadDirectory(a.directoryRoots)
				return
			}
			a.syncRolledBack = false
			a.message = i18n.TArgs("sync.done", len(report.Completed), report.Journal)
			a.loadDirectory(a.directoryRoots)
		})
	})
}

func (a *workspaceState) previewDirMerge() {
	if a.busy || a.remoteDirectory() {
		return
	}
	roots := a.directoryRoots
	if roots[0] == "" || roots[1] == "" {
		return
	}
	a.job(func(ctx context.Context) {
		decisions, err := dirmerge.Two(ctx, roots[0], roots[1])
		a.publish(func() {
			a.busy = false
			if err != nil {
				a.err = err.Error()
				return
			}
			a.dirMergeDecisions = decisions
			a.directoryList = ui.ListState{}
			a.message = i18n.TArgs("dirmerge.previewDone", len(decisions))
		})
	})
}

func (a *workspaceState) dirMergePreviewView(c *ui.Context) {
	t := c.Theme()
	pal := paletteFor(t)
	decisions := a.dirMergeDecisions
	conflicts := 0
	for _, d := range decisions {
		if d.Kind == dirmerge.KindConflict {
			conflicts++
		}
	}
	ui.Row(c).Height(38).Padding(0, 16).Gap(10).Background(pal.cardBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		ui.Button(c, i18n.T("sync.back")).Padding(4, 9).Radius(5).FontSize(11).OnClick(func() { a.dirMergeDecisions = nil; a.directoryList = ui.ListState{} })
		statusPill(c, i18n.TArgs("dirmerge.items", len(decisions)), pal.baseAccent, pal.baseBg)
		if conflicts > 0 {
			statusPill(c, i18n.TArgs("dirmerge.conflicts", conflicts), pal.dangerFg, pal.dangerBg)
		} else {
			statusPill(c, i18n.T("dirmerge.clean"), pal.successFg, pal.successBg)
		}
		ui.Box(c).Grow(1)
	})
	if len(decisions) == 0 {
		ui.Column(c).Grow(1).FillWidth().Center().Gap(6).Children(func() {
			ui.Icon(c, iconCheck).Size(22, 22).TextColor(pal.successFg)
			ui.Text(c, i18n.T("dirmerge.empty")).FontSize(15).Bold()
		})
		return
	}
	ui.List(c, &a.directoryList, len(decisions)).Grow(1).FillWidth().ItemKey(func(i int) any { return decisions[i].Path }).Rows(func(row ui.ListRow) {
		item := decisions[row.Index]
		rc := row.Context
		fg, bg := pal.baseAccent, pal.baseBg
		switch item.Kind {
		case dirmerge.KindConflict:
			fg, bg = pal.dangerFg, pal.dangerBg
		case dirmerge.KindLeft:
			fg, bg = pal.leftAccent, pal.leftBg
		case dirmerge.KindRight:
			fg, bg = pal.rightAccent, pal.rightBg
		case dirmerge.KindDelete:
			fg, bg = pal.dangerFg, pal.dangerBg
		case dirmerge.KindMerged:
			fg, bg = pal.successFg, pal.successBg
		}
		rowBg := ui.Transparent
		if row.Index%2 == 1 {
			rowBg = pal.subtleSurface.Alpha(0.35)
		}
		ui.Row(rc).Height(34).Padding(0, 14).Gap(10).Background(rowBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder.Alpha(0.5)).Children(func() {
			statusPill(rc, i18n.T("dirmerge.kind."+item.Kind), fg, bg)
			ui.Text(rc, item.Path).FontSize(12).Grow(1).SingleLine()
			if reason := dirMergeReason(item.Reason); reason != "" {
				ui.Text(rc, reason).FontSize(11).TextColor(pal.dangerFg).SingleLine()
			}
		})
	})
}

// dirMergeReason 把 dirmerge 的冲突原因映射为界面文案。
func dirMergeReason(reason string) string {
	switch reason {
	case "binary conflict":
		return i18n.T("dirmerge.reason.binary")
	case "delete/modify conflict":
		return i18n.T("dirmerge.reason.deleteModify")
	}
	return reason
}

// remoteDirectory 报告当前目录比较是否有一侧是远程目录。
func (a *workspaceState) remoteDirectory() bool {
	return remote.IsURL(a.directoryRoots[0]) || remote.IsURL(a.directoryRoots[1])
}

// openSource 打开目录比较的一侧：远程地址建立一条可复用的连接，done 负责关闭它。
// 连接错误包装为 remoteInputError，以便界面询问密码或确认主机密钥后重试。
func openSource(ctx context.Context, p string, auth remoteAuth) (compare.Source, func(), error) {
	if !remote.IsURL(p) {
		return compare.LocalSource(p), func() {}, nil
	}
	store, dir, err := remote.ParseDirURL(p)
	if err != nil {
		return nil, nil, err
	}
	auth.apply(p, store)
	conn, err := remote.Connect(ctx, store)
	if err != nil {
		return nil, nil, &remoteInputError{raw: p, err: err}
	}
	return taggedSource{Source: compare.RemoteSource(conn, dir), raw: p}, func() { conn.Close() }, nil
}

// taggedSource 给扫描错误标上远程地址：WebDAV 等无状态协议直到扫描时才发出第一个请求，
// 认证与主机问题也在这时才出现，需要同样能触发密码询问。
type taggedSource struct {
	compare.Source
	raw string
}

func (s taggedSource) Scan(ctx context.Context, opt compare.ScanOptions) (map[string]*compare.Metadata, error) {
	m, err := s.Source.Scan(ctx, opt)
	if err != nil && !errors.Is(err, context.Canceled) {
		return nil, &remoteInputError{raw: s.raw, err: err}
	}
	return m, err
}
