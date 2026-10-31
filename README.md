# GoCompare

基于 **MyGo Native UI** 的跨平台比较工具，目标覆盖 Beyond Compare 5 的比较、合并和同步工作流。

当前是开发中的首个可运行版本，尚未达到完整功能对齐。应用使用 Go 和 MyGo 原生绘制，无 HTML、JavaScript 或 WebView。

## 已实现

- 两个文本文件的分栏比较、语法及行内差异高亮、差异导航、自动换行。打开时识别 UTF-8、UTF-16 BOM 和可往返的 GBK；Latin-1 需显式指定。单文件上限 16 MiB。
- 在同一个对齐视图中直接编辑左右文件，支持撤销、查找替换；Cmd/Ctrl+S 保存当前焦点侧。另保留单侧专注编辑页面。
- 差异块双向复制、分别保存；关闭或重新比较前确认未保存修改。
- 保留 UTF-8 BOM、LF／CRLF 和末尾换行；检测磁盘内容、身份或权限变化后拒绝覆盖。
- 新建文件另存为、保存前原内容备份、文件／目录会话保存与载入。
- 递归目录比较、差异筛选、打开文件对与返回目录。
- 本地目录更新和镜像：计划预览、版本预检、确认执行、备份、操作日志及取消。
- 流式字节比较、目录 JSON 报告和同步 CLI。
- 原生界面、文档和 CLI 回归测试，以及三平台构建工作流。

## 运行

要求 Go **1.27.3**。框架固定为 MyGo **v0.7.3**。Linux Native UI 需要 GTK 3。

```sh
go run ./cmd/gocompare
go run ./cmd/gocompare left.txt right.txt
go run ./cmd/gocompare-cli left.txt right.txt
go run ./cmd/gocompare --directories ./left-dir ./right-dir
go run ./cmd/gocompare-cli dir ./left-dir ./right-dir
go run ./cmd/gocompare-cli sync --mode mirror-right ./left-dir ./right-dir
go tool mygo dev
```

直接构建 CLI 后运行，可以获得其原始退出码；`go run` 自身会包装子程序的非零退出状态。

```sh
mkdir -p build
go build -o build/gocompare-cli ./cmd/gocompare-cli
./build/gocompare-cli left.txt right.txt
```

## 验证与打包

```sh
CGO_ENABLED=0 go test ./...
go vet ./...
go tool mygo vet .
go test -race ./internal/document ./internal/compare ./internal/native
GOCOMPARE_SNAPSHOTS=/tmp/gocompare-snapshots go test ./internal/native -run TestWorkspaceScreenshots -count=1
go tool mygo build
```

MyGo 可以交叉编译 Linux、Windows 和 macOS；安装包、签名、公证和真实系统验收另见[平台支持](docs/platform-support.md)。

## 当前边界

文本界面使用 MyGo `DiffView`，通过受控的 `OldEditable` 补丁开放左侧编辑，已实现同屏双侧编辑；多栏合并尚未实现。依赖快照保存在 `vendor/`，补丁维护见 [框架扩展记录](docs/mygo-patch.md)。差异计算仍由框架在视图路径中触发，原型限制单个文本文件为 16 MiB；该上限不代表性能验收已经通过。

暂不读取符号链接、二进制、非 UTF-8、混合换行和单独 CR 换行文件；会明确报告错误。保存采用同目录临时文件与重命名，不承诺跨进程原子比较并交换、目录 fsync、硬链接关系、扩展属性或 ACL 保留。原生工作区保存前默认备份到系统用户配置目录下的 `GoCompare/backups`。另存为只创建新文件，现有目标会明确报错；目前依赖目标文件系统支持硬链接。目录同步仍缺少自动恢复、暂停续传、链接复制、类型冲突处理和完整元数据策略，详见 [操作契约](docs/operations.md)。

当前开发标识符为 `dev.gocompare.desktop`，Go module 使用本地路径 `gocompare`。正式发布前应确定仓库路径、产品标识符和签名主体。

## 开发资料

- [架构与数据流](docs/architecture.md)
- [功能对齐矩阵](docs/feature-parity.md)
- [开发阶段与验收](docs/roadmap.md)
- [设计规范](docs/design-system.md)
- [平台支持](docs/platform-support.md)
- [测试与故障场景](docs/testing.md)
- [MyGo 扩展与依赖维护](docs/mygo-patch.md)

## 会话与同步

`.gocompare` 会话文件记录来源路径、会话类型和显示选项，不包含未保存的编辑内容或凭据。

CLI 默认只做预览，退出码 1 表示存在差异或待执行操作。执行同步须明确指定 `--apply` 和位于两个根目录之外的备份目录：

```sh
./build/gocompare-cli sync --mode mirror-right --apply --backup-dir /path/to/backups ./left-dir ./right-dir
```

镜像可能删除目标中的单侧项。GUI 必须确认预览后才执行。每次操作的备份和日志独立保存，自动清理和一键恢复尚未实现。`GOCOMPARE_DATA_DIR` 可隔离开发／测试配置目录。

## 文件与目录比较可用性修订

日常操作集中于主工具栏，保存位于各侧文件标题内；比较规则在右上角“比较选项”。目录支持左右对齐的树形行、折叠、文件名搜索和单侧文件打开。`*.txt` 等不带斜线模式递归匹配文件名，带斜线模式匹配相对路径。

“忽略空白”现在更新真实差异高亮，编辑后仍生效，并随会话保存。单侧文件的空白另一侧可应用内容后创建保存。带过滤或特殊属性规则的同步当前明确拒绝，避免超出比较范围。
