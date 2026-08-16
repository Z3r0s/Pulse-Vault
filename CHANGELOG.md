# Changelog

All notable changes to Pulse-Vault are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

**Versioning policy:** Product version is the **Go native** release under `gui-go/`.

## [Unreleased]

_No unreleased changes yet._

## [0.2.0] - 2026-08-15

### Removed
- The **Python vault** is out. No PyInstaller, no Python engine in CI. Old code is sitting in `legacy/python/` if you want to look at it. Do not install that folder.

  `pip install pulse-vault` (when published) is a **launcher for the Go CLI**, not a revival of the interpreter vault.

  Why: Go doesn't have a GIL so the GUI doesn't freeze on encrypt. One language for the vault, CLI, and GUI. Our own crypto package instead of wrapping whoever's Python bindings. One `.exe` / binary, done.

  Timed on my Windows box 2026-08-14 (same files both sides). Re-run:

  `cd gui-go && go test ./internal/vault -run TestCompareGoVsArchivedPython -v`

  | Workload | Python | Go | |
  | --- | ---: | ---: | --- |
  | Encrypt 4 MiB (text-like) | 34.0 ms | 2.1 ms | 16x |
  | 4x 1 MiB encrypt (Go parallel) | 39.3 ms | 3.7 ms | 10.5x |
  | Extract 2 MiB | 26.2 ms | 4.9 ms | 5.3x |
  | Add 2 MiB | 44.3 ms | 22.2 ms | 2x |
  | Encrypt 4 MiB (photo/video-like) | 1932 ms | 6.3 ms | 306x |

  Old XZ vaults still open. Scrypt is the same on both, not in this table.

### Added
- Go crypto package: `github.com/Z3r0s/Pulse-Vault/gui-go/crypto`
- Protocol doc, threat model, benches, fuzz smoke
- CLI can unlock V1–V5 and migrate to the current V6 format
- Hide in picture (`--carrier` / GUI button)
- GUI: delete, change password, overwrite on extract, weak password warning
- CLI looks like a console now. `open` is a prompt. Also `delete`, `--plain`
- pip launcher (`packaging/pypi`) and snap recipe (`snap/snapcraft.yaml`) that ship the Go CLI
- `scripts/install.sh` / `scripts/install.ps1` / `scripts/install.cmd` hash-checked installers (CLI, optional GUI, Start Menu, build-from-clone if no tag)
- Deeper Why Go write-up with charts ([docs/WHY_GO.md](docs/WHY_GO.md)), [DISTRIBUTE.md](docs/DISTRIBUTE.md), [ROADMAP.md](docs/ROADMAP.md)

### Changed
- New vaults use V6 finalized streams: authenticated terminal records detect clean-boundary truncation. V5 and old XZ (flag 1) streams still decrypt. Random-looking files skip compression (flag 0) so we don't waste time on jpgs.
- ZIP rewrite copies old blobs instead of loading them all. Extract hashes while it writes.
- Builds: `-trimpath -buildvcs=false -s -w`. Vault only stores the filename, not `C:\Users\...`. ZIP timestamps are zeroed. File bytes are still exact.
- GUI shell: tighter brand mark, wrapping header, scroll instead of clip on a small window, lock/accent/hero motion, drag-and-drop add/open. Windows GUI builds use `-H windowsgui` so double-click does not open a console.
- Why Go (not Python, not Rust) is its own page ([docs/WHY_GO.md](docs/WHY_GO.md)) at the top of the README, with charts.
- Install docs lead with “download the exe / one-liner”, not `go build`. Scripts fall back to building from a clone when GitHub has no `v*` tag.
- CLI from a clone is `.\cli.ps1` / `./cli.sh` or `go install ...@main`. You do not set `CGO_ENABLED`.
- pip launcher 0.2.0: GitHub API (skips source-only releases), `--launcher-update` / `--launcher-info`, pin with `PULSE_VAULT_VERSION`, re-checks the cached SHA-256. `pip install -U pulse-vault` after you upload 0.2.0.
- Windows PE now carries DNSPulse company/product strings, an asInvoker manifest, and an optional Authenticode hook (`gui-go/scripts/sign-windows.ps1`). See [docs/TRUST.md](docs/TRUST.md).

### Fixed
- Hide in picture actually works (create used to refuse an existing image)
- ZIP start is found from the end of the file so a fake `PK` in the picture doesn't break it
- Can't add the vault into itself, can't use symlinks as the vault
- Picture prefix survives add / delete / password change

