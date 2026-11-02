package native

import (
	"bufio"
	"context"
	"errors"
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
	leftPE      pever.Info
	rightPE     pever.Info
	diffs       []string
	regDiffs    []regfile.DiffEntry
}

// loadVersion 在后台读取两侧文件。两侧都以注册表头开始时比较 .reg 文本，否则读取 PE 的版本与元数据。
// 不访问实时注册表。失败时清掉上一次的版本和差异，不交给文本比较。
func (a *workspaceState) loadVersion() {
	paths := a.paths
	a.infoGen++
	gen := a.infoGen
	auth := a.takeURLPasswords()
	a.job(func(ctx context.Context) {
		left, err := readInputBytes(ctx, paths[0], 128<<20, auth)
		var right []byte
		if err == nil {
			right, err = readInputBytes(ctx, paths[1], 128<<20, auth)
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
				if !a.handleRemoteError(err, a.load) {
					a.err = err.Error()
				}
				a.clearVersion()
				return
			}
			a.infoKind = result.kind
			a.infoLeft = result.left
			a.infoRight = result.right
			a.infoLeftPE = result.leftPE
			a.infoRightPE = result.rightPE
			a.infoDiffs = result.diffs
			a.infoRegDiffs = result.regDiffs
			a.infoRegPage = 0
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
	a.infoLeftPE = pever.Info{}
	a.infoRightPE = pever.Info{}
	a.infoDiffs = nil
	a.infoRegDiffs = nil
	a.infoRegPage = 0
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
				ui.Column(c).Grow(1).Basis(0).Padding(14, 16).Gap(8).Radius(8).Background(pal.cardBg).Border(1, pal.cardBorder).Children(func() {
					ui.Row(c).Gap(6).Children(func() {
						sideBadge(c, 0, pal)
						if a.infoLeftPE.Architecture != "" {
							statusPill(c, a.infoLeftPE.Architecture, pal.baseAccent, pal.baseBg)
						}
					})
					ui.Text(c, i18n.TArgs("version.left", a.infoLeft)).FontSize(14).Bold()
					a.peMetaRows(c, pal, a.infoLeftPE, a.infoRightPE)
				})
				ui.Column(c).Grow(1).Basis(0).Padding(14, 16).Gap(8).Radius(8).Background(pal.cardBg).Border(1, pal.cardBorder).Children(func() {
					ui.Row(c).Gap(6).Children(func() {
						sideBadge(c, 1, pal)
						if a.infoRightPE.Architecture != "" {
							statusPill(c, a.infoRightPE.Architecture, pal.baseAccent, pal.baseBg)
						}
					})
					ui.Text(c, i18n.TArgs("version.right", a.infoRight)).FontSize(14).Bold()
					a.peMetaRows(c, pal, a.infoRightPE, a.infoLeftPE)
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

		const pageSize = 50
		totalCount := len(a.infoDiffs)
		totalPages := (totalCount + pageSize - 1) / pageSize
		if totalPages == 0 {
			totalPages = 1
		}
		if a.infoRegPage >= totalPages {
			a.infoRegPage = totalPages - 1
		}
		if a.infoRegPage < 0 {
			a.infoRegPage = 0
		}
		start := a.infoRegPage * pageSize
		end := start + pageSize
		if end > totalCount {
			end = totalCount
		}

		ui.Column(c).FillWidth().Padding(12, 14).Gap(6).Radius(8).Background(pal.cardBg).Border(1, pal.cardBorder).Children(func() {
			if totalPages > 1 {
				ui.Row(c).FillWidth().Children(func() {
					ui.Box(c).Grow(1)
					ui.Text(c, i18n.TArgs("table.pageInfo", a.infoRegPage+1, totalPages, totalCount)).FontSize(11).TextColor(t.TextMuted)
				})
			}
			for i := start; i < end; i++ {
				line := a.infoDiffs[i]
				kind := ""
				if i < len(a.infoRegDiffs) {
					kind = a.infoRegDiffs[i].Kind
				}
				ui.Row(c).FillWidth().Padding(6, 10).Gap(6).Radius(5).Background(pal.diffBg).Children(func() {
					ui.Icon(c, iconAlert).Size(13, 13).TextColor(pal.diffFg)
					ui.Text(c, i18n.TArgs("version.diffLine", line)).Grow(1).FontSize(12).TextColor(pal.diffFg)
					if kind != "" {
						kFg, kBg := pal.diffFg, pal.cardBg
						if kind == "left_only" {
							kFg, kBg = pal.leftAccent, pal.leftBg
						} else if kind == "right_only" {
							kFg, kBg = pal.rightAccent, pal.rightBg
						}
						statusPill(c, i18n.T("version.kind."+kind), kFg, kBg)
					}
				})
			}
			if totalPages > 1 {
				ui.Row(c).FillWidth().Padding(4, 0).Gap(8).Children(func() {
					ui.Box(c).Grow(1)
					if a.infoRegPage > 0 {
						ui.Button(c, i18n.T("table.pagePrev")).FontSize(11).OnClick(func() {
							a.infoRegPage--
						})
					}
					if a.infoRegPage < totalPages-1 {
						ui.Button(c, i18n.T("table.pageNext")).FontSize(11).OnClick(func() {
							a.infoRegPage++
						})
					}
				})
			}
		})
	})
}

func (a *workspaceState) peMetaRows(c *ui.Context, pal uiPalette, self, other pever.Info) {
	if self.Architecture != "" || other.Architecture != "" {
		a.mediaFieldRow(c, pal, i18n.TArgs("version.arch", self.Architecture), self.Architecture != other.Architecture)
	}
	if self.ProductName != "" || other.ProductName != "" {
		a.mediaFieldRow(c, pal, i18n.TArgs("version.productName", self.ProductName), self.ProductName != other.ProductName)
	}
	if self.FileDescription != "" || other.FileDescription != "" {
		a.mediaFieldRow(c, pal, i18n.TArgs("version.fileDesc", self.FileDescription), self.FileDescription != other.FileDescription)
	}
	if self.CompanyName != "" || other.CompanyName != "" {
		a.mediaFieldRow(c, pal, i18n.TArgs("version.company", self.CompanyName), self.CompanyName != other.CompanyName)
	}
	if self.LegalCopyright != "" || other.LegalCopyright != "" {
		a.mediaFieldRow(c, pal, i18n.TArgs("version.copyright", self.LegalCopyright), self.LegalCopyright != other.LegalCopyright)
	}
	if self.SHA256 != "" || other.SHA256 != "" {
		a.mediaFieldRow(c, pal, i18n.TArgs("version.sha256", self.SHA256, self.Size), self.SHA256 != other.SHA256 || self.Size != other.Size)
	}
}

func compareVersionBytes(left, right []byte) (versionResult, error) {
	leftText, leftErr := regfile.Decode(left)
	rightText, rightErr := regfile.Decode(right)
	leftReg := leftErr == nil && looksLikeRegistry(leftText)
	rightReg := rightErr == nil && looksLikeRegistry(rightText)
	if leftReg || rightReg {
		if !leftReg || !rightReg {
			return versionResult{}, errors.New(i18n.T("version.mixed"))
		}
		lv, err := regfile.Parse(leftText)
		if err != nil {
			return versionResult{}, err
		}
		rv, err := regfile.Parse(rightText)
		if err != nil {
			return versionResult{}, err
		}
		details := regfile.DiffDetails(lv, rv)
		diffs := make([]string, len(details))
		for i, d := range details {
			diffs[i] = d.Path
		}
		return versionResult{kind: "reg", diffs: diffs, regDiffs: details}, nil
	}
	lInfo, err := pever.Inspect(left)
	if err != nil {
		return versionResult{}, err
	}
	rInfo, err := pever.Inspect(right)
	if err != nil {
		return versionResult{}, err
	}
	return versionResult{
		kind:    "pe",
		left:    lInfo.FileVersion,
		right:   rInfo.FileVersion,
		leftPE:  lInfo,
		rightPE: rInfo,
	}, nil
}

// looksLikeRegistry 只看第一条实质内容是否为合法 .reg 头（5.00 或 REGEDIT4），用来选择比较器。它不读取注册表。
func looksLikeRegistry(text string) bool {
	s := bufio.NewScanner(strings.NewReader(text))
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		return regfile.IsHeader(line)
	}
	return false
}

func readVersionFile(path string) ([]byte, error) {
	return readInputBytes(context.Background(), path, 128<<20, remoteAuth{})
}
