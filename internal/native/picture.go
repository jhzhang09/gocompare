package native

import (
	"context"
	"fmt"
	"image"
	"image/color"

	"github.com/egoist/mygo/ui"
	"gocompare/internal/i18n"
	"gocompare/internal/picture"
)

// pictureCanvasScale 把每个源像素放大成整数倍方块，测试在方块中心取样。
const pictureCanvasScale = 20

// pictureTinyLimit 以内的图片默认用像素网格显示，否则单个像素几乎看不见。
const pictureTinyLimit = 32

const (
	pictureSide = iota
	pictureOverlay
	pictureDifference
	pictureTolerance
	pictureReplace
	pictureFlipH
	pictureFlipV
)

// 缩放模式：适应窗口只缩小不放大；像素网格按 pictureCanvasScale 放大，用于极小图片。
const (
	pictureZoomFit = iota
	pictureZoom100
	pictureZoom200
	pictureZoomGrid
)

// pictureReplaceColor 是替换颜色模式的高亮色，选用自然图片里少见的品红。
var pictureReplaceColor = color.RGBA{R: 255, B: 255, A: 255}

// loadPictures 在后台解码两张图。失败时清空画布，不把内容交给文本比较。
func (a *workspaceState) loadPictures() {
	paths := a.paths
	a.pictureGen++
	gen := a.pictureGen
	a.job(func(ctx context.Context) {
		left, err := picture.Open(paths[0])
		var right image.Image
		if err == nil {
			right, err = picture.Open(paths[1])
		}
		if err == nil && ctx.Err() != nil {
			err = ctx.Err()
		}
		a.publish(func() {
			// 同一时刻只有一个后台任务，结束时总要释放 busy，过期结果只是不再采用。
			a.busy = false
			if gen != a.pictureGen {
				return
			}
			if err != nil {
				a.err = err.Error()
				a.clearPictures()
				return
			}
			a.pictureLeft = left
			a.pictureRight = right
			a.pictureMode = pictureSide
			a.pictureTol = 2
			a.pictureZoom = pictureZoomFit
			if isTiny(left) && isTiny(right) {
				a.pictureZoom = pictureZoomGrid
			}
			a.err = ""
			a.message = i18n.T("picture.loaded")
			a.renderPicture()
		})
	})
}

func isTiny(img image.Image) bool {
	b := img.Bounds()
	return b.Dx() <= pictureTinyLimit && b.Dy() <= pictureTinyLimit
}

func (a *workspaceState) clearPictures() {
	a.pictureGen++
	a.pictureLeft = nil
	a.pictureRight = nil
	a.pictureResult = nil
	a.pictureBitmap = nil
	a.pictureNote = ""
	a.pictureSummary = picture.Summary{}
	a.pictureRerender = false
}

func (a *workspaceState) showPicture(mode int) {
	if a.pictureLeft == nil || a.pictureRight == nil {
		return
	}
	a.pictureMode = mode
	a.renderPicture()
}

// setPictureTolerance 修改容差。容差影响统计、容差图和替换颜色，所以总是重算。
func (a *workspaceState) setPictureTolerance(tol uint8) {
	if a.pictureLeft == nil || a.pictureRight == nil {
		return
	}
	a.pictureTol = tol
	a.renderPicture()
}

func (a *workspaceState) showTolerance(tol uint8) {
	if a.pictureLeft == nil || a.pictureRight == nil {
		return
	}
	a.pictureMode = pictureTolerance
	a.setPictureTolerance(tol)
}

// renderPicture 在后台生成位图和差异统计，避免大图逐像素计算阻塞界面。
// 计算期间再次请求（例如拖动容差滑块）只记下需要重算，完成后按最新参数补算一次。
func (a *workspaceState) renderPicture() {
	left, right := a.pictureLeft, a.pictureRight
	if left == nil || right == nil {
		return
	}
	if a.busy {
		a.pictureRerender = true
		return
	}
	a.pictureRerender = false
	mode, tol := a.pictureMode, a.pictureTol
	a.pictureGen++
	gen := a.pictureGen
	a.job(func(ctx context.Context) {
		out, note, summary := buildPicture(left, right, mode, tol)
		a.publish(func() {
			a.busy = false
			if gen != a.pictureGen {
				return
			}
			a.pictureResult = out
			a.pictureNote = note
			a.pictureSummary = summary
			if a.pictureBitmap == nil {
				a.pictureBitmap = ui.NewBitmap(out).Pixelated()
			} else {
				a.pictureBitmap.Update(out)
			}
			if a.pictureRerender {
				a.renderPicture()
			}
		})
	})
}

