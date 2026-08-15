#!/usr/bin/env bash
# Multi-platform Pulse-Vault build (pure-Go CLI + optional host GUI).
#
# From gui-go/:
#   ./scripts/build-multi.sh
#   OUT_DIR=../dist VERSION=1.2.3 ./scripts/build-multi.sh
#
# CLI is always built with CGO_ENABLED=0 (cross-compiles windows/linux/darwin).
# Host GUI is built only when a CGO toolchain is available (not faked for other OS).

set -u

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT_DIR="${OUT_DIR:-./dist}"
VERSION="${VERSION:-dev}"
mkdir -p "$OUT_DIR"
OUT_DIR="$(cd "$OUT_DIR" && pwd)"

LDFLAGS="-s -w -X github.com/Z3r0s/Pulse-Vault/gui-go/internal/version.Version=${VERSION}"
failures=0
built=()

log() {
  local level="${2:-INFO}"
  printf '[%s][%s] %s\n' "$(date +%H:%M:%S)" "$level" "$1"
}

build_cli() {
  local goos="$1" goarch="$2" out_name="$3"
  local out_path="${OUT_DIR}/${out_name}"
  log "CLI ${goos}/${goarch} -> ${out_name} ..."
  if ! CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
      go build -trimpath -buildvcs=false -ldflags "$LDFLAGS" -o "$out_path" ./cmd/pulse-vault; then
    log "FAIL CLI ${goos}/${goarch}" "FAIL"
    failures=$((failures + 1))
    return
  fi
  if [[ ! -f "$out_path" ]]; then
    log "FAIL CLI ${goos}/${goarch} (no output file)" "FAIL"
    failures=$((failures + 1))
    return
  fi
  local len
  len=$(wc -c <"$out_path" | tr -d ' ')
  log "OK   CLI ${goos}/${goarch} (${len} bytes)" "OK"
  built+=("$out_name")
}

log "Pulse-Vault multi build (Version=${VERSION})"
log "OutDir=${OUT_DIR}"
log "Root=${ROOT}"

# --- Pure-Go CLI targets (CGO_ENABLED=0) ---
build_cli windows amd64 pulse-vault-windows-amd64.exe
build_cli linux   amd64 pulse-vault-linux-amd64
build_cli linux   arm64 pulse-vault-linux-arm64
build_cli darwin  amd64 pulse-vault-darwin-amd64
build_cli darwin  arm64 pulse-vault-darwin-arm64

# --- Host GUI (only when CGO is available; never invent foreign-OS GUI binaries) ---
host_os="$(go env GOHOSTOS)"
host_arch="$(go env GOHOSTARCH)"
if [[ "$host_os" == "windows" ]]; then
  gui_name="pulse-vault-gui-${host_os}-${host_arch}.exe"
else
  gui_name="pulse-vault-gui-${host_os}-${host_arch}"
fi
gui_out="${OUT_DIR}/${gui_name}"

cgo_ok=0
if command -v gcc >/dev/null 2>&1 || command -v clang >/dev/null 2>&1 || [[ -n "${CC:-}" ]]; then
  # Probe that CGO can actually link something trivial is expensive; presence of a C compiler is enough gate.
  cgo_ok=1
fi

if [[ "$cgo_ok" -eq 1 ]]; then
  log "GUI host ${host_os}/${host_arch} (CGO=1) -> ${gui_name} ..."
  if CGO_ENABLED=1 go build -trimpath -buildvcs=false -ldflags "-s -w" -o "$gui_out" .; then
    if [[ -f "$gui_out" ]]; then
      len=$(wc -c <"$gui_out" | tr -d ' ')
      log "OK   host GUI (${len} bytes)" "OK"
      built+=("$gui_name")
    else
      log "FAIL host GUI (no output file)" "FAIL"
      failures=$((failures + 1))
    fi
  else
    log "FAIL host GUI (CGO build failed; CLI artifacts kept)" "FAIL"
    failures=$((failures + 1))
  fi
else
  log "SKIP host GUI (CGO toolchain not available on this host)" "SKIP"
fi

# --- SHA256SUMS ---
sums_path="${OUT_DIR}/SHA256SUMS"
: >"$sums_path"
# Sort artifact names for stable checksums file
IFS=$'\n' sorted=($(printf '%s\n' "${built[@]:-}" | LC_ALL=C sort))
unset IFS
for name in "${sorted[@]:-}"; do
  [[ -z "$name" ]] && continue
  path="${OUT_DIR}/${name}"
  [[ -f "$path" ]] || continue
  if command -v sha256sum >/dev/null 2>&1; then
    # sha256sum prints "hash  path"; rewrite to basename only
    hash=$(sha256sum "$path" | awk '{print $1}')
  elif command -v shasum >/dev/null 2>&1; then
    hash=$(shasum -a 256 "$path" | awk '{print $1}')
  else
    hash=$(openssl dgst -sha256 "$path" | awk '{print $NF}')
  fi
  printf '%s  %s\n' "$hash" "$name" >>"$sums_path"
done
count=$(grep -c . "$sums_path" 2>/dev/null || echo 0)
log "Wrote ${sums_path} (${count} entries)"

echo
log "Built ${#built[@]} artifact(s); failures=${failures}"
if [[ "$failures" -gt 0 ]]; then
  log "Completed with failures" "FAIL"
  exit 1
fi
log "All requested targets succeeded" "OK"
exit 0
