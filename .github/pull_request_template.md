## What and why · 改了什么、为什么

<!-- Link related issues, e.g. "Fixes #123" · 关联的 Issue -->

## How it was tested · 如何验证

- [ ] `gofmt -l cmd internal tools` prints nothing
- [ ] `go vet ./...`
- [ ] `CGO_ENABLED=0 go test ./...`
- [ ] `go tool mygo vet .`
- [ ] Added or updated tests for the change · 已补充或更新测试

## Checklist · 检查清单

- [ ] User-visible text is translated in `internal/i18n` (zh-CN, zh-TW, en) · 界面文案已三语翻译
- [ ] File-writing code keeps the guarantees in `docs/operations.md` · 写文件的代码遵守安全约定
- [ ] Documentation / CHANGELOG updated if needed · 已按需更新文档与变更日志
