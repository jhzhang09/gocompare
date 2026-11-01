# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project adheres to [Semantic Versioning](https://semver.org/). Release notes on GitHub are also generated from merged pull requests.

## [Unreleased]

## [0.3.0] - 2026-11-01

### Added

- Remote files in File Compare: the new **Remote…** button next to each path opens a connection dialog for SFTP, SMB, FTP/FTPS and WebDAV. Type in host and user, or optionally import an SFTP host from `~/.ssh/config`; SMB shares can be picked from a list. SFTP signs in with a password, a private key (passphrase-protected keys are supported), ssh-agent or the default `~/.ssh/id_*` keys, and connection errors are explained in plain language. Host key algorithms are negotiated from the key types already in known_hosts, as OpenSSH does, so a host registered only with ed25519 no longer fails as "key mismatch" when the server also offers ECDSA, connect, then browse folders and double-click a file; with the other side filled, the comparison loads right away. Recent connections are remembered without passwords; a remembered folder is reused only for the same connection, and the dialog falls back to the start folder if it no longer exists. Remote URLs typed into the path field keep working, including S3, Dropbox, OneDrive and SVN. Passwords stay in memory only and are removed from the field, title and saved sessions. Unknown SSH host keys open a fingerprint dialog and are added to `~/.ssh/known_hosts` only after you trust them. Saving re-reads the remote file and refuses to overwrite changes made by someone else; Save As writes local files.
- Remote folders in Folder Compare: **Remote…** also appears in folder mode and picks a folder over SFTP, SMB, FTP/FTPS or WebDAV; either side, or both, can be remote. SFTP and SMB reuse one connection for the whole scan, and only same-size files are downloaded to compare contents. Opening a file from the result opens the remote file; a file present only on the local side opens an empty remote buffer that is created on save. Sync, folder merge and undo stay local-only and are disabled while a side is remote.
- Previous/Next Difference now highlight the whole change and show "2 / 7" in the toolbar and the status bar, so the jump is easy to follow.

## [0.2.0] - 2026-11-01

### Added

- Table Compare (F12): CSV, TSV, Excel (.xlsx) and HTML table comparison (`internal/table`). The CLI `table` command supports composite keys, column mapping and numeric tolerance; the Native GUI mode aligns by row number.
- Directory Merge (F10): Two-way and three-way directory merge with text conflict markers (`internal/dirmerge`, CLI `dirmerge`, GUI preview panel). Binary and delete/modify conflicts are written as `<path>.left` / `<path>.right` instead of placeholder files.
- Remote Storage Protocols (F17 & F18): FTP, FTPS (explicit TLS), SFTP, SMB2/3, WebDAV, AWS S3 SigV4, Dropbox, OneDrive and an SVN HTTP subset (`internal/remote`, CLI `remote [--timeout] [--known-hosts] [--identity]`). SFTP runs over SSH and only connects to hosts already listed in known_hosts; it authenticates with the URL password, ssh-agent or a private key. SMB uses NTLMv2 with `smb://[domain;]user:pass@host/share/path`. `webdav+https`, `s3+https` and `svn+https` select HTTPS; WebDAV and SVN accept Basic credentials.
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

[Unreleased]: https://github.com/jhzhang09/gocompare/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/jhzhang09/gocompare/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/jhzhang09/gocompare/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/jhzhang09/gocompare/releases/tag/v0.1.0
