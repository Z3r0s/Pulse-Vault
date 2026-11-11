# Downloads

A **[DNSPulse](https://dnspulse.org)** product. How to install: **[INSTALL.md](../INSTALL.md)**. How we publish: [DISTRIBUTE.md](DISTRIBUTE.md).

## Choose a download

Open the [latest GitHub release](https://github.com/Z3r0s/Pulse-Vault/releases/latest) and expand **Assets**. Choose the desktop app or CLI for your operating system and processor.

| Platform | Desktop app | CLI |
| --- | --- | --- |
| Windows x64 | `pulse-vault-gui-windows-amd64.exe` | `pulse-vault-windows-amd64.exe` |
| Windows ARM64 | See the source build guide | `pulse-vault-windows-arm64.exe` |
| Linux x64 / ARM64 | Choose the GUI asset if the release includes one for your architecture | `pulse-vault-linux-amd64` / `pulse-vault-linux-arm64` |
| macOS Intel / Apple silicon | Choose the GUI asset if the release includes one for your architecture | `pulse-vault-darwin-amd64` / `pulse-vault-darwin-arm64` |

## Install from a terminal

| You | Do this |
| --- | --- |
| Windows, want the window | [Releases](https://github.com/Z3r0s/Pulse-Vault/releases) → `pulse-vault-gui-windows-amd64.exe` |
| Windows, want the CLI | `irm https://raw.githubusercontent.com/Z3r0s/Pulse-Vault/main/scripts/install.ps1 \| iex` |
| Linux / macOS CLI | `curl -fsSL https://raw.githubusercontent.com/Z3r0s/Pulse-Vault/main/scripts/install.sh \| sh` |
| pip | `pip install pulse-vault`, then explicitly run `pulse-vault --launcher-install` (Go launcher; needs a `v*` tag) |
| This git clone | `.\cli.ps1` / `./cli.sh` (CLI, no env vars) or `scripts\install.cmd` |

## GitHub Release assets

Via [`.github/workflows/release-go.yml`](../.github/workflows/release-go.yml):

| Asset | What it is |
| --- | --- |
| `pulse-vault-gui-windows-amd64.exe` | **Desktop app** (double-click, no console) |
| `pulse-vault-gui-linux-*` / `darwin-*` | Desktop app for that OS |
| `pulse-vault-windows-amd64.exe` / `arm64` | CLI |
| `pulse-vault-linux-amd64` / `arm64` | CLI |
| `pulse-vault-darwin-amd64` / `arm64` | CLI |
| `SHA256SUMS` | Checksums (install scripts verify these) |
| `install.sh` / `install.ps1` | Same installers, attached to the tag |

Git tags and published release assets are separate steps. If the latest release does not contain a binary for your platform, use a clone and build from source, or see [DISTRIBUTE.md](DISTRIBUTE.md).

## Optional desktop update checks

The desktop app asks at launch whether you want to check for a newer stable release. Choosing **Not now** makes no network request; you can also use **Check updates** in the header later. If you opt in, GitHub sees the connection's source IP and request time. A VPN or `HTTPS_PROXY` configured before the app starts can route the request differently. The release download is a separate choice; requesting it also reveals the selected platform asset to GitHub. Pulse-Vault checks it against `SHA256SUMS` from that same release before saving it to your chosen folder. It does not install updates automatically; this checksum is not an independent publisher signature.

## Build it yourself

[INSTALL.md](../INSTALL.md) (developer section) and [gui-go/README.md](../gui-go/README.md).

```powershell
.\cli.ps1 -Build
```

```bash
./cli.sh --build
# or: go install github.com/Z3r0s/Pulse-Vault/gui-go/cmd/pulse-vault@latest
```

`pip install pulse-vault` installs a wrapper that runs a verified local Go CLI without a GitHub request during normal use. `pulse-vault --launcher-install` and `pulse-vault --launcher-update` are explicit GitHub download actions. Do not install [legacy/python/](../legacy/python/).

Site: [dnspulse.org](https://dnspulse.org).
