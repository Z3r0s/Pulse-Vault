[CmdletBinding()]
param(
    [ValidateRange(1024, 65535)]
    [int]$Port = 8765
)

$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
$docsRoot = Join-Path $repoRoot 'docs'
$wasmOutput = Join-Path $docsRoot 'crypto/pulse-vault-crypto.wasm'

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw 'Go is required to build the local crypto demo.'
}
if (-not (Get-Command python -ErrorAction SilentlyContinue)) {
    throw 'Python 3 is required to serve the local documentation preview.'
}

$listener = Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue | Select-Object -First 1
if ($listener) {
    throw "Port $Port is already in use. Choose another port with -Port."
}

$previousGoOS = $env:GOOS
$previousGoArch = $env:GOARCH
$previousCGOEnabled = $env:CGO_ENABLED

Push-Location $repoRoot
try {
    $env:GOOS = 'js'
    $env:GOARCH = 'wasm'
    $env:CGO_ENABLED = '0'
    & go build -trimpath -o $wasmOutput ./gui-go/cmd/cryptodemo
    if ($LASTEXITCODE -ne 0) {
        throw 'Building the local crypto demo failed.'
    }

    $goRoot = (& go env GOROOT).Trim()
    if ($LASTEXITCODE -ne 0) {
        throw 'Could not locate the active Go installation.'
    }
    $wasmRuntime = Join-Path $goRoot 'lib/wasm/wasm_exec.js'
    if (-not (Test-Path -LiteralPath $wasmRuntime)) {
        throw "The Go WebAssembly runtime was not found at $wasmRuntime."
    }
    Copy-Item -LiteralPath $wasmRuntime -Destination (Join-Path $docsRoot 'crypto/wasm_exec.js') -Force
}
finally {
    if ($null -eq $previousGoOS) { Remove-Item Env:GOOS -ErrorAction SilentlyContinue } else { $env:GOOS = $previousGoOS }
    if ($null -eq $previousGoArch) { Remove-Item Env:GOARCH -ErrorAction SilentlyContinue } else { $env:GOARCH = $previousGoArch }
    if ($null -eq $previousCGOEnabled) { Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue } else { $env:CGO_ENABLED = $previousCGOEnabled }
    Pop-Location
}

Write-Host "Serving the docs preview at http://127.0.0.1:$Port/ (Ctrl+C to stop)."
& python -m http.server $Port --bind 127.0.0.1 --directory $docsRoot
exit $LASTEXITCODE
