#!/bin/sh
set -eu

fail() { printf 'optimus installer: %s\n' "$*" >&2; exit 1; }
for tool in curl uname mktemp tar install awk id mkdir; do
  command -v "$tool" >/dev/null 2>&1 || fail "Required command missing: $tool"
done
case $(uname -s) in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) fail 'Use install.ps1 on Windows. Supported systems: Linux and macOS.' ;;
esac
case $(uname -m) in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) fail 'Supported architectures: x86_64 and ARM64.' ;;
esac
if command -v sha256sum >/dev/null 2>&1; then
  hash_tool=sha256sum
elif command -v shasum >/dev/null 2>&1; then
  hash_tool=shasum
else
  fail 'SHA-256 verification requires sha256sum or shasum.'
fi
repo=https://github.com/ahmadmeselmani/optimus
if [ -n "${OPTIMUS_VERSION:-}" ]; then
  printf '%s\n' "$OPTIMUS_VERSION" | awk '/^v[0-9]+\.[0-9]+\.[0-9]+$/ {ok=1} END {exit !ok}' || fail 'OPTIMUS_VERSION must be a release tag such as v0.1.0.'
  base="$repo/releases/download/$OPTIMUS_VERSION"
else
  base="$repo/releases/latest/download"
fi
archive="optimus_${os}_${arch}.tar.gz"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT HUP INT TERM
printf 'Downloading Optimus for %s/%s...\n' "$os" "$arch"
curl -fLsS --retry 3 "$base/$archive" -o "$work/$archive" || fail 'Download failed. Check that a public release exists.'
curl -fLsS --retry 3 "$base/checksums.txt" -o "$work/checksums.txt" || fail 'Could not download release checksums.'
expected=$(awk -v name="$archive" '$2 == name {print $1}' "$work/checksums.txt")
[ "${#expected}" -eq 64 ] || fail 'Missing or invalid archive checksum.'
if [ "$hash_tool" = sha256sum ]; then
  actual=$(sha256sum "$work/$archive" | awk '{print $1}')
else
  actual=$(shasum -a 256 "$work/$archive" | awk '{print $1}')
fi
[ "$expected" = "$actual" ] || fail 'Checksum mismatch; nothing was installed.'
tar -xzf "$work/$archive" -C "$work" optimus
dir=${OPTIMUS_INSTALL_DIR:-/usr/local/bin}
if [ -d "$dir" ] && [ -w "$dir" ]; then
  install -m 755 "$work/optimus" "$dir/optimus"
elif [ "$(id -u)" -eq 0 ]; then
  mkdir -p "$dir"
  install -m 755 "$work/optimus" "$dir/optimus"
else
  command -v sudo >/dev/null 2>&1 || fail "No permission to install in $dir. Set OPTIMUS_INSTALL_DIR to a writable directory on PATH."
  printf 'Installing into %s (sudo may ask for your password)...\n' "$dir"
  sudo mkdir -p "$dir"
  sudo install -m 755 "$work/optimus" "$dir/optimus"
fi
printf '\nOptimus installed at %s/optimus.\n' "$dir"
case :$PATH: in
  *:"$dir":*) printf 'Run: optimus\n' ;;
  *) printf 'Add %s to your PATH, then run: optimus\n' "$dir" ;;
esac
printf 'Ollama must be running with a local model installed. Optimus is free; no account or payment is required.\n'
