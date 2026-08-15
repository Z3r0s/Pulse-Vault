# Downloads

A **[DNSPulse](https://dnspulse.org)** product. Go GUI + CLI: [`gui-go/`](../gui-go/). Publish walkthrough: [DISTRIBUTE.md](DISTRIBUTE.md).

## Get a binary

| How | What you get |
| --- | --- |
| [GitHub Releases](https://github.com/Z3r0s/Pulse-Vault/releases) | CLI + GUI + `SHA256SUMS` after a `v*` tag |
| `scripts/install.ps1` / `scripts/install.sh` | CLI into a user bin dir, hash-checked |
| `pip install pulse-vault` | Launcher that fetches that same CLI |
| `snap install pulse-vault` | Store build of the Go CLI (after upload) |
| Build from source | [`INSTALL.md`](../INSTALL.md) |

```powershell
irm https://raw.githubusercontent.com/Z3r0s/Pulse-Vault/main/scripts/install.ps1 | iex
```

```bash
curl -fsSL https://raw.githubusercontent.com/Z3r0s/Pulse-Vault/main/scripts/install.sh | sh
```

### GitHub Release assets

Via [`.github/workflows/release-go.yml`](../.github/workflows/release-go.yml):

| Asset | Notes |
| --- | --- |
| `pulse-vault-windows-amd64.exe` / `arm64` | Pure-Go **CLI** |
| `pulse-vault-linux-amd64` / `arm64` | Pure-Go **CLI** |
| `pulse-vault-darwin-amd64` / `arm64` | Pure-Go **CLI** |
| `pulse-vault-gui-*` | Desktop **GUI** (built per OS runner; CGO/OpenGL) |
| `SHA256SUMS` | Checksums for release files |

If Releases still says “No releases found”, no tag has been pushed yet. Build locally (below) or see [DISTRIBUTE.md](DISTRIBUTE.md).

### Build from source (Go)

Full steps: [INSTALL.md](../INSTALL.md) and [gui-go/README.md](../gui-go/README.md).

```powershell
# Multi-OS CLI + host GUI (Windows example)
powershell -File gui-go/scripts/build-multi.ps1 -OutDir dist
```

```bash
# CLI only (any GOOS/GOARCH, no CGO)
cd gui-go
CGO_ENABLED=0 go build -o pulse-vault ./cmd/pulse-vault
./pulse-vault version

# Desktop GUI (needs CGO toolchain)
CGO_ENABLED=1 go build -o pulse-vault-gui .
./pulse-vault-gui --version
```

Official site: [dnspulse.org](https://dnspulse.org). Source and issues: this GitHub repository.

`pip install pulse-vault` wraps the Go CLI. Do not install [legacy/python/](../legacy/python/).

## Planned (toward 1.0)

Signed installers and a download page on [dnspulse.org](https://dnspulse.org). GitHub stays the file host. See [ROADMAP.md](ROADMAP.md).
