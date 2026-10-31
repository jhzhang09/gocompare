#!/usr/bin/env bash
# 为所有发布平台交叉编译 gocompare-cli，并打成 tar.gz（Windows 为 zip）。
# 用法：scripts/package-cli.sh <version> [outdir]，version 形如 v1.2.3。
set -euo pipefail

version="${1:?usage: package-cli.sh <version> [outdir]}"
out="${2:-dist}"
targets=(darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64)

root="$(cd "$(dirname "$0")/.." && pwd)"
mkdir -p "$out"
out="$(cd "$out" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

for target in "${targets[@]}"; do
	os="${target%/*}"
	arch="${target#*/}"
	name="gocompare-cli_${version#v}_${os}_${arch}"
	dir="$work/$name"
	mkdir -p "$dir"
	bin="gocompare-cli"
	[ "$os" = windows ] && bin="gocompare-cli.exe"
	(cd "$root" && CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
		-ldflags "-s -w -X gocompare/internal/brand.Version=$version" \
		-o "$dir/$bin" ./cmd/gocompare-cli)
	cp "$root/LICENSE" "$root/README.md" "$root/README.zh-CN.md" "$dir/"
	if [ "$os" = windows ]; then
		(cd "$work" && zip -qr "$out/$name.zip" "$name")
	else
		tar -C "$work" -czf "$out/$name.tar.gz" "$name"
	fi
	echo "packaged $name"
done
