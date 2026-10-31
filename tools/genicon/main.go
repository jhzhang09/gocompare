// Genicon 使用 MyGo 渲染器从 SVG 生成各平台共享的应用图标。
//
//	go run ./tools/genicon
package main

import (
	"image/png"
	"log"
	"os"

	"github.com/egoist/mygo/ui"
)

func main() {
	data, err := os.ReadFile("resources/icon.svg")
	if err != nil {
		log.Fatal(err)
	}
	svg := ui.MustParseSVG(data)
	img := ui.Render(func(c *ui.Context) {
		c.Root().Background(ui.Transparent)
		ui.Image(c, svg).Size(1024, 1024)
	}, 1024, 1024, 1)
	for _, path := range []string{"resources/icon.png", "internal/brand/icon.png"} {
		f, err := os.Create(path)
		if err != nil {
			log.Fatal(err)
		}
		if err = png.Encode(f, img); err != nil {
			f.Close()
			log.Fatal(err)
		}
		if err = f.Close(); err != nil {
			log.Fatal(err)
		}
	}
}
