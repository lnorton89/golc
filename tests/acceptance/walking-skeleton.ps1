[CmdletBinding()]
param(
    [ValidateSet("red", "bootstrap", "green")]
    [string]$Mode = "red",

    [string]$FixtureRoot
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($FixtureRoot)) {
    $FixtureRoot = Join-Path $PSScriptRoot "..\fixtures\config\walking-skeleton"
}

function Assert-FixtureRepository {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path
    )

    $fixture = Get-Item -LiteralPath $Path -ErrorAction Stop
    if (-not $fixture.PSIsContainer) {
        throw "FIXTURE_NOT_DIRECTORY: $Path"
    }

    $entries = @($fixture) + @(Get-ChildItem -LiteralPath $fixture.FullName -Recurse -Force)
    $reparsePoints = @($entries | Where-Object {
        ($_.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0
    })
    if ($reparsePoints.Count -ne 0) {
        throw "FIXTURE_REPARSE_POINT: fixture repositories must not contain links or reparse points"
    }

    $allowedFiles = @(
        "golc.project.toml",
        "config/toolchain.toml",
        "config/runtime.toml"
    )
    $prefix = $fixture.FullName + [System.IO.Path]::DirectorySeparatorChar
    $actualFiles = @(Get-ChildItem -LiteralPath $fixture.FullName -Recurse -Force -File | ForEach-Object {
        $_.FullName.Substring($prefix.Length).Replace("\", "/")
    })

    foreach ($relativePath in $actualFiles) {
        if ($allowedFiles -notcontains $relativePath) {
            throw "FIXTURE_UNEXPECTED_FILE: $relativePath"
        }
    }
    foreach ($relativePath in $allowedFiles) {
        if ($actualFiles -notcontains $relativePath) {
            throw "FIXTURE_MISSING_FILE: $relativePath"
        }
    }
}

function Copy-FixtureRepository {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$Source,

        [Parameter(Mandatory = $true)]
        [string]$Destination,

        [Parameter(Mandatory = $true)]
        [string]$RepositoryUnderTest
    )

    Assert-FixtureRepository -Path $Source
    New-Item -ItemType Directory -Path $Destination -ErrorAction Stop | Out-Null

    foreach ($entry in Get-ChildItem -LiteralPath $Source -Force) {
        Copy-Item -LiteralPath $entry.FullName -Destination $Destination -Recurse -Force -ErrorAction Stop
    }

    # Fixtures are data-only. If the repository has a root command, copy that
    # trusted command into the temporary checkout so a malformed implementation
    # cannot be mistaken for the expected RED state.
    $sourceCommand = Join-Path $RepositoryUnderTest "golc.ps1"
    if (Test-Path -LiteralPath $sourceCommand -PathType Leaf) {
        Copy-Item -LiteralPath $sourceCommand -Destination (Join-Path $Destination "golc.ps1") -Force
    }
}

function Invoke-Golc {
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
            StdErrText     = [System.Text.Encoding]::UTF8.GetString($stderrBytes)
        }
    }
    finally {
        if (Test-Path -LiteralPath $captureRoot) {
            Remove-Item -LiteralPath $captureRoot -Recurse -Force -ErrorAction Stop
        }
    }
}

