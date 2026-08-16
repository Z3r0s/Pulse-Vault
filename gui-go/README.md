# Pulse-Vault desktop GUI (Go)

Desktop app for V6 vaults from [DNSPulse](https://dnspulse.org), with V5 read compatibility. One Go binary. Version gets stamped from the git tag on release. `pip install pulse-vault` is a launcher for this CLI, not the retired Python tree.

## Features

- Create / open / lock vaults
- Hide in picture: write the vault after a cover image; open the picture file to unlock
- List encrypted contents
- Add files, extract files
- CLI: pulse-cyan command cards, inventory table, `open` interactive console, `delete`
- Visual shell: wrapping header, brand mark, nav rail, card stage, lock/accent pulse, hero empty state. Small windows scroll instead of clipping.
- Long ops run off the UI thread
- Current V6 format (Scrypt, ChaCha + AES, finalized streams, ZIP_STORED); V5 remains readable

## Install (users)

See **[INSTALL.md](../INSTALL.md)**. Windows: grab `pulse-vault-gui-windows-amd64.exe` from Releases, or run `..\scripts\install.cmd` from a clone.

## Build

Requirements:

- Go 1.25+
- CGO enabled (Fyne uses OpenGL)
- On Windows: [MSYS2](https://www.msys2.org/) `mingw-w64-x86_64-gcc` on `PATH` (or use `build.ps1`, which prepends `C:\msys64\mingw64\bin` when present)
- Optional: `goversioninfo` (auto-installed by `build.ps1`) for PE ProductName + icon embed

```powershell
cd gui-go
.\build.ps1 -Version 1.2.3
# Stamps version via ldflags, embeds Windows version resource + icon when tools allow.
# -H windowsgui: no console/PowerShell window when you double-click the exe.
.\pulse-vault-gui.exe --version
```

## Test

```powershell
cd gui-go
# CI suite — no gcc:
go test ./internal/crypto ./internal/vault ./internal/ui ./cmd/pulse-vault ./crypto -count=1
# or: .\test.ps1
# Don't: $env:CGO_ENABLED=1; go test ./...
# that compiles the Fyne GUI and needs gcc. Use .\build.ps1 for the window.
```

## Public crypto package

Applications that need the Pulse-Vault protocol can import the reusable
package directly:

```bash
go get github.com/Z3r0s/Pulse-Vault/gui-go/crypto
```

Use this for the same wire format. Don't swap ciphers without a new format marker and vectors.

## Layout

| Path | Role |
| --- | --- |
| `main.go` | Binary entry (Fyne app + `--version`) |
| `crypto` | Public Pulse-Vault protocol/crypto API for Go consumers |
| `internal/crypto` | Scrypt KDF, cascade AEAD, V5/V6 streams |
| `internal/vault` | ZIP container create/open/add/extract/lock |
| `internal/ui` | Multi-pane Fyne shell + dark theme |
| `cmd/interopcreate` | Dev helper to mint a vault for cross-language checks |

Python leftover is in [`legacy/python/`](../legacy/python/). This folder is the actual app.
