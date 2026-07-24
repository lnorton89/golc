# Installs the checksum-pinned Go and Mage archives from
# config/toolchain.toml for windows-amd64, verifies each SHA-256 against
# its exact committed pin, extracts it, and adds its binary directory to
# $env:GITHUB_PATH so subsequent workflow steps can invoke "go"/"mage"
# directly.
#
# Both are installed, not just Mage: Mage itself always needs *some* Go
# compiler on PATH regardless of how the mage binary was obtained,
# because it JIT-compiles the magefile package at every invocation
# rather than shipping a precompiled runner. Installing the project's
# own pinned Go here, the same way every other route in this project
# already refuses to trust an ambient toolchain
# (resolvePinnedGoExecutable never does a host PATH lookup either),
# means Mage's JIT compile never depends on whatever Go version (if any)
# a given hosted runner image happens to ship.
#
# This script reads config/toolchain.toml directly rather than duplicating
# its committed URL/checksum values, so it can never drift from the single
# authoritative pin (D-05/D-09: refer, never repeat).
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$TomlPath = Join-Path $RepoRoot "config\toolchain.toml"
$Platform = "windows-amd64"
$TomlLines = Get-Content -LiteralPath $TomlPath

function Read-Pin {
    param(
        [Parameter(Mandatory = $true)][string]$Tool,
        [Parameter(Mandatory = $true)][string]$Key
    )
    $section = '[toolchain.' + $Tool + '.platforms."' + $Platform + '"]'
    $inSection = $false
    foreach ($line in $TomlLines) {
        if ($line -eq $section) {
            $inSection = $true
            continue
        }
        if ($inSection -and $line -match '^\[') {
            break
        }
        if ($inSection -and $line -match ('^' + [regex]::Escape($Key) + '\s*=\s*"([^"]*)"')) {
            return $Matches[1]
        }
    }
    return $null
}

$TempRoot = if ($env:RUNNER_TEMP) { $env:RUNNER_TEMP } else { [System.IO.Path]::GetTempPath() }

function Install-PinnedArchive {
    param([Parameter(Mandatory = $true)][string]$Tool)

    $ArchiveUrl = Read-Pin -Tool $Tool -Key "archive_url"
    $ArchiveSha256 = Read-Pin -Tool $Tool -Key "archive_sha256"
    if (-not $ArchiveUrl -or -not $ArchiveSha256) {
        throw "install-pinned-mage.ps1: no committed $Tool pin for platform $Platform in $TomlPath"
    }

    $Staging = Join-Path $TempRoot ("$Tool-install-" + [guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $Staging -Force | Out-Null
    $ArchivePath = Join-Path $Staging "$Tool.zip"
    $ExtractDir = Join-Path $Staging "extracted"
    try {
        Invoke-WebRequest -Uri $ArchiveUrl -OutFile $ArchivePath -UseBasicParsing

        $Actual = (Get-FileHash -LiteralPath $ArchivePath -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($Actual -ne $ArchiveSha256) {
            throw "install-pinned-mage.ps1: checksum mismatch for ${ArchiveUrl}: got $Actual, want $ArchiveSha256"
        }

        Expand-Archive -LiteralPath $ArchivePath -DestinationPath $ExtractDir -Force
    }
    finally {
        # Only the downloaded archive is cleaned up. $ExtractDir must
        # survive this function's return: later workflow steps (mage
        # Bootstrap, etc.) need the extracted binary to still exist.
        Remove-Item -LiteralPath $ArchivePath -Force -ErrorAction SilentlyContinue
    }
    return $ExtractDir
}

$GoExtractDir = Install-PinnedArchive -Tool "go"
$GoBinary = Join-Path $GoExtractDir "go\bin\go.exe"
if (-not (Test-Path -LiteralPath $GoBinary)) {
    throw "install-pinned-mage.ps1: extracted Go archive does not contain go\bin\go.exe"
}
Add-Content -LiteralPath $env:GITHUB_PATH -Value (Split-Path -Parent $GoBinary)
Write-Output "install-pinned-mage.ps1: installed checksum-verified Go $Platform"

$MageExtractDir = Install-PinnedArchive -Tool "mage"
$MageBinary = Get-ChildItem -Path $MageExtractDir -Filter "mage.exe" -Recurse | Select-Object -First 1
if (-not $MageBinary) {
    throw "install-pinned-mage.ps1: extracted Mage archive does not contain mage.exe"
}
Add-Content -LiteralPath $env:GITHUB_PATH -Value $MageBinary.DirectoryName
Write-Output "install-pinned-mage.ps1: installed checksum-verified Mage $Platform"
