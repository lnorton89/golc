<#
.SYNOPSIS
Plan 01-07 acceptance: the committed Windows PR workflow
(.github/workflows/check.yml) invokes exactly the same root command graph a
contributor runs locally, in the same order, and contains no Linear secret
reference, remote trigger, or apply-capable command (CONTEXT D-03/D-10/
D-16, T-01-19/T-01-20).

.DESCRIPTION
This script never bootstraps or installs anything itself -- it requires a
prior successful `golc.ps1 bootstrap`, matching
tests/acceptance/offline.ps1's assumption of an already-bootstrapped
checkout.

  1. Before anything runs, LINEAR_API_KEY and LINEAR_TEAM_ID are confirmed
     absent from the process environment, so a passing result can never be
     explained by a coincidentally present credential.
  2. `golc.ps1 check --command-parity` is invoked exactly once. Its handler
     (internal/command/check.go) is the single authority that parses
     config/commands.toml's commands.pr.steps policy and
     .github/workflows/check.yml's own ordered golc.ps1 invocations,
     compares them, and scans the workflow file for a forbidden Linear
     secret reference, a non-pull_request trigger, or an apply-capable
     command. This script never duplicates that parsing or inventory
     itself -- it only asserts the route succeeds and reports its result.
#>
[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Invoke-Golc {
    <# Mirrors tests/acceptance/offline.ps1's Invoke-Golc exactly so every
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

function Assert-NoLinearCredentialPresent {
    <# Confirms LINEAR_API_KEY/LINEAR_TEAM_ID are absent from the process
       environment before this acceptance runs, so a passing result can
       never be explained by a coincidentally present credential
       (CONTEXT D-19/D-21). #>
    [CmdletBinding()]
    param()

    foreach ($credentialVariable in @("LINEAR_API_KEY", "LINEAR_TEAM_ID")) {
        if (Test-Path -LiteralPath "Env:\$credentialVariable") {
            throw "CREDENTIAL_PRESENT: $credentialVariable must not be set for this offline, credential-free acceptance"
        }
    }
}

$scriptExitCode = 1
try {
    $repositoryUnderTest = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot "..\.."))

    Assert-NoLinearCredentialPresent
    Write-Output "Command parity acceptance: no Linear credential is present in the process environment."

    $result = Invoke-Golc -RepositoryRoot $repositoryUnderTest -CommandArguments @("check", "--command-parity")
    Assert-GolcSucceeded -Result $result -Operation "check --command-parity"
    if ($result.StdOutText.Trim().Length -gt 0) {
        Write-Output $result.StdOutText.Trim()
    }
    Write-Output ("Command parity acceptance: the committed PR workflow exactly matches " +
        "config/commands.toml's commands.pr.steps graph in order, with no Linear secret reference, " +
        "remote trigger, or apply-capable command reachable.")

    $scriptExitCode = 0
}
catch {
    [Console]::Error.WriteLine("COMMAND_PARITY_ACCEPTANCE_FAILURE: " + $_.Exception.Message)
    $scriptExitCode = 1
}

exit $scriptExitCode
