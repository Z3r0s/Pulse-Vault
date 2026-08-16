# Trust, SmartScreen, and “is this a virus?”

A **[DNSPulse](https://dnspulse.org)** product. Short version: **nobody can make Windows promise it will never flag an unsigned `.exe`.** We can make the file look like a real product and you can **sign** it. After that, Microsoft still builds reputation over time.

Pulse-Vault does not contain malware. False positives happen because:

- new publisher / first-seen hash
- no Authenticode signature
- Go binaries share a runtime that some heuristics lump together
- `irm | iex` is a pattern malware uses, so Defender treats it as hostile even when the script is ours

## What we already do

- Company / product / copyright / icon in the PE resource (`gui-go/versioninfo.json`)
- Windows compatibility manifest (`gui-go/app.manifest`) — `asInvoker`, no admin prompt
- `-trimpath -buildvcs=false` so the binary does not embed your home path or git identity
- No UPX / no packer (packers get you boxed as malware)
- SHA-256 on GitHub Releases (`SHA256SUMS`); install scripts check that hash
- Optional Authenticode: [`gui-go/scripts/sign-windows.ps1`](../gui-go/scripts/sign-windows.ps1)

## What you (the publisher) must do

1. **Buy a code-signing certificate** in the DNSPulse / your legal name.
   - Regular OV Authenticode: users still see SmartScreen until the cert has reputation.
   - **EV Authenticode**: SmartScreen reputation starts much faster. This is the one you want if you can afford it.
   - Vendors: DigiCert, Sectigo, SSL.com. Needs a company or identity check. Budget weeks.
2. **Sign every Windows release exe** (CLI + GUI) with SHA-256 and a timestamp:

   ```powershell
   $env:PULSE_VAULT_PFX = "C:\certs\dnspulse.pfx"
   $env:PULSE_VAULT_PFX_PASSWORD = "..."
   cd gui-go
   .\build.ps1 -Out ..\dist\pulse-vault-gui.exe -Version 0.2.0
   .\scripts\sign-windows.ps1 -Path ..\dist\pulse-vault-gui.exe
   ```

   `build.ps1` also calls the signer if those env vars are set.
3. **Publish a `v*` GitHub Release** so people download *that* file, not a random build from a chat.
4. **Submit the signed files to Microsoft** (do this once per new major hash):
   - <https://www.microsoft.com/en-us/wdsi/filesubmission>
   - Pick “Software developer” → “I believe this file is incorrectly detected”
   - Attach the signed exe + a sentence: local encrypted vault from DNSPulse, source https://github.com/Z3r0s/Pulse-Vault, site https://dnspulse.org
5. **Optional:** upload the same signed files to [VirusTotal](https://www.virustotal.com) *after* signing. Unsigned first-seen Go bins collect junk detections; don’t panic, don’t “fix” crypto to please an AV.

## What users should install

Prefer, in this order:

1. GitHub Release exe (signed, once you have a cert)
2. `.\cli.ps1` / `./cli.sh` from a clone they fetched themselves
3. `go install github.com/Z3r0s/Pulse-Vault/gui-go/cmd/pulse-vault@v0.2.0`
4. `pip install pulse-vault` after you publish it (downloads the Release CLI)

Avoid advertising `irm ... | iex` as the main Windows path. It works, but Defender is trained to hate it.

## What this is not

We will not add Defender exclusions, disable AMSI, or hide the binary. That *is* malware behavior and would make the flags worse.

Ship path: [DISTRIBUTE.md](DISTRIBUTE.md).
