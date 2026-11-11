# Changelog

All notable changes to Pulse-Vault are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

**Versioning policy:** Product version is the **Go native** release under `gui-go/`.

## [Unreleased]

### Added
- A desktop file picker with an editable location, name filtering, common locations, folder history, and selection details.
- A GitHub README showcase linking the desktop walkthrough, interactive benchmarks, and browser crypto demo to their repository documentation.
- Folder import and full-tree restore with a V7 path-aware container marker.
- Progress and cancellation for add, delete, extract, and password rotation; temporary-space estimates for add, delete, and password rotation.
- Password rotation streams authenticated plaintext directly into replacement encryption without plaintext staging files.
- Regression coverage for folder round trips, cancellation rollback, legacy stream retention, unsafe extraction paths, and tampered-stream rotation.
- Batch delete validates every requested vault name before mutation and removes the whole set with one atomic rewrite; available in the vault API, CLI, and interactive console.
- Filename and path filtering in the desktop file list, with keyboard shortcuts and clear match counts.
- A repeatable CLI benchmark that verifies synthetic bulk-file restores, measures batch deletion, verifies what remains, and can sample process memory.
- CI runs the real CLI against a standard-profile 1,000-file workload and an 8 × 10 MiB random-data workload for three complete runs each, reporting median times while checking restored hashes and batch-delete integrity.
- CI measures the virtualized GUI row-update path across five one-second runs, filtering Go environment headers from public logs.
- Linux CLI benchmarks fall back to `/proc` RSS sampling and fail CI if a bulk workload produces no memory samples.
- An interactive benchmark explorer with workload selection, an accessible operation slider, sampled-memory plots, search-allocation charts, and bulk path-check comparisons.
- Static, GitHub-safe SVG charts generated from the same benchmark data used by the explorer, with workload details, units, scales, and methodology.
- Before-and-after graph for the bulk ZIP-index change, with CLI timing and sampled RSS tied to the same workload and benchmark source.
- Added repeatable synthetic-only GUI captures for the locked, empty, populated, and compact-window states.
- Added an optional startup update prompt with an explicit network privacy disclosure, manual header entry point, cancellable download progress, checksum verification, and no automatic installation.
- Added regression coverage proving startup and **Not now** make no release request, while **Check now** starts exactly one request through the updater service.
- Added a production-source guard that confines network clients to the updater and enforces that release checks follow consent and downloads follow destination selection.
- Changed the PyPI launcher to stay offline during normal CLI use; installing or updating its cached Go binary now requires an explicit launcher command.
- Added visual review captures for update consent, privacy disclosure, the example release, download location, progress, and verified completion.
- Added a GUI choice between standard and hardened Scrypt profiles, explicit offline-guessing and weak-password acknowledgments, and the ability to raise a vault's KDF cost during password rotation.
- Added a desktop notification-area icon with Show and Exit actions; closing the window locks the vault and hides the app, and active operations block close/exit until they finish.
- Added regression coverage for KDF profile selection, password-rotation profile upgrades, and user acknowledgment of weak-password warnings.
- Added unlocked-only size and added-date details to virtualized file rows without exposing hashes or internal blob identifiers to the UI.
- Added keyboard commands for saving the selected file, saving all files, and deleting the selected file; shortcuts ignore text entry, open dialogs, and active jobs.
- V5/V6 stream buffers now use known input sizes to reduce small-file allocation without shrinking the full chunks used for large incompressible files; added a repeatable stream benchmark.
- A threat-model note that states which vault details stay visible and why reverse engineering a local file or binary cannot be made impossible.

