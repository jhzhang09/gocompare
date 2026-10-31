# Security Policy

## Supported versions

GoCompare is pre-1.0. Security fixes are released for the **latest release** only; please upgrade before reporting.

## Reporting a vulnerability

**Please do not report security vulnerabilities through public issues.**

Report them privately with [GitHub's private vulnerability reporting](https://github.com/jhzhang09/gocompare/security/advisories/new). Include:

- the GoCompare version and operating system,
- a description of the issue and its impact,
- steps or files that reproduce it.

You should receive an acknowledgement within 7 days. We will keep you informed while a fix is prepared and credit you in the release notes unless you prefer otherwise.

## Scope

Issues of particular interest:

- data loss or unintended overwrites when saving, syncing, merging or undoing,
- path traversal or symlink escapes during folder comparison and sync,
- crashes or excessive memory use triggered by crafted files (images, ID3 tags, PE files, `.reg` files, archives),
- command execution through the extension (`extend`) interface beyond what a spec explicitly declares.

## 安全问题报告

请勿通过公开 Issue 报告安全漏洞，请使用上方的 GitHub 私密漏洞报告渠道，我们会在 7 天内回复。
