$ErrorActionPreference = "Stop"

function Emit-Progress([int]$Percent, [string]$Phase) {
    Write-Output "CSGHUB_PROGRESS|$Percent|$Phase"
}

$packageName = "@openai/codex"
$runtimeRoot = Join-Path $env:USERPROFILE ".local\share\codex"
$launcherDir = Join-Path $env:USERPROFILE ".local\bin"
$launcherPath = Join-Path $launcherDir "codex.exe"
Emit-Progress 5 "preflight"
Emit-Progress 35 "removing_package"
if (Get-Command npm -ErrorAction SilentlyContinue) {
    try {
        npm ls -g --depth=0 $packageName *> $null
        npm uninstall -g $packageName
    } catch {
        Write-Output "INFO: npm package $packageName is not installed"
    }
} else {
    Write-Output "INFO: npm not found, skipping legacy npm package removal"
}

Emit-Progress 55 "removing_runtime"
if (Test-Path $launcherPath) {
    Remove-Item -Path $launcherPath -Force -ErrorAction SilentlyContinue
}
if (Test-Path $runtimeRoot) {
    Remove-Item -Path $runtimeRoot -Recurse -Force -ErrorAction SilentlyContinue
}

Emit-Progress 80 "verifying_uninstall"
if (Get-Command codex -ErrorAction SilentlyContinue) {
    $cmd = (Get-Command codex).Source
    throw "Codex binary is still available at $cmd"
}

Emit-Progress 100 "complete"
Write-Output "INFO: Codex uninstallation complete"
