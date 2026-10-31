package native

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func writeTestImage(t *testing.T, path string, w, h int, fill color.RGBA) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, fill)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspacePictureRealImageFitAndModes(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "photo1.png")
	p2 := filepath.Join(dir, "photo2.png")

	// Create 400x600 images
	c1 := color.RGBA{R: 50, G: 100, B: 150, A: 255}
	c2 := color.RGBA{R: 200, G: 100, B: 50, A: 255}
	writeTestImage(t, p1, 400, 600, c1)
	writeTestImage(t, p2, 400, 600, c2)

	// 1. Test identical images
	a := &workspaceState{paths: [2]string{p1, p1}}
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()

	if err := tt.Click("图片比较"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("开始比较"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	if a.pictureLeft == nil || a.pictureRight == nil {
		t.Fatal("images not loaded")
	}

	// Should default to 适应窗口 (Fit to Window) for real images
	if a.pictureZoom != 0 {
		t.Fatalf("expected pictureZoom 0 (fit), got %d", a.pictureZoom)
	}

	// Canvas dimensions must fit viewport
	r, ok := tt.Find("图片画布")
	if !ok {
		t.Fatal("missing canvas element")
	}
	if r.W > 1200 || r.H > 800 {
		t.Fatalf("canvas should fit viewport, got %gx%g", r.W, r.H)
	}

	// Status must report identical
	if !tt.HasText("两图完全一致") {
		t.Fatalf("expected '两图完全一致' pill, got texts: %v", tt.Texts())
	}

	// Switch to 差值 (Difference)
	if err := tt.Click("差值"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.pictureMode != pictureDifference {
		t.Fatalf("expected pictureDifference, got %d", a.pictureMode)
	}
	// For identical images, difference pixel must be black (0,0,0)
	if got := a.pictureResult.RGBAAt(10, 10); got != (color.RGBA{A: 255}) {
		t.Fatalf("expected black difference for identical images, got %v", got)
	}

	// 2. Test different images
	a.paths = [2]string{p1, p2}
	if err := tt.Click("开始比较"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	if !tt.HasText("存在差异") {
		t.Fatalf("expected '存在差异' pill for different images, got: %v", tt.Texts())
	}

	// Switch to 叠加 (Overlay)
	if err := tt.Click("叠加"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.pictureMode != pictureOverlay {
		t.Fatalf("expected pictureOverlay, got %d", a.pictureMode)
	}
	// Overlay of (50,100,150) and (200,100,50) is ((50+200)/2, (100+100)/2, (150+50)/2) = (125, 100, 100)
	wantOverlay := color.RGBA{R: 125, G: 100, B: 100, A: 255}
	if got := a.pictureResult.RGBAAt(10, 10); got != wantOverlay {
		t.Fatalf("expected overlay pixel %v, got %v", wantOverlay, got)
	}

	// Test Zoom controls
	if err := tt.Click("1:1 实际"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.pictureZoom != 1 {
		t.Fatalf("expected pictureZoom 1, got %d", a.pictureZoom)
	}
	r1, _ := tt.Find("图片画布")
	if r1.W != 400 || r1.H != 600 {
		t.Fatalf("expected 1:1 canvas 400x600, got %gx%g", r1.W, r1.H)
	}
}