### Changed
- CLI prompts for passwords without echo in interactive terminals; command help recommends stdin or password files for automation.
- The normal CLI exposes `standard` and `hardened` KDF profiles; the low-cost profile is restricted to tests.
- README and format/security documentation describe the current Go application, use cases, supported formats, and security limits.
- Updated Fyne, cryptography, compression, and terminal dependencies; expanded CI coverage and Linux GUI build prerequisites.
- Pinned GitHub Actions to current releases, scoped write permissions to publishing and release jobs, and made the complete QA workflow manually runnable.
- Clarified that the project owns its Go crypto API and vault format while relying on maintained cryptographic primitives rather than custom cipher implementations.
- Reduced bulk-import flush overhead while retaining a sync of the completed replacement vault, and removed continuous idle UI animations.
- Reused folded path names and the search result buffer during filename filtering without retaining stale paths; built a Unicode-aware path index for incremental-add conflict checks.
- Reworked file rows into compact single-line entries and moved the encryption label to the section heading so all four sample entries fit in a 640 × 480 window.
- Updated desktop action emphasis to follow vault state: Create while locked, Add while populated; Lock uses a standard action style instead of the destructive treatment. Buttons refresh only when their priority changes.
- Right-sized stream buffers per input to lower small-file working memory while keeping large random-input throughput.
- Reused stream nonce and authentication-data buffers; the 10 MiB random-stream benchmark measured a fresh 6.15 ms median with 48 allocations per operation without changing the wire format.
- Virtualized file rows now update both labels and refresh the row container once; the latest isolated benchmark median is 1.727 µs/op, 336 B/op, and 8 allocations across five one-second runs. It measures one recycled row update and container refresh, not painting, scrolling, or frame time.
- Added GitHub Pages publishing and CI checks that keep benchmark charts synchronized with their source data.
- Added manual runs for the QA workflow and limited Actions tokens to read-only by default; only release publication receives repository write access.
- Stacked stream benchmark metrics at medium window widths so axis labels remain readable instead of clipping.
- Fixed in-page navigation offsets so the sticky header does not cover benchmark section headings.
- Kept benchmark navigation labels intact on narrow screens by wrapping links into clear rows instead of breaking words.
- Compact windows keep Create, Open, and Hide in a labeled row and group related file actions so all controls fit at 640 × 480 without covering the workspace; the sidebar shortcut guide lists only commands available in the current vault state.
- Expanded the cryptography research page with the actual Go stream demo, a proposed layered library contract, password-KDF and key-schedule analysis, AEAD and stream-boundary tradeoffs, file-vault password-rotation research, rollback and metadata limits, a staged research program with stopping rules, and a standards watch for post-quantum file sharing. The page distinguishes current protocol behavior from research proposals and keeps its performance evidence tied to named workloads.
- Enforced a 640 × 480 minimum content size and added a regression check for the minimum supported workspace.
- Set a more comfortable 1240 × 800 default GUI size and added resize hysteresis around compact layout thresholds to reduce flicker near breakpoints.
- Expanded the pull-request template to request test, visual, protocol, performance, and benchmark-privacy evidence.
- Expanded the benchmark explorer and README with explained static graphs, before/after vault sizes, and links to the CLI guide, methodology, and changelog.
- Expanded GitHub QA to race-check the desktop UI package and reject broken local Markdown links.
- Grouped Dependabot minor and patch version updates within Go modules and GitHub Actions; major and security updates remain individually reviewable.
- CI builds the Go/WebAssembly demo once from the checkout root, checks its runtime file and V6 encrypt/decrypt round trip, and syntax-checks both browser scripts.
- Added a Windows PowerShell docs-preview command that builds the ignored WebAssembly demo assets before serving the interactive crypto research page locally.
- CI checks the crypto research slider's ZIP-validation numbers against the benchmark JSON so the 71.4 µs historical baseline remains tied to the 1,000-entry validation workload.
- Recorded repeatable performance results: indexed search at 100,000 paths measured 2.65 ms after a 12.71 ms index build, while the bulk-add path check measured 1.55 ms versus 23.74 ms for the former scan. Published workload medians omit device and account identifiers.
- Marked the retired Python implementation as non-detectable for GitHub language statistics while keeping the interoperability oracle in the repository.
- Updated the Go toolchain, GitHub Actions, Fyne, cryptography, compression, and terminal dependencies; the CLI workload script now reports medians across repeated disposable runs without recording host OS or hardware.
- Updated release publishing to `softprops/action-gh-release@v3.0.4`, which uses the current Node 24 Actions runtime.
- Bulk verify and extract now validate and index the ZIP directory once per operation instead of reopening and rescanning it per file; the latest three-run 1,000-file repeated-data workload measured 2.73× faster verification, 22.2% less extract-all time, 2.57× faster post-delete verification, and 4.9 MB lower sampled peak RSS during extract-all.
- ZIP path validation scans path segments without a `strings.Split` allocation per entry; the latest 10,000-entry microbenchmark measured 1.38× faster validation, 42% fewer allocated bytes, and 99.7% fewer allocations.
- Rescoped the crypto research to Pulse-Vault's encrypted-file format, Go API, KDF and stream choices, and file-vault password rotation; password loss remains unrecoverable in the current format.
- Source-build installer and CLI wrappers now disable embedded VCS metadata, and the threat model documents the limited analysis friction from stripped release symbols and debug tables.

### Fixed
- Kept the local “click to encrypt” research demo runnable by preparing its WebAssembly binary and matching Go runtime before serving the docs preview.
- Kept the desktop workspace usable in short windows and improved primary-button text contrast.
- Kept filename-filter benchmark outputs live so the legacy append work is consumed and measured.
- Collapsed the redundant vault-name line in short windows so a 640 × 480 file view can show all four synthetic review rows.
- Rendered file rows through a Fyne widget renderer so filenames, size/date details, and the encrypted marker appear in the live list and screenshots.
- Refreshed the virtualized file list when it becomes visible after unlocking a populated vault.
- Kept the unlocked workspace label to the vault filename instead of exposing the full local path.
- Updated CLI banner and migration help to distinguish V6 streams from V7 path containers.
- Fixed Windows extraction publishing: no-overwrite extracts use a rooted hard link with a native no-replace fallback, and failed replacements keep the existing output intact.
- Replaced create and hide-copy file-save pickers that could truncate destinations before validation with name-and-folder forms; existing targets are preserved, and embedding into the selected carrier now asks before replacing it.
- Corrected the publisher line encoding in generated GitHub release notes.

### Security
- Updated the compression and Go network dependencies and moved CI builds to a supported Go toolchain after a vulnerability scan identified fixes newer than the existing bot proposals.
- Updated `golang.org/x/crypto` past the OpenPGP advisory; Pulse-Vault does not import that unmaintained package.
- Reject vault KDF records whose parameters do not match an application-supported profile, and keep the low-cost test profile out of the public crypto profile list. This prevents a crafted vault from advertising weak custom scrypt settings as `standard`.
- CI rejects identifying benchmark metadata and filters Go benchmark environment headers from logs.

## [0.2.0] - 2026-08-15

### Removed
- The **Python vault** is out. No PyInstaller, no Python engine in CI. Old code is sitting in `legacy/python/` if you want to look at it. Do not install that folder.

  `pip install pulse-vault` (when published) is a **launcher for the Go CLI**, not a revival of the interpreter vault.

  Why: Go doesn't have a GIL so the GUI doesn't freeze on encrypt. One language for the vault, CLI, and GUI. A project-owned Go crypto API and vault format, built on maintained cryptographic primitives instead of the archived Python runtime. One `.exe` / binary, done.

  The interoperability comparison uses identical files on both implementations. Re-run:

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
- Builds: `-trimpath -buildvcs=false -s -w`. Vault metadata stores the filename, not the source path. ZIP timestamps are zeroed. File bytes are still exact.
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
