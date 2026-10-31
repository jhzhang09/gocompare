# 实现验证记录

日期：2026-10-31。环境：macOS arm64、Go 1.27.3、MyGo v0.7.3。

下面各节保留当时的记录。当前结论以最后一节为准；早先写的“暂停后续传、目录合并和 4000 行以上合并未实现”以及“50 个测试”都已过时。

## 已执行

| 检查 | 结果 | 边界 |
|---|---|---|
| `CGO_ENABLED=0 go test ./...` | 14 个测试用例通过 | 本机执行，含保留的 Git／patch 测试 |
| `go vet ./...` | 通过 | 静态检查 |
| `go tool mygo vet .` | 通过 | 控件生命周期与捕获检查 |
| `go test -race ./internal/document ./internal/compare ./internal/native` | 通过 | 当前回归路径 |
| darwin/amd64、darwin/arm64 构建 | 通过 | arm64 实际启动，其余仅编译 |
| linux/amd64、linux/arm64 构建 | 通过 | 未在 Linux 运行 |
| windows/amd64、windows/arm64 构建 | 通过 | 未在 Windows 运行 |
| MyGo macOS arm64 应用打包 | 通过 | 生成 10.1 MB GoCompare.app，使用本机 ad-hoc 签名，未公证、未生成 DMG |
| MyGo Tester 截图 | 已渲染并检查 | CPU 渲染，不代表 GPU 验收 |
| macOS 原生窗口启动与自动化截图 | 通过 | 实际打开两文件并显示差异；未验证系统 IME、文件对话框和安装升级 |

原生测试使用 `-tags e2e` 与框架自动化接口。沙箱内首次运行发生系统 XPC 连接失败；获准在沙箱外运行后成功，进程已自动退出。截图来自运行应用的 CapturePage，不等同于屏幕级 GPU 图像对照测试。

GitHub Actions 工作流已配置但尚未远端执行。正式发行签名、公证、DMG／安装器、远程来源、目录同步和完整 BC 对齐不在本批验证结果中。

## 同屏双侧编辑增量验证

同日后续开发：初始化 main 分支，未提交、未添加远程。MyGo v0.7.3 通过标准 vendor 快照与独立补丁扩展 `OldEditable`，不修改全局模块缓存。

- 当前共 17 个 Go 测试用例通过，包括上游默认只读兼容、两侧直接键入中文、撤销、焦点侧保存、忙碌时双侧锁定、页面重建，以及长中文／emoji 行换行和插删后的差异块重建。
- 新增路径通过 race、`go vet`、`mygo vet` 检查；补丁通过 `git apply --reverse --check`。
- macOS 原生窗口通过 tests/native_smoke.py：真实运行应用的自动化输入路径上，两侧输入、撤销、焦点侧保存和截图均通过。该测试不证明真实系统输入法组合输入已验收。
- 本机 GoCompare.app 已重新打包，仍为 ad-hoc 签名，未公证。
- 本批 Windows／Linux amd64 已重新交叉编译通过，尚未实际运行。

原生脚本首次遇到窗口尚未创建的 `no_window`，已修正为等待 windows 清单出现后才读取元素树，重跑通过。

## 会话、目录与同步增量验证

同日继续开发：当前共 35 个 Go 测试用例通过。新增覆盖：会话格式／版本／相对路径／体积边界、另存为拒绝覆盖、原内容备份及失败阻断、目录递归配对与链接不跟随、同步预览只读、时间戳冲突、嵌套根目录拒绝、执行前版本变化、目标父路径链接替换、取消、原内容备份、操作日志，以及 GUI 确认／取消与执行后刷新。

- 全量 `CGO_ENABLED=0 go test ./...`、`go vet ./...`、`mygo vet` 通过。

## 后台比较、同步回滚与文本合并

同日继续：后台比较补丁已补齐行内对齐的取消检查，补丁可经 `git apply --reverse --check` 复核。同步执行失败后自动回滚已完成操作；成功执行后可按日志撤销。两方和三方文本合并可在 CLI 与原生工作区中决策冲突并另存输出。

