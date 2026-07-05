# Changelog

All notable changes to Pulse-Vault are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Comprehensive CLI tests in `tests/test_cli.py` covering guided commands (`create`, `list`, `verify`, `change-password`, `open` menu flows), error handling, password policy integration, argparse dispatch, and `--cli` entry points. Uses mocks for `getpass`, `input`, and stdout to enable headless testing of interactive flows.
- New `tests/test_main.py` exercising `main()` and early arg parsing for `--version`, `--cli`, `--help`, GUI dispatch paths (with heavy patching to avoid real windows), and ensuring CLI paths avoid pulling customtkinter when possible.
- Expanded property-based / advanced fuzz testing (Hypothesis) for crypto streams, filename sanitization roundtrips, and data edge cases.
- Additional vault tests for Unicode filenames, zero-length files, duplicate-add handling, empty vaults, and more tamper / policy scenarios.
- Deeper GUI smoke coverage and headless construction reliability.
- **New advanced test: `tests/test_hypothesis_vault_properties.py`** — the standout full roundtrip property-based test using Hypothesis. Features complex strategies for arbitrary file sets + password policy/adversarial passwords, bit-level tamper simulation on vault streams/metadata, KDF + encrypt/decrypt performance assertions and collection, brute-resistance elements (clean wrong-pw failures), integration with vectors + legacy fixtures, and GitHub-shining output: formatted console timing/tamper tables plus a `ci-artifacts/pulse-vault-security-fuzz-report.json` artifact. Runs fast under fast KDF. Selected as the winner design over standalone brute sim (partial pre-existing) or GUI+CLI matrix for maximum crypto-vault impressiveness and novelty.
- GitHub release automation (`.github/workflows/release.yml`) that parses the matching CHANGELOG section for release notes, builds sdist/wheel, attaches the advanced security property report + checksums on `v*` tags.
- Expanded CI (Windows matrix), Dependabot, CodeQL workflow, PR template enforcing changelog discipline, README badges, CODE_OF_CONDUCT.md, FUNDING.yml, dynamic version sourcing.
- **GUI polish for GitHub download users (top recommendations implemented):**
  - New always-visible "GitHub Releases" button in sidebar (`build_sidebar`).
  - Clickable version badge in sidebar linking to releases.
  - `open_github_releases()` + `show_downloads_dialog()` methods on `VaultGUI`.
  - Enhanced `show_about()` with dedicated "GitHub Downloads & Releases" section + buttons (links to releases + official site).
  - New `GitHubReleasesDialog` class and helper in `dialogs.py`.
  - Updated empty state in `build_main_view` with GitHub download guidance text.
  - Constants `GITHUB_RELEASES_URL` / `OFFICIAL_SITE`.
  - Expanded `tests/test_gui_smoke.py` (new tests + webbrowser mocks + dialog class check) while preserving all existing headless DummyCTk + full widget patching and fast KDF compatibility.
- `docs/DOWNLOADS.md` and README updated to reflect GUI surfacing of GitHub releases area.

### Fixed

- `test_app_can_be_constructed_headless` (GuiSmokeTests) now passes reliably in CI / headless Linux environments (no $DISPLAY / TclError). Uses precise mocks around CustomTkinter + Tk widget construction and root methods without altering production code.
- Minor: fixed `appearance_mode_optionemenu` -> `appearance_mode_optionmenu` (typo).

### Changed

- Vault I/O uses larger copy buffers and compact JSON metadata to reduce rewrite overhead.
- File import hashes and encrypts in a single disk pass instead of reading twice.
- Unlock probes only the vault header instead of a 5 MiB prefix; format marker is cached after unlock.
- V5 LZMA compression preset lowered for faster adds with modest size trade-off.
- Create-vault Scrypt runs off the UI thread; unlock status shows hardened profile when applicable.
- GUI sidebar row configuration and empty panel updated to accommodate new "GitHub Releases" button and download-focused messaging (no behavior change for vault ops).