---
phase: 01-offline-foundation-and-delivery-traceability
reviewed: 2026-07-21T09:22:15Z
depth: standard
files_reviewed: 79
files_reviewed_list:
  - .github/workflows/check.yml
  - .github/workflows/linear-sync.yml
  - cmd/golc-project/main.go
  - config/application-defaults.toml
  - config/commands.toml
  - config/generation.toml
  - config/integrations/linear.toml
  - config/runtime.toml
  - config/toolchain.toml
  - docs/development.md
  - internal/bootstrap/archive.go
  - internal/bootstrap/bootstrap.go
  - internal/bootstrap/bootstrap_test.go
  - internal/bootstrap/cache.go
  - internal/bootstrap/downloader.go
  - internal/command/build.go
  - internal/command/check.go
  - internal/command/config.go
  - internal/command/generate.go
  - internal/command/linear.go
  - internal/command/linear_sync.go
  - internal/command/linear_validate.go
  - internal/command/package.go
  - internal/command/router.go
  - internal/command/test.go
  - internal/command/tools.go
  - internal/contracts/generate.go
  - internal/contracts/linear.go
  - internal/contracts/linear_plan.go
  - internal/contracts/model.go
  - internal/contracts/normalize.go
  - internal/delivery/foundation.go
  - internal/delivery/graph.go
  - internal/projectconfig/decode.go
  - internal/projectconfig/load.go
  - internal/projectconfig/local.go
  - internal/projectconfig/model.go
  - internal/projectconfig/path.go
  - internal/projectconfig/registry.go
  - internal/projectconfig/resolve.go
  - internal/security/redact.go
  - internal/security/redact_test.go
  - internal/strictjson/decode.go
  - internal/trace/apply/apply_test.go
  - internal/trace/apply/engine.go
  - internal/trace/apply/guard.go
  - internal/trace/apply/journal.go
  - internal/trace/apply/model.go
  - internal/trace/catalog/id.go
  - internal/trace/catalog/migrate.go
  - internal/trace/catalog/model.go
  - internal/trace/catalog/parse.go
  - internal/trace/catalog/validate.go
  - internal/trace/reconcile/canonical.go
  - internal/trace/reconcile/diff.go
  - internal/trace/reconcile/marker.go
  - internal/trace/reconcile/model.go
  - internal/trace/transport/contract.go
  - internal/trace/transport/fake.go
  - internal/trace/transport/process.go
  - internal/trace/transport/process_test.go
  - tests/acceptance/linear-transport.ps1
  - tests/acceptance/offline.ps1
  - tools/linear-sync/src/adapter.ts
  - tools/linear-sync/src/ambient-node.d.ts
  - tools/linear-sync/src/cli.ts
  - tools/linear-sync/src/errors.ts
  - tools/linear-sync/src/pagination.ts
  - tools/linear-sync/src/protocol.ts
  - tools/linear-sync/src/redact.ts
  - tools/linear-sync/test/errors.test.ts
  - tools/linear-sync/test/mutation.test.ts
  - tools/linear-sync/test/operations.test.ts
  - tools/linear-sync/test/pagination.test.ts
  - tools/linear-sync/test/rate-limit.test.ts
  - tools/linear-sync/test/redact.test.ts
findings:
  critical: 2
  warning: 5
  info: 2
  total: 9
status: issues_found
---

# Phase 01: Code Review Report

**Reviewed:** 2026-07-21T09:22:15Z
**Depth:** standard
**Files Reviewed:** 79
**Status:** issues_found

## Summary

This phase builds a large, carefully engineered offline-first CLI (bootstrap,
config, generate/check/build/test, deterministic packaging) plus a
Linear-sync reconciliation/apply pipeline split across a Go core
(`internal/trace/{catalog,reconcile,apply,transport}`) and an isolated
TypeScript adapter (`tools/linear-sync`). The vast majority of the code is
defensive, well-tested, and internally consistent: strict JSON decoding,
canonical hashing, path-containment checks, checksum-verified bootstrap,
and a genuinely careful secret-redaction contract mirrored byte-for-byte
between Go (`internal/security`) and TypeScript (`redact.ts`).

