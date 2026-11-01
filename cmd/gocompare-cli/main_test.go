package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"gocompare/internal/brand"
	"gocompare/internal/compare"
	"gocompare/internal/extend"
	"gocompare/internal/integrate"
	"gocompare/internal/updater"
	"gocompare/internal/workspace"
)

func TestExitCodes(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	os.WriteFile(a, []byte("a"), 0o600)
	os.WriteFile(b, []byte("a"), 0o600)
	var out, stderr bytes.Buffer
	if got := run([]string{a, b}, &out, &stderr); got != 0 {
		t.Fatal(got)
	}
	os.WriteFile(b, []byte("b"), 0o600)
	if got := run([]string{a, b}, &out, &stderr); got != 1 {
		t.Fatal(got)
	}
	if got := run([]string{a, "missing"}, &out, &stderr); got != 2 {
		t.Fatal(got)
	}
	if got := run(nil, &out, &stderr); got != 2 {
		t.Fatal(got)
	}
}

func TestDirectoryCLI(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	var out, stderr bytes.Buffer
	if code := run([]string{"dir", left, right}, &out, &stderr); code != 0 {
		t.Fatal(code)
	}
	os.WriteFile(filepath.Join(left, "new"), []byte("x"), 0o600)
	out.Reset()
	if code := run([]string{"dir", left, right}, &out, &stderr); code != 1 || !bytes.Contains(out.Bytes(), []byte("left_only")) {
		t.Fatalf("%d %s", code, out.String())
	}
	if code := run([]string{"dir", left, filepath.Join(right, "missing")}, &out, &stderr); code != 2 {
		t.Fatal(code)
	}
}

func TestDirectoryRulesCLI(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	write := func(root, rel, text string) string {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		return full
	}
	leftA := write(left, "nested/a.txt", "same")
	rightA := write(right, "nested/a.txt", "same")
	leftB := write(left, "nested/b.bin", "x")
	rightB := write(right, "nested/b.bin", "y")
	if err := os.Chmod(leftA, 0o600); err != nil {
		t.Fatal(err)
	}
	// 用只读制造权限差异：Windows 没有 POSIX 权限位，只有只读属性能体现在 Mode() 里。
	if err := os.Chmod(rightA, 0o400); err != nil {
		t.Fatal(err)
	}
	leftInfo, err := os.Stat(leftA)
	if err != nil {
		t.Fatal(err)
	}
	rightInfo, err := os.Stat(rightA)
	if err != nil {
		t.Fatal(err)
	}
	if leftInfo.Mode().Perm() == rightInfo.Mode().Perm() {
		t.Fatalf("modes did not differ: %v %v", leftInfo.Mode().Perm(), rightInfo.Mode().Perm())
	}
	past := time.Unix(1_700_000_000, 0)
	if err = os.Chtimes(leftA, past, past); err != nil {
		t.Fatal(err)
	}
	if err = os.Chtimes(rightA, past.Add(time.Hour), past.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	before := map[string][]byte{}
	for _, path := range []string{leftA, rightA, leftB, rightB} {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		before[path] = data
	}
	var out, stderr bytes.Buffer
	code, rows := runDir(t, &out, &stderr, "dir", left, right)
	if code != 1 || rowStatus(rows, "nested/a.txt") != compare.Same || rowStatus(rows, "nested/b.bin") != compare.Different {
		t.Fatalf("default %d %+v", code, rows)
	}
	code, rows = runDir(t, &out, &stderr, "dir", "--compare-mode", left, right)
	if code != 1 || rowStatus(rows, "nested/a.txt") != compare.Different {
		t.Fatalf("mode %d %+v", code, rows)
	}
	code, rows = runDir(t, &out, &stderr, "dir", "--compare-time", left, right)
	if code != 1 || rowStatus(rows, "nested/a.txt") != compare.Different {
		t.Fatalf("time %d %+v", code, rows)
	}
	code, rows = runDir(t, &out, &stderr, "dir", "--filter", "nested/*.txt", left, right)
	if code != 0 || len(rows) != 1 || rows[0].Path != "nested/a.txt" || rows[0].Status != compare.Same || bytes.Contains(out.Bytes(), []byte("b.bin")) {
		t.Fatalf("filter %d %s %+v", code, out.String(), rows)
	}
	code, rows = runDir(t, &out, &stderr, "dir", "--compare-time", "--filter", "nested/*.txt", left, right)
	if code != 1 || len(rows) != 1 || rows[0].Path != "nested/a.txt" || rows[0].Status != compare.Different || bytes.Contains(out.Bytes(), []byte("b.bin")) {
		t.Fatalf("filter time %d %s %+v", code, out.String(), rows)
	}
	out.Reset()
	stderr.Reset()
	if code = run([]string{"dir", "--filter", "[", left, right}, &out, &stderr); code != 2 || !bytes.Contains(stderr.Bytes(), []byte("syntax error in pattern")) || out.Len() != 0 {
		t.Fatalf("bad filter %d out=%q err=%q", code, out.String(), stderr.String())
	}
	out.Reset()
	stderr.Reset()
	if code = run([]string{"dir", "--links", "nope", left, right}, &out, &stderr); code != 2 || !bytes.Contains(stderr.Bytes(), []byte("unknown link policy: nope")) || out.Len() != 0 {
		t.Fatalf("links %d out=%q err=%q", code, out.String(), stderr.String())
	}
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret")
	if err = os.WriteFile(secret, []byte("hidden"), 0o600); err != nil {
		t.Fatal(err)
	}
	linkLeft, linkRight := t.TempDir(), t.TempDir()
	if err = os.Symlink(secret, filepath.Join(linkLeft, "link")); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(secret, filepath.Join(linkRight, "link")); err != nil {
		t.Fatal(err)
	}
	code, rows = runDir(t, &out, &stderr, "dir", linkLeft, linkRight)
	if code != 0 || rowStatus(rows, "link") != compare.Same || bytes.Contains(out.Bytes(), []byte("hidden")) || bytes.Contains(out.Bytes(), []byte("symlink escapes root")) {
		t.Fatalf("record %d %s %+v", code, out.String(), rows)
	}
	code, rows = runDir(t, &out, &stderr, "dir", "--links", "follow", linkLeft, linkRight)
	if code != 2 || rowStatus(rows, "link") != compare.Failed || !bytes.Contains(out.Bytes(), []byte("symlink escapes root")) || bytes.Contains(out.Bytes(), []byte("hidden")) {
		t.Fatalf("follow %d %s %+v", code, out.String(), rows)
	}
	for path, data := range before {
		got, readErr := os.ReadFile(path)
		if readErr != nil || !bytes.Equal(got, data) {
			t.Fatalf("bytes changed %s", path)
		}
	}
	secretData, err := os.ReadFile(secret)
	if err != nil || string(secretData) != "hidden" {
		t.Fatal("secret changed")
	}
}

func runDir(t *testing.T, out, stderr *bytes.Buffer, args ...string) (int, []compare.Entry) {
	t.Helper()
	out.Reset()
	stderr.Reset()
	code := run(args, out, stderr)
	if code == 2 && out.Len() == 0 {
		t.Fatalf("dir failed: %s", stderr.String())
	}
	var rows []compare.Entry
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("json %v %s %s", err, out.String(), stderr.String())
	}
	return code, rows
}

