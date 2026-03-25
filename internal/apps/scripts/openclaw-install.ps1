$ErrorActionPreference = "Stop"

$requiredNodeVersion = [Version]"22.16.0"

function Emit-Progress([int]$Percent, [string]$Phase) {
    Write-Output "CSGHUB_PROGRESS|$Percent|$Phase"
}

function Get-NodeVersion {
    if (-not (Get-Command node -ErrorAction SilentlyContinue)) {
        return $null
    }
    try {
        $raw = (node -v).Trim()
        if (-not $raw) {
            return $null
        }
        return [Version]($raw.TrimStart("v").Split("-")[0])
    } catch {
        return $null
    }
}

function Test-NodeRequirement {
    $current = Get-NodeVersion
    return $current -and $current -ge $requiredNodeVersion
}

function Refresh-Path {
    $machine = [System.Environment]::GetEnvironmentVariable("Path", "Machine")
    $user = [System.Environment]::GetEnvironmentVariable("Path", "User")
    $env:Path = "$machine;$user"
}

function Ensure-Node22 {
    Emit-Progress 15 "ensuring_node"
    if (Test-NodeRequirement) {
        Write-Output "INFO: using Node.js $(node -v)"
        return
    }

    if (Get-Command winget -ErrorAction SilentlyContinue) {
        Write-Output "INFO: installing Node.js 22 with winget"
        winget install OpenJS.NodeJS.LTS --source winget --accept-package-agreements --accept-source-agreements
        Refresh-Path
    } elseif (Get-Command choco -ErrorAction SilentlyContinue) {
        Write-Output "INFO: installing Node.js 22 with Chocolatey"
        choco install nodejs-lts -y
        Refresh-Path
    } elseif (Get-Command scoop -ErrorAction SilentlyContinue) {
        Write-Output "INFO: installing Node.js 22 with Scoop"
        scoop install nodejs-lts
        Refresh-Path
    }

    if (-not (Test-NodeRequirement)) {
        throw "OpenClaw requires Node.js >= $requiredNodeVersion. Install Node.js 22+ and retry."
    }

    Write-Output "INFO: using Node.js $(node -v)"
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

$packageName = "openclaw@latest"
Emit-Progress 5 "preflight"
Ensure-Node22
if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
    throw "npm is required to install OpenClaw."
}
$registry = Resolve-Registry $packageName
Write-Output "INFO: using npm registry $registry"

Emit-Progress 35 "installing"
npm install -g $packageName --registry $registry

Emit-Progress 80 "verifying"
if (Get-Command openclaw -ErrorAction SilentlyContinue) {
    $cmd = (Get-Command openclaw).Source
    Write-Output "INFO: installed binary: $cmd"
    try { openclaw --version } catch {}
    Write-Output "INFO: run 'openclaw onboard --install-daemon' to finish interactive onboarding."
}

Emit-Progress 100 "complete"
Write-Output "INFO: OpenClaw installation complete"
