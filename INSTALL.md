# Installation

Detailed instructions for installing Pulse-Vault from source.

**Note:** Packaged downloads (binaries, etc.) are available on [GitHub Releases](https://github.com/Z3r0s/Pulse-Vault/releases) (once published) and planned for [dnspulse.org](https://dnspulse.org) toward 1.0. See [docs/DOWNLOADS.md](docs/DOWNLOADS.md) for more.

## From source (current method)

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

This makes Pulse-Vault suitable for headless use and future `apt`/`snap`/`pip` packaging.

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

Desktop metadata lives under `packaging/linux/`:

- desktop launcher
- MIME type registration
- AppStream metadata

The intended command name is:

```bash
pulse-vault
```

The intended vault extension is:

```text
.pulsevault
```

Legacy `.PulseVault` files remain supported. Opening one prompts before it is renamed.
