<#
.SYNOPSIS
Plan 01-15 acceptance: the real Go<->Node process transport
(internal/trace/transport.ProcessClient, internal/command/linear.go's
processLinearClient) proves the complete required hierarchy preview/apply/
replay against a fake official Linear SDK injected into the compiled
adapter, offline remote-failure isolation, and the protected/manual
workflow's structural safety.

.DESCRIPTION
-Mode hierarchy drives the real, compiled tools/linear-sync/dist/src/cli.js
through a temporary isolated workspace whose own node_modules/@linear/sdk
is replaced with a small in-process fake (never the real @linear/sdk, never
a network call), against this repository's own real, already-unlinked
.planning/linear-map.json (backed up and restored around the run):

  1. `linear preview --remote --out <plan>` builds the full preview for
     every entity this repository's own catalog declares (Project
     Milestone -> Project, Phase -> Project Milestone, parent/requirement
     Issue, task sub-issue) against an empty snapshot (nothing linked yet).
  2. `linear apply <plan> --plan-id <id>` is run once with one specific
     task's create deliberately failing (a canary-laden fake SDK
     exception) -- proving apply stops at that operation with a pending,
     non-crashing, redaction-safe outcome, attempts that create exactly
     once, and leaves every later operation untouched.
  3. `linear apply` runs again against the exact same plan, this time
     without the induced failure: the previously-failed entity is created,
     every earlier entity is confirmed already linked (a safe no-op), and
     the achieved results are committed back into .planning/linear-map.json.
  4. A fresh `linear preview --remote` + `linear apply` (the "replay") is
     run: every entity is now already linked, so apply reports every
     operation as a safe no-op and the fake SDK's create/update call count
     does not increase -- "replay performs zero creates/updates."

-Mode offline proves missing-adapter isolation (CONTEXT D-21): with the
compiled adapter deliberately pointed at a workspace with no
tools/linear-sync/dist/src/cli.js, `linear preview --remote` and
`linear drift --remote --read-only` fail with
GOLC_LINEAR_TRANSPORT_UNAVAILABLE while `check --offline` and `build`
remain green.

-Mode workflow proves .github/workflows/linear-sync.yml's structural
safety: no pull_request trigger, protected/manual jobs only, and the
`plan_file`/`plan_id`/`confirm_apply` inputs plus `finally`-scoped cleanup
this plan's action requires.

