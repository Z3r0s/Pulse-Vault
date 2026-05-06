# Vault Format

This document describes the current Pulse-Vault V5 container at a high level. It is intended for maintainers and security reviewers, not as a frozen compatibility contract.

## Container

V5 vaults are ZIP containers with encrypted metadata and encrypted file blobs.

```text
salt.bin
format.txt
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