### Security
- Same standard ciphers as before. New V6 streams require an authenticated terminal record, including empty files.
- Scrypt KDF records now have combined memory and work budgets before derivation starts.
- Vault rewrites use rollback-safe replacement; add and password-rotation staging no longer holds every blob in RAM.
- Interactive password changes use hidden terminal input; `--password-prompt` and `--new-password-prompt` avoid process-list exposure.
- Added pinned Staticcheck and govulncheck CI, CGO GUI compile smoke, Go 1.25 CI, and fixed x/image/xz dependencies.
- SHA-256 in the crypto package is only a file-integrity digest (verify/extract). Passwords stay on scrypt / legacy PBKDF2. CodeQL `go/weak-sensitive-data-hashing` was a false positive on that helper.

## [0.1.0] - 2026-07-22

### Go rewrite — speed, reliability, and a new desktop shell

Pulse-Vault **0.1.0** ships the **native Go** product path as primary. The previous Python / CustomTkinter stack hit practical **CPU and performance limits** under real vault workloads (KDF, streaming encryption, large-file add/extract, UI responsiveness). The Go CLI and multi-pane Fyne GUI target **speed and reliability** — a fast, solid local vault **without** a Python runtime, pip install, or PyInstaller as the default experience.

- **Primary product:** native Go binaries under `gui-go/` (desktop GUI + CLI).
- **Vault format unchanged:** V5 (`PULSEVAULT5_COMPRESSED_CASCADE`) — Scrypt, ChaCha20-Poly1305 + AES-GCM cascade, ZIP container. Existing `.pulsevault` files remain valid.
- **Python path retired:** `src/pulsevault/` and pip/PyInstaller packaging are **legacy reference only**. See `legacy/python/README.md`.

### Added
- **Major GUI overhaul (0.1.0 visual system):** obsidian + pulse-cyan theme, top command bar with monogram + lock badge, left nav rail with section labels, card-style main stage, hero locked state, richer encrypted file rows, status dock with progress.
- Pure-Go CLI (`gui-go/cmd/pulse-vault`): create / list / add / extract / info / version.
- Reactive vault ops: create / unlock / add / extract run **off the UI thread** via `fyne.Do` main-thread completion (Fyne 2.6+).
- Multi-OS CLI builds (windows/linux/darwin × amd64/arm64) via `gui-go/scripts/build-multi.ps1` and `.github/workflows/release-go.yml`.
- Windows production packaging: `build.ps1` stamps the shared `internal/version.Version`, embeds PE ProductName/icon via `goversioninfo`.
- Docs rewritten for Go-first install (`README.md`, `INSTALL.md`, `docs/DOWNLOADS.md`).

### Changed
- Product positioning: Go binaries first; Python demoted to legacy.
- Theme identity shifted from early blue-slate / Python Yaru-orange to **pulse-cyan on near-black**.

### Fixed
- Create-vault path after file-save dialogs (empty placeholder removed; zero-byte stubs accepted).
- Release GUI builds stamp the shared `internal/version.Version` (no longer stuck on `dev` in multi-OS/CI paths).

### Test / product parity
- Go vault API: change-password (full re-encrypt), verify/verify-all, delete, PeekKDF; CLI `verify` + `change-password`.
- Deep Go tests: multi-chunk streams, bitflip/truncation/corruption, lifecycle, KDF persistence, tamper.
- Dual-direction interop tests (`tests/test_go_interop.py`) + CI `go-parity` job alongside Python oracle suite.

## [0.0.21] - 2026-07-08

This release focuses on making Pulse-Vault feel like a proper, polished desktop application and getting the foundations solid for real distribution (PyPI, Snap, Ubuntu App Store, etc.). Historical Python/CustomTkinter track.

### Added
- Yaru/Ubuntu-inspired GUI theming (Python era).
- CLI/GUI import separation; AppStream metainfo; CI desktop validation.

### Changed
- README / INSTALL expansion for pip and store prep (superseded by Go path in 0.1.0).

[Unreleased]: https://github.com/Z3r0s/Pulse-Vault/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/Z3r0s/Pulse-Vault/releases/tag/v0.2.0
[0.1.0]: https://github.com/Z3r0s/Pulse-Vault/releases/tag/v0.1.0
[0.0.21]: https://github.com/Z3r0s/Pulse-Vault/releases/tag/v0.0.21
