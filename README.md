# PulseVault

PulseVault is a local encrypted file vault desktop app by DNSPulse. It stores files in a portable vault container, keeps operations offline, and streams large files so they do not need to be loaded fully into memory.

## Features

- **Local-only design:** No telemetry, cloud service, API keys, or network dependency.
- **Streaming file encryption:** Large files are processed in chunks and stored as encrypted entries inside the vault.
- **Scrypt key derivation:** Passwords are converted into encryption keys with a memory-hard KDF.
- **Cascade encryption:** File data is encrypted through ChaCha20-Poly1305 and AES-GCM.
- **Compression:** V5 vault entries use LZMA compression before encryption.
- **Password changes:** Existing file entries are re-encrypted when the vault password changes.
- **Optional carrier files:** A vault can be appended to a PNG, JPG, or MP4 carrier file for casual disguise.
- **Desktop UI:** Built with CustomTkinter.

## Quick Start

```bash
git clone https://github.com/z3r0s/pulsevault.git
cd pulsevault
pip install -r requirements.txt
python main.py
```

## Parrot OS Native Installer

PulseVault includes a wrapper script that installs dependencies in a dedicated virtual environment.

```bash
chmod +x install_parrot.sh
./install_parrot.sh
pulsevault
```

## Build a Debian Package

```bash
sudo apt install python3-stdeb fakeroot python3-all
python3 setup.py --command-packages=stdeb.command bdist_deb
sudo dpkg -i deb_dist/python3-pulsevault_*.deb
```

## Security Notes

- PulseVault depends on the strength and secrecy of your password.
- Generated passwords are shown in the UI but are not saved for you.
- Secure Open extracts plaintext files to a temporary app directory before launching them. The directory is removed when the app exits normally, but external viewers may create caches, thumbnails, recent-file records, or other artifacts.
- Carrier-file support appends vault data after the carrier. This can be useful for casual disguise, but it is not forensic protection.
- Keep backups of important vaults. Corruption or password loss can make contents unrecoverable.

## Current Vault Format

- **KDF:** Scrypt
- **File encryption:** ChaCha20-Poly1305 followed by AES-GCM
- **Compression:** LZMA/XZ before encryption
- **Container:** ZIP with encrypted metadata and encrypted `data/*.enc` entries
- **Current marker:** `PULSEVAULT5_COMPRESSED_CASCADE`

## License

MIT License. Created by DNSPulse.
