# Multi-platform Pulse-Vault build (pure-Go CLI + optional host GUI).
#
# From gui-go/:
#   .\scripts\build-multi.ps1
#   .\scripts\build-multi.ps1 -OutDir ..\dist -Version 1.2.3
#
# CLI is always built with CGO_ENABLED=0 (cross-compiles windows/linux/darwin).
# Host GUI is built only when a CGO toolchain is available (not faked for other OS).

param(
    [string]$OutDir = ".\dist",
    [string]$Version = "dev"
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
if (-not $Root) { $Root = (Get-Location).Path }
Set-Location $Root

$OutDir = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutDir)
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

$ldflags = "-s -w -X github.com/Z3r0s/Pulse-Vault/gui-go/internal/version.Version=$Version"
$failures = 0
$built = New-Object System.Collections.Generic.List[string]

function Write-Log {
    param([string]$Msg, [string]$Level = "INFO")
    $ts = Get-Date -Format "HH:mm:ss"
    Write-Host "[$ts][$Level] $Msg"
}

function Invoke-CLIBuild {
    param(
        [string]$Goos,
        [string]$Goarch,
        [string]$OutName
    )
    $outPath = Join-Path $OutDir $OutName
    Write-Log "CLI $Goos/$Goarch -> $OutName ..."
    $env:CGO_ENABLED = "0"
    $env:GOOS = $Goos
    $env:GOARCH = $Goarch
    # Avoid accidental Windows build modes when cross-compiling
    Remove-Item Env:GOARM -ErrorAction SilentlyContinue

    & go build -trimpath -buildvcs=false -ldflags $ldflags -o $outPath ./cmd/pulse-vault
    $code = $LASTEXITCODE
    if ($code -ne 0 -or -not (Test-Path $outPath)) {
        Write-Log "FAIL CLI $Goos/$Goarch (exit=$code)" "FAIL"
        $script:failures++
        return
    }
    $len = (Get-Item $outPath).Length
    Write-Log "OK   CLI $Goos/$Goarch ($len bytes)" "OK"
    $built.Add($OutName)
}

Write-Log "Pulse-Vault multi build (Version=$Version)"
Write-Log "OutDir=$OutDir"
Write-Log "Root=$Root"

# --- Pure-Go CLI targets (CGO_ENABLED=0) ---
Invoke-CLIBuild -Goos "windows" -Goarch "amd64" -OutName "pulse-vault-windows-amd64.exe"
Invoke-CLIBuild -Goos "linux"   -Goarch "amd64" -OutName "pulse-vault-linux-amd64"
Invoke-CLIBuild -Goos "linux"   -Goarch "arm64" -OutName "pulse-vault-linux-arm64"
Invoke-CLIBuild -Goos "darwin"  -Goarch "amd64" -OutName "pulse-vault-darwin-amd64"
Invoke-CLIBuild -Goos "darwin"  -Goarch "arm64" -OutName "pulse-vault-darwin-arm64"

# Reset GOOS/GOARCH so host GUI build uses the host
Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue

# --- Host GUI (only when CGO is available; never invent foreign-OS GUI binaries) ---
$hostOs = go env GOHOSTOS
$hostArch = go env GOHOSTARCH
$guiName = if ($hostOs -eq "windows") {
    "pulse-vault-gui-$hostOs-$hostArch.exe"
} else {
    "pulse-vault-gui-$hostOs-$hostArch"
}
$guiOut = Join-Path $OutDir $guiName

$cgoOk = $false
$mingw = "C:\msys64\mingw64\bin"
if (Test-Path "$mingw\gcc.exe") {
    $env:Path = "$mingw;" + $env:Path
    $env:CC = "gcc"
}
# Probe: can we compile with CGO?
$env:CGO_ENABLED = "1"
$probe = & go env CGO_ENABLED 2>$null
if ($probe -eq "1") {
    # Try a cheap check for a C compiler
    $gcc = Get-Command gcc -ErrorAction SilentlyContinue
    $clang = Get-Command clang -ErrorAction SilentlyContinue
    if ($gcc -or $clang -or $env:CC) {
        $cgoOk = $true
    }
}

if ($cgoOk) {
    Write-Log "GUI host $hostOs/$hostArch (CGO=1) -> $guiName Version=$Version ..."
    $env:CGO_ENABLED = "1"
    # Prefer full Windows production path (version stamp + PE icon/resource).
    if ($hostOs -eq "windows" -and (Test-Path (Join-Path $Root "build.ps1"))) {
        & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $Root "build.ps1") -Out $guiOut -Version $Version
        if ($LASTEXITCODE -ne 0 -or -not (Test-Path $guiOut)) {
            Write-Log "FAIL host GUI via build.ps1 (CLI artifacts kept)" "FAIL"
            $failures++
        } else {
            $len = (Get-Item $guiOut).Length
            Write-Log "OK   host GUI via build.ps1 ($len bytes, version=$Version)" "OK"
            $built.Add($guiName)
        }
    } else {
        $guiLdflags = "-H windowsgui -s -w -X github.com/Z3r0s/Pulse-Vault/gui-go/internal/version.Version=$Version"
        & go build -trimpath -buildvcs=false -ldflags $guiLdflags -o $guiOut .
        if ($LASTEXITCODE -ne 0 -or -not (Test-Path $guiOut)) {
            Write-Log "FAIL host GUI (CGO build failed; CLI artifacts kept)" "FAIL"
            $failures++
        } else {
            $len = (Get-Item $guiOut).Length
            Write-Log "OK   host GUI ($len bytes, version=$Version)" "OK"
            $built.Add($guiName)
        }
    }
} else {
    Write-Log "SKIP host GUI (CGO toolchain not available on this host)" "SKIP"
}

# --- SHA256SUMS ---
$sumsPath = Join-Path $OutDir "SHA256SUMS"
$lines = New-Object System.Collections.Generic.List[string]
foreach ($name in ($built | Sort-Object)) {
    $path = Join-Path $OutDir $name
    if (-not (Test-Path $path)) { continue }
    $hash = (Get-FileHash -Algorithm SHA256 -Path $path).Hash.ToLowerInvariant()
    # GNU coreutils style: "<hash>  <filename>"
    $lines.Add("$hash  $name")
}
# Write without BOM for portable checksums (PS 5.1 + Core)
[System.IO.File]::WriteAllLines($sumsPath, $lines.ToArray())
Write-Log "Wrote $sumsPath ($($lines.Count) entries)"

Write-Host ""
Write-Log "Built $($built.Count) artifact(s); failures=$failures"
if ($failures -gt 0) {
    Write-Log "Completed with failures" "FAIL"
    exit 1
}
Write-Log "All requested targets succeeded" "OK"
exit 0
