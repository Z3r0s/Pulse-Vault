# Downloads

Go GUI + CLI: [`gui-go/`](../gui-go/). Grab a release or build it yourself (`go build` / [`gui-go/build.ps1`](../gui-go/build.ps1)).

## Current (primary)

### GitHub Releases (recommended)

[GitHub Releases](https://github.com/Z3r0s/Pulse-Vault/releases) is the download area for pre-built binaries once a `v*` tag is published.

Typical assets (via [`.github/workflows/release-go.yml`](../.github/workflows/release-go.yml) and `gui-go/scripts/build-multi.*`):

| Asset | Notes |
| --- | --- |
| `pulse-vault-windows-amd64.exe` / `arm64` | Pure-Go **CLI** |
| `pulse-vault-linux-amd64` / `arm64` | Pure-Go **CLI** |
| `pulse-vault-darwin-amd64` / `arm64` | Pure-Go **CLI** |
| `pulse-vault-gui-*` | Desktop **GUI** (built per OS runner; CGO/OpenGL) |
| `SHA256SUMS` | Checksums for release files |

If the Releases page still shows “No releases found”, no tag has been pushed yet (pre-1.0). Build locally instead (below).

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

Old Python: [legacy/python/README.md](../legacy/python/README.md). Don't install it.

## Planned (toward 1.0)

Packaged installers, checksums, and polished end-user pages on [dnspulse.org](https://dnspulse.org). GitHub remains the public source tree and release asset host until then.
