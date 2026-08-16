# Install Pulse-Vault

A **[DNSPulse](https://dnspulse.org)** product. One local encrypted vault. No account.

**Want the window?** use the GUI. **Want a terminal?** use the CLI. Same vault files.

| I am on | I want | Do this |
| --- | --- | --- |
| Windows | the app | [Releases](https://github.com/Z3r0s/Pulse-Vault/releases) → `pulse-vault-gui-windows-amd64.exe` → double-click |
| Windows | the app, from this folder | double-click [`scripts/install.cmd`](scripts/install.cmd) |
| Windows | the CLI | `.\cli.ps1` from a clone, or Releases `pulse-vault-windows-amd64.exe` |
| Linux / macOS | the CLI | one-liner below |
| any OS | I already have Python | `pip install -U pulse-vault` *(live on PyPI — Go CLI launcher)* |
| this repo | CLI, right now | `.\cli.ps1` / `./cli.sh` (no env vars) |

Full asset list: [docs/DOWNLOADS.md](docs/DOWNLOADS.md). How we publish: [docs/DISTRIBUTE.md](docs/DISTRIBUTE.md).

## Windows

### Desktop app (easiest)

1. Open [GitHub Releases](https://github.com/Z3r0s/Pulse-Vault/releases).
2. Download `pulse-vault-gui-windows-amd64.exe`.
3. Double-click. No console window. No Python.

If Releases is empty, clone this repo and double-click `scripts\install.cmd` (builds CLI + GUI; GUI needs [MSYS2](https://www.msys2.org/) mingw gcc).

### Command line

From a clone (this is the path that does not look like malware to Defender):

```powershell
.\cli.ps1 version
.\scripts\install.ps1              # CLI into %LOCALAPPDATA%\Pulse-Vault + PATH
.\scripts\install.ps1 -WithGui     # CLI + app + Start Menu
```

Then:

```powershell
pulse-vault version
pulse-vault create $HOME\Documents\my.pulsevault --password 'choose-a-strong-password'
```

`irm | iex` works but Defender is trained to hate that pattern. Prefer the clone or a GitHub Release exe. SmartScreen notes: [docs/TRUST.md](docs/TRUST.md).

```powershell
.\scripts\install.ps1 -FromSource  # ignore GitHub, build here
```

## Linux and macOS

```bash
curl -fsSL https://raw.githubusercontent.com/Z3r0s/Pulse-Vault/main/scripts/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"   # this shell, if needed
pulse-vault version
```

```bash
curl -fsSL https://raw.githubusercontent.com/Z3r0s/Pulse-Vault/main/scripts/install.sh | sh -s -- --gui
```

From a clone (no tag required):

```bash
./scripts/install.sh
./scripts/install.sh --gui
./scripts/install.sh --from-source
```

macOS GUI from source needs Xcode CLT (`xcode-select --install`). Linux GUI from source needs gcc + OpenGL headers, e.g. `sudo apt install gcc libgl1-mesa-dev xorg-dev`.

## pip and snap

```bash
pip install -U pulse-vault
pulse-vault version
pulse-vault --launcher-info
pulse-vault --launcher-update
```

This is live on PyPI. The wrapper downloads the official **Go** CLI and checks `SHA256SUMS`. It is not [`legacy/python/`](legacy/python/). The GitHub Release must include the `pulse-vault-*` binaries (a source-only tag is not enough).

```bash
sudo snap install pulse-vault    # after the snap is on the store
```

## From this repo — CLI (no flags, no gcc)

Install [Go](https://go.dev/dl/). Then, from the repo root:

```powershell
.\cli.ps1 version
.\cli.ps1 create .\demo.pulsevault --password 'choose-a-strong-password'
.\cli.ps1 list .\demo.pulsevault --password 'choose-a-strong-password'
```

```bash
chmod +x cli.sh
./cli.sh version
./cli.sh create ./demo.pulsevault --password 'choose-a-strong-password'
```

That builds `pulse-vault.exe` / `pulse-vault` next to the script the first time. Rebuild: `.\cli.ps1 -Build` / `./cli.sh --build`.

Already have Go and just want it on PATH:

```bash
go install github.com/Z3r0s/Pulse-Vault/gui-go/cmd/pulse-vault@v0.2.0
```

Same thing, no extra env vars. `$(go env GOPATH)/bin` should be on your PATH.

`.\scripts\install.ps1 -FromSource` / `./scripts/install.sh --from-source` also builds the CLI and drops it in your user bin dir.

## From this repo — desktop app

The window is a different binary (needs a C compiler). The CLI does not.

```powershell
cd gui-go
.\build.ps1
```

```bash
cd gui-go
./../cli.sh version    # CLI, still no gcc
# GUI only:
# Windows: build.ps1   Linux/macOS: see gui-go/README.md
```

Windows GUI: [Go](https://go.dev/dl/) + [MSYS2](https://www.msys2.org/) mingw gcc (or let `build.ps1` find `C:\msys64\mingw64\bin`).

Tests: `cd gui-go` then `.\test.ps1` (or `go test ./internal/crypto ./internal/vault ./internal/ui ./cmd/pulse-vault ./crypto -count=1`).

## Check what you installed

Release files include `SHA256SUMS`. The install scripts check that automatically.

```text
.pulsevault
```

Format: [docs/VAULT_FORMAT.md](docs/VAULT_FORMAT.md). Current marker: `PULSEVAULT6_AUTHENTICATED_CASCADE` (V5 remains readable).

## Uninstall

- **Windows script:** delete `%LOCALAPPDATA%\Pulse-Vault` and the Start Menu shortcut “Pulse-Vault”. Remove that folder from your user PATH if you want.
- **Unix script:** `rm -f ~/.local/bin/pulse-vault ~/.local/bin/pulse-vault-gui`
- **pip:** `pip uninstall pulse-vault`
- **snap:** `sudo snap remove pulse-vault`

Site: [dnspulse.org](https://dnspulse.org).
