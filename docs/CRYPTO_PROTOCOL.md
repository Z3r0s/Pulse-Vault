# Pulse-Vault cryptographic protocol

This document defines the wire formats implemented by
[`gui-go/crypto`](../gui-go/crypto/). The package owns the protocol and its
compatibility API; it does not invent a new cipher. It composes standard
primitives and adds authenticated framing, bounded parsing, and stable format
identifiers.

## Primitive and key layout

V3–V6 derive a 64-byte key with Scrypt. The first 32 bytes are the
ChaCha20-Poly1305 key and the second 32 bytes are the AES-256-GCM key.

The built-in profiles are:

| Profile | N | r | p |
| --- | ---: | ---: | ---: |
| fast | 16 | 8 | 1 |
| standard | 32768 | 8 | 1 |
| hardened | 1048576 | 8 | 1 |

V1/V2 compatibility uses PBKDF2-HMAC-SHA256, 600,000 iterations, and a
32-byte AES-256-GCM key.

## Metadata cascade

V3–V6 metadata is encoded as UTF-8 JSON, then encrypted as:

1. ChaCha20-Poly1305 with a random 12-byte nonce.
2. AES-256-GCM over the resulting ciphertext with a random 12-byte nonce.

The stored value is `chacha_nonce || aes_nonce || outer_ciphertext`.

V1/V2 metadata uses `nonce || AES-GCM ciphertext`. V1 authenticates the
`Z3R0VAULT1` magic as AAD; V2 has no AAD.

## V5 stream format (legacy)

V5 streams are:

```text
PV5STRM1                       8 bytes
compression flag               1 byte (0 none, 1 XZ legacy, 2 zstd)
ChaCha base nonce              16 bytes
AES base nonce                 12 bytes
repeat {
    encrypted chunk length     4-byte big-endian unsigned integer
    encrypted chunk             length bytes
}
```

Writers emit at least one authenticated chunk, including for an empty
uncompressed input, so the header is covered by AEAD associated data in every
newly written stream. Readers continue to accept older Python streams that
represent an empty uncompressed input with no chunks.

When compression is enabled, new writers run zstd (flag 2) before chunking.
Flag 1 remains XZ for streams written by older Go/Python implementations;
readers must accept 0, 1, and 2. High-entropy input may be stored with flag 0
so the compressor is not spent on photos, video, or already-compressed files.

Don't expect 50 GB of photos to become 2 MB. That only happens on junk like all zeros.

Each chunk is encrypted ChaCha20-Poly1305 first and AES-GCM second. The first 12
bytes of each base nonce are XORed with the big-endian chunk index to produce
the per-chunk nonce. The authenticated data is:

```text
PV5STRM1 || compression flag || ChaCha base nonce || AES base nonce || index
```

Chunk sizes are bounded by the library's exported limits. Truncation,
reordering, nonce changes, length corruption, and authentication failures must
be rejected.

## V6 finalized stream format

Current Go vaults use `PV6STRM1`:

```text
PV6STRM1                       8 bytes
compression flag               1 byte (0 none, 2 zstd)
ChaCha base nonce              16 bytes
AES base nonce                 12 bytes
repeat {
    record kind                1 byte (0 data, 1 terminal)
    encrypted record length    4-byte big-endian unsigned integer
    encrypted record           length bytes
}
```

Data records use the V5 compression choices where applicable, and the final
record encrypts the fixed terminal marker `PULSEVAULT6-END`. V6 associated data
is `PV6STRM1 || compression flag || nonces || record kind || index`. Readers
must require a valid terminal record and reject trailing bytes after it.

## V4 compatibility stream

V4 uses the same two-layer chunk construction but stores only the 16-byte
ChaCha base nonce, 12-byte AES base nonce, and length-prefixed chunks. It has no
magic, compression flag, or authenticated data. V4 is supported for reading
legacy Python vaults and is not emitted by the current Go writer.

## Compatibility rules

- Existing format identifiers must not be silently redefined.
- New framing or primitive choices require a new format identifier.
- V1/V2 are read-only compatibility formats and should be migrated to V6.
- Cross-language vectors must be updated before changing any byte layout.
- Passwords, keys, and plaintext must not be logged by library consumers.

The protocol is not a substitute for independent cryptographic review. The
security properties depend on correct primitive implementations, random nonce
generation, strong passwords, safe key handling, and the host machine.
