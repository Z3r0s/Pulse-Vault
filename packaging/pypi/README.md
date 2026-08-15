# pulse-vault (PyPI)

**This is not the retired Python vault.**

`pip install pulse-vault` installs a small launcher from [DNSPulse](https://dnspulse.org). On first run it downloads the official **Go** CLI from [GitHub Releases](https://github.com/Z3r0s/Pulse-Vault/releases), checks `SHA256SUMS`, and execs that binary.

```bash
pip install pulse-vault
pulse-vault version
pulse-vault create demo.pulsevault --password 'choose-a-strong-password'
```

Override the binary if you already built it:

```bash
set PULSE_VAULT_BIN=C:\path\to\pulse-vault.exe   # Windows
export PULSE_VAULT_BIN=/usr/local/bin/pulse-vault
```

Do **not** install [`legacy/python/`](../../legacy/python/). That tree is an archive.

Publish walkthrough: [docs/DISTRIBUTE.md](../../docs/DISTRIBUTE.md).
