# Pulse-Vault threat model

This describes the security goals and limits of the Go application and its public Go crypto package. The archived Python code is not part of the current product.

## Security goals

- Protect stored file contents from someone who has the vault file but not its password, subject to the password's strength and the selected Scrypt cost.
- Accept only persisted Scrypt parameter sets that match a supported profile; the low-cost profile is test-only and is not accepted by normal builds.
- Detect modification, truncation, reordering, or substitution of authenticated metadata and V6 stream records.
- Use random base nonces and authenticated chunk indexes to separate stream records.
- Keep V5 and older supported data readable without treating legacy streams as if they had V6's final-record guarantee.
- Avoid exposing imported plaintext through password-rotation staging files. Adds and password rotation stage encrypted blobs before replacing the original container.

## Assumptions

- The password is sufficiently strong, kept private, and not reused elsewhere.
- The password-derived key is only as resistant to offline guessing as the password and recorded KDF cost. Standard uses a 32 MiB Scrypt memory term; hardened uses 1 GiB. Neither profile makes a weak or reused password safe.
- KDF profile fields are visible and attacker-controlled before unlock. The Go reader allowlists the supported parameter tuples and rejects weak custom tuples or a profile label that disagrees with its parameters.
- The operating system, Go runtime, and release artifact have not been compromised.
- Operating-system randomness is functioning.
- Users protect the computer and the unlocked session.
- Users treat extracted files as plaintext and keep independent backups of vaults they need.

## What Pulse-Vault does not protect

- Files after they have been extracted, or originals left at the source location.
- A compromised account, malware, keylogging, or an attacker with access while the vault is unlocked.
- Password recovery or offline guessing of a weak password.
- Secure deletion from disks, swap, backups, filesystem snapshots, or crash dumps.
- Metadata leakage such as approximate stored sizes, visible KDF parameters, or the presence of a vault appended to media.
- Detection of every possible filesystem or operating-system side channel.
- Network privacy if a user chooses the optional update check. GitHub sees the source IP and request time; a chosen release asset can disclose the requested platform. A VPN or configured HTTPS proxy changes the network route but does not hide the request from the VPN or proxy operator.

An encrypted vault and the application that reads it remain inspectable. Someone with the file can examine its container structure, count encrypted entries, and estimate their sizes; someone with the binary can disassemble or debug that program. Release builds strip symbol and DWARF debug tables, omit local source paths, and disable embedded VCS build metadata to reduce incidental build information and add modest analysis friction. A local application cannot make reverse engineering impossible. Obfuscation and anti-debugging do not replace encryption and are not part of Pulse-Vault's security guarantees. The security goal is to protect file contents and authenticate metadata when the password is kept secret.

Appending a vault to a picture or video is not cryptographic hiding. It keeps the media prefix intact for ordinary viewing, but a reviewer can inspect the appended ZIP data.

## Format notes

V6 streams authenticate data records and require a final authenticated record, so clean-boundary truncation is detected. V5 streams remain readable for compatibility but do not have that V6 final-record guarantee. V7 adds validated relative paths to encrypted metadata and can retain explicitly marked V5 streams when upgrading a legacy vault. Password rotation rewrites streams as V6 while preserving V7 paths.

Extraction writes plaintext to a temporary file under the chosen output directory, verifies authentication and the recorded digest, then renames it into place. This limits exposure of incomplete files but does not provide secure deletion guarantees. Extract-all preflights target conflicts; if a later stream is corrupt or the user cancels, already completed outputs remain on disk.

## Requirements for protocol changes

Changes to key derivation, nonce construction, associated data, compression order, record framing, metadata paths, or legacy dispatch should include:

1. Updated [cryptographic protocol](CRYPTO_PROTOCOL.md) and [container format](VAULT_FORMAT.md) documentation.
2. Stable success and failure vectors for public protocol behavior.
3. Round-trip, wrong-key, tamper, truncation, path-safety, and resource-limit tests as applicable.
4. An explicit format marker or migration rule when the wire format changes.
5. Independent cryptographic review before making claims beyond the guarantees described here.
