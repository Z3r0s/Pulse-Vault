# Install Pulse-Vault from GitHub Releases, or build it from this clone.
# DNSPulse — https://dnspulse.org
#
#   irm https://raw.githubusercontent.com/Z3r0s/Pulse-Vault/main/scripts/install.ps1 | iex
#   irm ... | iex   then  $env:PULSE_VAULT_GUI=1 for the desktop app too
#
# From a clone (works with no GitHub tag):
#   .\scripts\install.ps1 -Gui -FromSource

[CmdletBinding()]
param(
    [switch]$WithGui,
    [Alias("Gui")]
    [switch]$InstallGui,
    [switch]$FromSource,
    [switch]$SkipPath,
    [Alias("Dir")]
    [string]$InstallDir = ""
)

$ErrorActionPreference = "Stop"
$Repo = "Z3r0s/Pulse-Vault"
$Site = "https://dnspulse.org"
$Releases = "https://github.com/$Repo/releases/latest"

if (-not $InstallDir) {
    $InstallDir = Join-Path $env:LOCALAPPDATA "Pulse-Vault"
}
if ($env:PULSE_VAULT_GUI -eq "1") { $WithGui = $true }
if ($InstallGui) { $WithGui = $true }
if ($env:PULSE_VAULT_FROM_SOURCE -eq "1") { $FromSource = $true }

function Get-RepoRoot {
    if ($PSScriptRoot) {
        $cand = Split-Path -Parent $PSScriptRoot
        if (Test-Path (Join-Path $cand "gui-go\cmd\pulse-vault")) {
            return $cand
        }
    }
    return $null
}

function Get-CpuSuffix {
    $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
    switch ($arch) {
        "x64" { return "amd64" }
        "arm64" { return "arm64" }
        default { throw "unsupported CPU: $arch" }
    }
}

function Get-SumForAsset([string]$sumsFile, [string]$asset) {
    foreach ($line in Get-Content $sumsFile) {
        $parts = $line -split '\s+'
        if ($parts.Count -ge 2 -and ($parts[-1] -eq $asset -or $parts[-1] -eq "*$asset")) {
            return $parts[0].ToLowerInvariant()
        }
    }
    return $null
}

function Install-FromRelease {
    param([string]$Asset, [string]$DestPath)
    $base = "https://github.com/$Repo/releases/latest/download"
    $sums = Join-Path $InstallDir "SHA256SUMS"
    Write-Host "  downloading $Asset"
    try {
        Invoke-WebRequest -UseBasicParsing -Uri "$base/SHA256SUMS" -OutFile $sums
        Invoke-WebRequest -UseBasicParsing -Uri "$base/$Asset" -OutFile "$DestPath.part"
    } catch {
        throw "download failed (no published v* tag yet?). $($_.Exception.Message)"
    }
    $want = Get-SumForAsset $sums $Asset
    if (-not $want) {
        throw "SHA256SUMS has no $Asset. Open $Releases"
    }
    $got = (Get-FileHash -Algorithm SHA256 -Path "$DestPath.part").Hash.ToLowerInvariant()
    if ($got -ne $want) {
        Remove-Item -Force "$DestPath.part" -ErrorAction SilentlyContinue
        throw "SHA-256 mismatch for $Asset"
    }
    Move-Item -Force "$DestPath.part" $DestPath
}

function Install-CliFromSource([string]$root, [string]$dest) {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw "Go is not on PATH. Install https://go.dev/dl/ or wait for a GitHub Release."
    }
    Write-Host "  building CLI"
    $old = Get-Location
    Set-Location (Join-Path $root "gui-go")
    try {
        $env:CGO_ENABLED = "0"
        & go build -trimpath -ldflags "-s -w" -o $dest ./cmd/pulse-vault
        if ($LASTEXITCODE -ne 0) { throw "go build CLI failed" }
    } finally {
        Set-Location $old
    }
}

