$ErrorActionPreference = "Stop"

function Emit-Progress([int]$Percent, [string]$Phase) {
    Write-Output "CSGHUB_PROGRESS|$Percent|$Phase"
}

function Get-NpmGlobalRoot() {
    $output = & npm root -g 2>$null
    if ($LASTEXITCODE -ne 0) {
        return ""
    }
    return ($output | Out-String).Trim()
}

function Remove-StaleNpmTempDirs([string]$GlobalRoot, [string]$PackageDirName) {
    if ([string]::IsNullOrWhiteSpace($GlobalRoot) -or -not (Test-Path -LiteralPath $GlobalRoot)) {
        return
    }

    Get-ChildItem -LiteralPath $GlobalRoot -Force -Directory -Filter ".$PackageDirName-*" -ErrorAction SilentlyContinue | ForEach-Object {
        Remove-Item -LiteralPath $_.FullName -Recurse -Force
        Write-Output "INFO: removed stale npm temp dir $($_.FullName)"
    }
}

if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
    throw "npm is required to uninstall OpenClaw."
}

$packageName = "openclaw"
$packageDirName = "openclaw"
Emit-Progress 5 "preflight"
Emit-Progress 35 "removing_package"
$packageInstalled = $false
& npm ls -g --depth=0 $packageName *> $null
if ($LASTEXITCODE -eq 0) {
    $packageInstalled = $true
}

if ($packageInstalled) {
    $npmRoot = Get-NpmGlobalRoot
    Remove-StaleNpmTempDirs $npmRoot $packageDirName

    & npm uninstall -g $packageName
    if ($LASTEXITCODE -ne 0) {
        Write-Output "WARN: npm uninstall failed once, retrying after cleaning stale temp dirs"
        Remove-StaleNpmTempDirs $npmRoot $packageDirName
        & npm uninstall -g $packageName
        if ($LASTEXITCODE -ne 0) {
            throw "npm uninstall -g $packageName failed with exit code $LASTEXITCODE"
        }
    }
} else {
    Write-Output "INFO: npm package $packageName is not installed"
}

Emit-Progress 80 "verifying_uninstall"
if (Get-Command openclaw -ErrorAction SilentlyContinue) {
    $cmd = (Get-Command openclaw).Source
    throw "OpenClaw binary is still available at $cmd"
}

Emit-Progress 100 "complete"
Write-Output "INFO: OpenClaw uninstallation complete"