- 全量 `CGO_ENABLED=0 go test ./...` 通过，共 50 个测试。
- `go vet ./...`、`go tool mygo vet .` 通过。
- `go test -race` 覆盖 `internal/merge`、`internal/operations`、`internal/native`、`internal/workspace` 和 `cmd/gocompare-cli`，通过。
- 本批没有重新打包应用，也没有做 macOS 原生窗口冒烟。合并与撤销使用 `ui.NewTester` 和无窗口任务路径验证。
- Windows／Linux 仍未实际运行。暂停后续传、目录合并和 4000 行以上的文本合并未实现。
- native／operations／workspace／document／compare 的 race 检查通过。
- macOS 原生自动化的文件模式与目录模式均通过：目录打开文件对、两侧编辑／撤销／保存、返回父目录并截图。
- Windows／Linux amd64 已重新交叉编译通过；尚未实际运行。
- GoCompare.app 已重新打包，仍为本机 ad-hoc 签名。

同步真实执行当前仅在临时测试目录中验证，不代表网络共享、磁盘故障、权限组合或跨进程竞争验收完成。尚无自动回滚／日志恢复、多窗口恢复、后台差异结果协议、三方合并或完整格式与协议支持。

## 2026-10-31 全量功能验证

本机执行，证据目录中的 `go-test.log` 先保存计划指定的 `CGO_ENABLED=0 go test ./... -count=1`，退出码 0。同一日志随后追加 `go test ./... -count=1 -v`，因为无 `-v` 的输出只有包级 `ok`，不会打印用例名。追加段退出码 0，含 84 条顶层 `--- PASS`、协议子测试 `TestProtocols/ftp` 至 `TestProtocols/svn`，且没有 `--- FAIL`。新增包 `archive`、`hex`、`paginate`、`picture`、`remote`、`script`、`snapshot`、`table`、`textcompare`、`dirmerge` 以及 `internal`（ID3、`.reg`、docx／RTF／PDF、HTML 报告、PE `FileVersion`）均为 `ok`。

标为完成的能力都有成功路径和失败路径：

| 能力 | 成功 | 失败或拒绝 |
|---|---|---|
| F06 算法与规则 | `TestAlgorithmsReproduceRight` 还原右侧文本 | 非法正则、逆序锚点 |
| F10 目录合并 | 干净三方合并字节为 `L\nb\nR\n` | 删除／修改冲突 |
| F11 同步 | 预览不写目标；暂停后 `Resume` 写完 | 再次 `Resume` 已完成日志失败；取消回滚 |
| F12 表格 | 容差内 `1.00` 与 `1.005` 不算差异 | 重复键与 `3`／`4` 的变化 |
| F13 十六进制 | 搜索、插入、删除、撤销回到指定字节 | CLI 不同文件退出码 1，并输出 `offset 0` |

部分完成的远程来源在 `TestProtocols` 中覆盖 FTP、FTPS、SFTP、SMB、WebDAV、S3、Dropbox、OneDrive 和 SVN：读回 `alpha`、删除后不可读、`fail` 失败时 `other` 仍为 `beta`、取消返回 `context.Canceled`。无真实账号。7z 返回 `7z is not supported`，tar.gz 写入返回 `tar.gz is read-only`。空目录快照与空文件编码不同。

CLI 连续两次运行已交付的 `gocompare-cli`，标准输出和退出码一致。日志含三方冲突标记 `<<<<<<< left`、退出码 1、`1 conflicts`、同步预览 `preview left target absent`、`--apply` 退出码 0、`restore` 后 `restore removed target`，以及十六进制 `offset 0`。

`ui.NewTester` 点击“忽略空白”后界面为“按规则相同”，两侧文件字节不变。非 Windows 点击“实时注册表”失败，界面没有该文字。`go vet ./...`、`go tool mygo vet .` 和上述包的 `go test -race` 退出码均为 0。

