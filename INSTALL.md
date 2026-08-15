# Installation

Pulse-Vault is Go. GUI + CLI are in [`gui-go/`](gui-go/). You don't need Python. Why: [docs/WHY_GO.md](docs/WHY_GO.md).

Packaged downloads (when published) appear on [GitHub Releases](https://github.com/Z3r0s/Pulse-Vault/releases). See [docs/DOWNLOADS.md](docs/DOWNLOADS.md). Official site: [dnspulse.org](https://dnspulse.org).

## Requirements

| Component | Needs |
| --- | --- |
| **CLI** (`cmd/pulse-vault`) | Go 1.22+, **no CGO** — cross-compiles cleanly |
| **Desktop GUI** (`gui-go` main package) | Go 1.22+, **CGO** + OpenGL toolchain (Fyne) |

### Windows (GUI)

- [Go](https://go.dev/dl/)
- [MSYS2](https://www.msys2.org/) with `mingw-w64-x86_64-gcc` on `PATH`, **or** use `gui-go/build.ps1` (prepends `C:\msys64\mingw64\bin` when present)

### Linux (GUI)

```bash
# Debian/Ubuntu example — exact package names vary by distro
sudo apt update
sudo apt install golang-go gcc libgl1-mesa-dev xorg-dev
```

### macOS (GUI)

```bash
# Xcode command-line tools for CGO
xcode-select --install
```

## Build and run — desktop GUI (primary)

```powershell
# Windows
cd gui-go
.\build.ps1          # -H windowsgui: no console when you double-click the exe
.\pulse-vault-gui.exe
.\pulse-vault-gui.exe --version
```

```bash
# Linux / macOS
cd gui-go
CGO_ENABLED=1 go build -o pulse-vault-gui .
./pulse-vault-gui
./pulse-vault-gui --version
```

`--version` prints **Pulse-Vault** and identifies the Go native desktop client.

## Build and run — CLI

```bash
cd gui-go
go build -o pulse-vault ./cmd/pulse-vault

./pulse-vault version
./pulse-vault create my.pulsevault --password 'choose-a-strong-password'
./pulse-vault open my.pulsevault --password 'choose-a-strong-password'
./pulse-vault list my.pulsevault --password 'choose-a-strong-password'
./pulse-vault add my.pulsevault --password 'choose-a-strong-password' ./secret.txt
./pulse-vault extract my.pulsevault --password 'choose-a-strong-password' secret.txt ./out
./pulse-vault delete my.pulsevault --password 'choose-a-strong-password' secret.txt
./pulse-vault info my.pulsevault
./pulse-vault create hidden.png --carrier cover.png --password 'choose-a-strong-password'
```

Cross-compile example (CLI only, no CGO):

```bash
cd gui-go
GOOS=linux GOARCH=amd64 go build -o pulse-vault-linux-amd64 ./cmd/pulse-vault
GOOS=windows GOARCH=amd64 go build -o pulse-vault-windows-amd64.exe ./cmd/pulse-vault
GOOS=darwin GOARCH=arm64 go build -o pulse-vault-darwin-arm64 ./cmd/pulse-vault
```

## Tests

```bash
cd gui-go
# Crypto / vault / UI / CLI (no display required for these packages):
go test ./internal/crypto ./internal/vault ./internal/ui ./cmd/pulse-vault ./crypto -count=1

# Full module (GUI package needs CGO to compile):
CGO_ENABLED=1 go test ./... -count=1
```

## Vault file type

```text
.pulsevault
```

Format details: [docs/VAULT_FORMAT.md](docs/VAULT_FORMAT.md). Marker: `PULSEVAULT5_COMPRESSED_CASCADE`.

Linux desktop metadata (MIME, `.desktop`, AppStream) still lives under `packaging/linux/` for optional integration; primary install remains the Go binary.

## Releases and verification

When GitHub Releases attach builds:

1. Prefer checksums (`SHA256SUMS.txt`) from the same release.
2. Or build from tagged source with the steps above.

No Python needed.

Old Python code: [legacy/python/README.md](legacy/python/README.md). Don't install it.

## Multi-OS release builds

From the repo root (Windows):

```powershell
powershell -File gui-go/scripts/build-multi.ps1 -OutDir dist
```

Produces pure-Go CLI binaries for windows/linux/darwin (amd64+arm64), host GUI when CGO is available, and SHA256SUMS. CI: .github/workflows/release-go.yml.
