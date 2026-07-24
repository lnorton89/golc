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
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# Command/CommandArguments are extracted from the raw $args automatic
# variable rather than a declared param() block (Plan 01-15 deviation;
# CONTEXT D-03): a declared [Parameter(ValueFromRemainingArguments = $true)]
# collection still makes PowerShell 5.1's advanced parameter binder try to
# prefix-match every "-something"/"--something" token against the full
# common-parameter set (-OutVariable, -OutBuffer, ...) before it ever
# reaches CommandArguments -- so an otherwise-legitimate route argument
# like "--out <path>" (linear preview --remote --out <path>, linear
# preview --snapshot <path> --out <path>) fails at the shim itself with
# "the parameter name 'out' is ambiguous," before golc-project.exe's own
# strict per-route argument parsing is ever reached. $args carries every
# invocation argument verbatim with no such binding attempt.
$Command = ""
$CommandArguments = @()
if ($args.Count -gt 0) {
    $Command = [string]$args[0]
    if ($args.Count -gt 1) {
        $CommandArguments = [string[]]$args[1..($args.Count - 1)]
    }
}

$RepoRoot = $PSScriptRoot
$ToolchainManifestPath = Join-Path $RepoRoot "config\toolchain.toml"
$DownloadsDirectory = Join-Path $RepoRoot ".tools\cache\downloads"
$GoModCacheDirectory = Join-Path $RepoRoot ".tools\cache\go-mod"
$GoBuildCacheDirectory = Join-Path $RepoRoot ".tools\cache\go-build"
# Repository-local GOBIN: mirrors internal/bootstrap/cache.go's
# ProjectCacheLayout.GoBin exactly, so a project-local Go tool install
# (for example the pinned Wails CLI a future phase wires in) never lands
# in a machine-global bin directory.
$GoBinDirectory = Join-Path $RepoRoot ".tools\cache\go-bin"
# Project-local npm cache: only warmed/consulted when a contributor opts
# into the isolated tools/linear-sync workspace (`bootstrap --include
# linear-sync`, Plan 01-13/CONTEXT D-01). Mirrors
# internal/bootstrap/cache.go's ProjectCacheLayout.NpmCache exactly.
$NpmCacheDirectory = Join-Path $RepoRoot ".tools\cache\npm"
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
        if ($line -match '^\[(?<tool>toolchain\.(go|node))\.platforms\."windows-amd64"\]$') {
            $current = $Matches["tool"] + ".platforms.windows-amd64"
            if (-not $sections.ContainsKey($current)) {
                $sections[$current] = @{}
            }
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
    <# Repository-local Go paths (mirrors internal/bootstrap/cache.go's
       ProjectCacheLayout.Environment()). GOTOOLCHAIN=local forbids any
       silent toolchain download or host fallback once bootstrap begins;
       GOBIN keeps any Go-installed tool binary project-local instead of a
       machine-global bin directory. #>
    [CmdletBinding()]
    param()

    $env:GOTOOLCHAIN = "local"
    $env:GOMODCACHE = $GoModCacheDirectory
    $env:GOCACHE = $GoBuildCacheDirectory
    $env:GOBIN = $GoBinDirectory
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

function Test-LinearSyncNpmInstallMatches {
    <# Mirrors Test-InstalledManifestMatches's archive-pin skip contract
       (D-02, Plan 01-25): a recorded npm-ci manifest inside node_modules
       matching the exact current package.json/package-lock.json bytes,
       plus the pinned TypeScript compiler and every compiled dist output
       already present, means the exact-lock `npm ci` (and its tsc
       recompile) is never invoked again -- so a second bootstrap of an
       unchanged lock performs zero npm/network calls, not merely a fast
       one. The manifest lives inside node_modules so deleting node_modules
       naturally invalidates it. #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$LinearSyncDir,

        [Parameter(Mandatory = $true)]
        [string]$PackageJsonSha256,

        [Parameter(Mandatory = $true)]
        [string]$PackageLockSha256
    )

    $manifestPath = Join-Path $LinearSyncDir "node_modules\.golc-npm-ci-manifest.json"
    if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
        return $false
    }
    try {
        $manifest = [System.IO.File]::ReadAllText($manifestPath) | ConvertFrom-Json
    }
    catch {
        return $false
    }
    $packageJsonProperty = $manifest.PSObject.Properties["package_json_sha256"]
    $packageLockProperty = $manifest.PSObject.Properties["package_lock_sha256"]
    if ($null -eq $packageJsonProperty -or $null -eq $packageLockProperty) {
        return $false
    }
    if ($packageJsonProperty.Value -cne $PackageJsonSha256 -or $packageLockProperty.Value -cne $PackageLockSha256) {
        return $false
    }
    $tscPath = Join-Path $LinearSyncDir "node_modules\typescript\bin\tsc"
    if (-not (Test-Path -LiteralPath $tscPath -PathType Leaf)) {
        return $false
    }
    foreach ($distFile in @(
            (Join-Path $LinearSyncDir "dist\src\protocol.js"),
            (Join-Path $LinearSyncDir "dist\src\adapter.js"),
            (Join-Path $LinearSyncDir "dist\src\cli.js"),
            (Join-Path $LinearSyncDir "dist\test\operations.test.js")
        )) {
        if (-not (Test-Path -LiteralPath $distFile -PathType Leaf)) {
            return $false
        }
    }
    return $true
}

