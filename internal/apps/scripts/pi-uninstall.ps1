$ErrorActionPreference = "Continue"

function Emit-ProgressLine {
    param(
        [int]$Progress,
        [string]$Phase
    )
    Write-Output "CSGHUB_PROGRESS|$Progress|$Phase"
}

Emit-ProgressLine 20 "uninstalling_pi"
$installRoot = $env:CSGHUB_LITE_PI_INSTALL_ROOT
if ([string]::IsNullOrWhiteSpace($installRoot)) {
    $installRoot = Join-Path $env:USERPROFILE ".local\share\pi-coding-agent"
}
$launcherPath = Join-Path $env:USERPROFILE ".local\bin\pi.cmd"
$fdPath = Join-Path $env:USERPROFILE ".local\bin\fd.cmd"
$rgPath = Join-Path $env:USERPROFILE ".local\bin\rg.cmd"

function Remove-GeneratedLauncher {
    param([string]$Path)
    if ((Test-Path $Path) -and ((Get-Content $Path -Raw -ErrorAction SilentlyContinue) -match "csghub-lite")) {
        Remove-Item -Force $Path -ErrorAction SilentlyContinue
    }
}

Remove-Item -Recurse -Force $installRoot -ErrorAction SilentlyContinue
Remove-Item -Force $launcherPath -ErrorAction SilentlyContinue
Remove-GeneratedLauncher $fdPath
Remove-GeneratedLauncher $rgPath

Emit-ProgressLine 100 "complete"
Write-Output "INFO: Pi Coding Agent uninstall complete."
