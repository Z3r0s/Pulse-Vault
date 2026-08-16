#!/bin/sh
# Pulse-Vault CLI wrapper — DNSPulse https://dnspulse.org
# Build (if needed) and run the Pulse-Vault CLI.
# You do not set CGO_ENABLED. The CLI is plain Go.
#
#   ./cli.sh
#   ./cli.sh version
#   ./cli.sh create ./demo.pulsevault --password 'choose-a-strong-password'
#   ./cli.sh --build
set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
OUT="$ROOT/pulse-vault"
PKG="$ROOT/gui-go/cmd/pulse-vault"
FORCE=0

if [ ! -d "$PKG" ]; then
  echo "This script lives in the Pulse-Vault repo root." >&2
  exit 2
fi

if [ "${1:-}" = "--build" ] || [ "${1:-}" = "-build" ]; then
  FORCE=1
  shift
fi

build_cli() {
  if ! command -v go >/dev/null 2>&1; then
    echo "Install Go from https://go.dev/dl/  (no gcc needed for the CLI)" >&2
    exit 2
  fi
  echo "Building pulse-vault ..."
  (cd "$ROOT" && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$OUT" ./gui-go/cmd/pulse-vault)
}

if [ "$FORCE" -eq 1 ] || [ ! -x "$OUT" ]; then
  build_cli
fi

if [ "$FORCE" -eq 1 ] && [ "$#" -eq 0 ]; then
  echo "OK $OUT"
  exit 0
fi

exec "$OUT" "$@"
