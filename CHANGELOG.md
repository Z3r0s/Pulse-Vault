# Changelog

All notable changes to Pulse-Vault are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

**Versioning policy (early development):** Starts at 0.0.1, increments up through the 0.0.x series before 0.1.0. Update this file for every change (small or large).

## [Unreleased]

### Focus on distribution, safety, and usability
This release cycle emphasized making Pulse-Vault ready for easy installation via pip and proper desktop integration across platforms, while addressing common issues with packaged Python apps.

We made significant progress on pip readiness: GUI dependencies (customtkinter, tkinterdnd2) are now optional via `pip install "pulse-vault[gui]"`. The core package installs cleanly without pulling in GUI libs, so the guided CLI works perfectly in headless environments, servers, CI, or minimal containers. Entry points for `pulse-vault`, `pulsevault`, and `pulse-vault-cli` are properly defined. Classifiers and metadata were expanded for Windows and macOS support.

Desktop readiness was deepened too. The Windows builds now embed proper version metadata so the executable shows "DNSPulse" as the company in file properties. Linux desktop files (desktop entry, MIME type, AppStream metainfo) are validated in CI and documented for .deb, Snap, and Ubuntu App Store packaging. macOS builds were added to the release workflow so binaries for all three platforms land in GitHub Releases automatically on tagged versions.

A common pain point with PyInstaller-packaged apps is false positive malware detections by Windows Defender and other AVs. We updated the build process (both in CI and local scripts) to use `--clean --noupx`. This produces larger but "cleaner" executables that are far less likely to trigger heuristics. We also added documentation explaining why this happens (bundled Python runtime + compression looks suspicious to scanners) and how users can verify the builds themselves (checksums in releases, build from source, VirusTotal scans from multiple engines). The project is 100% open source with reproducible builds from the tagged source.

We added a small `packaging/verify-build.py` helper that can be run after building or installing to quickly check version, CLI smoke tests, and (importantly) that the CLI path does not accidentally pull in GUI libraries.

### Other improvements
- Extended and human-readable changelog entries (this one included) for better release notes on GitHub.
- Continued polishing for usability and safety (detailed confirmations, better defaults to "standard" profile, clearer status and onboarding).
- macOS support now included in automated binary releases alongside Linux and Windows.

See the GitHub Releases page for attached binaries, checksums, and the security fuzz report artifact.

## [0.0.21] - 2026-07-08

This release focuses on making Pulse-Vault feel like a proper, polished desktop application and getting the foundations solid for real distribution (PyPI, Snap, Ubuntu App Store, etc.).

We gave the GUI a proper Ubuntu 26.04 treatment. The interface now uses the signature Yaru orange (#E95420) as the primary accent, updated dark and light color palettes that match modern GNOME, consistent rounded corners, Ubuntu font where available, and refined spacing and button sizing. It looks and feels much more native when running on Linux desktops.

A long-standing packaging pain point is finally fixed: the guided CLI no longer drags in any GUI dependencies. `import pulsevault.cli` (and `pulse-vault --cli`) stays completely clean of tkinter and customtkinter. This makes the tool genuinely usable in headless and packaging scenarios.

Documentation and store readiness got a big push. The AppStream metainfo is now much richer with proper release descriptions, keywords, categories, and placeholders ready for screenshots. INSTALL.md, README, and related docs were expanded with clear guidance for pip users and future App Store/Snap submissions. We added desktop file and AppStream validation to CI.

Testing is in excellent shape. The full suite passes cleanly, CLI/GUI isolation is now properly enforced with fresh subprocess tests, and the Hypothesis-based security property tests are more robust (including better tamper coverage).

We spent time on thorough internal reviews covering security, GUI polish, and packaging readiness. The changelog format was refreshed to be more readable while staying compatible with our release tooling.

### Added
- Yaru/Ubuntu 26.04-inspired GUI theming (orange accent, refined palettes, radii, fonts, adaptive colors) across sidebar, treeview, dialogs, buttons, and more.
- Proper CLI/GUI import separation so CLI paths never load GUI libraries.
- Expanded AppStream metainfo with narrative release descriptions, keywords, categories, and screenshot section.
- New and strengthened isolation tests using subprocess checks.
- CI job for desktop-file and AppStream validation.
- More human-friendly changelog style while preserving automation compatibility.

### Changed
- README reorganization into dedicated INSTALL.md and improved cross-references.
- Various docs (INSTALL, DOWNLOADS, CONTRIBUTING, etc.) updated for pip, App Store, and Snap preparation.
- Theme and font helpers centralized with better Yaru alignment.

### Fixed
- CLI dispatch logic to reliably avoid GUI imports.
- Hypothesis property tests for more reliable tamper detection (especially MAC layer).

See the GitHub release for the full set of changes and the attached security fuzz report.

## [0.0.20] - 2026-07-04

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
- Versioning policy updated in the header (see above). Changelog discipline continues for every change.

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

[Unreleased]: https://github.com/Z3r0s/Pulse-Vault/compare/v0.0.21...HEAD
[0.0.21]: https://github.com/Z3r0s/Pulse-Vault/releases/tag/v0.0.21
[0.0.20]: https://github.com/Z3r0s/Pulse-Vault/releases/tag/v0.0.20
