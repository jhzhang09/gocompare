package brand

import (
	"bytes"
	"image/png"
	"os"
	"testing"

	_ "github.com/egoist/mygo"
)

func TestIconMatchesTheRenderedMark(t *testing.T) {
	onDisk, err := os.ReadFile("../../resources/icon.png")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(IconPNG(), onDisk) {
		t.Fatal("embedded icon differs from resources/icon.png")
	}
	img, err := png.Decode(bytes.NewReader(IconPNG()))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 1024 || img.Bounds().Dy() != 1024 {
		t.Fatalf("size %v", img.Bounds())
	}
	// 这些点对应 icon.svg 的底色、左右页、差异色和箭头，圆角外保持透明。
	checks := []struct {
		x, y       int
		r, g, b, a uint32
	}{
		{10, 10, 0, 0, 0, 0},
		{512, 512, 23, 43, 72, 255},
		{200, 250, 246, 248, 251, 255},
		{300, 450, 197, 229, 220, 255},
		{600, 450, 216, 203, 234, 255},
		{512, 488, 38, 93, 171, 255},
	}
	for _, check := range checks {
		r, g, b, a := img.At(check.x, check.y).RGBA()
		if r>>8 != check.r || g>>8 != check.g || b>>8 != check.b || a>>8 != check.a {
			t.Fatalf("pixel %d,%d = %d %d %d %d", check.x, check.y, r>>8, g>>8, b>>8, a>>8)
		}
	}
}

func TestFrameworkIsTheLinkedModule(t *testing.T) {
	version, err := Framework()
	if err != nil {
		t.Fatal(err)
	}
	if version != "v0.7.3" {
		t.Fatalf("framework %s", version)
	}
}
