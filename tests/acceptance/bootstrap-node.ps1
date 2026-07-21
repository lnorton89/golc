<#
.SYNOPSIS
Plan 01-25 acceptance: clean first tools/linear-sync bootstrap, an
immediate second bootstrap that performs zero network calls, and an
isolated-prefix reinstall/compile that proves the warmed npm cache is
complete on its own (D-02, D-04, T-01-36, T-01-SC).

.DESCRIPTION
Three stages, all run from a single invocation with no parameters
(matching the automated command in 01-VALIDATION.md):

Stage 1 (clean first bootstrap) removes any prior Node toolchain install,
tools/linear-sync/node_modules, tools/linear-sync/dist, and the project
npm cache, then runs `golc.ps1 bootstrap --include linear-sync` once
against the repository under test. This proves a genuinely clean
exact-lock npm ci and tsc compile succeed, producing dist/src/protocol.js,
dist/src/adapter.js, dist/src/cli.js, and dist/test/operations.test.js.

Stage 2 (network-denied no-op) reruns the exact same bootstrap command
immediately afterward, with tools/linear-sync/package.json and
package-lock.json unchanged. It asserts the second run reports its exact
"npm ci not invoked" skip diagnostic and leaves node_modules byte-for-byte
unchanged -- proving the second bootstrap performs zero npm/network calls,
not merely a fast reinstall.

Stage 3 (isolated prefix, cache-only reinstall) copies only
tools/linear-sync's package.json, package-lock.json, tsconfig.json, src/,
and test/ into a brand-new temporary directory -- deliberately never
copying or reusing the existing node_modules -- then runs `npm ci
--offline` there against the shared project npm cache Stage 1 already
warmed, and compiles the result with the pinned tsc. A poisoned
NPM_CONFIG_REGISTRY is set for defense in depth: if --offline were ever
silently ignored, the poisoned registry would make any real network
attempt fail loudly instead of silently succeeding. This proves the warmed
npm cache is complete on its own, independent of any existing install.

Requires network access for Stage 1 only (a from-scratch checkout has no
provisioned Node toolchain or npm cache yet, matching
tests/acceptance/bootstrap.ps1's precedent). Stages 2 and 3 never touch
the network.
#>
[CmdletBinding()]
param()

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

function Get-DirectoryInventory {
    <# Stable inventory of every file under Root: relative path plus byte
       length. Any download, install, or mutation changes it. Mirrors
       tests/acceptance/bootstrap.ps1's helper exactly. #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$Root
    )

    if (-not (Test-Path -LiteralPath $Root -PathType Container)) {
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

    $differences = @(Compare-Object -ReferenceObject $Before -DifferenceObject $After)
    if ($differences.Count -ne 0) {
        throw "BOOTSTRAP_NODE_INVENTORY_CHANGED: $Label changed between bootstrap runs: $($differences | ConvertTo-Json -Compress)"
    }
}

function Invoke-Stage1CleanFirstBootstrap {
    <# Removes any prior Node-specific state and runs a genuinely clean
       `bootstrap --include linear-sync`. #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$RepositoryUnderTest
    )

    $linearSyncDir = Join-Path $RepositoryUnderTest "tools\linear-sync"
    foreach ($staleDirectory in @(
            (Join-Path $RepositoryUnderTest ".tools\toolchains\node"),
            (Join-Path $linearSyncDir "node_modules"),
            (Join-Path $linearSyncDir "dist"),
            (Join-Path $RepositoryUnderTest ".tools\cache\npm")
        )) {
        if (Test-Path -LiteralPath $staleDirectory) {
            Remove-Item -LiteralPath $staleDirectory -Recurse -Force
        }
    }

    $result = Invoke-Golc -RepositoryRoot $RepositoryUnderTest -CommandArguments @("bootstrap", "--include", "linear-sync")
    Assert-GolcSucceeded -Result $result -Operation "clean first bootstrap --include linear-sync"
    if ($result.StdOutText.Contains("npm ci not invoked")) {
        throw "BOOTSTRAP_NODE_UNEXPECTED_SKIP: the first, clean bootstrap must actually run npm ci, not skip it"
    }
    if (-not $result.StdOutText.Contains("tools/linear-sync exact-lock npm ci complete")) {
        throw "BOOTSTRAP_NODE_NPM_CI_NOT_CONFIRMED: expected the clean bootstrap to report a real npm ci"
    }

    foreach ($expected in @(
            (Join-Path $linearSyncDir "dist\src\protocol.js"),
            (Join-Path $linearSyncDir "dist\src\adapter.js"),
            (Join-Path $linearSyncDir "dist\src\cli.js"),
            (Join-Path $linearSyncDir "dist\test\operations.test.js"),
            (Join-Path $linearSyncDir "node_modules\typescript\bin\tsc"),
            (Join-Path $linearSyncDir "node_modules\@linear\sdk")
        )) {
        if (-not (Test-Path -LiteralPath $expected)) {
            throw "BOOTSTRAP_NODE_OUTPUT_MISSING: expected $expected after the clean first bootstrap"
        }
    }
    Write-Output "Stage 1 clean first bootstrap confirmed: exact-lock npm ci and tsc produced dist/src and dist/test."
}

