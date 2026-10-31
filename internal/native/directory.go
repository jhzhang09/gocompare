package native

import (
	"context"
	"fmt"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"gocompare/internal/compare"
	"gocompare/internal/operations"
	"path/filepath"
	"strings"
	"time"
)

func (a *workspaceState) loadDirectory(paths [2]string) {
	a.scanSeen = nil
	a.scanPath = ""
	filter, mode, mtime, follow := a.dirFilter, a.compareMode, a.compareTime, a.followLinks
	a.job(func(ctx context.Context) {
		lastProgress := time.Time{}
		links := compare.LinkRecord
		if follow {
			links = compare.LinkFollow
		}
		rows, err := compare.CompareDirectories(ctx, paths[0], paths[1], compare.ScanOptions{
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
		a.publish(func() {
			a.busy = false
			if err != nil {
				a.err = err.Error()
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
	if a.syncChoice == "" {
		a.syncChoice = "向右更新"
	}
	a.directoryRows = rows
	a.directoryRoots = paths
	a.paths = paths
	a.kind = 1
	a.directoryList = ui.ListState{}
	if a.directoryNote != "" {
		a.message = a.directoryNote
		a.directoryNote = ""
	} else {
		a.message = fmt.Sprintf("目录比较完成 · %d 个条目", len(rows))
	}
	if a.win != nil {
		a.win.SetTitle("目录比较 — GoCompare")
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
			a.err = "该条目不是可比较的普通文件"
		}
		return
	}
	a.paths = [2]string{filepath.Join(a.directoryRoots[0], filepath.FromSlash(entry.Path)), filepath.Join(a.directoryRoots[1], filepath.FromSlash(entry.Path))}
	a.allowMissing = true
	a.kind = 0
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
	t := c.Theme()
	rows := a.visibleDirectoryRows()
	ui.Row(c).Padding(10, 18).Gap(10).Children(func() {
		ui.TextInput(c.Key("search-files"), &a.dirSearch).Placeholder("搜索文件名…").Label("搜索文件名").Grow(1)
		ui.Checkbox(c, &a.onlyDifferences, "仅显示差异")
		ui.Text(c, fmt.Sprintf("%d 项", len(rows))).TextColor(t.TextMuted)
	})
	if a.optionsOpen {
		ui.Row(c).Padding(5, 18).Gap(8).Children(func() {
			ui.Select(c, &a.syncChoice, []string{"向右更新", "向左更新", "双向更新", "镜像到右侧", "镜像到左侧"}).Disabled(a.busy)
			ui.Button(c, "同步预览").Disabled(a.busy).OnClick(a.previewSync)
			if a.lastJournal != "" && !a.syncRolledBack {
				ui.Button(c, "撤销上次同步").Disabled(a.busy).OnClick(a.undoSync)
			}
			if a.lastJournal != "" {
				ui.Button(c, "查看操作记录…").OnClick(func() {
					if err := mygo.Shell.OpenPath(a.lastJournal); err != nil {
						a.err = err.Error()
					}
				})
			}
		})
	}
	ui.Row(c).Padding(8, 18).Gap(12).Background(t.Surface).Children(func() {
		ui.Text(c, "左侧 · 名称 / 大小").Grow(1).Basis(0).Bold()
		ui.Text(c, "比较结果").Width(82).Bold()
		ui.Text(c, "右侧 · 名称 / 大小").Grow(1).Basis(0).Bold()
	})
	if len(rows) == 0 {
		ui.Column(c).Grow(1).FillWidth().Center().Gap(8).Children(func() {
			ui.Text(c, "没有符合条件的条目").FontSize(18).Bold()
			ui.Text(c, "尝试关闭差异筛选或清空搜索。").TextColor(t.TextMuted)
		})
		return
	}
	ui.List(c, &a.directoryList, len(rows)).Grow(1).FillWidth().ItemKey(func(i int) any { return rows[i].Path }).Rows(func(row ui.ListRow) {
		entry := rows[row.Index]
		c := row.Context
		color := t.TextMuted
		switch entry.Status {
		case compare.Different:
			color = ui.Hex("#b67818")
		case compare.LeftOnly:
			color = ui.Hex("#27846b")
		case compare.RightOnly:
			color = ui.Hex("#8263b8")
		case compare.Failed, compare.TypeChanged:
			color = ui.Hex("#c05245")
		}
		ui.Row(c).Height(36).Padding(0, 18).Gap(12).BorderWidth(0, 0, 1, 0).BorderColor(t.Border.Alpha(0.4)).Children(func() {
			for side, m := range []*compare.Metadata{entry.Left, entry.Right} {
				if side == 1 {
					ui.Text(c, statusLabel(entry.Status)).Width(82).FontSize(11).TextColor(color)
				}
				ui.Row(c).Grow(1).Basis(0).MinWidth(0).Gap(8).Children(func() {
					if m == nil {
						ui.Text(c, "—").Grow(1).TextColor(t.TextMuted)
						return
					}
					prefix := ""
					if m.Directory {
						prefix = "▾ "
						if a.closedDirs[entry.Path] {
							prefix = "▸ "
						}
					} else {
						prefix = "· "
					}
					ui.Text(c, prefix).Width(14).Margin(0, 0, 0, float32(strings.Count(entry.Path, "/")*12)).TextColor(color)
					ui.Button(c, filepath.Base(entry.Path)).Label(entry.Path).Grow(1).MinWidth(0).Justify(ui.Start).BorderWidth(0).Background(ui.Transparent).Tooltip(entry.Path + "\n" + entry.Error).Disabled(a.busy).OnClick(func() { a.openDirectoryEntry(entry) })
					ui.Text(c, metadataLabel(m)).Width(66).FontSize(11).TextColor(t.TextMuted)
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
		return "读取失败"
	}
	if m.Symlink {
		return "符号链接"
	}
	if m.Directory {
		return "目录"
	}
	return fmt.Sprintf("%d B", m.Size)
}
func statusLabel(s compare.Status) string {
	return map[compare.Status]string{compare.Same: "相同", compare.Different: "不同", compare.LeftOnly: "仅左侧", compare.RightOnly: "仅右侧", compare.TypeChanged: "类型不同", compare.Failed: "读取失败"}[s]
}

func (a *workspaceState) previewSync() {
	if a.busy {
		return
	}
	if a.directoryOptions.Filter != "" || a.directoryOptions.CompareMode || a.directoryOptions.CompareTime || a.directoryOptions.Links == compare.LinkFollow {
		a.err = "同步暂不支持当前过滤、属性或链接规则。请恢复默认规则并重新比较。"
		return
	}
	roots := a.directoryRoots
	mode := map[string]operations.Mode{"向右更新": operations.UpdateRight, "向左更新": operations.UpdateLeft, "双向更新": operations.UpdateBoth, "镜像到右侧": operations.MirrorRight, "镜像到左侧": operations.MirrorLeft}[a.syncChoice]
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
			a.message = "操作预览 · 文件尚未更改"
		})
	})
}
func (a *workspaceState) syncPreviewView(c *ui.Context) {
	t := c.Theme()
	plan := a.syncPlan
	ui.Row(c).Padding(8, 20).Gap(12).Children(func() {
		ui.Button(c, "返回比较结果").OnClick(func() { a.syncPlan = nil; a.directoryList = ui.ListState{} })
		ui.Text(c, fmt.Sprintf("%d 个待执行操作", len(plan.Actions))).Bold()
		ui.PrimaryButton(c, "执行同步…").Disabled(a.busy || len(plan.Actions) == 0).OnClick(func() { a.confirmSync = true })
	})
	if len(plan.Actions) == 0 {
		ui.Column(c).Grow(1).FillWidth().Center().Children(func() { ui.Text(c, "无需操作") })
		return
	}
	ui.List(c, &a.directoryList, len(plan.Actions)).Grow(1).FillWidth().ItemKey(func(i int) any { item := plan.Actions[i]; return item.Kind + ":" + item.Path }).Rows(func(row ui.ListRow) {
		item := plan.Actions[row.Index]
		ui.Row(row.Context).Padding(8, 20).Gap(16).Children(func() {
			ui.Text(row.Context, map[string]string{"copy": "复制", "delete": "删除", "create-directory": "创建目录", "remove-directory": "删除目录"}[item.Kind]).Width(90).Bold()
			ui.Text(row.Context, item.Path).Grow(1)
			ui.Text(row.Context, item.To).FontSize(12).TextColor(t.TextMuted)
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
			a.directoryNote = "已按操作记录撤销同步"
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
		a.err = "未配置备份目录，拒绝执行同步"
		return
	}
	a.job(func(ctx context.Context) {
		report, err := operations.Execute(ctx, plan, backup)
		a.publish(func() {
			a.busy = false
			a.lastJournal = report.Journal
			if err != nil {
				a.syncRolledBack = report.RolledBack
				if report.RolledBack {
					a.err = fmt.Sprintf("执行停止：%v；已回滚 %d 项", err, len(report.Undone))
					a.directoryNote = a.err
				} else {
					a.err = fmt.Sprintf("执行停止：%v；仍有 %d 项生效，记录：%s", err, len(report.Completed), report.Journal)
				}
				a.loadDirectory(a.directoryRoots)
				return
			}
			a.syncRolledBack = false
			a.message = fmt.Sprintf("已完成 %d 项操作；记录：%s", len(report.Completed), report.Journal)
			a.loadDirectory(a.directoryRoots)
		})
	})
}
