#!/usr/bin/env bash
# 根据构建输出的 SHA256SUMS.txt，自动更新 jhzhang09/homebrew-tap 仓库中的 Cask 与 Formula。
# 用法:
#   scripts/update-homebrew-tap.sh <version> <checksums_file>
# 环境变量:
#   TAP_GITHUB_TOKEN: 用于向 jhzhang09/homebrew-tap 推送提交的 GitHub Token（CI 环境使用）
#   TAP_REPO: 目标 Tap 仓库，默认 jhzhang09/homebrew-tap
set -euo pipefail

version_raw="${1:?usage: update-homebrew-tap.sh <version> <checksums_file>}"
checksums="${2:?usage: update-homebrew-tap.sh <version> <checksums_file>}"
version="${version_raw#v}"
tap_repo="${TAP_REPO:-jhzhang09/homebrew-tap}"

if [[ ! -f "$checksums" ]]; then
  echo "Error: checksums file not found at $checksums" >&2
  exit 1
fi

extract_sha() {
  local pattern="$1"
  local sha
  sha=$(grep -E "$pattern" "$checksums" | head -n 1 | awk '{print $1}')
  if [[ -z "$sha" ]]; then
    echo "Error: could not find hash for $pattern in $checksums" >&2
    exit 1
  fi
  echo "$sha"
}

echo "Extracting SHA256 checksums for version $version..."
dmg_sha=$(extract_sha "gocompare-[0-9].*\\.dmg$")
darwin_arm64_sha=$(extract_sha "gocompare-cli_.*_darwin_arm64\\.tar\\.gz$")
darwin_amd64_sha=$(extract_sha "gocompare-cli_.*_darwin_amd64\\.tar\\.gz$")
linux_arm64_sha=$(extract_sha "gocompare-cli_.*_linux_arm64\\.tar\\.gz$")
linux_amd64_sha=$(extract_sha "gocompare-cli_.*_linux_amd64\\.tar\\.gz$")

echo "  DMG:          $dmg_sha"
echo "  macOS arm64:  $darwin_arm64_sha"
echo "  macOS amd64:  $darwin_amd64_sha"
echo "  Linux arm64:  $linux_arm64_sha"
echo "  Linux amd64:  $linux_amd64_sha"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

echo "Cloning $tap_repo..."
if [[ -n "${TAP_GITHUB_TOKEN:-}" ]]; then
  git clone "https://x-access-token:${TAP_GITHUB_TOKEN}@github.com/${tap_repo}.git" "$work/tap"
elif [[ -d "/opt/homebrew/Library/Taps/${tap_repo}" ]]; then
  # 本地环境如果已有该 tap，优先使用
  git clone "/opt/homebrew/Library/Taps/${tap_repo}" "$work/tap"
else
  git clone "git@github.com:${tap_repo}.git" "$work/tap"
fi

cd "$work/tap"
git config user.name "github-actions[bot]"
git config user.email "github-actions[bot]@users.noreply.github.com"

mkdir -p Casks Formula

cat << EOF > Casks/gocompare.rb
cask "gocompare" do
  version "${version}"
  sha256 "${dmg_sha}"

  url "https://github.com/jhzhang09/gocompare/releases/download/v#{version}/gocompare-#{version}.dmg"
  name "GoCompare"
  desc "Native, cross-platform file and folder comparison tool"
  homepage "https://github.com/jhzhang09/gocompare"

  app "GoCompare.app"

  zap trash: [
    "~/Library/Application Support/GoCompare",
    "~/Library/Preferences/dev.gocompare.desktop.plist",
    "~/Library/Saved Application State/dev.gocompare.desktop.savedState",
  ]

  caveats <<~EOS
    GoCompare is not notarized with an Apple Developer account.
    To avoid macOS Gatekeeper blocking the app, install with --no-quarantine:
      brew install --cask --no-quarantine jhzhang09/tap/gocompare

    If already installed and macOS blocks it, run:
      xattr -dr com.apple.quarantine "#{appdir}/GoCompare.app"
    Or right-click the app in Finder and choose "Open".
  EOS
end
EOF

cat << EOF > Formula/gocompare-cli.rb
class GocompareCli < Formula
  desc "Native file and folder comparison CLI"
  homepage "https://github.com/jhzhang09/gocompare"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/jhzhang09/gocompare/releases/download/v${version}/gocompare-cli_${version}_darwin_arm64.tar.gz"
      sha256 "${darwin_arm64_sha}"
    else
      url "https://github.com/jhzhang09/gocompare/releases/download/v${version}/gocompare-cli_${version}_darwin_amd64.tar.gz"
      sha256 "${darwin_amd64_sha}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/jhzhang09/gocompare/releases/download/v${version}/gocompare-cli_${version}_linux_arm64.tar.gz"
      sha256 "${linux_arm64_sha}"
    else
      url "https://github.com/jhzhang09/gocompare/releases/download/v${version}/gocompare-cli_${version}_linux_amd64.tar.gz"
      sha256 "${linux_amd64_sha}"
    end
  end

  def install
    bin.install "gocompare-cli" => "gocompare"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/gocompare --version")
  end
end
EOF

if git diff --quiet; then
  echo "Tap repository is already up to date with version $version."
  exit 0
fi

git add Casks/gocompare.rb Formula/gocompare-cli.rb
git commit -m "chore(release): bump GoCompare to v${version}"

if [[ -n "${TAP_GITHUB_TOKEN:-}" ]]; then
  git push "https://x-access-token:${TAP_GITHUB_TOKEN}@github.com/${tap_repo}.git" HEAD:main
else
  git push origin HEAD:main
fi

echo "Successfully updated $tap_repo to v${version}!"
