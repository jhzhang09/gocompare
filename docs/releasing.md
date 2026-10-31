# 发版流程

日期：2026-11-01。适用于维护者。

## 产物

推送 `v<主>.<次>.<修订>` 标签后，[release.yml](../.github/workflows/release.yml) 依次执行：

| 任务 | Runner | 产物 |
|---|---|---|
| Verify | ubuntu | gofmt、vet、测试、MyGo 补丁校验，失败则不构建 |
| CLI archives | ubuntu | `scripts/package-cli.sh` 交叉编译 6 个平台的 `gocompare-cli`，tar.gz／zip，内含 LICENSE 与两份 README |
| App (macOS) | macos | `go tool mygo build -platform darwin/universal`：Universal `.dmg`（临时签名） |
| App (Linux) | ubuntu | amd64／arm64 的 `.deb` 与 `.tar.gz` |
| App (Windows) | windows | amd64／arm64 的 NSIS 安装程序（mygo 在 Windows 上自动下载 NSIS）与免安装 zip |
| Publish release | ubuntu | 汇总产物，生成 `SHA256SUMS.txt` 与构建来源证明，`gh release create` 发布；标签含 `-`（如 `v0.2.0-rc.1`）时标为预发布 |

版本号来自标签：工作流把 `mygo.json` 的 `version` 改成去掉 `v` 的标签（只在 CI 中修改，用于安装包命名），并通过 `MYGO_LDFLAGS`／`-ldflags` 注入 `gocompare/internal/brand.Version`，`--version` 和应用信息显示同一版本。本地构建显示 `dev`。

Release 正文由 [.github/release-notes.md](../.github/release-notes.md)（下载说明、签名提示）加上 GitHub 根据已合并 PR 自动生成的说明组成，分类规则在 [.github/release.yml](../.github/release.yml)。

## 步骤

1. 确认 `main` 上的 Checks 全部通过。
2. 更新 [CHANGELOG.md](../CHANGELOG.md)：把 `Unreleased` 的内容移到新版本标题下，并更新底部链接。
3. 可选：在 Actions 页面手动运行 Release（workflow_dispatch），只构建、不发布，版本号为 `0.0.0-dev`，用于提前检查三个平台的打包。
4. 打标签并推送：

   ```bash
   git tag -a v0.1.0 -m "GoCompare v0.1.0"
   git push origin v0.1.0
   ```

5. 工作流完成后检查 Release 页面的产物与说明；有问题时删除 Release 和标签，修复后重新打标签。

## 本地复现

```bash
scripts/package-cli.sh v0.1.0-local dist                       # CLI 归档
MYGO_LDFLAGS="-X gocompare/internal/brand.Version=v0.1.0-local" \
  go tool mygo build -platform darwin/universal -o dist          # 桌面应用（dmg 需在 macOS 上生成）
```

Windows 安装程序需要 NSIS，只在 Windows 上生成；其他平台的 Windows 构建只有可执行文件。

## 尚未配置

- macOS Developer ID 签名与公证：需要在 `mygo.json` 设置 `macos.signingIdentity`／`macos.notarize`，并在 CI 中导入证书。目前是临时签名，用户首次打开需右键"打开"。
- Windows 代码签名：目前未签名，SmartScreen 会提示。
- 应用内自动更新：MyGo 支持 `updates` 配置与 `mygo keygen` 签名密钥，尚未启用。
- 正式应用标识：`mygo.json` 的 `identifier` 仍是 `dev.gocompare.desktop`，正式发布前确认。

## 仓库设置

- 仓库：<https://github.com/jhzhang09/gocompare>
- 已开启 Private vulnerability reporting，SECURITY.md 依赖它。
