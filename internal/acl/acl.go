// Package acl 提供跨平台文件访问控制（ACL）与权限元信息的读取和比较。
package acl

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
)

// Entry 表示单条访问控制或权限记录。
type Entry struct {
	Principal string `json:"principal"` // 用户、群组或 SID
	Type      string `json:"type"`      // Allow 或 Deny
	Rights    string `json:"rights"`    // 权限描述（如 FullControl、Read、0755 等）
	Inherited bool   `json:"inherited"` // 是否从父目录继承
}

// Info 包含指定路径的权限与访问控制元信息。
type Info struct {
	Path      string  `json:"path"`
	Mode      string  `json:"mode"`
	Owner     string  `json:"owner,omitempty"`
	Group     string  `json:"group,omitempty"`
	Entries   []Entry `json:"entries,omitempty"`
	Supported bool    `json:"supported"`
}

// ErrUnsupported 表示当前平台尚未实现 ACL 读取。
var ErrUnsupported = errors.New("ACL inspection is not supported on this platform")

// Inspect 读取指定路径的访问权限与 ACL 信息。
func Inspect(path string) (*Info, error) {
	st, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}

	info := &Info{
		Path:      path,
		Mode:      st.Mode().String(),
		Supported: true,
	}

	fillPlatformACL(path, st, info)
	if !info.Supported {
		return info, ErrUnsupported
	}
	return info, nil
}

// Compare 比较两个路径的 ACL/权限信息，返回差异描述列表。
func Compare(left, right *Info) []string {
	if left == nil || right == nil {
		return []string{"one side is nil"}
	}
	if !left.Supported || !right.Supported {
		return []string{ErrUnsupported.Error()}
	}
	var diffs []string
	lm, rm := left.Mode, right.Mode
	// 两侧都有权限条目时，rwx 位已由条目逐项比较，这里只比较类型和特殊位前缀，避免同一差异报两遍。
	if len(left.Entries) > 0 && len(right.Entries) > 0 {
		lm, rm = modePrefix(lm), modePrefix(rm)
	}
	if lm != rm {
		diffs = append(diffs, fmt.Sprintf("mode differs: %s vs %s", left.Mode, right.Mode))
	}
	if left.Owner != "" && right.Owner != "" && left.Owner != right.Owner {
		diffs = append(diffs, fmt.Sprintf("owner differs: %s vs %s", left.Owner, right.Owner))
	}
	if left.Group != "" && right.Group != "" && left.Group != right.Group {
		diffs = append(diffs, fmt.Sprintf("group differs: %s vs %s", left.Group, right.Group))
	}

	maxEntries := len(left.Entries)
	if len(right.Entries) > maxEntries {
		maxEntries = len(right.Entries)
	}
	for i := 0; i < maxEntries; i++ {
		if i >= len(left.Entries) {
			diffs = append(diffs, fmt.Sprintf("right has extra entry: %+v", right.Entries[i]))
			continue
		}
		if i >= len(right.Entries) {
			diffs = append(diffs, fmt.Sprintf("left has extra entry: %+v", left.Entries[i]))
			continue
		}
		le, re := left.Entries[i], right.Entries[i]
		if le.Principal != re.Principal || le.Type != re.Type || le.Rights != re.Rights {
			diffs = append(diffs, fmt.Sprintf("entry[%d] differs: %s(%s:%s) vs %s(%s:%s)",
				i, le.Principal, le.Type, le.Rights, re.Principal, re.Type, re.Rights))
		}
	}

	return diffs
}

// modePrefix 去掉 os.FileMode.String() 末尾 9 个 rwx 字符，保留类型与 setuid/sticky 等标记。
func modePrefix(mode string) string {
	if len(mode) < 9 {
		return mode
	}
	return mode[:len(mode)-9]
}

// IsWindows 返回当前运行环境是否为 Windows。
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// FormatDiffs 把差异格式化为多行文本。
func FormatDiffs(diffs []string) string {
	if len(diffs) == 0 {
		return "ACL and permissions match"
	}
	return strings.Join(diffs, "\n")
}