However, tracing the actual "linear apply" call path against the much more
thorough `apply.RunApply`/`ResumePrefix`/`ReadByMarker` machinery that Go
package `apply` implements and tests turned up a serious integration gap:
the production CLI route never calls the safety machinery those other files
implement and test at length, and the one real-transport implementation of
the specific safety hook that machinery depends on is a permanent stub. The
result is a duplicate-object-creation risk under a very plausible operator
mistake (re-running `linear apply` against a plan file that has already
partially or fully succeeded, instead of re-running `linear preview`
first) — exactly the scenario the design notes and the acceptance script's
own comments say must never happen, but nothing in the shipped code
actually prevents it. A second, narrower blocker was found on the
Go\<->Node process boundary: one of the three operation kinds the adapter
handles (`read`) is not exception-safe, unlike the other two, which are
demonstrably exercised by every other code path that calls the same
underlying accessor.

## Critical Issues

### CR-01: `linear apply` bypasses `ValidatePlanFreshness` and journal-based resume, and the real transport's `ReadByMarker` is a permanent stub — replaying an already-applied plan file creates duplicate remote objects

**File:** `internal/command/linear.go:1385-1436` (`runLinearApply`), `internal/command/linear.go:856-858` (`processLinearClient.ReadByMarker`), `internal/trace/apply/engine.go:239-290` (`Apply` vs `RunApply`), `internal/trace/apply/journal.go:52-101` (`ResumePrefix`)

**Issue:**

`internal/trace/apply/engine.go` implements two entry points:

- `Apply(client, plan, mappings)` — attempts every operation in the plan,
  strictly one mutation each, no freshness check, no journal resume.
- `RunApply(client, plan, intents, mappings, snapshot, baselines, journal, lookupEnv)`
  — the documented "full exact-plan apply orchestration": it calls
  `ValidatePlanIntegrity`, then **`ValidatePlanFreshness`** (rejects a plan
  whose repository/remote state has drifted since the preview that produced
  it — CONTEXT D-18), then `GuardAgainstPullRequestMutation`, then
  **`ResumePrefix`** (loads a persisted journal and skips operations already
  confirmed achieved in a prior run), and only then attempts the remaining
  operations.

`internal/trace/apply/apply_test.go`'s `TestScopeLinearApplyResume` exercises
`RunApply` extensively — staleness rejection (`remote-stale.json`), PR-event
refusal, and resume-without-replay (`remote-partial-apply.json`) — proving
this machinery works correctly in isolation.

But `runLinearApply` in `internal/command/linear.go` — the actual handler
behind the self-registered `"linear apply"` route, the only production
caller of this package — calls the **lower-level `apply.Apply`** directly:

```go
results := apply.Apply(client, plan, migrated.RemoteMappings)
```

It never calls `apply.RunApply`, never calls `apply.LoadJournal`, and never
calls `apply.ValidatePlanFreshness`. The `.journal.json` file that
`commitApplyResults` writes after every apply is therefore write-only: no
code path anywhere in the production binary ever reads it back.

Separately, `applyUnlinkedOperation` (engine.go) is designed to protect
against exactly this gap at the per-operation level: before creating an
object, it calls `client.ReadByMarker(op.LocalID)` to discover whether a
prior, interrupted attempt already created it (so a retried create becomes a
safe no-op/update instead of a duplicate). But the one real implementation
of `RemoteClient` this repository ships,
`processLinearClient.ReadByMarker` (`internal/command/linear.go:856-858`),
is a permanent stub:

```go
func (c *processLinearClient) ReadByMarker(localID string) (apply.RemoteState, bool, error) {
	return apply.RemoteState{}, false, nil
}
```

It always reports "not found," documented in its own comment as a known
limitation (the wire protocol has no search-by-marker operation).

**Combined failure mode:** a contributor who re-runs
`golc.ps1 linear apply plan-a.json --plan-id <id>` a second time (for
example after a transient failure, a CI retry, or simply forgetting that a
plan file is single-use) — instead of first re-running `linear preview` —
will have every operation whose `Operation.LinearUUID` was `null` at
preview time (i.e., every entity that did not yet exist when the plan was
built) re-attempted as `applyUnlinkedOperation`. Because `op.LinearUUID` is
baked into the plan file and is never re-resolved against the now-updated
`.planning/linear-map.json`, and because `ReadByMarker` always reports
"not found," `client.Create(op)` is invoked again — creating a **second,
duplicate Linear object** for every entity the first run already
successfully created. Nothing rejects the stale plan (no freshness check
runs) and nothing detects the prior success (no marker discovery, no
journal). `tests/acceptance/linear-transport.ps1`'s own hierarchy scenario
explicitly avoids this exact case by design — its inline comment reads:

