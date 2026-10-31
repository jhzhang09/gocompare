package picture

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenPNGAndRejectsOtherBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dot.png")
	src := image.NewRGBA(image.Rect(0, 0, 2, 1))
	src.SetRGBA(0, 0, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	src.SetRGBA(1, 0, color.RGBA{R: 40, G: 50, B: 60, A: 255})
	writePNG(t, path, src)
	got, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	c := rgbaAt(t, got, 0, 0)
	if c != (color.RGBA{R: 10, G: 20, B: 30, A: 255}) || got.Bounds().Dx() != 2 {
		t.Fatalf("png pixel: %+v", c)
	}
	other := image.NewRGBA(image.Rect(0, 0, 2, 1))
	other.SetRGBA(0, 0, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	other.SetRGBA(1, 0, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	otherPath := filepath.Join(dir, "other.png")
	writePNG(t, otherPath, other)
	opened, err := Open(otherPath)
	if err != nil {
		t.Fatal(err)
	}
	diff := FirstDifference(got, opened)
	if !diff.SameSize || !diff.Found || diff.X != 1 || diff.Y != 0 {
		t.Fatalf("diff: %+v", diff)
	}
	if !FirstDifference(got, got).SameSize || FirstDifference(got, got).Found {
		t.Fatal("identical png reported a difference")
	}
	wide := image.NewRGBA(image.Rect(0, 0, 3, 1))
	widePath := filepath.Join(dir, "wide.png")
	writePNG(t, widePath, wide)
	wideImg, err := Open(widePath)
	if err != nil {
		t.Fatal(err)
	}
	size := FirstDifference(got, wideImg)
	if size.SameSize || !size.Found {
		t.Fatalf("size: %+v", size)
	}
	text := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(text, []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = Open(text); err == nil || !bytes.Contains([]byte(err.Error()), []byte("unknown format")) {
		t.Fatalf("text: %v", err)
	}
}

func TestOpenGIFExactAndJPEGBounds(t *testing.T) {
	dir := t.TempDir()
	pal := color.Palette{color.RGBA{A: 255}, color.RGBA{R: 255, A: 255}}
	g := image.NewPaletted(image.Rect(0, 0, 2, 1), pal)
	g.SetColorIndex(1, 0, 1)
	gifPath := filepath.Join(dir, "dot.gif")
	f, err := os.Create(gifPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = gif.Encode(f, g, nil); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := Open(gifPath)
	if err != nil {
		t.Fatal(err)
	}
	if rgbaAt(t, got, 1, 0).R != 255 || rgbaAt(t, got, 0, 0).R != 0 {
		t.Fatalf("gif: %+v %+v", rgbaAt(t, got, 0, 0), rgbaAt(t, got, 1, 0))
	}
	jpg := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			jpg.SetRGBA(x, y, color.RGBA{R: 200, G: 10, B: 10, A: 255})
		}
	}
	jpgPath := filepath.Join(dir, "red.jpg")
	jf, err := os.Create(jpgPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = jpeg.Encode(jf, jpg, &jpeg.Options{Quality: 100}); err != nil {
		jf.Close()
		t.Fatal(err)
	}
	if err = jf.Close(); err != nil {
		t.Fatal(err)
	}
	decoded, err := Open(jpgPath)
	if err != nil || decoded.Bounds().Dx() != 8 || decoded.Bounds().Dy() != 8 {
		t.Fatalf("jpeg: %v %+v", err, decoded)
	}
	if rgbaAt(t, decoded, 3, 3).R < 150 {
		t.Fatalf("jpeg red collapsed: %+v", rgbaAt(t, decoded, 3, 3))
	}
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func rgbaAt(t *testing.T, img image.Image, x, y int) color.RGBA {
	t.Helper()
	c, ok := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
	if !ok {
		t.Fatalf("pixel type %T", img.At(x, y))
	}
	return c
}