function Invoke-GolcBootstrapLinearSync {
    <# Provisions the isolated tools/linear-sync workspace (CONTEXT D-01/
       D-03; Plan 01-13/01-25): installs the pinned project-local Node
       archive exactly the way Install-ArchivePin already provisions Go,
       then runs an exact-lock `npm ci` with lifecycle scripts disabled
       against the already-committed package.json/package-lock.json, and
       finally compiles src/**/*.ts and test/**/*.ts (protocol, adapter,
       cli, and the fake-SDK hierarchy test) with the pinned project-local
       TypeScript compiler. It never runs `npm install`, never touches
       package.json or package-lock.json, and hard-fails
       (GOLC_BOOTSTRAP_NODE_LOCK_MUTATION) if either changes underneath it
       -- the same before/after hash discipline the Go module bootstrap
       block above already enforces for go.mod/go.sum (D-04). A recorded
       npm-ci manifest matching the current lock bytes (Plan 01-25,
       Test-LinearSyncNpmInstallMatches) makes a second bootstrap of an
       unchanged lock skip npm ci and tsc entirely -- zero network calls,
       not merely a fast no-op. #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [hashtable]$Manifest
    )

    if (-not $Manifest.ContainsKey("toolchain.node")) {
        throw "GOLC_NODE_TOOLCHAIN_MISSING: config/toolchain.toml must pin [toolchain.node] before 'bootstrap --include linear-sync'"
    }
    $nodeSection = $Manifest["toolchain.node"]
    $nodeVersion = Get-RequiredPinValue -Section $nodeSection -SectionName "toolchain.node" -Key "version"
    $nodePlatformName = "toolchain.node.platforms.windows-amd64"
    if (-not $Manifest.ContainsKey($nodePlatformName)) {
        throw 'GOLC_NODE_TOOLCHAIN_PLATFORM_MISSING: config/toolchain.toml must pin [toolchain.node.platforms."windows-amd64"]'
    }
    $nodePlatformSection = $Manifest[$nodePlatformName]
    $nodeInstallDir = Join-Path $RepoRoot (".tools\toolchains\node\" + $nodeVersion + "\windows-amd64")
    Install-ArchivePin `
        -DisplayName ("node " + $nodeVersion) `
        -Uri (Get-RequiredPinValue -Section $nodePlatformSection -SectionName $nodePlatformName -Key "archive_url") `
        -Sha256 (Get-RequiredPinValue -Section $nodePlatformSection -SectionName $nodePlatformName -Key "archive_sha256") `
        -InstallDir $nodeInstallDir

    $nodeExtractedDir = Join-Path $nodeInstallDir ("node-v" + $nodeVersion + "-win-x64")
    $nodeExecutable = Join-Path $nodeExtractedDir "node.exe"
    $npmCliPath = Join-Path $nodeExtractedDir "node_modules\npm\bin\npm-cli.js"
    if (-not (Test-Path -LiteralPath $nodeExecutable -PathType Leaf)) {
        throw "GOLC_NODE_TOOLCHAIN_MISSING: expected node executable at $nodeExecutable after provisioning"
    }
    if (-not (Test-Path -LiteralPath $npmCliPath -PathType Leaf)) {
        throw "GOLC_NODE_TOOLCHAIN_MISSING: expected npm-cli.js at $npmCliPath after provisioning"
    }

    $linearSyncDir = Join-Path $RepoRoot "tools\linear-sync"
    $packageJsonPath = Join-Path $linearSyncDir "package.json"
    $packageLockPath = Join-Path $linearSyncDir "package-lock.json"
    if (-not (Test-Path -LiteralPath $packageJsonPath -PathType Leaf)) {
        throw "GOLC_BOOTSTRAP_OFFLINE_ARTIFACT_MISSING: tools/linear-sync/package.json"
    }
    if (-not (Test-Path -LiteralPath $packageLockPath -PathType Leaf)) {
        throw "GOLC_BOOTSTRAP_OFFLINE_ARTIFACT_MISSING: tools/linear-sync/package-lock.json"
    }

    $packageJsonBefore = (Get-FileHash -LiteralPath $packageJsonPath -Algorithm SHA256).Hash
    $packageLockBefore = (Get-FileHash -LiteralPath $packageLockPath -Algorithm SHA256).Hash

    if (Test-LinearSyncNpmInstallMatches -LinearSyncDir $linearSyncDir -PackageJsonSha256 $packageJsonBefore -PackageLockSha256 $packageLockBefore) {
        Write-Output "GOLC bootstrap: tools/linear-sync already matches package-lock.json $packageLockBefore; npm ci not invoked."
        return
    }

    New-Item -ItemType Directory -Path $NpmCacheDirectory -Force | Out-Null

    Push-Location $linearSyncDir
    try {
        $env:NPM_CONFIG_CACHE = $NpmCacheDirectory
        # Exact-lock install: --ignore-scripts disables every lifecycle
        # script (T-01-36/T-01-SC), and `ci` refuses to resolve anything
        # outside the committed, human-approved package-lock.json.
        & $nodeExecutable $npmCliPath "ci" "--ignore-scripts" "--no-audit" "--no-fund"
        if ($LASTEXITCODE -ne 0) {
            throw "GOLC_BOOTSTRAP_NPM_CI_FAILED: npm ci exited $LASTEXITCODE"
        }
        Write-Output "GOLC bootstrap: tools/linear-sync exact-lock npm ci complete (lifecycle scripts disabled)."

        $tscPath = Join-Path $linearSyncDir "node_modules\typescript\bin\tsc"
        if (-not (Test-Path -LiteralPath $tscPath -PathType Leaf)) {
            throw "GOLC_BOOTSTRAP_LINEAR_SYNC_BUILD_FAILED: pinned TypeScript compiler missing at $tscPath after npm ci"
        }
        & $nodeExecutable $tscPath "-p" (Join-Path $linearSyncDir "tsconfig.json")
        if ($LASTEXITCODE -ne 0) {
            throw "GOLC_BOOTSTRAP_LINEAR_SYNC_BUILD_FAILED: tsc exited $LASTEXITCODE"
        }
    }
    finally {
        Remove-Item Env:\NPM_CONFIG_CACHE -ErrorAction SilentlyContinue
        Pop-Location
    }

    foreach ($expectedDistFile in @(
            (Join-Path $linearSyncDir "dist\src\protocol.js"),
            (Join-Path $linearSyncDir "dist\src\adapter.js"),
            (Join-Path $linearSyncDir "dist\src\cli.js"),
            (Join-Path $linearSyncDir "dist\test\operations.test.js")
        )) {
        if (-not (Test-Path -LiteralPath $expectedDistFile -PathType Leaf)) {
            throw "GOLC_BOOTSTRAP_LINEAR_SYNC_BUILD_FAILED: expected compiled $expectedDistFile"
        }
    }

    $packageJsonAfter = (Get-FileHash -LiteralPath $packageJsonPath -Algorithm SHA256).Hash
    $packageLockAfter = (Get-FileHash -LiteralPath $packageLockPath -Algorithm SHA256).Hash
    if ($packageJsonAfter -cne $packageJsonBefore -or $packageLockAfter -cne $packageLockBefore) {
        throw "GOLC_BOOTSTRAP_NODE_LOCK_MUTATION: bootstrap must never rewrite tools/linear-sync/package.json or package-lock.json"
    }

    # Recording this manifest inside node_modules (rather than under
    # .tools/manifest) means deleting node_modules naturally invalidates
    # it -- a stale manifest can never survive without its install.
    $npmCiManifestJson = [ordered]@{
        package_json_sha256 = $packageJsonBefore
        package_lock_sha256 = $packageLockBefore
    } | ConvertTo-Json
    $utf8NoBomManifest = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText((Join-Path $linearSyncDir "node_modules\.golc-npm-ci-manifest.json"), $npmCiManifestJson + "`n", $utf8NoBomManifest)

    Write-Output "GOLC bootstrap: tools/linear-sync compiled (src + test); pins/lock unchanged."
}

