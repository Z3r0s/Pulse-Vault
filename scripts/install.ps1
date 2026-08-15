# Install the official Pulse-Vault Go CLI from GitHub Releases.
# DNSPulse — https://dnspulse.org
#   irm https://raw.githubusercontent.com/Z3r0s/Pulse-Vault/main/scripts/install.ps1 | iex

$ErrorActionPreference = "Stop"
$Repo = "Z3r0s/Pulse-Vault"
$DestDir = Join-Path $env:LOCALAPPDATA "Pulse-Vault"
New-Item -ItemType Directory -Force -Path $DestDir | Out-Null

$arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
switch ($arch) {
    "x64" { $suffix = "amd64" }
    "arm64" { $suffix = "arm64" }
    default { throw "unsupported CPU: $arch" }
}
$asset = "pulse-vault-windows-$suffix.exe"
$base = "https://github.com/$Repo/releases/latest/download"

Write-Host "Pulse-Vault (DNSPulse / dnspulse.org)"
Write-Host "Fetching $asset ..."
$sums = Join-Path $DestDir "SHA256SUMS"
$out = Join-Path $DestDir "pulse-vault.exe"
Invoke-WebRequest -UseBasicParsing -Uri "$base/SHA256SUMS" -OutFile $sums
Invoke-WebRequest -UseBasicParsing -Uri "$base/$asset" -OutFile "$out.part"

$want = $null
Get-Content $sums | ForEach-Object {
    $parts = $_ -split '\s+'
    if ($parts.Count -ge 2 -and ($parts[-1] -eq $asset -or $parts[-1] -eq "*$asset")) {
        $want = $parts[0].ToLowerInvariant()
    }
}
if (-not $want) { throw "SHA256SUMS has no $asset — has a v* tag been published?" }
$got = (Get-FileHash -Algorithm SHA256 -Path "$out.part").Hash.ToLowerInvariant()
if ($got -ne $want) { throw "SHA-256 mismatch: got $got want $want" }
Move-Item -Force "$out.part" $out

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$DestDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$DestDir", "User")
    $env:Path = "$env:Path;$DestDir"
    Write-Host "Added $DestDir to your user PATH (new terminals pick this up)."
}

Write-Host "Installed $out"
& $out version
Write-Host "Site: https://dnspulse.org"
