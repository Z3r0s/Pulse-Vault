# Engineering overview

Pulse-Vault is a local encrypted file vault maintained by [DNSPulse](https://dnspulse.org). The current product is written in Go: one codebase contains the vault container, command-line interface, and Fyne desktop app. The earlier Python application is archived and is not used by the Go product.

This page explains the implementation and its tradeoffs. It is not a claim that choosing a programming language makes the product secure; the security properties come from the protocol, its implementation, review, and the user's password and device.

## What ships

| Area | Implementation |
| --- | --- |
| Desktop app | Fyne, with long-running operations off the UI thread and progress/cancel controls for archive operations. |
| Command line | Go binary; builds without CGO. Interactive password entry is hidden. |
| Container | ZIP with neutral member timestamps, encrypted metadata, and encrypted file blobs. Rewrites use a temporary container and replace the original after completion. |
| Key derivation | Scrypt. The file records the parameters used, and unlock checks them against resource limits. |
| File streams | V6 uses ChaCha20-Poly1305 followed by AES-GCM for authenticated, framed chunks and a final record. V5 and earlier supported streams remain readable. |
| Folder paths | V7 stores validated relative paths in encrypted metadata. Its blobs use V6 streams; an upgraded V5 vault can temporarily retain individually marked V5 blobs until password rotation. |
| Compression | zstd for data that shrinks; XZ is included for reading older streams. |
| Public API | `gui-go/crypto` exposes the protocol helpers. `internal/vault` owns container operations. |

The main dependencies are listed in [`go.mod`](../go.mod). The security-sensitive code is not the Fyne UI: review [the crypto package](../gui-go/crypto/), [the vault package](../gui-go/internal/vault/), and the protocol and threat-model documents.

## Why one Go product

The practical goal was to ship a desktop app and CLI that share the same file format and can be built as native binaries. Go gives this project one implementation for the container, CLI, and GUI, a straightforward cross-platform build process for the CLI, and concurrency primitives useful for keeping long operations out of the UI event loop.

The GUI still has a C toolchain requirement because Fyne's desktop backend uses CGO. The CLI does not. That distinction matters when building on Windows and in CI, and is documented in [`gui-go/README.md`](../gui-go/README.md).

The archived Python implementation remains useful as historical compatibility context. It should not be used to create new vaults or treated as a supported application path. The PyPI package, where available, is a launcher that downloads the Go CLI.

## Review and verification

Run the Go tests from the repository root:

```powershell
go test ./gui-go/internal/crypto ./gui-go/internal/vault ./gui-go/internal/ui ./gui-go/cmd/pulse-vault ./gui-go/crypto -count=1
```

The suite covers fixed crypto vectors, stream round trips, tamper and truncation failures, legacy reads, vault rewrites, folder round trips, password rotation, and CLI flows. The Fyne desktop package needs CGO and a C compiler to build; see the GUI guide for platform requirements.

Useful review material:

- [Cryptographic protocol](CRYPTO_PROTOCOL.md)
- [Vault container format](VAULT_FORMAT.md)
- [Threat model](THREAT_MODEL.md)
- [Security policy](../SECURITY.md)
- [Dependency manifest](../go.mod)

Performance measurements against the archived Python implementation are kept in the changelog and the compatibility benchmark test. They describe specific workloads on one machine and should not be read as a general security or performance guarantee.
