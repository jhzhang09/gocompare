## Downloads · 下载

| Platform · 平台 | Desktop app · 桌面应用 | CLI · 命令行 |
|---|---|---|
| macOS (Apple Silicon & Intel) | `gocompare-<version>.dmg` | `gocompare-cli_<version>_darwin_<arch>.tar.gz` |
| Windows | `gocompare-setup-<version>-<arch>.exe` (installer · 安装版) / `gocompare-<version>-windows-<arch>-portable.zip` (portable · 免安装版) | `gocompare-cli_<version>_windows_<arch>.zip` |
| Linux | `gocompare_<version>_<arch>.deb` / `gocompare-<version>-linux-<arch>.tar.gz` | `gocompare-cli_<version>_linux_<arch>.tar.gz` |

Verify downloads with `SHA256SUMS.txt`, or with the build provenance attestation: `gh attestation verify <file> --repo jhzhang09/gocompare`.
可用 `SHA256SUMS.txt` 或构建来源证明校验下载文件：`gh attestation verify <文件> --repo jhzhang09/gocompare`。

> **macOS:** the app is ad-hoc signed, not notarized. On first launch, right-click GoCompare.app → **Open**, or run `xattr -dr com.apple.quarantine /Applications/GoCompare.app`.
> **macOS：** 应用使用临时签名，尚未经过公证。首次启动时请右键点击 GoCompare.app → **打开**，或执行 `xattr -dr com.apple.quarantine /Applications/GoCompare.app`。
>
> **Windows:** the binaries are not code-signed, so SmartScreen may warn; choose **More info → Run anyway**.
> **Windows：** 程序未做代码签名，SmartScreen 可能提示风险，请选择**更多信息 → 仍要运行**。

