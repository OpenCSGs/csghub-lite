$ErrorActionPreference = "Stop"

function Emit-Progress([int]$Percent, [string]$Phase) {
    Write-Output "CSGHUB_PROGRESS|$Percent|$Phase"
}

$nativeBin = Join-Path $env:USERPROFILE ".local\bin\claude.exe"
$nativeShare = Join-Path $env:USERPROFILE ".local\share\claude"
$npmPackage = "@anthropic-ai/claude-code"

Emit-Progress 5 "preflight"

Emit-Progress 35 "removing_binary"
if (Test-Path $nativeBin) {
    Remove-Item -Path $nativeBin -Force
    Write-Output "INFO: removed $nativeBin"
} else {
    Write-Output "INFO: native Claude binary not found at $nativeBin"
}

Emit-Progress 65 "removing_files"
if (Test-Path $nativeShare) {
    Remove-Item -Path $nativeShare -Recurse -Force
    Write-Output "INFO: removed $nativeShare"
} else {
    Write-Output "INFO: native Claude runtime not found at $nativeShare"
}

if (Get-Command npm -ErrorAction SilentlyContinue) {
    try {
        npm ls -g --depth=0 $npmPackage *> $null
        Emit-Progress 80 "removing_package"
        npm uninstall -g $npmPackage
    } catch {
        # Ignore missing npm package.
    }
}

Emit-Progress 90 "verifying_uninstall"
if (Get-Command claude -ErrorAction SilentlyContinue) {
    $cmd = (Get-Command claude).Source
    throw "Claude Code binary is still available at $cmd"
}

Emit-Progress 100 "complete"
Write-Output "INFO: Claude Code uninstallation complete"
