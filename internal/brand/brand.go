// Package brand 提供 GoCompare 的应用图标，以及当前二进制所链接的 MyGo 版本。
package brand

import (
	"errors"
	"runtime/debug"

	_ "embed"
)

//go:embed icon.png
var iconPNG []byte

// IconPNG 返回由 resources/icon.svg 渲染出的应用图标。
// 返回的切片与嵌入数据共用底层数组，调用方不得修改。
func IconPNG() []byte { return iconPNG }

// Framework 返回构建信息里 github.com/egoist/mygo 的版本。
// 没有该模块或版本为空时返回错误，不另造一个版本号。
func Framework() (string, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", errors.New("build info unavailable")
	}
	for _, dep := range info.Deps {
		if dep.Path != "github.com/egoist/mygo" {
			continue
		}
		version := dep.Version
		if dep.Replace != nil {
			version = dep.Replace.Version
		}
		if version == "" {
			return "", errors.New("mygo version is empty")
		}
		return version, nil
	}
	return "", errors.New("mygo module is not linked")
}
