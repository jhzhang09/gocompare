package native

import (
	"context"
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
	ui.Row(c).Padding(8, 20).Gap(8).Children(func() {
		ui.Button(c, "并排").Disabled(a.busy || a.pictureLeft == nil).OnClick(func() { a.showPicture(pictureSide) })
		ui.Button(c, "叠加").Disabled(a.busy || a.pictureLeft == nil).OnClick(func() { a.showPicture(pictureOverlay) })
		ui.Button(c, "差值").Disabled(a.busy || a.pictureLeft == nil).OnClick(func() { a.showPicture(pictureDifference) })
		ui.Button(c, "容差 2").Disabled(a.busy || a.pictureLeft == nil).OnClick(func() { a.showTolerance(2) })
		ui.Button(c, "容差 255").Disabled(a.busy || a.pictureLeft == nil).OnClick(func() { a.showTolerance(255) })
		ui.Button(c, "替换颜色").Disabled(a.busy || a.pictureLeft == nil).OnClick(func() { a.showPicture(pictureReplace) })
		ui.Button(c, "水平翻转").Disabled(a.busy || a.pictureLeft == nil).OnClick(func() { a.showPicture(pictureFlipH) })
		ui.Button(c, "垂直翻转").Disabled(a.busy || a.pictureLeft == nil).OnClick(func() { a.showPicture(pictureFlipV) })
	})
	if a.pictureNote != "" {
		ui.Text(c, a.pictureNote).Padding(4, 20)
	}
	if a.pictureBitmap == nil || a.pictureResult == nil {
		if a.err == "" {
			ui.Text(c, "选择两张 PNG、JPEG 或 GIF").Padding(8, 20)
		}
		return
	}
	b := a.pictureResult.Bounds()
	w := float32(b.Dx() * pictureCanvasScale)
	h := float32(b.Dy() * pictureCanvasScale)
	ui.Image(c, a.pictureBitmap).Size(w, h).MaxWidth(w).MaxHeight(h).AlignSelf(ui.Start).Label("图片画布")
}
