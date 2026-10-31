package operations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Version 用于在预览和执行之间检测文件变化。目录不比较时间，避免自身子项操作造成误报。
type Version struct {
	Exists    bool   `json:"exists"`
	Directory bool   `json:"directory"`
	Mode      uint32 `json:"mode"`
	Size      int64  `json:"size"`
	Modified  int64  `json:"modified"`
	Hash      string `json:"hash,omitempty"`
}

// Record 保存已完成的操作与原内容备份，用于后续恢复和审计。
type Record struct {
	Action Action `json:"action"`
	Backup string `json:"backup,omitempty"`
}

// Report 即使执行失败也返回已经完成的项，禁止用整批成功掩盖部分执行。
// Completed 是仍然生效的操作；Undone 是已成功撤销的操作。RolledBack 表示没有仍生效的操作。
type Report struct {
	Version       int      `json:"version"`
	Completed     []Record `json:"completed"`
	Undone        []Record `json:"undone,omitempty"`
	Journal       string   `json:"journal"`
	Pending       *Record  `json:"pending,omitempty"`
	RolledBack    bool     `json:"rolled_back,omitempty"`
	RollbackError string   `json:"rollback_error,omitempty"`
	Paused        bool     `json:"paused,omitempty"`
	Plan          *Plan    `json:"plan,omitempty"`
	Next          int      `json:"next,omitempty"`
}

var ErrChanged = errors.New("operation inputs changed since preview")

// ErrPaused 表示执行在指定动作前停下，已完成的动作保留，日志可被 Resume 继续。
var ErrPaused = errors.New("sync paused")

func fingerprint(ctx context.Context, path string) (Version, error) {
	var v Version
	if err := ctx.Err(); err != nil {
		return v, err
	}
	st, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return v, fmt.Errorf("symlink not allowed: %s", path)
	}
	v = Version{Exists: true, Directory: st.IsDir(), Mode: uint32(st.Mode().Perm())}
	if v.Directory {
		return v, nil
	}
	if !st.Mode().IsRegular() {
		return v, fmt.Errorf("not a regular file: %s", path)
	}
	v.Size = st.Size()
	v.Modified = st.ModTime().UnixNano()
	f, err := os.Open(path)
	if err != nil {
		return v, err
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, 64<<10)
	for {
		if err := ctx.Err(); err != nil {
			return v, err
		}
		n, err := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return v, err
		}
	}
	v.Hash = hex.EncodeToString(h.Sum(nil))
	return v, nil
}
func safePath(root, relative string) (string, error) {
	if !filepath.IsAbs(root) || !filepath.IsLocal(filepath.FromSlash(relative)) || filepath.Clean(relative) == "." {
		return "", errors.New("invalid operation path")
	}
	for current := root; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("symlink in root path: %s", current)
		}
		if current == filepath.Dir(current) {
			break
		}
	}
	path := root
	for _, part := range splitPath(relative) {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("symlink in operation path: %s", path)
		}
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}
	return path, nil
}
func splitPath(path string) []string {
	var parts []string
	for path != "." && path != "" {
		dir, base := filepath.Split(filepath.Clean(filepath.FromSlash(path)))
		parts = append([]string{base}, parts...)
		path = filepath.Clean(dir)
	}
	return parts
}
func matches(ctx context.Context, path string, want Version) error {
	got, err := fingerprint(ctx, path)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("%w: %s", ErrChanged, path)
	}
	return nil
}

// Execute 执行明确确认的计划。破坏性文件操作先备份，遇到错误停止。
// 若已经有操作生效，会按相反顺序尝试回滚到执行前；回滚本身失败时保留仍生效的前缀。
// 本地检查与重命名之间不提供跨进程原子 CAS。
func Execute(ctx context.Context, p Plan, backupRoot string) (Report, error) {
	return runExecute(ctx, p, backupRoot, -1, Report{})
}

// ExecutePauseBefore 在第 actionIndex 个动作开始前暂停。已完成的动作不回滚，日志仍是这一份。
func ExecutePauseBefore(ctx context.Context, p Plan, backupRoot string, actionIndex int) (Report, error) {
	if actionIndex < 0 || actionIndex > len(p.Actions) {
		return Report{}, fmt.Errorf("pause index %d is outside 0..%d", actionIndex, len(p.Actions))
	}
	return runExecute(ctx, p, backupRoot, actionIndex, Report{})
}

