#!/bin/bash

# PulseVault Installation Script for Parrot OS
# This script creates a dedicated Python virtual environment to safely install 
# dependencies without breaking Debian/Parrot OS's managed packages.

set -e

echo -e "\e[1;34m[\e[0m*\e[1;34m]\e[0m \e[1;32mStarting PulseVault Installation for Parrot OS...\e[0m"

# 1. Install necessary system dependencies
echo -e "\e[1;34m[\e[0m*\e[1;34m]\e[0m \e[1mInstalling required system packages (python3-venv, python3-tk)...\e[0m"
sudo apt-get update
sudo apt-get install -y python3-venv python3-tk python3-pip

# 2. Setup Virtual Environment
INSTALL_DIR="$HOME/.pulsevault"
echo -e "\e[1;34m[\e[0m*\e[1;34m]\e[0m \e[1mCreating secure virtual environment in $INSTALL_DIR...\e[0m"

if [ -d "$INSTALL_DIR" ]; then
    echo -e "\e[1;33m[!]\e[0m \e[1mExisting installation found. Cleaning up...\e[0m"
    rm -rf "$INSTALL_DIR"
fi

mkdir -p "$INSTALL_DIR"
python3 -m venv "$INSTALL_DIR/venv"

# 3. Copy project files to the installation directory
echo -e "\e[1;34m[\e[0m*\e[1;34m]\e[0m \e[1mCopying PulseVault core files...\e[0m"
cp -r core gui main.py requirements.txt "$INSTALL_DIR/"

# 4. Install Python dependencies inside the venv
echo -e "\e[1;34m[\e[0m*\e[1;34m]\e[0m \e[1mInstalling cryptographic libraries into the virtual environment...\e[0m"
source "$INSTALL_DIR/venv/bin/activate"
pip install --upgrade pip
pip install -r "$INSTALL_DIR/requirements.txt"
deactivate

# 5. Create a global launch wrapper
WRAPPER_PATH="/usr/local/bin/pulsevault"
echo -e "\e[1;34m[\e[0m*\e[1;34m]\e[0m \e[1mCreating global launch command ($WRAPPER_PATH)...\e[0m"

sudo bash -c "cat > $WRAPPER_PATH" << EOF
#!/bin/bash
# Wrapper to launch PulseVault using its virtual environment

# Suppress VMware 3D graphics warnings
export LIBGL_ALWAYS_SOFTWARE=1
export GALLIUM_DRIVER=llvmpipe

source "$INSTALL_DIR/venv/bin/activate"
python "$INSTALL_DIR/main.py" "\$@"
deactivate
EOF

sudo chmod +x "$WRAPPER_PATH"

# 6. Desktop Shortcut (Optional but helpful for Parrot GUI)
DESKTOP_FILE="$HOME/.local/share/applications/pulsevault.desktop"
echo -e "\e[1;34m[\e[0m*\e[1;34m]\e[0m \e[1mCreating Desktop Entry...\e[0m"
mkdir -p "$HOME/.local/share/applications"

cat > "$DESKTOP_FILE" << EOF
[Desktop Entry]
Version=2.0
Type=Application
Name=PulseVault
Comment=High-Security Cascading File Vault
Exec=$WRAPPER_PATH %f
Terminal=false
Categories=Utility;Security;
MimeType=application/x-pulsevault;
EOF

chmod +x "$DESKTOP_FILE"

echo -e "\n\e[1;32m[✓] Installation Complete!\e[0m"
echo -e "You can now launch PulseVault in two ways:"
echo -e "  1. Type \e[1;36mpulsevault\e[0m in your terminal."
echo -e "  2. Open it from your Parrot OS Applications menu."
echo -e "To open a vault directly from the terminal, run: \e[1;36mpulsevault /path/to/file.PulseVault\e[0m"
