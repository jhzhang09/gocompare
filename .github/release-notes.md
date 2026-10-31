## Downloads · 下载

| Platform · 平台 | Desktop app · 桌面应用 | CLI · 命令行 |
|---|---|---|
| macOS (Apple Silicon & Intel) | `gocompare-<version>.dmg` | `gocompare-cli_<version>_darwin_<arch>.tar.gz` |
| Windows | `gocompare-setup-<version>-<arch>.exe` (installer · 安装版) / `gocompare-<version>-windows-<arch>-portable.zip` (portable · 免安装版) | `gocompare-cli_<version>_windows_<arch>.zip` |
| Linux | `gocompare_<version>_<arch>.deb` / `gocompare-<version>-linux-<arch>.tar.gz` | `gocompare-cli_<version>_linux_<arch>.tar.gz` |

Homebrew: `brew install --cask jhzhang09/tap/gocompare` (app) · `brew install jhzhang09/tap/gocompare-cli` (CLI)

Verify downloads with `SHA256SUMS.txt`, or with the build provenance attestation: `gh attestation verify <file> --repo jhzhang09/gocompare`.
可用 `SHA256SUMS.txt` 或构建来源证明校验下载文件：`gh attestation verify <文件> --repo jhzhang09/gocompare`。

> **macOS:** the app is ad-hoc signed, not notarized. Run `xattr -dr com.apple.quarantine /Applications/GoCompare.app` once, or choose **System Settings → Privacy & Security → Open Anyway** after the first launch attempt.
> **macOS：** 应用使用临时签名，尚未经过公证。执行一次 `xattr -dr com.apple.quarantine /Applications/GoCompare.app`，或在首次尝试打开后到**系统设置 → 隐私与安全性**点击**仍要打开**。
>
> **Windows:** the binaries are not code-signed, so SmartScreen may warn; choose **More info → Run anyway**.
> **Windows：** 程序未做代码签名，SmartScreen 可能提示风险，请选择**更多信息 → 仍要运行**。

