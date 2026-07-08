# Downloads

Pulse-Vault is distributed in two phases:

## Current (pre-1.0)

**Full installation instructions** (including Linux `tkinter` system packages, venv setup, Windows/macOS, Parrot script, pip install -e, CLI entries, desktop integration, App Store / Snap preparation notes) are in [INSTALL.md](../INSTALL.md).

GitHub is the canonical source repository and issue tracker. There is no official packaged download channel yet (see GitHub Releases below).

**pip users:** `pip install -e .` from source gives full CLI/GUI entry points (`pulse-vault`, `pulse-vault-cli`). See INSTALL.md for details. Future releases will publish wheels/sdists.

**App Store / Ubuntu readiness:** Expanded AppStream metainfo in `packaging/linux/io.github.z3r0s.PulseVault.metainfo.xml` (detailed releases, provides binaries, screenshots placeholders, OARS, keywords). Validated in CI. Remaining: real screenshots (user-provided). See "Ubuntu App Store / AppStream Readiness" in INSTALL.md.

**GUI users**: The desktop app now includes a prominent **"GitHub Releases"** sidebar button (always available), a clickable version badge linking to releases, and an expanded **About > GitHub Downloads & Releases** section (plus new dedicated dialog). These directly surface the GitHub Releases page.

GitHub Releases is the dedicated download area (prominently linked from the app's "GitHub Releases" sidebar button, version badge, and About dialog).

It provides (once the first `v*` tag is pushed and the release workflow runs):
- Standalone executables for Linux (Ubuntu), Windows, and macOS (via GitHub Releases on tagged versions). See INSTALL.md for important notes on Windows antivirus false positives (very common with PyInstaller; we use clean builds + no UPX + checksums to help).
- Source distributions + wheels
- Checksums (SHA256SUMS)
- Security fuzz/property test reports
- Full changelog-driven release notes

You may currently see "No releases found" because no tags have been created yet (pre-1.0). Visit https://github.com/Z3r0s/Pulse-Vault/releases to create the first one or view history.

## Planned (toward 1.0)

Packaged builds, checksums, and release notes will be hosted on [dnspulse.org](https://dnspulse.org). GitHub will remain the public source tree; dnspulse.org will become the primary end-user download page once installers are ready.

The GUI enhancements ensure that even source-based users (and future binary users) are one click away from the official release assets.