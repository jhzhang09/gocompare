# GoCompare

<p align="center">
  <strong>一款基于 MyGo Native UI 构建的现代化、高性能跨平台文件与目录对比合并工具。</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.27+-00ADD8?style=flat-square&logo=go" alt="Go Version" />
  <img src="https://img.shields.io/badge/GUI-MyGo%20v0.7.3-000000?style=flat-square" alt="MyGo GUI" />
  <img src="https://img.shields.io/badge/Platform-macOS%20%7C%20Windows%20%7C%20Linux-brightgreen?style=flat-square" alt="Platform" />
  <img src="https://img.shields.io/badge/License-MIT-blue?style=flat-square" alt="License" />
</p>

---

GoCompare 专为追求极致启动速度、原生系统质感与可靠数据安全性的开发者打造。应用采用 Go 语言与 [MyGo](https://github.com/egoist/mygo) 框架原生渲染，**不包含任何 Electron、WebView、HTML 或 Node.js 运行时**，内存占用极低，毫秒级即时冷启动。

提供现代化的 GUI 交互工作台以及功能完备的独立 CLI 工具，满足日常代码审查、多方冲突合并、目录镜像同步与 CI/CD 自动化流水线需求。

---

## ✨ 核心特性

- ⚡️ **纯原生极速体验**：纯 Go 编写，直接调用系统原生图形管线绘制，无浏览器内核开销，资源占用极小，启动如飞。
- 📋 **纯文本即时粘贴对比**：无需提前创建或选择本地文件，开箱即提供双栏文本工作台，直接粘贴系统剪贴板（支持一键粘贴按钮与 ⌘V）或键入文本，实时高亮差异，支持一键互换两侧、清空、复制及另存为文件。
- 📝 **双向即时文本对比与编辑**：
  - 左右分栏对齐展示，支持行内字符级高亮、**忽略空白与忽略空行**、平滑同步滚动。
  - **同屏双侧实时编辑**，支持撤销、重做、多字符集查找替换与快捷键保存。
  - 智能编码识别（UTF-8、UTF-16 BOM、GBK 等），智能保真换行符（CRLF/LF）与文件末尾空行。
- 🔀 **三方文本合并**：
  - 基准列（Base）、左侧列（Left）、右侧列（Right）直观三栏对齐。
  - 单击即可采纳单侧或双侧内容，冲突实时可视化，一键决策并生成目标合并结果。
- 📁 **目录对比与事务级同步**：
  - 递归式树状目录比对，支持层级折叠、名称实时过滤与差异状态筛选。
  - 提供安全的目录更新、双向同步与单向镜像规划。
  - **安全至上**：支持只读干燥预览（Dry-run）、操作前安全备份、事务日志与中断回滚，杜绝误操作导致的数据丢失。
- 🖼️ **多格式专用对比**：
  - **图像比对**：支持 PNG、JPEG、GIF；提供并排对比、透明叠加、像素差值高亮与容差调节。
  - **十六进制比对**：二进制文件逐字节分析与偏移定位。
  - **元数据对比**：MP3/ID3v2 媒体标签、PE 文件版本信息与 Windows `.reg` 文本差异对比。
- 🎨 **现代高级审美**：
  - 采用现代化微质感设计语言，深度适配系统原生浅色/暗色双主题。
  - 统一的双胶囊路径栏、面包屑窗格头、12px 圆角状态药丸标签与内联矢量图标。
- 💻 **全功能 CLI 工具**：提供独立的 `gocompare-cli` 控制台程序，完美融入终端脚本与自动化流水线。

---

## 🚀 快速上手

### 运行环境要求
- **Go**: 1.27.3 或更高版本
- **MyGo**: v0.7.3
- **操作系统**: macOS 12+ / Linux (需安装 GTK 3) / Windows 10+

### 从源码运行

```sh
# 启动 GUI 主工作台
go run ./cmd/gocompare

# 使用 GUI 对比两个文件
go run ./cmd/gocompare left.txt right.txt

# 使用 GUI 对比两个目录
go run ./cmd/gocompare --directories ./left-dir ./right-dir

# 启动 MyGo 开发者热重载模式
go tool mygo dev
```

---

## 💻 命令行工具 (CLI)

GoCompare 提供了轻量独立的命令行工具 `gocompare-cli`，适合直接在终端中快速查看差异或集成至脚本中。

### 构建 CLI
```sh
mkdir -p build
go build -o build/gocompare-cli ./cmd/gocompare-cli
```

### 常用命令示例

```sh
# 1. 文本文件对比（差异时退出码为 1，无差异时为 0）
./build/gocompare-cli file1.txt file2.txt

# 2. 目录递归比对并输出结构化 JSON 报告
./build/gocompare-cli dir ./project-v1 ./project-v2

# 3. 目录同步预览 (默认只读，不修改任何文件)
./build/gocompare-cli sync --mode mirror-right ./source-dir ./target-dir

# 4. 目录同步执行 (需明确指定 --apply 与外部独立备份目录)
./build/gocompare-cli sync --mode mirror-right --apply --backup-dir /tmp/backup ./source-dir ./target-dir

# 5. 三方文本合并
./build/gocompare-cli merge --base base.txt left.txt right.txt --output merged.txt
```

---

## 🛠️ 构建与打包

GoCompare 支持通过 MyGo 工具链快速打包各平台安装程序：

```sh
# 运行单元测试与静态分析
CGO_ENABLED=0 go test ./...
go vet ./...
go tool mygo vet .

# 执行并发安全检查 (Race Detector)
go test -race ./internal/document ./internal/compare ./internal/native

# 构建当前平台应用安装包 (macOS 生成 .app 与 .dmg)
go tool mygo build

# 跨平台交叉编译 (支持多架构)
go tool mygo build -platform darwin/universal,windows/amd64,linux/amd64
```

编译产物默认输出至 `build/` 目录下。

---

## 🛡️ 安全与数据完整性保证

在执行文件保存与目录同步操作时，GoCompare 遵循严格的安全工程规范：
1. **原子保存与安全备份**：文件保存采用同级临时文件写入与安全重命名机制，原生工作区在覆写前默认自动备份至用户配置目录。
2. **外部修改保护**：检测到磁盘底层文件被外部进程改动或发生权限冲突时，主动拒绝覆写并明确警告。
3. **事务日志与回滚**：目录批量同步采用两阶段验证机制。每次操作生成独立挂起日志（Pending Log），遇不可恢复错误或用户取消时支持自动中断与回滚。

详细约定请参阅 [目录操作与同步安全契约](docs/operations.md)。

---

## 📚 开发与进阶文档

- [系统架构与数据流分层](docs/architecture.md)：探讨 `cmd`、`native`、`compare` 与 `document` 的模块边界
- [原生界面设计系统](docs/design-system.md)：双主题配色、组件设计哲学与交互规范
- [功能规划与支持矩阵](docs/feature-parity.md)：全场景比对功能规划与完成度跟踪
- [开发阶段路线图](docs/roadmap.md)：P0 ~ P8 阶段演化目标与实现策略
- [跨平台支持矩阵](docs/platform-support.md)：macOS、Windows 与 Linux 平台构建与实机适配说明
- [MyGo 框架补丁维护](docs/mygo-patch.md)：深入解析 `vendor/` 补丁机制与依赖维护规约
- [测试与质量验证指南](docs/testing.md)：自动化回归测试、无头测试与故障场景覆盖

---

## 🤝 参与贡献

欢迎对 GoCompare 提交 Issue 和 Pull Request！
在提交代码前，请确保运行以下检查并通过所有测试：

```sh
go test ./...
go vet ./...
go tool mygo vet .
```

---

## 🙏 致谢 (Acknowledgements)

GoCompare 的极致性能与原生交互体验离不开优秀的开源生态，特别致谢：

- **[MyGo](https://github.com/egoist/mygo)** (by [@egoist](https://github.com/egoist)) —— 强大、轻量、开箱即用的 Go 原生跨平台桌面 GUI 框架。

---

## 📄 开源许可证

本项目基于 [MIT License](LICENSE) 开源协议。
