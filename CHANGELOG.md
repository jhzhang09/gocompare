# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project adheres to [Semantic Versioning](https://semver.org/). Release notes on GitHub are also generated from merged pull requests.

## [Unreleased]

### Added

- Homebrew tap: `brew install --cask jhzhang09/tap/gocompare` for the desktop app (also links a `gocompare` command) and `brew install jhzhang09/tap/gocompare-cli` for the CLI on macOS and Linux. Stable releases update the tap automatically.

### Fixed

- macOS install instructions no longer use Homebrew's removed `--no-quarantine` option or the right-click → Open bypass that macOS 15 dropped.

## [0.1.0] - 2026-11-01

First public release.

### Added

- Native desktop workspace with text, file, folder, three-way merge, picture, media tag and version/registry comparison.
- `gocompare-cli` with stable exit codes, folder sync with preview, backups, journal, rollback and undo, and Git `difftool`/`mergetool` support.
- Picture tolerance map, background color replacement and per-side flipping.
- English, Simplified Chinese and Traditional Chinese UI.
- `--version` for both executables; release builds embed the tag version.
- Release workflow that publishes installers, CLI archives, checksums and build provenance attestations.

### Fixed

- Ignoring blank lines no longer rewrites the editable buffers or loses unsaved edits.
- Undoing a sync refuses to overwrite or delete files changed after the sync.
- Case-only name collisions on case-insensitive file systems are rejected before sync.
- Merges treat a missing final newline as a change; merge output can be saved repeatedly.
- `.reg` exports in UTF-16 with continued hex lines and ID3v2.4 tags are parsed correctly.

[Unreleased]: https://github.com/jhzhang09/gocompare/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/jhzhang09/gocompare/releases/tag/v0.1.0
