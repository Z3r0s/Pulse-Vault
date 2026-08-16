# Pulse-Vault CLI wrapper. DNSPulse https://dnspulse.org
# Build (if needed) and run the Pulse-Vault CLI.
# You do not set CGO_ENABLED. The CLI is plain Go.
#
#   .\cli.ps1
#   .\cli.ps1 version
#   .\cli.ps1 create .\demo.pulsevault --password 'choose-a-strong-password'
#   .\cli.ps1 -Build

[CmdletBinding()]
param(
    [switch]$Build,
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$CliArgs
)

$ErrorActionPreference = "Stop"
$Root = $PSScriptRoot
if (-not $Root) { $Root = Get-Location }
$Out = Join-Path $Root "pulse-vault.exe"
$Pkg = Join-Path $Root "gui-go\cmd\pulse-vault"

if (-not (Test-Path $Pkg)) {
    Write-Host "This script lives in the Pulse-Vault repo root."
    exit 2
}

function Build-Cli {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Host "Install Go from https://go.dev/dl/  (no gcc needed for the CLI)"
        exit 2
    }
    $prev = $env:CGO_ENABLED
    $env:CGO_ENABLED = "0"
    try {
        Write-Host "Building pulse-vault.exe ..."
        Push-Location $Root
        $cliDir = Join-Path $Root "gui-go\cmd\pulse-vault"
        $syso = Join-Path $cliDir "resource_windows_amd64.syso"
        if (Get-Command goversioninfo -ErrorAction SilentlyContinue) {
            Push-Location $cliDir
            try {
                & goversioninfo -64 -o "resource_windows_amd64.syso" -icon (Join-Path $Root "gui-go\assets\pulse-vault.ico") -manifest (Join-Path $Root "gui-go\app.manifest")
            } catch {
                Write-Host "  (version resource skipped)"
            }
            Pop-Location
        }
        & go build -trimpath -ldflags "-s -w" -o $Out ./gui-go/cmd/pulse-vault
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    } finally {
        Pop-Location
        if ($null -eq $prev) {
            Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
        } else {
            $env:CGO_ENABLED = $prev
        }
    }
}

if ($Build -or -not (Test-Path $Out)) {
    Build-Cli
}

if ($Build -and ($null -eq $CliArgs -or $CliArgs.Count -eq 0)) {
    Write-Host "OK $Out"
    exit 0
}

if ($null -eq $CliArgs) { $CliArgs = @() }
& $Out @CliArgs
exit $LASTEXITCODE
