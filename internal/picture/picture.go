// Package picture 用像素而不是整文件哈希比较图片。
package picture

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"os"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// MaxPixels 是单张图片的像素上限（约 6700 万像素）。解码前按文件头检查，
// 防止声明超大尺寸的小文件在解码时耗尽内存。
const MaxPixels = 64 << 20

// Open 解码 PNG、JPEG 或 GIF，并转换成原点为 (0,0) 的 *image.RGBA，后续逐像素比较不再走接口转换。
// GIF 只返回第一帧，不播放后续帧。其他内容和未知格式返回错误，不按整文件哈希比较。
// JPEG 的 EXIF 方向不做旋转，按存储的像素比较。
func Open(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return nil, err
	}
	switch format {
	case "png", "jpeg", "gif":
	default:
		return nil, fmt.Errorf("%s is not supported", format)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > MaxPixels {
		return nil, fmt.Errorf("image %dx%d exceeds %d pixel limit", cfg.Width, cfg.Height, MaxPixels)
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return toRGBA(img), nil
}

// toRGBA 把任意图片转换成原点为 (0,0) 的预乘 RGBA。
func toRGBA(src image.Image) *image.RGBA {
	b := src.Bounds()
	if rgba, ok := src.(*image.RGBA); ok && b.Min == (image.Point{}) {
		return rgba
	}
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), src, b.Min, draw.Src)
	return out
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

// Within 判断两张图尺寸相同，且每个像素的通道差都不超过 tol。
func Within(a, b image.Image, tol uint8) bool {
	s := Summarize(a, b, tol)
	return s.SameSize && s.Exceeding == 0
}

// Summary 统计两张图的像素差异。尺寸不同时，只在一侧存在的像素同时计入 Differing 和 Exceeding。
type Summary struct {
	SameSize  bool
	Total     int // 两图外接矩形内的像素数
	Differing int // 任一通道不同的像素数
	Exceeding int // 任一通道差超过容差的像素数
	First     image.Point
	Found     bool // 是否存在 Differing 像素；First 为扫描顺序中的第一处
}

// Summarize 一次扫描得到差异像素数、超出容差的像素数和第一处差异。
func Summarize(a, b image.Image, tol uint8) Summary {
	w, h := boundsOf(a, b)
	s := Summary{SameSize: a.Bounds().Dx() == b.Bounds().Dx() && a.Bounds().Dy() == b.Bounds().Dy(), Total: w * h}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ac, aok := at(a, x, y)
			bc, bok := at(b, x, y)
			if aok && bok && ac == bc {
				continue
			}
			if !s.Found {
				s.Found, s.First = true, image.Point{X: x, Y: y}
			}
			s.Differing++
			if !aok || !bok || exceeds(ac, bc, tol) {
				s.Exceeding++
			}
		}
	}
	return s
}

// ToleranceMap 标出超出容差的像素：超出的画成红色，容差内的画成左图的暗灰度，
// 只在一侧存在的区域也算超出。用来直观看到压缩噪点被过滤后剩下的真实差异。
func ToleranceMap(a, b image.Image, tol uint8) *image.RGBA {
	w, h := boundsOf(a, b)
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ac, aok := at(a, x, y)
			bc, bok := at(b, x, y)
			if !aok || !bok || exceeds(ac, bc, tol) {
				out.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
				continue
			}
			// ITU-R BT.601 亮度，压到 40% 让红色差异更醒目。
			l := uint8((299*int(ac.R) + 587*int(ac.G) + 114*int(ac.B)) * 2 / 5 / 1000)
			out.SetRGBA(x, y, color.RGBA{R: l, G: l, B: l, A: 255})
		}
	}
	return out
}

func exceeds(a, b color.RGBA, tol uint8) bool {
	return dist(a.R, b.R) > tol || dist(a.G, b.G) > tol || dist(a.B, b.B) > tol || dist(a.A, b.A) > tol
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
	// Open 产出的都是 *image.RGBA，直接取值避免每个像素经过接口和颜色模型转换。
	if rgba, ok := src.(*image.RGBA); ok {
		return rgba.RGBAAt(b.Min.X+x, b.Min.Y+y), true
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
	if a > b {
		return a - b
	}
	return b - a
}
