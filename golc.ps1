<#
.SYNOPSIS
GOLC root command shim (Windows PowerShell 5.1).

.DESCRIPTION
The only supported contributor entrypoint. `bootstrap` provisions pinned
project-local tools from exact archive pins in config/toolchain.toml:
archive bytes are verified against a committed SHA-256 before extraction,
extraction happens in a staging directory and is promoted atomically, and a
matching installed manifest makes a second bootstrap perform zero
archive-source calls. Every other subcommand delegates to the pinned
project-local command. Bootstrap treats manifest pins as immutable inputs:
it never consults an update feed and never rewrites go.mod, go.sum, or
config/toolchain.toml.
#>
[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [string]$Command = "",

    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$CommandArguments = @()
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$RepoRoot = $PSScriptRoot
$ToolchainManifestPath = Join-Path $RepoRoot "config\toolchain.toml"
$DownloadsDirectory = Join-Path $RepoRoot ".tools\cache\downloads"
$GoModCacheDirectory = Join-Path $RepoRoot ".tools\cache\go-mod"
$GoBuildCacheDirectory = Join-Path $RepoRoot ".tools\cache\go-build"
$RecordDirectory = Join-Path $RepoRoot ".tools\manifest"
$GolcProjectExecutable = Join-Path $RepoRoot ".tools\installs\golc_project\bin\golc-project.exe"
$InstallManifestName = ".golc-install-manifest.json"

Add-Type -AssemblyName System.IO.Compression.FileSystem

function Read-GolcToml {
    <# Minimal bootstrap-safe reader for config/toolchain.toml: sections,
       quoted string values, and bare integers only. #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path
    )

    $sections = @{ "" = @{} }
    $current = ""
    $lineNumber = 0
    foreach ($rawLine in [System.IO.File]::ReadAllLines($Path)) {
        $lineNumber++
        $line = $rawLine.Trim()
        if ($line -eq "" -or $line.StartsWith("#")) {
            continue
        }
        if ($line -match '^\[(?<name>[A-Za-z0-9_.-]+)\]$') {
            $current = $Matches["name"]
            if (-not $sections.ContainsKey($current)) {
                $sections[$current] = @{}
            }
            continue
        }
        if ($line -match '^(?<key>[A-Za-z0-9_-]+)\s*=\s*(?<value>.+)$') {
            $key = $Matches["key"]
            $rawValue = $Matches["value"].Trim()
            if ($rawValue -match '^"(?<text>[^"]*)"$') {
                $sections[$current][$key] = $Matches["text"]
            }
            elseif ($rawValue -match '^\d+$') {
                $sections[$current][$key] = [int]$rawValue
            }
            else {
                throw "GOLC_TOOLCHAIN_PARSE: unsupported value at ${Path}:${lineNumber}"
            }
            continue
        }
        throw "GOLC_TOOLCHAIN_PARSE: unsupported syntax at ${Path}:${lineNumber}"
    }
    return $sections
}

function Get-RequiredPinValue {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [hashtable]$Section,

        [Parameter(Mandatory = $true)]
        [string]$SectionName,

        [Parameter(Mandatory = $true)]
        [string]$Key
    )

    if (-not $Section.ContainsKey($Key)) {
        throw "GOLC_TOOLCHAIN_PARSE: [$SectionName] is missing required key '$Key'"
    }
    return $Section[$Key]
}

function Assert-Sha256Pin {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$Sha256
    )

    $normalized = $Sha256.Trim().ToLowerInvariant()
    if ($normalized -notmatch '^[0-9a-f]{64}$') {
        throw "GOLC_BOOTSTRAP_CHECKSUM_FORMAT: pin '$Sha256' is not a lowercase 64-character SHA-256"
    }
    return $normalized
}

function Test-InstalledManifestMatches {
    <# Mirrors the internal/bootstrap InstalledMatches contract at the shim
       layer: a readable install manifest recording the same exact archive
       pin means the archive source is never consulted. #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$InstallDir,

        [Parameter(Mandatory = $true)]
        [string]$Sha256
    )

    $manifestPath = Join-Path $InstallDir $InstallManifestName
    if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
        return $false
    }
    try {
        $manifest = [System.IO.File]::ReadAllText($manifestPath) | ConvertFrom-Json
    }
    catch {
        return $false
    }
    $property = $manifest.PSObject.Properties["archive_sha256"]
    if ($null -eq $property) {
        return $false
    }
    return ($property.Value -ceq $Sha256)
}

