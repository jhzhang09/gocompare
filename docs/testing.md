# 测试与故障场景

## 当前检查

- document：BOM／CRLF 保存、无末尾换行、外部修改、同内容文件替换、取消、无效编码与混合换行。
- compare／CLI：空文件、块边界、长度差、换行差、目录和错误输入、退出码。
- native：差异块复制、保存、模式切换、未保存确认、外部修改提示及截图。
- git／diff：迁移保留原有回归用例。

测试截图由 MyGo Tester 的 CPU 渲染生成，用于布局检查；不证明 Metal／D3D／OpenGL、系统输入法和文件对话框运行正确。

## 后续验收

- 核心算法：Unicode、重复文本、空内容、超长单行、大量插删；性质测试和 fuzz。
- 文件操作：权限、磁盘满、磁盘移除、文件被占用、重命名、符号链接、ACL、异常退出。
- UI：真实键盘、IME、焦点离开返回、页面消失重建、深浅主题、高 DPI、无障碍。
- 跨平台：构建、无窗口测试、真实运行和安装升级各自留证。
- 性能：百万行、十万目录项、大型二进制、高延迟网络；基准记录设备、数据集、首次结果、UI 帧耗时及内存。

当前原型 16 MiB 限制只是资源边界，未执行规模性能验收。远程操作和多文件同步不能用单文件保存测试代替。

## 原生自动化回归

```sh
go build -tags e2e -o /tmp/gocompare-e2e ./cmd/gocompare
python3 tests/native_smoke.py /tmp/gocompare-e2e --output /tmp/gocompare-native-result
```

脚本创建临时文件并自动关闭应用；输出目录保存日志、元素树和截图。只有 e2e 构建包含自动化接口。脚本依赖 Unix domain socket；Windows 需在支持 Python AF_UNIX 的环境中单独验证。

同屏双侧回归还覆盖默认只读兼容、I/O 锁定、焦点侧保存、长行换行及插删后差异块的精确重建。依赖补丁维护见 mygo-patch.md。

目录原生回归：

```sh
python3 tests/native_smoke.py /tmp/gocompare-e2e --directories --output /tmp/gocompare-directory-result
```

同步执行测试全部使用隔离临时目录；包含预览不写入、确认取消不修改、旧计划拒绝、父路径链接替换拒绝、原内容备份、已完成日志、备份目录不得位于来源内等场景。自动恢复与跨进程 CAS 尚未提供。
