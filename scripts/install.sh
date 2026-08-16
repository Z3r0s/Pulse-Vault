#!/bin/sh
# Install Pulse-Vault from GitHub Releases, or build it from this clone.
# DNSPulse — https://dnspulse.org
#
#   curl -fsSL https://raw.githubusercontent.com/Z3r0s/Pulse-Vault/main/scripts/install.sh | sh
#   curl ... | sh -s -- --gui
#
# From a clone (works with no GitHub tag):
#   ./scripts/install.sh --from-source
#   ./scripts/install.sh --gui --from-source
set -eu

REPO="Z3r0s/Pulse-Vault"
SITE="https://dnspulse.org"
RELEASES="https://github.com/$REPO/releases/latest"
PREFIX="${PREFIX:-$HOME/.local}"
BIN_DIR="${BIN_DIR:-$PREFIX/bin}"
WANT_GUI=0
FROM_SOURCE=0

usage() {
  cat <<EOF
Install Pulse-Vault (DNSPulse / $SITE)

Usage: install.sh [--gui] [--from-source] [--help]

  --gui           also install the desktop GUI when a build exists
  --from-source   build the CLI from this git clone (no GitHub tag needed)
  PREFIX          install prefix (default: \$HOME/.local)
  BIN_DIR         binary dir (default: \$PREFIX/bin)

Examples:
  curl -fsSL https://raw.githubusercontent.com/Z3r0s/Pulse-Vault/main/scripts/install.sh | sh
  ./scripts/install.sh --from-source
EOF
}

for arg in "$@"; do
  case "$arg" in
    --gui) WANT_GUI=1 ;;
    --from-source) FROM_SOURCE=1 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $arg" >&2; usage >&2; exit 2 ;;
  esac
done

file_sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    echo "need sha256sum or shasum" >&2
    exit 1
  fi
}

detect_asset() {
  os=$(uname -s | tr 'A-Z' 'a-z')
  arch=$(uname -m)
  case "$arch" in
    x86_64|amd64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) echo "unsupported CPU: $arch" >&2; exit 1 ;;
  esac
  case "$os" in
    linux) echo "pulse-vault-linux-$arch" ;;
    darwin) echo "pulse-vault-darwin-$arch" ;;
    mingw*|msys*|cygwin*) echo "use scripts/install.ps1 on Windows" >&2; exit 1 ;;
    *) echo "unsupported OS: $os" >&2; exit 1 ;;
  esac
}

repo_root() {
  script=$0
  case "$script" in
    /*) dir=$(dirname "$script") ;;
    *) dir=$(dirname "$PWD/$script") ;;
  esac
  root=$(cd "$dir/.." 2>/dev/null && pwd)
  if [ -f "$root/gui-go/cmd/pulse-vault/main.go" ]; then
    echo "$root"
  fi
}

download_release() {
  asset=$1
  dest=$2
  base="https://github.com/$REPO/releases/latest/download"
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT
  echo "  downloading $asset"
  if ! curl -fsSL "$base/SHA256SUMS" -o "$tmp/SHA256SUMS"; then
    echo "download failed — has a v* tag been published?" >&2
    echo "  $RELEASES" >&2
    return 1
  fi
  if ! curl -fsSL "$base/$asset" -o "$tmp/$asset"; then
    echo "missing asset $asset" >&2
    return 1
  fi
  want=$(awk -v n="$asset" '$NF==n || $NF=="*"n {print $1; exit}' "$tmp/SHA256SUMS")
  if [ -z "$want" ]; then
    echo "SHA256SUMS has no $asset" >&2
    return 1
  fi
  got=$(file_sha256 "$tmp/$asset")
  if [ "$got" != "$want" ]; then
    echo "SHA-256 mismatch for $asset" >&2
    return 1
  fi
  mkdir -p "$(dirname "$dest")"
  if command -v install >/dev/null 2>&1; then
    install -m 0755 "$tmp/$asset" "$dest"
  else
    cp "$tmp/$asset" "$dest"
    chmod 0755 "$dest"
  fi
  rm -rf "$tmp"
  trap - EXIT
}

build_cli() {
  root=$1
  dest=$2
  if ! command -v go >/dev/null 2>&1; then
    echo "Go is not on PATH. Install https://go.dev/dl/ or wait for a GitHub Release." >&2
    exit 1
  fi
  echo "  building CLI"
  mkdir -p "$(dirname "$dest")"
  (cd "$root/gui-go" && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$dest" ./cmd/pulse-vault)
}

build_gui() {
  root=$1
  dest=$2
  if ! command -v go >/dev/null 2>&1; then
    echo "Go is not on PATH." >&2
    exit 1
  fi
  echo "  building GUI from source (needs CGO + OpenGL headers)"
  mkdir -p "$(dirname "$dest")"
  (cd "$root/gui-go" && CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o "$dest" .)
}

echo "Pulse-Vault  ·  DNSPulse  ·  $SITE"
echo "Install dir: $BIN_DIR"
mkdir -p "$BIN_DIR"

ASSET=$(detect_asset)
CLI="$BIN_DIR/pulse-vault"
GUI="$BIN_DIR/pulse-vault-gui"
ROOT=$(repo_root || true)
USED_SOURCE=0

if [ "$FROM_SOURCE" -eq 1 ]; then
  if [ -z "$ROOT" ]; then
    echo "--from-source needs a git clone. Run ./scripts/install.sh from the repo." >&2
    exit 1
  fi
  build_cli "$ROOT" "$CLI"
  USED_SOURCE=1
elif download_release "$ASSET" "$CLI"; then
  :
elif [ -n "$ROOT" ] && command -v go >/dev/null 2>&1; then
  echo "Release download failed; building CLI from this clone instead."
  build_cli "$ROOT" "$CLI"
  USED_SOURCE=1
else
  echo ""
  echo "Could not download a release. Either:"
  echo "  1. Open $RELEASES and grab $ASSET"
  echo "  2. Clone the repo and run:  ./scripts/install.sh --from-source"
  echo "  3. Wait until a v* tag is published."
  exit 1
fi

if [ "$WANT_GUI" -eq 1 ]; then
  os=$(uname -s | tr 'A-Z' 'a-z')
  arch=$(uname -m)
  case "$arch" in
    x86_64|amd64) garch=amd64 ;;
    aarch64|arm64) garch=arm64 ;;
    *) garch=amd64 ;;
  esac
  if [ "$USED_SOURCE" -eq 1 ] && [ -n "$ROOT" ]; then
    build_gui "$ROOT" "$GUI"
  elif download_release "pulse-vault-gui-${os}-${garch}" "$GUI"; then
    :
  elif [ -n "$ROOT" ]; then
    echo "GUI release missing; trying a local build."
    build_gui "$ROOT" "$GUI"
  else
    echo "No GUI asset for ${os}/${garch}. Use the CLI or build from source." >&2
    exit 1
  fi
  echo "GUI: $GUI"
fi

case ":$PATH:" in
  *:"$BIN_DIR":*) ;;
  *)
    echo ""
    echo "Add $BIN_DIR to PATH for this shell:"
    echo "  export PATH=\"$BIN_DIR:\$PATH\""
    echo "On Linux, ~/.local/bin is usually picked up on the next login."
    ;;
esac

echo ""
echo "CLI:  $CLI"
"$CLI" version 2>/dev/null || true
echo "Try:  pulse-vault version"
echo "Site: $SITE"