function New-ChecksumToolchainFixture {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$RepositoryRoot,

        # When provided, the archive carries this real executable instead of
        # the inert bootstrap-mode payload. Green mode packages the actual
        # pinned project command so the temporary checkout runs it for real.
        [string]$PayloadExecutable
    )

    $toolchainPath = Join-Path $RepositoryRoot "config\toolchain.toml"
    if (-not (Test-Path -LiteralPath $toolchainPath -PathType Leaf)) {
        throw "FIXTURE_MISSING_FILE: config/toolchain.toml"
    }

    $archiveRoot = Join-Path $RepositoryRoot ".fixture-toolchain"
    $payloadBin = Join-Path $archiveRoot "payload\bin"
    $archivePath = Join-Path $archiveRoot "golc-project.zip"
    New-Item -ItemType Directory -Path $payloadBin -Force -ErrorAction Stop | Out-Null

    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    if ([string]::IsNullOrWhiteSpace($PayloadExecutable)) {
        [System.IO.File]::WriteAllText(
            (Join-Path $payloadBin "golc-project.exe"),
            "walking-skeleton tool payload`n",
            $utf8NoBom
        )
    }
    else {
        if (-not (Test-Path -LiteralPath $PayloadExecutable -PathType Leaf)) {
            throw "FIXTURE_PAYLOAD_MISSING: $PayloadExecutable"
        }
        Copy-Item -LiteralPath $PayloadExecutable -Destination (Join-Path $payloadBin "golc-project.exe") -Force
    }
    Compress-Archive -Path (Join-Path $archiveRoot "payload\*") -DestinationPath $archivePath -Force

    # The digest is calculated before bootstrap sees the source metadata.
    $sha256 = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    $archiveUri = ([System.Uri]::new($archivePath)).AbsoluteUri

    $toolchainToml = [System.IO.File]::ReadAllText($toolchainPath)
    if (-not $toolchainToml.Contains("__GOLC_FIXTURE_ARCHIVE_URI__")) {
        throw "FIXTURE_ARCHIVE_URI_PLACEHOLDER_MISSING"
    }
    if (-not $toolchainToml.Contains("__GOLC_FIXTURE_ARCHIVE_SHA256__")) {
        throw "FIXTURE_ARCHIVE_SHA256_PLACEHOLDER_MISSING"
    }

    $toolchainToml = $toolchainToml.Replace("__GOLC_FIXTURE_ARCHIVE_URI__", $archiveUri)
    $toolchainToml = $toolchainToml.Replace("__GOLC_FIXTURE_ARCHIVE_SHA256__", $sha256)
    [System.IO.File]::WriteAllText($toolchainPath, $toolchainToml, $utf8NoBom)

    return [pscustomobject]@{
        ArchivePath = $archivePath
        Sha256      = $sha256
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

function Convert-OutputBytesToText {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [byte[]]$Bytes
    )

    if ($Bytes.Length -ge 2 -and $Bytes[0] -eq 0xFF -and $Bytes[1] -eq 0xFE) {
        return [System.Text.Encoding]::Unicode.GetString($Bytes, 2, $Bytes.Length - 2)
    }
    if ($Bytes.Length -ge 3 -and $Bytes[0] -eq 0xEF -and $Bytes[1] -eq 0xBB -and $Bytes[2] -eq 0xBF) {
        return [System.Text.Encoding]::UTF8.GetString($Bytes, 3, $Bytes.Length - 3)
    }
    return [System.Text.Encoding]::UTF8.GetString($Bytes)
}

function Assert-RuntimeInspection {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [byte[]]$Bytes
    )

    $text = (Convert-OutputBytesToText -Bytes $Bytes).Trim()
    if ([string]::IsNullOrWhiteSpace($text)) {
        throw "RUNTIME_INSPECT_EMPTY: expected JSON output"
    }

    try {
        $document = $text | ConvertFrom-Json -ErrorAction Stop
    }
    catch {
        throw "RUNTIME_INSPECT_INVALID_JSON: $($_.Exception.Message)"
    }

    $runtimeProperty = $document.PSObject.Properties["runtime"]
    if ($null -eq $runtimeProperty) {
        throw "RUNTIME_INSPECT_MISSING_VALUE: JSON must contain runtime.log_level"
    }
    $logLevelProperty = $runtimeProperty.Value.PSObject.Properties["log_level"]
    if ($null -eq $logLevelProperty -or $logLevelProperty.Value -ne "info") {
        throw "RUNTIME_INSPECT_WRONG_VALUE: expected runtime.log_level=info"
    }
}

