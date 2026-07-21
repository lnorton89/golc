<#
.SYNOPSIS
Plan 01-28 acceptance: project-local cache warming and bootstrap
idempotence (D-01, D-02, T-01-13, T-01-SC).

.DESCRIPTION
Two independent stages, both run from a single invocation with no
parameters (matching the automated command in 01-VALIDATION.md):

Stage 1 (offline, temporary fixture repository) proves the exact
corrupt/retry/idempotent contract for golc.ps1's generic checksum-pinned
tool-archive install (`[tools.<name>]` + Install-ArchivePin), using only a
locally built zip archive referenced through a file:// URI — no network
call is ever made:
  - a corrupt pin (wrong SHA-256) makes the first bootstrap fail and
    leaves no install directory;
  - correcting the pin makes a retry succeed and warms the downloads
    cache plus the install manifest;
  - deleting the archive source entirely and rerunning bootstrap still
    succeeds with zero archive-source calls (InstalledMatches skip) and
    leaves the promoted install byte-identical.

Stage 2 (the repository under test, in place) proves the project-local
Go cache-warming contract golc.ps1/internal/bootstrap/cache.go establish:
GOBIN/GOMODCACHE/GOCACHE/downloads/manifest directories are created,
go.mod/go.sum are never mutated, and an immediately repeated bootstrap
performs zero new archive/module transport (module-cache and GOBIN
inventories are byte-for-byte unchanged between the two runs).
#>
[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Invoke-Golc {
    <# Runs golc.ps1 in RepositoryRoot with CommandArguments and returns a
       classification/exit-code/output object. Mirrors
       tests/acceptance/walking-skeleton.ps1's Invoke-Golc exactly so both
       acceptance scripts observe process boundaries identically. #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$RepositoryRoot,

        [string[]]$CommandArguments = @()
    )

    $rootCommand = Join-Path $RepositoryRoot "golc.ps1"
    if (-not (Test-Path -LiteralPath $rootCommand -PathType Leaf)) {
        return [pscustomobject]@{
            Classification = "missing-root-command"
            ExitCode       = $null
            StdOutBytes    = [byte[]]@()
            StdErrBytes    = [byte[]]@()
            StdOutText     = ""
            StdErrText     = "golc.ps1 is absent"
        }
    }

    $windowsPowerShell = Get-Command "powershell.exe" -CommandType Application -ErrorAction Stop
    $captureRoot = Join-Path $RepositoryRoot (".acceptance-capture-" + [guid]::NewGuid().ToString("N"))
    $stdoutPath = Join-Path $captureRoot "stdout.bin"
    $stderrPath = Join-Path $captureRoot "stderr.bin"

    New-Item -ItemType Directory -Path $captureRoot -ErrorAction Stop | Out-Null
    try {
        $processArguments = @(
            "-NoProfile",
            "-NonInteractive",
            "-ExecutionPolicy",
            "Bypass",
            "-File",
            ('"' + $rootCommand + '"')
        ) + $CommandArguments

        $process = Start-Process `
            -FilePath $windowsPowerShell.Source `
            -ArgumentList $processArguments `
            -WorkingDirectory $RepositoryRoot `
            -RedirectStandardOutput $stdoutPath `
            -RedirectStandardError $stderrPath `
            -Wait `
            -PassThru `
            -NoNewWindow

        $stdoutBytes = [System.IO.File]::ReadAllBytes($stdoutPath)
        $stderrBytes = [System.IO.File]::ReadAllBytes($stderrPath)
        $classification = "completed"
        if ($process.ExitCode -ne 0) {
            $classification = "root-command-failed"
        }

        return [pscustomobject]@{
            Classification = $classification
            ExitCode       = $process.ExitCode
            StdOutBytes    = $stdoutBytes
            StdErrBytes    = $stderrBytes
            StdOutText     = [System.Text.Encoding]::UTF8.GetString($stdoutBytes)
            StdErrText     = [System.Text.Encoding]::UTF8.GetString($stderrBytes)
        }
    }
    finally {
        if (Test-Path -LiteralPath $captureRoot) {
            Remove-Item -LiteralPath $captureRoot -Recurse -Force -ErrorAction Stop
        }
    }
}

function Assert-GolcSucceeded {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [psobject]$Result,

        [Parameter(Mandatory = $true)]
        [string]$Operation
    )

    if ($Result.Classification -eq "missing-root-command") {
        throw "ROOT_COMMAND_MISSING: $Operation requires golc.ps1"
    }
    if ($Result.ExitCode -ne 0) {
        throw "ROOT_COMMAND_FAILED: $Operation exited $($Result.ExitCode): $($Result.StdErrText.Trim())"
    }
}

