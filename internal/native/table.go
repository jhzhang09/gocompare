package native

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"gocompare/internal/i18n"
	"gocompare/internal/table"
)

func (a *workspaceState) loadTable() {
	paths := a.paths
	a.tableGen++
	gen := a.tableGen
	auth := a.takeURLPasswords()
	a.job(func(ctx context.Context) {
		leftData, err := readInputBytes(ctx, paths[0], 32<<20, auth)
		var rightData []byte
		if err == nil {
			rightData, err = readInputBytes(ctx, paths[1], 32<<20, auth)
		}
		var leftSheets, rightSheets []table.Sheet
		var diffs []table.Diff
		if err == nil {
			leftSheets, err = table.ParseFile(paths[0], leftData)
		}
		if err == nil {
			rightSheets, err = table.ParseFile(paths[1], rightData)
		}
		if err == nil {
			diffs = table.CompareSheets(leftSheets, rightSheets, table.Options{})
		}
		if err == nil && ctx.Err() != nil {
			err = ctx.Err()
		}
		a.publish(func() {
			if gen != a.tableGen {
				return
			}
			a.busy = false
			if err != nil {
				if !a.handleRemoteError(err, a.load) {
					a.err = err.Error()
				}
				a.clearTable()
				return
			}
			a.tableSheets = [2][]table.Sheet{leftSheets, rightSheets}
			a.tableDiffs = diffs
			a.tableReady = true
			a.tableSelectedSheet = ""
			a.tablePage = 0
			a.err = ""
			if len(diffs) == 0 {
				a.message = i18n.T("table.same")
			} else {
				a.message = i18n.TArgs("table.diffCount", len(diffs))
			}
		})
	})
}

func readTableFile(path string) ([]byte, error) {
	return readInputBytes(context.Background(), path, 32<<20, remoteAuth{})
}

func (a *workspaceState) clearTable() {
	a.tableSheets = [2][]table.Sheet{}
	a.tableDiffs = nil
	a.tableReady = false
	a.tableSelectedSheet = ""
	a.tablePage = 0
}

type tableSheetTab struct {
	name      string
	diffCount int
	isLeft    bool
	isRight   bool
}

func collectSheetTabs(sheets [2][]table.Sheet, diffs []table.Diff) []tableSheetTab {
	var tabs []tableSheetTab
	idxMap := make(map[string]int)

	for _, s := range sheets[0] {
		idxMap[s.Name] = len(tabs)
		tabs = append(tabs, tableSheetTab{name: s.Name, isLeft: true})
	}
	for _, s := range sheets[1] {
		if idx, ok := idxMap[s.Name]; ok {
			tabs[idx].isRight = true
		} else {
			idxMap[s.Name] = len(tabs)
			tabs = append(tabs, tableSheetTab{name: s.Name, isRight: true})
		}
	}
	for _, d := range diffs {
		if idx, ok := idxMap[d.Sheet]; ok {
			tabs[idx].diffCount++
		}
	}
	return tabs
}

func formatCellCoord(key string, col int) string {
	rowIdx, err := strconv.Atoi(key)
	colLetter := table.ColLetter(col)
	if err == nil {
		rowNum := rowIdx + 1
		colNum := col + 1
		ref := fmt.Sprintf("%s%d", colLetter, rowNum)
		return i18n.TArgs("table.cellCoord", ref, rowNum, colNum)
	}
	return i18n.TArgs("table.cellKey", displayKey(key), col+1, colLetter)
}