> "re-preview (not a blind re-apply of the stale plan -- ... a stale plan's
> already-achieved operations would still show as unlinked and would be
> re-attempted as creates; a fresh preview instead observes the entities
> already committed to .planning/linear-map.json as now-linked...)"

— i.e. the test authors were aware of this exact failure mode and worked
around it in the acceptance script rather than closing it in production
code. No test in this repository exercises "apply the same already-applied
plan file twice in a row without an intervening preview."

**Fix:**

```go
// runLinearApply, after decodeAndValidatePlanStrict/plan-id check:
migrated, err := catalog.MigrateV1ToV2(request.Root)
...
snapshotter, ok := client.(interface {
    CaptureSnapshot() (transport.Snapshot, error)
})
if !ok {
    return Result{ExitCode: 1, Stderr: []byte("GOLC_LINEAR_TRANSPORT_UNAVAILABLE: transport cannot capture a fresh snapshot\n")}
}
snapshot, err := snapshotter.CaptureSnapshot()
...
journalPath := resolvedPlanFile + ".journal.json"
journal, err := apply.LoadJournal(journalPath)
...
report, newJournal, err := apply.RunApply(
    client, plan, intentsFromMigratedMap(migrated), migrated.RemoteMappings,
    snapshot, nil /* baselines */, journal, os.LookupEnv,
)
```