function Assert-GolcFailed {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [psobject]$Result,

        [Parameter(Mandatory = $true)]
        [string]$Operation,

        [Parameter(Mandatory = $true)]
        [string]$ExpectedDiagnostic
    )

    if ($Result.ExitCode -eq 0) {
        throw "EXPECTED_FAILURE: $Operation unexpectedly succeeded"
    }
    if (-not $Result.StdErrText.Contains($ExpectedDiagnostic)) {
        throw "WRONG_FAILURE: $Operation failed but stderr did not contain '$ExpectedDiagnostic': $($Result.StdErrText.Trim())"
    }
}

function New-FixtureToolArchive {
    <# Builds a small zip archive with one file entry and returns its path
       plus lowercase hex SHA-256, entirely offline. #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$Directory
    )

    New-Item -ItemType Directory -Path $Directory -Force | Out-Null
    $payloadDir = Join-Path $Directory "payload\bin"
    New-Item -ItemType Directory -Path $payloadDir -Force | Out-Null
    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText((Join-Path $payloadDir "fixture-tool.exe"), "fixture tool payload`n", $utf8NoBom)

    $archivePath = Join-Path $Directory "fixture-tool.zip"
    Compress-Archive -Path (Join-Path $Directory "payload\*") -DestinationPath $archivePath -Force

    $sha256 = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    return [pscustomobject]@{
        ArchivePath = $archivePath
        Sha256      = $sha256
    }
}

function Write-FixtureToolchainManifest {
    <# Writes a minimal config/toolchain.toml declaring exactly one
       [tools.fixture] archive pin — no [toolchain.go] section and no
       go.mod present, so Invoke-GolcBootstrap exercises only the generic
       tool-archive-pin path this stage is testing. #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$RepositoryRoot,

        [Parameter(Mandatory = $true)]
        [string]$ArchiveUri,

        [Parameter(Mandatory = $true)]
        [string]$Sha256
    )

    $configDir = Join-Path $RepositoryRoot "config"
    New-Item -ItemType Directory -Path $configDir -Force | Out-Null
    $body = "schema_version = 1`n`n[tools.fixture]`narchive_uri = `"$ArchiveUri`"`narchive_sha256 = `"$Sha256`"`n"
    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText((Join-Path $configDir "toolchain.toml"), $body, $utf8NoBom)
}

function Get-DirectoryInventory {
    <# Stable inventory of every file under Root: relative path plus byte
       length. Any download, install, or mutation changes it. #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$Root
    )

    if (-not (Test-Path -LiteralPath $Root -PathType Container)) {
        # The unary comma forces PowerShell to emit one empty-array object
        # instead of unrolling a zero-element array into zero pipeline
        # objects (which a caller would observe as $null, not @()).
        return , @()
    }
    $resolvedRoot = (Get-Item -LiteralPath $Root).FullName
    $entries = @(Get-ChildItem -LiteralPath $resolvedRoot -Recurse -Force -File -ErrorAction SilentlyContinue | ForEach-Object {
            $_.FullName.Substring($resolvedRoot.Length).TrimStart("\", "/").Replace("\", "/") + "|" + $_.Length
        } | Sort-Object)
    return , $entries
}

function Assert-SameInventory {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$Label,

        [Parameter(Mandatory = $true)]
        [AllowEmptyCollection()]
        [string[]]$Before,

        [Parameter(Mandatory = $true)]
        [AllowEmptyCollection()]
        [string[]]$After
    )

    # Compare-Object returns $null (not an empty array) when both sides are
    # empty, so wrap defensively before checking Count.
    $differences = @(Compare-Object -ReferenceObject $Before -DifferenceObject $After)
    if ($differences.Count -ne 0) {
        throw "BOOTSTRAP_CACHE_INVENTORY_CHANGED: $Label changed between bootstrap runs: $($differences | ConvertTo-Json -Compress)"
    }
}

