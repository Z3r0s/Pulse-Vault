# Pulse-Vault

[![Tests](https://github.com/Z3r0s/Pulse-Vault/actions/workflows/test.yml/badge.svg)](https://github.com/Z3r0s/Pulse-Vault/actions/workflows/test.yml)
[![Latest Release](https://img.shields.io/github/v/release/Z3r0s/Pulse-Vault?sort=semver)](https://github.com/Z3r0s/Pulse-Vault/releases/latest)
[![Changelog](https://img.shields.io/badge/CHANGELOG-Keep%20a%20Changelog-blue)](CHANGELOG.md)

> **Note:** The Tests badge reflects the status of the most recent CI run on the `main` branch (currently all tests pass). The Latest Release badge will indicate a version once the first `vX.Y.Z` tag is pushed (releases are created by the workflow). You may see "No releases found" on the Releases page until then. See the "Changelog & Releases" section below.

Official site: [dnspulse.org](https://dnspulse.org)

Pulse-Vault is a local encrypted file vault from DNSPulse for Linux desktops. It stores files and folders in a portable `.pulsevault` container, works offline, and uses authenticated encryption with a memory-hard password derivation function.

Install from this GitHub repository for now. Packaged downloads on [dnspulse.org](https://dnspulse.org) are planned toward 1.0. See [docs/DOWNLOADS.md](docs/DOWNLOADS.md).

Windows can still run Pulse-Vault from source, but the primary packaging target is the Linux desktop.

## Goals

- Keep files private inside a locked local vault.
- Make vault contents hard to inspect, tamper with, or reverse after encryption.
- Stay simple enough for normal desktop users.
- Avoid cloud services, telemetry, accounts, or network dependencies.

## Features

- Local-only desktop GUI with direct "GitHub Releases" sidebar button, clickable version badge, and dedicated downloads section in About (links to the dedicated GitHub Releases area for builds and notes; shows guidance if no releases published yet).
- Streaming encryption for large files.
- Scrypt password-based key derivation with per-vault Standard or Hardened profiles.
- ChaCha20-Poly1305 plus AES-GCM cascade encryption.
- LZMA compression before file encryption.
- Vault verification without extracting plaintext files.
- Password rotation that re-encrypts file entries.
- Optional carrier-file disguise by appending vault data to media files.
- Drag-and-drop file import into an unlocked vault.

## Install

From source (current method):

### Linux (Ubuntu, Debian, Parrot, etc.)

Pulse-Vault uses CustomTkinter, which needs Python's `tkinter` module. On Linux,
`tkinter` is **not** installed by pip — it comes from a system package that must
match the Python version used for your virtual environment.

Install system packages first:

```bash
sudo apt update
sudo apt install python3-venv python3-pip python3-tk
```

If your default `python3` is not the version you use for the venv (for example
Python 3.14 from a PPA), install the matching `-tk` package as well:

```bash
python3 --version
# Example output: Python 3.14.x
sudo apt install python3.14-tk
```

Verify `tkinter` works **before** creating the venv:

```bash
python3 -c "import tkinter; print('tkinter OK')"
```

If that command fails, Pulse-Vault will not start until the correct `-tk` package
is installed for that exact Python version.

Then clone, install, and run:

```bash
git clone https://github.com/Z3r0s/Pulse-Vault.git
cd Pulse-Vault
python3 -m venv .venv
source .venv/bin/activate
pip install -e .
python -c "import tkinter; print('tkinter OK in venv')"
pulse-vault
```

If you installed `python3-tk` after creating the venv and still see
`ModuleNotFoundError: No module named 'tkinter'`, recreate the venv:

```bash
rm -rf .venv
python3 -m venv .venv
source .venv/bin/activate
pip install -e .
pulse-vault
```

### Windows / macOS

```bash
git clone https://github.com/Z3r0s/Pulse-Vault.git
cd Pulse-Vault
python3 -m venv .venv
source .venv/bin/activate   # Windows: .venv\Scripts\activate
pip install -e .
pulse-vault
```

After `pip install -e .` you also get a guided CLI (no GUI required):

```bash
pulse-vault --help
pulse-vault --cli --help
pulse-vault --cli create my.vault
pulse-vault --cli open my.vault     # then interactive menu: list / add / extract / verify...
pulse-vault-cli create my.vault     # dedicated CLI entry point
```

This makes Pulse-Vault suitable for headless use and future `apt`/`snap`/`pip` packaging.

Development shortcut:

```bash
pip install -r requirements.txt
python main.py
```

On Parrot OS or other Debian-style desktops:

```bash
chmod +x install_parrot.sh
./install_parrot.sh
```

That installs into `~/.local/share/pulse-vault`, adds a `pulse-vault` command,
registers the `.pulsevault` MIME type, and installs the desktop launcher.

Desktop metadata lives under `packaging/linux/`:

- desktop launcher
- MIME type registration
- AppStream metadata

The intended command name is:

```bash
pulse-vault
```

The intended vault extension is:

```text
.pulsevault
```

Legacy `.PulseVault` files remain supported. Opening one prompts before it is renamed.

## Usage (GUI)

1. Launch `pulse-vault` (or `python -m pulsevault`).
2. Click **+ New Vault** (optionally select a carrier image/video for casual disguise) or **Open Vault**.
3. Choose Scrypt profile (Standard recommended; Hardened for higher brute-force cost).
4. Use a strong unique password (14+ chars, variety enforced).
5. Add files/folders via buttons or drag-and-drop.
6. Double-click or use **Extract** / **Secure Open** (temporary plaintext launch).
7. **GitHub Releases** button and version badge (clickable) open the dedicated downloads area on GitHub. The page may show "No releases found" until the first tag is created.
8. Lock when done. Vault is a single portable file.

See the in-app **Security Notes** (sidebar) for architecture details.

## CLI (guided / packaging friendly)

See `pulse-vault --cli --help`. Useful for scripts, servers, or when no GUI is available. The same binary supports both modes.

## Packaging & Desktop Integration

See `packaging/linux/` and `docs/DOWNLOADS.md`.

## Security & Docs

Read the security docs:

- [Threat Model](docs/THREAT_MODEL.md)
- [Vault Format](docs/VAULT_FORMAT.md)
- [Security Policy](SECURITY.md)

## Current Vault Format

- KDF: Scrypt
- File encryption: ChaCha20-Poly1305 followed by AES-GCM
- Compression: LZMA/XZ before encryption
- Container: ZIP with encrypted metadata and encrypted `data/*.enc` entries
- Current marker: `PULSEVAULT5_COMPRESSED_CASCADE`

## Changelog & Releases

See [CHANGELOG.md](CHANGELOG.md) (Keep a Changelog + SemVer).

**GitHub Releases** is the dedicated download area (see the "GitHub Releases" button in the app sidebar and version badge). Releases are created automatically when a version tag (`vX.Y.Z`) is pushed — the workflow attaches builds, the fuzz report, and checksums.

Currently (pre-1.0 development) you may see "No releases found" on the [Releases page](https://github.com/Z3r0s/Pulse-Vault/releases) until the first tag is created. Source + wheels are always available via the repo or by tagging.

[View releases / create first tag →](https://github.com/Z3r0s/Pulse-Vault/releases)

## License

MIT License. Created by DNSPulse / Z3r0s.
