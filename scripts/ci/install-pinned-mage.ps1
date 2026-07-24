# Installs the checksum-pinned Mage archive from config/toolchain.toml for
# windows-amd64, verifies its SHA-256 against the exact committed pin,
# extracts it, and adds its directory to $env:GITHUB_PATH so subsequent
# workflow steps can invoke "mage" directly.
#
# This exists because an ambient `go install github.com/magefile/mage@...`
# cannot be relied on in CI: it is absent entirely on some hosted macOS
# runners, and even where it succeeds elsewhere, its GOPATH/bin output is
# not on PATH for later steps by default.
#
# This script reads config/toolchain.toml directly rather than duplicating
# its committed URL/checksum values, so it can never drift from the single
# authoritative pin (D-05/D-09: refer, never repeat).
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$TomlPath = Join-Path $RepoRoot "config\toolchain.toml"
$Platform = "windows-amd64"
$Section = '[toolchain.mage.platforms."' + $Platform + '"]'

function Read-Pin {
    param([Parameter(Mandatory = $true)][string]$Key)

    $inSection = $false
    foreach ($line in Get-Content -LiteralPath $TomlPath) {
        if ($line -eq $Section) {
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

$ArchiveUrl = Read-Pin -Key "archive_url"
$ArchiveSha256 = Read-Pin -Key "archive_sha256"
if (-not $ArchiveUrl -or -not $ArchiveSha256) {
    throw "install-pinned-mage.ps1: no committed Mage pin for platform $Platform in $TomlPath"
}

$TempRoot = if ($env:RUNNER_TEMP) { $env:RUNNER_TEMP } else { [System.IO.Path]::GetTempPath() }
$Staging = Join-Path $TempRoot ("mage-install-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $Staging -Force | Out-Null
$ArchivePath = Join-Path $Staging "mage.zip"
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
    # Only the downloaded archive is cleaned up. $ExtractDir must survive
    # this script's exit: its mage.exe is what $env:GITHUB_PATH now points
    # at, and later workflow steps (mage Bootstrap, etc.) need it to still
    # exist on disk.
    Remove-Item -LiteralPath $ArchivePath -Force -ErrorAction SilentlyContinue
}

$MageBinary = Get-ChildItem -Path $ExtractDir -Filter "mage.exe" -Recurse | Select-Object -First 1
if (-not $MageBinary) {
    throw "install-pinned-mage.ps1: extracted archive does not contain mage.exe"
}
Add-Content -LiteralPath $env:GITHUB_PATH -Value $MageBinary.DirectoryName
Write-Output "install-pinned-mage.ps1: installed checksum-verified Mage $Platform from $ArchiveUrl"
