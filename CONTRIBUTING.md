# Contributing

Thanks for helping improve Pulse-Vault.

Official site: [dnspulse.org](https://dnspulse.org). Packaged downloads will be hosted there later.

App is Go, under [`gui-go/`](gui-go/). Install notes: [INSTALL.md](INSTALL.md).

## Development Setup

```bash
cd gui-go
go test ./internal/crypto ./internal/vault ./internal/ui ./cmd/pulse-vault ./crypto -count=1
# from repo root:
go test ./packaging -count=1
PYTHONPATH=packaging/pypi/src python -m unittest discover -s packaging/pypi/tests -v
```

That's what CI runs. No gcc needed. How to tag / pip / snap: [docs/DISTRIBUTE.md](docs/DISTRIBUTE.md).

Don't set `CGO_ENABLED=1` and then `go test ./...` unless `gcc` is on PATH — Fyne will try to compile the GUI package and die with `cgo: C compiler "gcc" not found`. Windows: `.\test.ps1` (skips GUI if no gcc) or `.\build.ps1` for the actual window.

CLI from the repo root: `.\cli.ps1` / `./cli.sh` (do not set `CGO_ENABLED`). Windows GUI: [`gui-go/build.ps1`](gui-go/build.ps1). See [INSTALL.md](INSTALL.md).

`tests/vectors/` is locked in. Don't regenerate those unless you actually changed the format.

## Changelog & Release Process

- Always update the `## [Unreleased]` section in [CHANGELOG.md](CHANGELOG.md) before submitting a PR (use standard Keep a Changelog headings: Added, Changed, Fixed, Security, etc.).
- The PR template will remind you.
- To cut a release:
  1. Finalize `[Unreleased]` → rename it to `## [X.Y.Z] - YYYY-MM-DD`.
  2. Add a fresh `## [Unreleased]` at the top.
  3. Update the latest `<release>` entry in `packaging/linux/io.github.z3r0s.PulseVault.metainfo.xml`.
  4. Run the Go tests above (`cd gui-go` and `go test ./internal/crypto ./internal/vault ./internal/ui ./cmd/pulse-vault ./crypto -count=1`).
  5. Commit, create `vX.Y.Z`, and push the tag.
  6. Verify the Go binaries and `SHA256SUMS` are present in the GitHub Release.

The repository-root `go.mod` versions the reusable package and application
together. Consumers can install the public API with:

```bash
go get github.com/Z3r0s/Pulse-Vault/gui-go/crypto@vX.Y.Z
```

## Version Sync Notes (for packaging / App Store readiness)

- The Git tag is the release source for Go CLI/GUI/library runtime versioning.
- Always sync `CHANGELOG.md` and `packaging/linux/io.github.z3r0s.PulseVault.metainfo.xml` before tagging.
- After version bump: locally validate with `desktop-file-validate packaging/linux/pulse-vault.desktop` and `appstream-util validate-relax packaging/linux/io.github.z3r0s.PulseVault.metainfo.xml` (or use CI).
- For Snap/Flatpak: ensure the metainfo version matches the snap version string.
- Test post-bump: from `gui-go/`, `go build` the CLI and `.\build.ps1` the GUI (windowsgui, no console), then `pulse-vault version` / `pulse-vault-gui --version`.
- The `.github/workflows/release-go.yml` workflow will automatically:
  - Run Go public-package, vault, CLI, and GUI tests
  - Build CLI artifacts for supported OS/architecture combinations
  - Build host GUI artifacts with CGO
  - Generate `SHA256SUMS`
  - Create the GitHub Release with matching assets
- See also the [GitHub Releases page](https://github.com/Z3r0s/Pulse-Vault/releases) and `docs/DOWNLOADS.md`.

Changelog is the record. Keep it updated.

## Guidelines

- Offline, local only. No telemetry.
- Test format / password change / migrate if you touch those.
- Don't invent a new cipher. New format = new marker + new vectors.
- Don't commit real vaults or keys.
