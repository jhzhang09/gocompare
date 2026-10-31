package native

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"golang.org/x/image/bmp"
)

func TestWorkspacePictureCanvas(t *testing.T) {
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.png")
	rightPath := filepath.Join(dir, "right.png")
	leftPx := [2][2]color.RGBA{
		{{R: 200, G: 10, B: 20, A: 255}, {B: 200, A: 255}},
		{{G: 200, A: 255}, {R: 10, A: 255}},
	}
	rightPx := leftPx
	rightPx[0][0] = color.RGBA{R: 20, G: 30, B: 180, A: 255}
	writeGridPNG(t, leftPath, leftPx)
	writeGridPNG(t, rightPath, rightPx)
	runGridComparison(t, leftPath, rightPath, leftPx)
}

func TestWorkspacePictureGIF(t *testing.T) {
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.gif")
	rightPath := filepath.Join(dir, "right.gif")
	leftPx := [2][2]color.RGBA{
		{{R: 200, G: 10, B: 20, A: 255}, {B: 200, A: 255}},
		{{G: 200, A: 255}, {R: 10, A: 255}},
	}
	rightPx := leftPx
	rightPx[0][0] = color.RGBA{R: 20, G: 30, B: 180, A: 255}
	writeGridGIF(t, leftPath, leftPx)
	writeGridGIF(t, rightPath, rightPx)
	runGridComparison(t, leftPath, rightPath, leftPx)
}

