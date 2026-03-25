$ErrorActionPreference = "Stop"

function Emit-Progress([int]$Percent, [string]$Phase) {
    Write-Output "CSGHUB_PROGRESS|$Percent|$Phase"
}

if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
    throw "npm is required to uninstall OpenClaw."
}

$packageName = "openclaw"
Emit-Progress 5 "preflight"
Emit-Progress 35 "removing_package"
try {
    npm ls -g --depth=0 $packageName *> $null
    npm uninstall -g $packageName
} catch {
    Write-Output "INFO: npm package $packageName is not installed"
}

Emit-Progress 80 "verifying_uninstall"
if (Get-Command openclaw -ErrorAction SilentlyContinue) {
    $cmd = (Get-Command openclaw).Source
    throw "OpenClaw binary is still available at $cmd"
}

Emit-Progress 100 "complete"
Write-Output "INFO: OpenClaw uninstallation complete"
