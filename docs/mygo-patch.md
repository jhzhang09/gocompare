# MyGo 双侧编辑扩展

日期：2026-10-31。基线：官方 MyGo v0.7.3，模块版本保持固定。

## 改动与契约

上游 `DiffState.Editable` 只控制新侧，旧侧固定只读。新增 `OldEditable bool`：零值继续只读，仅分栏旧侧由该字段控制。新侧仍由 `Editable` 控制；统一视图的编辑策略不变。

GoCompare 同时开启两个字段；加载与保存期间同时关闭。编辑沿用上游 Buffer、输入、撤销、差异重算、共享滚动、换行和占位布局；应用未复制编辑器实现，也未导入框架 internal 包。

另有默认关闭的后台比较接口。`ExternalComparison` 为真时，视图不在绘制路径上重算差异；应用通过 `NewComparisonRequest` 捕获缓冲区快照，再调用可取消的 `Compute`。`ApplyComparison` 只接受同一对缓冲区、同一版本和同一行内模式的结果。`ApplyHunks` 接受调用方给出的半开行区间，供文本锚点对齐写回同一视图。取消检查位于行匹配、差异压缩和行内对齐。零值保持上游同步行为。

该扩展仍未解决多栏合并或左右显示名称的本地化。手工对齐由应用计算后经 `ApplyHunks` 显示。上游对旧侧变化会进行完整重算，因此左侧大文件编辑仍会触发新的后台任务。

## 依赖交付与维护

依赖由 `go mod vendor` 生成，约 11 MiB，包含构建所需的第三方源文件和许可证。这是可重现的依赖快照，避免依赖尚未存在的远程 fork；不是应用私有编辑器副本。Go 的默认模块模式会使用该目录。不要使用 `-mod=mod` 构建 GUI，该选项绕过补丁。

改动唯一来源：`patches/mygo-v0.7.3-native.patch`，校验值在 `patches/mygo-v0.7.3-native.json`。它修改 `internal/diff` 的匹配与压缩，以及 `ui/editor` 的 `diffview.go`、`diffmodel.go`、`patch.go`，并新增 `internal/diff/context.go` 与 `ui/editor/comparison.go`。不要手改其他依赖文件。

重新生成同版依赖：

```sh
go mod vendor
git apply --check patches/mygo-v0.7.3-native.patch
git apply patches/mygo-v0.7.3-native.patch
```

验证已正确应用：

```sh
git apply --reverse --check patches/mygo-v0.7.3-native.patch
CGO_ENABLED=0 go test ./...
go tool mygo vet .
```

升级框架前先检查是否已有等价公开接口；若已有，迁移调用并移除补丁。否则核对新版本布局、焦点和输入路径，重新生成补丁并执行双侧键盘、撤销、忙碌锁定、换行与页面重建测试。不能把旧补丁强制套到未知版本。

测试位于 internal/native/app_test.go；默认左侧只读与显式允许编辑分别验证，应用还验证当前焦点侧保存。

## 可用性修复增量

新增 `ui/editor/comparison_rules.go` 的 ComputeRules：基于固定快照运行可取消的行比较，支持不改变行数的空白规范化与单调分段锚点，结果沿用 Comparison 身份／版本校验。应用的手工对齐不再使用无请求版本的 ApplyHunks。

`hunkEdit` 修复空目标的占位行：将内容应用到空白文件时不再无条件追加换行。增加含／不含末尾换行、多空行和中文内容的双向复制测试。补丁清单与摘要已同步更新。
