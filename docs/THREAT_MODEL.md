# Threat Model

Pulse-Vault is designed for local, offline file privacy. It protects files after they are added to a locked vault and helps detect tampering before decrypted data is trusted.

## Protects Against

- Someone copying or stealing a locked vault file.
- Casual inspection of vault metadata, file names, or file contents.
- Tampering with encrypted metadata or encrypted file entries.
- Offline password guessing made more expensive through Scrypt.
- Vault-specific Scrypt cost recorded in `kdf.json` so unlock cost stays stable across machines.
- Large-file memory pressure by encrypting and decrypting file entries in streams.

## Does Not Protect Against

- Malware, keyloggers, remote access tools, or screen capture on an already compromised computer.
- Weak, reused, leaked, or forgotten vault passwords.
- Plaintext files after a user extracts or opens them outside the vault.
- External apps creating caches, thumbnails, recent-file entries, backups, or temporary files.
- Forensic proof that a carrier file contains appended data.

## Security Goals

- No network service, telemetry, cloud account, or remote dependency.
- No recovery backdoor.
- Authenticated encryption for metadata and file contents.
- Temporary plaintext outputs are removed on failed extraction.
- Password rotation re-encrypts file entries without writing plaintext staging files.

## Scrypt Profiles

| Profile | N | r | p | Approx. peak memory | Intended use |
| --- | ---: | ---: | ---: | --- | --- |
| `fast` | 16 | 8 | 1 | ~16 KiB | Maintainer tests and CI only |
| `standard` | 32768 | 8 | 1 | ~32 MiB | Default for new vaults |
| `hardened` | 1048576 | 8 | 1 | ~1 GiB | Higher guessing cost, slower unlock |

Hardened settings materially increase unlock time and RAM use. They do not replace a strong password.

## User Responsibilities

- Use a strong, unique password or generated key.
- Keep vault backups in safe places.
- Lock the vault when finished.
- Be careful with Secure Open and extracted plaintext files.
- Keep the operating system and dependencies updated.