本机曾启动 `cmd/gocompare`，进程保持运行；日志里有输入法服务的 IMK 警告。这次启动不构成三平台验收。Windows 实时注册表、ACL、MTP、MSSCCI、真实云账号、签名公证仍未实现。交叉编译未在本轮重跑，不能写成目标系统已运行。

同日补上 F04：`TestAnchorAlignsInTheView` 在界面中把左侧 `keep` 对齐到右侧 `keep`，断言该行离开差异块，逆序锚点报 `anchor out of order` 且结果不变，解除对齐后该行重新成为差异，文件字节不变。`TestNavigateAndApplyBothDirections` 用上一处／下一处和两侧应用按钮改变缓冲区，并确认写盘仍留到保存。

同日补上 F02：`TestReadDetectedChoosesEncodingAndRejectsLimits` 覆盖 GBK 往返、UTF-16 BOM、UTF-8 优先、Latin-1 不猜测、NUL 拒绝，以及恰好 16 MiB 可读、超出拒绝。`TestSyntaxInlineAndDetectedText` 通过工作区 `load` 打开 GBK `.go`，断言 `func` 为关键字、注释为 `TokenComment`、唯一差异块是第 4 行，界面文字含 `GBK / LF`，并且行内高亮开关只改变该单词附近的像素。

## 2026-10-31 三栏对齐后再验证

F07 改为完成。`TestAlignmentsKeepThreeSides` 断言冲突行是基准 `b`、左侧 `left`、右侧 `right`，`Take` 把输出变成 `a\nleft\n` 后这三列原文仍在。`TestAlignmentsShowUnchangedSideAndPadding` 断言单侧修改仍显示未改一侧，并且左侧多出的 `L2` 在右侧没有对应行。`TestTwoWayAlignmentHasNoBase` 断言两方合并没有基准列。

`TestThreeColumnMergeAlignment` 从 `workspaceState.load` 进入合并，`ui.NewTester` 找到「基准第2行 b」「左侧第2行 left」「右侧第2行 right」，三格纵坐标重叠且从左到右排列；采用左侧后输出为 `a\nleft\n`，三列仍在，三个输入文件字节不变。`TestThreeColumnMergeScrollsTogether` 在 400 行里把第 301 行滚进视口前断言它未被构建，滚入后三列同时出现。

本轮 `go-test.log` 仍是先 `CGO_ENABLED=0 go test ./... -count=1`（`exit:0`），再追加同范围 `-v`（`vexit:0`）。追加段有 93 条顶层 `--- PASS`，协议子测试 `TestProtocols/ftp` 至 `TestProtocols/svn` 均通过，没有 `--- FAIL`。两次 `gocompare-cli` 的 `cli-run-1.log` 与 `cli-run-2.log` 经 `cmp` 一致，含 `<<<<<<< left`、`1 conflicts`、退出码 1、`preview left target absent`、`--apply` 退出码 0、`restore removed target` 和 `offset 0`。`ui-tester.log` 含上述三栏用例以及忽略空白、隐藏实时注册表，`ui-exit:0`。`vet.log` 中 `go vet ./...`、`go tool mygo vet .` 和 `go test -race`（merge、native、operations、workspace、gocompare-cli）均为 0。

F05 的 ACL、F08 的独立合并窗口，以及 F28、F29、F30、真实云账号、签名公证和三平台实机窗口仍未完成。本轮没有交叉编译，也不能写成目标系统已运行。

## 2026-10-31 独立合并窗口与图片解码

F08 改为完成。`TestIndependentMergeWindow` 在文件比较界面点击「打开合并窗口」后，主窗口 `kind` 仍为 0，仍有「忽略空白」，且没有「独立合并窗口」或「基准第2行 b」。新窗口的 `mergeWindowView` 显示这三列并对齐，没有文件比较工具栏。采用左侧后输出文件为 `a\nleft\n`，三个输入文件字节不变，主窗口的 `mergeResult` 仍为空。两方窗口没有「对齐列 基准」。`TestIndependentMergeWindowRejects` 在未选文件时显示错误且不创建窗口；二进制文件错误含 `UTF-8 required`。应用未就绪，测试没有调用 `NewWindow`。

