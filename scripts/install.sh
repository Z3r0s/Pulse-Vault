#!/bin/sh
# Install the official Pulse-Vault Go CLI from GitHub Releases.
# DNSPulse — https://dnspulse.org
set -eu

REPO="Z3r0s/Pulse-Vault"
PREFIX="${PREFIX:-$HOME/.local}"
BIN_DIR="${BIN_DIR:-$PREFIX/bin}"

os=$(uname -s | tr 'A-Z' 'a-z')
arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo "unsupported CPU: $arch" >&2; exit 1 ;;
esac
case "$os" in
  linux) asset="pulse-vault-linux-$arch" ;;
  darwin) asset="pulse-vault-darwin-$arch" ;;
  *) echo "use scripts/install.ps1 on Windows" >&2; exit 1 ;;
esac

base="https://github.com/$REPO/releases/latest/download"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Pulse-Vault (DNSPulse / dnspulse.org)"
echo "Fetching $asset ..."
curl -fsSL "$base/SHA256SUMS" -o "$tmp/SHA256SUMS"
curl -fsSL "$base/$asset" -o "$tmp/$asset"

want=$(awk -v n="$asset" '$NF==n || $NF=="*"n {print $1; exit}' "$tmp/SHA256SUMS")
if [ -z "$want" ]; then
  echo "SHA256SUMS has no $asset — has a v* tag been published?" >&2
  exit 1
fi
got=$(sha256sum "$tmp/$asset" | awk '{print $1}')
if [ "$got" != "$want" ]; then
  echo "SHA-256 mismatch: got $got want $want" >&2
  exit 1
fi

mkdir -p "$BIN_DIR"
install -m 0755 "$tmp/$asset" "$BIN_DIR/pulse-vault"
echo "Installed $BIN_DIR/pulse-vault"
"$BIN_DIR/pulse-vault" version || true
echo "Add $BIN_DIR to PATH if needed. Site: https://dnspulse.org"