function Get-ModuleCacheInventory {
    <# Stable inventory of the project-local Go module cache: relative path
       plus byte length for every file. Any download or mutation changes it. #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$CacheRoot
    )

    if (-not (Test-Path -LiteralPath $CacheRoot -PathType Container)) {
        throw "BOOTSTRAP_MODULE_CACHE_MISSING: $CacheRoot"
    }
    $resolvedRoot = (Get-Item -LiteralPath $CacheRoot).FullName
    return @(Get-ChildItem -LiteralPath $resolvedRoot -Recurse -Force -File | ForEach-Object {
            $_.FullName.Substring($resolvedRoot.Length).TrimStart("\", "/").Replace("\", "/") + "|" + $_.Length
        } | Sort-Object)
}

function Assert-OfflineSchemaProbe {
    <# Reruns the bootstrap schema probe (the internal/bootstrap tests that
       exercise VerifyArchive/InstallStaged/InstalledMatches and the
       TOML-to-Invopop schema emission) with GOPROXY=off, readonly module
       mode, and a proxy transport that fails any network call, then proves
       go.mod, go.sum, and the warmed module-cache inventory are unchanged. #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$RepositoryRoot
    )

    $toolchainRoot = Join-Path $RepositoryRoot ".tools\toolchains\go"
    if (-not (Test-Path -LiteralPath $toolchainRoot -PathType Container)) {
        throw "OFFLINE_PROBE_GO_MISSING: $toolchainRoot"
    }
    $goExecutable = @(Get-ChildItem -LiteralPath $toolchainRoot -Recurse -Force -Filter "go.exe" |
            Where-Object { $_.Directory.Name -eq "bin" }) | Select-Object -First 1
    if ($null -eq $goExecutable) {
        throw "OFFLINE_PROBE_GO_MISSING: no provisioned go.exe under $toolchainRoot"
    }

    $goModPath = Join-Path $RepositoryRoot "go.mod"
    $goSumPath = Join-Path $RepositoryRoot "go.sum"
    $moduleCache = Join-Path $RepositoryRoot ".tools\cache\go-mod"
    $modBefore = (Get-FileHash -LiteralPath $goModPath -Algorithm SHA256).Hash
    $sumBefore = (Get-FileHash -LiteralPath $goSumPath -Algorithm SHA256).Hash
    $inventoryBefore = Get-ModuleCacheInventory -CacheRoot $moduleCache

    $environmentNames = @(
        "GOPROXY", "GOFLAGS", "GOTOOLCHAIN", "GOMODCACHE", "GOCACHE",
        "GONOSUMDB", "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY"
    )
    $savedEnvironment = @{}
    foreach ($name in $environmentNames) {
        $savedEnvironment[$name] = [System.Environment]::GetEnvironmentVariable($name)
    }
    $probeExitCode = $null
    try {
        # Network denial: module resolution is forbidden outright and any
        # HTTP transport call fails immediately against a dead proxy.
        [System.Environment]::SetEnvironmentVariable("GOPROXY", "off")
        [System.Environment]::SetEnvironmentVariable("GOFLAGS", "-mod=readonly")
        [System.Environment]::SetEnvironmentVariable("GOTOOLCHAIN", "local")
        [System.Environment]::SetEnvironmentVariable("GOMODCACHE", $moduleCache)
        [System.Environment]::SetEnvironmentVariable("GOCACHE", (Join-Path $RepositoryRoot ".tools\cache\go-build"))
        [System.Environment]::SetEnvironmentVariable("HTTP_PROXY", "http://127.0.0.1:9")
        [System.Environment]::SetEnvironmentVariable("HTTPS_PROXY", "http://127.0.0.1:9")
        [System.Environment]::SetEnvironmentVariable("NO_PROXY", $null)

        Push-Location $RepositoryRoot
        try {
            & $goExecutable.FullName test -count=1 `
                -run "TestVerifyArchive|TestInstallStaged|TestInstalledMatches|TestSchemaProbe" `
                ./internal/bootstrap/
            $probeExitCode = $LASTEXITCODE
        }
        finally {
            Pop-Location
        }
    }
    finally {
        foreach ($name in $environmentNames) {
            [System.Environment]::SetEnvironmentVariable($name, $savedEnvironment[$name])
        }
    }

    if ($probeExitCode -ne 0) {
        throw "OFFLINE_PROBE_FAILED: network-denied schema probe exited $probeExitCode"
    }

    $modAfter = (Get-FileHash -LiteralPath $goModPath -Algorithm SHA256).Hash
    $sumAfter = (Get-FileHash -LiteralPath $goSumPath -Algorithm SHA256).Hash
    if ($modAfter -cne $modBefore) {
        throw "OFFLINE_PROBE_LOCK_MUTATION: go.mod changed during the network-denied probe"
    }
    if ($sumAfter -cne $sumBefore) {
        throw "OFFLINE_PROBE_LOCK_MUTATION: go.sum changed during the network-denied probe"
    }
    $inventoryAfter = Get-ModuleCacheInventory -CacheRoot $moduleCache
    $differences = @(Compare-Object -ReferenceObject $inventoryBefore -DifferenceObject $inventoryAfter)
    if ($differences.Count -ne 0) {
        throw "OFFLINE_PROBE_CACHE_MUTATION: module cache inventory changed; a download or mutation occurred"
    }
}

