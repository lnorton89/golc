<#
.SYNOPSIS
Plan 01-06/01-20 acceptance: exact registered core routes (generate,
check, build, test) run through one offline-enforced graph (D-02/D-03/
D-10, T-01-15, T-01-17), and the deterministic foundation package
(T-01-16) reproduces byte-identical output.

.DESCRIPTION
-Mode core runs entirely against the repository under test, through
golc.ps1 only (never a raw go/npm command), so this acceptance proves the
exact contributor-facing routes, not just the underlying Go API:

  1. `golc.ps1 build` succeeds: the "build" route is reachable and
     compiles every project package with the pinned toolchain and
     repository-local caches.
  2. `golc.ps1 test --quick` succeeds: the bare-quick graph orchestration
     form (internal/delivery's "test" Step) passes.
  3. `golc.ps1 check --offline` succeeds and reports every core step —
     generate, check, build, and test — completed with network denied:
     the one declarative offline core graph internal/delivery/graph.go
     owns (LoadGraph/RunOffline), executed through internal/command's
     self-registered routes.
  4. `golc.ps1 generate --check` immediately afterward still reports zero
     drift, proving check --offline's own generate step left the
     committed schemas byte-identical.

-Mode package proves Plan 01-20's deterministic foundation package
(T-01-16): the exact registered "package --foundation" route is reachable,
and running it twice produces byte-identical ZIP, manifest, and checksum
output:

  1. `golc.ps1 package --foundation` succeeds once, and the resulting ZIP,
     manifest, and .sha256 checksum sidecar at dist/foundation/ are copied
     aside.
  2. `golc.ps1 package --foundation` runs a second time against the same
     repository state.
  3. The second run's ZIP, manifest, and checksum sidecar bytes are
     compared byte-for-byte against the first run's copies — any
     divergence (a machine timestamp, path, or non-deterministic ordering
     leaking into the archive) fails this acceptance.

Requires a prior successful `golc.ps1 bootstrap` (the pinned project-local
Go toolchain and warmed caches must already exist) — this script never
bootstraps or installs anything itself, matching
tests/acceptance/command-parity.ps1's PR-CI precedent of assuming a
bootstrapped checkout.
#>
[CmdletBinding()]
param(
    [ValidateSet("core", "package")]
    [string]$Mode = "core"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Invoke-Golc {
    <# Runs golc.ps1 in RepositoryRoot with CommandArguments and returns a
       classification/exit-code/output object. Mirrors
       tests/acceptance/bootstrap.ps1's Invoke-Golc exactly so every
       acceptance script observes process boundaries identically. #>
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

function Invoke-CoreOfflineAcceptance {
    <# Exercises every registered core route (build, test, check --offline,
       generate --check) directly through golc.ps1 against the repository
       under test, proving the one declarative offline graph is reachable
       and network-denied end to end. #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$RepositoryUnderTest
    )

    if (-not (Test-Path -LiteralPath (Join-Path $RepositoryUnderTest "golc.ps1") -PathType Leaf)) {
        throw "ROOT_COMMAND_MISSING: offline core acceptance requires golc.ps1"
    }

    $buildResult = Invoke-Golc -RepositoryRoot $RepositoryUnderTest -CommandArguments @("build")
    Assert-GolcSucceeded -Result $buildResult -Operation "build"
    Write-Output "Offline core acceptance: build route compiled every project package."

    $quickResult = Invoke-Golc -RepositoryRoot $RepositoryUnderTest -CommandArguments @("test", "--quick")
    Assert-GolcSucceeded -Result $quickResult -Operation "test --quick"
    Write-Output "Offline core acceptance: test --quick route passed (go vet)."

    $offlineResult = Invoke-Golc -RepositoryRoot $RepositoryUnderTest -CommandArguments @("check", "--offline")
    Assert-GolcSucceeded -Result $offlineResult -Operation "check --offline"
    if (-not $offlineResult.StdOutText.Contains("generate, check, build, and test all completed with network denied")) {
        throw "OFFLINE_GRAPH_NOT_CONFIRMED: expected check --offline to report the complete offline core graph"
    }
    Write-Output "Offline core acceptance: check --offline ran the complete generate/check/build/test graph with network denied."

    $driftResult = Invoke-Golc -RepositoryRoot $RepositoryUnderTest -CommandArguments @("generate", "--check")
    Assert-GolcSucceeded -Result $driftResult -Operation "generate --check"
    Write-Output "Offline core acceptance: committed schemas remain drift-free after the offline graph's own generate step."
}

function Invoke-FoundationPackageAcceptance {
    <# Exercises the registered "package --foundation" route twice against
       the repository under test and asserts the ZIP, manifest, and
       checksum sidecar bytes are identical both times (T-01-16). #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$RepositoryUnderTest
    )

    if (-not (Test-Path -LiteralPath (Join-Path $RepositoryUnderTest "golc.ps1") -PathType Leaf)) {
        throw "ROOT_COMMAND_MISSING: foundation package acceptance requires golc.ps1"
    }

    $distDirectory = Join-Path $RepositoryUnderTest "dist\foundation"
    $zipPath = Join-Path $distDirectory "golc-foundation-windows-amd64.zip"
    $manifestPath = Join-Path $distDirectory "golc-foundation-windows-amd64.manifest.json"
    $checksumPath = Join-Path $distDirectory "golc-foundation-windows-amd64.zip.sha256"

    $firstResult = Invoke-Golc -RepositoryRoot $RepositoryUnderTest -CommandArguments @("package", "--foundation")
    Assert-GolcSucceeded -Result $firstResult -Operation "package --foundation (first run)"
    foreach ($path in @($zipPath, $manifestPath, $checksumPath)) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
            throw "FOUNDATION_OUTPUT_MISSING: expected $path after the first package --foundation run"
        }
    }
    $firstZipBytes = [System.IO.File]::ReadAllBytes($zipPath)
    $firstManifestBytes = [System.IO.File]::ReadAllBytes($manifestPath)
    $firstChecksumBytes = [System.IO.File]::ReadAllBytes($checksumPath)
    Write-Output "Foundation package acceptance: first package --foundation run produced a ZIP, manifest, and checksum sidecar."

    $secondResult = Invoke-Golc -RepositoryRoot $RepositoryUnderTest -CommandArguments @("package", "--foundation")
    Assert-GolcSucceeded -Result $secondResult -Operation "package --foundation (second run)"
    $secondZipBytes = [System.IO.File]::ReadAllBytes($zipPath)
    $secondManifestBytes = [System.IO.File]::ReadAllBytes($manifestPath)
    $secondChecksumBytes = [System.IO.File]::ReadAllBytes($checksumPath)

    # Byte-array equality via Base64 string comparison avoids a dependency
    # on System.Linq.Enumerable being loaded in Windows PowerShell 5.1.
    if ([System.Convert]::ToBase64String($firstZipBytes) -cne [System.Convert]::ToBase64String($secondZipBytes)) {
        throw "FOUNDATION_NOT_DETERMINISTIC: repeated package --foundation produced different ZIP bytes"
    }
    if ([System.Convert]::ToBase64String($firstManifestBytes) -cne [System.Convert]::ToBase64String($secondManifestBytes)) {
        throw "FOUNDATION_NOT_DETERMINISTIC: repeated package --foundation produced different manifest bytes"
    }
    if ([System.Convert]::ToBase64String($firstChecksumBytes) -cne [System.Convert]::ToBase64String($secondChecksumBytes)) {
        throw "FOUNDATION_NOT_DETERMINISTIC: repeated package --foundation produced different checksum sidecar bytes"
    }
    Write-Output "Foundation package acceptance: repeated package --foundation produced byte-identical ZIP, manifest, and checksum output."
}

$scriptExitCode = 1
try {
    $repositoryUnderTest = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot "..\.."))

    switch ($Mode) {
        "core" {
            Invoke-CoreOfflineAcceptance -RepositoryUnderTest $repositoryUnderTest
            Write-Output "Offline acceptance confirmed: exact registered core routes ran through one graph with network denied."
        }
        "package" {
            Invoke-FoundationPackageAcceptance -RepositoryUnderTest $repositoryUnderTest
            Write-Output "Offline acceptance confirmed: package --foundation is reachable and byte-reproducible."
        }
    }

    $scriptExitCode = 0
}
catch {
    [Console]::Error.WriteLine("OFFLINE_ACCEPTANCE_FAILURE: " + $_.Exception.Message)
    $scriptExitCode = 1
}

exit $scriptExitCode
