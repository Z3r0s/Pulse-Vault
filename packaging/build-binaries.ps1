# Local build script for Pulse-Vault binaries (Windows example)
# Run from repo root in PowerShell: .\packaging\build-binaries.ps1

$ErrorActionPreference = "Stop"

Write-Host "Installing PyInstaller and deps..."
python -m pip install --upgrade pip pyinstaller
pip install -e '.[dev]'

Write-Host "Building Windows exe with DNSPulse metadata (using --clean --noupx to minimize AV false positives)..."
pyinstaller --onefile --windowed --name pulse-vault `
  --add-data "src/pulsevault/assets; pulsevault/assets" `
  --hidden-import customtkinter `
  --hidden-import tkinterdnd2 `
  --clean --noupx `
  --version-file packaging/windows/version.txt `
  src/pulsevault/main.py

Write-Host "Build complete. Output in dist/pulse-vault.exe"
Write-Host "File properties should show Company: DNSPulse, Product: Pulse-Vault"

Write-Host "Running quick verification script..."
python packaging/verify-build.py --check-version --cli-smoke

Write-Host "Note: PyInstaller builds can trigger false positives in Windows Defender / AV. See INSTALL.md for verification steps and how to build from source yourself."

# For signing (requires code signing cert):
# signtool sign /f your-cert.pfx /p your-password /t http://timestamp.sectigo.com /v dist/pulse-vault.exe
# Then the exe will be Authenticode signed as DNSPulse.

# Note: For production, purchase a proper code signing certificate from a CA (e.g. Sectigo, DigiCert).
# EV cert recommended to bypass Windows SmartScreen more easily.