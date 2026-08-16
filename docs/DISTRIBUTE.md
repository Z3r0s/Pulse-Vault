# Ship Pulse-Vault (GitHub Releases, pip, snap)

Product from **[DNSPulse](https://dnspulse.org)**. The thing users run is the **Go** CLI / GUI under [`gui-go/`](../gui-go/). pip and snap are *delivery* — they fetch or build that Go binary. They do not publish [`legacy/python/`](../legacy/python/).

## 1. GitHub Releases (do this first)

pip and the install scripts download `releases/latest`. Until a `v*` tag exists they will fail with “not in SHA256SUMS”.

1. Finish [`CHANGELOG.md`](../CHANGELOG.md): rename `[Unreleased]` to `[X.Y.Z] - YYYY-MM-DD`.
2. Add a matching `<release>` in [`packaging/linux/io.github.z3r0s.PulseVault.metainfo.xml`](../packaging/linux/io.github.z3r0s.PulseVault.metainfo.xml).
3. Commit on `main`.
4. Tag and push:

```bash
git tag -a v0.2.0 -m "Pulse-Vault 0.2.0"
git push origin v0.2.0
```

5. Watch [Actions → Release Go](https://github.com/Z3r0s/Pulse-Vault/actions). It builds CLI (windows/linux/darwin × amd64/arm64), host GUI, `SHA256SUMS`, and attaches them to the tag.

Users then (see [INSTALL.md](../INSTALL.md)):

```text
Windows app:  Releases → pulse-vault-gui-windows-amd64.exe
Windows CLI:  irm https://raw.githubusercontent.com/Z3r0s/Pulse-Vault/main/scripts/install.ps1 | iex
Linux/macOS:  curl -fsSL https://raw.githubusercontent.com/Z3r0s/Pulse-Vault/main/scripts/install.sh | sh
Clone, no tag:  scripts\install.cmd   or   ./scripts/install.sh --from-source
```

Manual dispatch: Actions → Release Go → Run workflow (no tag = artifacts only, no GitHub Release).

## 2. You, submitting to PyPI (pip)

Do this on your own machine after the GitHub Release exists. The package is [`packaging/pypi/`](../packaging/pypi/). It is a **launcher** that fetches the official Go CLI. It is not the retired Python vault.

### One-time (30 minutes)

1. Open <https://pypi.org/account/register/> and create an account. Use a real email.
2. Also register on <https://test.pypi.org/account/register/> (same idea, separate site).
3. On both: enable 2FA (Account settings → Two-factor).
4. On pypi.org: Account settings → API tokens → **Add API token**.
   - First token: scope “Entire account”. After the project exists, make a new token scoped to `pulse-vault` and delete the first one.
   - Copy the token once (`pypi-...`). You will not see it again.
5. Check the name: <https://pypi.org/project/pulse-vault/>
   - **404 / not found** → you can claim `pulse-vault`.
   - **Your old Python package** → upload this as a new version (0.2.0+) and say the interpreter vault is gone.
   - **Someone else owns it** → change `name` in `packaging/pypi/pyproject.toml` to e.g. `dnspulse-pulse-vault` and use that in `pip install`.

### Every release

```bash
python -m pip install --upgrade build twine
cd packaging/pypi
python -m build
python -m twine check dist/*
```

Dry run (Test PyPI):

```bash
python -m twine upload --repository testpypi dist/*
python -m pip install -i https://test.pypi.org/simple/ --extra-index-url https://pypi.org/simple pulse-vault
pulse-vault version
```

When that works, the real upload:

```bash
python -m twine upload dist/*
```

Username is `__token__`. Password is the API token (including the `pypi-` prefix).

Bump `version` in `packaging/pypi/pyproject.toml` and `packaging/pypi/src/pulse_vault/__init__.py` for each upload (PyPI will not replace 0.1.0). Current tree is **0.2.0**.

The tag that triggers CI must be lowercase `v0.2.0`. A GitHub Release named “V0.2.0” on tag `upload` does **not** attach CLI binaries, and pip will say so.

Users then run:

```bash
pip install -U pulse-vault
pulse-vault version
pulse-vault --launcher-info
```

Do **not** `pip install` [`legacy/python/`](../legacy/python/).

## 3. You, submitting to the Snap Store

Recipe: [`snap/snapcraft.yaml`](../snap/snapcraft.yaml). Builds the Go CLI. The GUI stays on GitHub Releases.

You need an Ubuntu machine or Multipass/LXD. Windows can do this in WSL **after** you install a distro (`wsl --install -d Ubuntu`).

### One-time

1. Create an account at <https://snapcraft.io> (Ubuntu One login).
2. On that Ubuntu/WSL box:

```bash
sudo snap install snapcraft --classic
sudo snap install lxd
sudo lxd init --auto
snapcraft login
snapcraft register pulse-vault
```

If `pulse-vault` is taken, change `name:` in `snap/snapcraft.yaml` and register that name instead.

### Every release (from the repo root)

```bash
cd /path/to/OpenSourceFileVault
# keep version: in snap/snapcraft.yaml in sync with the git tag
snapcraft
snapcraft upload --release=edge pulse-vault_0.1.0_amd64.snap
```

Local check before upload:

```bash
sudo snap install --dangerous ./pulse-vault_0.1.0_amd64.snap
pulse-vault version
sudo snap remove pulse-vault
```

Then in the [Snapcraft dashboard](https://snapcraft.io/pulse-vault/releases):

1. Confirm the revision landed on **edge**.
2. Promote edge → **beta** when you have tried it.
3. Promote beta → **candidate**, wait a day.
4. Promote candidate → **stable**. That is what `sudo snap install pulse-vault` hits.

Leave `grade: devel` until you are ready for stable, then change it to `stable` in `snapcraft.yaml` on the next upload.

Fill the store listing (icon, summary, dnspulse.org, license MIT) on the dashboard. Use `gui-go/assets/pulse-vault.png` as the icon.

## Order that actually works

1. Tag a GitHub Release so binaries and `SHA256SUMS` exist.
2. Then pip (the launcher needs `latest`).
3. Snap can build from source without a tag, but store users still expect a version that matches the tag.

Site copy and download buttons belong on [dnspulse.org](https://dnspulse.org) once the first `v*` tag is public. See [ROADMAP.md](ROADMAP.md).

Windows SmartScreen / “virus” flags: [TRUST.md](TRUST.md). Sign the Windows exes before you tell the world to download them.
