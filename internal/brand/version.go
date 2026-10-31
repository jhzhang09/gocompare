package brand

import "runtime/debug"

// Version 是 GoCompare 自身的版本号，发版构建通过
// -ldflags "-X gocompare/internal/brand.Version=v1.2.3" 注入。
var Version = "dev"

// AppVersion 返回注入的版本号；未注入时使用 go install 写进构建信息的模块版本，都没有则为 "dev"。
func AppVersion() string {
	if Version != "dev" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return Version
}