// buildPicture 按模式生成画布，并返回说明文案的 i18n 键和差异统计。
// 翻转和替换颜色把处理后的左图与右图并排，方便核对镜像或去背景后的内容。
func buildPicture(left, right image.Image, mode int, tol uint8) (*image.RGBA, string, picture.Summary) {
	summary := picture.Summarize(left, right, tol)
	switch mode {
	case pictureOverlay:
		return picture.Overlay(left, right), "picture.modeOverlay", summary
	case pictureDifference:
		return picture.Difference(left, right), "picture.modeDifference", summary
	case pictureTolerance:
		note := "picture.outOfTolerance"
		if summary.SameSize && summary.Exceeding == 0 {
			note = "picture.withinTolerance"
		}
		return picture.ToleranceMap(left, right, tol), note, summary
	case pictureReplace:
		// 以左图左上角像素作为背景色，两侧按同一颜色和容差替换，保证对照是对称的。
		b := left.Bounds()
		from := left.At(b.Min.X, b.Min.Y)
		out := picture.SideBySide(
			picture.ReplaceColor(left, from, pictureReplaceColor, tol),
			picture.ReplaceColor(right, from, pictureReplaceColor, tol),
		)
		return out, "picture.modeReplace", summary
	case pictureFlipH:
		return picture.SideBySide(picture.FlipH(left), right), "picture.modeFlipH", summary
	case pictureFlipV:
		return picture.SideBySide(picture.FlipV(left), right), "picture.modeFlipV", summary
	default:
		return picture.SideBySide(left, right), "picture.modeSide", summary
	}
}

func (a *workspaceState) pictureView(c *ui.Context) {
	t := c.Theme()
	pal := paletteFor(t)
	loaded := a.pictureLeft != nil && a.pictureRight != nil
	disabled := a.busy || !loaded

	modeBtn := func(label string, active bool, fn func()) {
		b := ui.Button(c, label).Padding(3, 8).Radius(5).FontSize(11).Disabled(disabled)
		if active && !disabled {
			b.Background(pal.baseBg).Border(1, pal.baseAccent.Alpha(0.35)).TextColor(pal.baseAccent).Bold()
		}
		b.OnClick(fn)
	}
	ui.Row(c).Height(38).Padding(0, 14).Gap(5).Background(pal.cardBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		modeBtn(i18n.T("picture.modeSide"), a.pictureMode == pictureSide, func() { a.showPicture(pictureSide) })
		modeBtn(i18n.T("picture.modeOverlay"), a.pictureMode == pictureOverlay, func() { a.showPicture(pictureOverlay) })
		modeBtn(i18n.T("picture.modeDifference"), a.pictureMode == pictureDifference, func() { a.showPicture(pictureDifference) })
		modeBtn(i18n.T("picture.modeTolerance"), a.pictureMode == pictureTolerance, func() { a.showPicture(pictureTolerance) })
		ui.Divider(c).Height(14)
		modeBtn(i18n.T("picture.modeReplace"), a.pictureMode == pictureReplace, func() { a.showPicture(pictureReplace) })
		modeBtn(i18n.T("picture.modeFlipH"), a.pictureMode == pictureFlipH, func() { a.showPicture(pictureFlipH) })
		modeBtn(i18n.T("picture.modeFlipV"), a.pictureMode == pictureFlipV, func() { a.showPicture(pictureFlipV) })
		if loaded {
			ui.Divider(c).Height(14)
			modeBtn(i18n.T("picture.zoomFit"), a.pictureZoom == pictureZoomFit, func() { a.pictureZoom = pictureZoomFit })
			modeBtn(i18n.T("picture.zoom100"), a.pictureZoom == pictureZoom100, func() { a.pictureZoom = pictureZoom100 })
			modeBtn(i18n.T("picture.zoom200"), a.pictureZoom == pictureZoom200, func() { a.pictureZoom = pictureZoom200 })
			modeBtn(i18n.T("picture.zoomGrid"), a.pictureZoom == pictureZoomGrid, func() { a.pictureZoom = pictureZoomGrid })
		}
	})

	if loaded {
		a.pictureStatusRow(c, pal, disabled)
	}

	if a.pictureBitmap == nil || a.pictureResult == nil {
		if a.err == "" {
			a.modeEmptyView(c, pal, kindPicture)
		}
		return
	}
	a.pictureCanvas(c, pal)
}

