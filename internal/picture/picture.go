// Package picture 用像素而不是整文件哈希比较图片。
package picture

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// Open 解码 PNG、JPEG 或 GIF。GIF 只返回第一帧，不播放后续帧。其他内容和未知格式返回错误，不按整文件哈希比较。
func Open(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, format, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	switch format {
	case "png", "jpeg", "gif":
		return img, nil
	default:
		return nil, fmt.Errorf("%s is not supported", format)
	}
}

// PixelDiff 是两张图的第一处像素差异。尺寸不同时 Found 为 true，且 X、Y 无意义。
type PixelDiff struct {
	SameSize bool
	Found    bool
	X        int
	Y        int
}

// FirstDifference 按从左到右、从上到下找到第一处通道不同的像素。
func FirstDifference(a, b image.Image) PixelDiff {
	same := a.Bounds().Dx() == b.Bounds().Dx() && a.Bounds().Dy() == b.Bounds().Dy()
	if !same {
		return PixelDiff{Found: true}
	}
	w, h := a.Bounds().Dx(), a.Bounds().Dy()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ac, _ := at(a, x, y)
			bc, _ := at(b, x, y)
			if ac != bc {
				return PixelDiff{SameSize: true, Found: true, X: x, Y: y}
			}
		}
	}
	return PixelDiff{SameSize: true}
}

// SideBySide 把两张图横向拼在一起。
func SideBySide(a, b image.Image) *image.RGBA {
	ab, bb := a.Bounds(), b.Bounds()
	h := ab.Dy()
	if bb.Dy() > h {
		h = bb.Dy()
	}
	out := image.NewRGBA(image.Rect(0, 0, ab.Dx()+bb.Dx(), h))
	blit(out, a, 0, 0)
	blit(out, b, ab.Dx(), 0)
	return out
}

// Overlay 按像素平均叠加。超出一侧范围的像素保留另一侧。
func Overlay(a, b image.Image) *image.RGBA {
	w, h := boundsOf(a, b)
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ac, aok := at(a, x, y)
			bc, bok := at(b, x, y)
			switch {
			case aok && bok:
				out.SetRGBA(x, y, color.RGBA{
					R: uint8((int(ac.R) + int(bc.R)) / 2),
					G: uint8((int(ac.G) + int(bc.G)) / 2),
					B: uint8((int(ac.B) + int(bc.B)) / 2),
					A: uint8((int(ac.A) + int(bc.A)) / 2),
				})
			case aok:
				out.SetRGBA(x, y, ac)
			case bok:
				out.SetRGBA(x, y, bc)
			}
		}
	}
	return out
}

// Difference 输出每个通道的绝对差。
func Difference(a, b image.Image) *image.RGBA {
	w, h := boundsOf(a, b)
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ac, _ := at(a, x, y)
			bc, _ := at(b, x, y)
			out.SetRGBA(x, y, color.RGBA{
				R: dist(ac.R, bc.R),
				G: dist(ac.G, bc.G),
				B: dist(ac.B, bc.B),
				A: 255,
			})
		}
	}
	return out
}

// Within 判断两张图每个像素的通道差是否都不超过 tol。
func Within(a, b image.Image, tol uint8) bool {
	w, h := boundsOf(a, b)
	if a.Bounds().Dx() != b.Bounds().Dx() || a.Bounds().Dy() != b.Bounds().Dy() {
		return false
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ac, _ := at(a, x, y)
			bc, _ := at(b, x, y)
			if dist(ac.R, bc.R) > tol || dist(ac.G, bc.G) > tol || dist(ac.B, bc.B) > tol || dist(ac.A, bc.A) > tol {
				return false
			}
		}
	}
	return true
}

// ReplaceColor 把容差内的颜色替换成另一种颜色。
func ReplaceColor(src image.Image, from, to color.Color, tol uint8) *image.RGBA {
	fr, fg, fb, fa := from.RGBA()
	tr, tg, tb, ta := to.RGBA()
	target := color.RGBA{R: uint8(tr >> 8), G: uint8(tg >> 8), B: uint8(tb >> 8), A: uint8(ta >> 8)}
	b := src.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c, _ := at(src, x, y)
			if dist(c.R, uint8(fr>>8)) <= tol && dist(c.G, uint8(fg>>8)) <= tol && dist(c.B, uint8(fb>>8)) <= tol && dist(c.A, uint8(fa>>8)) <= tol {
				out.SetRGBA(x, y, target)
				continue
			}
			out.SetRGBA(x, y, c)
		}
	}
	return out
}

// FlipH 左右翻转。
func FlipH(src image.Image) *image.RGBA {
	b := src.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c, _ := at(src, x, y)
			out.SetRGBA(b.Dx()-1-x, y, c)
		}
	}
	return out
}

// FlipV 上下翻转。
func FlipV(src image.Image) *image.RGBA {
	b := src.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	h := b.Dy()
	for y := 0; y < h; y++ {
		for x := 0; x < b.Dx(); x++ {
			c, _ := at(src, x, y)
			out.SetRGBA(x, h-1-y, c)
		}
	}
	return out
}

func boundsOf(a, b image.Image) (int, int) {
	w, h := a.Bounds().Dx(), a.Bounds().Dy()
	if b.Bounds().Dx() > w {
		w = b.Bounds().Dx()
	}
	if b.Bounds().Dy() > h {
		h = b.Bounds().Dy()
	}
	return w, h
}

func at(src image.Image, x, y int) (color.RGBA, bool) {
	b := src.Bounds()
	if x < 0 || y < 0 || x >= b.Dx() || y >= b.Dy() {
		return color.RGBA{}, false
	}
	r, g, bb, a := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(bb >> 8), A: uint8(a >> 8)}, true
}

func blit(dst *image.RGBA, src image.Image, ox, oy int) {
	b := src.Bounds()
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c, _ := at(src, x, y)
			dst.SetRGBA(ox+x, oy+y, c)
		}
	}
}

func dist(a, b uint8) uint8 {
	return uint8(math.Abs(float64(a) - float64(b)))
}
