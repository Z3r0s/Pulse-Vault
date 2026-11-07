# Roadmap

Pulse-Vault is a [DNSPulse](https://dnspulse.org) local encrypted file vault with a Go desktop app and CLI. Folder import and restore, integrity verification, password rotation, and media carriers are implemented. This list focuses on making the existing tool easier to trust and adopt; it is a direction, not a release promise.

## Near term

1. **Make first use obvious.** Add a short in-app first-run path for creating a vault, adding a folder, and explaining where the vault file is saved.
2. **Improve recovery guidance.** Explain that the password cannot be reset, encourage a separate copy of the vault, and make the restore path easy to find before users need it.
3. **Publish clear downloads.** Keep per-platform app/CLI assets, hashes, supported OS versions, and source-build steps together on the release page and website.
4. **Build trust in releases.** Sign Windows binaries, add macOS notarization if distributing the GUI there, publish an SBOM, and document how to verify checksums.
5. **Ask for independent review.** Have an external cryptography reviewer examine the protocol and implementation before making broader security claims or calling the format stable.

## Product work to evaluate

- **Unlock-time integrity check.** Offer an optional check after unlock and show a compact result, with a clear cost for large vaults.
- **Auto-lock.** Let users choose an idle timeout, while explaining that this only shortens the unlocked window; it does not protect against a compromised session.
- **Safer replacement workflow.** Improve progress and recovery messaging for slow operations, and make it clear when completed extracted files remain after cancellation.
- **Import/export review.** Gather user feedback on folder conflict behavior, overwrite prompts, and preserving paths across operating systems.
- **Search names.** Add filename search if users need it; keep content search out unless there is a clear local-only design that does not leave an index of plaintext behind.

## Keep out of scope unless the product changes

- Cloud accounts, sync, telemetry, or remote key custody. Pulse-Vault is local-only today.
- Claims that hiding a vault after media makes it undetectable.
- Secure deletion or anti-forensics promises.
- New cryptographic primitives without a format design, test vectors, compatibility plan, and independent review.

## Release readiness

- Windows GUI and CLI builds work from a clean checkout.
- Current and legacy formats have documented compatibility behavior and automated tamper/truncation coverage.
- Release downloads include checksums and a clear source link.
- Installation, password handling, backups, recovery limits, and known security boundaries are explained in plain language.
- Security contact and vulnerability handling instructions are easy to find.