F14 仍是部分。`TestOpenPNGAndRejectsOtherBytes` 读回 PNG 像素 `(10,20,30)`，第一处差异在 `(1,0)`，尺寸不同算差异，文本文件返回 `unknown format`。`TestOpenGIFExactAndJPEGBounds` 断言 GIF 红色像素和 JPEG 的 8×8 边界。`TestPictureCLIReadsPixels` 通过 `run` 得到 `pixel 1,0`、`equal` 和退出码 2。工作区没有图片画布。

本轮 `go-test.log` 先是 `CGO_ENABLED=0 go test ./... -count=1`（`exit:0`），再追加 `-v`（`vexit:0`）。追加段有 98 条顶层 `--- PASS`，没有 `--- FAIL`。两次 CLI 的 `cmp` 结果为 0，日志仍含 `1 conflicts`、`preview left target absent`、`restore removed target` 和 `offset 0`。`ui-tester.log` 的 `ui-exit:0`。`vet.log` 中 `go vet ./...`、`go tool mygo vet .` 和 `go test -race` 均为 0。

F05 的 ACL、F01 的三平台实机窗口、F17／F18 的真实服务器和账号、F28 签名公证、F29 进程扩展、F30 的 Windows ACL／MTP／MSSCCI 仍不能标完成。本轮没有交叉编译。

## 2026-10-31 工作区图片画布

F14 仍是部分。`TestWorkspacePictureCanvas` 从分段「图片比较」和「开始比较」进入 `load`，用两张磁盘上的 2×2 PNG。并排左上角为 R200 G10 B20，右侧起点为 R20 G30 B180；叠加该点为 R110 G20 B100；差值为 R180 G20 B160。容差 2 显示「超出容差」，容差 255 显示「容差内相同」，画面仍是差值。替换颜色把左上角换成 G10。水平翻转后左上角为原 `(1,0)` 的 B200，垂直翻转后左上角为原 `(0,1)` 的 G200。`pictureResult` 与「图片画布」按 20 倍放大后的方块中心颜色一致。两次读取输入文件的字节不变。`TestWorkspacePictureRejectsText` 对文本文件得到含 `unknown format` 的错误，界面有「错误」，没有「图片画布」，kind 仍是 3。GIF、JPEG 和其他格式没有走这条画布用例，也没有播放。

本轮 `go-test.log` 先是 `CGO_ENABLED=0 go test ./... -count=1`（`exit:0`），再追加 `-v`（`vexit:0`）。追加段有 100 条顶层 `--- PASS`，含 `TestWorkspacePictureCanvas` 和 `TestWorkspacePictureRejectsText`，没有 `--- FAIL`。`TestProtocols/ftp` 至 `TestProtocols/svn` 通过。两次 CLI 的 `cmp` 为 0，日志含 `<<<<<<< left`、`1 conflicts`、退出码 1、`preview left target absent`、`--apply` 退出码 0、`restore removed target` 和 `offset 0`。`ui-tester.log` 的 `ui-exit:0`。`vet.log` 中 `go vet ./...`、`go tool mygo vet .` 和 `go test -race` 均为 0。原生进程能启动，日志里的 IMK mach port 警告不是启动失败，也不构成三平台验收。本轮没有交叉编译。

## 2026-10-31 GIF 与 JPEG 画布

