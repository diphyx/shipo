#!/usr/bin/env bash
# Builds the release archives into dist/: static `shipo` binaries for macOS, Linux, and Windows,
# on amd64 and arm64, each with the README and LICENSE; a .zip for Windows, a .tar.gz for the
# rest, and checksums.txt over them all.
#
#   .github/scripts/package.sh v1.2.3
set -euo pipefail

tag="${1:?usage: package.sh <tag>}"
version="${tag#v}"
targets=(darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64)

checksum() {
    if command -v sha256sum >/dev/null; then
        sha256sum -- "$@"
    else
        shasum -a 256 -- "$@"
    fi
}

rm -rf dist
mkdir -p dist
for target in "${targets[@]}"; do
    os="${target%/*}"
    arch="${target#*/}"
    name="shipo_${version}_${os}_${arch}"
    binary="shipo"
    if [[ "$os" == "windows" ]]; then
        binary="shipo.exe"
    fi
    mkdir -p "dist/$name"
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
        -ldflags "-s -w -X main.version=$version" -o "dist/$name/$binary" .
    cp README.md LICENSE "dist/$name/"
    if [[ "$os" == "windows" ]]; then
        (cd dist && zip -qr "$name.zip" "$name")
    else
        tar -C dist -czf "dist/$name.tar.gz" "$name"
    fi
    rm -rf "dist/$name"
done
(cd dist && checksum *.tar.gz *.zip > checksums.txt)
