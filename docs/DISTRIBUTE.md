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

Users then:

```text
Windows:  irm https://raw.githubusercontent.com/Z3r0s/Pulse-Vault/main/scripts/install.ps1 | iex
Linux/macOS:  curl -fsSL https://raw.githubusercontent.com/Z3r0s/Pulse-Vault/main/scripts/install.sh | sh
Or click:     https://github.com/Z3r0s/Pulse-Vault/releases
```

Manual dispatch: Actions → Release Go → Run workflow (no tag = artifacts only, no GitHub Release).

## 2. pip (PyPI)

The package lives in [`packaging/pypi/`](../packaging/pypi/). It is a launcher: first `pulse-vault` call downloads the matching Go asset and checks SHA-256.

### One-time PyPI setup

1. Create an account on [pypi.org](https://pypi.org) (and [test.pypi.org](https://test.pypi.org)).
2. Enable 2FA.
3. Create an **API token** (scope: entire account the first time, then lock it to `pulse-vault`).
4. Check the name: <https://pypi.org/project/pulse-vault/>
   - Empty → you can claim `pulse-vault`.
   - Occupied by someone else → change `name` in `packaging/pypi/pyproject.toml` (e.g. `dnspulse-pulse-vault`) and the `pip install` lines in the README.
   - Occupied by *your* old Python vault → upload this as a **new version** (0.2.0+) and say in the long description that the interpreter vault is gone.

### Build and upload

```bash
python -m pip install --upgrade build twine
cd packaging/pypi
python -m build
python -m twine check dist/*

# dry run
python -m twine upload --repository testpypi dist/*

# real
python -m twine upload dist/*
```

Twine will ask for username `__token__` and the token as the password. Or set `TWINE_USERNAME` / `TWINE_PASSWORD`.

Test:

```bash
python -m pip install -i https://test.pypi.org/simple/ pulse-vault
pulse-vault version
```

Users (after the GitHub Release exists):

```bash
pip install pulse-vault
pulse-vault version
```

CI already runs the launcher unit tests (no network). Do not point pip at `legacy/python/`.

## 3. snap (Snap Store)

[`snap/snapcraft.yaml`](../snap/snapcraft.yaml) builds the Go CLI with `CGO_ENABLED=0`. Strict confinement, `home` + `removable-media`.

### One-time Snapcraft setup

1. Account: <https://snapcraft.io>
2. Register the name (once):

```bash
sudo snap install snapcraft --classic
snapcraft login
snapcraft register pulse-vault
```

If `pulse-vault` is taken, change `name:` in `snap/snapcraft.yaml` and register that.

### Build and upload

On Ubuntu (or a Multipass/LXD box):

```bash
cd /path/to/OpenSourceFileVault
snapcraft
snapcraft upload --release=edge pulse-vault_0.1.0_amd64.snap
```

`grade: devel` is correct until you want `stable`. Promote in the Snapcraft dashboard: edge → beta → candidate → stable.

Local try:

```bash
sudo snap install --dangerous pulse-vault_0.1.0_amd64.snap
pulse-vault version
```

The GUI is **not** in the snap (Fyne + OpenGL + strict confinement is a second project). Point GUI users at GitHub Releases.

## Order that actually works

1. Tag a GitHub Release so binaries and `SHA256SUMS` exist.
2. Then pip (the launcher needs `latest`).
3. Snap can build from source without a tag, but store users still expect a version that matches the tag.

Site copy and download buttons belong on [dnspulse.org](https://dnspulse.org) once the first `v*` tag is public. See [ROADMAP.md](ROADMAP.md).
