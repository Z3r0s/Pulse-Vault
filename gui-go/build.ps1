# Build the Pulse-Vault Go desktop GUI into a single native Windows-ready binary.
# Requires: Go 1.22+, CGO (MSYS2 mingw64 gcc on Windows).
#
# Usage (from gui-go/):
#   .\build.ps1
#   .\build.ps1 -Out ..\dist\pulse-vault-gui.exe -Version 1.2.3

param(
    [string]$Out = ".\pulse-vault-gui.exe",
    [string]$Version = "dev"
)

$ErrorActionPreference = "Stop"
$Root = $PSScriptRoot
Set-Location $Root

$mingw = "C:\msys64\mingw64\bin"
if (Test-Path "$mingw\gcc.exe") {
    $env:Path = "$mingw;" + $env:Path
    $env:CC = "gcc"
}
$env:Path = "$(go env GOPATH)\bin;" + $env:Path
$env:CGO_ENABLED = "1"

Write-Host "CGO_ENABLED=$env:CGO_ENABLED CC=$env:CC Version=$Version"

$syso = Join-Path $Root "resource_windows_amd64.syso"
$branded = $false
$onWindows = ($env:OS -match "Windows") -or ($PSVersionTable.Platform -eq "Win32NT") -or (-not $PSVersionTable.Platform)
if ($onWindows) {
    if (-not (Get-Command goversioninfo -ErrorAction SilentlyContinue)) {
        Write-Host "Installing goversioninfo for PE version/icon embed..."
        go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
        $env:Path = "$(go env GOPATH)\bin;" + $env:Path
    }
    if (Get-Command goversioninfo -ErrorAction SilentlyContinue) {
        Write-Host "Generating Windows version resource (ProductName=Pulse-Vault + icon + manifest)..."
        & goversioninfo -64 -o $syso -icon "assets\pulse-vault.ico" -manifest "app.manifest"
        if (Test-Path $syso) {
            $branded = $true
            Write-Host "  OK $syso ($((Get-Item $syso).Length) bytes)"
        } else {
            Write-Host "  WARN: syso not produced"
        }
    } else {
        Write-Host "  WARN: goversioninfo unavailable"
    }
}

# -H windowsgui = no console/PowerShell window when you double-click the exe
$ldflags = "-H windowsgui -s -w -X github.com/Z3r0s/Pulse-Vault/gui-go/internal/version.Version=$Version"
Write-Host "Building $Out ..."
go build -trimpath -buildvcs=false -ldflags $ldflags -o $Out .
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
$item = Get-Item $Out
Write-Host ("OK {0} ({1:N0} bytes) brandedPE={2} version={3}" -f $item.FullName, $item.Length, $branded, $Version)
$sign = Join-Path $Root "scripts\sign-windows.ps1"
if (Test-Path $sign) {
    & powershell -NoProfile -ExecutionPolicy Bypass -File $sign -Path $item.FullName
}
