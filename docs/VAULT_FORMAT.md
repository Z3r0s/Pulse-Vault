# Vault Format

This document describes the current Pulse-Vault V6 container at a high level. It is intended for maintainers and security reviewers, not as a frozen compatibility contract.

## Container

V6 vaults are ZIP containers with encrypted metadata and encrypted file blobs.

```text
salt.bin
format.txt
kdf.json
metadata.enc
data/<uuid>.enc
data/<uuid>.enc
```

Hide-in-picture: vault ZIP is stuck on the end of a PNG/JPEG/MP4. The picture still opens. It's a disguise, not magic.

Go locates the ZIP via the end-of-central-directory record so a `PK\x03\x04` sequence inside the image does not shift the prefix. The prefix is preserved on rewrite (add, delete, password change).

## Format Marker

```text
PULSEVAULT6_AUTHENTICATED_CASCADE
```

V1–V4 vaults are upgraded to V6 during migration or password rotation. V5
vaults remain readable and retain their marker during ordinary add/delete
writes; password rotation upgrades their file streams to V6.

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

`kdf.json` stores the Scrypt parameters used when the vault was created. Unlock always uses the recorded values. Vaults created before 0.0.20 may omit this file and fall back to the runtime default profile. (Note: versioning scheme reset to 0.0.x for granular early development.)

Deterministic KDF and stream vectors for the `fast` and `standard` profiles live in `tests/vectors/` as frozen golden fixtures. The Go test suite reads these files; they are not regenerated as part of the default maintainer workflow.

## Metadata

`metadata.enc` stores encrypted JSON metadata, including file names, sizes, timestamps, SHA-256 digests, and internal blob IDs.

Metadata is encrypted with the same cascade construction used by legacy in-memory records:

1. ChaCha20-Poly1305
2. AES-GCM

## File Entries

Each file is stored as an encrypted stream under `data/<uuid>.enc`.

V5 stream layout (legacy, still readable):

```text
magic | compression_flag | chacha_nonce | aes_nonce | repeated encrypted chunks
```

Chunks are bound to the header + index. New writes: zstd (flag 2). Old files: XZ (flag 1). Random-looking files: no compress (flag 0). Decrypt handles all three. Lossless — a jpg is not going to shrink 50x.

V6 stream layout (current writes):

```text
PV6STRM1 | compression_flag | chacha_nonce | aes_nonce |
repeated: record_kind | record_length | encrypted_record
```

Record kind `0` is encrypted data and kind `1` is an encrypted terminal
marker. The terminal marker is required, authenticated with the chunk index,
and checked after decompression. This makes truncation at a clean chunk
boundary fail instead of looking like a valid EOF.

## Integrity

- Metadata authentication fails unlock if metadata is corrupted or the password is wrong.
- File stream authentication fails extraction or verification if encrypted blobs are corrupted.
- SHA-256 digests are stored for added files and checked during verification and extraction.

## Compatibility Notes

Pulse-Vault keeps read support for V1–V5, then writes new vaults and password
rotations as V6. Existing V5 vaults remain V5 on ordinary add/delete writes so
their historical blobs stay readable; password rotation upgrades all blobs to
V6.
