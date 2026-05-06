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

## Non-Goals

Pulse-Vault does not claim to protect against malware on an already-compromised machine, weak passwords, or artifacts created by third-party applications after a user opens or extracts a file.
