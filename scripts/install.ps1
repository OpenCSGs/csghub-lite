Param(
    [string]$InstallDir = "$HOME\bin"
)

$ErrorActionPreference = "Stop"

$Repo = "OpenCSGs/csghub-lite"
$BinaryName = "csghub-lite.exe"
$LlamaCppRepo = "ggml-org/llama.cpp"

$GitHubApi = "https://api.github.com/repos"
$GitLabHost = "https://git-devops.opencsg.com"
$GitLabApi = "$GitLabHost/api/v4/projects"
$GitLabCsghubId = "392"
$GitLabLlamaId = "393"

function Info([string]$msg) { Write-Host "[INFO] $msg" -ForegroundColor Green }
function Warn([string]$msg) { Write-Host "[WARN] $msg" -ForegroundColor Yellow }
function Fail([string]$msg) { Write-Host "[ERROR] $msg" -ForegroundColor Red; exit 1 }

function Detect-Region {
    $region = $env:CSGHUB_LITE_REGION
    if ($region) { return $region }
    try {
        $country = (Invoke-WebRequest -Uri "https://ipinfo.io/country" -UseBasicParsing -TimeoutSec 5).Content.Trim()
        if ($country -eq "CN") { return "CN" }
        if ($country) { return "INTL" }
    } catch {}
    return "CN"
}

function Download-File([string]$Url, [string]$OutFile) {
    Invoke-WebRequest -Uri $Url -OutFile $OutFile -UseBasicParsing
}

function Try-Download {
    param([string]$OutFile, [string[]]$Urls)
    foreach ($url in $Urls) {
        try {
            Info "Trying $url"
            Download-File -Url $url -OutFile $OutFile
            Info "Downloaded from $url"
            return $true
        } catch {
            Warn "Failed: $url"
        }
    }
    return $false
}

function Try-DownloadText {
    param([string[]]$Urls)
    foreach ($url in $Urls) {
        try {
            return (Invoke-RestMethod -Uri $url -UseBasicParsing -TimeoutSec 30)
        } catch {
            continue
        }
    }
    return $null
}

function Region-Download {
    param([string]$OutFile, [string]$GitHubUrl, [string]$GitLabUrl)
    if ($script:Region -eq "CN") {
        return Try-Download -OutFile $OutFile -Urls @($GitLabUrl, $GitHubUrl)
    } else {
        return Try-Download -OutFile $OutFile -Urls @($GitHubUrl, $GitLabUrl)
    }
}

function Region-DownloadText {
    param([string]$GitHubUrl, [string]$GitLabUrl)
    if ($script:Region -eq "CN") {
        return Try-DownloadText -Urls @($GitLabUrl, $GitHubUrl)
    } else {
        return Try-DownloadText -Urls @($GitHubUrl, $GitLabUrl)
    }
}

function Get-LatestVersion {
    $ghUrl = "$GitHubApi/$Repo/releases/latest"
    $glUrl = "$GitLabApi/$GitLabCsghubId/releases/permalink/latest"
    $release = Region-DownloadText -GitHubUrl $ghUrl -GitLabUrl $glUrl
    if ($release -and $release.tag_name) { return $release.tag_name }
    return $null
}

function Ensure-PathContains([string]$dir) {
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $parts = @()
    if ($userPath) { $parts = $userPath.Split(';') }
    if ($parts -notcontains $dir) {
        $newPath = if ($userPath) { "$dir;$userPath" } else { $dir }
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        if ($env:Path -notlike "*$dir*") {
            $env:Path = "$dir;$env:Path"
        }
        Info "Added $dir to PATH."
    }
}

