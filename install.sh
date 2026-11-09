#!/bin/sh
# Installs the latest switcheroo release. Set SWITCHEROO_INSTALL_DIR to change where.
set -eu

repo=kilupskalvis/switcheroo
dir=${SWITCHEROO_INSTALL_DIR:-$HOME/.local/bin}

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case $os in
  linux | darwin) ;;
  *) echo "switcheroo: unsupported OS: $os" >&2; exit 1 ;;
esac
case $(uname -m) in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) echo "switcheroo: unsupported CPU: $(uname -m)" >&2; exit 1 ;;
esac

asset=switcheroo_${os}_${arch}.tar.gz
url=https://github.com/$repo/releases/latest/download
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

curl -fsSL -o "$tmp/$asset" "$url/$asset"
curl -fsSL -o "$tmp/checksums.txt" "$url/checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then sum="sha256sum"; else sum="shasum -a 256"; fi
if ! (cd "$tmp" && grep " $asset\$" checksums.txt | $sum -c - >/dev/null); then
  echo "switcheroo: checksum mismatch for $asset" >&2
  exit 1
fi

tar -xzf "$tmp/$asset" -C "$tmp" switcheroo
mkdir -p "$dir"
install -m 0755 "$tmp/switcheroo" "$dir/switcheroo"
echo "Installed switcheroo to $dir/switcheroo"
case ":$PATH:" in
  *":$dir:"*) ;;
  *) echo "Add $dir to your PATH to use it." ;;
esac
