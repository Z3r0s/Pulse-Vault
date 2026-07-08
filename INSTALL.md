# Installation

Detailed instructions for installing Pulse-Vault from source.

**Note:** Packaged downloads (binaries, etc.) are available on [GitHub Releases](https://github.com/Z3r0s/Pulse-Vault/releases) (once published) and planned for [dnspulse.org](https://dnspulse.org) toward 1.0. See [docs/DOWNLOADS.md](docs/DOWNLOADS.md) for more.

## From source (current method)

### pip install (recommended for development & future packaging)

After cloning:

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -e .
```

This registers the entry points:
- `pulse-vault` (GUI + CLI via --cli)
- `pulsevault`
- `pulse-vault-cli` (dedicated guided CLI, packaging friendly)

Test:
```bash
pulse-vault --version
pulse-vault --help
pulse-vault-cli --help
pulse-vault --cli create demo.pulsevault
```

See "Desktop Integration" below for full Linux desktop launcher + MIME.

### Linux (Ubuntu, Debian, Parrot, etc.)

Pulse-Vault uses CustomTkinter, which needs Python's `tkinter` module. On Linux,
`tkinter` is **not** installed by pip — it comes from a system package that must
match the Python version used for your virtual environment.

Install system packages first:

```bash
sudo apt update
sudo apt install python3-venv python3-pip python3-tk
```

If your default `python3` is not the version you use for the venv (for example
Python 3.14 from a PPA), install the matching `-tk` package as well:

```bash
python3 --version
# Example output: Python 3.14.x
sudo apt install python3.14-tk
```

Verify `tkinter` works **before** creating the venv:

```bash
python3 -c "import tkinter; print('tkinter OK')"
```

If that command fails, Pulse-Vault will not start until the correct `-tk` package
is installed for that exact Python version.

Then clone, install, and run:

```bash
git clone https://github.com/Z3r0s/Pulse-Vault.git
cd Pulse-Vault
python3 -m venv .venv
source .venv/bin/activate
pip install -e .
python -c "import tkinter; print('tkinter OK in venv')"
pulse-vault
```

If you installed `python3-tk` after creating the venv and still see
`ModuleNotFoundError: No module named 'tkinter'`, recreate the venv:

```bash
rm -rf .venv
python3 -m venv .venv
source .venv/bin/activate
pip install -e .
pulse-vault
```

### Windows / macOS

```bash
git clone https://github.com/Z3r0s/Pulse-Vault.git
cd Pulse-Vault
python3 -m venv .venv
source .venv/bin/activate   # Windows: .venv\Scripts\activate
pip install -e .
pulse-vault
```

After `pip install -e .` you also get a guided CLI (no GUI required):

```bash
pulse-vault --help
pulse-vault --cli --help
pulse-vault --cli create my.vault
pulse-vault --cli open my.vault     # then interactive menu: list / add / extract / verify...
pulse-vault-cli create my.vault     # dedicated CLI entry point
```

For the full GUI experience use the optional extra:

```bash
pip install "pulse-vault[gui]"
```

This makes Pulse-Vault suitable for headless use and future `apt`/`snap`/`pip` packaging. The core package has no GUI dependencies, so `pulse-vault-cli` and `pulse-vault --cli` work in minimal environments.

### Development shortcut

```bash
pip install -r requirements.txt
python main.py
```

### On Parrot OS or other Debian-style desktops

```bash
chmod +x install_parrot.sh
./install_parrot.sh
```

That installs into `~/.local/share/pulse-vault`, adds a `pulse-vault` command,
registers the `.pulsevault` MIME type, and installs the desktop launcher.

### Desktop Integration (Linux)

Desktop metadata lives under `packaging/linux/` (used by install_parrot.sh, future .deb, Snap, Flatpak, and Ubuntu App Store):

- `pulse-vault.desktop` — launcher (Name, Exec, Icon, MimeType, Categories)
- `application-x-pulsevault.xml` — MIME type for `.pulsevault` (and legacy)
- `io.github.z3r0s.PulseVault.metainfo.xml` — AppStream metadata (for GNOME Software, Ubuntu App Store, appstreamcli, etc.)

The intended command name is:

```bash
pulse-vault
```

The intended vault extension is:

```text
.pulsevault
```

Legacy `.PulseVault` files remain supported. Opening one prompts before it is renamed.

After manual desktop registration (or via install_parrot.sh):

```bash
# Update caches (usually done by install script)
update-desktop-database ~/.local/share/applications || true
update-mime-database ~/.local/share/mime || true
# For system-wide: use sudo and /usr/local paths + xdg commands
```

### Ubuntu App Store / AppStream Readiness

The metainfo is production-ready for Ubuntu App Store / GNOME Software (AppStream):

- Detailed long description, keywords, categories, OARS content rating (all none)
- Multiple releases with descriptions (synced from CHANGELOG)
- Provides binaries (pulse-vault, pulsevault, pulse-vault-cli)
- URLs for homepage, bugtracker, vcs, help
- Screenshots section present (placeholders — **user must add real screenshot images** hosted publicly and update URLs before store submission)

**To validate locally (Ubuntu):**
```bash
sudo apt install -y desktop-file-utils appstream-util
desktop-file-validate packaging/linux/pulse-vault.desktop
appstream-util validate-relax --nonet packaging/linux/io.github.z3r0s.PulseVault.metainfo.xml
```

See CI (`.github/workflows/test.yml`) for automated check on PRs.

**Remaining for store submission:** real screenshots (see metainfo comments), any store-specific assets/screenshots (e.g. 1280x720 banner), and publishing account on Ubuntu App Store / Snap Store.

### Snap / Snapcraft Preparation

Pulse-Vault is designed to be Snap-friendly (CLI entries, metainfo, desktop file, assets package data).

**Suggestions / hints for snapcraft.yaml (create in repo root when ready):**

```yaml
name: pulse-vault
base: core22  # or core24
version: git  # or match __version__
summary: Local encrypted file vault
description: |
  ...
grade: stable
confinement: strict

apps:
  pulse-vault:
    command: bin/pulse-vault
    desktop: usr/share/applications/pulse-vault.desktop  # or snap/gui/
    common-id: io.github.z3r0s.PulseVault
    plugs: [home, removable-media, x11, wayland, opengl, network-bind]  # minimal; review for offline

parts:
  pulse-vault:
    plugin: python
    source: .
    python-packages: [cryptography, customtkinter, tkinterdnd2]
    build-packages: [python3-tk]  # system tk
    stage-packages: [python3-tk]
    # Use the packaging/linux/ files:
    # organize or override to place desktop + metainfo + icon + mime
    # e.g. snap/gui/pulse-vault.desktop , snap/gui/io.github...metainfo.xml
    # Include icons in hicolor from src/pulsevault/assets or scaled versions.

  # For icons: stage multiple sizes or use icon: src/pulsevault/assets/pulse-vault.png
```

**Tips:**
- Use `snapcraft` with `adopt-info` from metainfo for version/summary.
- Copy `packaging/linux/pulse-vault.desktop`, MIME xml, and metainfo into `snap/gui/` or organize in part.
- Provide icon at 256x256+ (and scalable if possible) in `snap/gui/`.
- Test with `snapcraft` + `snap try` + `snap run pulse-vault`.
- For strict confinement, may need additional plugs for file access (home, etc.) and GUI (x11/wayland).
- CI hint: add a snap build job later using snapcraft action.
- The existing `pulse-vault-cli` entry point is ideal for snap command variants.

See also packaging/linux/ files and pyproject.toml for entry points / package-data.

### Releases, Binaries, and Antivirus / Malware Scanner Notes

GitHub Releases (triggered on `v*` tags) provide:
- Source + wheels (via `python -m build`)
- Standalone binaries for Linux, Windows, and macOS (via PyInstaller in the release workflow)
- SHA256 checksums (`SHA256SUMS.txt`)
- The latest security property/fuzz report

**Windows .exe false positives (very common with Python apps)**

Executables built with PyInstaller (like the ones we ship) often get flagged by Windows Defender, VirusTotal, and other antivirus tools. This is a well-known false positive for legitimate Python desktop apps. It happens because:

- The Python interpreter and all dependencies are packed into one file.
- Antivirus heuristics flag "unknown" self-extracting or bundled executables.
- Things that look like packers (even when they're not) trigger warnings.

**How we reduce the noise:**
- We build with `--clean --noupx` (no UPX compression, which is a big trigger for scanners).
- We include proper version info so the .exe identifies as "DNSPulse / Pulse-Vault".
- Every release ships SHA256SUMS.txt for easy verification.
- The full source is public — you can (and are encouraged to) build it yourself.

**How to check a binary is safe:**
1. Compare the SHA256 hash against the one in `SHA256SUMS.txt` from the same GitHub Release.
2. Upload to VirusTotal — look at the number of engines that flag it (usually very few, and they are heuristic-based).
3. On Windows, right-click the .exe → Properties and look at the Details tab (you should see our company and product name).
4. Build it yourself from the tagged source using the instructions in `packaging/build-binaries.ps1`.
5. If Defender quarantines it, you can restore the file after you've verified the hash.

We ship no obfuscation, no telemetry, and no tricks. Everything is reproducible from the public repo. If something still looks suspicious on a particular scanner, open an issue with the hash and we'll look into it.

For the most trusted experience, build from source or use the pip/CLI version (no exe needed).

### Desktop Integration (Linux)

### Future App Stores

GitHub Releases currently provide Linux/Windows binaries (via release workflow). Planned primary hosting on dnspulse.org toward 1.0. Ubuntu App Store / Snap Store submissions will use the expanded AppStream metainfo.