F14 改为完成。播放不在这一条里，仍是 F15，本轮没有做播放器。`TestWorkspacePictureGIF` 用调色板 GIF 走「图片比较」和「开始比较」，像素与 PNG 样本相同：并排左上角 R200 G10 B20，右侧起点 R20 G30 B180，叠加为 R110 G20 B100，差值为 R180 G20 B160，容差 2 为「超出容差」，容差 255 为「容差内相同」。`TestWorkspacePictureJPEG` 用质量 100 的 16×16 灰块：左上象限 40，右侧对应点 200，叠加 120，差值 160；水平翻转后左上象限变为 220，垂直翻转后变为 20。这些颜色同时在 `pictureResult` 和「图片画布」方块中心。`TestWorkspacePictureGIFStaysOnFirstFrame` 的第二帧是红色，`Advance` 1 秒后左上角仍是黑色，界面没有「播放」。BMP 错误含 `bmp is not supported` 且没有画布。输入字节不变。

本轮 `go-test.log` 先是 `CGO_ENABLED=0 go test ./... -count=1`（`exit:0`），再追加 `-v`（`vexit:0`）。追加段有 104 条顶层 `--- PASS`，含上述 GIF、JPEG、第一帧和 BMP 用例，没有 `--- FAIL`。`TestProtocols/ftp` 至 `TestProtocols/svn` 通过。两次 CLI 的 `cmp` 为 0，日志含 `<<<<<<< left`、`1 conflicts`、`preview left target absent`、`restore removed target` 和 `offset 0`。`ui-tester.log` 的 `ui-exit:0`。`vet.log` 中 `go vet ./...`、`go tool mygo vet .` 和 `go test -race` 均为 0。原生进程已启动，这不是三平台验收。本轮没有交叉编译。

## 2026-10-31 媒体标签

F15 仍是部分。没有播放器。`TestMediaCLIReadsTags` 通过已交付的 `run` 读取磁盘上的 ID3v2：标准输出为 `title LeftSong`、`artist 甲`、`album LeftAlbum`，退出码 0；文本文件的标准错误是 `not an id3v2 header`，退出码 2，标准输出为空。`TestWorkspaceMediaTags` 从分段「媒体标签」和「开始比较」进入 `load`。左侧标题 `LeftSong`、艺术家「甲」、专辑 `LeftAlbum`，右侧为 `RightSong`、`乙`、`RightAlbum`，这些字符串同时在模型和界面文本里。界面有「不能播放」，没有名为「播放」的控件。`Advance` 1 秒后标题仍是 `LeftSong`。`TestWorkspaceMediaRejectsText` 的错误是 `not an id3v2 header`，界面有「错误」，没有「左侧标题」，kind 仍是 4。两次读取输入字节不变。

本轮 `go-test.log` 先是 `CGO_ENABLED=0 go test ./... -count=1`（`exit:0`），再追加 `-v`（`vexit:0`）。追加段有 108 条顶层 `--- PASS`，含 `TestID3Title`、`TestID3ArtistAndAlbum`、`TestMediaCLIReadsTags`、`TestWorkspaceMediaTags` 和 `TestWorkspaceMediaRejectsText`，没有 `--- FAIL`。`TestProtocols/ftp` 至 `TestProtocols/svn` 通过。两次 CLI 的 `cmp` 为 0，日志含 `<<<<<<< left`、`1 conflicts`、`preview left target absent`、`restore removed target` 和 `offset 0`。`ui-tester.log` 的 `ui-exit:0`。`vet.log` 中 `go vet ./...`、`go tool mygo vet .` 和 `go test -race` 均为 0。原生进程已启动，这不是三平台验收，也不是签名公证。F01、F05、F17、F18、F28、F29、F30 没有改成完成。本轮没有交叉编译。

## 2026-10-31 进程扩展

