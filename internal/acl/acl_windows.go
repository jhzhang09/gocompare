//go:build windows

package acl

import "os"

func fillPlatformACL(path string, st os.FileInfo, info *Info) {
	// 尚未读取 Windows 安全描述符，不能用占位权限参与比较。
	info.Supported = false
}
