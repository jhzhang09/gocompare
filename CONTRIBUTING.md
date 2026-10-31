# Contributing to GoCompare

Thanks for your interest in GoCompare! Bug reports, feature ideas, documentation fixes and code are all welcome. Issues and pull requests may be written in English or Chinese.

欢迎参与！Issue 和 Pull Request 用中文或英文都可以。

## Reporting bugs and requesting features

- Search [existing issues](https://github.com/jhzhang09/gocompare/issues) first.
- Use the issue templates; include your OS, GoCompare version (`gocompare --version`) and the smallest files or steps that reproduce the problem.
- **Security problems:** do not open a public issue — see [SECURITY.md](SECURITY.md).

## Development setup

You need Go 1.27.3 or newer. Dependencies are vendored; no network access or cgo is required.

```bash
git clone https://github.com/jhzhang09/gocompare.git
cd gocompare
go run ./cmd/gocompare            # desktop app
go run ./cmd/gocompare-cli --help # command line
```

Linux additionally needs GTK 3 development libraries to *run* the desktop app (building works anywhere).

## Before you open a pull request

Run the same checks as CI:

```bash
gofmt -l cmd internal tools     # must print nothing
go vet ./...
CGO_ENABLED=0 go test ./...
go tool mygo vet .
git apply --reverse --check patches/mygo-v0.7.3-native.patch
```

Guidelines:

- **Keep changes focused.** One logical change per pull request; avoid unrelated refactors or reformatting.
- **Add tests.** Bug fixes should come with a test that fails without the fix. UI behavior can be tested headlessly with `ui.NewTester` (see `internal/native/*_test.go`).
- **Never trade safety for convenience.** Code that writes files must keep the guarantees in [docs/operations.md](docs/operations.md): no silent overwrites, baseline checks, backups and rollback.
- **Translate UI text.** Every user-visible string goes through `internal/i18n` with Simplified Chinese, Traditional Chinese and English; `go test ./internal/i18n` checks completeness.
- **Do not edit `vendor/` directly.** The MyGo framework carries a small patch; follow [docs/mygo-patch.md](docs/mygo-patch.md) to change or upgrade it.
- **Comments** explain intent, constraints and edge cases. Existing code comments are in Chinese; either language is fine for new ones.

## Commit messages

We use [Conventional Commits](https://www.conventionalcommits.org/): `feat: …`, `fix: …`, `docs: …`, `chore: …`, `refactor: …`, `test: …`. Pull request labels (`feature`, `bug`, `documentation`, `breaking`) drive the generated release notes.

## License

By contributing, you agree that your contributions are licensed under the [MIT License](LICENSE).