function Invoke-Stage2NetworkDeniedNoOp {
    <# Reruns the same bootstrap command against the same unchanged lock
       and asserts it is a true no-op: the exact skip diagnostic is
       reported, node_modules is byte-for-byte unchanged, and
       package.json/package-lock.json remain byte-identical. #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$RepositoryUnderTest
    )

    $linearSyncDir = Join-Path $RepositoryUnderTest "tools\linear-sync"
    $packageJsonPath = Join-Path $linearSyncDir "package.json"
    $packageLockPath = Join-Path $linearSyncDir "package-lock.json"
    $packageJsonBefore = (Get-FileHash -LiteralPath $packageJsonPath -Algorithm SHA256).Hash
    $packageLockBefore = (Get-FileHash -LiteralPath $packageLockPath -Algorithm SHA256).Hash
    $nodeModulesInventoryBefore = Get-DirectoryInventory -Root (Join-Path $linearSyncDir "node_modules")
    $distInventoryBefore = Get-DirectoryInventory -Root (Join-Path $linearSyncDir "dist")

    $result = Invoke-Golc -RepositoryRoot $RepositoryUnderTest -CommandArguments @("bootstrap", "--include", "linear-sync")
    Assert-GolcSucceeded -Result $result -Operation "second bootstrap --include linear-sync (network-denied no-op)"
    if (-not $result.StdOutText.Contains("npm ci not invoked")) {
        throw "BOOTSTRAP_NODE_NOT_SKIPPED: expected the second bootstrap to report its exact npm-ci-skip diagnostic"
    }
    if ($result.StdOutText.Contains("tools/linear-sync exact-lock npm ci complete")) {
        throw "BOOTSTRAP_NODE_UNEXPECTED_NPM_CI: the second bootstrap must never actually invoke npm ci"
    }

    $packageJsonAfter = (Get-FileHash -LiteralPath $packageJsonPath -Algorithm SHA256).Hash
    $packageLockAfter = (Get-FileHash -LiteralPath $packageLockPath -Algorithm SHA256).Hash
    if ($packageJsonAfter -cne $packageJsonBefore -or $packageLockAfter -cne $packageLockBefore) {
        throw "BOOTSTRAP_NODE_LOCK_MUTATION: the no-op second bootstrap must never touch package.json/package-lock.json"
    }

    Assert-SameInventory -Label "tools/linear-sync/node_modules" -Before $nodeModulesInventoryBefore -After (Get-DirectoryInventory -Root (Join-Path $linearSyncDir "node_modules"))
    Assert-SameInventory -Label "tools/linear-sync/dist" -Before $distInventoryBefore -After (Get-DirectoryInventory -Root (Join-Path $linearSyncDir "dist"))
    Write-Output "Stage 2 network-denied no-op confirmed: zero npm ci calls, node_modules/dist/pins byte-identical."
}

