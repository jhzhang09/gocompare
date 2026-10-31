# 平台支持

MyGo v0.7.3 的声明范围：macOS 12+、Windows 10／11、Linux GTK 3；amd64 与 arm64。应用本身需要逐平台验证，不继承框架测试结论。

| 平台 | 构建目标 | 运行与发布验收 |
|---|---|---|
| macOS | darwin/amd64、darwin/arm64、Universal | 原生窗口、输入法、多屏、签名、公证、DMG |
| Windows | windows/amd64、windows/arm64 | DPI、长路径、UNC、占用文件、替换保存、安装器与签名 |
| Linux | linux/amd64、linux/arm64 | GTK 3、X11／Wayland、中文字体及输入法、deb 与归档 |

```sh
go tool mygo build -platform darwin/universal,windows/amd64,windows/arm64,linux/amd64,linux/arm64
```

macOS 签名／磁盘镜像需要 Mac；安装器依赖对应工具。RPM 不属于当前已配置产物。CI 构建原始二进制，并未配置发布、签名密钥或自动更新服务。

GUI 可执行文件为 GoCompare，独立控制台程序为 gocompare-cli，避免 Windows 大小写不敏感路径冲突。Windows 不用 GUI 子系统程序代替批处理入口。

开发标识符 `dev.gocompare.desktop` 尚未作为正式发行身份；正式模块仓库与签名主体在发布前确定。
