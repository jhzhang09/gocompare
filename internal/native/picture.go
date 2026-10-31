package native

import (
	"context"
	"fmt"
	"image"
	"image/color"

	"github.com/egoist/mygo/ui"
	"gocompare/internal/picture"
)

// pictureCanvasScale 把每个源像素放大成整数倍方块，测试在方块中心取样。
const pictureCanvasScale = 20

const (
	pictureSide = iota
	pictureOverlay
	pictureDifference
	pictureTolerance
	pictureReplace
	pictureFlipH
	pictureFlipV
)

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
			if gen != a.pictureGen {
				return
			}
			a.busy = false
			if err != nil {
				a.err = err.Error()
				a.clearPictures()
				return
			}
			a.pictureLeft = left
			a.pictureRight = right
			a.pictureMode = pictureSide
			a.pictureTol = 2
			a.renderPicture()
			a.err = ""
			a.message = "已加载图片"
		})
	})
}

func (a *workspaceState) clearPictures() {
	a.pictureLeft = nil
	a.pictureRight = nil
	a.pictureResult = nil
	a.pictureBitmap = nil
	a.pictureNote = ""
}

func (a *workspaceState) showPicture(mode int) {
	if a.busy || a.pictureLeft == nil || a.pictureRight == nil {
		return
	}
	a.pictureMode = mode
	a.renderPicture()
}

func (a *workspaceState) showTolerance(tol uint8) {
	if a.busy || a.pictureLeft == nil || a.pictureRight == nil {
		return
	}
	a.pictureMode = pictureTolerance
	a.pictureTol = tol
	a.renderPicture()
}

// renderPicture 只在加载完成或切换模式时计算。视图只显示已有位图。
func (a *workspaceState) renderPicture() {
	left, right := a.pictureLeft, a.pictureRight
	if left == nil || right == nil {
		return
	}
	var out *image.RGBA
	switch a.pictureMode {
	case pictureOverlay:
		out = picture.Overlay(left, right)
		a.pictureNote = "叠加"
	case pictureDifference:
		out = picture.Difference(left, right)
		a.pictureNote = "差值"
	case pictureTolerance:
		out = picture.Difference(left, right)
		if picture.Within(left, right, a.pictureTol) {
			a.pictureNote = "容差内相同"
		} else {
			a.pictureNote = "超出容差"
		}
	case pictureReplace:
		b := left.Bounds()
		from := left.At(b.Min.X, b.Min.Y)
		replaced := picture.ReplaceColor(left, from, color.RGBA{G: 10, A: 255}, 0)
		out = picture.SideBySide(replaced, right)
		a.pictureNote = "替换颜色"
	case pictureFlipH:
		out = picture.SideBySide(picture.FlipH(left), right)
		a.pictureNote = "水平翻转"
	case pictureFlipV:
		out = picture.SideBySide(picture.FlipV(left), right)
		a.pictureNote = "垂直翻转"
	default:
		out = picture.SideBySide(left, right)
		a.pictureNote = "并排"
	}
	a.pictureResult = out
	if a.pictureBitmap == nil {
		a.pictureBitmap = ui.NewBitmap(out).Pixelated()
		return
	}
	a.pictureBitmap.Update(out)
}

func (a *workspaceState) pictureView(c *ui.Context) {
	t := c.Theme()
	pal := paletteFor(t)
	disabled := a.busy || a.pictureLeft == nil

	ui.Row(c).Height(38).Padding(0, 14).Gap(5).Background(pal.cardBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.cardBorder).Children(func() {
		modeBtn := func(label string, active bool, fn func()) {
			b := ui.Button(c, label).Padding(3, 8).Radius(5).FontSize(11).Disabled(disabled)
			if active && !disabled {
				b.Background(pal.baseBg).Border(1, pal.baseAccent.Alpha(0.35)).TextColor(pal.baseAccent).Bold()
			}
			b.OnClick(fn)
		}
		modeBtn("并排", a.pictureMode == pictureSide, func() { a.showPicture(pictureSide) })
		modeBtn("叠加", a.pictureMode == pictureOverlay, func() { a.showPicture(pictureOverlay) })
		modeBtn("差值", a.pictureMode == pictureDifference, func() { a.showPicture(pictureDifference) })
		ui.Divider(c).Height(14)
		modeBtn("容差 2", a.pictureMode == pictureTolerance && a.pictureTol == 2, func() { a.showTolerance(2) })
		modeBtn("容差 255", a.pictureMode == pictureTolerance && a.pictureTol == 255, func() { a.showTolerance(255) })
		ui.Divider(c).Height(14)
		modeBtn("替换颜色", a.pictureMode == pictureReplace, func() { a.showPicture(pictureReplace) })
		modeBtn("水平翻转", a.pictureMode == pictureFlipH, func() { a.showPicture(pictureFlipH) })
		modeBtn("垂直翻转", a.pictureMode == pictureFlipV, func() { a.showPicture(pictureFlipV) })

		ui.Box(c).Grow(1)
		if a.pictureLeft != nil && a.pictureRight != nil {
			lb, rb := a.pictureLeft.Bounds(), a.pictureRight.Bounds()
			statusPill(c, fmt.Sprintf("%d×%d vs %d×%d px", lb.Dx(), lb.Dy(), rb.Dx(), rb.Dy()), pal.sameFg, pal.sameBg)
		}
		if a.pictureNote != "" {
			fg, bg := pal.baseAccent, pal.baseBg
			if a.pictureNote == "超出容差" {
				fg, bg = pal.diffFg, pal.diffBg
			} else if a.pictureNote == "容差内相同" {
				fg, bg = pal.successFg, pal.successBg
			}
			statusPill(c, a.pictureNote, fg, bg)
		}
	})

	if a.pictureBitmap == nil || a.pictureResult == nil {
		if a.err == "" {
			ui.Column(c).Grow(1).FillWidth().Center().Padding(24).Children(func() {
				ui.Column(c).MaxWidth(480).Padding(24, 28).Gap(10).Radius(12).Background(pal.cardBg).Border(1, pal.cardBorder).Center().Children(func() {
					ui.Row(c).Size(32, 32).Center().Radius(8).Background(pal.baseBg).Children(func() {
						ui.Icon(c, iconImageDiff).Size(16, 16).TextColor(pal.baseAccent)
					})
					ui.Text(c, "选择两张 PNG、JPEG 或 GIF").FontSize(17).Bold()
					ui.Text(c, "支持并排、半透明叠加、像素差值、容差阈值校验及镜像翻转").FontSize(11).TextColor(t.TextMuted)
				})
			})
		}
		return
	}

	b := a.pictureResult.Bounds()
	w := float32(b.Dx() * pictureCanvasScale)
	h := float32(b.Dy() * pictureCanvasScale)
	ui.Column(c).Grow(1).FillWidth().Padding(12, 16).Children(func() {
		ui.Image(c, a.pictureBitmap).Size(w, h).MaxWidth(w).MaxHeight(h).AlignSelf(ui.Start).Label("图片画布")
	})
}
