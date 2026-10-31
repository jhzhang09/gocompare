package operations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// LoadReport 读取执行日志。日志只描述已经记录的操作，不证明磁盘仍与日志一致。
func LoadReport(path string) (Report, error) {
	var report Report
	f, err := os.Open(path)
	if err != nil {
		return report, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (32<<20)+1))
	if err != nil {
		return report, err
	}
	if len(data) > 32<<20 {
		return report, errors.New("sync journal exceeds 32 MiB limit")
	}
	if err = json.Unmarshal(data, &report); err != nil {
		return report, err
	}
	if report.Version != 1 {
		return report, fmt.Errorf("unsupported sync journal version: %d", report.Version)
	}
	if report.Journal == "" {
		report.Journal = path
	}
	return report, nil
}

// Restore 撤销日志中仍然生效的操作，并写回同一份日志。
// 已标记回滚且没有剩余操作时拒绝重复撤销。
func Restore(ctx context.Context, journalPath string) (Report, error) {
	report, err := LoadReport(journalPath)
	if err != nil {
		return report, err
	}
	if report.RolledBack && len(report.Completed) == 0 {
		return report, errors.New("sync journal is already rolled back")
	}
	if len(report.Completed) == 0 {
		return report, errors.New("sync journal has no completed operations")
	}
	remaining, undone, undoErr := undoReverse(ctx, report.Completed)
	report.Completed = remaining
	report.Undone = append(report.Undone, undone...)
	if undoErr != nil {
		report.RollbackError = undoErr.Error()
	} else {
		report.RolledBack = true
		report.Pending = nil
		report.RollbackError = ""
	}
	writeErr := writeJournal(report)
	if undoErr != nil {
		return report, undoErr
	}
	return report, writeErr
}

func writeJournal(report Report) error {
	if report.Journal == "" {
		return errors.New("missing journal path")
	}
	dir := filepath.Dir(report.Journal)
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	f, err := os.CreateTemp(dir, ".journal-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, report.Journal)
}

// undoReverse 从最后一项开始撤销。失败时 remaining 仍是尚未撤销的前缀。
func undoReverse(ctx context.Context, records []Record) (remaining, undone []Record, err error) {
	remaining = append([]Record(nil), records...)
	for len(remaining) > 0 {
		if err = ctx.Err(); err != nil {
			return remaining, undone, err
		}
		last := remaining[len(remaining)-1]
		if err = undoRecord(ctx, last); err != nil {
			return remaining, undone, err
		}
		remaining = remaining[:len(remaining)-1]
		undone = append(undone, last)
	}
	return nil, undone, nil
}

func undoRecord(ctx context.Context, record Record) error {
	action := record.Action
	target, err := safePath(action.To, action.Path)
	if err != nil {
		return err
	}
	switch action.Kind {
	case "delete":
		if err = regularBackup(record.Backup); err != nil {
			return err
		}
		if _, err = os.Lstat(target); !os.IsNotExist(err) {
			return fmt.Errorf("restore refused, path exists: %s", target)
		}
		mode := os.FileMode(action.TargetVersion.Mode) & os.ModePerm
		if mode == 0 {
			mode = 0o600
		}
		if err = copyExclusive(ctx, record.Backup, target, mode); err != nil {
			return err
		}
		return restoreTime(target, action.TargetVersion.Modified)
	case "remove-directory":
		if _, err = os.Lstat(target); !os.IsNotExist(err) {
			return fmt.Errorf("restore refused, directory exists: %s", target)
		}
		return os.Mkdir(target, 0o755)
	case "create-directory":
		info, statErr := os.Lstat(target)
		if os.IsNotExist(statErr) {
			return nil
		}
		if statErr != nil {
			return statErr
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("restore refused, not a directory: %s", target)
		}
		return os.Remove(target)
	case "copy":
		info, statErr := os.Lstat(target)
		if os.IsNotExist(statErr) && !action.TargetVersion.Exists {
			return nil
		}
		if statErr != nil {
			return statErr
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("restore refused, copy target is not a regular file: %s", target)
		}
		// 只撤销仍是本次同步写入内容的文件。之后被修改过就拒绝，避免覆盖或删除用户的新内容。
		if err = sameContent(ctx, target, action.SourceVersion); err != nil {
			return err
		}
		if !action.TargetVersion.Exists {
			return os.Remove(target)
		}
		if err = regularBackup(record.Backup); err != nil {
			return err
		}
		return replaceWithBackup(ctx, record.Backup, target, action.TargetVersion)
	default:
		return fmt.Errorf("unknown operation: %s", action.Kind)
	}
}

// sameContent 比较大小和 SHA-256，不比较权限和时间：复制时权限经过 umask，不能据此判定被修改。
func sameContent(ctx context.Context, path string, want Version) error {
	got, err := fingerprint(ctx, path)
	if err != nil {
		return err
	}
	if !got.Exists || got.Size != want.Size || got.Hash != want.Hash {
		return fmt.Errorf("restore refused, file changed after sync: %s", path)
	}
	return nil
}

func regularBackup(path string) error {
	if path == "" {
		return errors.New("backup missing")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("backup is not a regular file: %s", path)
	}
	return nil
}

func replaceWithBackup(ctx context.Context, backup, target string, version Version) error {
	if err := regularBackup(backup); err != nil {
		return err
	}
	mode := os.FileMode(version.Mode) & os.ModePerm
	if mode == 0 {
		mode = 0o600
	}
	temp, err := os.CreateTemp(filepath.Dir(target), ".gocompare-restore-*")
	if err != nil {
		return err
	}
	name := temp.Name()
	temp.Close()
	os.Remove(name)
	if err = copyExclusive(ctx, backup, name, mode); err != nil {
		return err
	}
	if err = restoreTime(name, version.Modified); err != nil {
		os.Remove(name)
		return err
	}
	if err = os.Rename(name, target); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

func restoreTime(path string, modified int64) error {
	if modified == 0 {
		return nil
	}
	stamp := time.Unix(0, modified)
	return os.Chtimes(path, stamp, stamp)
}
