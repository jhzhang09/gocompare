//go:build e2e

package main

// 自动化接口仅在显式 e2e 构建时启用，不进入发行构建。
import _ "github.com/egoist/mygo/plugins/automation"
