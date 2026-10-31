package native

import (
	"bytes"
	"image/png"
	"io"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
	"gocompare/internal/brand"
)

func TestWorkspaceShowsBrandAndFramework(t *testing.T) {
	version, err := brand.Framework()
	if err != nil {
		t.Fatal(err)
	}
	if version != "v0.7.3" {
		t.Fatalf("framework %s", version)
	}
	img, err := png.Decode(bytes.NewReader(brand.IconPNG()))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, a := img.At(512, 488).RGBA()
	if r>>8 != 38 || g>>8 != 93 || b>>8 != 171 || a>>8 != 255 {
		t.Fatalf("arrow pixel %d %d %d %d", r>>8, g>>8, b>>8, a>>8)
	}
	aState := &workspaceState{}
	tt := ui.NewTester(aState.view, 1380, 880)
	tt.Frame()
	if aState.err != "" || aState.frameworkVersion() != version {
		t.Fatalf("version err=%s %s", aState.err, aState.frameworkVersion())
	}
	if !tt.HasText("GoCompare") || tt.HasText("MyGo "+version) {
		t.Fatalf("texts %v", tt.Texts())
	}
	if _, ok := tt.Find("应用图标"); !ok {
		t.Fatal("brand icon is not in the window")
	}
	if aState.icon == nil {
		t.Fatal("icon was not decoded for the window")
	}
}

func TestRunHelpReportsFramework(t *testing.T) {
	version, err := brand.Framework()
	if err != nil {
		t.Fatal(err)
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = writer
	runErr := Run([]string{"--help"})
	writer.Close()
	os.Stdout = previous
	out, readErr := io.ReadAll(reader)
	reader.Close()
	if runErr != nil || readErr != nil {
		t.Fatalf("help %v %v", runErr, readErr)
	}
	want := "GoCompare (" + version + ")\nUsage: gocompare [left-file right-file] | --directories left-dir right-dir\n"
	if string(out) != want {
		t.Fatalf("help %q", out)
	}
}