F29 改为部分。`TestExtendCLI` 通过已交付的 `run` 调用 `extend`。比较夹具对两个内容为 `one` 的文件输出 `same one`、退出码 0；对 `one` 与 `two` 输出 `different one two`、退出码 1。转换 `Hello` 得到 `text:Hello`。转换失败的标准错误是 `extension exit 2: boom`，标准输出为空。压缩结果文件是 `PACK:raw`，标准输出是 `packed 8 bytes`。没有结果文件时标准错误是 `extension output missing`。未知种类是 `unknown extension kind`。超过 16 MiB 的标准输出被拒绝，且标准输出里没有成功内容。`TestExecuteCancelStopsTheProcess` 在子进程写下 pid 后取消，返回 `context.Canceled`，该 pid 不再存在。`TestExecuteRejectsPartialCompressAndEmbeddedPlaceholder` 对退出码 4 得到 `extension exit 4: failpack`，返回的载荷为空，输入字节不变；子进程自己写下的 `partial` 不作为成功结果。嵌入在参数中间的占位符被拒绝。规格不经 shell。没有 Windows 夹具，也不是 Beyond Compare 插件包。

本轮 `go-test.log` 先是 `CGO_ENABLED=0 go test ./... -count=1`（`exit:0`），再追加 `-v`（`vexit:0`）。追加段有 111 条顶层 `--- PASS`，含上述三个扩展用例，没有 `--- FAIL`。`TestProtocols/ftp` 至 `TestProtocols/svn` 通过。两次 CLI 的 `cmp` 为 0，日志含 `<<<<<<< left`、`1 conflicts`、`preview left target absent`、`restore removed target` 和 `offset 0`。`ui-tester.log` 的 `ui-exit:0`。`vet.log` 中 `go vet ./...`、`go tool mygo vet .` 和 `go test -race`（含 `extend`）均为 0。原生进程已启动，这不是三平台验收，也不是签名公证。F28 与 F30 仍是未实现。本轮没有交叉编译。

## 2026-10-31 目录规则

F09 改为完成。`TestDirectoryRulesCLI` 通过已交付的 `run` 调用 `dir`。内容相同、权限 `0600`/`0640`、修改时间不同的 `nested/a.txt` 在默认比较里是 `same`，`--compare-mode` 和 `--compare-time` 后是 `different`。`--filter nested/*.txt` 只留下这一行且不含 `b.bin`；再加上 `--compare-time` 时这一行是 `different`。过滤 `[` 的标准错误含 `syntax error in pattern`，退出码 2，标准输出为空。`--links nope` 为 `unknown link policy: nope`。两侧符号链接指向同一外部文件时，默认记录不含 `hidden`，也没有 `symlink escapes root`；`--links follow` 的 JSON 含该错误，退出码 2，标准输出仍不含 `hidden`。`TestWorkspaceDirectoryRules` 从「目录比较」进入：勾选「比较权限」后 `nested/a.txt` 显示「不同」，取消后显示「相同」且 `nested/b.bin` 仍是「不同」；「比较时间」同样把它标成不同。输入「目录过滤」`nested/*.txt` 后列表里只剩 `nested/a.txt`，扫描顺序是 `nested` 早于 `nested/a.txt`，最近扫描仍是 `nested/b.bin`。`TestWorkspaceDirectoryRejectsBadFilter` 的错误含 `syntax error in pattern`，不安装结果行。`TestWorkspaceDirectoryLinkPolicy` 默认不跟随，勾选「跟随链接」后该行是「读取失败」，错误为 `symlink escapes root`，界面没有 `hidden`。输入字节不变。不比较 ACL。成环仍由 `TestLinkFollowRejectsEscapeAndCycle` 覆盖。

本轮 `go-test.log` 先是 `CGO_ENABLED=0 go test ./... -count=1`（`exit:0`），再追加 `-v`（`vexit:0`）。追加段有 115 条顶层 `--- PASS`，含上述目录用例，没有 `--- FAIL`。`TestProtocols/ftp` 至 `TestProtocols/svn` 通过。两次 CLI 的 `cmp` 为 0，日志含 `<<<<<<< left`、`1 conflicts`、`preview left target absent`、`restore removed target` 和 `offset 0`。`ui-tester.log` 的 `ui-exit:0`，含三个新的目录工作区用例。`vet.log` 中 `go vet ./...`、`go tool mygo vet .` 和 `go test -race`（含 `native`）均为 0。原生进程已启动，这不是三平台验收，也不是签名公证。F01、F03 的真实输入法、F05 的 ACL、F15 的播放器、F17、F18、F28、F30 没有改成完成。本轮没有交叉编译。

