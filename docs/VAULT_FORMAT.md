# Vault format

This describes the container markers currently read and written by the Go implementation. It is a maintainer overview; the cryptographic details are in [CRYPTO_PROTOCOL.md](CRYPTO_PROTOCOL.md).

## Container layout

V5, V6, and V7 are ZIP containers. Members use ZIP's store method; file data is already compressed when compression is useful.

```text
salt.bin
format.txt
kdf.json
metadata.enc
data/<uuid>.enc
...
```

`metadata.enc` contains authenticated encrypted JSON. It records file names, sizes, timestamps, SHA-256 digests, and blob IDs. `format.txt` and `kdf.json` are visible container metadata. V7 paths are relative and use `/` separators inside metadata; the V7 marker is checked against the authenticated metadata version to reject marker downgrades.

The ZIP directory is visible. It reveals the entry count and encrypted blob sizes, while the format marker and Scrypt parameters are also readable. File names and paths are encrypted in `metadata.enc`; file lengths are still inferable from their ciphertext. This format does not pad entries to hide size or make the container impossible to inspect.

### Vaults carried after media

The container can be appended after a PNG, JPEG, or supported video file. Pulse-Vault locates the ZIP directory and preserves the leading media bytes when rewriting the vault. The media may still open in an ordinary viewer. This is a convenience and disguise only; it is not steganography and does not hide the appended data from inspection.

## Format markers and writes

| Marker | Use |
| --- | --- |
| `PULSEVAULT5_COMPRESSED_CASCADE` | Legacy authenticated stream format; read and retained by ordinary flat-file changes to V5 vaults. |
| `PULSEVAULT6_AUTHENTICATED_CASCADE` | Current stream format for new vaults and flat-file additions. |
| `PULSEVAULT7_PATHS_AUTHENTICATED_CASCADE` | Path-preserving container for folder trees; its new streams use V6 framing. |

New vaults use V6. Adding flat files to a V5 or V6 vault keeps that container marker. Adding a folder changes the container to V7 so relative paths can be stored. A V7 container may contain V5 streams retained from an older V5 vault and V6 streams added or re-encrypted later; each blob's `stream_format` metadata identifies its reader. Password rotation re-encrypts all streams as V6 and preserves V7 paths.

V1–V4 are legacy formats. The Go implementation reads them where supported and migrates their contents during password rotation. V5 remains readable. See the [protocol specification](CRYPTO_PROTOCOL.md) for compatibility boundaries.

## Key derivation

New formats derive a 64-byte key from the password and random salt with Scrypt. The first 32 bytes are used with ChaCha20-Poly1305; the second 32 bytes are used with AES-256-GCM. The salt is stored in `salt.bin`. `kdf.json` records the exact parameters, which unlock validates against resource limits before deriving the key.

| Profile | N | r | p | Intended use |
| --- | ---: | ---: | ---: | --- |
| `standard` | 32768 | 8 | 1 | Default for normal use. |
| `hardened` | 1048576 | 8 | 1 | Higher unlock cost on systems with enough memory. |
| `fast` | 16 | 8 | 1 | Tests only; the CLI requires `PULSEVAULT_TEST_FAST_KDF=1`. |

Files created before KDF metadata was recorded use a documented compatibility fallback. Do not lower the KDF cost on real vaults to make tests faster.

## Metadata and file streams

Metadata is serialized as JSON and encrypted with the same two-stage authenticated construction as described in the protocol document. V7 validates every stored path on unlock and rejects absolute paths, traversal, non-portable names, and Windows device names.

V5 stream layout (legacy, still readable):

```text
magic | compression_flag | chacha_nonce | aes_nonce | repeated encrypted chunks
```

V6 stream layout (used for new writes):

```text
PV6STRM1 | compression_flag | chacha_nonce | aes_nonce |
repeated: record_kind | record_length | encrypted_record
```

V6 record kind `0` is an encrypted data chunk; kind `1` is the required encrypted terminal record. The terminal record is authenticated with its chunk index, and readers reject a missing terminal, trailing bytes, malformed records, and authentication failures. New writes use zstd only when it reduces the data; older XZ streams remain readable.

## Integrity and compatibility

- Metadata authentication failure prevents unlock.
- File stream authentication failure prevents extraction and verification.
- SHA-256 digests recorded at import are checked by `verify` and extraction.
- V5 streams are authenticated but do not have V6's terminal-record guarantee; V5 compatibility remains available for existing data.
- Password rotation verifies each old stream while writing its replacement, then atomically replaces the container after the new ZIP is complete.

An interrupted add, delete, or password rotation leaves the original vault file in place. Temporary operation files contain encrypted staged blobs, not plaintext. Extraction writes a temporary plaintext file in the selected output folder and publishes it only after authentication and hash checks succeed. Secure deletion of temporary or extracted plaintext from storage, swap, backups, and snapshots is outside the format's guarantees.
