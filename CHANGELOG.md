# Changelog

All notable changes to Pulse-Vault are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-06-22

First public pre-release. Internal vault format work reached V5, but the shipped
application version starts at 0.1.0 while the project is still early.

### Added

- Local encrypted file vault with CustomTkinter desktop GUI.
- V5 vault format: Scrypt KDF, LZMA compression, ChaCha20-Poly1305 + AES-GCM cascade.
- Streaming encryption for large files.
- Vault verification without extracting plaintext.
- Password rotation with full file re-encryption.
- Carrier-file disguise mode.
- Drag-and-drop file import into an unlocked vault.
- Linux desktop metadata, MIME registration, and `install_parrot.sh`.
- Security docs: threat model, vault format, security policy.
- GitHub Actions test workflow.

### Changed

- Legacy `.PulseVault` / `.vault` files prompt before rename to `.pulsevault`.
- Older vault formats prompt before upgrade instead of migrating silently on unlock.
- Version numbering reset to 0.1.0 for the public release track.

### Security

- Hardened V5 stream authentication with per-chunk associated data.
- ZIP container validation before unlock.
- Filename sanitization and symlink refusal on extract.

[Unreleased]: https://github.com/Z3r0s/Pulse-Vault/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/Z3r0s/Pulse-Vault/releases/tag/v0.1.0