# Beyond Compare 5 功能对齐矩阵

基线：BC 5.2.7（Standard 与 Pro）、MyGo v0.7.3；核对日期：2026-10-31。

“部分”表示已有可运行能力，但该条尚未完整验收。“待验证”不等于支持。“未实现”保留为最终目标。

| 编号 | 能力 | 状态 | 当前证据／下一步 |
|---|---|---|---|
| F01 | 原生窗口、GoCompare 品牌、最新框架 | 部分 | 工作区用嵌入的 `icon.png`：箭头像素 `(512,488)` 为 R38 G93 B171，圆角外 `(10,10)` 透明，左右页和底色与 `icon.svg` 一致。界面有「应用图标」和 `MyGo v0.7.3`，版本来自构建信息里的 `github.com/egoist/mygo`。`Run(--help)` 打印 `GoCompare (v0.7.3)`。启动时用 `Dock.SetIcon` 设置同一张 PNG；darwin 的 `Window.SetIcon` 不会改图标。Windows 与 Linux 实机窗口未运行，因此不标完成 |
| F02 | 双文件文本比较、语法与行内差异 | 完成 | 工作区用 `ReadDetected` 打开可识别的 GBK `.go`：`func` 为关键字、`//` 为注释，差异块只有改动的那一行；行内高亮开关后的像素差落在 `alpha`／`beta` 上。界面显示 `GBK / LF`。超过 16 MiB 拒绝。Latin-1 仍须显式指定 |
| F03 | 双侧编辑、撤销、查找替换 | 部分 | 同屏双侧输入、撤销、焦点侧保存和 `ReplaceAll` 已测；真实 IME 待验证 |
| F04 | 差异导航与双向应用 | 完成 | 界面可上一处／下一处差异，并把当前块应用到左侧或右侧，且不立即写盘。标记左右行后，左侧 `keep` 对齐到右侧 `keep`，该行不再落入差异块；逆序锚点显示错误且不改变结果；解除对齐后第一行重新成为差异 |
| F05 | 保真保存与外部修改检测 | 部分 | 覆盖式另存为在外部修改时返回 `ErrChanged` 且不写；ACL 未实现 |
| F06 | 算法、重要性与忽略规则 | 完成 | Myers／Patience／Histogram 可还原右侧文本；空白、大小写、注释、正则、列和替换已测，非法正则与逆序锚点会失败 |
| F07 | 同屏双侧及三方对齐 | 完成 | 文件比较仍用共享滚动的双侧视图。三方合并在同一界面列出基准、左侧、右侧：`a/b`、`a/left`、`a/right` 时第 2 行三列分别是 `b`、`left`、`right` 且纵坐标重叠；单侧修改仍显示未改的一侧，较短一侧标成空行。决策后输出变为 `a\nleft\n`，三列原文仍在，且不写输入文件。视口外的第 301 行滚动前不会构建，滚到该行后三列同时出现 |
| F08 | 两方／三方文本合并与独立输出 | 完成 | 主窗口点「打开合并窗口」后仍停留在文件比较。新窗口显示基准 `b`、左侧 `left`、右侧 `right` 且横向对齐，没有「忽略空白」。采用左侧后输出为 `a\nleft\n`，输入文件不变。两方窗口没有基准列。未选择文件或二进制内容拒绝，且不创建窗口。应用未就绪时不调用 `NewWindow`，测试驱动的是该窗口的内容 |
| F09 | 递归目录比较 | 完成 | CLI `dir` 与工作区「目录比较」把过滤、权限、修改时间和链接策略交给 `CompareDirectories`。默认时 `nested/a.txt` 为相同；`--compare-mode` 与「比较权限」后为不同，`--compare-time` 与「比较时间」后也为不同。`--filter nested/*.txt` 与「目录过滤」只留下这一行，列表里没有 `nested/b.bin`，扫描顺序仍是 `nested` 早于 `nested/a.txt`，最近扫描可以是被过滤掉的 `nested/b.bin`。非法过滤 `[` 的标准错误含 `syntax error in pattern`，退出码 2，工作区不安装结果行。`--links nope` 为 `unknown link policy: nope`。两侧指向同一外部文件时，默认记录不含 `hidden`，也没有 `symlink escapes root`；`--links follow` 与「跟随链接」得到该错误且退出码 2，不显示文件内容。输入字节不变。不比较 ACL。成环仍由库测试覆盖 |
| F10 | 两方／三方目录合并 | 未实现 | 2026-10-31 删除了从未接入 GUI／CLI 的原型包，重新实现时需接入入口并补验收（原 `internal/dirmerge`） |
| F11 | 更新、镜像、同步预览与恢复 | 完成 | 预览不写目标、确认执行、失败回滚、日志撤销、暂停后从同一日志继续已测。成功后若用户再改文件，恢复仍用备份覆盖 |
| F12 | CSV／TSV／Excel／HTML 表格 | 未实现 | 2026-10-31 删除了从未接入 GUI／CLI 的原型包，重新实现时需接入入口并补验收（原 `internal/table`） |
| F13 | 十六进制比较与编辑 | 完成 | 分块差异、搜索、插入、删除和撤销已测；CLI `hex` 输出偏移 |
| F14 | 图片比较 | 完成 | PNG 与 GIF 的 2×2 样本、以及 JPEG 的 16×16 灰块都从「图片比较」进入画布。并排、叠加、差值、容差（0–255 滑块，超出容差的像素标红，容差内显示左图 40% 亮度）、替换颜色（左图左上角颜色按当前容差在两侧同时替换为品红）的指定像素同时出现在 `pictureResult` 和「图片画布」方块中心。翻转是左右两侧各自的水平／垂直开关，可叠加、再点恢复，所有模式和统计都按翻转后的图片比较，图例标明哪一侧已翻转。差异像素数、超出容差像素数与画布一起在后台计算；解码前按文件头拒绝超过 `picture.MaxPixels` 的图片。JPEG 左上象限灰度为 40，右侧对应点为 200，叠加为 120，差值为 160。GIF 第二帧是红色，推进 1 秒后画布仍是第一帧黑色，界面没有「播放」。文本文件含 `unknown format`，BMP 含 `bmp is not supported`，两者都没有画布。输入字节不变。播放仍属于 F15 |
| F15 | 媒体标签与播放 | 部分 | CLI `media` 与工作区「媒体标签」读出 ID3v2 的 `TIT2`、`TPE1`、`TALB`。左侧标题 `LeftSong`、艺术家「甲」、专辑 `LeftAlbum`，右侧为 `RightSong`／「乙」／`RightAlbum`。界面有「不能播放」，没有名为「播放」的控件；推进 1 秒后标题仍是 `LeftSong`。文本文件错误为 `not an id3v2 header`，不显示标签。输入字节不变。没有播放器。只读取文件开头的标签区（上限 16 MiB），支持 ID3v2.3／2.4、扩展头，以及 Latin-1、UTF-16（两种字节序）和 UTF-8 文本；v2.2、整体不同步和压缩帧明确报错 |
| F16 | PE 版本信息与注册表 | 完成 | CLI `pe` 与工作区「版本与注册表」从手造 PE 读出 `FileVersion`。左侧 `1.2.3.4`、右侧 `9.8.7.6` 时界面为「左侧版本 1.2.3.4」「右侧版本 9.8.7.6」「版本不同」，标准输出是 `left 1.2.3.4` 与 `right 9.8.7.6`，退出码 1。两侧同为 `1.2.3.4` 时界面为「版本相同」，退出码 0。DWORD 对齐的 String 在键后带填充 0，仍按 `wValueLength` 读出 `1.2.3.4`，不会报 `empty FileVersion`。CLI 拿该文件和未对齐的 `1.2.3.4` 比较，退出码 0。工作区右侧用同样对齐的 `9.8.7.6`，界面是「右侧版本 9.8.7.6」和「版本不同」。CLI `reg` 与同一工作区比较 `.reg` 5.00：左侧 `Name=One` 且有 `Extra=Keep`，右侧 `Name=Two` 且没有 `Extra`。标准输出和界面列出 `HKEY_CURRENT_USER\Software\GoCompare\Extra` 与同键的 `Name`，退出码 1，界面不显示值 `One`。相同文件输出 `equal`，界面为「注册表相同」，退出码 0。文本文件错误是 `not a PE file`，界面有「错误」，没有「左侧版本」。一侧 `.reg`、一侧 PE 时错误是「两侧须同为 .reg 或同为 PE」，不显示版本或差异。界面写明「不读取实时注册表」。输入字节不变。不读实时注册表。`.reg` 支持注册表编辑器默认的 UTF-16LE（以及 UTF-16BE、UTF-8 BOM）、反斜杠续行的 hex 值和含 `=`／转义引号的值名 |
| F17 | FTP／FTPS／SFTP／SMB／WebDAV | 未实现 | 2026-10-31 删除了从未接入 GUI／CLI 的原型包，重新实现时需接入入口并补验收（原 `internal/remote`） |
| F18 | S3／Dropbox／OneDrive／SVN | 部分 | 同上，另含 S3 SigV4 头、Dropbox content API、Graph 与 SVN HTTP 子集。无真实账号，不能标完成 |
| F19 | 压缩包读取与修改 | 未实现 | 2026-10-31 删除了从未接入 GUI／CLI 的原型包，重新实现时需接入入口并补验收（原 `internal/archive`） |
| F20 | 无内容目录快照 | 未实现 | 2026-10-31 删除了从未接入 GUI／CLI 的原型包，重新实现时需接入入口并补验收（原 `internal/snapshot`） |
| F21 | Word／PDF／RTF 等转换 | 部分 | docx、RTF 和未压缩 PDF 流可转文本；不可逆，不识别全部 PDF 过滤器 |
| F22 | 格式化 HTML 原生展示 | 部分 | `htmlview` 抽出文本块并另存报告，包内不导入 WebView；不是完整排版 |
| F23 | 会话、多窗口、工作区恢复 | 部分 | 工作区可原子保存多个会话并恢复路径，不写入编辑缓冲；界面仍是单窗口。重新打开合并会话会重新计算 |
| F24 | CLI | 部分 | 比较、目录（过滤、权限、时间、链接策略）、同步、合并、十六进制、图片、媒体标签、PE 版本、`.reg` 文本、进程扩展、脚本、报告、分页和工作区入口已测；不是通用脚本语言 |
| F25 | 脚本、报告、打印 | 部分 | 脚本调用同一比较核心；HTML 报告可另存；分页器可测。未接操作系统打印对话框 |
| F26 | Git difftool／mergetool、文件管理器集成 | 部分 | `difftool`／`mergetool` 和文件管理器参数调用同一 CLI 核心。`mergetool` 按 Git 约定替换已存在的 `$MERGED`（写入前核对读取时的基线），已用真实 `git mergetool` 验证；`merge` 子命令仍拒绝覆盖。未注册到 Git 或系统文件管理器 |
| F27 | Unicode／MBCS、区域规则与国际化 | 部分 | 内容支持 UTF-8、UTF-16LE／BE、GBK 和 Latin-1；界面为中文。区域规则未做 |
| F28 | 安装、更新、签名与平台专属能力 | 未实现 | 无签名公证和安装升级的环境证据 |
| F29 | 外部比较、转换、压缩扩展接口 | 部分 | `gocompare-cli extend` 读取版本 1 的 JSON 规格，按 argv 启动绝对路径，不经 shell。比较夹具对相同的 `one` 输出 `same one` 且退出码 0，对 `one` 与 `two` 输出 `different one two` 且退出码 1。转换 `Hello` 得到 `text:Hello`；失败时标准错误是 `extension exit 2: boom`，标准输出为空。压缩写出 `PACK:raw` 并打印 `packed 8 bytes`；进程退出码 0 但没有结果文件时是 `extension output missing`。退出码 4 时子进程写下的 `partial` 不会成为成功结果。占位符必须独占一个参数。标准输出超过 16 MiB 被拒绝。取消 ctx 会结束该进程。不是 Beyond Compare 插件包，也没有在 Windows 上运行这些夹具 |
| F30 | Windows ACL／MTP／MSSCCI | 未实现 | 非 Windows 的 `PlatformActions` 为空，界面测试确认不显示“实时注册表”。这些能力本身未实现 |

## 官方依据

- [完整功能表](https://www.scootersoftware.com/kb/feature_compare)
- [文本对齐](https://www.scootersoftware.com/v5help/sessiontextalignment.html)
- [文本合并](https://www.scootersoftware.com/v5help/viewtextmerge.html)
- [目录合并](https://www.scootersoftware.com/v5help/viewdirmerge.html)
- [同步操作](https://www.scootersoftware.com/v5help/dir_sync_how_to_sync.html)
- [表格](https://www.scootersoftware.com/v5help/viewtable.html)
- [压缩包](https://www.scootersoftware.com/v5help/archive_files.html)
- [文件格式](https://www.scootersoftware.com/v5help/file_formats.html)
- [脚本](https://www.scootersoftware.com/v5help/scripting_reference.html)

BC 文件格式兼容单独验收，不能根据具备相同业务功能就宣称兼容 BC 会话、脚本或快照格式。