function Invoke-GolcBootstrap {
    [CmdletBinding()]
    param(
        [switch]$IncludeLinearSync
    )

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
        if ($cacheSection.ContainsKey("gobin")) {
            $script:GoBinDirectory = Resolve-CacheDirectory -RelativePath $cacheSection["gobin"]
        }
    }

    # Warm the complete project-local cache layout up front (mirrors
    # internal/bootstrap/cache.go's ProjectCacheLayout.Warm): every
    # directory is created if missing and never touched if it already
    # exists, so this step alone is always a safe idempotent no-op.
    foreach ($cacheDirectory in @($DownloadsDirectory, $GoModCacheDirectory, $GoBuildCacheDirectory, $GoBinDirectory, $NpmCacheDirectory, $RecordDirectory)) {
        New-Item -ItemType Directory -Path $cacheDirectory -Force | Out-Null
    }
    Write-Output "GOLC bootstrap: project-local cache layout warmed (GOBIN=$GoBinDirectory, GOMODCACHE=$GoModCacheDirectory, GOCACHE=$GoBuildCacheDirectory)."

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
            -Uri (Get-RequiredPinValue -Section $section -SectionName $sectionName -Key "archive_url") `
            -Sha256 (Get-RequiredPinValue -Section $section -SectionName $sectionName -Key "archive_sha256") `
            -InstallDir (Join-Path $RepoRoot (".tools\installs\" + $toolName))
    }

    # Pinned Go toolchain.
    $goExecutable = $null
    if ($manifest.ContainsKey("toolchain.go")) {
        $goSection = $manifest["toolchain.go"]
        $goVersion = Get-RequiredPinValue -Section $goSection -SectionName "toolchain.go" -Key "version"
        $goPlatformName = "toolchain.go.platforms.windows-amd64"
        if (-not $manifest.ContainsKey($goPlatformName)) {
            throw 'GOLC_GO_TOOLCHAIN_PLATFORM_MISSING: config/toolchain.toml must pin [toolchain.go.platforms."windows-amd64"]'
        }
        $goPlatformSection = $manifest[$goPlatformName]
        $goInstallDir = Join-Path $RepoRoot (".tools\toolchains\go\" + $goVersion + "\windows-amd64")
        Install-ArchivePin `
            -DisplayName ("go " + $goVersion) `
            -Uri (Get-RequiredPinValue -Section $goPlatformSection -SectionName $goPlatformName -Key "archive_url") `
            -Sha256 (Get-RequiredPinValue -Section $goPlatformSection -SectionName $goPlatformName -Key "archive_sha256") `
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

    if ($IncludeLinearSync) {
        Invoke-GolcBootstrapLinearSync -Manifest $manifest
    }

    Write-Output "GOLC bootstrap: complete."
}

$shimExitCode = 1
try {
    switch ($Command) {
        "" {
            [Console]::Error.WriteLine("GOLC_USAGE: golc.ps1 <bootstrap|config|check|generate|build|test|package|linear> [arguments]")
            $shimExitCode = 1
        }
        "bootstrap" {
            # Only "--include linear-sync" is a supported bootstrap
            # argument today (Plan 01-13): it opts a contributor into
            # provisioning the isolated tools/linear-sync Node workspace.
            # Anything else fails closed with a stable usage diagnostic
            # rather than being silently ignored.
            $includeLinearSync = $false
            $i = 0
            while ($i -lt $CommandArguments.Count) {
                $argument = $CommandArguments[$i]
                if ($argument -eq "--include") {
                    if ($i + 1 -ge $CommandArguments.Count) {
                        throw "GOLC_BOOTSTRAP_USAGE: --include requires a value; usage: bootstrap [--include linear-sync]"
                    }
                    $includeTarget = $CommandArguments[$i + 1]
                    if ($includeTarget -ne "linear-sync") {
                        throw "GOLC_BOOTSTRAP_USAGE: unsupported --include target '$includeTarget'; only 'linear-sync' is supported"
                    }
                    $includeLinearSync = $true
                    $i += 2
                }
                else {
                    throw "GOLC_BOOTSTRAP_USAGE: unsupported argument '$argument'; usage: bootstrap [--include linear-sync]"
                }
            }
            Invoke-GolcBootstrap -IncludeLinearSync:$includeLinearSync
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
