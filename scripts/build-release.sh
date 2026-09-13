#!/usr/bin/env bash
set -euo pipefail

version=${1:?Usage: scripts/build-release.sh v0.1.0}
if [[ ! $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo 'Release version must have the form v0.1.0.' >&2
  exit 1
fi
cd "$(dirname "$0")/.."
[[ -f LICENSE ]] || { echo 'Add an open-source LICENSE before building a release.' >&2; exit 1; }
mkdir -p dist
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
assets=()
for os in linux darwin windows; do
  for arch in amd64 arm64; do
    binary=optimus
    [[ $os != windows ]] || binary=optimus.exe
    CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath \
      -ldflags="-s -w -X main.version=$version" -o "$work/$binary" ./cmd/optimus
    cp README.md LICENSE "$work/"
    asset="optimus_${os}_${arch}"
    if [[ $os == windows ]]; then
      asset="$asset.zip"
      # A fresh archive avoids retaining files from a previous build.
      (cd "$work" && zip -q "archive.zip" "$binary" README.md LICENSE)
      mv "$work/archive.zip" "dist/$asset"
    else
      asset="$asset.tar.gz"
      tar -czf "dist/$asset" -C "$work" "$binary" README.md LICENSE
    fi
    assets+=("$asset")
  done
done
cp install.sh install.ps1 dist/
assets+=(install.sh install.ps1)
if command -v sha256sum >/dev/null 2>&1; then
  (cd dist && sha256sum "${assets[@]}") > "$work/checksums.txt"
else
  (cd dist && shasum -a 256 "${assets[@]}") > "$work/checksums.txt"
fi
mv "$work/checksums.txt" dist/checksums.txt
