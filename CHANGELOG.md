# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project adheres to [Semantic Versioning](https://semver.org/). Release notes on GitHub are also generated from merged pull requests.

## [Unreleased]

### Added

- Table Compare (F12): CSV, TSV, Excel (.xlsx) and HTML table comparison (`internal/table`). The CLI `table` command supports composite keys, column mapping and numeric tolerance; the Native GUI mode aligns by row number.
- Directory Merge (F10): Two-way and three-way directory merge with text conflict markers (`internal/dirmerge`, CLI `dirmerge`, GUI preview panel). Binary and delete/modify conflicts are written as `<path>.left` / `<path>.right` instead of placeholder files.
- Remote Storage Protocols (F17 & F18): FTP, FTPS (explicit TLS), WebDAV, AWS S3 SigV4, Dropbox, OneDrive and an SVN HTTP subset (`internal/remote`, CLI `remote [--timeout]`). `webdav+https`, `s3+https` and `svn+https` select HTTPS; WebDAV and SVN accept Basic credentials. `sftp://` and `smb://` URLs are rejected because SSH and NTLM/Kerberos authentication are not implemented.
- Archive Management (F19): Zip read/write/pack, tar.gz read-only, archive content comparison (`internal/archive`, CLI `archive`). `pack` never overwrites an existing output file.
- Contentless Directory Snapshot (F20): Fast snapshot creation and offline comparison (`internal/snapshot`, CLI `snapshot`). `create` never overwrites an existing snapshot file.
- Document Text Extraction (F21): docx, RTF and PDF text extraction (`internal/doctext`, CLI `doctext`). PDF text comes only from text-showing operators in content streams, including FlateDecode streams; font, image and other non-content streams are skipped. File comparison opens docx and PDF as read-only extracted text; RTF stays editable source, and merges reject extracted documents.
- Software Update Checker (F28): In-app and CLI update checks against GitHub Releases with SemVer precedence (pre-releases included) and whole-word platform asset matching that picks the desktop installer in the app and the CLI archive in the CLI (`internal/updater`, CLI `check-update`, Native GUI Settings). The in-app check runs alongside other work without interrupting it.
- Permission Comparison (F30): Unix mode, owner and group inspection and diffing (`internal/acl`, CLI `acl`). Windows ACLs are reported as unsupported.
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
