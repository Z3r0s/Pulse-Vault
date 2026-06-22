# Pulse-Vault

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

- Local-only desktop GUI.
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

```bash
git clone https://github.com/Z3r0s/Pulse-Vault.git
cd Pulse-Vault
python3 -m venv .venv
source .venv/bin/activate
pip install -e .
pulse-vault
```

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

Legacy `.PulseVault` files remain supported. Opening one prompts before it is
renamed to `.pulsevault`.

## Security Notes

- Your password is the root secret. If it is weak or lost, Pulse-Vault cannot save you.
- Pulse-Vault is designed for offline file privacy, not protection against malware on an already-unlocked computer.
- Secure Open extracts plaintext files to a temporary app directory before launching them. External viewers may create their own caches or recent-file entries.
- Carrier-file mode is casual disguise, not forensic invisibility.
- Keep backups. Vault corruption or password loss can make contents unrecoverable.

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

## Changelog

See [CHANGELOG.md](CHANGELOG.md).

## License

MIT License. Created by DNSPulse / Z3r0s.
