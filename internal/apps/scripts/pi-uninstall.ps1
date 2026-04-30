$ErrorActionPreference = "Continue"

function Emit-ProgressLine {
    param(
        [int]$Progress,
        [string]$Phase
    )
    Write-Output "[progress] $Progress $Phase"
}

Emit-ProgressLine 20 "uninstalling_pi"
if (Get-Command npm -ErrorAction SilentlyContinue) {
    npm uninstall -g @mariozechner/pi-coding-agent
}

Emit-ProgressLine 100 "complete"
Write-Output "INFO: Pi Coding Agent uninstall complete."
