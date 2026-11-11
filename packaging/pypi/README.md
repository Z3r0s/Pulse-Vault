# pulse-vault (PyPI)

Official [DNSPulse](https://dnspulse.org) installer for **Pulse-Vault**.

`pip install pulse-vault` is **not** the old Python vault. It is a small launcher for the official **Go** CLI. After installation, ordinary commands use a locally cached binary and make no GitHub request. If no binary is installed yet, choose the explicit download action:

```bash
pip install -U pulse-vault
pulse-vault --launcher-install  # explicitly download and verify the Go CLI
pulse-vault version
pulse-vault create demo.pulsevault --password 'choose-a-strong-password'
```

```text
pulse-vault --launcher-info      # cached path, tag, sha256
pulse-vault --launcher-install   # explicitly fetch the CLI if needed
pulse-vault --launcher-update    # explicitly fetch the latest CLI again
pulse-vault --launcher-which
pulse-vault --launcher-help
```

Normal CLI use stays offline. The explicit install or update command contacts GitHub, which can see the connection's source IP and request time.

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
