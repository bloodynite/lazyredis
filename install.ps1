#Requires -Version 5.0
<#
.SYNOPSIS
Install lazyredis from GitHub Releases (Windows).

.DESCRIPTION
Downloads the Windows release binary for your CPU architecture, verifies
its SHA256 against the published SHA256SUMS, and installs it into a
user-writable directory (no admin required). Adds the install directory
to the user PATH for the current and future sessions.

.PARAMETER Version
Release tag (e.g. v0.2.0). Default: latest. Falls back to the
LAZYREDIS_VERSION environment variable.

.PARAMETER InstallDir
Destination directory. Default: $env:LOCALAPPDATA\Programs\lazyredis.
Falls back to the LAZYREDIS_INSTALL_DIR environment variable.

.EXAMPLE
iwr -useb https://raw.githubusercontent.com/bloodynite/lazyredis/main/install.ps1 | iex

.EXAMPLE
$env:LAZYREDIS_VERSION='v0.2.0'; iwr -useb https://raw.githubusercontent.com/bloodynite/lazyredis/main/install.ps1 | iex

.EXAMPLE
.\install.ps1 -Version v0.2.0 -InstallDir C:\Tools\lazyredis
#>
[CmdletBinding()]
param(
    [string]$Version,
    [string]$InstallDir
)

$ErrorActionPreference = "Stop"

$Repo = "bloodynite/lazyredis"
$BinName = "lazyredis.exe"
$RepoUrl = "https://github.com/$Repo"

if (-not $Version)    { $Version    = $env:LAZYREDIS_VERSION }
if (-not $Version)    { $Version    = "latest" }
if (-not $InstallDir) { $InstallDir = $env:LAZYREDIS_INSTALL_DIR }

function Write-Info([string]$msg) {
    Write-Host $msg
}

function Resolve-Latest {
    $url = "https://api.github.com/repos/$Repo/releases/latest"
    $resp = Invoke-RestMethod -Uri $url -TimeoutSec 30
    if (-not $resp.tag_name) {
        throw "could not parse tag_name from latest release"
    }
    return $resp.tag_name
}

switch -Wildcard ($Version) {
    "latest" { $Version = Resolve-Latest }
    "v*"     { }
    default  { throw "Version must be 'latest' or start with 'v' (got: $Version)" }
}

if (-not $InstallDir) {
    if (-not $env:LOCALAPPDATA) {
        throw "LOCALAPPDATA is not set; pass -InstallDir explicitly"
    }
    $InstallDir = Join-Path $env:LOCALAPPDATA "Programs\lazyredis"
}

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { "amd64" }
    "ARM64" { "arm64" }
    default { throw "unsupported architecture: $($env:PROCESSOR_ARCHITECTURE)" }
}

$asset    = "lazyredis-windows-$arch.exe"
$baseUrl  = "$RepoUrl/releases/download/$Version"
$binUrl   = "$baseUrl/$asset"
$sumsUrl  = "$baseUrl/SHA256SUMS"

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("lazyredis-" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmp | Out-Null

try {
    Write-Info "Installing lazyredis $Version to $InstallDir\$BinName"

    $binPath = Join-Path $tmp $asset
    Write-Info "Downloading $binUrl"
    Invoke-WebRequest -Uri $binUrl -OutFile $binPath -UseBasicParsing

    try {
        $sumsPath = Join-Path $tmp "SHA256SUMS"
        Invoke-WebRequest -Uri $sumsUrl -OutFile $sumsPath -UseBasicParsing -ErrorAction Stop
    } catch [System.Net.WebException] {
        $code = $null
        if ($_.Exception.Response) { $code = [int]$_.Exception.Response.StatusCode }
        if ($code -eq 404) {
            Write-Info "warning: SHA256SUMS not published for $Version; skipping integrity check"
        } else {
            throw
        }
    }

    if (Test-Path (Join-Path $tmp "SHA256SUMS")) {
        $expected = $null
        Get-Content (Join-Path $tmp "SHA256SUMS") | ForEach-Object {
            $parts = $_ -split '\s+', 2
            if ($parts.Length -eq 2 -and $parts[1] -eq $asset) {
                $script:expected = $parts[0]
            }
        }
        if ($expected) {
            $actual = (Get-FileHash -Algorithm SHA256 -Path $binPath).Hash.ToLower()
            if ($actual -ne $expected.ToLower()) {
                throw "SHA256 mismatch for $asset`nexpected: $expected`nactual:   $actual"
            }
            Write-Info "SHA256 verified"
        } else {
            Write-Info "warning: SHA256SUMS does not list $asset; skipping integrity check"
        }
    }

    $dest = Join-Path $InstallDir $BinName
    Move-Item -Path $binPath -Destination $dest -Force
    Write-Info "Installed $BinName $Version to $dest"

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
        $env:Path = "$env:Path;$InstallDir"
        Write-Info "Added $InstallDir to your PATH (current and future sessions)"
    }

    Write-Info ""
    Write-Info "Restart your shell and run: lazyredis --version"
} finally {
    if (Test-Path $tmp) {
        Remove-Item -Path $tmp -Recurse -Force -ErrorAction SilentlyContinue
    }
}