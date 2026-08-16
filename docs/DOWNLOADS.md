# Downloads

A **[DNSPulse](https://dnspulse.org)** product. How to install: **[INSTALL.md](../INSTALL.md)**. How we publish: [DISTRIBUTE.md](DISTRIBUTE.md).

## Fastest

| You | Do this |
| --- | --- |
| Windows, want the window | [Releases](https://github.com/Z3r0s/Pulse-Vault/releases) → `pulse-vault-gui-windows-amd64.exe` |
| Windows, want the CLI | `irm https://raw.githubusercontent.com/Z3r0s/Pulse-Vault/main/scripts/install.ps1 \| iex` |
| Linux / macOS CLI | `curl -fsSL https://raw.githubusercontent.com/Z3r0s/Pulse-Vault/main/scripts/install.sh \| sh` |
| pip | `pip install pulse-vault` (Go launcher; needs a `v*` tag) |
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

If Releases says “No releases found”, no `v*` tag has been pushed. Use a clone + `--from-source`, or see [DISTRIBUTE.md](DISTRIBUTE.md).

## Build it yourself

[INSTALL.md](../INSTALL.md) (developer section) and [gui-go/README.md](../gui-go/README.md).

```powershell
.\cli.ps1 -Build
```

```bash
./cli.sh --build
# or: go install github.com/Z3r0s/Pulse-Vault/gui-go/cmd/pulse-vault@main
```

`pip install pulse-vault` wraps the Go CLI. Do not install [legacy/python/](../legacy/python/).

Site: [dnspulse.org](https://dnspulse.org).