## 2026-10-31 品牌与框架版本

F01 仍是部分。`TestIconMatchesTheRenderedMark` 解码嵌入的 `icon.png`，并确认它与 `resources/icon.png` 字节相同。圆角外 `(10,10)` 透明，中心底色为 R23 G43 B72，左页 `(200,250)` 为 R246 G248 B251，左侧差异色 `(300,450)` 为 R197 G229 B220，右侧差异色 `(600,450)` 为 R216 G203 B234，箭头 `(512,488)` 为 R38 G93 B171。`TestFrameworkIsTheLinkedModule` 从构建信息读到 `github.com/egoist/mygo` 的 `v0.7.3`。`TestWorkspaceShowsBrandAndFramework` 在工作区看到「应用图标」和 `MyGo v0.7.3`。`TestRunHelpReportsFramework` 调用 `Run([]string{"--help"})`，标准输出是 `GoCompare (v0.7.3)` 和原来的用法行。`Run` 在创建窗口前调用 `Dock.SetIcon`；失败会退出。Windows 与 Linux 实机窗口没有运行，F28 签名公证和 F30 仍是未实现。

本轮 `go-test.log` 先是 `CGO_ENABLED=0 go test ./... -count=1`（`exit:0`），再追加 `-v`（`vexit:0`）。追加段有 119 条顶层 `--- PASS`，含上述四个品牌用例，没有 `--- FAIL`。`TestProtocols/ftp` 至 `TestProtocols/svn` 通过。两次 CLI 的 `cmp` 为 0，日志含 `<<<<<<< left`、`1 conflicts`、`preview left target absent`、`restore removed target` 和 `offset 0`。`ui-tester.log` 的 `ui-exit:0`。`vet.log` 中 `go vet ./...`、`go tool mygo vet .` 和 `go test -race`（含 `brand` 与 `native`）均为 0。原生进程已启动，这不是三平台验收，也不是签名公证。本轮没有交叉编译。

## 2026-10-31 PE 版本与注册表文本

F16 改为完成。比较的是文件，不读取实时注册表。`TestPEAndRegistryCLI` 通过已交付的 `run` 调用 `pe` 和 `reg`。手造 PE 的左侧 `FileVersion` 是 `1.2.3.4`、右侧是 `9.8.7.6` 时，标准输出为 `left 1.2.3.4` 与 `right 9.8.7.6`，退出码 1；两侧同为 `1.2.3.4` 时退出码 0。文本文件的标准错误是 `not a PE file`，退出码 2，标准输出为空。参数不足时标准错误含 `Usage: gocompare-cli pe`。`.reg` 5.00 左侧 `Name=One` 且有 `Extra=Keep`，右侧 `Name=Two` 且没有 `Extra`，标准输出是 `HKEY_CURRENT_USER\Software\GoCompare\Extra` 和同键的 `Name`，退出码 1。同一文件输出 `equal`，退出码 0。非注册表文本的标准错误是 `unsupported registry header: hello`，退出码 2。两次读取输入字节不变。`TestWorkspacePEVersions` 从分段「版本与注册表」和「开始比较」进入 `load`。不同版本显示「左侧版本 1.2.3.4」「右侧版本 9.8.7.6」「版本不同」；相同版本显示「版本相同」，界面没有 `9.8.7.6`。`TestWorkspaceRegistryDiff` 列出上述两个键，界面有「不读取实时注册表」，没有值 `One`，也没有「左侧版本」。相同文件显示「注册表相同」。`TestWorkspaceVersionRejectsText` 的错误是 `not a PE file`，界面有「错误」，没有「左侧版本」。一侧 `.reg`、一侧 PE 的错误是「两侧须同为 .reg 或同为 PE」，不显示 `1.2.3.4` 或差异行。kind 仍是 5，不安装文本窗格。输入字节不变。

