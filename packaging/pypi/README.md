# pulse-vault (PyPI)

Official [DNSPulse](https://dnspulse.org) installer for **Pulse-Vault**.

`pip install pulse-vault` is **not** the old Python vault. It is a small launcher. On first run it downloads the official **Go** CLI from [GitHub Releases](https://github.com/Z3r0s/Pulse-Vault/releases), checks `SHA256SUMS`, then runs that binary.

```bash
pip install -U pulse-vault
pulse-vault version
pulse-vault create demo.pulsevault --password 'choose-a-strong-password'
```

```text
pulse-vault --launcher-info      # cached path, tag, sha256
pulse-vault --launcher-update    # fetch the latest CLI again
pulse-vault --launcher-which
pulse-vault --launcher-help
```

Pin a release:

```bash
set PULSE_VAULT_VERSION=v0.2.0
export PULSE_VAULT_VERSION=v0.2.0
```

Use a binary you already built:

```bash
set PULSE_VAULT_BIN=C:\path\to\pulse-vault.exe
export PULSE_VAULT_BIN=/usr/local/bin/pulse-vault
```

The Go CLI must be attached to the GitHub Release (`pulse-vault-windows-amd64.exe` and friends, plus `SHA256SUMS`). A source-only tag is not enough.

Site: [dnspulse.org](https://dnspulse.org) · Source: [github.com/Z3r0s/Pulse-Vault](https://github.com/Z3r0s/Pulse-Vault)
