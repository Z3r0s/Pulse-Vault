# Optional Authenticode sign. Does nothing unless a cert is configured.
# DNSPulse — https://dnspulse.org
#
#   $env:PULSE_VAULT_PFX = "C:\certs\dnspulse.pfx"
#   $env:PULSE_VAULT_PFX_PASSWORD = "..."
#   .\scripts\sign-windows.ps1 -Path ..\pulse-vault-gui.exe
#
# Or use a cert already in the store:
#   $env:PULSE_VAULT_CERT_THUMBPRINT = "AABBCC..."
#
# Unsigned Go exes often trip SmartScreen. Signing does not make a file
# "not a virus" — it names the publisher. Microsoft still needs reputation.
# See docs/TRUST.md.

param(
    [Parameter(Mandatory = $true)]
    [string]$Path
)

$ErrorActionPreference = "Stop"
if (-not (Test-Path $Path)) { throw "missing $Path" }

$signtool = Get-Command signtool -ErrorAction SilentlyContinue
if (-not $signtool) {
    $kits = "${env:ProgramFiles(x86)}\Windows Kits\10\bin"
    if (Test-Path $kits) {
        $found = Get-ChildItem -Path $kits -Recurse -Filter signtool.exe -ErrorAction SilentlyContinue |
            Where-Object { $_.FullName -match "x64" } |
            Select-Object -First 1
        if ($found) { $signtool = $found.FullName }
    }
}
if (-not $signtool) {
    Write-Host "signtool.exe not found. Install Windows SDK. Skipping sign for $Path"
    exit 0
}

$stamp = "http://timestamp.digicert.com"
if ($env:PULSE_VAULT_PFX) {
    $pwd = $env:PULSE_VAULT_PFX_PASSWORD
    Write-Host "Signing $Path with PFX"
    & $signtool sign /fd SHA256 /td SHA256 /tr $stamp /f $env:PULSE_VAULT_PFX /p $pwd $Path
    exit $LASTEXITCODE
}
if ($env:PULSE_VAULT_CERT_THUMBPRINT) {
    Write-Host "Signing $Path with store cert $($env:PULSE_VAULT_CERT_THUMBPRINT)"
    & $signtool sign /fd SHA256 /td SHA256 /tr $stamp /sha1 $env:PULSE_VAULT_CERT_THUMBPRINT $Path
    exit $LASTEXITCODE
}

Write-Host "No PULSE_VAULT_PFX or PULSE_VAULT_CERT_THUMBPRINT set. Left $Path unsigned."
exit 0
