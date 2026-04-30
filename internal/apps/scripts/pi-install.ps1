$ErrorActionPreference = "Stop"

function Emit-ProgressLine {
    param(
        [int]$Progress,
        [string]$Phase
    )
    Write-Output "CSGHUB_PROGRESS|$Progress|$Phase"
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
$installRoot = $env:CSGHUB_LITE_PI_INSTALL_ROOT
if ([string]::IsNullOrWhiteSpace($installRoot)) {
    $installRoot = Join-Path $env:USERPROFILE ".local\share\pi-coding-agent"
}
$launcherDir = Join-Path $env:USERPROFILE ".local\bin"
$launcherPath = Join-Path $launcherDir "pi.cmd"
$actualBin = Join-Path $installRoot "pi.cmd"

function Ensure-PiSearchTools {
    Emit-ProgressLine 70 "ensuring_search_tools"

    if (Get-Command fd -ErrorAction SilentlyContinue) {
        Write-Output "INFO: using existing fd"
    } elseif (Get-Command fdfind -ErrorAction SilentlyContinue) {
        $fdfind = (Get-Command fdfind).Source
        Set-Content -Path (Join-Path $launcherDir "fd.cmd") -Encoding ASCII -Value "@echo off`r`nrem csghub-lite generated fd launcher`r`ncall `"$fdfind`" %*`r`n"
        Write-Output "INFO: linked fd to existing fdfind"
    } else {
        $fdPath = Join-Path $launcherDir "fd.cmd"
        Set-Content -Path $fdPath -Encoding ASCII -Value @'
@echo off
rem csghub-lite generated fd fallback
if "%~1"=="--version" (
  echo fd fallback for csghub-lite
  exit /b 0
)
powershell -NoProfile -ExecutionPolicy Bypass -Command "$pattern=$args[0]; if (-not $pattern) { $pattern='' }; Get-ChildItem -Recurse -Force -ErrorAction SilentlyContinue | Where-Object { $_.FullName -notmatch '\\(node_modules|\.git|\.pi|\.csghub-lite)\\' -and ($pattern -eq '' -or $_.Name -like ('*' + $pattern + '*')) } | ForEach-Object { $_.FullName }" %*
'@
        Write-Output "INFO: installed fd fallback launcher: $fdPath"
    }

    if (Get-Command rg -ErrorAction SilentlyContinue) {
        Write-Output "INFO: using existing rg"
    } else {
        $rgPath = Join-Path $launcherDir "rg.cmd"
        Set-Content -Path $rgPath -Encoding ASCII -Value @'
@echo off
rem csghub-lite generated rg fallback
if "%~1"=="--version" (
  echo ripgrep fallback for csghub-lite
  exit /b 0
)
powershell -NoProfile -ExecutionPolicy Bypass -Command "$pattern=$args[0]; $paths=@($args | Select-Object -Skip 1); if (-not $paths -or $paths.Count -eq 0) { $paths=@('.') }; Select-String -Path $paths -Pattern $pattern -Recurse -ErrorAction SilentlyContinue | ForEach-Object { \"$($_.Path):$($_.LineNumber):$($_.Line)\" }" %*
'@
        Write-Output "INFO: installed rg fallback launcher: $rgPath"
    }
}

Emit-ProgressLine 30 "installing_pi"
Write-Output "INFO: installing Pi Coding Agent package $package to $installRoot"
Remove-Item -Recurse -Force $installRoot -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $installRoot, $launcherDir | Out-Null
npm install -g --prefix="$installRoot" --registry="$registry" "$package"
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}
if (-not (Test-Path $actualBin)) {
    Write-Output "ERROR: Pi was installed but npm did not create $actualBin."
    exit 1
}

Set-Content -Path $launcherPath -Encoding ASCII -Value "@echo off`r`ncall `"$actualBin`" %*`r`n"
$env:PATH = "$launcherDir;$env:PATH"
Ensure-PiSearchTools

Emit-ProgressLine 85 "verifying_pi"
if (-not (Get-Command pi -ErrorAction SilentlyContinue) -and -not (Test-Path $launcherPath)) {
    Write-Output "ERROR: Pi was installed but the pi command was not found on PATH."
    Write-Output "INFO: launcher was written to $launcherPath; add $launcherDir to PATH and retry."
    exit 1
}

& $launcherPath --version
Emit-ProgressLine 100 "complete"
Write-Output "INFO: Pi Coding Agent installed successfully."
Write-Output "INFO: updated launcher $launcherPath"