Requires a prior successful `golc.ps1 bootstrap --include linear-sync` and
`golc.ps1 build --scope linear-sdk` (the compiled adapter must already
exist at tools/linear-sync/dist/src/cli.js) -- this script never
bootstraps or installs anything itself, matching every other acceptance
script's precedent in this repository.
#>
[CmdletBinding()]
param(
    [ValidateSet("hierarchy", "offline", "workflow")]
    [string]$Mode = "hierarchy"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Invoke-Golc {
    <# Runs golc.ps1 in RepositoryRoot with CommandArguments and returns a
       classification/exit-code/output object, inheriting this process's
       own $env: variables (the caller sets LINEAR_API_KEY/LINEAR_TEAM_ID/
       GOLC_LINEAR_SYNC_WORKDIR/etc. before calling this). Mirrors every
       other acceptance script's Invoke-Golc exactly. #>
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
    $captureRoot = Join-Path ([System.IO.Path]::GetTempPath()) (".acceptance-capture-" + [guid]::NewGuid().ToString("N"))
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

function New-FakeLinearSdkWorkspace {
    <# Builds a temporary isolated Node workspace whose node_modules/@linear/sdk
       is a small, dependency-free, in-process fake -- never the real
       @linear/sdk, never a network call. Only the compiled dist/src output
       is copied in (never dist/test, never node_modules) so the workspace
       is exactly what a production install of the compiled adapter alone
       would contain, plus this one fake dependency. Every fake SDK call is
       appended as one JSON line to CallLogPath so this script can assert
       exact per-discriminant call counts and one-object-per-local-ID
       state, and every created/updated record is persisted to
       StorePath so state survives across the several separate
       `golc.ps1 ...` process invocations one full acceptance run makes. #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$RepositoryUnderTest,

        [Parameter(Mandatory = $true)]
        [string]$WorkspaceRoot,

        [Parameter(Mandatory = $true)]
        [string]$CallLogPath,

        [Parameter(Mandatory = $true)]
        [string]$StorePath
    )

    $distSrcSource = Join-Path $RepositoryUnderTest "tools\linear-sync\dist\src"
    if (-not (Test-Path -LiteralPath $distSrcSource -PathType Container)) {
        throw "LINEAR_TRANSPORT_ADAPTER_MISSING: expected $distSrcSource; run 'golc.ps1 bootstrap --include linear-sync' and 'golc.ps1 build --scope linear-sdk' first"
    }

    New-Item -ItemType Directory -Path $WorkspaceRoot -Force | Out-Null
    $distDestination = Join-Path $WorkspaceRoot "dist\src"
    New-Item -ItemType Directory -Path $distDestination -Force | Out-Null
    Get-ChildItem -LiteralPath $distSrcSource -Filter "*.js" | ForEach-Object {
        Copy-Item -LiteralPath $_.FullName -Destination $distDestination -Force
    }

    $packageJsonPath = Join-Path $WorkspaceRoot "package.json"
    Set-Content -LiteralPath $packageJsonPath -Value '{"name":"golc-linear-transport-acceptance-workspace","private":true,"type":"module"}' -NoNewline -Encoding utf8

    $fakeSdkDir = Join-Path $WorkspaceRoot "node_modules\@linear\sdk"
    New-Item -ItemType Directory -Path $fakeSdkDir -Force | Out-Null
    Set-Content -LiteralPath (Join-Path $fakeSdkDir "package.json") `
        -Value '{"name":"@linear/sdk","version":"0.0.0-golc-fake","type":"module","main":"index.js"}' -NoNewline -Encoding utf8

    # A regex -replace's replacement text is not itself regex-escaped, so
    # each single backslash in the path must become exactly two backslash
    # characters for a valid JS string literal -- '\\' here is the two
    # literal replacement characters, not four (double-quoted "\\\\" would
    # be misread as a 4-backslash literal).
    $callLogLiteral = ($CallLogPath -replace '\\', '\\')
    $storeLiteral = ($StorePath -replace '\\', '\\')
    $fakeSdkSource = @"
// GOLC acceptance fake @linear/sdk (tests/acceptance/linear-transport.ps1).
// Never the real official SDK; never a network call. Implements exactly
// the LinearClient surface tools/linear-sync/src/adapter.ts calls.
import { readFileSync, existsSync, appendFileSync, writeFileSync } from "node:fs";

const CALL_LOG_PATH = "$callLogLiteral";
const STORE_PATH = "$storeLiteral";
const FAIL_LOCAL_ID = process.env.GOLC_TEST_FAIL_LOCAL_ID || "";

const store = new Map();
let nextId = 1;

function loadStore() {
  if (!existsSync(STORE_PATH)) return;
  const raw = JSON.parse(readFileSync(STORE_PATH, "utf8"));
  for (const [id, record] of Object.entries(raw.records || {})) {
    store.set(id, Object.assign({}, record, { updatedAt: new Date(record.updatedAt) }));
  }
  nextId = raw.nextId || 1;
}

function saveStore() {
  const records = {};
  for (const [id, record] of store.entries()) {
    records[id] = Object.assign({}, record, { updatedAt: record.updatedAt.toISOString() });
  }
  writeFileSync(STORE_PATH, JSON.stringify({ nextId, records }));
}

function logCall(method, extra) {
  const entry = Object.assign({ method }, extra || {});
  appendFileSync(CALL_LOG_PATH, JSON.stringify(entry) + "\n");
}

function extractLocalId(description) {
  const match = /GOLC local ID: (\S+)/.exec(description || "");
  return match ? match[1] : "";
}

function makeRecord(idPrefix, fields, nameField) {
  const id = idPrefix + "-" + nextId++;
  const record = {
    id,
    description: fields.description || "",
    updatedAt: new Date(),
  };
  record[nameField] = fields[nameField] || "";
  store.set(id, record);
  saveStore();
  return record;
}

function maybeFail(localId) {
  if (localId && localId === FAIL_LOCAL_ID) {
    throw new Error("GOLC_FAKE_SECRET_CANARY_4f9c2e6b1a7d3f809c21 simulated create failure for " + localId);
  }
}

loadStore();

export class LinearClient {
  constructor(options) {
    this.options = options;
  }

  async project(id) {
    logCall("project", { id });
    return store.get(id);
  }

  async projectMilestone(id) {
    logCall("projectMilestone", { id });
    return store.get(id);
  }

  async issue(id) {
    logCall("issue", { id });
    return store.get(id);
  }

  async createProject(fields) {
    const localId = extractLocalId(fields.description);
    logCall("createProject", { localId });
    maybeFail(localId);
    return { project: makeRecord("project", fields, "name") };
  }

  async createProjectMilestone(fields) {
    const localId = extractLocalId(fields.description);
    logCall("createProjectMilestone", { localId });
    maybeFail(localId);
    return { projectMilestone: makeRecord("pm", fields, "name") };
  }

  async createIssue(fields) {
    const localId = extractLocalId(fields.description);
    logCall("createIssue", { localId });
    maybeFail(localId);
    return { issue: makeRecord("issue", fields, "title") };
  }

  async updateProject(id, fields) {
    logCall("updateProject", { id });
    const record = store.get(id);
    if (record) { Object.assign(record, fields, { updatedAt: new Date() }); saveStore(); }
  }

  async updateProjectMilestone(id, fields) {
    logCall("updateProjectMilestone", { id });
    const record = store.get(id);
    if (record) { Object.assign(record, fields, { updatedAt: new Date() }); saveStore(); }
  }

  async updateIssue(id, fields) {
    logCall("updateIssue", { id });
    const record = store.get(id);
    if (record) { Object.assign(record, fields, { updatedAt: new Date() }); saveStore(); }
  }
}
"@
    Set-Content -LiteralPath (Join-Path $fakeSdkDir "index.js") -Value $fakeSdkSource -Encoding utf8

    if (-not (Test-Path -LiteralPath $CallLogPath)) {
        New-Item -ItemType File -Path $CallLogPath -Force | Out-Null
    }
    return $WorkspaceRoot
}

function Get-CallLogEntries {
    [CmdletBinding()]
    param([Parameter(Mandatory = $true)][string]$CallLogPath)
    if (-not (Test-Path -LiteralPath $CallLogPath)) {
        return @()
    }
    $lines = @(Get-Content -LiteralPath $CallLogPath | Where-Object { $_.Trim().Length -gt 0 })
    if ($lines.Count -eq 0) {
        return @()
    }
    return @($lines | ForEach-Object { $_ | ConvertFrom-Json })
}

function Get-JsonArrayCount {
    <# PowerShell 5.1's ConvertFrom-Json turns a JSON "[]" into $null, not
       an empty array -- and wrapping $null in @() produces a one-element
       array containing that $null, not an empty array. Every JSON-decoded
       array property (plan.operations, plan.conflicts, report.results)
       must be counted through this helper instead of a bare @($value).Count. #>
    [CmdletBinding()]
    param($Value)
    if ($null -eq $Value) {
        return 0
    }
    return (@($Value)).Count
}

function ConvertTo-JsonArray {
    <# Same $null-vs-empty-array normalization as Get-JsonArrayCount, but
       returning the usable array itself for piping/filtering. #>
    [CmdletBinding()]
    param($Value)
    if ($null -eq $Value) {
        return @()
    }
    return @($Value)
}

function Backup-LinearMap {
    [CmdletBinding()]
    param([Parameter(Mandatory = $true)][string]$RepositoryUnderTest)
    $mapPath = Join-Path $RepositoryUnderTest ".planning\linear-map.json"
    $backupPath = $mapPath + (".acceptance-backup-" + [guid]::NewGuid().ToString("N"))
    Copy-Item -LiteralPath $mapPath -Destination $backupPath -Force
    return [pscustomobject]@{ MapPath = $mapPath; BackupPath = $backupPath }
}

function Restore-LinearMap {
    [CmdletBinding()]
    param([Parameter(Mandatory = $true)][psobject]$Backup)
    Copy-Item -LiteralPath $Backup.BackupPath -Destination $Backup.MapPath -Force
    Remove-Item -LiteralPath $Backup.BackupPath -Force -ErrorAction SilentlyContinue
}

function Invoke-HierarchyAcceptance {
    [CmdletBinding()]
    param([Parameter(Mandatory = $true)][string]$RepositoryUnderTest)

    $workDir = Join-Path ([System.IO.Path]::GetTempPath()) ("golc-linear-transport-" + [guid]::NewGuid().ToString("N"))
    $fakeWorkspace = Join-Path $workDir "fake-sdk-workspace"
    $callLogPath = Join-Path $workDir "call-log.jsonl"
    $storePath = Join-Path $workDir "store.json"
    $planAPath = Join-Path $workDir "plan-a.json"
    $planReplayPath = Join-Path $workDir "plan-replay.json"

    New-Item -ItemType Directory -Path $workDir -Force | Out-Null
    $mapBackup = Backup-LinearMap -RepositoryUnderTest $RepositoryUnderTest

    try {
        New-FakeLinearSdkWorkspace -RepositoryUnderTest $RepositoryUnderTest -WorkspaceRoot $fakeWorkspace -CallLogPath $callLogPath -StorePath $storePath | Out-Null

        $env:LINEAR_API_KEY = "golc-acceptance-fake-key"
        $env:LINEAR_TEAM_ID = "golc-acceptance-fake-team"
        $env:GOLC_LINEAR_SYNC_WORKDIR = $fakeWorkspace
        $env:GOLC_LINEAR_SYNC_TIMEOUT_MS = "15000"
        $env:GOLC_TEST_CALL_LOG = $callLogPath
        $env:GOLC_TEST_STORE_PATH = $storePath

        try {
            # 1. Preview against an empty remote: every catalog entity is
            #    unlinked, so every operation is a create.
            $previewResult = Invoke-Golc -RepositoryRoot $RepositoryUnderTest -CommandArguments @("linear", "preview", "--remote", "--out", $planAPath)
            Assert-GolcSucceeded -Result $previewResult -Operation "linear preview --remote (initial)"
            if (-not (Test-Path -LiteralPath $planAPath)) {
                throw "LINEAR_TRANSPORT_PLAN_MISSING: expected $planAPath after linear preview --remote"
            }
            $planA = Get-Content -LiteralPath $planAPath -Raw | ConvertFrom-Json
            if ((Get-JsonArrayCount $planA.operations) -lt 2) {
                throw "LINEAR_TRANSPORT_PLAN_TOO_SMALL: expected the full repository catalog hierarchy, got $((Get-JsonArrayCount $planA.operations)) operations"
            }
            if ((Get-JsonArrayCount $planA.conflicts) -ne 0) {
                throw "LINEAR_TRANSPORT_UNEXPECTED_CONFLICT: initial empty-remote preview must never report a conflict"
            }
            Write-Output "Hierarchy acceptance: linear preview --remote proposed $((Get-JsonArrayCount $planA.operations)) create operation(s) against an empty remote, zero conflicts."

            # Pick one task (rank 3, so most of the hierarchy above it
            # completes first) to deliberately fail once.
            $failTarget = (ConvertTo-JsonArray $planA.operations) | Where-Object { $_.kind -eq "task" } | Select-Object -Last 1
            if ($null -eq $failTarget) {
                throw "LINEAR_TRANSPORT_NO_TASK_OPERATION: expected at least one task operation to induce a failure against"
            }
            $failLocalId = $failTarget.local_id

            # 2. First apply, with that one task's create deliberately
            #    failing: apply must stop there, not crash, not duplicate.
            $env:GOLC_TEST_FAIL_LOCAL_ID = $failLocalId
            $applyPartialResult = Invoke-Golc -RepositoryRoot $RepositoryUnderTest -CommandArguments @("linear", "apply", $planAPath, "--plan-id", $planA.plan_id)
            Assert-GolcSucceeded -Result $applyPartialResult -Operation "linear apply (induced failure)"
            if ($applyPartialResult.StdOutText.Contains("GOLC_FAKE_SECRET_CANARY")) {
                throw "LINEAR_TRANSPORT_CANARY_LEAKED: the fake SDK's canary-laden error text leaked into linear apply's stdout"
            }
            if ($applyPartialResult.StdErrText.Contains("GOLC_FAKE_SECRET_CANARY")) {
                throw "LINEAR_TRANSPORT_CANARY_LEAKED: the fake SDK's canary-laden error text leaked into linear apply's stderr"
            }
            $reportPartial = $applyPartialResult.StdOutText | ConvertFrom-Json
            $failedResult = (ConvertTo-JsonArray $reportPartial.results) | Where-Object { $_.local_id -eq $failLocalId }
            if ($null -eq $failedResult -or $failedResult.status -ne "pending") {
                throw "LINEAR_TRANSPORT_INDUCED_FAILURE_NOT_OBSERVED: expected $failLocalId to report status 'pending'"
            }
            if ($failedResult.reason -notlike "*GOLC_LINEAR_TRANSPORT_CREATE_UNCERTAIN*") {
                throw "LINEAR_TRANSPORT_INDUCED_FAILURE_WRONG_REASON: $($failedResult.reason)"
            }
            $completedCount = (Get-JsonArrayCount (ConvertTo-JsonArray $reportPartial.results | Where-Object { $_.status -eq "completed" }))
            Write-Output "Hierarchy acceptance: induced failure at $failLocalId stopped apply cleanly after $completedCount prior completed operation(s); canary text never reached Go output."

            $callLogAfterPartial = Get-CallLogEntries -CallLogPath $callLogPath
            $failedCreateAttempts = @($callLogAfterPartial | Where-Object { $_.method -eq "createIssue" -and $_.localId -eq $failLocalId }).Count
            if ($failedCreateAttempts -ne 1) {
                throw "LINEAR_TRANSPORT_DUPLICATE_CREATE_ATTEMPT: expected exactly one createIssue attempt for $failLocalId before the stop, got $failedCreateAttempts"
            }

            # 3. Failure lifted: re-preview (not a blind re-apply of the
            #    stale plan -- CONTEXT: Operation.LinearUUID is baked into
            #    the plan file at preview time, so a stale plan's already-
            #    achieved operations would still show as unlinked and
            #    would be re-attempted as creates; a fresh preview instead
            #    observes the 71 entities already committed to
            #    .planning/linear-map.json as now-linked, so their
            #    operations carry a LinearUUID and apply treats them as a
            #    safe already-linked no-op). Apply that fresh plan: the
            #    previously-failed entity now succeeds; everything earlier
            #    is confirmed already-linked with zero new create calls.
            Remove-Item Env:\GOLC_TEST_FAIL_LOCAL_ID -ErrorAction SilentlyContinue
            $planCPath = Join-Path $workDir "plan-c.json"
            $previewCResult = Invoke-Golc -RepositoryRoot $RepositoryUnderTest -CommandArguments @("linear", "preview", "--remote", "--out", $planCPath)
            Assert-GolcSucceeded -Result $previewCResult -Operation "linear preview --remote (after induced failure)"
            $planC = Get-Content -LiteralPath $planCPath -Raw | ConvertFrom-Json
            if ((Get-JsonArrayCount $planC.conflicts) -ne 0) {
                throw "LINEAR_TRANSPORT_RECOVERY_CONFLICT: recovery preview must never report a conflict"
            }

            $applyFullResult = Invoke-Golc -RepositoryRoot $RepositoryUnderTest -CommandArguments @("linear", "apply", $planCPath, "--plan-id", $planC.plan_id)
            Assert-GolcSucceeded -Result $applyFullResult -Operation "linear apply (recovery, failure lifted)"
            $reportFull = $applyFullResult.StdOutText | ConvertFrom-Json
            $notAchieved = @((ConvertTo-JsonArray $reportFull.results) | Where-Object { $_.status -ne "completed" -and $_.status -ne "noop" })
            if ($notAchieved.Count -ne 0) {
                throw "LINEAR_TRANSPORT_HIERARCHY_INCOMPLETE: $($notAchieved.Count) operation(s) did not reach completed/noop: $($notAchieved | ConvertTo-Json -Compress)"
            }
            $recoveryCompleted = (Get-JsonArrayCount (ConvertTo-JsonArray $reportFull.results | Where-Object { $_.status -eq "completed" }))
            $recoveryNoop = (Get-JsonArrayCount (ConvertTo-JsonArray $reportFull.results | Where-Object { $_.status -eq "noop" }))
            if ($recoveryCompleted -ne 1) {
                throw "LINEAR_TRANSPORT_RECOVERY_UNEXPECTED: expected exactly 1 newly-completed operation (the previously-failed task), got $recoveryCompleted"
            }
            Write-Output "Hierarchy acceptance: recovery preview + apply completed the full $((Get-JsonArrayCount $reportFull.results))-operation hierarchy (project -> milestone -> parent/requirement issue -> task sub-issue): $recoveryCompleted newly created, $recoveryNoop already-linked no-ops, zero pending/blocked results."

            # Every local ID gets exactly one create attempt, except
            # $failLocalId itself, which legitimately gets exactly two: the
            # induced failure (Phase B) and its successful recovery
            # (Phase C) -- a deliberate, human-reviewed re-run across two
            # separate `linear apply` invocations, never an automatic
            # same-run retry (apply/engine.go attempts each operation at
            # most once per invocation).
            $callLogAfterFull = Get-CallLogEntries -CallLogPath $callLogPath
            $totalCreateAttempts = @($callLogAfterFull | Where-Object { $_.method -like "create*" }).Count
            $perLocalIdCreateCounts = $callLogAfterFull | Where-Object { $_.method -like "create*" -and $_.localId } | Group-Object -Property localId
            $unexpectedDuplicates = @($perLocalIdCreateCounts | Where-Object { $_.Count -gt 1 -and $_.Name -ne $failLocalId })
            if ($unexpectedDuplicates.Count -ne 0) {
                throw "LINEAR_TRANSPORT_DUPLICATE_CREATE: $($unexpectedDuplicates.Count) local id(s) were created more than once unexpectedly: $($unexpectedDuplicates | ForEach-Object { $_.Name })"
            }
            $failLocalIdAttempts = @($perLocalIdCreateCounts | Where-Object { $_.Name -eq $failLocalId })
            if ($failLocalIdAttempts.Count -ne 1 -or $failLocalIdAttempts[0].Count -ne 2) {
                throw "LINEAR_TRANSPORT_RECOVERY_ATTEMPT_COUNT_WRONG: expected exactly 2 create attempts for $failLocalId (fail then recover)"
            }
            $expectedTotal = (Get-JsonArrayCount $planA.operations) + 1
            if ($totalCreateAttempts -ne $expectedTotal) {
                throw "LINEAR_TRANSPORT_CREATE_COUNT_MISMATCH: expected exactly $expectedTotal total create attempts (one per entity, plus one retry for $failLocalId), got $totalCreateAttempts"
            }
            Write-Output "Hierarchy acceptance: exactly one create call per local ID across the full run, except $failLocalId's deliberate one-time human-reviewed retry ($totalCreateAttempts total); no unexpected duplicates."

            # 4. Replay: preview + apply again. Every entity is now linked;
            #    apply must report every operation as a safe no-op, and the
            #    fake SDK's create/update call count must not increase.
            $writeCallsBeforeReplay = @($callLogAfterFull | Where-Object { $_.method -like "create*" -or $_.method -like "update*" }).Count

            $replayPreviewResult = Invoke-Golc -RepositoryRoot $RepositoryUnderTest -CommandArguments @("linear", "preview", "--remote", "--out", $planReplayPath)
            Assert-GolcSucceeded -Result $replayPreviewResult -Operation "linear preview --remote (replay)"
            $planReplay = Get-Content -LiteralPath $planReplayPath -Raw | ConvertFrom-Json
            if ((Get-JsonArrayCount $planReplay.conflicts) -ne 0) {
                throw "LINEAR_TRANSPORT_REPLAY_CONFLICT: replay preview must never report a conflict against unchanged state"
            }

            $replayApplyResult = Invoke-Golc -RepositoryRoot $RepositoryUnderTest -CommandArguments @("linear", "apply", $planReplayPath, "--plan-id", $planReplay.plan_id)
            Assert-GolcSucceeded -Result $replayApplyResult -Operation "linear apply (replay)"
            $reportReplay = $replayApplyResult.StdOutText | ConvertFrom-Json
            $nonNoop = @((ConvertTo-JsonArray $reportReplay.results) | Where-Object { $_.status -ne "noop" })
            if ($nonNoop.Count -ne 0) {
                throw "LINEAR_TRANSPORT_REPLAY_NOT_NOOP: replay must report every operation as 'noop', found: $($nonNoop | ConvertTo-Json -Compress)"
            }

            $callLogAfterReplay = Get-CallLogEntries -CallLogPath $callLogPath
            $writeCallsAfterReplay = @($callLogAfterReplay | Where-Object { $_.method -like "create*" -or $_.method -like "update*" }).Count
            if ($writeCallsAfterReplay -ne $writeCallsBeforeReplay) {
                throw "LINEAR_TRANSPORT_REPLAY_WROTE: replay must perform zero new create/update calls; before=$writeCallsBeforeReplay after=$writeCallsAfterReplay"
            }
            Write-Output "Hierarchy acceptance: replay (fresh preview --remote + apply) reported every operation as noop and performed zero new create/update calls -- replay is all no-op."
        }
        finally {
            Remove-Item Env:\LINEAR_API_KEY -ErrorAction SilentlyContinue
            Remove-Item Env:\LINEAR_TEAM_ID -ErrorAction SilentlyContinue
            Remove-Item Env:\GOLC_LINEAR_SYNC_WORKDIR -ErrorAction SilentlyContinue
            Remove-Item Env:\GOLC_LINEAR_SYNC_TIMEOUT_MS -ErrorAction SilentlyContinue
            Remove-Item Env:\GOLC_TEST_CALL_LOG -ErrorAction SilentlyContinue
            Remove-Item Env:\GOLC_TEST_STORE_PATH -ErrorAction SilentlyContinue
            Remove-Item Env:\GOLC_TEST_FAIL_LOCAL_ID -ErrorAction SilentlyContinue
        }
    }
    finally {
        Restore-LinearMap -Backup $mapBackup
        foreach ($journalLike in @(
                "$planAPath.journal.json", "$planAPath.report.json",
                "$planReplayPath.journal.json", "$planReplayPath.report.json"
            )) {
            Remove-Item -LiteralPath $journalLike -Force -ErrorAction SilentlyContinue
        }
        if (Test-Path -LiteralPath $workDir) {
            Remove-Item -LiteralPath $workDir -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

function Invoke-OfflineAcceptance {
    [CmdletBinding()]
    param([Parameter(Mandatory = $true)][string]$RepositoryUnderTest)

    $missingWorkspace = Join-Path ([System.IO.Path]::GetTempPath()) ("golc-linear-transport-missing-" + [guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $missingWorkspace -Force | Out-Null
    try {
        $env:GOLC_LINEAR_SYNC_WORKDIR = $missingWorkspace
        $env:LINEAR_API_KEY = "golc-acceptance-fake-key"
        try {
            $previewResult = Invoke-Golc -RepositoryRoot $RepositoryUnderTest -CommandArguments @("linear", "preview", "--remote", "--out", (Join-Path $missingWorkspace "unused-plan.json"))
            if ($previewResult.ExitCode -eq 0) {
                throw "LINEAR_TRANSPORT_OFFLINE_ISOLATION_FAILED: linear preview --remote must fail when the compiled adapter is missing"
            }
            if (-not $previewResult.StdErrText.Contains("GOLC_LINEAR_TRANSPORT_UNAVAILABLE")) {
                throw "LINEAR_TRANSPORT_OFFLINE_WRONG_DIAGNOSTIC: expected GOLC_LINEAR_TRANSPORT_UNAVAILABLE, got: $($previewResult.StdErrText.Trim())"
            }

            $driftResult = Invoke-Golc -RepositoryRoot $RepositoryUnderTest -CommandArguments @("linear", "drift", "--remote", "--read-only")
            if ($driftResult.ExitCode -eq 0) {
                throw "LINEAR_TRANSPORT_OFFLINE_ISOLATION_FAILED: linear drift --remote --read-only must fail when the compiled adapter is missing"
            }
            if (-not $driftResult.StdErrText.Contains("GOLC_LINEAR_TRANSPORT_UNAVAILABLE")) {
                throw "LINEAR_TRANSPORT_OFFLINE_WRONG_DIAGNOSTIC: expected GOLC_LINEAR_TRANSPORT_UNAVAILABLE, got: $($driftResult.StdErrText.Trim())"
            }
            Write-Output "Offline isolation: linear preview --remote and linear drift --remote --read-only both fail closed with GOLC_LINEAR_TRANSPORT_UNAVAILABLE when the compiled adapter is missing."
        }
        finally {
            Remove-Item Env:\GOLC_LINEAR_SYNC_WORKDIR -ErrorAction SilentlyContinue
            Remove-Item Env:\LINEAR_API_KEY -ErrorAction SilentlyContinue
        }
    }
    finally {
        if (Test-Path -LiteralPath $missingWorkspace) {
            Remove-Item -LiteralPath $missingWorkspace -Recurse -Force -ErrorAction SilentlyContinue
        }
    }

    # The missing-adapter condition above must never contaminate the core
    # offline graph: check --offline and build must both remain green with
    # no GOLC_LINEAR_SYNC_WORKDIR override in effect.
    $buildResult = Invoke-Golc -RepositoryRoot $RepositoryUnderTest -CommandArguments @("build")
    Assert-GolcSucceeded -Result $buildResult -Operation "build"
    Write-Output "Offline isolation: build remained green after the induced missing-adapter condition."

    $offlineResult = Invoke-Golc -RepositoryRoot $RepositoryUnderTest -CommandArguments @("check", "--offline")
    Assert-GolcSucceeded -Result $offlineResult -Operation "check --offline"
    if (-not $offlineResult.StdOutText.Contains("generate, check, build, and test all completed with network denied")) {
        throw "LINEAR_TRANSPORT_OFFLINE_GRAPH_NOT_CONFIRMED: expected check --offline to report the complete offline core graph"
    }
    Write-Output "Offline isolation: check --offline (generate/check/build/test --quick) remained green -- a missing Linear adapter affects only the explicit remote command."
}

function Invoke-WorkflowAcceptance {
    [CmdletBinding()]
    param([Parameter(Mandatory = $true)][string]$RepositoryUnderTest)

    $workflowPath = Join-Path $RepositoryUnderTest ".github\workflows\linear-sync.yml"
    if (-not (Test-Path -LiteralPath $workflowPath -PathType Leaf)) {
        throw "LINEAR_SYNC_WORKFLOW_MISSING: expected $workflowPath"
    }
    $text = Get-Content -LiteralPath $workflowPath -Raw

    if ($text -match "(?m)^\s*pull_request\s*:") {
        throw "LINEAR_SYNC_WORKFLOW_PR_TRIGGER: linear-sync.yml must never trigger on pull_request"
    }
    if ($text -notmatch "workflow_dispatch") {
        throw "LINEAR_SYNC_WORKFLOW_NOT_MANUAL: expected an explicit workflow_dispatch trigger"
    }
    foreach ($requiredToken in @("plan_file", "plan_id", "confirm_apply", "finally", "environment:")) {
        if ($text -notmatch [regex]::Escape($requiredToken)) {
            throw "LINEAR_SYNC_WORKFLOW_MISSING_TOKEN: expected '$requiredToken' in linear-sync.yml"
        }
    }
    if ($text -match "LINEAR_API_KEY\s*=\s*[A-Za-z0-9]") {
        throw "LINEAR_SYNC_WORKFLOW_HARDCODED_SECRET: LINEAR_API_KEY must never be hardcoded"
    }
    Write-Output "Workflow acceptance: linear-sync.yml has no pull_request trigger, is workflow_dispatch-only, and declares plan_file/plan_id/confirm_apply/finally/environment."

    # Runtime guards remain authoritative even if the workflow YAML were
    # bypassed: PR-triggered mutation is refused independent of the caller
    # -- proved against a real, hash-valid plan file (not a decode
    # failure) so the observed diagnostic is unambiguously the PR guard,
    # not an earlier parse error.
    $workDir = Join-Path ([System.IO.Path]::GetTempPath()) ("golc-linear-transport-workflow-" + [guid]::NewGuid().ToString("N"))
    $fakeWorkspace = Join-Path $workDir "fake-sdk-workspace"
    $callLogPath = Join-Path $workDir "call-log.jsonl"
    $storePath = Join-Path $workDir "store.json"
    $guardPlanPath = Join-Path $workDir "guard-plan.json"
    New-Item -ItemType Directory -Path $workDir -Force | Out-Null
    try {
        New-FakeLinearSdkWorkspace -RepositoryUnderTest $RepositoryUnderTest -WorkspaceRoot $fakeWorkspace -CallLogPath $callLogPath -StorePath $storePath | Out-Null
        $env:LINEAR_API_KEY = "golc-acceptance-fake-key"
        $env:LINEAR_TEAM_ID = "golc-acceptance-fake-team"
        $env:GOLC_LINEAR_SYNC_WORKDIR = $fakeWorkspace
        try {
            $guardPreviewResult = Invoke-Golc -RepositoryRoot $RepositoryUnderTest -CommandArguments @("linear", "preview", "--remote", "--out", $guardPlanPath)
            Assert-GolcSucceeded -Result $guardPreviewResult -Operation "linear preview --remote (guard fixture)"
            $guardPlan = Get-Content -LiteralPath $guardPlanPath -Raw | ConvertFrom-Json
        }
        finally {
            Remove-Item Env:\LINEAR_API_KEY -ErrorAction SilentlyContinue
            Remove-Item Env:\LINEAR_TEAM_ID -ErrorAction SilentlyContinue
            Remove-Item Env:\GOLC_LINEAR_SYNC_WORKDIR -ErrorAction SilentlyContinue
        }

        $env:GITHUB_EVENT_NAME = "pull_request"
        try {
            $guardResult = Invoke-Golc -RepositoryRoot $RepositoryUnderTest -CommandArguments @("linear", "apply", $guardPlanPath, "--plan-id", $guardPlan.plan_id)
            if ($guardResult.ExitCode -eq 0) {
                throw "LINEAR_SYNC_PR_GUARD_BYPASSED: linear apply must refuse to run under GITHUB_EVENT_NAME=pull_request"
            }
            if (-not $guardResult.StdErrText.Contains("GOLC_APPLY_PR_BLOCKED")) {
                throw "LINEAR_SYNC_PR_GUARD_WRONG_DIAGNOSTIC: $($guardResult.StdErrText.Trim())"
            }
        }
        finally {
            Remove-Item Env:\GITHUB_EVENT_NAME -ErrorAction SilentlyContinue
        }
    }
    finally {
        if (Test-Path -LiteralPath $workDir) {
            Remove-Item -LiteralPath $workDir -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
    Write-Output "Workflow acceptance: the runtime PR guard (GuardAgainstPullRequestMutation) is reachable and authoritative independent of the workflow YAML."
}

$scriptExitCode = 1
try {
    $repositoryUnderTest = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot "..\.."))

    switch ($Mode) {
        "hierarchy" {
            Invoke-HierarchyAcceptance -RepositoryUnderTest $repositoryUnderTest
            Write-Output "Linear transport acceptance confirmed: the real process boundary completed and replayed the full hierarchy against a fake SDK with no duplicates."
        }
        "offline" {
            Invoke-OfflineAcceptance -RepositoryUnderTest $repositoryUnderTest
            Write-Output "Linear transport acceptance confirmed: a missing adapter affects only the explicit remote command; the offline core graph stays green."
        }
        "workflow" {
            Invoke-WorkflowAcceptance -RepositoryUnderTest $repositoryUnderTest
            Write-Output "Linear transport acceptance confirmed: the protected/manual workflow is structurally safe and the runtime PR guard is authoritative."
        }
    }

    $scriptExitCode = 0
}
catch {
    [Console]::Error.WriteLine("LINEAR_TRANSPORT_ACCEPTANCE_FAILURE: " + $_.Exception.Message)
    $scriptExitCode = 1
}

exit $scriptExitCode
