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

function Stop-OpenClawProcesses {
    Get-Process -ErrorAction SilentlyContinue | Where-Object {
        $_.ProcessName -like "openclaw*" -or $_.Path -like "*openclaw*"
    } | ForEach-Object {
        try {
            Stop-Process -Id $_.Id -Force -ErrorAction Stop
            Write-Output "INFO: stopped process $($_.Id) $($_.ProcessName)"
        } catch {
            Write-Output "WARN: failed to stop process $($_.Id): $($_.Exception.Message)"
        }
    }
}

function Remove-IfExists([string]$Path) {
    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path)) {
        return
    }
    try {
        Remove-Item -LiteralPath $Path -Recurse -Force -ErrorAction Stop
        Write-Output "INFO: removed $Path"
    } catch {
        Write-Output "WARN: failed to remove ${Path}: $($_.Exception.Message)"
    }
}

function Force-RemoveOpenClawFiles {
    $npmRoot = ""
    if (Get-Command npm -ErrorAction SilentlyContinue) {
        $npmRoot = Get-NpmGlobalRoot
        if (-not [string]::IsNullOrWhiteSpace($npmRoot)) {
            Remove-IfExists (Join-Path $npmRoot $packageDirName)
            Remove-StaleNpmTempDirs $npmRoot $packageDirName
        }
    }

    $candidates = @()
    if ($env:USERPROFILE) {
        $candidates += Join-Path $env:USERPROFILE "bin\openclaw"
        $candidates += Join-Path $env:USERPROFILE ".local\bin\openclaw"
    }
    if ($env:APPDATA) {
        $candidates += Join-Path $env:APPDATA "npm\openclaw"
        $candidates += Join-Path $env:APPDATA "npm\openclaw.cmd"
        $candidates += Join-Path $env:APPDATA "npm\openclaw.ps1"
    }
    $cmd = Get-Command openclaw -ErrorAction SilentlyContinue
    if ($cmd) {
        $candidates += $cmd.Source
    }

    $candidates | Where-Object { $_ } | Select-Object -Unique | ForEach-Object {
        Remove-IfExists $_
    }
}

$packageName = "openclaw"
$packageDirName = "openclaw"
Emit-Progress 5 "preflight"
Stop-OpenClawProcesses

Emit-Progress 35 "removing_package"
$packageInstalled = $false
if (Get-Command npm -ErrorAction SilentlyContinue) {
    & npm ls -g --depth=0 $packageName *> $null
    if ($LASTEXITCODE -eq 0) {
        $packageInstalled = $true
    }
} else {
    Write-Output "WARN: npm is not available; continuing with forced file cleanup"
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
            Write-Output "WARN: npm uninstall -g $packageName failed with exit code $LASTEXITCODE; continuing with forced file cleanup"
        }
    }
} else {
    Write-Output "INFO: npm package $packageName is not installed"
}

Emit-Progress 65 "cleaning_up"
Force-RemoveOpenClawFiles

Emit-Progress 80 "verifying_uninstall"
if (Get-Command openclaw -ErrorAction SilentlyContinue) {
    $cmd = (Get-Command openclaw).Source
    Write-Output "WARN: OpenClaw binary is still available at $cmd"
}

Emit-Progress 100 "complete"
Write-Output "INFO: OpenClaw forced uninstallation complete"