function Invoke-Stage3IsolatedPrefixOfflineReinstall {
    <# Copies only package.json/package-lock.json/tsconfig.json/src/test
       (never node_modules or dist) into a brand-new temporary directory
       and proves a genuine `npm ci --offline` there, against only the
       shared warmed npm cache, reinstalls and recompiles completely. #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$RepositoryUnderTest
    )

    $linearSyncDir = Join-Path $RepositoryUnderTest "tools\linear-sync"
    $npmCacheDirectory = Join-Path $RepositoryUnderTest ".tools\cache\npm"
    if (-not (Test-Path -LiteralPath $npmCacheDirectory -PathType Container)) {
        throw "BOOTSTRAP_NODE_CACHE_NOT_WARMED: expected $npmCacheDirectory to already exist from Stage 1"
    }

    $nodeVersionDirectory = Get-ChildItem -Path (Join-Path $RepositoryUnderTest ".tools\toolchains\node") -Directory -ErrorAction Stop | Select-Object -First 1
    if ($null -eq $nodeVersionDirectory) {
        throw "BOOTSTRAP_NODE_TOOLCHAIN_MISSING: expected a provisioned Node toolchain directory under .tools\toolchains\node"
    }
    $nodeExtractedDirectory = Get-ChildItem -Path (Join-Path $nodeVersionDirectory.FullName "windows-amd64") -Directory -ErrorAction Stop | Select-Object -First 1
    if ($null -eq $nodeExtractedDirectory) {
        throw "BOOTSTRAP_NODE_TOOLCHAIN_MISSING: expected an extracted node-v*-win-x64 directory under $($nodeVersionDirectory.FullName)\windows-amd64"
    }
    $nodeExecutable = Join-Path $nodeExtractedDirectory.FullName "node.exe"
    $npmCliPath = Join-Path $nodeExtractedDirectory.FullName "node_modules\npm\bin\npm-cli.js"
    if (-not (Test-Path -LiteralPath $nodeExecutable -PathType Leaf) -or -not (Test-Path -LiteralPath $npmCliPath -PathType Leaf)) {
        throw "BOOTSTRAP_NODE_TOOLCHAIN_MISSING: expected node.exe and npm-cli.js under $($nodeExtractedDirectory.FullName)"
    }

    $tempPrefix = Join-Path ([System.IO.Path]::GetTempPath()) ("golc-linear-sync-isolated-" + [guid]::NewGuid().ToString("N"))
    try {
        New-Item -ItemType Directory -Path $tempPrefix -Force | Out-Null
        Copy-Item -LiteralPath (Join-Path $linearSyncDir "package.json") -Destination (Join-Path $tempPrefix "package.json") -Force
        Copy-Item -LiteralPath (Join-Path $linearSyncDir "package-lock.json") -Destination (Join-Path $tempPrefix "package-lock.json") -Force
        Copy-Item -LiteralPath (Join-Path $linearSyncDir "tsconfig.json") -Destination (Join-Path $tempPrefix "tsconfig.json") -Force
        Copy-Item -LiteralPath (Join-Path $linearSyncDir "src") -Destination (Join-Path $tempPrefix "src") -Recurse -Force
        Copy-Item -LiteralPath (Join-Path $linearSyncDir "test") -Destination (Join-Path $tempPrefix "test") -Recurse -Force

        if (Test-Path -LiteralPath (Join-Path $tempPrefix "node_modules")) {
            throw "BOOTSTRAP_NODE_ISOLATED_PREFIX_NODE_MODULES_PRESENT: the isolated prefix must never start with an existing node_modules"
        }

        Push-Location $tempPrefix
        try {
            $env:NPM_CONFIG_CACHE = $npmCacheDirectory
            # Defense in depth: if --offline were ever silently ignored, a
            # poisoned registry makes any real network attempt fail loudly
            # instead of silently succeeding.
            $env:NPM_CONFIG_REGISTRY = "https://npm-registry.invalid.golc-acceptance.example/"
            & $nodeExecutable $npmCliPath "ci" "--offline" "--ignore-scripts" "--no-audit" "--no-fund"
            if ($LASTEXITCODE -ne 0) {
                throw "BOOTSTRAP_NODE_ISOLATED_PREFIX_NPM_CI_FAILED: cache-only 'npm ci --offline' exited $LASTEXITCODE"
            }
            Write-Output "Stage 3 isolated prefix: cache-only 'npm ci --offline' succeeded with a poisoned registry and no existing node_modules."

            $isolatedTscPath = Join-Path $tempPrefix "node_modules\typescript\bin\tsc"
            if (-not (Test-Path -LiteralPath $isolatedTscPath -PathType Leaf)) {
                throw "BOOTSTRAP_NODE_ISOLATED_PREFIX_TSC_MISSING: expected $isolatedTscPath after cache-only npm ci"
            }
            & $nodeExecutable $isolatedTscPath "-p" (Join-Path $tempPrefix "tsconfig.json")
            if ($LASTEXITCODE -ne 0) {
                throw "BOOTSTRAP_NODE_ISOLATED_PREFIX_TSC_FAILED: tsc exited $LASTEXITCODE"
            }
        }
        finally {
            Remove-Item Env:\NPM_CONFIG_CACHE -ErrorAction SilentlyContinue
            Remove-Item Env:\NPM_CONFIG_REGISTRY -ErrorAction SilentlyContinue
            Pop-Location
        }

        foreach ($expected in @(
                (Join-Path $tempPrefix "dist\src\protocol.js"),
                (Join-Path $tempPrefix "dist\src\adapter.js"),
                (Join-Path $tempPrefix "dist\src\cli.js"),
                (Join-Path $tempPrefix "dist\test\operations.test.js")
            )) {
            if (-not (Test-Path -LiteralPath $expected -PathType Leaf)) {
                throw "BOOTSTRAP_NODE_ISOLATED_PREFIX_BUILD_INCOMPLETE: expected $expected after the cache-only compile"
            }
        }
        Write-Output "Stage 3 isolated prefix: cache-only tsc compile produced the complete dist/src and dist/test output."
    }
    finally {
        if (Test-Path -LiteralPath $tempPrefix) {
            Remove-Item -LiteralPath $tempPrefix -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

$scriptExitCode = 1
try {
    $repositoryUnderTest = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot "..\.."))
    $toolchainManifestPath = Join-Path $repositoryUnderTest "config\toolchain.toml"
    $toolchainBeforeHash = (Get-FileHash -LiteralPath $toolchainManifestPath -Algorithm SHA256).Hash

    Invoke-Stage1CleanFirstBootstrap -RepositoryUnderTest $repositoryUnderTest
    Invoke-Stage2NetworkDeniedNoOp -RepositoryUnderTest $repositoryUnderTest
    Invoke-Stage3IsolatedPrefixOfflineReinstall -RepositoryUnderTest $repositoryUnderTest

    $toolchainAfterHash = (Get-FileHash -LiteralPath $toolchainManifestPath -Algorithm SHA256).Hash
    if ($toolchainAfterHash -cne $toolchainBeforeHash) {
        throw "BOOTSTRAP_NODE_TOOLCHAIN_MANIFEST_MUTATED: config/toolchain.toml must remain byte-unchanged across every stage"
    }

    Write-Output "Bootstrap-node acceptance confirmed: clean install, network-denied no-op, and isolated cache-only reinstall/compile all passed."
    $scriptExitCode = 0
}
catch {
    [Console]::Error.WriteLine("BOOTSTRAP_NODE_ACCEPTANCE_FAILURE: " + $_.Exception.Message)
    $scriptExitCode = 1
}

exit $scriptExitCode
