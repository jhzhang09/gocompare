package extend

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestExecuteCancelStopsTheProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("扩展夹具是 POSIX shell 脚本，Windows 上需要另写批处理夹具")
	}
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	tool := filepath.Join(dir, "sleep.sh")
	script := "#!/bin/sh\necho $$ > \"$3\"\nsleep 30\n"
	if err := os.WriteFile(tool, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	left := filepath.Join(dir, "left")
	right := filepath.Join(dir, "right")
	if err := os.WriteFile(left, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("b"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(ready); err == nil {
				time.Sleep(50 * time.Millisecond)
				cancel()
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	spec := Spec{Version: 1, Kind: "compare", Args: []string{tool, "%left", "%right", ready}}
	start := time.Now()
	_, err := Execute(ctx, spec, Paths{Left: left, Right: right})
	if time.Since(start) > 5*time.Second {
		t.Fatal("cancel did not stop the extension")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	pidText, err := os.ReadFile(ready)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidText)))
	if err != nil {
		t.Fatal(err)
	}
	if exec.Command("ps", "-p", strconv.Itoa(pid)).Run() == nil {
		t.Fatalf("pid %d still running", pid)
	}
}

func TestExecuteRejectsPartialCompressAndEmbeddedPlaceholder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("扩展夹具是 POSIX shell 脚本，Windows 上需要另写批处理夹具")
	}
	dir := t.TempDir()
	tool := filepath.Join(dir, "bad.sh")
	script := "#!/bin/sh\nprintf partial > \"$2\"\nprintf 'failpack\\n' >&2\nexit 4\n"
	if err := os.WriteFile(tool, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	in := filepath.Join(dir, "in")
	out := filepath.Join(dir, "out")
	before := []byte("raw")
	if err := os.WriteFile(in, before, 0o600); err != nil {
		t.Fatal(err)
	}
	spec := Spec{Version: 1, Kind: "compress", Args: []string{tool, "%in", "%out"}}
	got, err := Execute(context.Background(), spec, Paths{In: in, Out: out})
	if err == nil || err.Error() != "extension exit 4: failpack" || got.Payload != nil {
		t.Fatalf("partial %+v %v", got, err)
	}
	after, err := os.ReadFile(in)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("compress failure changed the input")
	}
	partial, err := os.ReadFile(out)
	if err != nil || string(partial) != "partial" {
		t.Fatalf("child output %q %v", partial, err)
	}
	embedded := Spec{Version: 1, Kind: "compare", Args: []string{tool, "pre%left", "%right"}}
	if _, err = Execute(context.Background(), embedded, Paths{Left: in, Right: in}); err == nil || err.Error() != "placeholder must be a whole argument" {
		t.Fatalf("placeholder: %v", err)
	}
}