function Install-GuiFromSource([string]$root, [string]$dest) {
    $build = Join-Path $root "gui-go\build.ps1"
    if (-not (Test-Path $build)) { throw "gui-go/build.ps1 missing" }
    Write-Host "  building GUI from source (needs gcc / MSYS2)"
    & powershell -NoProfile -ExecutionPolicy Bypass -File $build -Out $dest -Version "dev"
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path $dest)) {
        throw "GUI build failed. Install MSYS2 mingw gcc or download a release exe."
    }
}

function Add-StartMenuShortcut([string]$target) {
    $menu = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs"
    New-Item -ItemType Directory -Force -Path $menu | Out-Null
    $lnk = Join-Path $menu "Pulse-Vault.lnk"
    $w = New-Object -ComObject WScript.Shell
    $sc = $w.CreateShortcut($lnk)
    $sc.TargetPath = $target
    $sc.WorkingDirectory = Split-Path $target
    $sc.Description = "Pulse-Vault from DNSPulse"
    $sc.Save()
    return $lnk
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
$suffix = Get-CpuSuffix
$cliPath = Join-Path $InstallDir "pulse-vault.exe"
$guiPath = Join-Path $InstallDir "pulse-vault-gui.exe"
$root = Get-RepoRoot
$usedSource = $false

Write-Host "Pulse-Vault / DNSPulse / $Site"
Write-Host "Install dir: $InstallDir"

try {
    if ($FromSource) {
        if (-not $root) { throw "-FromSource needs a git clone (run .\scripts\install.ps1 from the repo)." }
        Install-CliFromSource $root $cliPath
        $usedSource = $true
    } else {
        try {
            Install-FromRelease -Asset "pulse-vault-windows-$suffix.exe" -DestPath $cliPath
        } catch {
            if ($root -and (Get-Command go -ErrorAction SilentlyContinue)) {
                Write-Host "Release download failed; building CLI from this clone instead."
                Write-Host "  $($_.Exception.Message)"
                Install-CliFromSource $root $cliPath
                $usedSource = $true
            } else {
                Write-Host ""
                Write-Host "Could not download a release. Either:"
                Write-Host "  1. Open $Releases and grab pulse-vault-windows-$suffix.exe"
                Write-Host "  2. Clone the repo and run:  .\scripts\install.ps1 -FromSource"
                Write-Host "  3. Wait until a v* tag is published."
                throw
            }
        }
    }

    if ($WithGui) {
        if ($usedSource -and $root) {
            Install-GuiFromSource $root $guiPath
        } else {
            # CI publishes an amd64 Windows GUI from windows-latest.
            $guiAsset = "pulse-vault-gui-windows-amd64.exe"
            if ($suffix -eq "arm64") {
                Write-Host "Note: GitHub GUI build is amd64; installing that (Windows ARM can run it)."
            }
            try {
                Install-FromRelease -Asset $guiAsset -DestPath $guiPath
            } catch {
                if ($root) {
                    Write-Host "GUI release missing; trying a local build."
                    Install-GuiFromSource $root $guiPath
                } else {
                    throw
                }
            }
        }
        $shortcut = Add-StartMenuShortcut $guiPath
        Write-Host "GUI:      $guiPath"
        Write-Host "Start Menu: $shortcut"
    }
} catch {
    Write-Host "ERROR: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "Help: $Site   Releases: $Releases"
    exit 1
}

if (-not $SkipPath) {
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if (-not $userPath) { $userPath = "" }
    if ($userPath -notlike "*$InstallDir*") {
        $newPath = $userPath + ";" + $InstallDir
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        $env:Path = $env:Path + ";" + $InstallDir
        Write-Host "Added $InstallDir to your user PATH (open a new terminal)."
    }
}

Write-Host ""
Write-Host "CLI:  $cliPath"
Write-Host "Try:  pulse-vault version"
if ($WithGui) {
    Write-Host "App:  Start Menu -> Pulse-Vault   (or double-click pulse-vault-gui.exe)"
}
Write-Host "Site: $Site"
