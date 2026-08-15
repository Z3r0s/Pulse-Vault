# Security Policy

Pulse-Vault is security-sensitive software. Please report vulnerabilities responsibly.

## Supported Versions

Until v1.0.0, only the latest public release is supported.

## Reporting A Vulnerability

Please open a private security advisory on GitHub if available, or contact the maintainer through the repository.

Do not publish exploit details for an unpatched vulnerability until there has been reasonable time to respond.

## Scope

Security-sensitive issues include:

- incorrect encryption or decryption behavior
- password-change data loss
- authentication bypass
- plaintext leakage
- predictable keys, salts, or nonces
- vault tampering not detected when it should be
- unsafe packaging or update behavior

Supported code is `gui-go/` and `gui-go/crypto`. Python in `legacy/python/` is dead.

Format is in `docs/CRYPTO_PROTOCOL.md`. Regular ciphers, not a new one. If you file a crypto bug, include the format version, a tiny repro, and whether it's the app or the crypto package.

## Non-Goals

Won't save you if the machine is already owned, the password is "password", or some other app leaves junk after you extract.
