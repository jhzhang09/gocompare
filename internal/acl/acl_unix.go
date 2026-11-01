//go:build !windows

package acl

import (
	"fmt"
	"os"
	"syscall"
)

func fillPlatformACL(path string, st os.FileInfo, info *Info) {
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok || sys == nil {
		return
	}
	info.Owner = fmt.Sprintf("uid:%d", sys.Uid)
	info.Group = fmt.Sprintf("gid:%d", sys.Gid)

	// 构建基础权限 Entry
	perm := st.Mode().Perm()
	info.Entries = []Entry{
		{
			Principal: fmt.Sprintf("owner(%s)", info.Owner),
			Type:      "Allow",
			Rights:    fmt.Sprintf("%03o", (perm>>6)&7),
		},
		{
			Principal: fmt.Sprintf("group(%s)", info.Group),
			Type:      "Allow",
			Rights:    fmt.Sprintf("%03o", (perm>>3)&7),
		},
		{
			Principal: "others",
			Type:      "Allow",
			Rights:    fmt.Sprintf("%03o", perm&7),
		},
	}
}