// Resume 从暂停日志的 Next 继续，把新完成的动作追加进同一份日志。
func Resume(ctx context.Context, journalPath string) (Report, error) {
	loaded, err := LoadReport(journalPath)
	if err != nil {
		return loaded, err
	}
	if !loaded.Paused || loaded.Plan == nil {
		return loaded, errors.New("sync journal is not paused")
	}
	return runExecute(ctx, *loaded.Plan, "", -1, loaded)
}

func runExecute(ctx context.Context, p Plan, backupRoot string, pauseBefore int, resume Report) (report Report, err error) {
	resuming := resume.Journal != ""
	report = Report{Version: 1, Completed: []Record{}}
	start := 0
	if resuming {
		report = resume
		start = resume.Next
		if start < 0 || start > len(p.Actions) {
			return report, fmt.Errorf("invalid resume index %d", start)
		}
	}
	defer func() {
		if err == nil || errors.Is(err, ErrPaused) || len(report.Completed) == 0 {
			return
		}
		report.Paused = false
		// 调用方取消执行时仍要完成回滚，不能沿用已经取消的 Context。
		remaining, undone, undoErr := undoReverse(context.WithoutCancel(ctx), report.Completed)
		report.Completed = remaining
		report.Undone = append(report.Undone, undone...)
		if undoErr != nil {
			report.RollbackError = undoErr.Error()
			err = fmt.Errorf("%w; rollback failed: %v", err, undoErr)
		} else {
			report.RolledBack = true
			report.Pending = nil
		}
		if report.Journal != "" {
			_ = writeJournal(report)
		}
	}()
	if !resuming && backupRoot == "" {
		return report, errors.New("backup directory is required")
	}
	for _, root := range []string{p.Left, p.Right} {
		actual, err := filepath.EvalSymlinks(root)
		if err != nil {
			return report, err
		}
		expected := p.LeftResolved
		if root == p.Right {
			expected = p.RightResolved
		}
		if actual != expected {
			return report, ErrChanged
		}
	}
	if !resuming {
		backupRoot, err = resolveCreationPath(backupRoot)
		if err != nil {
			return report, err
		}
	}
	dir := ""
	if resuming {
		dir = filepath.Dir(report.Journal)
	}
	guard := backupRoot
	if resuming {
		guard = dir
	}
	for _, root := range []string{p.LeftResolved, p.RightResolved} {
		rel, err := filepath.Rel(root, guard)
		if err == nil && (rel == "." || filepath.IsLocal(rel)) {
			return report, errors.New("backup directory must be outside both sync roots")
		}
	}
	if !resuming {
		if err = os.MkdirAll(backupRoot, 0o700); err != nil {
			return report, err
		}
		// 备份目录下使用固定的 sync 子目录，同一次备份只保留一份日志，路径不随运行变化。
		dir = filepath.Join(backupRoot, "sync")
		if err = os.Mkdir(dir, 0o700); err != nil {
			return report, err
		}
		report.Journal = filepath.Join(dir, "journal.json")
	}
	// 批量预检发生在任何目标变更之前。已暂停前完成的动作不再按旧版本重检。
	for i, a := range p.Actions {
		if i < start {
			continue
		}
		if a.To != p.Left && a.To != p.Right {
			return report, errors.New("invalid destination root")
		}
		target, err := safePath(a.To, a.Path)
		if err != nil {
			return report, err
		}
		if err = matches(ctx, target, a.TargetVersion); err != nil {
			return report, err
		}
		switch a.Kind {
		case "copy":
			if a.From != p.Left && a.From != p.Right || a.From == a.To {
				return report, errors.New("invalid source root")
			}
			source, err := safePath(a.From, a.Path)
			if err != nil {
				return report, err
			}
			if err = matches(ctx, source, a.SourceVersion); err != nil {
				return report, err
			}
		case "create-directory", "delete", "remove-directory":
		default:
			return report, errors.New("unknown operation")
		}
	}
	persist := func() error { return writeJournal(report) }
	if !resuming {
		if err = persist(); err != nil {
			return report, err
		}
	}
	for i := start; i < len(p.Actions); i++ {
		a := p.Actions[i]
		if pauseBefore >= 0 && i == pauseBefore {
			report.Paused = true
			report.Next = i
			planCopy := p
			report.Plan = &planCopy
			if err = persist(); err != nil {
				report.Paused = false
				report.Plan = nil
				return report, err
			}
			return report, ErrPaused
		}
		if err = ctx.Err(); err != nil {
			return report, err
		}
		target, err := safePath(a.To, a.Path)
		if err != nil {
			return report, err
		}
		if err = matches(ctx, target, a.TargetVersion); err != nil {
			return report, err
		}
		record := Record{Action: a}
		if a.TargetVersion.Exists && !a.TargetVersion.Directory {
			backup := filepath.Join(dir, fmt.Sprintf("%06d.bak", len(report.Completed)))
			if err = copyExclusive(ctx, target, backup, 0o600); err != nil {
				return report, err
			}
			record.Backup = backup
			if err = matches(ctx, target, a.TargetVersion); err != nil {
				return report, err
			}
		}
		report.Pending = &record
		if err = persist(); err != nil {
			return report, err
		}
		if err = ctx.Err(); err != nil {
			return report, err
		}
		switch a.Kind {
		case "create-directory":
			err = os.Mkdir(target, 0o755)
		case "delete", "remove-directory":
			err = os.Remove(target)
		case "copy":
			var source string
			source, err = safePath(a.From, a.Path)
			if err != nil {
				break
			}
			if err = matches(ctx, source, a.SourceVersion); err != nil {
				break
			}
			var temp *os.File
			temp, err = os.CreateTemp(filepath.Dir(target), ".gocompare-copy-*")
			if err != nil {
				break
			}
			name := temp.Name()
			temp.Close()
			os.Remove(name)
			err = copyExclusive(ctx, source, name, os.FileMode(a.SourceVersion.Mode))
			if err == nil {
				stamp := time.Unix(0, a.SourceVersion.Modified)
				err = os.Chtimes(name, stamp, stamp)
			}
			if err == nil {
				err = matches(ctx, source, a.SourceVersion)
			}
			if err == nil {
				err = matches(ctx, target, a.TargetVersion)
			}
			if err == nil {
				if a.TargetVersion.Exists {
					err = os.Rename(name, target)
				} else {
					err = publishNew(ctx, name, target, a.SourceVersion)
				}
			}
			os.Remove(name)
		}
		if err != nil {
			return report, err
		}
		report.Completed = append(report.Completed, record)
		report.Pending = nil
		if resuming {
			report.Next = i + 1
			report.Paused = true
		}
		if err = persist(); err != nil {
			return report, fmt.Errorf("operation completed but journal update failed: %w", err)
		}
	}
	if resuming {
		report.Paused = false
		if err = persist(); err != nil {
			return report, err
		}
	}
	return report, nil
}