func rowStatus(rows []compare.Entry, path string) compare.Status {
	for _, row := range rows {
		if row.Path == path {
			return row.Status
		}
	}
	return ""
}

func TestMergeAndRestoreCLI(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base")
	left := filepath.Join(dir, "left")
	right := filepath.Join(dir, "right")
	output := filepath.Join(dir, "out")
	os.WriteFile(base, []byte("a\nb\n"), 0o600)
	os.WriteFile(left, []byte("a\nleft\n"), 0o600)
	os.WriteFile(right, []byte("a\nright\n"), 0o600)
	var out, stderr bytes.Buffer
	if code := run([]string{"merge", "--base", base, "--output", output, left, right}, &out, &stderr); code != 1 {
		t.Fatalf("merge %d %s", code, stderr.String())
	}
	data, err := os.ReadFile(output)
	if err != nil || !bytes.Contains(data, []byte("<<<<<<< left")) {
		t.Fatalf("output: %q %v", data, err)
	}
	if code := run([]string{"merge", "--output", output, left, right}, &out, &stderr); code != 2 {
		t.Fatal("overwrite accepted")
	}
	leftDir, rightDir, backup := t.TempDir(), t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(leftDir, "a"), []byte("a"), 0o600)
	out.Reset()
	stderr.Reset()
	if code := run([]string{"sync", "--mode", "mirror-right", "--apply", "--backup-dir", backup, leftDir, rightDir}, &out, &stderr); code != 0 {
		t.Fatalf("sync %d %s", code, stderr.String())
	}
	var report struct {
		Journal string `json:"journal"`
	}
	if err = json.Unmarshal(out.Bytes(), &report); err != nil || report.Journal == "" {
		t.Fatalf("journal: %v %s", err, out.String())
	}
	journal := report.Journal
	out.Reset()
	if code := run([]string{"restore", journal}, &out, &stderr); code != 0 {
		t.Fatalf("restore %d %s", code, stderr.String())
	}
	if _, err = os.Stat(filepath.Join(rightDir, "a")); !os.IsNotExist(err) {
		t.Fatal("restore left copied file")
	}
}

func TestSyncCLIRequiresExplicitApply(t *testing.T) {
	left, right, backup := t.TempDir(), t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(left, "a"), []byte("a"), 0o600)
	var out, stderr bytes.Buffer
	if code := run([]string{"sync", "--mode", "mirror-right", left, right}, &out, &stderr); code != 1 {
		t.Fatal(code)
	}
	if _, err := os.Stat(filepath.Join(right, "a")); !os.IsNotExist(err) {
		t.Fatal("preview wrote file")
	}
	if code := run([]string{"sync", "--apply", left, right}, &out, &stderr); code != 2 {
		t.Fatal("apply without backup allowed")
	}
	if code := run([]string{"sync", "--mode", "mirror-right", "--apply", "--backup-dir", backup, left, right}, &out, &stderr); code != 0 {
		t.Fatalf("execute %d: %s", code, stderr.String())
	}
}

