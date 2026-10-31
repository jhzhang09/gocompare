<div align="center">

<img src="resources/icon.png" width="96" alt="GoCompare 图标">

# GoCompare

**用 Go 编写的原生跨平台文件与目录比较工具。**

比较文本、目录、图片和二进制元数据，进行三方合并，安全地同步目录——既有快速的原生窗口，也有可脚本化的命令行。

[![Checks](https://github.com/jhzhang09/gocompare/actions/workflows/check.yml/badge.svg)](https://github.com/jhzhang09/gocompare/actions/workflows/check.yml)
[![Release](https://img.shields.io/github/v/release/jhzhang09/gocompare?include_prereleases&sort=semver)](https://github.com/jhzhang09/gocompare/releases)
[![Go](https://img.shields.io/github/go-mod/go-version/jhzhang09/gocompare)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
![Platforms](https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey)

[English](README.md) · **简体中文**

</div>

![文本比较](docs/images/text-compare.png)

## 为什么选择 GoCompare

- **真正的原生界面。** 基于 [MyGo](https://github.com/egoist/mygo) 构建：纯 Go 实现，不含 Electron、WebView、Node.js，也不依赖 cgo。每个平台一个小巧的可执行文件，可在任意机器上交叉编译。
- **默认安全。** 保存时绝不覆盖已被外部修改的文件；目录同步总是先预览，覆盖或删除前先备份，出错时自动回滚。
- **界面与命令行共用同一套核心。** 窗口里能做的事都可以写成脚本，退出码稳定，方便接入 CI 和 Git。

> [!NOTE]
> GoCompare 仍处于早期开发阶段（1.0 之前）。比较、合并和同步的核心路径有充分的测试，但部分功能尚未完整，详见[功能矩阵](docs/feature-parity.md)。欢迎反馈问题和建议。

## 功能

| 模式 | 说明 |
|---|---|
| **文本比较** | 直接在左右两栏粘贴或输入文本，实时显示差异并按词高亮；可交换、复制、清空、另存为任一侧。 |
| **文件比较** | 左右分栏对比，**两侧都可编辑**，支持撤销重做、查找替换、手动对齐锚点、忽略空白和空行。自动识别 UTF-8、UTF-16（带 BOM）和 GBK，保存时保留原编码、BOM 和换行符。 |
| **目录比较** | 递归树形视图，可折叠、过滤、只看差异；可选比较权限和修改时间，安全地跟随符号链接。 |
| **目录同步** | 向左、向右、双向更新或镜像。总是先生成预览计划；执行时备份、写日志，失败自动回滚，可一键撤销。 |
| **三方合并** | 基准、左侧、右侧三栏对齐，每个冲突可选采用左侧、右侧、双方或基准，输出可自由编辑；也可作为 Git 的 `mergetool` 使用。 |
| **图片比较** | 支持 PNG、JPEG、GIF：并排、叠加、像素差值、0–255 容差图（真实差异标红）、背景色替换，以及左右两侧分别翻转。 |
| **媒体标签** | 比较 ID3v2.3／2.4 的标题、艺术家和专辑，不解码音频。 |
| **版本与注册表** | 比较 PE 文件的 `FileVersion` 资源和 Windows `.reg` 导出文件（UTF-16 或 UTF-8），不读取实时注册表。 |
| **会话与多语言** | 保存和重新打开比较会话。界面支持简体中文、繁體中文和英文，可选浅色、深色或跟随系统。 |

<table>
  <tr>
    <td><img src="docs/images/folder-compare.png" alt="目录比较"></td>
    <td><img src="docs/images/picture-compare.png" alt="图片比较的容差图"></td>
  </tr>
  <tr>
    <td align="center">目录比较</td>
    <td align="center">图片比较（容差图）</td>
  </tr>
</table>

## 安装

从 [**Releases**](https://github.com/jhzhang09/gocompare/releases) 页面下载最新版本。

| 平台 | 桌面应用 | 命令行 |
|---|---|---|
| macOS 12+（Apple Silicon 与 Intel） | `gocompare-<version>.dmg` | `gocompare-cli_<version>_darwin_<arch>.tar.gz` |
| Windows 10／11 | `gocompare-setup-<version>-<arch>.exe`（安装版）或免安装 `.zip` | `gocompare-cli_<version>_windows_<arch>.zip` |
| Linux（GTK 3） | `.deb` 或 `.tar.gz` | `gocompare-cli_<version>_linux_<arch>.tar.gz` |

每个版本都附带 `SHA256SUMS.txt` 和[构建来源证明](https://docs.github.com/actions/security-for-github-actions/using-artifact-attestations)，可用 `gh attestation verify <文件> --repo jhzhang09/gocompare` 校验。

<details>
<summary><b>macOS 提示"无法打开 GoCompare"</b></summary>

发布版本使用临时签名，尚未经过公证。首次启动时请右键点击 **GoCompare.app → 打开**，或执行：

```bash
xattr -dr com.apple.quarantine /Applications/GoCompare.app
```
</details>

<details>
<summary><b>Windows 出现 SmartScreen 警告</b></summary>

程序暂未做代码签名，请选择**更多信息 → 仍要运行**。
</details>

<details>
<summary><b>Linux</b></summary>

```bash
sudo apt install ./gocompare_<version>_amd64.deb
# 或者不安装，直接运行：
tar xzf gocompare-<version>-linux-amd64.tar.gz && ./gocompare
```

需要 GTK 3，大多数桌面发行版都已自带。
</details>

### 从源码构建

需要 Go 1.27.3 或更高版本。所有依赖都已放在 `vendor/` 中，构建无需联网。

```bash
git clone https://github.com/jhzhang09/gocompare.git
cd gocompare
go run ./cmd/gocompare                       # 运行桌面应用
go build -o build/ ./cmd/gocompare-cli        # 构建命令行工具
go tool mygo build                            # 打包当前平台的应用（macOS 生成 .app 与 .dmg）
```

## 使用

### 桌面应用

```bash
gocompare                                    # 打开文本比较工作区
gocompare left.txt right.txt                 # 比较两个文件
gocompare --directories ./left ./right       # 比较两个目录
gocompare --version
```

macOS 上的可执行文件位于 `/Applications/GoCompare.app/Contents/MacOS/GoCompare`。

### 命令行

`gocompare-cli` 的退出码：输入相同（或合并无冲突）为 **0**，存在差异（或仍有冲突）为 **1**，出错为 **2**。

```bash
gocompare-cli left.txt right.txt                                  # 比较两个文件
gocompare-cli dir --filter '*.go' ./v1 ./v2                       # 以 JSON 输出目录比较报告
gocompare-cli preview mirror-right ./src ./dst                    # 生成同步计划（只读）
gocompare-cli sync --mode mirror-right --apply --backup-dir ~/gocompare-backup ./src ./dst
gocompare-cli restore ~/gocompare-backup/sync/journal.json        # 撤销一次同步
gocompare-cli merge --base base.txt --output merged.txt left.txt right.txt
gocompare-cli picture before.png after.png
```

同步模式：`update-right`、`update-left`、`update-both`、`mirror-right`、`mirror-left`。运行 `gocompare-cli --help` 查看全部命令，包括 `hex`、`media`、`pe`、`reg`、`script`、`report` 和 `extend`。

### 集成到 Git

```bash
git config --global diff.tool gocompare
git config --global difftool.gocompare.cmd 'gocompare "$LOCAL" "$REMOTE"'
git config --global merge.tool gocompare
git config --global mergetool.gocompare.cmd 'gocompare-cli mergetool --base "$BASE" --output "$MERGED" "$LOCAL" "$REMOTE"'
git config --global mergetool.gocompare.trustExitCode true
```

`mergetool` 把合并结果写入 `$MERGED`；仍有冲突时退出码为 1，Git 会保持该文件为未解决状态，直到你处理完冲突。

## 安全保证

- **保存是原子的，并能感知冲突。** 先写入同目录的临时文件，只有磁盘上的文件仍与加载时一致才替换原文件；"另存为"绝不覆盖已有文件。
- **目录同步遵循"计划 → 确认 → 执行"。** 每个操作都记录预期的大小、SHA-256、权限和时间；执行时重新核对，覆盖或删除前先备份，每一步写入日志，任何错误或取消都会回滚已完成的步骤。
- **撤销不会破坏更新的内容。** 撤销同步时，同步之后被修改过的文件一律不动；在大小写不敏感的文件系统上，只差大小写的文件名冲突会直接拒绝生成计划。

完整约定见 [docs/operations.md](docs/operations.md)。

## 文档

| 文档 | 内容 |
|---|---|
| [架构](docs/architecture.md) | 模块边界与数据流 |
| [功能矩阵](docs/feature-parity.md) | 已完成、部分完成和计划中的功能 |
| [平台支持](docs/platform-support.md) | 构建目标与各平台状态 |
| [操作契约](docs/operations.md) | 保存与同步的安全保证 |
| [测试](docs/testing.md) | 自动化测试与原生回归测试 |
| [MyGo 补丁](docs/mygo-patch.md) | vendor 中框架补丁的维护方式 |
| [发版流程](docs/releasing.md) | 维护者如何发布新版本 |

## 参与贡献

欢迎贡献代码！请先阅读 [CONTRIBUTING.md](CONTRIBUTING.md) 和[行为准则](CODE_OF_CONDUCT.md)。提交前请确保以下检查全部通过：

```bash
gofmt -l cmd internal tools     # 应当没有任何输出
go vet ./...
CGO_ENABLED=0 go test ./...
go tool mygo vet .
```

发现安全问题请按 [SECURITY.md](SECURITY.md) 私下报告，不要公开提交 Issue。

## 致谢

- [MyGo](https://github.com/egoist/mygo)（作者 [@egoist](https://github.com/egoist)）—— GoCompare 所基于的 Go 原生桌面 UI 框架。
- [golang.org/x/text](https://pkg.go.dev/golang.org/x/text) 与 [golang.org/x/image](https://pkg.go.dev/golang.org/x/image)。

## 许可证

[MIT](LICENSE) © GoCompare Contributors
