// Package integrate 提供 Git、文件管理器和平台专属动作的入口参数。
// 非 Windows 不返回实时注册表、ACL、MTP 或 MSSCCI，调用方不得把它们画成可用功能。
package integrate

import "runtime"

// FileManagerArgs 是文件管理器打开比较时应交给 gocompare-cli 的参数（不含程序名）。
func FileManagerArgs(left, right string) []string {
	return []string{left, right}
}

// DifftoolArgs 是 Git difftool 调用比较核心的参数。
func DifftoolArgs(left, right string) []string {
	return []string{"difftool", left, right}
}

// MergetoolArgs 是 Git mergetool 调用三方合并核心的参数。
func MergetoolArgs(base, left, right, output string) []string {
	return []string{"mergetool", "--base", base, "--output", output, left, right}
}

// PlatformActions 返回当前系统可以展示的 Windows 专属动作。
// 其他系统返回 nil。这些动作在本仓库没有对应实现，返回名称只用于避免在非 Windows 上显示。
func PlatformActions() []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	return []string{"实时注册表", "ACL", "MTP", "MSSCCI"}
}
