# Where Pulse-Vault can go

A product from **[DNSPulse](https://dnspulse.org)**. This is a recommended order, not a promise.

Pulse-Vault is already a real local vault: V6 finalized format with V5 read compatibility, Go CLI + Fyne GUI, hide-in-picture, checksummed releases. “Full fledged” is distribution, trust, and a few product edges — not a rewrite.

## Do next (makes it a product people can install)

1. **Cut the first `v*` tag.** pip, snap, and the install scripts all look at `releases/latest`. Until that exists, “just download it” is a local `go build`. Walkthrough: [DISTRIBUTE.md](DISTRIBUTE.md).
2. **Put downloads on [dnspulse.org](https://dnspulse.org).** Three buttons (Windows GUI, Windows CLI, checksums), plus Linux/macOS. Same files GitHub already attaches. The site is the brand; GitHub is the file host.
3. **Screenshots.** AppStream / Snap / the site still point at missing `screenshots/*.png`. One locked hero, one unlocked list, one hide-in-picture. Host them in-repo or on the site.
4. **Code signing.** Windows Authenticode so SmartScreen stops training people to click through. Apple notarization if you care about macOS GUI. This is the #1 “feels legit” item after a tag.

## Trust and review

5. **Reproducible CLI builds.** Document `CGO_ENABLED=0 go build -trimpath -buildvcs=false` and publish a short “build it yourself, hashes match” page on dnspulse.org.
6. **SBOM + provenance.** `govulncheck` in CI, a CycloneDX/SPDX file on the release, later SLSA provenance from the release workflow.
7. **Independent look at the format.** You already have a threat model and vectors. Pay for a short review of `docs/CRYPTO_PROTOCOL.md` + `gui-go/crypto` before calling it 1.0. Do not add a new cipher to look busy.

## Packaging after pip + snap

8. **Homebrew tap** (`brew install z3r0s/tap/pulse-vault`) — macOS/Linux CLI users expect this more than snap.
9. **winget** manifest once the exe is signed.
10. **Flatpak** if you want GNOME Software. Reuse the AppStream file. Harder than snap for Fyne.
11. **Microsoft Store / Mac App Store** last. Sandbox + GUI + local files is a month of work for little security gain.

## Product features worth building

12. **Folder add / extract.** People think in folders. Stream it; do not load the tree.
13. **Keyfile + password.** Optional second factor that is just more KDF input. Format bump or a metadata flag — design it before coding.
14. **Verify on open.** You have `verify`. Run it after unlock and show a single line, not a dialog novel.
15. **Auto-lock.** Timer after unlock. Memory still had the key; this just shortens the window.
16. **Watch-one-folder (optional, off by default).** “Drop files here, they go in the vault.” Easy to get wrong (sync loops, leftovers). Only if users ask.
17. **Search names, not contents.** Searching plaintext inside the vault means decrypting everything. Name search is enough.

## Features to refuse (or park)

- Cloud sync, accounts, telemetry. That is a different product.
- Homemade ciphers, “military grade” copy, steganography that claims to hide from a lab.
- Wiping free space / shadow copies / “anti-forensics.” We already drop host paths and ZIP timestamps. Stay there.
- Mobile apps until the desktop install story is boring.
- Rewriting the vault in Rust for the blog post.

## 1.0 bar (suggested)

- Tagged release with CLI + GUI + `SHA256SUMS`
- pip launcher and snap edge both install that same CLI
- dnspulse.org download page
- Signed Windows GUI
- Screenshots
- `govulncheck` clean
- Threat model and protocol doc match the bits

Until then, stay honest: this is a solid 0.x local vault from DNSPulse, not a platform.
