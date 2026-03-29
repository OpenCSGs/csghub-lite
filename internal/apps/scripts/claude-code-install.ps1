param(
    [string]$Target = "latest"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"
$packageName = "@anthropic-ai/claude-code"

function Emit-Progress([int]$Percent, [string]$Phase) {
    Write-Output "CSGHUB_PROGRESS|$Percent|$Phase"
}

function Choose-Registry() {
    $registries = @()
    if ($env:NPM_CONFIG_REGISTRY) {
        $registries += $env:NPM_CONFIG_REGISTRY
    }
    $registries += @(
        "https://registry.npmmirror.com",
        "https://registry.npmjs.org"
    )

    $seen = @{}
    foreach ($registry in $registries) {
        if ([string]::IsNullOrWhiteSpace($registry) -or $seen.ContainsKey($registry)) {
            continue
        }
        $seen[$registry] = $true
        Write-Output "INFO: checking npm registry $registry"
        & npm view $packageName version --registry $registry *> $null
        if ($LASTEXITCODE -eq 0) {
            return $registry
        }
    }

    return ""
}

function Install-WithNpm() {
    if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
        Write-Output "INFO: npm not found; falling back to native Claude Code installer"
        return $false
    }

    Emit-Progress 10 "preflight"
    $registry = Choose-Registry
    if ([string]::IsNullOrWhiteSpace($registry)) {
        Write-Output "WARN: unable to reach a working npm registry for $packageName; falling back to native installer"
        return $false
    }

    $packageSpec = $packageName
    if ($Target -and $Target -ne "latest") {
        $packageSpec = "$packageName@$Target"
    }

    Write-Output "INFO: using npm registry $registry"
    Emit-Progress 25 "resolving_latest"
    Emit-Progress 55 "installing"
    & npm install -g $packageSpec --registry $registry
    if ($LASTEXITCODE -ne 0) {
        throw "npm install -g $packageSpec failed with exit code $LASTEXITCODE"
    }

    Emit-Progress 90 "verifying"
    if (-not (Get-Command claude -ErrorAction SilentlyContinue)) {
        throw "Claude Code command was not found on PATH after npm installation"
    }

    try { claude --version } catch {}
    Emit-Progress 100 "complete"
    Write-Output "INFO: Claude Code installation complete via npm"
    return $true
}

if (Install-WithNpm) {
    return
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