本轮 `go-test.log` 先是 `CGO_ENABLED=0 go test ./... -count=1`（`exit:0`），再追加 `-v`（`vexit:0`）。追加段有 123 条顶层 `--- PASS`，含 `TestPEAndRegistryCLI`、`TestWorkspacePEVersions`、`TestWorkspaceRegistryDiff` 和 `TestWorkspaceVersionRejectsText`，没有 `--- FAIL`。`TestProtocols/ftp` 至 `TestProtocols/svn` 通过。两次 CLI 的 `cmp` 为 0，日志含 `<<<<<<< left`、`1 conflicts`、`preview left target absent`、`restore removed target`、`rolled_back":true` 和 `offset 0`。`ui-tester.log` 的 `ui-exit:0`，含上述三个工作区用例。`vet.log` 中 `go vet ./...`、`go tool mygo vet .` 和 `go test -race`（含 `native` 与 `gocompare-cli`）均为 0。原生进程已启动，这不是三平台验收，也不是签名公证，也不是实时注册表。F01、F03 的真实输入法、F05 的 ACL、F15 的播放器、F17、F18、F28、F30 没有改成完成。本轮没有交叉编译。

## 2026-10-31 对齐的 FileVersion

F16 仍是完成。原先把键和值直接拼接，填充 0 会被当成版本结束符。`TestPEFileVersionAlignedString` 的资源是标准 String：6 字节头、UTF-16 `FileVersion`、32 位填充 0、值 `1.2.3.4`。`wValueLength` 按字计算。`FileVersion` 返回 `1.2.3.4`，不是 `empty FileVersion`。键值直接拼接的 `TestPEFileVersion` 仍返回 `1.2.3.4`。`TestPEAndRegistryCLI` 通过已交付的 `run` 比较这个对齐文件和未对齐的 `1.2.3.4`，标准输出是 `left 1.2.3.4` 与 `right 1.2.3.4`，退出码 0。`TestWorkspacePEVersions` 的右侧换成同样对齐的 `9.8.7.6`，界面有「右侧版本 9.8.7.6」和「版本不同」。CLI 与工作区都调用 `pever.FileVersion`。

本轮 `go-test.log` 先是 `CGO_ENABLED=0 go test ./... -count=1`（`exit:0`），再追加 `-v`（`vexit:0`）。追加段有 124 条顶层 `--- PASS`，含 `TestPEFileVersionAlignedString`，没有 `--- FAIL`。`TestProtocols/ftp` 至 `TestProtocols/svn` 通过。两次 CLI 的 `cmp` 为 0，日志含 `<<<<<<< left`、`1 conflicts`、`preview left target absent`、`restore removed target`、`rolled_back":true` 和 `offset 0`。`ui-tester.log` 的 `ui-exit:0`，含 `TestWorkspacePEVersions`。`vet.log` 中 `go vet ./...`、`go tool mygo vet .` 和 `go test -race`（含 `pever`、`native` 与 `gocompare-cli`）均为 0。原生进程已启动，这不是三平台验收，也不是签名公证，也不是实时注册表。F01、F03 的真实输入法、F05 的 ACL、F15 的播放器、F17、F18、F28、F30 没有改成完成。本轮没有交叉编译。

## 文件／目录工作区可用性修复

本批全量 146 个 Go 测试通过，native／document／compare／workspace race 检查通过；go vet、mygo vet 和补丁检查通过。浅深主题截图已检查，主操作与按侧保存通过 900／1380 像素边界断言。macOS 原生文件与目录回归均验证背景计算实际完成及规则切换、编辑、撤销、保存和返回目录。

详见 usability-review.md。本批没有提交或推送 Git，当前仓库仍无首个提交。测试通过不代表所有扩展模块具有产品级可用性。
