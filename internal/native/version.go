package native

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/egoist/mygo/ui"
	"gocompare/internal/i18n"
	"gocompare/internal/pever"
	"gocompare/internal/regfile"
)

// versionResult 是一次 PE 版本或 .reg 文本比较的结果。kind 为 pe 或 reg。
type versionResult struct {
	kind        string
	left, right string
	diffs       []string
}

// loadVersion 在后台读取两侧文件。两侧都以注册表 5.00 头开始时比较 .reg 文本，否则读取 PE 的 FileVersion。
// 不访问实时注册表。失败时清掉上一次的版本和差异，不交给文本比较。
func (a *workspaceState) loadVersion() {
	paths := a.paths
	a.infoGen++
	gen := a.infoGen
	a.job(func(ctx context.Context) {
		left, err := readVersionFile(paths[0])
		var right []byte
		if err == nil {
			right, err = readVersionFile(paths[1])
		}
		var result versionResult
		if err == nil && ctx.Err() != nil {
			err = ctx.Err()
		}
		if err == nil {
			result, err = compareVersionBytes(left, right)
		}
		a.publish(func() {
			if gen != a.infoGen {
				return
			}
			a.busy = false
			if err != nil {
				a.err = err.Error()
				a.clearVersion()
				return
			}
			a.infoKind = result.kind
			a.infoLeft = result.left
			a.infoRight = result.right
			a.infoDiffs = result.diffs
			a.infoReady = true
			a.err = ""
			a.message = i18n.T("version.loaded")
		})
	})
}

func (a *workspaceState) clearVersion() {
	a.infoKind = ""
	a.infoLeft = ""
	a.infoRight = ""
	a.infoDiffs = nil
	a.infoReady = false
}

// versionView 只展示已经读到的 FileVersion 或 .reg 差异。实时注册表不在这里读取。
func (a *workspaceState) versionView(c *ui.Context) {
	t := c.Theme()
	pal := paletteFor(t)

	ui.Row(c).Height(36).Padding(0, 16).Gap(8).Background(pal.cardBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		ui.Icon(c, iconVersionReg).Size(13, 13).TextColor(pal.baseAccent)
		ui.Text(c, i18n.T("version.header")).FontSize(11).Bold()
		statusPill(c, i18n.T("version.noLive"), pal.sameFg, pal.sameBg)
	})

	if !a.infoReady {
		if a.err == "" {
			a.modeEmptyView(c, pal, 5)
		}
		return
	}

	ui.Column(c).Grow(1).FillWidth().Padding(14, 16).Gap(12).Children(func() {
		if a.infoKind == "pe" {
			same := a.infoLeft == a.infoRight
			note := i18n.T("version.different")
			fg, bg := pal.diffFg, pal.diffBg
			if same {
				note = i18n.T("version.same")
				fg, bg = pal.successFg, pal.successBg
			}
			ui.Row(c).Padding(8, 12).Gap(8).Radius(6).Background(bg).Children(func() {
				ui.Icon(c, iconCheck).Size(14, 14).TextColor(fg)
				ui.Text(c, note).Bold().FontSize(12).TextColor(fg)
			})
			ui.Row(c).FillWidth().Gap(12).Children(func() {
				ui.Column(c).Grow(1).Basis(0).Padding(14, 16).Gap(6).Radius(8).Background(pal.cardBg).Border(1, pal.cardBorder).Children(func() {
					sideBadge(c, 0, pal)
					ui.Text(c, i18n.TArgs("version.left", a.infoLeft)).FontSize(14).Bold()
				})
				ui.Column(c).Grow(1).Basis(0).Padding(14, 16).Gap(6).Radius(8).Background(pal.cardBg).Border(1, pal.cardBorder).Children(func() {
					sideBadge(c, 1, pal)
					ui.Text(c, i18n.TArgs("version.right", a.infoRight)).FontSize(14).Bold()
				})
			})
			return
		}

		if len(a.infoDiffs) == 0 {
			ui.Row(c).Padding(10, 14).Gap(8).Radius(6).Background(pal.successBg).Children(func() {
				ui.Icon(c, iconCheck).Size(14, 14).TextColor(pal.successFg)
				ui.Text(c, i18n.T("version.regSame")).Bold().FontSize(13).TextColor(pal.successFg)
			})
			return
		}

		ui.Column(c).FillWidth().Padding(12, 14).Gap(6).Radius(8).Background(pal.cardBg).Border(1, pal.cardBorder).Children(func() {
			for _, line := range a.infoDiffs {
				ui.Row(c).FillWidth().Padding(6, 10).Gap(6).Radius(5).Background(pal.diffBg).Children(func() {
					ui.Icon(c, iconAlert).Size(13, 13).TextColor(pal.diffFg)
					ui.Text(c, i18n.TArgs("version.diffLine", line)).Grow(1).FontSize(12).TextColor(pal.diffFg)
				})
			}
		})
	})
}

func compareVersionBytes(left, right []byte) (versionResult, error) {
	leftReg, rightReg := looksLikeRegistry(left), looksLikeRegistry(right)
	if leftReg || rightReg {
		if !leftReg || !rightReg {
			return versionResult{}, errors.New(i18n.T("version.mixed"))
		}
		lv, err := regfile.Parse(string(left))
		if err != nil {
			return versionResult{}, err
		}
		rv, err := regfile.Parse(string(right))
		if err != nil {
			return versionResult{}, err
		}
		return versionResult{kind: "reg", diffs: regfile.Diff(lv, rv)}, nil
	}
	lv, err := pever.FileVersion(left)
	if err != nil {
		return versionResult{}, err
	}
	rv, err := pever.FileVersion(right)
	if err != nil {
		return versionResult{}, err
	}
	return versionResult{kind: "pe", left: lv, right: rv}, nil
}

// looksLikeRegistry 只看第一条实质内容是否为 5.00 头，用来选择比较器。它不读取注册表。
func looksLikeRegistry(data []byte) bool {
	s := bufio.NewScanner(bytes.NewReader(data))
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		return line == "Windows Registry Editor Version 5.00"
	}
	return false
}

func readVersionFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	const max = 16 << 20
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if len(data) > max {
		return nil, fmt.Errorf("%s exceeds 16 MiB", path)
	}
	return data, nil
}
