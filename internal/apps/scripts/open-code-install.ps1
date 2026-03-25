$ErrorActionPreference = "Stop"

function Emit-Progress([int]$Percent, [string]$Phase) {
    Write-Output "CSGHUB_PROGRESS|$Percent|$Phase"
}

function Resolve-Registry([string]$PackageName) {
    $registries = @()
    if ($env:NPM_CONFIG_REGISTRY) {
        $registries += $env:NPM_CONFIG_REGISTRY
    }
    $registries += "https://registry.npmmirror.com"
    $registries += "https://registry.npmjs.org"

    foreach ($registry in ($registries | Select-Object -Unique)) {
        Write-Host "INFO: checking npm registry $registry"
        try {
            npm view $PackageName version --registry $registry *> $null
            return $registry
        } catch {
            continue
        }
    }
    throw "unable to reach a working npm registry for $PackageName"
}

if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
    throw "npm is required to install OpenCode."
}

$packageName = "opencode-ai@latest"
Emit-Progress 5 "preflight"
$registry = Resolve-Registry $packageName
Write-Output "INFO: using npm registry $registry"

Emit-Progress 30 "installing"
npm install -g $packageName --registry $registry

Emit-Progress 80 "verifying"
if (Get-Command opencode -ErrorAction SilentlyContinue) {
    $cmd = (Get-Command opencode).Source
    Write-Output "INFO: installed binary: $cmd"
    try { opencode --version } catch {}
}

Emit-Progress 100 "complete"
Write-Output "INFO: OpenCode installation complete"