func (a *workspaceState) tableView(c *ui.Context) {
	t := c.Theme()
	pal := paletteFor(t)

	ui.Row(c).Height(36).Padding(0, 16).Gap(8).Background(pal.cardBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		ui.Icon(c, iconTable).Size(13, 13).TextColor(pal.baseAccent)
		ui.Text(c, i18n.T("mode.table")).FontSize(11).Bold()
		if a.tableReady {
			if len(a.tableDiffs) == 0 {
				statusPill(c, i18n.T("table.same"), pal.sameFg, pal.sameBg)
			} else {
				statusPill(c, i18n.TArgs("table.diffCount", len(a.tableDiffs)), pal.diffFg, pal.diffBg)
			}
		}
	})

	if !a.tableReady {
		if a.err == "" {
			a.modeEmptyView(c, pal, kindTable)
		}
		return
	}

	ui.Column(c).Grow(1).FillWidth().Padding(14, 16).Gap(12).Children(func() {
		// 状态概览卡片
		ui.Row(c).FillWidth().Padding(10, 14).Gap(12).Radius(8).Background(pal.cardBg).Border(1, pal.cardBorder).Children(func() {
			ui.Text(c, i18n.TArgs("table.sheetCount", len(a.tableSheets[0]), len(a.tableSheets[1]))).FontSize(12)
			ui.Box(c).Grow(1)
			if len(a.tableDiffs) == 0 {
				ui.Text(c, i18n.T("table.same")).FontSize(12).TextColor(pal.sameFg).Bold()
			} else {
				ui.Text(c, i18n.TArgs("table.diffCount", len(a.tableDiffs))).FontSize(12).TextColor(pal.diffFg).Bold()
			}
		})

		// 工作表标签栏（若存在多张工作表）
		tabs := collectSheetTabs(a.tableSheets, a.tableDiffs)
		if len(tabs) > 1 {
			ui.Row(c).FillWidth().Gap(6).Children(func() {
				// 全部工作表 Tab
				allActive := a.tableSelectedSheet == ""
				allBg := pal.cardBg
				allBorder := pal.cardBorder
				allFg := t.Text
				if allActive {
					allBg = pal.baseAccent
					allBorder = pal.baseAccent
					allFg = t.AccentText
				}
				ui.Row(c).Padding(4, 10).Radius(6).Background(allBg).Border(1, allBorder).OnClick(func() {
					a.tableSelectedSheet = ""
					a.tablePage = 0
				}).Children(func() {
					ui.Text(c, i18n.TArgs("table.allSheets", len(a.tableDiffs))).FontSize(11).Bold().TextColor(allFg)
				})

				// 各 Sheet Tab
				for _, tab := range tabs {
					tabName := tab.name
					isActive := a.tableSelectedSheet == tabName
					tabBg := pal.cardBg
					tabBorder := pal.cardBorder
					tabFg := t.Text
					if isActive {
						tabBg = pal.baseAccent
						tabBorder = pal.baseAccent
						tabFg = t.AccentText
					}
					tabItem := tab
					ui.Row(c).Padding(4, 10).Gap(6).Radius(6).Background(tabBg).Border(1, tabBorder).OnClick(func() {
						a.tableSelectedSheet = tabName
						a.tablePage = 0
					}).Children(func() {
						ui.Text(c, tabName).FontSize(11).Bold().TextColor(tabFg)
						if tabItem.diffCount == 0 {
							statusPill(c, i18n.T("table.sheetNoDiff"), pal.sameFg, pal.sameBg)
						} else {
							statusPill(c, strconv.Itoa(tabItem.diffCount), pal.diffFg, pal.diffBg)
						}
					})
				}
			})
		}

		// 过滤当前选中工作表的差异
		var filtered []table.Diff
		for _, d := range a.tableDiffs {
			if a.tableSelectedSheet == "" || d.Sheet == a.tableSelectedSheet {
				filtered = append(filtered, d)
			}
		}

		// 分页计算
		const pageSize = 50
		totalCount := len(filtered)
		totalPages := (totalCount + pageSize - 1) / pageSize
		if totalPages == 0 {
			totalPages = 1
		}
		if a.tablePage >= totalPages {
			a.tablePage = totalPages - 1
		}
		if a.tablePage < 0 {
			a.tablePage = 0
		}
		start := a.tablePage * pageSize
		end := start + pageSize
		if end > totalCount {
			end = totalCount
		}

		// 差异明细列表与分页控件
		if totalCount > 0 {
			ui.Column(c).Grow(1).FillWidth().Padding(10, 12).Gap(8).Radius(8).Background(pal.cardBg).Border(1, pal.cardBorder).Children(func() {
				ui.Row(c).FillWidth().Children(func() {
					ui.Text(c, i18n.T("table.details")).FontSize(11).Bold().TextColor(t.TextMuted)
					ui.Box(c).Grow(1)
					if totalPages > 1 {
						ui.Text(c, i18n.TArgs("table.pageInfo", a.tablePage+1, totalPages, totalCount)).FontSize(11).TextColor(t.TextMuted)
					}
				})

				for i := start; i < end; i++ {
					a.renderDiffCard(c, filtered[i], pal, t)
				}

				if totalPages > 1 {
					ui.Row(c).FillWidth().Padding(4, 0).Gap(8).Children(func() {
						ui.Box(c).Grow(1)
						if a.tablePage > 0 {
							ui.Button(c, i18n.T("table.pagePrev")).FontSize(11).OnClick(func() {
								a.tablePage--
							})
						}
						if a.tablePage < totalPages-1 {
							ui.Button(c, i18n.T("table.pageNext")).FontSize(11).OnClick(func() {
								a.tablePage++
							})
						}
					})
				}
			})
		}
	})
}

