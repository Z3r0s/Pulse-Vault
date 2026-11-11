# Go application

This directory contains Pulse-Vault's desktop app, CLI, vault implementation, and public Go crypto package. The current application is all Go. The retired Python application is archived at [`legacy/python/`](../legacy/python/).

For everyday use, start with the [desktop walkthrough](../docs/TWO_MINUTE_GUIDE.md) or [CLI guide](../docs/CLI_GUIDE.md). This page covers building and working on the Go source.

## Features

- Create, open, lock, and inspect vaults.
- Add one file, several files, or a folder tree while preserving relative paths.
- Extract one file or restore all files, with path validation and overwrite checks.
- Verify encrypted streams and stored file digests.
- Change a password by verifying and re-encrypting each file stream.
- Choose Standard or Hardened password derivation when creating a vault; raise an existing vault's KDF cost during password rotation.
- Append a vault to supported image/video media. This preserves the media prefix but is not steganography.
- Cancelable long-running GUI work, with progress and a temporary-space estimate before add or password rotation.

The desktop window opens at 1240×800 and keeps a minimum size of about 640×480 so the navigation, file search, and workspace remain usable together. The layout tightens its header and collapses secondary context when the window is short. On launch, the app offers an optional GitHub release check; it makes no request until confirmed.

The GUI explains that a copied vault allows offline password guesses. Standard key derivation uses 32 MiB; Hardened uses 1 GiB and takes more memory and time. A long, unique passphrase remains the main defense. Password-policy warnings require an explicit choice before a new vault or rotated password is accepted.

Desktop builds with Fyne system-tray support add **Show Pulse-Vault** and **Exit Pulse-Vault** actions to the notification-area menu. Closing the window locks the vault and hides the app in the tray; an active operation must finish before the window can be hidden or the app exited.

New vaults use the V6 authenticated stream format. Folder trees use the V7 container marker and V6 streams; V5 streams remain readable. See [the format guide](../docs/VAULT_FORMAT.md).

## Build

Requirements:

- Go 1.26 or later; the module recommends Go 1.27.3 and CI uses it. With Go's automatic toolchain selection enabled, an older installation downloads the recommended toolchain when needed.
- The CLI builds without CGO.
- The desktop app uses Fyne and needs CGO plus a C compiler. On Windows, install MSYS2 `mingw-w64-x86_64-gcc`; `build.ps1` can use `C:\msys64\mingw64\bin` when present.

From the repository root, run `.\cli.ps1 version` on Windows or `./cli.sh version` on Linux/macOS to build and run the CLI. To build the desktop app on Windows:

```powershell
cd gui-go
.\build.ps1 -Version 1.2.3
```

The release workflow stamps the version from the tag. See [INSTALL.md](../INSTALL.md) for release and source installs.

## Tests

Run the non-GUI suite from the repository root:

```powershell
go test ./gui-go/internal/crypto ./gui-go/internal/vault ./gui-go/internal/ui ./gui-go/cmd/pulse-vault ./gui-go/crypto -count=1
```

Or run `.\test.ps1` from this directory. Building or testing the top-level desktop package also compiles the Fyne/CGO backend and needs the platform C toolchain.

## Packages

| Path | Role |
| --- | --- |
| `main.go` | Desktop app entry point and version output. |
| `cmd/pulse-vault` | CLI and interactive console. |
| `crypto` | Public protocol and cryptographic API. |
| `internal/crypto` | Scrypt, authenticated encryption, and V5/V6 stream readers/writers. |
| `internal/vault` | Container create/open, add/extract, verification, and password rotation. |
| `internal/ui` | Fyne interface and UI operation handling. |
| `internal/version` | Build version injected by release tooling. |

The full test and threat-model notes are in [`docs/WHY_GO.md`](../docs/WHY_GO.md) and [`docs/THREAT_MODEL.md`](../docs/THREAT_MODEL.md).