function Assert-ArchiveEntriesContained {
    <# Rejects archive entries that could escape the extraction root before
       any byte is extracted. #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$ArchivePath
    )

    $archive = [System.IO.Compression.ZipFile]::OpenRead($ArchivePath)
    try {
        foreach ($entry in $archive.Entries) {
            $name = $entry.FullName
            if ([string]::IsNullOrWhiteSpace($name)) {
                throw "GOLC_BOOTSTRAP_TRAVERSAL: empty archive entry name"
            }
            $normalized = $name.Replace("\", "/")
            if ($normalized.StartsWith("/") -or $normalized.Contains(":")) {
                throw "GOLC_BOOTSTRAP_TRAVERSAL: rooted archive entry '$name'"
            }
            foreach ($segment in $normalized.Split("/")) {
                if ($segment -eq "..") {
                    throw "GOLC_BOOTSTRAP_TRAVERSAL: dot-dot segment in archive entry '$name'"
                }
            }
        }
    }
    finally {
        $archive.Dispose()
    }
}

function Get-VerifiedArchive {
    <# Returns a local archive path whose bytes match the exact SHA-256 pin.
       A verified copy in the downloads cache is reused without any
       archive-source call; otherwise the source is fetched to a staging
       file, verified, and promoted into the cache. #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$Uri,

        [Parameter(Mandatory = $true)]
        [string]$Sha256
    )

    $expected = Assert-Sha256Pin -Sha256 $Sha256
    New-Item -ItemType Directory -Path $DownloadsDirectory -Force | Out-Null
    $cachedPath = Join-Path $DownloadsDirectory ($expected + ".zip")
    if (Test-Path -LiteralPath $cachedPath -PathType Leaf) {
        $cachedHash = (Get-FileHash -LiteralPath $cachedPath -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($cachedHash -eq $expected) {
            return $cachedPath
        }
        Remove-Item -LiteralPath $cachedPath -Force
    }

    $parsedUri = [System.Uri]::new($Uri)
    $stagingPath = Join-Path $DownloadsDirectory (".partial-" + [guid]::NewGuid().ToString("N") + ".zip")
    try {
        if ($parsedUri.Scheme -eq "file") {
            $sourcePath = $parsedUri.LocalPath
            if (-not (Test-Path -LiteralPath $sourcePath -PathType Leaf)) {
                throw "GOLC_BOOTSTRAP_OFFLINE_ARTIFACT_MISSING: $sourcePath"
            }
            Copy-Item -LiteralPath $sourcePath -Destination $stagingPath -Force
        }
        elseif ($parsedUri.Scheme -eq "http" -or $parsedUri.Scheme -eq "https") {
            [System.Net.ServicePointManager]::SecurityProtocol = `
                [System.Net.ServicePointManager]::SecurityProtocol -bor [System.Net.SecurityProtocolType]::Tls12
            $client = New-Object System.Net.WebClient
            try {
                $client.DownloadFile($parsedUri.AbsoluteUri, $stagingPath)
            }
            catch {
                throw "GOLC_BOOTSTRAP_DOWNLOAD_FAILED: $Uri :: $($_.Exception.Message)"
            }
            finally {
                $client.Dispose()
            }
        }
        else {
            throw "GOLC_TOOLCHAIN_PARSE: unsupported archive scheme '$($parsedUri.Scheme)'"
        }

        $actual = (Get-FileHash -LiteralPath $stagingPath -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($actual -ne $expected) {
            throw "GOLC_BOOTSTRAP_CHECKSUM_MISMATCH: $Uri has sha256 $actual, pin requires $expected"
        }
        Move-Item -LiteralPath $stagingPath -Destination $cachedPath -Force
        return $cachedPath
    }
    finally {
        if (Test-Path -LiteralPath $stagingPath) {
            Remove-Item -LiteralPath $stagingPath -Force -ErrorAction SilentlyContinue
        }
    }
}

function Install-PinnedArchive {
    <# Extracts a verified archive into a staging directory beside the
       install target and promotes it with a single rename, recording the
       install manifest that later makes bootstrap skip the archive source. #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$ArchivePath,

        [Parameter(Mandatory = $true)]
        [string]$Sha256,

        [Parameter(Mandatory = $true)]
        [string]$InstallDir
    )

    $expected = Assert-Sha256Pin -Sha256 $Sha256
    Assert-ArchiveEntriesContained -ArchivePath $ArchivePath

    $parent = Split-Path -Parent $InstallDir
    New-Item -ItemType Directory -Path $parent -Force | Out-Null
    $stagingDir = Join-Path $parent (".golc-staging-" + [guid]::NewGuid().ToString("N"))
    try {
        [System.IO.Compression.ZipFile]::ExtractToDirectory($ArchivePath, $stagingDir)
        $fileCount = @(Get-ChildItem -LiteralPath $stagingDir -Recurse -Force -File).Count
        $manifestJson = [ordered]@{
            archive_sha256 = $expected
            file_count     = $fileCount
        } | ConvertTo-Json
        $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
        [System.IO.File]::WriteAllText((Join-Path $stagingDir $InstallManifestName), $manifestJson + "`n", $utf8NoBom)

        if (Test-Path -LiteralPath $InstallDir) {
            Remove-Item -LiteralPath $InstallDir -Recurse -Force
        }
        Move-Item -LiteralPath $stagingDir -Destination $InstallDir
    }
    catch {
        if (Test-Path -LiteralPath $stagingDir) {
            Remove-Item -LiteralPath $stagingDir -Recurse -Force -ErrorAction SilentlyContinue
        }
        throw
    }
}

function Install-ArchivePin {
    <# Provisions one exact archive pin. A matching installed manifest means
       zero archive-source calls; anything else verifies bytes before any
       extraction and promotes atomically. #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$DisplayName,

        [Parameter(Mandatory = $true)]
        [string]$Uri,

        [Parameter(Mandatory = $true)]
        [string]$Sha256,

        [Parameter(Mandatory = $true)]
        [string]$InstallDir
    )

    $expected = Assert-Sha256Pin -Sha256 $Sha256
    if (Test-InstalledManifestMatches -InstallDir $InstallDir -Sha256 $expected) {
        Write-Output "GOLC bootstrap: $DisplayName already matches pin $expected; archive source not consulted."
        return
    }
    $archivePath = Get-VerifiedArchive -Uri $Uri -Sha256 $expected
    Install-PinnedArchive -ArchivePath $archivePath -Sha256 $expected -InstallDir $InstallDir
    Write-Output "GOLC bootstrap: $DisplayName installed from checksum-verified archive."
}

