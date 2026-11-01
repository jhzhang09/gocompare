package native

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/egoist/mygo/ui"
	"gocompare/internal/i18n"
	"gocompare/internal/table"
)

func (a *workspaceState) loadTable() {
	paths := a.paths
	a.tableGen++
	gen := a.tableGen
	a.job(func(ctx context.Context) {
		leftData, err := readTableFile(paths[0])
		var rightData []byte
		if err == nil {
			rightData, err = readTableFile(paths[1])
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
				a.err = err.Error()
				a.clearTable()
				return
			}
			a.tableSheets = [2][]table.Sheet{leftSheets, rightSheets}
			a.tableDiffs = diffs
			a.tableReady = true
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
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (32<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 32<<20 {
		return nil, fmt.Errorf("table exceeds 32 MiB")
	}
	return data, nil
}

func (a *workspaceState) clearTable() {
	a.tableSheets = [2][]table.Sheet{}
	a.tableDiffs = nil
	a.tableReady = false
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

		// 差异明细列表
		if len(a.tableDiffs) > 0 {
			ui.Column(c).Grow(1).FillWidth().Padding(8, 12).Gap(6).Radius(8).Background(pal.cardBg).Border(1, pal.cardBorder).Children(func() {
				ui.Text(c, i18n.T("table.details")).FontSize(11).Bold().TextColor(t.TextMuted)
				maxShow := 50
				if len(a.tableDiffs) < maxShow {
					maxShow = len(a.tableDiffs)
				}
				for i := 0; i < maxShow; i++ {
					d := a.tableDiffs[i]
					ui.Row(c).Padding(4, 6).Gap(8).Radius(4).Background(pal.subtleSurface).Children(func() {
						tagBg, tagFg := pal.diffBg, pal.diffFg
						statusPill(c, d.Sheet, pal.baseAccent, pal.baseBg)
						if d.Kind == "changed" {
							statusPill(c, i18n.TArgs("table.cell", displayKey(d.Key), d.Column+1), tagFg, tagBg)
							ui.Text(c, fmt.Sprintf("%q ≠ %q", d.Left, d.Right)).FontSize(11)
						} else {
							statusPill(c, i18n.T("table.kind."+d.Kind), tagFg, tagBg)
							ui.Text(c, displayKey(d.Key)).FontSize(11)
						}
					})
				}
				if len(a.tableDiffs) > maxShow {
					ui.Text(c, i18n.TArgs("table.more", len(a.tableDiffs)-maxShow)).FontSize(11).TextColor(t.TextMuted)
				}
			})
		}
	})
}

// displayKey 把复合键内部的 \x1f 分隔符换成可读的逗号。
func displayKey(key string) string {
	return strings.ReplaceAll(key, "\x1f", ", ")
}