and either implement a real `ReadByMarker` for `processLinearClient` (a
description-search/list operation would need to be added to
`tools/linear-sync/src/protocol.ts`'s `Operation` contract first), or, at
minimum, document loudly in the CLI's own usage/help text that a plan file
is strictly single-use and must never be re-applied without a fresh
`linear preview` — today nothing enforces or even warns about this at the
Go layer.

---

### CR-02: `adapter.ts`'s `readOperation` has no error handling, unlike every other SDK call site — an SDK read failure (e.g. reading an archived/deleted Linear object) crashes the whole NDJSON session, not just that one operation

**File:** `tools/linear-sync/src/adapter.ts:198-204`

**Issue:**

Every other place this file calls the Linear SDK and could observe an
exception is wrapped in `try`/`catch` and converted into a safe, typed
outcome: `createOperation` (198-284), `updateOperation` (294-322), and
`confirmReadback` (215-226, which calls the exact same `readByEntity`
helper) all catch and convert. `readOperation` — the handler for every
plain `"read"` action — does not:

```ts
async function readOperation(client: LinearClient, entity: EntityKind, linearUUID: string): Promise<ReadResult> {
  const handle = await readByEntity(client, entity, linearUUID);
  if (!handle) {
    return { found: false };
  }
  return { found: true, record: normalize(entity, handle) };
}
```

If the underlying `client.project(id)`/`client.projectMilestone(id)`/
`client.issue(id)` call throws for a missing/archived/deleted remote object
(a documented behavior of GraphQL single-entity accessor queries, and the
exact reason `confirmReadback`'s otherwise-identical call is wrapped), the
exception propagates uncaught out of `LinearSdkAdapter.execute` into
`cli.ts`'s `handleLine`, which is itself never wrapped in a `try`/`catch`
either (`src/cli.ts:99-111`, `await handleLine(line)`). This rejects the
`runCLI()` promise and permanently stops the `for await (const chunk of
input)` loop — the Node process silently stops reading any further stdin
lines for the remainder of its lifetime.

On the Go side, `internal/trace/transport/process.go`'s `ProcessClient` is a
single long-lived process reused across many sequential `Call()`s within one
CLI invocation (`processLinearClient` in `internal/command/linear.go` wraps
exactly one `ProcessClient`, used across the whole `CaptureSnapshot` loop
and the whole `Apply` run). Once the Node reader loop has stopped, every
subsequent `Call()` on that same client will hang until the per-call
deadline (`GOLC_LINEAR_SYNC_TIMEOUT_MS`, defaulting to 30s) fires and
returns `GOLC_TRANSPORT_TIMEOUT` — turning one archived/deleted Linear
issue into repeated 30-second timeouts (or an immediate
`GOLC_TRANSPORT_PROCESS_EXITED` if Node does exit) for every remaining
operation in that run, rather than the intended graceful
`{found: false}` outcome `ReadResult` exists to express.

This gap is untested: `tools/linear-sync/test/operations.test.ts`'s
`FakeLinearClient.project/projectMilestone/issue` always resolve to
`undefined` for a miss (never throw), so the exception path this finding
describes is never exercised anywhere in the test suite, unlike the
mutation-failure paths, which `mutation.test.ts`'s `HostileLinearClient`
specifically drives with a throwing `issue()` (via `readbackFails`) — but
only through `confirmReadback`, never through plain `readOperation`.

**Fix:**

```ts
async function readOperation(client: LinearClient, entity: EntityKind, linearUUID: string): Promise<ReadResult> {
  let handle: LinearEntityHandle | undefined;
  try {
    handle = await readByEntity(client, entity, linearUUID);
  } catch {
    return { found: false };
  }
  if (!handle) {
    return { found: false };
  }
  return { found: true, record: normalize(entity, handle) };
}
```

(A "not found" and "SDK threw because it's not found" are indistinguishable
to this caller by design elsewhere in the file — `confirmReadback` already
treats both identically.) Add a fixture/test case mirroring
`mutation.test.ts`'s `HostileLinearClient` but for a plain read, and add a
Go-side acceptance scenario in `tests/acceptance/linear-transport.ps1` that
deletes/archives one already-linked local ID's remote object out from under
`CaptureSnapshot` and asserts the run still completes.

## Warnings

### WR-01: `errors.ts`'s `decideRetry`/bounded-read-retry policy is fully implemented and unit-tested but never invoked by any production code path

**File:** `tools/linear-sync/src/errors.ts:224-287`, `tools/linear-sync/src/adapter.ts` (no import of `decideRetry`)

**Issue:** `decideRetry` and `DEFAULT_MAX_RETRY_ATTEMPTS` implement — and
`rate-limit.test.ts` thoroughly tests — "a `read` observing a
`server_error` may retry within bounds; a mutation observing `partial`/
`rate_limited` returns `stop_write` immediately." Neither `adapter.ts` nor
`cli.ts` ever imports or calls `decideRetry`. `readOperation`/
`createOperation`/`updateOperation` each make exactly one SDK attempt with
no retry logic of any kind — there is no code path today where a transient
5xx read failure is ever retried, contradicting the extensive doc comments
in `errors.ts` describing this as "the one retry policy this workspace ever
applies."

**Fix:** Either wire `decideRetry` into `readOperation` (loop up to
`DEFAULT_MAX_RETRY_ATTEMPTS` on a classified `server_error`), or remove/
relabel the doc comments and exported symbols so they no longer claim a
retry behavior the shipped adapter does not exhibit.

### WR-02: `apply.CommitResultAtomically` is not atomic across its three output files

**File:** `internal/trace/apply/journal.go:124-177`

**Issue:** The function stages and renames `mapPath`, then `journalPath`,
then `reportPath`, in that order. If the `mapPath` rename succeeds but the
`journalPath` rename then fails (disk full, permissions changed mid-run,
etc.), the function returns an error, but `.planning/linear-map.json` has
already been overwritten to mark entities "linked" — while the journal that
would let a subsequent apply recognize those operations as already achieved
was never written. The doc comment describes this as "persists ... as one
validated result," which is true per-file (temp + rename) but not across
the group. In practice the system likely self-heals on the next run (the
now-linked mapping routes those operations through
`applyLinkedOperation`'s own idempotency check instead), but the partial
state is real and worth either fixing (rename in reverse-dependency order,
or write a single combined temp marker first) or documenting explicitly.

### WR-03: `RunOffline` mutates process-wide global state (`os.Setenv`/`os.Unsetenv`, `http.DefaultTransport`) with no synchronization

**File:** `internal/delivery/graph.go:304-360`

**Issue:** `RunOffline` installs environment variables and replaces
`http.DefaultTransport` for the duration of a run, restoring both
afterward. This is safe today because nothing in this codebase calls it
from more than one goroutine and no test uses `t.Parallel()` around it, but
it is a latent footgun: any future caller that runs `check --offline` (or a
test that exercises it) concurrently with anything else touching
`http.DefaultTransport` or the mutated environment variables (including
another `RunOffline` invocation) will race. Consider threading an explicit
`*http.Client`/transport value through the offline graph's executors
instead of mutating the package-level default, and consider a
process-lifetime mutex guarding `RunOffline` if concurrent invocation is
ever a realistic possibility.

### WR-04: `protocol.ts`'s `decodeOperation` does not validate the required per-entity field shape for `create`/`update`, weakening the "unknown shapes fail strict decoding" contract the package doc claims

**File:** `tools/linear-sync/src/protocol.ts:363-377`

**Issue:** For `action === "create"`/`"update"`, `decodeOperation` only
checks that `value.fields` `isRecord(value.fields)` — an empty object
`{}` or an object missing `name`/`title`/`teamId` etc. passes strict
decoding and is cast straight through (`as unknown as Operation`). The
package doc comment states "an operation that does not match one of these
exact shapes fails strict decoding (`decodeOperation` below) rather than
silently reaching a best-effort GraphQL call" — but a malformed `fields`
object *does* reach the SDK call today, only to fail later (if at all) with
whatever error shape the SDK itself produces, at which point it flows
through `createOperation`/`updateOperation`'s catch-and-classify path as a
generic `LINEAR_MUTATION_UNCERTAIN`, discarding the actual "which field was
missing" diagnostic entirely.

**Fix:** Add minimal required-field checks per `(entity, action)` pair in
`decodeOperation` (e.g. `ProjectFields.name`/`teamIds`,
`IssueFields.title`/`teamId`), matching the same fail-closed discipline
already applied to `entity`/`action`/`linearUUID`.

### WR-05: `processLinearClient.ReadByMarker` being a permanent stub (see CR-01) also means every `linear apply` in production always takes the "create" branch for any not-yet-linked entity, never the "discover and adopt" branch documented at length in `engine.go`

**File:** `internal/command/linear.go:844-858`, `internal/trace/apply/engine.go:129-170`

**Issue:** This is the root cause shared with CR-01, called out separately
because it degrades a *first*-run guarantee too, not only the replay
scenario: `engine.go`'s `applyUnlinkedOperation` doc comment states "a
create that actually succeeded on a prior, interrupted attempt is found and
treated as achieved instead of retried into a duplicate" — this is one of
the headline reliability claims of the D-17/D-21 apply design. Against the
real transport, that discovery path can never fire (it's hard-coded to
`found: false`), so the *only* thing standing between "single interrupted
create" and "duplicate object on retry" for a first-time apply is whatever
external process re-runs `linear preview` before retrying — a manual
discipline, not an enforced one. Already partially disclosed in code
comments as a known limitation deferred to a future plan; flagged here so
it is visible in this phase's review rather than only in a plan-level
"Known Stubs" note.

## Info

### IN-01: `internal/bootstrap` extracts every archive entry with a fixed `0o755` mode regardless of the archive's own recorded permissions

**File:** `internal/bootstrap/bootstrap.go:233`, `internal/bootstrap/archive.go` (`extractEntry`, shared by both `InstallStaged` and `ExtractVerified`)

**Issue:** `extractEntry` always opens the destination file with
`os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755`, discarding whatever mode bits
the zip entry itself recorded. Harmless on the Windows-only Phase 1 target
(file mode bits are largely cosmetic there), but worth tightening before
any future cross-platform bootstrap work, since a non-executable config
file being extracted world-executable is a minor unnecessary permission
widening.

### IN-02: `forbiddenPatterns`/`FORBIDDEN_PATTERNS`'s `"sk-"` entry is a very broad substring match

**File:** `internal/security/redact.go:37-43`, `tools/linear-sync/src/redact.ts:42`

**Issue:** `"sk-"` as a bare substring will also flag legitimate,
non-secret text that happens to contain that sequence (for example a
hyphenated word ending in "sk" followed by any word starting with a
different letter is unlikely, but a UUID or identifier fragment containing
`sk-` is plausible). Not a security problem (it only ever makes redaction
*more* aggressive, never less), but could produce confusing false-positive
`<redacted>` output or false canary-scan failures in `check --concern
project` / `check --command-parity` if such a byte sequence ever appears in
committed, legitimately-safe generated content. Consider anchoring the
pattern more precisely (e.g. requiring a following run of the SDK's actual
key alphabet) if this ever causes a false-positive CI failure.

---

_Reviewed: 2026-07-21T09:22:15Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