function Set-ProjectGoEnvironment {
    <# Repository-local Go paths. GOTOOLCHAIN=local forbids any silent
       toolchain download or host fallback once bootstrap begins. #>
    [CmdletBinding()]
    param()

    $env:GOTOOLCHAIN = "local"
    $env:GOMODCACHE = $GoModCacheDirectory
    $env:GOCACHE = $GoBuildCacheDirectory
    $env:GOFLAGS = "-mod=readonly"
}

function Invoke-ProjectGo {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$GoExecutable,

        [Parameter(Mandatory = $true)]
        [string[]]$Arguments,

        [Parameter(Mandatory = $true)]
        [string]$FailureDiagnostic
    )

    & $GoExecutable @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "${FailureDiagnostic}: go $($Arguments -join ' ') exited $LASTEXITCODE"
    }
}

function Resolve-CacheDirectory {
    <# Cache paths are repository-local by contract: relative, no dot-dot,
       and never rooted outside the checkout. #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$RelativePath
    )

    $normalized = $RelativePath.Replace("/", "\").Trim()
    if ($normalized -eq "" -or $normalized.Contains(":") -or $normalized.StartsWith("\")) {
        throw "GOLC_TOOLCHAIN_PARSE: cache path '$RelativePath' must be repository-relative"
    }
    foreach ($segment in $normalized.Split("\")) {
        if ($segment -eq "..") {
            throw "GOLC_TOOLCHAIN_PARSE: cache path '$RelativePath' escapes the repository"
        }
    }
    return (Join-Path $RepoRoot $normalized)
}

function Invoke-GolcBootstrap {
    [CmdletBinding()]
    param()

    if (-not (Test-Path -LiteralPath $ToolchainManifestPath -PathType Leaf)) {
        throw "GOLC_TOOLCHAIN_MANIFEST_MISSING: config/toolchain.toml"
    }
    $manifest = Read-GolcToml -Path $ToolchainManifestPath

    if ($manifest.ContainsKey("cache")) {
        $cacheSection = $manifest["cache"]
        if ($cacheSection.ContainsKey("downloads")) {
            $script:DownloadsDirectory = Resolve-CacheDirectory -RelativePath $cacheSection["downloads"]
        }
        if ($cacheSection.ContainsKey("gomodcache")) {
            $script:GoModCacheDirectory = Resolve-CacheDirectory -RelativePath $cacheSection["gomodcache"]
        }
        if ($cacheSection.ContainsKey("gocache")) {
            $script:GoBuildCacheDirectory = Resolve-CacheDirectory -RelativePath $cacheSection["gocache"]
        }
    }

    # Tool archive pins (the acceptance harness injects its archive source
    # here; production pins are committed in config/toolchain.toml only).
    foreach ($sectionName in @($manifest.Keys | Where-Object { $_ -like "tools.*" } | Sort-Object)) {
        $toolName = $sectionName.Substring("tools.".Length)
        if ($toolName -notmatch '^[a-z0-9_]+$') {
            throw "GOLC_TOOLCHAIN_PARSE: invalid tool name '$toolName'"
        }
        $section = $manifest[$sectionName]
        Install-ArchivePin `
            -DisplayName $toolName `
            -Uri (Get-RequiredPinValue -Section $section -SectionName $sectionName -Key "archive_uri") `
            -Sha256 (Get-RequiredPinValue -Section $section -SectionName $sectionName -Key "archive_sha256") `
            -InstallDir (Join-Path $RepoRoot (".tools\installs\" + $toolName))
    }

    # Pinned Go toolchain.
    $goExecutable = $null
    if ($manifest.ContainsKey("toolchain.go")) {
        $goSection = $manifest["toolchain.go"]
        $goVersion = Get-RequiredPinValue -Section $goSection -SectionName "toolchain.go" -Key "version"
        $goInstallDir = Join-Path $RepoRoot (".tools\toolchains\go\" + $goVersion + "\windows-amd64")
        Install-ArchivePin `
            -DisplayName ("go " + $goVersion) `
            -Uri (Get-RequiredPinValue -Section $goSection -SectionName "toolchain.go" -Key "archive_url") `
            -Sha256 (Get-RequiredPinValue -Section $goSection -SectionName "toolchain.go" -Key "archive_sha256") `
            -InstallDir $goInstallDir
        $goExecutable = Join-Path $goInstallDir "go\bin\go.exe"
    }

    # Go module cache warm and bootstrap probe: only meaningful where the
    # repository declares its module authority.
    $goModPath = Join-Path $RepoRoot "go.mod"
    if (Test-Path -LiteralPath $goModPath -PathType Leaf) {
        if ($null -eq $goExecutable -or -not (Test-Path -LiteralPath $goExecutable -PathType Leaf)) {
            throw "GOLC_GO_TOOLCHAIN_MISSING: config/toolchain.toml must pin the Go toolchain before module bootstrap"
        }
        Set-ProjectGoEnvironment
        $goSumPath = Join-Path $RepoRoot "go.sum"
        if (-not (Test-Path -LiteralPath $goSumPath -PathType Leaf)) {
            throw "GOLC_BOOTSTRAP_OFFLINE_ARTIFACT_MISSING: go.sum"
        }
        $modBefore = (Get-FileHash -LiteralPath $goModPath -Algorithm SHA256).Hash
        $sumBefore = (Get-FileHash -LiteralPath $goSumPath -Algorithm SHA256).Hash

        Push-Location $RepoRoot
        try {
            Invoke-ProjectGo -GoExecutable $goExecutable `
                -Arguments @("mod", "download", "all") `
                -FailureDiagnostic "GOLC_BOOTSTRAP_MODULE_DOWNLOAD"
            Invoke-ProjectGo -GoExecutable $goExecutable `
                -Arguments @("mod", "verify") `
                -FailureDiagnostic "GOLC_BOOTSTRAP_MODULE_VERIFY"

            $moduleGraph = & $goExecutable list -m all
            if ($LASTEXITCODE -ne 0) {
                throw "GOLC_BOOTSTRAP_MODULE_GRAPH: go list -m all exited $LASTEXITCODE"
            }
            foreach ($requiredModule in @(
                    "github.com/BurntSushi/toml v1.6.0",
                    "github.com/invopop/jsonschema v0.14.0"
                )) {
                if (@($moduleGraph) -cnotcontains $requiredModule) {
                    throw "GOLC_BOOTSTRAP_MODULE_PIN_MISSING: $requiredModule"
                }
            }
            New-Item -ItemType Directory -Path $RecordDirectory -Force | Out-Null
            $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
            [System.IO.File]::WriteAllLines((Join-Path $RecordDirectory "go-modules.txt"), [string[]]@($moduleGraph), $utf8NoBom)

            # Compile and run the bootstrap probe: strict TOML decode plus
            # Invopop JSON Schema reflection against the warmed cache.
            Invoke-ProjectGo -GoExecutable $goExecutable `
                -Arguments @("test", "-count=1", "./internal/bootstrap/") `
                -FailureDiagnostic "GOLC_BOOTSTRAP_PROBE_FAILED"

            # Build the pinned project command that every normal subcommand
            # delegates to. -trimpath keeps the binary independent of the
            # checkout path; readonly module mode is already in force.
            $golcProjectPackage = Join-Path $RepoRoot "cmd\golc-project"
            if (Test-Path -LiteralPath $golcProjectPackage -PathType Container) {
                New-Item -ItemType Directory -Path (Split-Path -Parent $GolcProjectExecutable) -Force | Out-Null
                Invoke-ProjectGo -GoExecutable $goExecutable `
                    -Arguments @("build", "-trimpath", "-o", $GolcProjectExecutable, "./cmd/golc-project") `
                    -FailureDiagnostic "GOLC_BOOTSTRAP_PROJECT_BUILD"
                Write-Output "GOLC bootstrap: golc-project command built from source."
            }
        }
        finally {
            Pop-Location
        }

        $modAfter = (Get-FileHash -LiteralPath $goModPath -Algorithm SHA256).Hash
        $sumAfter = (Get-FileHash -LiteralPath $goSumPath -Algorithm SHA256).Hash
        if ($modAfter -cne $modBefore -or $sumAfter -cne $sumBefore) {
            throw "GOLC_BOOTSTRAP_LOCK_MUTATION: bootstrap must never rewrite go.mod or go.sum"
        }
        Write-Output "GOLC bootstrap: module graph warmed, verified, and recorded; probe passed; locks unchanged."
    }

    Write-Output "GOLC bootstrap: complete."
}

$shimExitCode = 1
try {
    switch ($Command) {
        "" {
            [Console]::Error.WriteLine("GOLC_USAGE: golc.ps1 <bootstrap|check|generate|build|test|package|linear> [arguments]")
            $shimExitCode = 1
        }
        "bootstrap" {
            Invoke-GolcBootstrap
            $shimExitCode = 0
        }
        default {
            if (-not (Test-Path -LiteralPath $GolcProjectExecutable -PathType Leaf)) {
                [Console]::Error.WriteLine("GOLC_TOOL_MISSING: run 'powershell -NoProfile -File .\golc.ps1 bootstrap' first")
                $shimExitCode = 1
            }
            else {
                Set-ProjectGoEnvironment
                # Delegation passes the repository root explicitly so command
                # behavior never depends on the caller's working directory.
                $env:GOLC_PROJECT_ROOT = $RepoRoot
                & $GolcProjectExecutable $Command @CommandArguments
                $shimExitCode = $LASTEXITCODE
            }
        }
    }
}
catch {
    [Console]::Error.WriteLine("GOLC_FAILURE: " + $_.Exception.Message)
    $shimExitCode = 1
}

exit $shimExitCode
