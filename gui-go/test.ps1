# Product test suite. Does not need gcc.
# GUI (`go test .`) only runs if gcc is actually on PATH.
#
#   cd gui-go
#   .\test.ps1
#
# Don't do:  $env:CGO_ENABLED=1; go test ./...
# That forces CGO on the Fyne package even when gcc isn't installed.

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$mingw = "C:\msys64\mingw64\bin"
if (Test-Path "$mingw\gcc.exe") {
    $env:Path = "$mingw;" + $env:Path
}

$pkgs = @(
    "./internal/crypto",
    "./internal/vault",
    "./internal/ui",
    "./cmd/pulse-vault",
    "./crypto"
)

$hadCgo = $env:CGO_ENABLED
$env:CGO_ENABLED = "0"
Write-Host "CGO_ENABLED=0  go test $($pkgs -join ' ') -count=1"
go test @pkgs -count=1
if ($LASTEXITCODE -ne 0) {
    if ($null -eq $hadCgo) { Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue } else { $env:CGO_ENABLED = $hadCgo }
    exit $LASTEXITCODE
}

$gcc = Get-Command gcc -ErrorAction SilentlyContinue
if ($gcc) {
    $env:CGO_ENABLED = "1"
    Write-Host "gcc found ($($gcc.Source))  CGO_ENABLED=1  go test . -count=1"
    go test . -count=1
    $code = $LASTEXITCODE
} else {
    Write-Host "gcc not on PATH — skipped GUI package (github.com/Z3r0s/Pulse-Vault/gui-go)."
    Write-Host "That's expected. Product tests above are what CI runs."
    Write-Host "Want the GUI binary? .\build.ps1  (installs/finds MSYS2 mingw)."
    $code = 0
}

if ($null -eq $hadCgo) { Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue } else { $env:CGO_ENABLED = $hadCgo }
exit $code
