<div align="center">

<img src="resources/icon.png" width="96" alt="GoCompare icon">

# GoCompare

**A native, cross-platform file and folder comparison tool written in Go.**

Compare text, files, folders, tables and images — locally or on SFTP, SMB, FTP and WebDAV servers — merge three-way, and sync directories safely, in a fast native window or from the command line.

[![Checks](https://github.com/jhzhang09/gocompare/actions/workflows/check.yml/badge.svg)](https://github.com/jhzhang09/gocompare/actions/workflows/check.yml)
[![Release](https://img.shields.io/github/v/release/jhzhang09/gocompare?include_prereleases&sort=semver)](https://github.com/jhzhang09/gocompare/releases)
[![Go](https://img.shields.io/github/go-mod/go-version/jhzhang09/gocompare)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
![Platforms](https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey)

**English** · [简体中文](README.zh-CN.md)

</div>

![Text compare](docs/images/text-compare.png)

## Why GoCompare

- **Native, not a web view.** Built with [MyGo](https://github.com/egoist/mygo): pure Go, no Electron, WebView, Node.js or cgo. One small binary per platform, cross-compiled from any machine.
- **Safe by default.** Saves never overwrite a file that changed on disk, folder sync always previews first, backs up before overwriting or deleting, and rolls back on failure.
- **Local or remote, same workflow.** Compare a file or a whole folder on your machine against one on an SFTP, SMB, FTP or WebDAV server — pick it in a dialog, edit, and save back safely.
- **GUI and CLI share one core.** Everything you can do in the window you can script, with stable exit codes for CI and Git.

> [!NOTE]
> GoCompare is in early development (pre-1.0). Core comparison, merge and sync paths are well tested, but some features are still partial — see the [feature matrix](docs/feature-parity.md). Feedback and bug reports are very welcome.

## Features

| Mode | What it does |
|---|---|
| **Text compare** | Paste or type into two panes and see differences instantly, with word-level highlights. Swap, copy, clear and save either side. |
| **File compare** | Side-by-side diff with **both sides editable**, undo/redo, find & replace, manual alignment anchors, ignore whitespace / blank lines. Previous / Next Difference highlights the change and shows where you are ("2 / 7"). Detects UTF-8, UTF-16 (BOM) and GBK and preserves encoding, BOM and line endings on save. Word (.docx) and PDF files open as read-only extracted text. |
| **Folder compare** | Recursive tree with collapse, filter and "only differences". Optionally compares permissions and modification times and follows symlinks safely. Either side can be a remote folder. |
| **Remote sources (all modes)** | Click **Remote…** next to a path in any comparison mode (files, folders, merges, tables, images, media, versions) to connect to **SFTP, SMB, FTP/FTPS or WebDAV**, then browse and pick a file or folder — no URLs to type. SFTP hosts can be imported from `~/.ssh/config` and sign in with a password, a private key (with passphrase), ssh-agent or your default keys; unknown host keys are shown for you to verify. Recent connections are remembered without passwords. S3, Dropbox, OneDrive and SVN work through URLs. |
| **Drag & drop auto-fill** | Drag files or folders directly onto the path capsules or into the empty view to auto-fill. Dropping two items at once populates both sides and begins comparison immediately. Dropping a folder in File Compare automatically switches to Directory Compare. |
| **Folder sync** | Update left/right/both or mirror. Always previews a plan; executes with backups, a journal, automatic rollback on failure and one-click undo. |
| **Table compare** | CSV, TSV, Excel (.xlsx) and HTML tables, cell by cell. The CLI adds key columns, column mapping and numeric tolerance. |
| **Three-way merge** | Base / left / right aligned columns, take left / right / both / base per conflict, edit the output freely. Also works as a Git `mergetool`. |
| **Picture compare** | PNG, JPEG, GIF: side by side, overlay, pixel difference, a 0–255 tolerance map that marks real changes in red, background color replacement and per-side flipping. |
| **Media tags** | Compares ID3v2.3 / v2.4 title, artist and album without decoding audio. |
| **Version & registry** | Compares PE `FileVersion` resources and Windows `.reg` exports (UTF-16 or UTF-8) without touching the live registry. |
| **Sessions & i18n** | Save and reopen comparison sessions. UI in English, 简体中文 and 繁體中文; light, dark or system theme. Checks GitHub for new releases from Settings. |

<table>
  <tr>
    <td><img src="docs/images/folder-compare.png" alt="Folder compare"></td>
    <td><img src="docs/images/picture-compare.png" alt="Picture compare with tolerance map"></td>
  </tr>
  <tr>
    <td align="center">Folder compare</td>
    <td align="center">Picture compare (tolerance map)</td>
  </tr>
</table>

## Remote files and drag-and-drop

All comparison modes (**File compare**, **Folder compare**, **Text merge**, **Table compare**, **Picture compare**, **Media tags**, and **Version compare**) can connect directly to servers or load dropped files. Either side — or both — can be remote.

### Remote connections

1. Click **Remote…** next to the left or right path in any mode.
2. Choose the protocol and enter the host and user name. For SFTP you can also import a host from `~/.ssh/config`.
3. Enter a password, or choose a private key (fill in its passphrase if it has one). Leave both empty to use ssh-agent or your default keys.
4. Click **Connect**, browse the server, and pick a file (**Open**) or a folder (**Choose this folder**). If the other side is already set, the comparison starts right away.

| Protocol | Browse in the dialog | Sign-in |
|---|---|---|
| SFTP (SSH) | Yes | Password, private key (+ passphrase), ssh-agent, `~/.ssh/id_*` |
| SMB (Windows share / NAS) | Yes, including the list of shares | User, password, optional domain (NTLMv2) |
| FTP / FTPS | Yes (servers with MLSD) | User and password |
| WebDAV / WebDAV over HTTPS | Yes | User and password (Basic) |
| S3, Dropbox, OneDrive, SVN | Type the URL into the path field | Credentials in the URL |

- **In folder compare**, files that exist only locally open an empty remote side that is created when you save. Sync and folder merge stay local-only and are disabled while a side is remote.
- **Host keys are verified.** The first connection to an SFTP server shows its fingerprint and adds it to `~/.ssh/known_hosts` only after you trust it; a changed key is refused.
- **Secrets stay in memory.** Passwords and key passphrases are never written to disk, sessions or settings. Recent connections are remembered without them.

### Drag-and-drop auto-fill

- **Single item**: Drag a file or directory over either the left or right path capsule to fill that side. When the other side already has a path, comparison begins automatically.
- **Multiple items**: Select two files or directories from your desktop or file manager and drop them onto either path capsule or into the empty workspace card; left and right paths will be populated simultaneously and loaded immediately.
- **Smart mode switching**: Dropping a folder into File Compare automatically switches to Directory Compare. In Text Merge, base and output paths can also be filled via drag-and-drop or browsing.

## Installation

### Homebrew

```bash
# Desktop app (macOS) — also adds the `gocompare` command
brew install --cask jhzhang09/tap/gocompare

# Command line (macOS and Linux) — installs `gocompare-cli`
brew install jhzhang09/tap/gocompare-cli
```

The desktop app is not notarized yet, so macOS blocks its first launch; see [macOS: "GoCompare can't be opened"](#macos-gatekeeper) below.

### Manual download

Download the latest build from the [**Releases**](https://github.com/jhzhang09/gocompare/releases) page.

| Platform | Desktop app | Command line |
|---|---|---|
| macOS 12+ (Apple Silicon & Intel) | `gocompare-<version>.dmg` | `gocompare-cli_<version>_darwin_<arch>.tar.gz` |
| Windows 10 / 11 | `gocompare-setup-<version>-<arch>.exe` or the portable `.zip` | `gocompare-cli_<version>_windows_<arch>.zip` |
| Linux (GTK 3) | `.deb` or `.tar.gz` | `gocompare-cli_<version>_linux_<arch>.tar.gz` |

Every release ships `SHA256SUMS.txt` and a [build provenance attestation](https://docs.github.com/actions/security-for-github-actions/using-artifact-attestations) you can check with `gh attestation verify <file> --repo jhzhang09/gocompare`.

<a id="macos-gatekeeper"></a>
<details>
<summary><b>macOS: "GoCompare can't be opened"</b></summary>

Release builds are ad-hoc signed but not notarized yet. Allow the app once by running:

```bash
xattr -dr com.apple.quarantine /Applications/GoCompare.app
```

or try to open it, then choose **System Settings → Privacy & Security → Open Anyway**. (On macOS 15 and later, right-click → Open no longer bypasses this check.)
</details>

<details>
<summary><b>Windows: SmartScreen warning</b></summary>

The binaries are not code-signed yet. Choose **More info → Run anyway**.
</details>

<details>
<summary><b>Linux</b></summary>

```bash
sudo apt install ./gocompare_<version>_amd64.deb
# or, without installing:
tar xzf gocompare-<version>-linux-amd64.tar.gz && ./gocompare
```

GTK 3 is required; it is present on most desktop distributions.
</details>

### Build from source

Requires Go 1.27.3 or newer. All dependencies are vendored, so no network access is needed.

```bash
git clone https://github.com/jhzhang09/gocompare.git
cd gocompare
go run ./cmd/gocompare                       # run the desktop app
go build -o build/ ./cmd/gocompare-cli        # build the CLI
go tool mygo build                            # package the app for this platform (.app/.dmg on macOS)
```

## Usage

### Desktop app

```bash
gocompare                                    # start with the text compare workspace
gocompare left.txt right.txt                 # compare two files
gocompare --directories ./left ./right       # compare two folders
gocompare --version
```

With the Homebrew cask the `gocompare` command is on your `PATH`; otherwise the executable lives at `/Applications/GoCompare.app/Contents/MacOS/GoCompare`.

### Command line

`gocompare-cli` exits with **0** when inputs are equal (or a merge is clean), **1** when they differ (or conflicts remain) and **2** on errors.

```bash
gocompare-cli left.txt right.txt                                  # compare two files
gocompare-cli dir --filter '*.go' ./v1 ./v2                       # folder report as JSON
gocompare-cli preview mirror-right ./src ./dst                    # sync plan (read-only)
gocompare-cli sync --mode mirror-right --apply --backup-dir ~/gocompare-backup ./src ./dst
gocompare-cli restore ~/gocompare-backup/sync/journal.json        # undo a sync
gocompare-cli merge --base base.txt --output merged.txt left.txt right.txt
gocompare-cli picture before.png after.png
```

Sync modes: `update-right`, `update-left`, `update-both`, `mirror-right`, `mirror-left`.

More commands for remote files, tables, archives and snapshots:

```bash
gocompare-cli remote compare sftp://deploy@web1/etc/app.conf sftp://deploy@web2/etc/app.conf
gocompare-cli remote read "smb://CORP;bob@nas/docs/report.txt" > report.txt
gocompare-cli table --keys 0 --tolerance 0.01 before.csv after.xlsx
gocompare-cli archive compare release-1.zip release-2.zip
gocompare-cli snapshot create ./build build.snapshot        # later: snapshot compare a.snapshot b.snapshot
gocompare-cli dirmerge --base ./base --output ./merged ./left ./right
```

SFTP uses `~/.ssh/known_hosts` (or `--known-hosts`) and never trusts an unknown host silently; pass `--identity` for a key and set `GOCOMPARE_SSH_KEY_PASSPHRASE` if it has a passphrase. Run `gocompare-cli --help` for every command, including `hex`, `media`, `pe`, `reg`, `doctext`, `acl`, `check-update`, `script`, `report` and `extend`.

### Git integration

```bash
git config --global diff.tool gocompare
git config --global difftool.gocompare.cmd 'gocompare "$LOCAL" "$REMOTE"'
git config --global merge.tool gocompare
git config --global mergetool.gocompare.cmd 'gocompare-cli mergetool --base "$BASE" --output "$MERGED" "$LOCAL" "$REMOTE"'
git config --global mergetool.gocompare.trustExitCode true
```

`mergetool` writes the merge result into `$MERGED` and exits with 1 while conflicts remain, so Git keeps the file marked as unresolved until you fix it.

## Safety guarantees

- **Saves are atomic and conflict-aware.** Files are written to a temporary file in the same folder and renamed into place only if the file on disk still matches what was loaded. "Save as" never overwrites an existing file.
- **Folder sync is plan → confirm → execute.** Every action records the size, SHA-256, mode and time it expects. Execution re-checks them, backs up files before overwriting or deleting, journals each step and rolls back completed steps on any error or cancellation.
- **Remote saves are checked too.** Saving a remote file re-reads it first and refuses to overwrite changes someone else made since you opened it. Passwords and key passphrases stay in memory only and are stripped from paths, window titles and saved sessions.
- **Undo never destroys newer work.** Undoing a sync refuses to touch files that changed after the sync, and plans are refused when names collide only by letter case on case-insensitive file systems.

The full contract is in [docs/operations.md](docs/operations.md) (Chinese).

## Documentation

| Document | Contents |
|---|---|
| [Architecture](docs/architecture.md) | Module boundaries and data flow |
| [Feature matrix](docs/feature-parity.md) | What is complete, partial or planned |
| [Platform support](docs/platform-support.md) | Build targets and per-platform status |
| [Operations contract](docs/operations.md) | Save and sync safety guarantees |
| [Testing](docs/testing.md) | Automated and native regression tests |
| [MyGo patch](docs/mygo-patch.md) | How the vendored framework patch is maintained |
| [Releasing](docs/releasing.md) | How maintainers cut a release |

Most design documents are currently written in Chinese.

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) and our [Code of Conduct](CODE_OF_CONDUCT.md). In short:

```bash
gofmt -l cmd internal tools     # must print nothing
go vet ./...
CGO_ENABLED=0 go test ./...
go tool mygo vet .
```

To report a security issue, please follow [SECURITY.md](SECURITY.md) instead of opening a public issue.

## Acknowledgements

- [MyGo](https://github.com/egoist/mygo) by [@egoist](https://github.com/egoist) — the native Go desktop UI framework GoCompare is built on.
- [golang.org/x/text](https://pkg.go.dev/golang.org/x/text), [golang.org/x/image](https://pkg.go.dev/golang.org/x/image), [golang.org/x/net](https://pkg.go.dev/golang.org/x/net) and [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto).
- [pkg/sftp](https://github.com/pkg/sftp) and [go-smb2](https://github.com/hirochachacha/go-smb2) for SFTP and SMB access.

## License

[MIT](LICENSE) © GoCompare Contributors
