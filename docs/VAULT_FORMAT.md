# Vault Format

This document describes the current Pulse-Vault V5 container at a high level. It is intended for maintainers and security reviewers, not as a frozen compatibility contract.

## Container

V5 vaults are ZIP containers with encrypted metadata and encrypted file blobs.

```text
salt.bin
format.txt
kdf.json
metadata.enc
data/<uuid>.enc
data/<uuid>.enc
```

Optional carrier mode prepends an image or video before the ZIP data. The vault ZIP is appended after the carrier bytes.

## Format Marker

```text
PULSEVAULT5_COMPRESSED_CASCADE
```

Legacy vaults are upgraded to the current format when opened and saved.

## Key Derivation

Pulse-Vault derives a 64-byte key from the user password and random salt using Scrypt.

- first 32 bytes: ChaCha20-Poly1305 key
- second 32 bytes: AES-GCM key

The salt is stored in `salt.bin`.

### Scrypt Profiles

Runtime KDF cost is selected by `PULSEVAULT_SCRYPT_PROFILE`:

| Profile | N | r | p | Intended use |
| --- | ---: | ---: | ---: | --- |
| `fast` | 16 | 8 | 1 | CI and local test runs |
| `standard` | 32768 | 8 | 1 | Default production setting |
| `hardened` | 1048576 | 8 | 1 | Higher-cost unlock for sensitive vaults |

`PULSEVAULT_TEST_FAST_KDF=1` selects the `fast` profile for backward compatibility.
`PULSEVAULT_SCRYPT_N` can override `N` when the active profile is `fast`.

`kdf.json` stores the Scrypt parameters used when the vault was created. Unlock always uses the recorded values. Vaults created before 0.2.1 may omit this file and fall back to the runtime default profile.

Deterministic KDF and stream vectors for the `fast` profile live in `tests/vectors/`. A `standard` KDF vector is optional and can be regenerated with `python tests/generate_vectors.py --profile standard`.

## Metadata

`metadata.enc` stores encrypted JSON metadata, including file names, sizes, timestamps, SHA-256 digests, and internal blob IDs.

Metadata is encrypted with the same cascade construction used by legacy in-memory records:

1. ChaCha20-Poly1305
2. AES-GCM

## File Entries

Each file is stored as an encrypted stream under `data/<uuid>.enc`.

V5 stream layout:

```text
magic | compression_flag | chacha_nonce | aes_nonce | repeated encrypted chunks
```

Each chunk is authenticated with associated data that binds it to the stream header and chunk index. File contents are compressed with LZMA/XZ before encryption unless compression is disabled by the writer.

## Integrity

- Metadata authentication fails unlock if metadata is corrupted or the password is wrong.
- File stream authentication fails extraction or verification if encrypted blobs are corrupted.
- SHA-256 digests are stored for added files and checked during verification and extraction.

## Compatibility Notes

Pulse-Vault keeps read support for older vault markers where practical, then rewrites current vaults as V5 on save or password rotation.