func TestWorkspacePictureJPEG(t *testing.T) {
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.jpg")
	rightPath := filepath.Join(dir, "right.jpg")
	tl, tr := gray(40), gray(220)
	bl, br := gray(20), gray(110)
	rightTL := gray(200)
	writeQuadrantJPEG(t, leftPath, [2][2]color.RGBA{{tl, tr}, {bl, br}})
	writeQuadrantJPEG(t, rightPath, [2][2]color.RGBA{{rightTL, tr}, {bl, br}})
	beforeLeft, err := os.ReadFile(leftPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeRight, err := os.ReadFile(rightPath)
	if err != nil {
		t.Fatal(err)
	}
	a := &workspaceState{paths: [2]string{leftPath, rightPath}}
	tt := ui.NewTester(a.view, 1380, 1280)
	tt.Frame()
	if err = tt.Click("图片比较"); err != nil {
		t.Fatal(err)
	}
	if err = tt.Click("开始比较"); err != nil {
		t.Fatal(err)
	}
	if a.err != "" || a.kind != 3 || rgbaOf(a.pictureLeft, 4, 4) != tl {
		t.Fatalf("jpeg load kind=%d err=%s origin=%v", a.kind, a.err, rgbaOf(a.pictureLeft, 4, 4))
	}
	assertPicture(t, tt, a, "并排", map[image.Point]color.RGBA{
		{4, 4}: tl, {12, 4}: tr, {4, 12}: bl, {12, 12}: br,
		{20, 4}: rightTL, {28, 12}: br,
	})
	if err = tt.Click("叠加"); err != nil {
		t.Fatal(err)
	}
	assertPicture(t, tt, a, "叠加", map[image.Point]color.RGBA{
		{4, 4}: gray(120), {12, 4}: tr, {4, 12}: bl,
	})
	if err = tt.Click("差值"); err != nil {
		t.Fatal(err)
	}
	assertPicture(t, tt, a, "差值", map[image.Point]color.RGBA{
		{4, 4}: gray(160), {12, 4}: {A: 255}, {4, 12}: {A: 255},
	})
	if err = tt.Click("容差 2"); err != nil {
		t.Fatal(err)
	}
	assertPicture(t, tt, a, "超出容差", map[image.Point]color.RGBA{{4, 4}: gray(160)})
	if !tt.HasText("超出容差") {
		t.Fatalf("tolerance note: %v", tt.Texts())
	}
	if err = tt.Click("容差 255"); err != nil {
		t.Fatal(err)
	}
	assertPicture(t, tt, a, "容差内相同", map[image.Point]color.RGBA{{4, 4}: gray(160)})
	if !tt.HasText("容差内相同") {
		t.Fatalf("tolerance note: %v", tt.Texts())
	}
	if err = tt.Click("替换颜色"); err != nil {
		t.Fatal(err)
	}
	assertPicture(t, tt, a, "替换颜色", map[image.Point]color.RGBA{
		{4, 4}: {G: 10, A: 255}, {12, 4}: tr, {16, 4}: rightTL,
	})
	if err = tt.Click("水平翻转"); err != nil {
		t.Fatal(err)
	}
	assertPicture(t, tt, a, "水平翻转", map[image.Point]color.RGBA{
		{4, 4}: tr, {12, 4}: tl, {4, 12}: br, {12, 12}: bl, {20, 4}: rightTL,
	})
	if err = tt.Click("垂直翻转"); err != nil {
		t.Fatal(err)
	}
	assertPicture(t, tt, a, "垂直翻转", map[image.Point]color.RGBA{
		{4, 4}: bl, {4, 12}: tl, {12, 4}: br, {12, 12}: tr, {20, 12}: bl,
	})
	afterLeft, err := os.ReadFile(leftPath)
	if err != nil {
		t.Fatal(err)
	}
	afterRight, err := os.ReadFile(rightPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(beforeLeft, afterLeft) || !bytes.Equal(beforeRight, afterRight) {
		t.Fatal("jpeg inputs changed")
	}
}

func TestWorkspacePictureGIFStaysOnFirstFrame(t *testing.T) {
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "move.gif")
	rightPath := filepath.Join(dir, "still.gif")
	black := color.RGBA{A: 255}
	red := color.RGBA{R: 255, A: 255}
	writeTwoFrameGIF(t, leftPath, black, red)
	writeGridGIF(t, rightPath, [2][2]color.RGBA{{black, black}, {black, black}})
	a := &workspaceState{paths: [2]string{leftPath, rightPath}}
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()
	if err := tt.Click("图片比较"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("开始比较"); err != nil {
		t.Fatal(err)
	}
	if a.err != "" {
		t.Fatal(a.err)
	}
	assertPicture(t, tt, a, "并排", map[image.Point]color.RGBA{
		{0, 0}: black, {1, 0}: black, {2, 0}: black,
	})
	if tt.HasText("播放") {
		t.Fatal("picture comparison shows a player")
	}
	tt.Advance(time.Second)
	assertPicture(t, tt, a, "并排", map[image.Point]color.RGBA{{0, 0}: black, {1, 0}: black})
	if rgbaOf(a.pictureLeft, 0, 0) == red {
		t.Fatal("advanced to the second GIF frame")
	}
}

func runGridComparison(t *testing.T, leftPath, rightPath string, leftPx [2][2]color.RGBA) {
	t.Helper()
	rightPx := leftPx
	rightPx[0][0] = color.RGBA{R: 20, G: 30, B: 180, A: 255}
	beforeLeft, err := os.ReadFile(leftPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeRight, err := os.ReadFile(rightPath)
	if err != nil {
		t.Fatal(err)
	}

	a := &workspaceState{paths: [2]string{leftPath, rightPath}}
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()
	if err = tt.Click("图片比较"); err != nil {
		t.Fatal(err)
	}
	if a.kind != 3 {
		t.Fatalf("kind %d", a.kind)
	}
	if _, ok := tt.Find("图片画布"); ok {
		t.Fatal("canvas before load")
	}
	if err = tt.Click("开始比较"); err != nil {
		t.Fatal(err)
	}
	if a.err != "" || a.kind != 3 || a.panes[0] != nil {
		t.Fatalf("load kind=%d err=%s panes=%v", a.kind, a.err, a.panes[0] != nil)
	}
	if got := rgbaOf(a.pictureLeft, 0, 0); got != leftPx[0][0] {
		t.Fatalf("decoded left origin %v", got)
	}

	assertPicture(t, tt, a, "并排", map[image.Point]color.RGBA{
		{0, 0}: leftPx[0][0],
		{1, 0}: leftPx[0][1],
		{2, 0}: rightPx[0][0],
		{3, 1}: rightPx[1][1],
	})
	if err = tt.Click("叠加"); err != nil {
		t.Fatal(err)
	}
	assertPicture(t, tt, a, "叠加", map[image.Point]color.RGBA{
		{0, 0}: {R: 110, G: 20, B: 100, A: 255},
		{1, 0}: leftPx[0][1],
		{0, 1}: leftPx[1][0],
		{1, 1}: leftPx[1][1],
	})
	if err = tt.Click("差值"); err != nil {
		t.Fatal(err)
	}
	assertPicture(t, tt, a, "差值", map[image.Point]color.RGBA{
		{0, 0}: {R: 180, G: 20, B: 160, A: 255},
		{1, 0}: {A: 255},
		{0, 1}: {A: 255},
		{1, 1}: {A: 255},
	})
	if err = tt.Click("容差 2"); err != nil {
		t.Fatal(err)
	}
	assertPicture(t, tt, a, "超出容差", map[image.Point]color.RGBA{
		{0, 0}: {R: 180, G: 20, B: 160, A: 255},
		{1, 1}: {A: 255},
	})
	if !tt.HasText("超出容差") {
		t.Fatalf("tolerance note: %v", tt.Texts())
	}
	if err = tt.Click("容差 255"); err != nil {
		t.Fatal(err)
	}
	assertPicture(t, tt, a, "容差内相同", map[image.Point]color.RGBA{
		{0, 0}: {R: 180, G: 20, B: 160, A: 255},
		{1, 1}: {A: 255},
	})
	if !tt.HasText("容差内相同") {
		t.Fatalf("tolerance note: %v", tt.Texts())
	}
	if err = tt.Click("替换颜色"); err != nil {
		t.Fatal(err)
	}
	assertPicture(t, tt, a, "替换颜色", map[image.Point]color.RGBA{
		{0, 0}: {G: 10, A: 255},
		{1, 0}: leftPx[0][1],
		{0, 1}: leftPx[1][0],
		{2, 0}: rightPx[0][0],
	})
	if err = tt.Click("水平翻转"); err != nil {
		t.Fatal(err)
	}
	assertPicture(t, tt, a, "水平翻转", map[image.Point]color.RGBA{
		{0, 0}: leftPx[0][1],
		{1, 0}: leftPx[0][0],
		{0, 1}: leftPx[1][1],
		{1, 1}: leftPx[1][0],
		{2, 0}: rightPx[0][0],
	})
	if err = tt.Click("垂直翻转"); err != nil {
		t.Fatal(err)
	}
	assertPicture(t, tt, a, "垂直翻转", map[image.Point]color.RGBA{
		{0, 0}: leftPx[1][0],
		{1, 0}: leftPx[1][1],
		{0, 1}: leftPx[0][0],
		{1, 1}: leftPx[0][1],
		{2, 1}: rightPx[1][0],
		{3, 1}: rightPx[1][1],
	})

	afterLeft, err := os.ReadFile(leftPath)
	if err != nil {
		t.Fatal(err)
	}
	afterRight, err := os.ReadFile(rightPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(beforeLeft, afterLeft) || !bytes.Equal(beforeRight, afterRight) {
		t.Fatal("picture inputs changed")
	}
}

func TestWorkspacePictureRejectsText(t *testing.T) {
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.txt")
	rightPath := filepath.Join(dir, "right.txt")
	original := []byte("not a picture\n")
	if err := os.WriteFile(leftPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rightPath, []byte("also text\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := &workspaceState{kind: 3, paths: [2]string{leftPath, rightPath}}
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()
	if err := tt.Click("开始比较"); err != nil {
		t.Fatal(err)
	}
	if a.kind != 3 || a.pictureResult != nil || a.pictureBitmap != nil || !bytes.Contains([]byte(a.err), []byte("unknown format")) {
		t.Fatalf("kind=%d result=%v bitmap=%v err=%s", a.kind, a.pictureResult != nil, a.pictureBitmap != nil, a.err)
	}
	if !tt.HasText("错误") {
		t.Fatalf("texts %v", tt.Texts())
	}
	if _, ok := tt.Find("图片画布"); ok {
		t.Fatal("canvas shown for text")
	}
	got, err := os.ReadFile(leftPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatal("text input changed")
	}
}

func TestWorkspacePictureRejectsBMP(t *testing.T) {
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.bmp")
	rightPath := filepath.Join(dir, "right.bmp")
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	writeBMP(t, leftPath, img)
	writeBMP(t, rightPath, img)
	before, err := os.ReadFile(leftPath)
	if err != nil {
		t.Fatal(err)
	}
	a := &workspaceState{kind: 3, paths: [2]string{leftPath, rightPath}}
	tt := ui.NewTester(a.view, 1380, 880)
	tt.Frame()
	if err = tt.Click("开始比较"); err != nil {
		t.Fatal(err)
	}
	if a.kind != 3 || a.pictureResult != nil || !bytes.Contains([]byte(a.err), []byte("bmp is not supported")) {
		t.Fatalf("kind=%d result=%v err=%s", a.kind, a.pictureResult != nil, a.err)
	}
	if _, ok := tt.Find("图片画布"); ok {
		t.Fatal("canvas shown for bmp")
	}
	got, err := os.ReadFile(leftPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, before) {
		t.Fatal("bmp input changed")
	}
}

func assertPicture(t *testing.T, tt *ui.Tester, a *workspaceState, note string, want map[image.Point]color.RGBA) {
	t.Helper()
	if a.pictureResult == nil || a.pictureNote != note {
		t.Fatalf("note %q result %v err %s", a.pictureNote, a.pictureResult != nil, a.err)
	}
	for p, color := range want {
		if got := a.pictureResult.RGBAAt(p.X, p.Y); got != color {
			t.Fatalf("%s pixel %v got %v want %v", note, p, got, color)
		}
	}
	sampleCanvas(t, tt, a.pictureResult)
}

func sampleCanvas(t *testing.T, tt *ui.Tester, img *image.RGBA) {
	t.Helper()
	r, ok := tt.Find("图片画布")
	if !ok {
		t.Fatal("missing canvas")
	}
	b := img.Bounds()
	wantW := float32(b.Dx() * pictureCanvasScale)
	wantH := float32(b.Dy() * pictureCanvasScale)
	if math.Abs(float64(r.W-wantW)) > 0.5 || math.Abs(float64(r.H-wantH)) > 0.5 {
		t.Fatalf("canvas box %v want %gx%g", r, wantW, wantH)
	}
	frame := tt.Image()
	ox := int(math.Round(float64(r.X)))
	oy := int(math.Round(float64(r.Y)))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			sx := ox + x*pictureCanvasScale + pictureCanvasScale/2
			sy := oy + y*pictureCanvasScale + pictureCanvasScale/2
			got := frame.RGBAAt(sx, sy)
			want := img.RGBAAt(x, y)
			if got != want {
				t.Fatalf("screen %d,%d at %d,%d got %v want %v box %v", x, y, sx, sy, got, want, r)
			}
		}
	}
}

func rgbaOf(img image.Image, x, y int) color.RGBA {
	b := img.Bounds()
	r, g, bb, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(bb >> 8), A: uint8(a >> 8)}
}

func writeGridPNG(t *testing.T, path string, pix [2][2]color.RGBA) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.SetRGBA(0, 0, pix[0][0])
	img.SetRGBA(1, 0, pix[0][1])
	img.SetRGBA(0, 1, pix[1][0])
	img.SetRGBA(1, 1, pix[1][1])
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func gray(v uint8) color.RGBA { return color.RGBA{R: v, G: v, B: v, A: 255} }

func writeGridGIF(t *testing.T, path string, pix [2][2]color.RGBA) {
	t.Helper()
	var pal color.Palette
	index := map[color.RGBA]uint8{}
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			c := pix[y][x]
			if _, ok := index[c]; ok {
				continue
			}
			index[c] = uint8(len(pal))
			pal = append(pal, c)
		}
	}
	img := image.NewPaletted(image.Rect(0, 0, 2, 2), pal)
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.SetColorIndex(x, y, index[pix[y][x]])
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = gif.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
}

func writeTwoFrameGIF(t *testing.T, path string, first, second color.RGBA) {
	t.Helper()
	pal := color.Palette{first, second}
	a := image.NewPaletted(image.Rect(0, 0, 2, 2), pal)
	b := image.NewPaletted(image.Rect(0, 0, 2, 2), pal)
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			b.SetColorIndex(x, y, 1)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = gif.EncodeAll(f, &gif.GIF{Image: []*image.Paletted{a, b}, Delay: []int{10, 10}}); err != nil {
		t.Fatal(err)
	}
}

func writeQuadrantJPEG(t *testing.T, path string, q [2][2]color.RGBA) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetRGBA(x, y, q[y/8][x/8])
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = jpeg.Encode(f, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
}

func writeBMP(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = bmp.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
