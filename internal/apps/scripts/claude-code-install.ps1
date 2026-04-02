param(
    [string]$Target = "latest"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"
$DefaultDistBaseUrl = "https://opencsg-public-resource.oss-cn-beijing.aliyuncs.com/claude-code-releases"
$DistBaseUrl = if ($env:CSGHUB_LITE_CLAUDE_DIST_BASE_URL) { $env:CSGHUB_LITE_CLAUDE_DIST_BASE_URL } else { $DefaultDistBaseUrl }

function Emit-Progress([int]$Percent, [string]$Phase) {
    Write-Output "CSGHUB_PROGRESS|$Percent|$Phase"
}

function Trim-TrailingSlash([string]$Value) {
    if ([string]::IsNullOrWhiteSpace($Value)) {
        return $Value
    }
    return $Value.TrimEnd('/')
}

function Normalize-RequestedVersion([string]$Value) {
    if ([string]::IsNullOrWhiteSpace($Value) -or $Value -eq "latest") {
        return "latest"
    }
    return $Value.TrimStart('v')
}

function Ensure-PathContains([string]$Dir) {
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $parts = @()
    if ($userPath) { $parts = $userPath.Split(';') }
    if ($parts -notcontains $Dir) {
        $newPath = if ($userPath) { "$Dir;$userPath" } else { $Dir }
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    }
    if ($env:Path -notlike "*$Dir*") {
        $env:Path = "$Dir;$env:Path"
    }
}

function Install-NativeRuntime([string]$Version, [string]$BinaryName, [string]$BinaryPath) {
    $launcherDir = Join-Path $env:USERPROFILE ".local\bin"
    $versionsDir = Join-Path $env:USERPROFILE ".local\share\claude\versions\$Version"
    $versionPath = Join-Path $versionsDir $BinaryName
    $launcherPath = Join-Path $launcherDir "claude.exe"

    New-Item -ItemType Directory -Force -Path $launcherDir | Out-Null
    New-Item -ItemType Directory -Force -Path $versionsDir | Out-Null

    if (Test-Path $versionPath) {
        Remove-Item -Path $versionPath -Force
    }
    Move-Item -Path $BinaryPath -Destination $versionPath -Force

    if (Test-Path $launcherPath) {
        Remove-Item -Path $launcherPath -Force
    }

    $linked = $false
    try {
        New-Item -ItemType HardLink -Path $launcherPath -Target $versionPath | Out-Null
        $linked = $true
    } catch {
        Copy-Item -Path $versionPath -Destination $launcherPath -Force
    }

    Ensure-PathContains -Dir $launcherDir
    Write-Output "INFO: installed Claude Code runtime $Version to $versionPath"
    if ($linked) {
        Write-Output "INFO: updated launcher $launcherPath via hard link"
    } else {
        Write-Output "INFO: updated launcher $launcherPath via file copy"
    }
}

if (-not [Environment]::Is64BitProcess) {
    throw "Claude Code does not support 32-bit Windows."
}

$distBaseUrl = Trim-TrailingSlash $DistBaseUrl
$downloadDir = Join-Path $env:USERPROFILE ".claude\downloads"

Emit-Progress 10 "detecting_platform"
if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
    $platform = "win32-arm64"
} else {
    $platform = "win32-x64"
}

New-Item -ItemType Directory -Force -Path $downloadDir | Out-Null

Emit-Progress 25 "resolving_latest"
$requestedVersion = Normalize-RequestedVersion $Target
$version = if ($requestedVersion -eq "latest") {
    (Invoke-RestMethod -Uri "$distBaseUrl/latest" -ErrorAction Stop).ToString().Trim()
} else {
    $requestedVersion
}
if ([string]::IsNullOrWhiteSpace($version)) {
    throw "failed to resolve Claude Code version"
}

$manifest = Invoke-RestMethod -Uri "$distBaseUrl/$version/manifest.json" -ErrorAction Stop
$platformMeta = $manifest.platforms.$platform
if (-not $platformMeta) {
    throw "platform $platform not found in manifest"
}

$checksum = $platformMeta.checksum
if (-not $checksum) {
    throw "checksum missing for platform $platform"
}

$binaryName = if ($platformMeta.binary) { $platformMeta.binary } else { "claude.exe" }
$binaryExtension = [IO.Path]::GetExtension($binaryName)
$binaryStem = [IO.Path]::GetFileNameWithoutExtension($binaryName)
if ([string]::IsNullOrWhiteSpace($binaryExtension)) {
    $binaryPath = Join-Path $downloadDir "$binaryName-$version-$platform"
} else {
    $binaryPath = Join-Path $downloadDir "$binaryStem-$version-$platform$binaryExtension"
}
Emit-Progress 55 "downloading_binary"
Write-Output "INFO: downloading Claude Code $version for $platform from $distBaseUrl"
Invoke-WebRequest -Uri "$distBaseUrl/$version/$platform/$binaryName" -OutFile $binaryPath -ErrorAction Stop

Emit-Progress 75 "verifying_checksum"
$actualChecksum = (Get-FileHash -Path $binaryPath -Algorithm SHA256).Hash.ToLower()
if ($actualChecksum -ne $checksum) {
    Remove-Item -Force $binaryPath -ErrorAction SilentlyContinue
    throw "checksum verification failed"
}

Emit-Progress 90 "installing_runtime"
Install-NativeRuntime -Version $version -BinaryName $binaryName -BinaryPath $binaryPath

Emit-Progress 100 "complete"
if (Get-Command claude -ErrorAction SilentlyContinue) {
    try { claude --version } catch {}
}
Write-Output "INFO: Claude Code installation complete"
