param(
    [string]$Target = "latest"
)

$ErrorActionPreference = "Stop"
$DefaultDistBaseUrl = "https://opencsg-public-resource.oss-cn-beijing.aliyuncs.com/open-code-releases"
$DistBaseUrl = if ($env:CSGHUB_LITE_OPEN_CODE_DIST_BASE_URL) { $env:CSGHUB_LITE_OPEN_CODE_DIST_BASE_URL } else { $DefaultDistBaseUrl }

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

function Install-ExtractedRuntime([string]$Version, [string]$BinaryName, [string]$ExtractedDir) {
    $launcherDir = Join-Path $env:USERPROFILE ".local\bin"
    $runtimeRoot = Join-Path $env:USERPROFILE ".local\share\opencode\versions"
    $versionDir = Join-Path $runtimeRoot $Version
    $binaryPath = Join-Path $versionDir $BinaryName
    $launcherPath = Join-Path $launcherDir "opencode.exe"

    New-Item -ItemType Directory -Force -Path $launcherDir | Out-Null
    New-Item -ItemType Directory -Force -Path $runtimeRoot | Out-Null

    if (Test-Path $versionDir) {
        Remove-Item -Path $versionDir -Recurse -Force
    }
    Move-Item -Path $ExtractedDir -Destination $versionDir -Force

    if (-not (Test-Path $binaryPath)) {
        throw "extracted binary not found at $binaryPath"
    }

    if (Test-Path $launcherPath) {
        Remove-Item -Path $launcherPath -Force
    }

    $linked = $false
    try {
        New-Item -ItemType HardLink -Path $launcherPath -Target $binaryPath | Out-Null
        $linked = $true
    } catch {
        Copy-Item -Path $binaryPath -Destination $launcherPath -Force
    }

    Ensure-PathContains -Dir $launcherDir
    Write-Output "INFO: installed OpenCode runtime $Version to $versionDir"
    if ($linked) {
        Write-Output "INFO: updated launcher $launcherPath via hard link"
    } else {
        Write-Output "INFO: updated launcher $launcherPath via file copy"
    }
}

if (-not [Environment]::Is64BitProcess) {
    throw "OpenCode does not support 32-bit Windows."
}

$distBaseUrl = Trim-TrailingSlash $DistBaseUrl
$requestedVersion = Normalize-RequestedVersion $Target
$workDir = Join-Path $env:TEMP ("opencode-install-" + [guid]::NewGuid().ToString("N"))
$downloadDir = Join-Path $workDir "downloads"
$extractDir = Join-Path $workDir "extract"

try {
    Emit-Progress 10 "detecting_platform"
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
        $platform = "win32-arm64"
    } else {
        $platform = "win32-x64"
    }

    New-Item -ItemType Directory -Force -Path $downloadDir | Out-Null

    Emit-Progress 25 "resolving_latest"
    $version = if ($requestedVersion -eq "latest") {
        (Invoke-RestMethod -Uri "$distBaseUrl/latest" -ErrorAction Stop).ToString().Trim()
    } else {
        $requestedVersion
    }
    if ([string]::IsNullOrWhiteSpace($version)) {
        throw "failed to resolve OpenCode version"
    }

    $manifest = Invoke-RestMethod -Uri "$distBaseUrl/$version/manifest.json" -ErrorAction Stop
    $platformMeta = $manifest.platforms.$platform
    if (-not $platformMeta) {
        throw "platform $platform not found in manifest"
    }

    $checksum = $platformMeta.checksum
    $binaryName = $platformMeta.binary
    $assetName = $platformMeta.asset
    $archiveFormat = $platformMeta.archive_format
    if (-not $checksum -or -not $binaryName -or -not $assetName -or -not $archiveFormat) {
        throw "manifest is missing fields for platform $platform"
    }
    if ($archiveFormat -ne "zip") {
        throw "unsupported archive format $archiveFormat for $platform"
    }

    $archivePath = Join-Path $downloadDir $assetName
    Emit-Progress 55 "downloading_archive"
    Write-Output "INFO: downloading OpenCode $version for $platform from $distBaseUrl"
    Invoke-WebRequest -Uri "$distBaseUrl/$version/$platform/$assetName" -OutFile $archivePath -ErrorAction Stop

    Emit-Progress 75 "verifying_checksum"
    $actualChecksum = (Get-FileHash -Path $archivePath -Algorithm SHA256).Hash.ToLower()
    if ($actualChecksum -ne $checksum) {
        throw "checksum verification failed"
    }

    Emit-Progress 90 "installing_runtime"
    Expand-Archive -Path $archivePath -DestinationPath $extractDir -Force
    Install-ExtractedRuntime -Version $version -BinaryName $binaryName -ExtractedDir $extractDir

    Emit-Progress 100 "complete"
    if (Get-Command opencode -ErrorAction SilentlyContinue) {
        try { opencode --version } catch {}
    }
    Write-Output "INFO: OpenCode installation complete"
} finally {
    if (Test-Path $workDir) {
        Remove-Item -Path $workDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}