func TestHexDifftoolMergetoolAndScript(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left")
	right := filepath.Join(dir, "right")
	base := filepath.Join(dir, "base")
	output := filepath.Join(dir, "out")
	os.WriteFile(left, []byte("aaaa"), 0o600)
	os.WriteFile(right, []byte("aaab"), 0o600)
	os.WriteFile(base, []byte("a\nb\n"), 0o600)
	mergeLeft := filepath.Join(dir, "mleft")
	mergeRight := filepath.Join(dir, "mright")
	os.WriteFile(mergeLeft, []byte("a\nleft\n"), 0o600)
	os.WriteFile(mergeRight, []byte("a\nright\n"), 0o600)
	var out, stderr bytes.Buffer
	if code := run(integrate.FileManagerArgs(left, right), &out, &stderr); code != 1 || !bytes.Contains(out.Bytes(), []byte("different")) {
		t.Fatalf("file manager %d %s", code, out.String())
	}
	out.Reset()
	if code := run(integrate.DifftoolArgs(left, right), &out, &stderr); code != 1 || !bytes.Contains(out.Bytes(), []byte("different")) {
		t.Fatalf("difftool %d %s", code, out.String())
	}
	out.Reset()
	if code := run([]string{"hex", left, right}, &out, &stderr); code != 1 || !bytes.Contains(out.Bytes(), []byte("offset 0")) {
		t.Fatalf("hex %d %s", code, out.String())
	}
	if code := run(integrate.MergetoolArgs(base, mergeLeft, mergeRight, output), &out, &stderr); code != 1 {
		t.Fatalf("mergetool %d %s", code, stderr.String())
	}
	data, err := os.ReadFile(output)
	if err != nil || !bytes.Contains(data, []byte("<<<<<<< left")) {
		t.Fatalf("mergetool output %q %v", data, err)
	}
	scriptPath := filepath.Join(dir, "job.gcs")
	os.WriteFile(scriptPath, []byte("hex "+left+" "+right+"\n"), 0o600)
	out.Reset()
	if code := run([]string{"script", scriptPath}, &out, &stderr); code != 1 || !bytes.Contains(out.Bytes(), []byte("offset 0")) {
		t.Fatalf("script %d %s %s", code, out.String(), stderr.String())
	}
	report := filepath.Join(dir, "report.html")
	if code := run([]string{"report", "--title", "Report", "--input", left, "--output", report}, &out, &stderr); code != 0 {
		t.Fatalf("report %d %s", code, stderr.String())
	}
	html, err := os.ReadFile(report)
	if err != nil || !bytes.Contains(html, []byte("aaaa")) || bytes.Contains(html, []byte("webview")) {
		t.Fatalf("html %s %v", html, err)
	}
	pages := filepath.Join(dir, "pages.txt")
	os.WriteFile(pages, []byte("a\nb\nc\nd\ne"), 0o600)
	out.Reset()
	if code := run([]string{"paginate", "--width", "10", "--rows", "2", "--input", pages}, &out, &stderr); code != 0 || !bytes.Contains(out.Bytes(), []byte("\f")) || !bytes.Contains(out.Bytes(), []byte("a\nb")) {
		t.Fatalf("paginate %d %q %s", code, out.String(), stderr.String())
	}
	session := filepath.Join(dir, "one.gocompare")
	if err = workspace.Write(session, workspace.Session{Version: 1, Kind: "text", Left: left, Right: right}); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(dir, "bundle.json")
	if code := run([]string{"workspace", "save", bundle, session}, &out, &stderr); code != 0 {
		t.Fatalf("save %d %s", code, stderr.String())
	}
	out.Reset()
	if code := run([]string{"workspace", "recover", bundle}, &out, &stderr); code != 0 || !bytes.Contains(out.Bytes(), []byte(`"kind":"text"`)) {
		t.Fatalf("recover %d %s", code, out.String())
	}
}

