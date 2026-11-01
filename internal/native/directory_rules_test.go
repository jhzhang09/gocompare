package native

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"gocompare/internal/compare"
)

func TestWorkspaceDirectoryRules(t *testing.T) {
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
	a := &workspaceState{paths: [2]string{left, right}}
	a.optionsOpen = true
	tt := ui.NewTester(a.view, 1600, 880)
	tt.Frame()
	if err = tt.Click("目录比较"); err != nil {
		t.Fatal(err)
	}
	if err = tt.Click("比较权限"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.compareMode {
		t.Fatal("permission rule did not toggle")
	}
	if err = tt.Click("开始比较"); err != nil {
		t.Fatal(err)
	}
	if a.err != "" || a.kind != 1 || directoryStatus(a, "nested/a.txt") != compare.Different || directoryStatus(a, "nested/b.bin") != compare.Different {
		t.Fatalf("mode rows err=%s %+v", a.err, a.directoryRows)
	}
	if !tt.HasText("nested/a.txt") || !tt.HasText("nested/b.bin") || !tt.HasText("不同") || tt.HasText("相同") {
		t.Fatalf("mode texts %v", tt.Texts())
	}
	assertScanOrder(t, a.scanSeen)
	if a.scanPath == "" {
		t.Fatalf("scan path %q texts %v", a.scanPath, tt.Texts())
	}
	if err = tt.Click("比较权限"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.compareMode {
		t.Fatal("permission rule stayed on")
	}
	if err = tt.Click("开始比较"); err != nil {
		t.Fatal(err)
	}
	if directoryStatus(a, "nested/a.txt") != compare.Same || directoryStatus(a, "nested/b.bin") != compare.Different {
		t.Fatalf("default rows %+v", a.directoryRows)
	}
	if !tt.HasText("相同") || !tt.HasText("不同") {
		t.Fatalf("default texts %v", tt.Texts())
	}
	if err = tt.Click("比较时间"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err = tt.Click("开始比较"); err != nil {
		t.Fatal(err)
	}
	if !a.compareTime || directoryStatus(a, "nested/a.txt") != compare.Different {
		t.Fatalf("time rows %+v", a.directoryRows)
	}
	if err = tt.Click("比较时间"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err = tt.Click("目录过滤"); err != nil {
		t.Fatal(err)
	}
	tt.Type("nested/*.txt")
	tt.Frame()
	if a.dirFilter != "nested/*.txt" {
		t.Fatalf("filter %q", a.dirFilter)
	}
	if err = tt.Click("开始比较"); err != nil {
		t.Fatal(err)
	}
	_, filteredBin := tt.Find("nested/b.bin")
	_, keptText := tt.Find("nested/a.txt")
	if a.err != "" || directoryStatus(a, "nested/a.txt") != compare.Same || directoryStatus(a, "nested/b.bin") != "" || filteredBin || !keptText || !tt.HasText("相同") || tt.HasText("不同") {
		t.Fatalf("filter err=%s texts=%v rows=%+v", a.err, tt.Texts(), a.directoryRows)
	}
	if a.scanPath != "nested/b.bin" {
		t.Fatalf("filtered name left the scan: %q", a.scanPath)
	}
	assertScanOrder(t, a.scanSeen)
	for path, data := range before {
		got, readErr := os.ReadFile(path)
		if readErr != nil || !bytes.Equal(got, data) {
			t.Fatalf("bytes changed %s", path)
		}
	}
}

func TestWorkspaceDirectoryRejectsBadFilter(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	for _, root := range []string{left, right} {
		if err := os.Mkdir(filepath.Join(root, "nested"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "nested", "a.txt"), []byte("same"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	sample := filepath.Join(left, "nested", "a.txt")
	before, err := os.ReadFile(sample)
	if err != nil {
		t.Fatal(err)
	}
	a := &workspaceState{paths: [2]string{left, right}}
	a.optionsOpen = true
	tt := ui.NewTester(a.view, 1600, 880)
	tt.Frame()
	if err = tt.Click("目录比较"); err != nil {
		t.Fatal(err)
	}
	if err = tt.Click("目录过滤"); err != nil {
		t.Fatal(err)
	}
	tt.Type("[")
	tt.Frame()
	if err = tt.Click("开始比较"); err != nil {
		t.Fatal(err)
	}
	if a.kind != 1 || a.directoryRows != nil || a.err == "" || !bytes.Contains([]byte(a.err), []byte("syntax error in pattern")) {
		t.Fatalf("kind=%d rows=%v err=%s", a.kind, a.directoryRows, a.err)
	}
	if !tt.HasText("错误") || tt.HasText("nested/a.txt") {
		t.Fatalf("texts %v", tt.Texts())
	}
	got, err := os.ReadFile(sample)
	if err != nil || !bytes.Equal(got, before) {
		t.Fatal("bytes changed")
	}
}

func TestWorkspaceDirectoryLinkPolicy(t *testing.T) {
	left, right, outside := t.TempDir(), t.TempDir(), t.TempDir()
	secret := filepath.Join(outside, "secret")
	if err := os.WriteFile(secret, []byte("hidden"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(left, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(right, "link")); err != nil {
		t.Fatal(err)
	}
	a := &workspaceState{paths: [2]string{left, right}}
	a.optionsOpen = true
	tt := ui.NewTester(a.view, 1600, 880)
	tt.Frame()
	if err := tt.Click("目录比较"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("开始比较"); err != nil {
		t.Fatal(err)
	}
	row := directoryRow(a, "link")
	if a.err != "" || a.followLinks || row.Status != compare.Same || row.Error != "" || tt.HasText("读取失败") {
		t.Fatalf("record err=%s follow=%v row=%+v texts=%v", a.err, a.followLinks, row, tt.Texts())
	}
	if err := tt.Click("跟随链接"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("开始比较"); err != nil {
		t.Fatal(err)
	}
	row = directoryRow(a, "link")
	if !a.followLinks || row.Status != compare.Failed || row.Error != "symlink escapes root" || !tt.HasText("读取失败") || !tt.HasText("link") {
		t.Fatalf("follow err=%s row=%+v texts=%v", a.err, row, tt.Texts())
	}
	for _, text := range tt.Texts() {
		if text == "hidden" {
			t.Fatal("followed file content is visible")
		}
	}
	got, err := os.ReadFile(secret)
	if err != nil || string(got) != "hidden" {
		t.Fatal("secret changed")
	}
}

func directoryStatus(a *workspaceState, path string) compare.Status {
	return directoryRow(a, path).Status
}

func directoryRow(a *workspaceState, path string) compare.Entry {
	for _, row := range a.directoryRows {
		if row.Path == path {
			return row
		}
	}
	return compare.Entry{}
}

func assertScanOrder(t *testing.T, seen []string) {
	t.Helper()
	parent, child := -1, -1
	for i, rel := range seen {
		if rel == "nested" && parent < 0 {
			parent = i
		}
		if rel == "nested/a.txt" && child < 0 {
			child = i
		}
	}
	if parent < 0 || child < 0 || parent > child {
		t.Fatalf("progress order: %v", seen)
	}
}

func TestWorkspaceDirectoryMergePreview(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(left, "a.txt"), []byte("left\n"), 0o600)
	os.WriteFile(filepath.Join(right, "a.txt"), []byte("right\n"), 0o600)
	os.WriteFile(filepath.Join(right, "b.txt"), []byte("right only\n"), 0o600)

	a := &workspaceState{paths: [2]string{left, right}, directoryRoots: [2]string{left, right}}
	a.previewDirMerge()

	if len(a.dirMergeDecisions) != 2 {
		t.Fatalf("expected 2 decisions, got %d", len(a.dirMergeDecisions))
	}

	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()

	// 重新安装目录结果（例如换根目录重新比较）后不能继续显示旧的合并预览。
	a.installDirectory([2]string{left, right}, nil)
	if a.dirMergeDecisions != nil {
		t.Fatal("stale directory merge preview survived a rescan")
	}
}
