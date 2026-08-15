# Pulse-Vault Threat Model

What this thing is supposed to stop, and what it isn't. Applies to the Go app
and `gui-go/crypto`. Python in `legacy/python/` doesn't count.

## Security goals

- Prevent an attacker without the password from recovering vault contents.
- Detect modification, truncation, reordering, and substitution of encrypted
  metadata or stream chunks.
- Make password guessing materially expensive through the selected Scrypt
  profile (or PBKDF2 only for legacy V1/V2 compatibility).
- Avoid nonce reuse within a key and stream by using random base nonces and
  authenticated chunk indices.
- Preserve compatibility with existing V1–V5 files without weakening the
  authentication checks for newly written V5 files.

## Assumptions

- The password is not guessable and is not reused elsewhere.
- The operating system, Go runtime, and release artifacts have not already
  been compromised.
- Randomness provided by the operating system is available and functioning.
- A caller that needs atomic extraction uses a temporary destination and only
  publishes it after decryption succeeds.

## Out of scope

- Malware or an attacker with code execution in the user account.
- Password recovery, weak-password protection, or protection against offline
  guessing of a deliberately weak password.
- Metadata leakage from file sizes, entry names required by the format, or the
  fact that a vault exists.
- Secure deletion from storage, swap, backups, or filesystem snapshots.
- A claim that Pulse-Vault's composition is stronger than the underlying
  audited primitives. New cryptographic primitives are not introduced here.

## Review requirements for protocol changes

Any change to key derivation, nonce construction, associated data, chunk
framing, compression ordering, or legacy dispatch must include:

1. Updated `docs/CRYPTO_PROTOCOL.md` documentation.
2. Cross-language golden vectors for both success and failure cases.
3. Round-trip, tamper, truncation, wrong-key, and resource-limit tests.
4. An explicit format identifier or migration rule when the wire format
   changes.
5. Independent cryptographic review before describing the change as secure.