func (a *workspaceState) renderDiffCard(c *ui.Context, d table.Diff, pal uiPalette, t *ui.Theme) {
	tagBg, tagFg := pal.diffBg, pal.diffFg
	ui.Column(c).FillWidth().Padding(8, 10).Gap(6).Radius(6).Background(pal.subtleSurface).Border(1, pal.cardBorder).Children(func() {
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			statusPill(c, d.Sheet, pal.baseAccent, pal.baseBg)
			if d.Kind == "changed" {
				statusPill(c, formatCellCoord(d.Key, d.Column), tagFg, tagBg)
			} else {
				statusPill(c, i18n.T("table.kind."+d.Kind), tagFg, tagBg)
			}
		})

		switch d.Kind {
		case "changed":
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				// 左侧单元格
				ui.Column(c).Grow(1).Basis(0).Padding(6, 8).Radius(4).Background(pal.leftBg).Border(1, pal.leftAccent.Alpha(0.20)).Children(func() {
					ui.Row(c).Gap(6).Children(func() {
						sideBadge(c, 0, pal)
					})
					if d.Left == "" {
						ui.Text(c, i18n.T("table.valEmpty")).FontSize(11).TextColor(t.TextMuted)
					} else {
						ui.Text(c, d.Left).FontSize(11).TextColor(t.Text)
					}
				})

				// 差异指示
				ui.Column(c).Shrink(0).Padding(10, 2).Children(func() {
					ui.Text(c, "≠").FontSize(13).Bold().TextColor(pal.diffFg)
				})

				// 右侧单元格
				ui.Column(c).Grow(1).Basis(0).Padding(6, 8).Radius(4).Background(pal.rightBg).Border(1, pal.rightAccent.Alpha(0.20)).Children(func() {
					ui.Row(c).Gap(6).Children(func() {
						sideBadge(c, 1, pal)
					})
					if d.Right == "" {
						ui.Text(c, i18n.T("table.valEmpty")).FontSize(11).TextColor(t.TextMuted)
					} else {
						ui.Text(c, d.Right).FontSize(11).TextColor(t.Text)
					}
				})
			})
		case "sheet_left_only":
			ui.Text(c, i18n.T("table.sheetMissingRight")).FontSize(11).TextColor(pal.leftAccent)
		case "sheet_right_only":
			ui.Text(c, i18n.T("table.sheetMissingLeft")).FontSize(11).TextColor(pal.rightAccent)
		default:
			ui.Text(c, displayKey(d.Key)).FontSize(11).TextColor(t.Text)
		}
	})
}

// displayKey 把复合键内部的 \x1f 分隔符换成可读的逗号。
func displayKey(key string) string {
	return strings.ReplaceAll(key, "\x1f", ", ")
}
