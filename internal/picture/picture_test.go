package picture

import (
	"image"
	"image/color"
	"testing"
)

func solid(c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.SetRGBA(0, 0, c)
	img.SetRGBA(1, 0, c)
	return img
}

func TestPixelsNotFileHash(t *testing.T) {
	red := solid(color.RGBA{R: 200, A: 255})
	blue := solid(color.RGBA{B: 100, A: 255})
	side := SideBySide(red, blue)
	if side.Bounds().Dx() != 4 || side.RGBAAt(0, 0).R != 200 || side.RGBAAt(2, 0).B != 100 {
		t.Fatalf("side: %v %v", side.RGBAAt(0, 0), side.RGBAAt(2, 0))
	}
	over := Overlay(red, blue)
	if over.RGBAAt(0, 0).R != 100 || over.RGBAAt(0, 0).B != 50 {
		t.Fatalf("overlay: %v", over.RGBAAt(0, 0))
	}
	diff := Difference(red, blue)
	if diff.RGBAAt(0, 0).R != 200 || diff.RGBAAt(0, 0).B != 100 {
		t.Fatalf("difference: %v", diff.RGBAAt(0, 0))
	}
	near := solid(color.RGBA{R: 202, A: 255})
	if !Within(red, near, 2) || Within(red, near, 1) {
		t.Fatal("tolerance")
	}
	replaced := ReplaceColor(red, color.RGBA{R: 200, A: 255}, color.RGBA{G: 10, A: 255}, 0)
	if replaced.RGBAAt(0, 0).G != 10 || replaced.RGBAAt(1, 0).R != 0 {
		t.Fatalf("replace: %v", replaced.RGBAAt(0, 0))
	}
	red.SetRGBA(1, 0, color.RGBA{G: 7, A: 255})
	flipped := FlipH(red)
	if flipped.RGBAAt(0, 0).G != 7 || flipped.RGBAAt(1, 0).R != 200 {
		t.Fatalf("flip: %v %v", flipped.RGBAAt(0, 0), flipped.RGBAAt(1, 0))
	}
	tall := image.NewRGBA(image.Rect(0, 0, 1, 2))
	tall.SetRGBA(0, 0, color.RGBA{R: 200, A: 255})
	tall.SetRGBA(0, 1, color.RGBA{G: 7, A: 255})
	flippedV := FlipV(tall)
	if flippedV.RGBAAt(0, 0).G != 7 || flippedV.RGBAAt(0, 1).R != 200 {
		t.Fatalf("flipv: %v %v", flippedV.RGBAAt(0, 0), flippedV.RGBAAt(0, 1))
	}
}

func TestSummarizeAndToleranceMap(t *testing.T) {
	a := image.NewRGBA(image.Rect(0, 0, 2, 1))
	b := image.NewRGBA(image.Rect(0, 0, 3, 1))
	a.SetRGBA(0, 0, color.RGBA{R: 100, A: 255})
	a.SetRGBA(1, 0, color.RGBA{R: 100, A: 255})
	b.SetRGBA(0, 0, color.RGBA{R: 101, A: 255})
	b.SetRGBA(1, 0, color.RGBA{R: 150, A: 255})
	s := Summarize(a, b, 2)
	// 第 0 列在容差内但不同，第 1 列超出，第 2 列只在右图存在。
	if s.SameSize || s.Total != 3 || s.Differing != 3 || s.Exceeding != 2 || !s.Found || s.First != (image.Point{}) {
		t.Fatalf("summary: %+v", s)
	}
	m := ToleranceMap(a, b, 2)
	if m.RGBAAt(0, 0) != (color.RGBA{R: 11, G: 11, B: 11, A: 255}) || m.RGBAAt(1, 0) != (color.RGBA{R: 255, A: 255}) || m.RGBAAt(2, 0) != (color.RGBA{R: 255, A: 255}) {
		t.Fatalf("map: %v %v %v", m.RGBAAt(0, 0), m.RGBAAt(1, 0), m.RGBAAt(2, 0))
	}
	if s := Summarize(a, a, 0); s.Found || s.Differing != 0 || !s.SameSize {
		t.Fatalf("identical: %+v", s)
	}
}