func TestPictureCLIReadsPixels(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left.png")
	right := filepath.Join(dir, "right.png")
	writeSolidPNG(t, left, color.RGBA{R: 10, G: 20, B: 30, A: 255}, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	writeSolidPNG(t, right, color.RGBA{R: 10, G: 20, B: 30, A: 255}, color.RGBA{R: 9, G: 20, B: 30, A: 255})
	var out, stderr bytes.Buffer
	if code := run([]string{"picture", left, right}, &out, &stderr); code != 1 || out.String() != "pixel 1,0\n" {
		t.Fatalf("picture %d %q %s", code, out.String(), stderr.String())
	}
	out.Reset()
	if code := run([]string{"picture", left, left}, &out, &stderr); code != 0 || out.String() != "equal\n" {
		t.Fatalf("same %d %q %s", code, out.String(), stderr.String())
	}
	note := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(note, []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	stderr.Reset()
	if code := run([]string{"picture", left, note}, &out, &stderr); code != 2 || !bytes.Contains(stderr.Bytes(), []byte("unknown format")) {
		t.Fatalf("reject %d %q %s", code, out.String(), stderr.String())
	}
}

func TestMediaCLIReadsTags(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "song.bin")
	writeCLIID3(t, path, "LeftSong", "甲", "LeftAlbum")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	if code := run([]string{"media", path}, &out, &stderr); code != 0 || out.String() != "title LeftSong\nartist 甲\nalbum LeftAlbum\n" {
		t.Fatalf("media %d %q %s", code, out.String(), stderr.String())
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("media read changed the file")
	}
	note := filepath.Join(dir, "note.txt")
	if err = os.WriteFile(note, []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	stderr.Reset()
	if code := run([]string{"media", note}, &out, &stderr); code != 2 || stderr.String() != "not an id3v2 header\n" || out.Len() != 0 {
		t.Fatalf("reject %d %q %s", code, out.String(), stderr.String())
	}
}

func TestExtendCLI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("扩展夹具是 POSIX shell 脚本，Windows 上需要另写批处理夹具")
	}
	dir := t.TempDir()
	compareTool := writeTool(t, filepath.Join(dir, "compare.sh"), "#!/bin/sh\nleft=$(cat \"$1\")\nright=$(cat \"$2\")\nif [ \"$left\" = \"$right\" ]; then\n  printf 'same %s\\n' \"$left\"\n  exit 0\nfi\nprintf 'different %s %s\\n' \"$left\" \"$right\"\nexit 1\n")
	convertTool := writeTool(t, filepath.Join(dir, "convert.sh"), "#!/bin/sh\nprintf 'text:%s\\n' \"$(cat \"$1\")\"\nexit 0\n")
	convertFail := writeTool(t, filepath.Join(dir, "convert-fail.sh"), "#!/bin/sh\nprintf LEAK\nprintf 'boom\\n' >&2\nexit 2\n")
	packTool := writeTool(t, filepath.Join(dir, "pack.sh"), "#!/bin/sh\nprintf 'PACK:%s' \"$(cat \"$1\")\" > \"$2\"\nexit 0\n")
	packMissing := writeTool(t, filepath.Join(dir, "pack-missing.sh"), "#!/bin/sh\nexit 0\n")
	left := filepath.Join(dir, "left.txt")
	right := filepath.Join(dir, "right.txt")
	same := filepath.Join(dir, "same.txt")
	if err := os.WriteFile(left, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(same, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	beforeLeft, err := os.ReadFile(left)
	if err != nil {
		t.Fatal(err)
	}
	spec := writeSpec(t, filepath.Join(dir, "compare.json"), extend.Spec{Version: 1, Kind: "compare", Args: []string{compareTool, "%left", "%right"}})
	var out, stderr bytes.Buffer
	if code := run([]string{"extend", spec, left, same}, &out, &stderr); code != 0 || out.String() != "same one\n" {
		t.Fatalf("same %d %q %s", code, out.String(), stderr.String())
	}
	out.Reset()
	stderr.Reset()
	if code := run([]string{"extend", spec, left, right}, &out, &stderr); code != 1 || out.String() != "different one two\n" {
		t.Fatalf("different %d %q %s", code, out.String(), stderr.String())
	}
	afterLeft, err := os.ReadFile(left)
	if err != nil || !bytes.Equal(beforeLeft, afterLeft) {
		t.Fatal("compare changed the input")
	}

	in := filepath.Join(dir, "note.txt")
	if err = os.WriteFile(in, []byte("Hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	convertSpec := writeSpec(t, filepath.Join(dir, "convert.json"), extend.Spec{Version: 1, Kind: "convert", Args: []string{convertTool, "%in"}})
	out.Reset()
	stderr.Reset()
	if code := run([]string{"extend", convertSpec, in}, &out, &stderr); code != 0 || out.String() != "text:Hello\n" {
		t.Fatalf("convert %d %q %s", code, out.String(), stderr.String())
	}
	failSpec := writeSpec(t, filepath.Join(dir, "convert-fail.json"), extend.Spec{Version: 1, Kind: "convert", Args: []string{convertFail, "%in"}})
	out.Reset()
	stderr.Reset()
	if code := run([]string{"extend", failSpec, in}, &out, &stderr); code != 2 || out.Len() != 0 || stderr.String() != "extension exit 2: boom\n" {
		t.Fatalf("convert fail %d %q %s", code, out.String(), stderr.String())
	}

	packed := filepath.Join(dir, "packed.bin")
	packSpec := writeSpec(t, filepath.Join(dir, "pack.json"), extend.Spec{Version: 1, Kind: "compress", Args: []string{packTool, "%in", "%out"}})
	raw := filepath.Join(dir, "raw.bin")
	if err = os.WriteFile(raw, []byte("raw"), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	stderr.Reset()
	if code := run([]string{"extend", packSpec, raw, packed}, &out, &stderr); code != 0 || out.String() != "packed 8 bytes\n" {
		t.Fatalf("pack %d %q %s", code, out.String(), stderr.String())
	}
	got, err := os.ReadFile(packed)
	if err != nil || string(got) != "PACK:raw" {
		t.Fatalf("packed file %q %v", got, err)
	}
	missingSpec := writeSpec(t, filepath.Join(dir, "missing.json"), extend.Spec{Version: 1, Kind: "compress", Args: []string{packMissing, "%in", "%out"}})
	missingOut := filepath.Join(dir, "missing.bin")
	out.Reset()
	stderr.Reset()
	if code := run([]string{"extend", missingSpec, raw, missingOut}, &out, &stderr); code != 2 || out.Len() != 0 || stderr.String() != "extension output missing\n" {
		t.Fatalf("missing %d %q %s", code, out.String(), stderr.String())
	}
	if _, err = os.Stat(missingOut); !os.IsNotExist(err) {
		t.Fatalf("missing output exists: %v", err)
	}

	badKind := writeSpec(t, filepath.Join(dir, "bad.json"), extend.Spec{Version: 1, Kind: "nope", Args: []string{"/bin/sh"}})
	out.Reset()
	stderr.Reset()
	if code := run([]string{"extend", badKind, raw}, &out, &stderr); code != 2 || stderr.String() != "unknown extension kind\n" {
		t.Fatalf("kind %d %q %s", code, out.String(), stderr.String())
	}
	huge := writeTool(t, filepath.Join(dir, "huge.sh"), "#!/bin/sh\ndd if=/dev/zero bs=1048576 count=16 2>/dev/null | tr '\\0' 'x'\nprintf x\nexit 0\n")
	hugeSpec := writeSpec(t, filepath.Join(dir, "huge.json"), extend.Spec{Version: 1, Kind: "convert", Args: []string{huge, "%in"}})
	out.Reset()
	stderr.Reset()
	if code := run([]string{"extend", hugeSpec, in}, &out, &stderr); code != 2 || !bytes.Contains(stderr.Bytes(), []byte("extension output exceeds 16 MiB")) || bytes.Contains(out.Bytes(), []byte("xxx")) {
		t.Fatalf("huge %d out %d stderr %s", code, out.Len(), stderr.String())
	}
}

func writeSpec(t *testing.T, path string, spec extend.Spec) string {
	t.Helper()
	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeTool(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeCLIID3(t *testing.T, path, title, artist, album string) {
	t.Helper()
	frames := append(cliTextFrame("TIT2", 0, append([]byte(title), 0)), cliTextFrame("TPE1", 1, cliUTF16(artist))...)
	frames = append(frames, cliTextFrame("TALB", 0, append([]byte(album), 0))...)
	data := make([]byte, 10+len(frames))
	copy(data[0:3], "ID3")
	data[3] = 3
	data[6] = byte(len(frames) >> 21 & 0x7f)
	data[7] = byte(len(frames) >> 14 & 0x7f)
	data[8] = byte(len(frames) >> 7 & 0x7f)
	data[9] = byte(len(frames) & 0x7f)
	copy(data[10:], frames)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func cliTextFrame(id string, enc byte, text []byte) []byte {
	payload := append([]byte{enc}, text...)
	frame := make([]byte, 10+len(payload))
	copy(frame[0:4], id)
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(payload)))
	copy(frame[10:], payload)
	return frame
}

func cliUTF16(s string) []byte {
	u := utf16.Encode([]rune(s))
	b := []byte{0xff, 0xfe}
	for _, v := range u {
		b = append(b, byte(v), byte(v>>8))
	}
	return append(b, 0, 0)
}

func TestPEAndRegistryCLI(t *testing.T) {
	dir := t.TempDir()
	leftPE := filepath.Join(dir, "left.exe")
	rightPE := filepath.Join(dir, "right.exe")
	samePE := filepath.Join(dir, "same.exe")
	if err := os.WriteFile(leftPE, cliPEWithVersion("1.2.3.4"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rightPE, cliPEWithVersion("9.8.7.6"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(samePE, cliPEWithVersion("1.2.3.4"), 0o600); err != nil {
		t.Fatal(err)
	}
	alignedPE := filepath.Join(dir, "aligned.exe")
	if err := os.WriteFile(alignedPE, cliPEWithBlob(cliAlignedVersion("1.2.3.4")), 0o600); err != nil {
		t.Fatal(err)
	}
	beforePE, err := os.ReadFile(leftPE)
	if err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	if code := run([]string{"pe", leftPE, rightPE}, &out, &stderr); code != 1 || out.String() != "left 1.2.3.4\nright 9.8.7.6\n" || stderr.Len() != 0 {
		t.Fatalf("pe diff %d %q %s", code, out.String(), stderr.String())
	}
	out.Reset()
	if code := run([]string{"pe", leftPE, samePE}, &out, &stderr); code != 0 || out.String() != "left 1.2.3.4\nright 1.2.3.4\n" {
		t.Fatalf("pe same %d %q %s", code, out.String(), stderr.String())
	}
	out.Reset()
	stderr.Reset()
	if code := run([]string{"pe", alignedPE, samePE}, &out, &stderr); code != 0 || out.String() != "left 1.2.3.4\nright 1.2.3.4\n" || stderr.Len() != 0 {
		t.Fatalf("pe aligned %d %q %s", code, out.String(), stderr.String())
	}
	note := filepath.Join(dir, "note.txt")
	if err = os.WriteFile(note, []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	stderr.Reset()
	if code := run([]string{"pe", leftPE, note}, &out, &stderr); code != 2 || stderr.String() != "not a PE file\n" || out.Len() != 0 {
		t.Fatalf("pe reject %d %q %s", code, out.String(), stderr.String())
	}
	out.Reset()
	stderr.Reset()
	if code := run([]string{"pe", leftPE}, &out, &stderr); code != 2 || !bytes.Contains(stderr.Bytes(), []byte("Usage: gocompare-cli pe")) || out.Len() != 0 {
		t.Fatalf("pe usage %d %q %s", code, out.String(), stderr.String())
	}
	afterPE, err := os.ReadFile(leftPE)
	if err != nil || !bytes.Equal(beforePE, afterPE) {
		t.Fatal("pe read changed the file")
	}

	leftReg := filepath.Join(dir, "left.reg")
	rightReg := filepath.Join(dir, "right.reg")
	leftText := "Windows Registry Editor Version 5.00\n\n[HKEY_CURRENT_USER\\Software\\GoCompare]\n\"Name\"=\"One\"\n\"Extra\"=\"Keep\"\n"
	rightText := "Windows Registry Editor Version 5.00\n\n[HKEY_CURRENT_USER\\Software\\GoCompare]\n\"Name\"=\"Two\"\n"
	if err = os.WriteFile(leftReg, []byte(leftText), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(rightReg, []byte(rightText), 0o600); err != nil {
		t.Fatal(err)
	}
	beforeReg, err := os.ReadFile(leftReg)
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	stderr.Reset()
	wantDiff := "HKEY_CURRENT_USER\\Software\\GoCompare\\Extra\nHKEY_CURRENT_USER\\Software\\GoCompare\\Name\n"
	if code := run([]string{"reg", leftReg, rightReg}, &out, &stderr); code != 1 || out.String() != wantDiff || stderr.Len() != 0 {
		t.Fatalf("reg diff %d %q %s", code, out.String(), stderr.String())
	}
	out.Reset()
	if code := run([]string{"reg", leftReg, leftReg}, &out, &stderr); code != 0 || out.String() != "equal\n" {
		t.Fatalf("reg same %d %q %s", code, out.String(), stderr.String())
	}
	out.Reset()
	stderr.Reset()
	if code := run([]string{"reg", leftReg, note}, &out, &stderr); code != 2 || stderr.String() != "unsupported registry header: hello\n" || out.Len() != 0 {
		t.Fatalf("reg reject %d %q %s", code, out.String(), stderr.String())
	}
	afterReg, err := os.ReadFile(leftReg)
	if err != nil || !bytes.Equal(beforeReg, afterReg) {
		t.Fatal("reg read changed the file")
	}
}

func cliPEWithVersion(version string) []byte {
	return cliPEWithBlob(append(cliUTF16Z("FileVersion"), cliUTF16Z(version)...))
}

func cliAlignedVersion(version string) []byte {
	key := cliUTF16Z("FileVersion")
	value := cliUTF16Z(version)
	body := make([]byte, 6+len(key))
	copy(body[6:], key)
	for len(body)%4 != 0 {
		body = append(body, 0)
	}
	body = append(body, value...)
	binary.LittleEndian.PutUint16(body[0:], uint16(len(body)))
	binary.LittleEndian.PutUint16(body[2:], uint16(len(value)/2))
	binary.LittleEndian.PutUint16(body[4:], 1)
	return body
}

func cliPEWithBlob(blob []byte) []byte {
	const blobAt = 88
	sec := make([]byte, blobAt+len(blob))
	cliPutDir(sec, 0, 16, 24, true)
	cliPutDir(sec, 24, 1, 48, true)
	cliPutDir(sec, 48, 0, 72, false)
	const sectionVA = 0x1000
	binary.LittleEndian.PutUint32(sec[72:], uint32(sectionVA+blobAt))
	binary.LittleEndian.PutUint32(sec[76:], uint32(len(blob)))
	copy(sec[blobAt:], blob)
	file := make([]byte, 0x80+len(sec))
	file[0], file[1] = 'M', 'Z'
	binary.LittleEndian.PutUint32(file[0x3c:], 0x40)
	copy(file[0x40:], []byte{'P', 'E', 0, 0})
	binary.LittleEndian.PutUint16(file[0x40+4+2:], 1)
	off := 0x40 + 4 + 20
	copy(file[off:off+8], []byte(".rsrc"))
	binary.LittleEndian.PutUint32(file[off+8:], uint32(len(sec)))
	binary.LittleEndian.PutUint32(file[off+12:], sectionVA)
	binary.LittleEndian.PutUint32(file[off+16:], uint32(len(sec)))
	binary.LittleEndian.PutUint32(file[off+20:], 0x80)
	copy(file[0x80:], sec)
	return file
}

func cliPutDir(b []byte, off, id, target int, sub bool) {
	binary.LittleEndian.PutUint16(b[off+14:], 1)
	binary.LittleEndian.PutUint32(b[off+16:], uint32(id))
	rel := uint32(target)
	if sub {
		rel |= 0x80000000
	}
	binary.LittleEndian.PutUint32(b[off+20:], rel)
}

func cliUTF16Z(s string) []byte {
	u := utf16.Encode([]rune(s))
	out := make([]byte, (len(u)+1)*2)
	for i, v := range u {
		binary.LittleEndian.PutUint16(out[i*2:], v)
	}
	return out
}

func writeSolidPNG(t *testing.T, path string, a, b color.RGBA) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.SetRGBA(0, 0, a)
	img.SetRGBA(1, 0, b)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// Git 在调用 mergetool 前已写好 $MERGED，必须能替换它；merge 子命令仍然不覆盖已有文件。
func TestMergetoolReplacesExistingMerged(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	base, left, right := write("base", "a\nb\nc\n"), write("left", "a\nL\nc\n"), write("right", "a\nb\nc\n")
	merged := write("merged", "a\n<<<<<<< ours\nL\n=======\nb\n>>>>>>> theirs\nc\n")
	var out, stderr bytes.Buffer
	if code := run(integrate.MergetoolArgs(base, left, right, merged), &out, &stderr); code != 0 {
		t.Fatalf("mergetool %d %s", code, stderr.String())
	}
	if data, _ := os.ReadFile(merged); string(data) != "a\nL\nc\n" {
		t.Fatalf("merged %q", data)
	}
	stderr.Reset()
	if code := run([]string{"merge", "--base", base, "--output", merged, left, right}, &out, &stderr); code != 2 {
		t.Fatalf("merge must refuse an existing output, got %d", code)
	}
}

// 合并和文件比较一样自动识别 GBK，输出沿用左侧编码。
func TestMergeDetectsGBK(t *testing.T) {
	dir := t.TempDir()
	gbk := func(name string, raw []byte) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	// “中文” 的 GBK 编码是 D6 D0 CE C4。
	base := gbk("base", []byte("a\n\xd6\xd0\xce\xc4\n"))
	left := gbk("left", []byte("x\n\xd6\xd0\xce\xc4\n"))
	right := gbk("right", []byte("a\n\xd6\xd0\xce\xc4\n"))
	output := filepath.Join(dir, "out")
	var out, stderr bytes.Buffer
	if code := run([]string{"merge", "--base", base, "--output", output, left, right}, &out, &stderr); code != 0 {
		t.Fatalf("merge %d %s", code, stderr.String())
	}
	if data, _ := os.ReadFile(output); string(data) != "x\n\xd6\xd0\xce\xc4\n" {
		t.Fatalf("output %x", data)
	}
}

func TestVersionFlag(t *testing.T) {
	old := brand.Version
	brand.Version = "v9.8.7"
	defer func() { brand.Version = old }()
	var out, stderr bytes.Buffer
	if code := run([]string{"--version"}, &out, &stderr); code != 0 || out.String() != "gocompare-cli v9.8.7\n" {
		t.Fatalf("%d %q", code, out.String())
	}
}

func TestTableArchiveSnapshotRemoteAndDoctextCLI(t *testing.T) {
	dir := t.TempDir()

	// 1. Table CLI
	tLeft := filepath.Join(dir, "left.csv")
	tRight := filepath.Join(dir, "right.csv")
	os.WriteFile(tLeft, []byte("id,val\n1,10.0\n2,20.0\n"), 0o600)
	os.WriteFile(tRight, []byte("id,val\n1,10.005\n2,25.0\n"), 0o600)

	var out, stderr bytes.Buffer
	// Without tolerance, different
	if code := run([]string{"table", tLeft, tRight}, &out, &stderr); code != 1 {
		t.Fatalf("table expected 1, got %d (err: %s)", code, stderr.String())
	}
	out.Reset()
	stderr.Reset()
	// With keys and tolerance 0.01, row 1 is equal (10.0 vs 10.005), row 2 is diff
	if code := run([]string{"table", "--keys", "0", "--tolerance", "0.01", tLeft, tRight}, &out, &stderr); code != 1 {
		t.Fatalf("table expected 1, got %d", code)
	}
	if !strings.Contains(out.String(), "20.0 != 25.0") {
		t.Fatalf("unexpected table output: %s", out.String())
	}
	out.Reset()
	stderr.Reset()
	// Equal table
	if code := run([]string{"table", tLeft, tLeft}, &out, &stderr); code != 0 || strings.TrimSpace(out.String()) != "equal" {
		t.Fatalf("expected table equal 0, got %d %q", code, out.String())
	}

	// 2. Archive CLI
	f1 := filepath.Join(dir, "f1.txt")
	f2 := filepath.Join(dir, "f2.txt")
	os.WriteFile(f1, []byte("hello"), 0o600)
	os.WriteFile(f2, []byte("world"), 0o600)
	zip1 := filepath.Join(dir, "a.zip")
	zip2 := filepath.Join(dir, "b.zip")
	out.Reset()
	stderr.Reset()
	if code := run([]string{"archive", "pack", "zip", zip1, f1, f2}, &out, &stderr); code != 0 {
		t.Fatalf("archive pack: %d %s", code, stderr.String())
	}
	out.Reset()
	stderr.Reset()
	if code := run([]string{"archive", "pack", "zip", zip2, f1}, &out, &stderr); code != 0 {
		t.Fatalf("archive pack 2: %d %s", code, stderr.String())
	}
	out.Reset()
	stderr.Reset()
	if code := run([]string{"archive", "list", zip1}, &out, &stderr); code != 0 || !strings.Contains(out.String(), "f1.txt") {
		t.Fatalf("archive list: %d %s", code, out.String())
	}
	out.Reset()
	stderr.Reset()
	if code := run([]string{"archive", "compare", zip1, zip2}, &out, &stderr); code != 1 {
		t.Fatalf("archive compare diff expected 1, got %d", code)
	}
	out.Reset()
	stderr.Reset()
	if code := run([]string{"archive", "compare", zip1, zip1}, &out, &stderr); code != 0 || strings.TrimSpace(out.String()) != "equal" {
		t.Fatalf("archive compare same expected 0, got %d %s", code, out.String())
	}

	// 3. Snapshot CLI
	snapDir := filepath.Join(dir, "snap_src")
	os.MkdirAll(filepath.Join(snapDir, "sub"), 0o755)
	os.WriteFile(filepath.Join(snapDir, "sub", "x.txt"), []byte("123"), 0o600)
	snapFile1 := filepath.Join(dir, "s1.snp")
	snapFile2 := filepath.Join(dir, "s2.snp")
	out.Reset()
	stderr.Reset()
	if code := run([]string{"snapshot", "create", snapDir, snapFile1}, &out, &stderr); code != 0 {
		t.Fatalf("snapshot create: %d %s", code, stderr.String())
	}
	out.Reset()
	stderr.Reset()
	if code := run([]string{"snapshot", "list", snapFile1}, &out, &stderr); code != 0 || !strings.Contains(out.String(), "sub/x.txt") {
		t.Fatalf("snapshot list: %d %s", code, out.String())
	}
	// Change file
	os.WriteFile(filepath.Join(snapDir, "sub", "x.txt"), []byte("123456"), 0o600)
	out.Reset()
	stderr.Reset()
	if code := run([]string{"snapshot", "create", snapDir, snapFile2}, &out, &stderr); code != 0 {
		t.Fatalf("snapshot create 2: %d %s", code, stderr.String())
	}
	out.Reset()
	stderr.Reset()
	if code := run([]string{"snapshot", "compare", snapFile1, snapFile2}, &out, &stderr); code != 1 {
		t.Fatalf("snapshot compare diff: %d", code)
	}
	out.Reset()
	stderr.Reset()
	if code := run([]string{"snapshot", "compare", snapFile1, snapFile1}, &out, &stderr); code != 0 || strings.TrimSpace(out.String()) != "equal" {
		t.Fatalf("snapshot compare equal: %d %s", code, out.String())
	}

	// 4. DocText CLI
	rtfFile := filepath.Join(dir, "test.rtf")
	os.WriteFile(rtfFile, []byte("{\\rtf1 Hello \\par DocText}"), 0o600)
	out.Reset()
	stderr.Reset()
	if code := run([]string{"doctext", rtfFile}, &out, &stderr); code != 0 || out.String() != "Hello DocText" {
		t.Fatalf("doctext rtf: %d %q", code, out.String())
	}

	// 5. Remote CLI (WebDAV test server)
	files := make(map[string][]byte)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch r.Method {
		case http.MethodGet:
			data, ok := files[p]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Write(data)
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			files[p] = body
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			delete(files, p)
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	u := strings.Replace(srv.URL, "http://", "webdav://", 1) + "/remote.txt"
	localFile := filepath.Join(dir, "local.txt")
	os.WriteFile(localFile, []byte("remote content"), 0o600)

	out.Reset()
	stderr.Reset()
	if code := run([]string{"remote", "write", u, localFile}, &out, &stderr); code != 0 {
		t.Fatalf("remote write: %d %s", code, stderr.String())
	}
	out.Reset()
	stderr.Reset()
	if code := run([]string{"remote", "read", u}, &out, &stderr); code != 0 || out.String() != "remote content" {
		t.Fatalf("remote read: %d %q", code, out.String())
	}
	out.Reset()
	stderr.Reset()
	if code := run([]string{"remote", "compare", u, u}, &out, &stderr); code != 0 || strings.TrimSpace(out.String()) != "equal" {
		t.Fatalf("remote compare equal: %d %s", code, out.String())
	}
	out.Reset()
	stderr.Reset()
	if code := run([]string{"remote", "delete", u}, &out, &stderr); code != 0 {
		t.Fatalf("remote delete: %d %s", code, stderr.String())
	}
}

func TestDirMergeCLI(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base")
	left := filepath.Join(dir, "left")
	right := filepath.Join(dir, "right")
	outDir := filepath.Join(dir, "merged_out")

	os.MkdirAll(base, 0o755)
	os.MkdirAll(left, 0o755)
	os.MkdirAll(right, 0o755)

	os.WriteFile(filepath.Join(base, "f.txt"), []byte("base\n"), 0o600)
	os.WriteFile(filepath.Join(left, "f.txt"), []byte("left\n"), 0o600)
	os.WriteFile(filepath.Join(right, "f.txt"), []byte("base\n"), 0o600)
	os.WriteFile(filepath.Join(right, "r_only.txt"), []byte("r\n"), 0o600)

	var out, stderr bytes.Buffer
	// 3-way dirmerge: left change adopted, right only adopted -> clean merge (code 0)
	if code := run([]string{"dirmerge", "--base", base, "--output", outDir, left, right}, &out, &stderr); code != 0 {
		t.Fatalf("dirmerge expected 0, got %d (%s)", code, stderr.String())
	}
	if data, _ := os.ReadFile(filepath.Join(outDir, "f.txt")); string(data) != "left\n" {
		t.Fatalf("merged f.txt = %q, want left", string(data))
	}
	if data, _ := os.ReadFile(filepath.Join(outDir, "r_only.txt")); string(data) != "r\n" {
		t.Fatalf("merged r_only.txt = %q, want r", string(data))
	}

	// 2-way dirmerge with conflict
	out.Reset()
	stderr.Reset()
	conflictOut := filepath.Join(dir, "conflict_out")
	if code := run([]string{"dirmerge", "--output", conflictOut, left, right}, &out, &stderr); code != 1 {
		t.Fatalf("dirmerge conflict expected 1, got %d", code)
	}
	if !strings.Contains(out.String(), "conflict\tf.txt") {
		t.Fatalf("expected conflict in output, got %s", out.String())
	}
}

func TestCheckUpdateCLI(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(updater.Release{
			TagName: "v9.9.9",
			Body:    "Major new features",
			HTMLURL: "https://github.com/jhzhang09/gocompare/releases/tag/v9.9.9",
		})
	}))
	defer ts.Close()

	var out, stderr bytes.Buffer
	// Test newer available -> code 1
	code := run([]string{"check-update", "--url", ts.URL}, &out, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1 for newer update, got %d, out: %s, err: %s", code, out.String(), stderr.String())
	}
	if !strings.Contains(out.String(), "New version available: v9.9.9") {
		t.Fatalf("expected output to mention v9.9.9, got: %s", out.String())
	}

	// Test current version already same as remote -> code 0
	old := brand.Version
	brand.Version = "v9.9.9"
	defer func() { brand.Version = old }()
	out.Reset()
	stderr.Reset()
	code = run([]string{"check-update", "--url", ts.URL}, &out, &stderr)
	if code != 0 {
		t.Fatalf("expected code 0 when up-to-date, got %d", code)
	}
	if !strings.Contains(out.String(), "up to date") {
		t.Fatalf("expected output to say up to date, got: %s", out.String())
	}
}

func TestACLCLI(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "f1.txt")
	f2 := filepath.Join(dir, "f2.txt")
	f3 := filepath.Join(dir, "f3.txt")

	_ = os.WriteFile(f1, []byte("data"), 0o644)
	_ = os.WriteFile(f2, []byte("data"), 0o644)
	_ = os.WriteFile(f3, []byte("data"), 0o755)

	var out, stderr bytes.Buffer
	// 1. Same permissions -> code 0
	code := run([]string{"acl", f1, f2}, &out, &stderr)
	if runtime.GOOS == "windows" {
		if code != 2 || !strings.Contains(stderr.String(), "not supported") {
			t.Fatalf("unsupported ACL must fail: %d %s", code, stderr.String())
		}
		return
	}
	if code != 0 {
		t.Fatalf("expected code 0 for matching permissions, got %d (err: %s)", code, stderr.String())
	}
	if !strings.Contains(out.String(), "match") {
		t.Fatalf("expected output to contain match, got %s", out.String())
	}

	// 2. Different permissions -> code 1
	out.Reset()
	stderr.Reset()
	code = run([]string{"acl", f1, f3}, &out, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1 for differing permissions, got %d", code)
	}

	// 3. Missing file -> code 2
	out.Reset()
	stderr.Reset()
	code = run([]string{"acl", f1, filepath.Join(dir, "missing.txt")}, &out, &stderr)
	if code != 2 {
		t.Fatalf("expected code 2 on missing file, got %d", code)
	}
}

func TestCLIOutputsNeverOverwrite(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	os.WriteFile(a, []byte("a"), 0o600)
	existing := filepath.Join(dir, "existing")
	os.WriteFile(existing, []byte("keep"), 0o600)
	var out, stderr bytes.Buffer
	if code := run([]string{"archive", "pack", "zip", existing, a}, &out, &stderr); code != 2 {
		t.Fatalf("pack overwrote existing output: %d", code)
	}
	if code := run([]string{"snapshot", "create", dir, existing}, &out, &stderr); code != 2 {
		t.Fatalf("snapshot overwrote existing output: %d", code)
	}
	if data, _ := os.ReadFile(existing); string(data) != "keep" {
		t.Fatalf("existing output changed: %q", data)
	}
	sub := filepath.Join(dir, "sub")
	os.Mkdir(sub, 0o700)
	b := filepath.Join(sub, "a.txt")
	os.WriteFile(b, []byte("b"), 0o600)
	stderr.Reset()
	if code := run([]string{"archive", "pack", "zip", filepath.Join(dir, "dup.zip"), a, b}, &out, &stderr); code != 2 || !strings.Contains(stderr.String(), "duplicate") {
		t.Fatalf("duplicate entry names accepted: %d %s", code, stderr.String())
	}
}

func TestCLITableMapAndValidation(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "l.csv")
	right := filepath.Join(dir, "r.csv")
	os.WriteFile(left, []byte("id,name\n1,a\n"), 0o600)
	os.WriteFile(right, []byte("name,id\na,1\n"), 0o600)
	var out, stderr bytes.Buffer
	if code := run([]string{"table", "--keys", "0", "--map", "1,0", left, right}, &out, &stderr); code != 0 {
		t.Fatalf("mapped columns should match: %d %s %s", code, out.String(), stderr.String())
	}
	for _, args := range [][]string{
		{"table", "--keys", "-1", left, right},
		{"table", "--map", "0,-2", left, right},
		{"table", "--tolerance", "-1", left, right},
	} {
		if code := run(args, &out, &stderr); code != 2 {
			t.Fatalf("%v accepted: %d", args, code)
		}
	}
}

func TestCLIRemoteTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()
	u := strings.Replace(srv.URL, "http://", "webdav://", 1) + "/slow"
	var out, stderr bytes.Buffer
	start := time.Now()
	if code := run([]string{"remote", "--timeout", "200ms", "read", u}, &out, &stderr); code != 2 {
		t.Fatalf("timeout read: %d", code)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("remote read ignored --timeout")
	}
}

func TestCLIRemoteSFTPFlags(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no_known_hosts")
	var out, stderr bytes.Buffer
	code := run([]string{"remote", "--known-hosts", missing, "read", "sftp://u:p@127.0.0.1:1/x"}, &out, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), missing) {
		t.Fatalf("--known-hosts not applied: %d %s", code, stderr.String())
	}
	stderr.Reset()
	empty := filepath.Join(t.TempDir(), "known_hosts")
	os.WriteFile(empty, nil, 0o600)
	badKey := filepath.Join(t.TempDir(), "id")
	code = run([]string{"remote", "--known-hosts", empty, "--identity", badKey, "read", "sftp://u@127.0.0.1:1/x"}, &out, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "read identity") {
		t.Fatalf("--identity not applied: %d %s", code, stderr.String())
	}
}
