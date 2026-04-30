$ErrorActionPreference = "Stop"

function Emit-ProgressLine {
    param(
        [int]$Progress,
        [string]$Phase
    )
    Write-Output "[progress] $Progress $Phase"
}

Emit-ProgressLine 5 "checking_node"
if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
    Write-Output "ERROR: npm (Node.js) is required to install Pi Coding Agent."
    Write-Output "Install Node.js first: https://nodejs.org/"
    exit 1
}

$registry = $env:NPM_CONFIG_REGISTRY
if ([string]::IsNullOrWhiteSpace($registry)) {
    $registry = "https://registry.npmmirror.com"
}
$package = $env:CSGHUB_LITE_PI_PACKAGE
if ([string]::IsNullOrWhiteSpace($package)) {
    $package = "@mariozechner/pi-coding-agent@latest"
}

Emit-ProgressLine 30 "installing_pi"
Write-Output "INFO: installing Pi Coding Agent package $package"
npm install -g --registry="$registry" "$package"
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}

Emit-ProgressLine 85 "verifying_pi"
if (-not (Get-Command pi -ErrorAction SilentlyContinue)) {
    Write-Output "ERROR: Pi was installed but the pi command was not found on PATH."
    exit 1
}

pi --version
Emit-ProgressLine 100 "complete"
Write-Output "INFO: Pi Coding Agent installed successfully."