function Install-CsghubLite {
    $arch = $env:PROCESSOR_ARCHITECTURE
    switch ($arch) {
        "AMD64" { $archToken = "amd64" }
        "ARM64" { $archToken = "arm64" }
        default { Fail "Unsupported architecture: $arch" }
    }

    $version = if ($env:CSGHUB_LITE_VERSION) { $env:CSGHUB_LITE_VERSION } else { Get-LatestVersion }
    if (-not $version) { Fail "Could not determine latest version. Set CSGHUB_LITE_VERSION manually." }
    Info "Version: $version"

    $versionNum = $version.TrimStart('v')
    $archiveName = "csghub-lite_${versionNum}_windows-${archToken}.zip"
    $githubUrl = "https://github.com/$Repo/releases/download/$version/$archiveName"
    $gitlabUrl = "$GitLabApi/$GitLabCsghubId/packages/generic/csghub-lite/${versionNum}/${archiveName}"

    $tmpDir = Join-Path ([IO.Path]::GetTempPath()) ("csghub-lite-install-" + [Guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null
    $zipPath = Join-Path $tmpDir $archiveName

    if (-not (Region-Download -OutFile $zipPath -GitHubUrl $githubUrl -GitLabUrl $gitlabUrl)) {
        Fail "Failed to download csghub-lite."
    }

    Expand-Archive -Path $zipPath -DestinationPath $tmpDir -Force
    $bin = Get-ChildItem -Path $tmpDir -Recurse -Filter "csghub-lite.exe" | Select-Object -First 1
    if (-not $bin) { Fail "csghub-lite.exe not found in archive." }

    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    $target = Join-Path $InstallDir "csghub-lite.exe"
    Copy-Item -Path $bin.FullName -Destination $target -Force
    Ensure-PathContains -dir $InstallDir
    Info "Installed csghub-lite to $target"
}

function Install-LlamaServer {
    $existingLlama = Get-Command "llama-server.exe" -ErrorAction SilentlyContinue
    if ($existingLlama) {
        Info "llama-server found at $($existingLlama.Source), upgrading to latest version..."
    } else {
        Warn "llama-server not found. It is required for model inference."
    }

    $customCmd = $env:CSGHUB_LITE_LLAMA_CPP_INSTALL_CMD
    if ($customCmd) {
        Info "Installing llama.cpp via custom command..."
        try {
            powershell -NoProfile -ExecutionPolicy Bypass -Command $customCmd | Out-Null
            if (Get-Command "llama-server.exe" -ErrorAction SilentlyContinue) {
                Info "llama-server installed."
                return
            }
        } catch {
            Warn "Custom install command failed: $customCmd"
        }
    }

    # Get latest llama.cpp release tag
    $ghUrl = "$GitHubApi/$LlamaCppRepo/releases/latest"
    $glUrl = "$GitLabApi/$GitLabLlamaId/releases/permalink/latest"
    $release = Region-DownloadText -GitHubUrl $ghUrl -GitLabUrl $glUrl
    if (-not $release -or -not $release.tag_name) {
        Warn "Failed to query llama.cpp release metadata."
        return
    }

    $llamaTag = $release.tag_name
    Info "llama.cpp release: $llamaTag"

    $arch = $env:PROCESSOR_ARCHITECTURE
    $archToken = if ($arch -eq "AMD64") { "x64" } elseif ($arch -eq "ARM64") { "arm64" } else { $null }
    if (-not $archToken) {
        Warn "Unsupported architecture for llama-server: $arch"
        return
    }

    $hasCuda = [bool](Get-Command "nvidia-smi" -ErrorAction SilentlyContinue)

    # Build ordered list of candidate assets (best match first)
    $candidates = @()
    $cudartName = $null
    if ($hasCuda) {
        Info "NVIDIA GPU detected, trying CUDA build first."
        $candidates += @{ Asset = "llama-${llamaTag}-bin-win-cuda-12.4-${archToken}.zip"; Cudart = "cudart-llama-bin-win-cuda-12.4-${archToken}.zip" }
        $candidates += @{ Asset = "llama-${llamaTag}-bin-win-vulkan-${archToken}.zip"; Cudart = $null }
    }
    $candidates += @{ Asset = "llama-${llamaTag}-bin-win-cpu-${archToken}.zip"; Cudart = $null }

    $tmpDir = Join-Path ([IO.Path]::GetTempPath()) ("llama-install-" + [Guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null

    $downloaded = $false
    $assetName = $null
    foreach ($c in $candidates) {
        $tryAsset = $c.Asset
        $githubDl = "https://github.com/$LlamaCppRepo/releases/download/$llamaTag/$tryAsset"
        $gitlabDl = "$GitLabApi/$GitLabLlamaId/packages/generic/llama-cpp/$llamaTag/$tryAsset"
        $zipPath = Join-Path $tmpDir $tryAsset
        if (Region-Download -OutFile $zipPath -GitHubUrl $githubDl -GitLabUrl $gitlabDl) {
            $assetName = $tryAsset
            $cudartName = $c.Cudart
            $downloaded = $true
            break
        }
        Warn "Asset $tryAsset not available, trying next option..."
    }
    if (-not $downloaded) {
        Warn "Failed to download llama.cpp."
        return
    }
    Info "Downloaded $assetName"

    if ($cudartName) {
        $cudartGh = "https://github.com/$LlamaCppRepo/releases/download/$llamaTag/$cudartName"
        $cudartGl = "$GitLabApi/$GitLabLlamaId/packages/generic/llama-cpp/$llamaTag/$cudartName"
        $cudartZip = Join-Path $tmpDir $cudartName
        if (Region-Download -OutFile $cudartZip -GitHubUrl $cudartGh -GitLabUrl $cudartGl) {
            Expand-Archive -Path $cudartZip -DestinationPath $tmpDir -Force
        } else {
            Warn "Failed to download CUDA runtime. GPU acceleration may not work."
        }
    }

    Expand-Archive -Path $zipPath -DestinationPath $tmpDir -Force
    $server = Get-ChildItem -Path $tmpDir -Recurse -Filter "llama-server.exe" | Select-Object -First 1
    if (-not $server) {
        Warn "llama-server.exe not found in archive."
        return
    }

    $llamaInstallDir = $InstallDir
    if ($existingLlama) {
        $llamaInstallDir = Split-Path $existingLlama.Source -Parent
    }

    New-Item -ItemType Directory -Force -Path $llamaInstallDir | Out-Null
    Copy-Item -Path $server.FullName -Destination (Join-Path $llamaInstallDir "llama-server.exe") -Force
    Get-ChildItem -Path $server.Directory.FullName -Filter "*.dll" | ForEach-Object {
        Copy-Item -Path $_.FullName -Destination (Join-Path $llamaInstallDir $_.Name) -Force
    }
    Ensure-PathContains -dir $llamaInstallDir
    Info "Installed llama-server to $llamaInstallDir"
}

function Check-Existing {
    $existing = Get-Command "csghub-lite.exe" -ErrorAction SilentlyContinue
    if (-not $existing) {
        Info "No existing installation found."
        return
    }

    $oldVersion = "unknown"
    try {
        $oldVersion = (& $existing.Source --version 2>$null) | Select-Object -First 1
        if (-not $oldVersion) { $oldVersion = "unknown" }
    } catch {}

    Write-Host ""
    Warn "Existing installation detected:"
    Write-Host "  Binary:  $($existing.Source)"
    Write-Host "  Version: $oldVersion"

    $procs = Get-Process -Name "csghub-lite" -ErrorAction SilentlyContinue
    $hasRunning = $false
    if ($procs) {
        $hasRunning = $true
        Warn "Running csghub-lite process(es) detected."
    }

    if ($env:CSGHUB_LITE_FORCE -eq "1") {
        if ($hasRunning) {
            Info "Force mode: stopping running processes..."
            $procs | Stop-Process -Force -ErrorAction SilentlyContinue
            Start-Sleep -Seconds 2
        }
        return
    }

    Write-Host ""
    if ($hasRunning) {
        $prompt = "Stop running instances and replace with the new version? [y/N]"
    } else {
        $prompt = "Replace the existing installation? [y/N]"
    }

    $answer = Read-Host $prompt
    if ($answer -notmatch '^[yY](es)?$') {
        Info "Installation cancelled."
        exit 0
    }

    if ($hasRunning) {
        Info "Stopping running processes..."
        $procs | Stop-Process -Force -ErrorAction SilentlyContinue
        Start-Sleep -Seconds 2
    }
}

function Install-PythonDeps {
    $pythonPkgs = @("torch", "safetensors", "gguf", "transformers")

    $python = $null
    foreach ($name in @("python3", "python")) {
        $cmd = Get-Command $name -ErrorAction SilentlyContinue
        if ($cmd) {
            try {
                $ver = & $cmd.Source -c "import sys; print(sys.version_info.major)" 2>$null
                if ($ver -eq "3") {
                    $python = $cmd.Source
                    break
                }
            } catch {}
        }
    }

    if (-not $python) {
        Warn "Python 3 not found. It is required to convert SafeTensors models to GGUF."
        $answer = Read-Host "Install Python 3? You can download from https://www.python.org/downloads/ [y/N]"
        if ($answer -match '^[yY](es)?$') {
            Info "Opening Python download page..."
            Start-Process "https://www.python.org/downloads/"
            Warn "After installing Python, re-run this script or manually install: pip install torch safetensors gguf"
        } else {
            Warn "Skipping Python setup. SafeTensors auto-conversion will not be available."
        }
        return
    }

    Info "Python 3 found: $(& $python --version 2>&1)"

    $missing = @()
    foreach ($pkg in $pythonPkgs) {
        try {
            & $python -c "import $pkg" 2>$null
            if ($LASTEXITCODE -ne 0) { $missing += $pkg }
        } catch {
            $missing += $pkg
        }
    }

    if ($missing.Count -eq 0) {
        Info "Python dependencies already installed (torch, safetensors, gguf, transformers)."
        return
    }

    $missingStr = $missing -join ", "
    Warn "Missing Python packages: $missingStr"
    $answer = Read-Host "Install them now? (CPU-only torch, ~300MB) [y/N]"
    if ($answer -match '^[yY](es)?$') {
        $pkgList = $missing -join " "
        Info "Installing: $pkgList..."
        $pipArgs = @("-m", "pip", "install")
        if ($missing -contains "torch") {
            $pipArgs += @("--extra-index-url", "https://download.pytorch.org/whl/cpu")
        }
        $pipArgs += $missing
        & $python @pipArgs
        if ($LASTEXITCODE -eq 0) {
            Info "Python dependencies installed successfully."
        } else {
            Warn "pip install failed. Try manually: $python -m pip install $pkgList"
        }
    } else {
        Warn "Skipping. Install later with: $python -m pip install $($missing -join ' ')"
    }
}

# ---- Main ----
$script:Region = Detect-Region
Info "Detected region: $script:Region"

Info "Checking for existing installation..."
Check-Existing

Info "Installing csghub-lite..."
Install-CsghubLite

$autoInstall = if ($env:CSGHUB_LITE_AUTO_INSTALL_LLAMA_SERVER) { $env:CSGHUB_LITE_AUTO_INSTALL_LLAMA_SERVER } else { "1" }
if ($autoInstall -eq "1") {
    Install-LlamaServer
}

Info "Checking Python environment for model conversion..."
Install-PythonDeps

Write-Host ""
Write-Host "Quick start:" -ForegroundColor White
Write-Host "  csghub-lite serve                       # Start server with Web UI"
Write-Host "  csghub-lite run Qwen/Qwen3-0.6B-GGUF    # Run a model"
Write-Host "  csghub-lite ps                          # List running models"
Write-Host "  csghub-lite login                       # Set CSGHub token"
Write-Host "  csghub-lite --help                      # Show all commands"
Write-Host ""
Write-Host "Web UI:" -ForegroundColor White
Write-Host "  Start the server and open " -NoNewline
Write-Host "http://localhost:11435" -ForegroundColor Cyan -NoNewline
Write-Host " in your browser."
Write-Host "  Dashboard, Marketplace, Library and Chat are all available."
Write-Host ""
Write-Host "Want more?" -ForegroundColor White
Write-Host "  Visit https://opencsg.com for advanced features,"
Write-Host "  enterprise solutions, and the full CSGHub platform."
Write-Host ""