function Invoke-Stage1FixtureArchiveContract {
    <# Corrupt rejection, corrected retry, and idempotent zero-call rerun
       for the generic checksum-pinned tool-archive install, entirely
       offline via a file:// archive source. #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$RepositoryUnderTest
    )

    $temporaryRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("golc-bootstrap-acceptance-" + [guid]::NewGuid().ToString("N"))
    $workingRepository = Join-Path $temporaryRoot "repository"
    $fixtureRoot = Join-Path $temporaryRoot "fixture"
    try {
        New-Item -ItemType Directory -Path $workingRepository -Force | Out-Null
        Copy-Item -LiteralPath (Join-Path $RepositoryUnderTest "golc.ps1") -Destination (Join-Path $workingRepository "golc.ps1") -Force

        $fixture = New-FixtureToolArchive -Directory $fixtureRoot
        $archiveUri = ([System.Uri]::new($fixture.ArchivePath)).AbsoluteUri
        $wrongSha256 = ("0" * 64)

        # 1. A corrupt pin (wrong SHA-256) must fail closed and leave no
        # install directory.
        Write-FixtureToolchainManifest -RepositoryRoot $workingRepository -ArchiveUri $archiveUri -Sha256 $wrongSha256
        $corruptResult = Invoke-Golc -RepositoryRoot $workingRepository -CommandArguments @("bootstrap")
        Assert-GolcFailed -Result $corruptResult -Operation "corrupt tool-archive bootstrap" -ExpectedDiagnostic "GOLC_BOOTSTRAP_CHECKSUM_MISMATCH"
        $installDir = Join-Path $workingRepository ".tools\installs\fixture"
        if (Test-Path -LiteralPath $installDir) {
            throw "BOOTSTRAP_CACHE_CORRUPT_INSTALLED: a corrupt pin must never promote an install"
        }
        Write-Output "Stage 1 corrupt-rejection confirmed: wrong pin failed closed with no install."

        # 2. Correcting the pin must make an immediate retry succeed and
        # warm the downloads cache plus the promoted install.
        Write-FixtureToolchainManifest -RepositoryRoot $workingRepository -ArchiveUri $archiveUri -Sha256 $fixture.Sha256
        $retryResult = Invoke-Golc -RepositoryRoot $workingRepository -CommandArguments @("bootstrap")
        Assert-GolcSucceeded -Result $retryResult -Operation "corrected tool-archive bootstrap retry"
        $installedPayload = Join-Path $installDir "bin\fixture-tool.exe"
        if (-not (Test-Path -LiteralPath $installedPayload -PathType Leaf)) {
            throw "BOOTSTRAP_CACHE_RETRY_MISSING_PAYLOAD: expected $installedPayload after the corrected retry"
        }
        $downloadsCache = Join-Path $workingRepository ".tools\cache\downloads"
        if (-not (Test-Path -LiteralPath $downloadsCache -PathType Container) -or (@(Get-ChildItem -LiteralPath $downloadsCache -File)).Count -eq 0) {
            throw "BOOTSTRAP_CACHE_NOT_WARMED: expected a cached archive under $downloadsCache after the corrected retry"
        }
        Write-Output "Stage 1 correct-retry confirmed: cache warmed and install promoted."

        # 3. Deleting the archive source entirely and rerunning bootstrap
        # must still succeed with zero archive-source calls, and the
        # promoted install must be byte-identical (InstalledMatches skip).
        $beforeInstallHash = (Get-FileHash -LiteralPath $installedPayload -Algorithm SHA256).Hash
        Remove-Item -LiteralPath $fixture.ArchivePath -Force
        Remove-Item -LiteralPath $downloadsCache -Recurse -Force
        $idempotentResult = Invoke-Golc -RepositoryRoot $workingRepository -CommandArguments @("bootstrap")
        Assert-GolcSucceeded -Result $idempotentResult -Operation "idempotent tool-archive bootstrap rerun without an archive source"
        if (-not $idempotentResult.StdOutText.Contains("archive source not consulted")) {
            throw "BOOTSTRAP_CACHE_UNEXPECTED_TRANSPORT: expected the rerun to report the archive source was not consulted"
        }
        $afterInstallHash = (Get-FileHash -LiteralPath $installedPayload -Algorithm SHA256).Hash
        if ($afterInstallHash -cne $beforeInstallHash) {
            throw "BOOTSTRAP_CACHE_INSTALL_CHANGED: the idempotent rerun must not alter the promoted install"
        }
        Write-Output "Stage 1 idempotent-rerun confirmed: zero archive-source calls, install unchanged."
    }
    finally {
        if (Test-Path -LiteralPath $temporaryRoot) {
            Remove-Item -LiteralPath $temporaryRoot -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

function Invoke-Stage2ProjectCacheWarmContract {
    <# Proves the project-local Go cache-warming contract on the repository
       under test: GOBIN/GOMODCACHE/GOCACHE/downloads/manifest directories
       exist, go.mod/go.sum are never mutated, and an immediately repeated
       bootstrap performs zero new archive/module transport. #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$RepositoryUnderTest
    )

    $goModPath = Join-Path $RepositoryUnderTest "go.mod"
    $goSumPath = Join-Path $RepositoryUnderTest "go.sum"
    $modBefore = (Get-FileHash -LiteralPath $goModPath -Algorithm SHA256).Hash
    $sumBefore = (Get-FileHash -LiteralPath $goSumPath -Algorithm SHA256).Hash

    $firstResult = Invoke-Golc -RepositoryRoot $RepositoryUnderTest -CommandArguments @("bootstrap")
    Assert-GolcSucceeded -Result $firstResult -Operation "repository cache-warm bootstrap"
    if (-not $firstResult.StdOutText.Contains("project-local cache layout warmed")) {
        throw "BOOTSTRAP_CACHE_NOT_ANNOUNCED: expected bootstrap to report the warmed cache layout"
    }

    $goBinDirectory = Join-Path $RepositoryUnderTest ".tools\cache\go-bin"
    $goModCacheDirectory = Join-Path $RepositoryUnderTest ".tools\cache\go-mod"
    $goBuildCacheDirectory = Join-Path $RepositoryUnderTest ".tools\cache\go-build"
    $downloadsDirectory = Join-Path $RepositoryUnderTest ".tools\cache\downloads"
    $manifestDirectory = Join-Path $RepositoryUnderTest ".tools\manifest"
    foreach ($required in @($goBinDirectory, $goModCacheDirectory, $goBuildCacheDirectory, $downloadsDirectory, $manifestDirectory)) {
        if (-not (Test-Path -LiteralPath $required -PathType Container)) {
            throw "BOOTSTRAP_CACHE_DIRECTORY_MISSING: expected $required after bootstrap"
        }
    }

    $modAfterFirst = (Get-FileHash -LiteralPath $goModPath -Algorithm SHA256).Hash
    $sumAfterFirst = (Get-FileHash -LiteralPath $goSumPath -Algorithm SHA256).Hash
    if ($modAfterFirst -cne $modBefore -or $sumAfterFirst -cne $sumBefore) {
        throw "BOOTSTRAP_CACHE_LOCK_MUTATION: bootstrap must never rewrite go.mod or go.sum"
    }
    Write-Output "Stage 2 cache-warm confirmed: GOBIN/GOMODCACHE/GOCACHE/downloads/manifest exist; locks unchanged."

    $moduleCacheInventoryBefore = Get-DirectoryInventory -Root $goModCacheDirectory
    $goBinInventoryBefore = Get-DirectoryInventory -Root $goBinDirectory
    $downloadsInventoryBefore = Get-DirectoryInventory -Root $downloadsDirectory

    $secondResult = Invoke-Golc -RepositoryRoot $RepositoryUnderTest -CommandArguments @("bootstrap")
    Assert-GolcSucceeded -Result $secondResult -Operation "repository idempotent bootstrap rerun"
    if ($secondResult.StdOutText.Contains("installed from checksum-verified archive")) {
        throw "BOOTSTRAP_CACHE_UNEXPECTED_INSTALL: the idempotent rerun must not perform a fresh archive install"
    }
    if (-not $secondResult.StdOutText.Contains("already matches pin")) {
        throw "BOOTSTRAP_CACHE_MANIFEST_NOT_VALIDATED: expected the rerun to report an already-matching pin"
    }

    $modAfterSecond = (Get-FileHash -LiteralPath $goModPath -Algorithm SHA256).Hash
    $sumAfterSecond = (Get-FileHash -LiteralPath $goSumPath -Algorithm SHA256).Hash
    if ($modAfterSecond -cne $modBefore -or $sumAfterSecond -cne $sumBefore) {
        throw "BOOTSTRAP_CACHE_LOCK_MUTATION: the idempotent rerun must never rewrite go.mod or go.sum"
    }

    Assert-SameInventory -Label "go module cache" -Before $moduleCacheInventoryBefore -After (Get-DirectoryInventory -Root $goModCacheDirectory)
    Assert-SameInventory -Label "GOBIN" -Before $goBinInventoryBefore -After (Get-DirectoryInventory -Root $goBinDirectory)
    Assert-SameInventory -Label "downloads cache" -Before $downloadsInventoryBefore -After (Get-DirectoryInventory -Root $downloadsDirectory)
    Write-Output "Stage 2 idempotent-rerun confirmed: zero new transport, module cache/GOBIN/downloads byte-identical, locks unchanged."
}

$scriptExitCode = 1
try {
    $repositoryUnderTest = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot "..\.."))

    Invoke-Stage1FixtureArchiveContract -RepositoryUnderTest $repositoryUnderTest
    Invoke-Stage2ProjectCacheWarmContract -RepositoryUnderTest $repositoryUnderTest

    Write-Output "Bootstrap cache-warm contract confirmed: corrupt rejection, correct retry, cache warm, and idempotent rerun."
    $scriptExitCode = 0
}
catch {
    [Console]::Error.WriteLine("BOOTSTRAP_ACCEPTANCE_FAILURE: " + $_.Exception.Message)
    $scriptExitCode = 1
}

exit $scriptExitCode
