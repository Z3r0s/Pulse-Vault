#!/bin/bash

# Pulse-Vault installer for Parrot OS / Debian-style desktops.
# Installs the app in an isolated virtual environment and registers a launcher.

set -e

APP_NAME="Pulse-Vault"
INSTALL_DIR="$HOME/.local/share/pulse-vault"
BIN_DIR="$HOME/.local/bin"
WRAPPER_PATH="$BIN_DIR/pulse-vault"
DESKTOP_FILE="$HOME/.local/share/applications/pulse-vault.desktop"
MIME_DIR="$HOME/.local/share/mime/packages"
ICON_DIR="$HOME/.local/share/icons/hicolor/256x256/apps"

echo "[*] Installing $APP_NAME..."

echo "[*] Installing system dependencies..."
sudo apt-get update
sudo apt-get install -y python3-venv python3-tk python3-pip

echo "[*] Creating virtual environment at $INSTALL_DIR..."
rm -rf "$INSTALL_DIR"
mkdir -p "$INSTALL_DIR" "$BIN_DIR" "$HOME/.local/share/applications" "$MIME_DIR" "$ICON_DIR"
python3 -m venv "$INSTALL_DIR/venv"

echo "[*] Installing Pulse-Vault into the virtual environment..."
"$INSTALL_DIR/venv/bin/pip" install --upgrade pip
"$INSTALL_DIR/venv/bin/pip" install .

cat > "$WRAPPER_PATH" << EOF
#!/bin/bash
export LIBGL_ALWAYS_SOFTWARE=1
export GALLIUM_DRIVER=llvmpipe
exec "$INSTALL_DIR/venv/bin/pulse-vault" "\$@"
EOF
chmod +x "$WRAPPER_PATH"

cp packaging/linux/pulse-vault.desktop "$DESKTOP_FILE"
cp packaging/linux/application-x-pulsevault.xml "$MIME_DIR/application-x-pulsevault.xml"
cp src/pulsevault/assets/pulse-vault.png "$ICON_DIR/pulse-vault.png"

if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database "$HOME/.local/share/applications" || true
fi

if command -v update-mime-database >/dev/null 2>&1; then
  update-mime-database "$HOME/.local/share/mime" || true
fi

echo "[+] Installation complete."
echo "    Run: pulse-vault"
echo "    Open a vault: pulse-vault /path/to/file.pulsevault"
