param(
    [string]$Target = "latest"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

function Emit-Progress([int]$Percent, [string]$Phase) {
    Write-Output "CSGHUB_PROGRESS|$Percent|$Phase"
}

if (-not [Environment]::Is64BitProcess) {
    throw "Claude Code does not support 32-bit Windows."
}

$bucket = "https://storage.googleapis.com/claude-code-dist-86c565f3-f756-42ad-8dfa-d59b1c096819/claude-code-releases"
$downloadDir = Join-Path $env:USERPROFILE ".claude\downloads"

Emit-Progress 10 "detecting_platform"
if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
    $platform = "win32-arm64"
} else {
    $platform = "win32-x64"
}

New-Item -ItemType Directory -Force -Path $downloadDir | Out-Null

Emit-Progress 25 "resolving_latest"
$version = Invoke-RestMethod -Uri "$bucket/latest" -ErrorAction Stop
$manifest = Invoke-RestMethod -Uri "$bucket/$version/manifest.json" -ErrorAction Stop
$checksum = $manifest.platforms.$platform.checksum
if (-not $checksum) {
    throw "platform $platform not found in manifest"
}

$binaryPath = Join-Path $downloadDir "claude-$version-$platform.exe"
Emit-Progress 55 "downloading_binary"
Invoke-WebRequest -Uri "$bucket/$version/$platform/claude.exe" -OutFile $binaryPath -ErrorAction Stop

Emit-Progress 75 "verifying_checksum"
$actualChecksum = (Get-FileHash -Path $binaryPath -Algorithm SHA256).Hash.ToLower()
if ($actualChecksum -ne $checksum) {
    Remove-Item -Force $binaryPath -ErrorAction SilentlyContinue
    throw "checksum verification failed"
}

Emit-Progress 90 "running_installer"
& $binaryPath install $Target
Start-Sleep -Seconds 1
Remove-Item -Force $binaryPath -ErrorAction SilentlyContinue

Emit-Progress 100 "complete"
if (Get-Command claude -ErrorAction SilentlyContinue) {
    try { claude --version } catch {}
}
Write-Output "INFO: Claude Code installation complete"