$temporaryRoot = $null
$scriptExitCode = 1
try {
    $resolvedFixtureRoot = (Resolve-Path -LiteralPath $FixtureRoot -ErrorAction Stop).Path
    $repositoryUnderTest = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot "..\.."))
    $temporaryRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("golc-walking-skeleton-" + [guid]::NewGuid().ToString("N"))
    $workingRepository = Join-Path $temporaryRoot "repository"

    New-Item -ItemType Directory -Path $temporaryRoot -ErrorAction Stop | Out-Null
    Copy-FixtureRepository `
        -Source $resolvedFixtureRoot `
        -Destination $workingRepository `
        -RepositoryUnderTest $repositoryUnderTest

    switch ($Mode) {
        "red" {
            $redResult = Invoke-Golc `
                -RepositoryRoot $workingRepository `
                -CommandArguments @("config", "inspect", "runtime", "--format", "json")

            if ($redResult.Classification -ne "missing-root-command") {
                throw "RED_WRONG_FAILURE: expected only an absent golc.ps1, got $($redResult.Classification)"
            }
            Write-Output "RED contract confirmed: golc.ps1 is the only missing implementation."
        }
        "bootstrap" {
            # Stage 1: checksum-controlled provisioning in the temporary
            # checkout. The digest is calculated before bootstrap sees the
            # source metadata, and bootstrap must verify bytes before any
            # extraction (VerifyArchive/InstallStaged contract).
            $toolchainFixture = New-ChecksumToolchainFixture -RepositoryRoot $workingRepository
            if ([string]::IsNullOrWhiteSpace($toolchainFixture.Sha256)) {
                throw "FIXTURE_ARCHIVE_HASH_EMPTY"
            }
            $bootstrapResult = Invoke-Golc -RepositoryRoot $workingRepository -CommandArguments @("bootstrap")
            Assert-GolcSucceeded -Result $bootstrapResult -Operation "bootstrap"

            # Stage 2: the archive source is deleted, so the second bootstrap
            # can only succeed through a matching installed manifest
            # (InstalledMatches contract) with zero archive-source calls.
            Remove-Item -LiteralPath $toolchainFixture.ArchivePath -Force
            $downloadsCache = Join-Path $workingRepository ".tools\cache\downloads"
            if (Test-Path -LiteralPath $downloadsCache) {
                Remove-Item -LiteralPath $downloadsCache -Recurse -Force
            }
            $secondBootstrap = Invoke-Golc -RepositoryRoot $workingRepository -CommandArguments @("bootstrap")
            Assert-GolcSucceeded -Result $secondBootstrap -Operation "second bootstrap without archive source"

            # Stage 3: online cache warm in the repository under test.
            # Bootstrap provisions the pinned Go toolchain, downloads the
            # complete go.mod graph into the project-local module cache,
            # verifies the selected graph and sums, and compiles/runs the
            # TOML + Invopop schema probe online.
            $repositoryBootstrap = Invoke-Golc -RepositoryRoot $repositoryUnderTest -CommandArguments @("bootstrap")
            Assert-GolcSucceeded -Result $repositoryBootstrap -Operation "repository online bootstrap"

            # Stage 4: immediately rerun the schema probe with the network
            # denied and prove resolution required no download or mutation.
            Assert-OfflineSchemaProbe -RepositoryRoot $repositoryUnderTest

            Write-Output "Bootstrap contract confirmed: checksum-controlled install, idempotent manifest match, warmed module graph, and network-denied schema resolution."
        }
        "green" {
            # The green slice runs the real pinned project command inside the
            # temporary checkout. Ensure the repository under test has
            # bootstrapped and built it (offline after the first warm), then
            # package that executable as the checksum-pinned tool archive.
            $repositoryBootstrap = Invoke-Golc -RepositoryRoot $repositoryUnderTest -CommandArguments @("bootstrap")
            Assert-GolcSucceeded -Result $repositoryBootstrap -Operation "repository bootstrap before green"
            $projectExecutable = Join-Path $repositoryUnderTest ".tools\installs\golc_project\bin\golc-project.exe"
            if (-not (Test-Path -LiteralPath $projectExecutable -PathType Leaf)) {
                throw "GREEN_TOOL_MISSING: repository bootstrap did not produce golc-project.exe"
            }

            $toolchainFixture = New-ChecksumToolchainFixture `
                -RepositoryRoot $workingRepository `
                -PayloadExecutable $projectExecutable
            if ([string]::IsNullOrWhiteSpace($toolchainFixture.Sha256)) {
                throw "FIXTURE_ARCHIVE_HASH_EMPTY"
            }
            $bootstrapResult = Invoke-Golc -RepositoryRoot $workingRepository -CommandArguments @("bootstrap")
            Assert-GolcSucceeded -Result $bootstrapResult -Operation "bootstrap"

            $firstInspection = Invoke-Golc `
                -RepositoryRoot $workingRepository `
                -CommandArguments @("config", "inspect", "runtime", "--format", "json")
            $secondInspection = Invoke-Golc `
                -RepositoryRoot $workingRepository `
                -CommandArguments @("config", "inspect", "runtime", "--format", "json")
            Assert-GolcSucceeded -Result $firstInspection -Operation "first runtime inspection"
            Assert-GolcSucceeded -Result $secondInspection -Operation "second runtime inspection"

            $firstBytes = [Convert]::ToBase64String($firstInspection.StdOutBytes)
            $secondBytes = [Convert]::ToBase64String($secondInspection.StdOutBytes)
            if ($firstBytes -cne $secondBytes) {
                throw "RUNTIME_INSPECT_NONDETERMINISTIC: repeated JSON output was not byte-identical"
            }
            Assert-RuntimeInspection -Bytes $firstInspection.StdOutBytes
            Write-Output "Green contract confirmed: runtime.log_level is deterministic and byte-identical."
        }
    }

    $scriptExitCode = 0
}
catch {
    [Console]::Error.WriteLine("WALKING_SKELETON_FAILURE: " + $_.Exception.Message)
    $scriptExitCode = 1
}
finally {
    if ($null -ne $temporaryRoot -and (Test-Path -LiteralPath $temporaryRoot)) {
        try {
            Remove-Item -LiteralPath $temporaryRoot -Recurse -Force -ErrorAction Stop
        }
        catch {
            [Console]::Error.WriteLine("WALKING_SKELETON_CLEANUP_FAILURE: " + $_.Exception.Message)
            $scriptExitCode = 1
        }
    }
}

exit $scriptExitCode
