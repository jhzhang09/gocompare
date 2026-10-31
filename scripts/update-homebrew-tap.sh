#!/usr/bin/env bash
# 根据发版产物的 SHA256SUMS.txt 生成 Homebrew Cask 与 Formula，并提交到 tap 仓库。
# 用法:
#   scripts/update-homebrew-tap.sh <version> <checksums_file>
# 环境变量:
#   TAP_GITHUB_TOKEN  推送 tap 仓库用的令牌（CI 使用）；本地不设置时通过 SSH 克隆和推送
#   TAP_REPO          目标 tap 仓库，默认 jhzhang09/homebrew-tap
#   TAP_DRY_RUN       设为任意值时只把生成的文件写到 TAP_OUT（默认 ./tap-preview），不克隆、不推送
set -euo pipefail

version_raw="${1:?usage: update-homebrew-tap.sh <version> <checksums_file>}"
checksums="${2:?usage: update-homebrew-tap.sh <version> <checksums_file>}"
version="${version_raw#v}"
tap_repo="${TAP_REPO:-jhzhang09/homebrew-tap}"
release_url="https://github.com/jhzhang09/gocompare/releases/download/v${version}"

if [[ ! -f "$checksums" ]]; then
  echo "Error: checksums file not found at $checksums" >&2
  exit 1
fi

# extract_sha 按完整文件名取哈希，确保与本次版本的产物对应。
extract_sha() {
  local file="$1" sha
  sha=$(awk -v f="$file" '$2 == f || $2 == "*" f {print $1; exit}' "$checksums")
  if [[ ! "$sha" =~ ^[0-9a-f]{64}$ ]]; then
    echo "Error: no SHA-256 for $file in $checksums" >&2
    exit 1
  fi
  echo "$sha"
}

dmg_sha=$(extract_sha "gocompare-${version}.dmg")
darwin_arm64_sha=$(extract_sha "gocompare-cli_${version}_darwin_arm64.tar.gz")
darwin_amd64_sha=$(extract_sha "gocompare-cli_${version}_darwin_amd64.tar.gz")
linux_arm64_sha=$(extract_sha "gocompare-cli_${version}_linux_arm64.tar.gz")
linux_amd64_sha=$(extract_sha "gocompare-cli_${version}_linux_amd64.tar.gz")

# generate 把 Cask 和 Formula 写到目录 $1。
generate() {
  local dir="$1"
  mkdir -p "$dir/Casks" "$dir/Formula"

  # Cask 中的 #{...} 是 Ruby 插值，由 Homebrew 求值；${...} 由本脚本替换。
  cat <<EOF >"$dir/Casks/gocompare.rb"
cask "gocompare" do
  version "${version}"
  sha256 "${dmg_sha}"

  url "https://github.com/jhzhang09/gocompare/releases/download/v#{version}/gocompare-#{version}.dmg"
  name "GoCompare"
  desc "Native file and folder comparison tool"
  homepage "https://github.com/jhzhang09/gocompare"

  livecheck do
    url :url
    strategy :github_latest
  end

  depends_on macos: :monterey

  app "GoCompare.app"
  # 与 Linux 安装包和 README 一致：命令 gocompare 打开桌面应用，命令行工具是 gocompare-cli。
  binary "#{appdir}/GoCompare.app/Contents/MacOS/GoCompare", target: "gocompare"

  zap trash: [
    "~/.config/gocompare",
    "~/Library/Application Support/GoCompare",
    "~/Library/Preferences/dev.gocompare.desktop.plist",
    "~/Library/Saved Application State/dev.gocompare.desktop.savedState",
  ]

  caveats <<~EOS
    GoCompare is ad-hoc signed but not notarized, so macOS blocks the first launch.
    Allow it once with either:
      xattr -dr com.apple.quarantine "#{appdir}/GoCompare.app"
    or open it, then choose System Settings > Privacy & Security > Open Anyway.
  EOS
end
EOF

  cat <<EOF >"$dir/Formula/gocompare-cli.rb"
class GocompareCli < Formula
  desc "Command-line file and folder comparison, merge and sync"
  homepage "https://github.com/jhzhang09/gocompare"
  license "MIT"

  livecheck do
    url :stable
    strategy :github_latest
  end

  on_macos do
    if Hardware::CPU.arm?
      url "${release_url}/gocompare-cli_${version}_darwin_arm64.tar.gz"
      sha256 "${darwin_arm64_sha}"
    else
      url "${release_url}/gocompare-cli_${version}_darwin_amd64.tar.gz"
      sha256 "${darwin_amd64_sha}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "${release_url}/gocompare-cli_${version}_linux_arm64.tar.gz"
      sha256 "${linux_arm64_sha}"
    else
      url "${release_url}/gocompare-cli_${version}_linux_amd64.tar.gz"
      sha256 "${linux_amd64_sha}"
    end
  end

  def install
    bin.install "gocompare-cli"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/gocompare-cli --version")
    (testpath/"a.txt").write "same\\n"
    (testpath/"b.txt").write "same\\n"
    system bin/"gocompare-cli", testpath/"a.txt", testpath/"b.txt"
  end
end
EOF

  # 有 Ruby 时先做语法检查，避免把坏文件推到 tap。
  if command -v ruby >/dev/null 2>&1; then
    ruby -c "$dir/Casks/gocompare.rb" >/dev/null
    ruby -c "$dir/Formula/gocompare-cli.rb" >/dev/null
  fi
}

if [[ -n "${TAP_DRY_RUN:-}" ]]; then
  out="${TAP_OUT:-tap-preview}"
  generate "$out"
  echo "Generated Homebrew files for v${version} in $out (dry run, nothing pushed)."
  exit 0
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

if [[ -n "${TAP_GITHUB_TOKEN:-}" ]]; then
  remote="https://x-access-token:${TAP_GITHUB_TOKEN}@github.com/${tap_repo}.git"
else
  remote="git@github.com:${tap_repo}.git"
fi
git clone --quiet --depth 1 "$remote" "$work/tap"
cd "$work/tap"
# CI 中以 github-actions[bot] 署名；本地运行沿用维护者自己的 Git 身份。
if [[ -n "${TAP_GITHUB_TOKEN:-}" ]]; then
  git config user.name "github-actions[bot]"
  git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
fi

generate .
git add Casks/gocompare.rb Formula/gocompare-cli.rb
# 包含新建文件在内比较暂存区，tap 中还没有这些文件时也能正确提交。
if git diff --cached --quiet; then
  echo "$tap_repo is already up to date with v${version}."
  exit 0
fi

git commit --quiet -m "gocompare ${version}"
git push --quiet "$remote" HEAD:main
echo "Updated $tap_repo to v${version}."