// publishNew 把临时副本发布到不存在的目标，绝不覆盖。优先用硬链接；
// exFAT、FAT32 和部分网络共享不支持时，退回排他创建并再次复制，写入期间目标可能短暂不完整。
func publishNew(ctx context.Context, temp, target string, source Version) error {
	err := linkFile(temp, target)
	if err == nil || errors.Is(err, os.ErrExist) {
		return err
	}
	if err = copyExclusive(ctx, temp, target, os.FileMode(source.Mode)); err != nil {
		return err
	}
	// 与硬链接路径保持一致：权限同样经过 umask，时间戳沿用源文件。
	stamp := time.Unix(0, source.Modified)
	if err = os.Chtimes(target, stamp, stamp); err != nil {
		os.Remove(target)
	}
	return err
}

func copyExclusive(ctx context.Context, source, target string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	success := false
	defer func() {
		out.Close()
		if !success {
			os.Remove(target)
		}
	}()
	buf := make([]byte, 64<<10)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := in.Read(buf)
		if n > 0 {
			if _, err = out.Write(buf[:n]); err != nil {
				return err
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if err = out.Sync(); err != nil {
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	success = true
	return nil
}

func resolveCreationPath(path string) (string, error) {
	full, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var tail []string
	current := full
	for {
		_, err = os.Lstat(current)
		if err == nil {
			base, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			for i := len(tail) - 1; i >= 0; i-- {
				base = filepath.Join(base, tail[i])
			}
			return base, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		tail = append(tail, filepath.Base(current))
		current = parent
	}
}

// linkFile 可在测试中替换，用来模拟不支持硬链接的文件系统。
var linkFile = os.Link
