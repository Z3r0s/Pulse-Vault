# Downloads

Pulse-Vault is distributed in two phases:

## Current (pre-1.0)

Install from source on GitHub:

```bash
git clone https://github.com/Z3r0s/Pulse-Vault.git
cd Pulse-Vault
python3 -m venv .venv
source .venv/bin/activate
pip install -e .
pulse-vault
```

GitHub is the canonical source repository and issue tracker. There is no official packaged download channel yet.

**GUI users**: The desktop app now includes a prominent **"GitHub Releases"** sidebar button (always available), a clickable version badge linking to releases, and an expanded **About > GitHub Downloads & Releases** section (plus new dedicated dialog). These directly surface the GitHub Releases page.

GitHub Releases is now the dedicated download area, providing:
- Standalone executables for Linux (Ubuntu) and Windows
- Source distributions + wheels
- Checksums (SHA256SUMS)
- Security fuzz/property test reports
- Full changelog-driven release notes

Download the latest from https://github.com/Z3r0s/Pulse-Vault/releases

## Planned (toward 1.0)

Packaged builds, checksums, and release notes will be hosted on [dnspulse.org](https://dnspulse.org). GitHub will remain the public source tree; dnspulse.org will become the primary end-user download page once installers are ready.

The GUI enhancements ensure that even source-based users (and future binary users) are one click away from the official release assets.