// pictureStatusRow 显示容差滑块和缓存的差异统计；统计在后台随画布一起计算，绘制时不扫描像素。
func (a *workspaceState) pictureStatusRow(c *ui.Context, pal uiPalette, disabled bool) {
	t := c.Theme()
	ui.Row(c).Height(34).Padding(0, 14).Gap(8).Background(pal.subtleSurface).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		ui.Text(c, i18n.TArgs("picture.toleranceValue", a.pictureTol)).FontSize(11).TextColor(t.TextMuted).SingleLine()
		a.pictureTolSlider = float64(a.pictureTol)
		ui.Slider(c, &a.pictureTolSlider, 0, 255).Width(180).Disabled(disabled).Label(i18n.T("picture.toleranceSlider")).OnChange(func() {
			a.setPictureTolerance(uint8(a.pictureTolSlider + 0.5))
		})
		ui.Box(c).Grow(1)
		lb, rb := a.pictureLeft.Bounds(), a.pictureRight.Bounds()
		statusPill(c, fmt.Sprintf("%d×%d vs %d×%d px", lb.Dx(), lb.Dy(), rb.Dx(), rb.Dy()), pal.sameFg, pal.sameBg)
		s := a.pictureSummary
		switch {
		case s.Total == 0:
		case !s.Found:
			statusPill(c, i18n.T("picture.identical"), pal.successFg, pal.successBg)
		case !s.SameSize:
			statusPill(c, i18n.T("picture.sizeMismatch"), pal.diffFg, pal.diffBg)
		default:
			statusPill(c, i18n.TArgs("picture.diffFound", s.First.X, s.First.Y), pal.diffFg, pal.diffBg)
		}
		if s.Total > 0 && s.Found {
			percent := float64(s.Exceeding) * 100 / float64(s.Total)
			statusPill(c, i18n.TArgs("picture.exceedCount", s.Exceeding, percent), pal.diffFg, pal.diffBg)
		}
		if a.pictureNote != "" {
			fg, bg := pal.baseAccent, pal.baseBg
			switch a.pictureNote {
			case "picture.outOfTolerance":
				fg, bg = pal.diffFg, pal.diffBg
			case "picture.withinTolerance":
				fg, bg = pal.successFg, pal.successBg
			}
			statusPill(c, i18n.T(a.pictureNote), fg, bg)
		}
	})
}

func (a *workspaceState) pictureCanvas(c *ui.Context, pal uiPalette) {
	t := c.Theme()
	b := a.pictureResult.Bounds()
	bw, bh := float32(b.Dx()), float32(b.Dy())
	sideBySide := a.pictureMode == pictureSide || a.pictureMode == pictureReplace || a.pictureMode == pictureFlipH || a.pictureMode == pictureFlipV
	legend := func(width float32, side int, img image.Image) {
		row := ui.Row(c).Gap(4)
		if width > 0 {
			row.Width(width)
		} else {
			row.Grow(1).Basis(0)
		}
		row.Children(func() {
			sideBadge(c, side, pal)
			ui.Text(c, i18n.T([]string{"picture.leftImage", "picture.rightImage"}[side])).FontSize(11).Bold().TextColor(pal.sidebarText)
			ui.Text(c, fmt.Sprintf("%d×%d px", img.Bounds().Dx(), img.Bounds().Dy())).FontSize(10).TextColor(t.TextMuted)
		})
	}

	if a.pictureZoom == pictureZoomFit {
		// 适应窗口交给框架按可用区域等比缩放：小图放大（像素化显示），大图只缩小不放大。
		fit := ui.ScaleDown
		if bw <= 256 && bh <= 256 {
			fit = ui.Contain
		}
		ui.Column(c).Grow(1).FillWidth().Padding(12, 16).Gap(6).Children(func() {
			if sideBySide {
				ui.Row(c).FillWidth().Children(func() {
					legend(0, 0, a.pictureLeft)
					legend(0, 1, a.pictureRight)
				})
			}
			ui.Image(c, a.pictureBitmap).Fit(fit).Grow(1).FillWidth().Label(i18n.T("a11y.pictureCanvas"))
		})
		return
	}

	scale := float32(1)
	switch a.pictureZoom {
	case pictureZoom200:
		scale = 2
	case pictureZoomGrid:
		scale = pictureCanvasScale
	}
	w, h := bw*scale, bh*scale
	ui.ScrollBoth(c).Grow(1).FillWidth().Children(func() {
		ui.Column(c).Padding(12, 16).Gap(6).AlignSelf(ui.Start).Children(func() {
			if sideBySide {
				lw := float32(a.pictureLeft.Bounds().Dx()) * scale
				rw := float32(a.pictureRight.Bounds().Dx()) * scale
				// 像素网格下单侧可能只有几十像素宽，标签不按宽度截断。
				if a.pictureZoom != pictureZoomGrid || lw >= 120 {
					ui.Row(c).Width(w).Children(func() {
						legend(lw, 0, a.pictureLeft)
						legend(rw, 1, a.pictureRight)
					})
				}
			}
			ui.Image(c, a.pictureBitmap).Size(w, h).MaxWidth(w).MaxHeight(h).AlignSelf(ui.Start).Label(i18n.T("a11y.pictureCanvas"))
		})
	})
}
