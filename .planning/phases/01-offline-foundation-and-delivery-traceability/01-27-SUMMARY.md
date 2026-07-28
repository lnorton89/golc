---
phase: 01-offline-foundation-and-delivery-traceability
plan: 27
subsystem: infra
tags: [nodejs, typescript, linear-sdk, redaction, canary-scan, mutation-uncertainty]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 26
    provides: "tools/linear-sync/src/errors.ts normalizeGraphQLResult/normalizeRateLimit/decideRetry/RequestContext, tools/linear-sync/src/protocol.ts TransportDiagnostic, tools/linear-sync/src/adapter.ts captureSnapshot's probeGraphQLResult/probeRateLimit preflight contract"
provides:
  - "tools/linear-sync/src/redact.ts: safeError -- the sole producer of an 'unknown' MutationOutcome's diagnostic; converts any raw create/update SDK failure into the exact allowlisted TransportDiagnostic surface, reading only a fixed classification of Error.name, never message/stack/headers/request-body/credential/client content"
  - "tools/linear-sync/src/redact.ts: scanCanary/scanCanaryAll -- byte-for-byte TypeScript-side mirror of internal/security/redact.go's ScanCanary/ScanCanaryAll/CanaryToken, proving no fake-secret byte survives normalization"
  - "tools/linear-sync/src/protocol.ts: MutationOutcome -- discriminated union ({status:'confirmed'} & MutationResult) | {status:'unknown', diagnostic}; create/update's OperationResult now maps to MutationOutcome instead of a bare, throw-on-failure MutationResult"
  - "tools/linear-sync/src/adapter.ts: createOperation/updateOperation attempt exactly one mutation call plus one mandatory readback (confirmReadback) and never throw a raw exception -- any failure from either call returns a typed unknown MutationOutcome immediately, with zero automatic retry"
  - "tools/linear-sync/test/redact.test.ts + test/mutation.test.ts: marker TestScopeLinearTransportNode -- proof of safeError/scanCanary's allowlist and the commit/timeout/readback-timeout uncertain-outcome contract against tests/fixtures/linear/mutation-uncertain.json"
  - "internal/command/linear_sync.go: MustDeclareNodeScope registration for scope 'linear-transport-node' (Command runs dist/test/redact.test.js and dist/test/mutation.test.js together)"
affects: [linear-sync]

tech-stack:
  added: []
  patterns:
    - "safeError(error, context) is the sole producer of an 'unknown' MutationOutcome's TransportDiagnostic (redact.ts): it never reads a raw exception's .message, .stack, or any attached property -- only Error.name feeds a fixed three-value classification (LINEAR_MUTATION_TIMEOUT / LINEAR_MUTATION_NETWORK_ERROR / LINEAR_MUTATION_UNCERTAIN), so a hostile or malformed raw error can never smuggle credential/header/request-body content into a diagnostic by construction."
    - "createOperation/updateOperation (adapter.ts) each make at most two remote calls total -- the mutation attempt itself and its immediately-following mandatory readback (confirmReadback) -- wrapped in try/catch that converts any exception from either call into a typed 'unknown' MutationOutcome via safeError, with no retry loop anywhere in this module. Go retains sole authority for discovering the true remote postcondition of an 'unknown' mutation (internal/trace/apply/engine.go's applyUnlinkedOperation, identity-marker discovery); this transport never re-attempts or guesses."
    - "scanCanary/scanCanaryAll/CANARY_TOKEN/forbidden-pattern list in redact.ts are a byte-for-byte TypeScript mirror of internal/security/redact.go's ScanCanary/ScanCanaryAll/CanaryToken/forbiddenPatterns, giving both languages the same offline secret-leak proof mechanism."

key-files:
  created:
    - tools/linear-sync/src/redact.ts
    - tools/linear-sync/test/redact.test.ts
    - tools/linear-sync/test/mutation.test.ts
    - tests/fixtures/linear/mutation-uncertain.json
  modified:
    - tools/linear-sync/src/protocol.ts
    - tools/linear-sync/src/adapter.ts
    - internal/command/linear_sync.go
    - config/commands.toml
    - tools/linear-sync/test/operations.test.ts

key-decisions:
  - "MutationOutcome's 'confirmed' variant is `{status: 'confirmed'} & MutationResult` (not a redesigned shape) so it still carries a bare `.record` field -- this kept Plan 01-25's existing operations.test.ts assertions (`createResult.record`, `updateResult.record`) passing unchanged at runtime; only its own `readResult` cast at line 222 needed a Rule 1 type-error fix (see Deviations)."
  - "safeError classifies failures into exactly three fixed codes by inspecting only Error.name (never .message/.stack/any attached property): AbortError/TimeoutError -> LINEAR_MUTATION_TIMEOUT, a TypeError carrying a structured (object-shaped) .cause -> LINEAR_MUTATION_NETWORK_ERROR, everything else (including non-Error throws) -> LINEAR_MUTATION_UNCERTAIN. This gives Go a coarse-but-safe signal without ever reading free-text content that could carry credentials or PII."
  - "confirmReadback (adapter.ts) replaces the old readBackOrFail: instead of throwing ProtocolDecodeError on a missing/failed readback, it now returns a typed 'unknown' MutationOutcome via safeError for both a thrown readback exception and a not-found readback -- both are equally uncertain outcomes Go must discover, never distinguished by a throw vs. a typed value."
  - "linear-transport-node's Go-side MustDeclareNodeScope registration (internal/command/linear_sync.go) and config/commands.toml prose update were added even though only config/commands.toml was in this plan's files_modified frontmatter -- the exact same Rule 3 precedent Plan 01-26 already established for linear-transport-errors: config/commands.toml's scope block is prose-only, and the real self-registration that makes `test --quick --scope linear-transport-node` resolve to a runnable Command lives in Go."

requirements-completed: [CONF-04, LINR-03, LINR-04]

coverage:
  - id: D1
    description: "createOperation/updateOperation return a typed 'unknown' MutationOutcome immediately (via safeError) the instant the mutation SDK call itself, or its immediately following mandatory readback, throws any exception -- with zero automatic retry (exactly one mutation call attempted, plus exactly one readback call when the mutation itself succeeded)"
    requirement: LINR-03
    verification:
      - kind: unit
        ref: "tools/linear-sync/test/mutation.test.ts: TestScopeLinearTransportNode (5 scenarios: create timeout, create network failure, create generic partial error, update generic partial error, update commit-then-readback-timeout), asserting outcome.status === 'unknown' and exact SDK call counts (1 or 2, never more)"
        status: pass
    human_judgment: false
  - id: D2
    description: "safeError never lets a raw create/update exception's message, stack, headers, request body, or credential reach the returned TransportDiagnostic -- only the allowlisted operation/code/endpoint fields are ever populated, and code is always one of three fixed constants"
    requirement: LINR-04
    verification:
      - kind: unit
        ref: "tools/linear-sync/test/redact.test.ts: 'safeError never leaks a raw exception's message, stack, or attached fields...' and 'safeError classifies AbortError/TimeoutError...'; tools/linear-sync/test/mutation.test.ts: TestScopeLinearTransportNode's per-scenario allowlist/scanCanary assertions against a hostile canary-laden raw error"
        status: pass
    human_judgment: false
  - id: D3
    description: "scanCanary/scanCanaryAll detect the exact CANARY_TOKEN and every forbidden secret-shaped substring (LINEAR_API_KEY=, Bearer , sk-, lin_api_), byte-for-byte matching internal/security/redact.go's Go-side contract, and report zero violations for clean input"
    requirement: CONF-04
    verification:
      - kind: unit
        ref: "tools/linear-sync/test/redact.test.ts: 'scanCanary detects the exact CANARY_TOKEN...' and 'scanCanaryAll scans every named source...'"
        status: pass
    human_judgment: false
  - id: D4
    description: "Exact Node scope linear-transport-node is registered in the authoritative command graph (config/commands.toml prose + internal/command/linear_sync.go MustDeclareNodeScope) before invocation, with matching marker TestScopeLinearTransportNode, and the registered scope exits 0"
    requirement: CONF-04
    verification:
      - kind: e2e
        ref: "powershell -NoProfile -File .\\golc.ps1 test --quick --scope linear-transport-node"
        status: pass
    human_judgment: false

duration: ~65min
completed: 2026-07-21
status: complete
---

# Phase 1 Plan 27: Redact Transport Output and Return Uncertain Writes Without Retry Summary

**`redact.ts`'s allowlisted TransportDiagnostic/canary-scan contract plus adapter.ts's typed, non-retrying create/update `MutationOutcome`, proven under a newly self-registered `linear-transport-node` Node scope against a hostile canary-laden commit/timeout fixture**

## Performance

- **Duration:** ~65min (worktree had no bootstrapped `.tools/`; the main checkout's own `.tools/` cache was mirrored via `robocopy /MIR`, then `bootstrap --include linear-sync` still ran live for Node/npm/tsc)
- **Started:** 2026-07-21T06:55:00Z (approx.)
- **Completed:** 2026-07-21T08:07:09Z
- **Tasks:** 1
- **Files modified:** 9 (4 created, 5 modified)

## Accomplishments

- `tools/linear-sync/src/redact.ts`'s `safeError(error, context)` is the sole producer of an "unknown" `MutationOutcome`'s diagnostic (CONTEXT D-20/D-21; T-01-40/T-01-41): it never reads a raw exception's `.message`, `.stack`, or any attached property (a client instance, environment map, request body, or header set) -- only `Error.name` feeds a fixed three-value classification (`LINEAR_MUTATION_TIMEOUT` for `AbortError`/`TimeoutError`, `LINEAR_MUTATION_NETWORK_ERROR` for a `TypeError` carrying a structured `.cause`, `LINEAR_MUTATION_UNCERTAIN` for everything else including non-`Error` throws), so a hostile or malformed raw error can never smuggle credential/header/request-body content into a diagnostic by construction.
- `redact.ts`'s `scanCanary`/`scanCanaryAll`/`CANARY_TOKEN` are a byte-for-byte TypeScript mirror of `internal/security/redact.go`'s `ScanCanary`/`ScanCanaryAll`/`CanaryToken`/`forbiddenPatterns` (same exact token value and forbidden substring list: `LINEAR_API_KEY=`, `Bearer `, `sk-`, `lin_api_`), giving both languages the same offline secret-leak proof mechanism.
- `tools/linear-sync/src/protocol.ts` gains `MutationOutcome = ({status: "confirmed"} & MutationResult) | {status: "unknown", diagnostic: TransportDiagnostic}`; `OperationResult<TOperation>` now maps `"create"`/`"update"` to `MutationOutcome` instead of a bare, throw-on-failure `MutationResult`.
- `tools/linear-sync/src/adapter.ts`'s `createOperation`/`updateOperation` each make at most two remote calls total -- the mutation attempt itself, then its immediately-following mandatory readback via the new `confirmReadback` helper (replacing the old throw-on-failure `readBackOrFail`) -- both wrapped in try/catch that converts any exception into a typed `"unknown"` `MutationOutcome` via `safeError`, immediately, with no retry loop anywhere in this module. A missing (not-found) readback record is treated identically to a thrown readback exception: both are equally uncertain outcomes. Go retains sole authority for discovering the true remote postcondition of an `"unknown"` mutation (`internal/trace/apply/engine.go`'s `applyUnlinkedOperation`, identity-marker discovery) -- this transport never re-attempts or guesses on its own.
- `tools/linear-sync/test/redact.test.ts` proves, entirely offline: `scanCanary`/`scanCanaryAll` detect the exact canary token and every forbidden secret-shaped substring (and report zero violations for clean input); `safeError` never leaks a hostile raw error's message/stack/attached-headers/attached-client/attached-requestBody content even when they deliberately embed the canary token, and classifies `AbortError`/`TimeoutError`/network-`TypeError`/generic-`Error`/non-`Error` throws to the correct fixed code.
- `tools/linear-sync/test/mutation.test.ts` registers `TestScopeLinearTransportNode` and proves, against `tests/fixtures/linear/mutation-uncertain.json`'s five scenarios (create-timeout, create-network-failure, create-generic-partial-error, update-generic-partial-error, and update-commit-then-readback-timeout) and a hostile fake `LinearClient` that throws a canary/credential-laden raw error: every scenario returns `MutationOutcome.status === "unknown"` with the expected diagnostic code, exactly one (or, for the commit-then-readback-timeout scenario, exactly two) SDK call(s) are ever attempted (zero automatic write retry), and the rendered outcome never contains the hostile error's canary token, forbidden substrings, or attached-field names.
- `config/commands.toml`'s Linear SDK workspace scopes prose block now documents all five registered scopes (adding `linear-transport-node`); `internal/command/linear_sync.go` registers the actual `MustDeclareNodeScope` for `"linear-transport-node"` (marker `TestScopeLinearTransportNode`, Command targeting both `dist/test/redact.test.js` and `dist/test/mutation.test.js`).
- Live end-to-end verification: `powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-transport-node` exits 0 (10 passing tests). Also re-verified as regression: `test --quick --scope linear-sdk-operations` (46 passing -- its broad glob now also picks up `redact.test.ts`/`mutation.test.ts`), `test --quick --scope linear-transport-pagination` (8 passing, unchanged), `test --quick --scope linear-transport-errors` (19 passing, unchanged), `check --offline` (generate/check/build/test all with network denied, including the "no fake-secret bytes found" concern check), and the full `golc.ps1 test` (all Go packages plus all four Node scopes) all pass cleanly. `go vet ./...` and `gofmt` report no issues on `internal/command/linear_sync.go`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Redact transport output and return uncertain writes without retry** - `9c02b34` (feat)

**Plan metadata:** committed with this summary

## Files Created/Modified

- `tools/linear-sync/src/redact.ts` - `safeError`, `scanCanary`, `scanCanaryAll`, `CANARY_TOKEN`, `CanaryViolation`.
- `tools/linear-sync/src/protocol.ts` - `MutationOutcome`; `OperationResult<TOperation>` now maps create/update to `MutationOutcome`.
- `tools/linear-sync/src/adapter.ts` - `confirmReadback` (replaces `readBackOrFail`); `createOperation`/`updateOperation` now single-attempt/typed-unknown-outcome, importing `safeError` from `./redact.js`.
- `tools/linear-sync/test/redact.test.ts` - `scanCanary`/`scanCanaryAll`/`safeError` allowlist and classification contract (4 top-level assertions).
- `tools/linear-sync/test/mutation.test.ts` - `TestScopeLinearTransportNode` fixture-driven contract (5 nested scenario subtests).
- `tests/fixtures/linear/mutation-uncertain.json` - commit/timeout unknown-outcome transcript (5 scenarios) plus a canary/credential-laden hostile message.
- `internal/command/linear_sync.go` - `MustDeclareNodeScope` registration for `"linear-transport-node"`; new `linearSyncNodeTestFilesNode` constant and `linearSyncNodeTestCommandNode` helper.
- `config/commands.toml` - extended the existing Linear SDK workspace scopes prose block to document all five registered scopes.
- `tools/linear-sync/test/operations.test.ts` - one cast fix (`readResult`) required by `MutationOutcome`'s new shape (see Deviations).

## Decisions Made

See frontmatter `key-decisions` for the full list. Highlights:

- **`MutationOutcome`'s `"confirmed"` variant intersects with `MutationResult`** (`{status: "confirmed"} & MutationResult`) rather than redesigning the success shape, so it still carries a bare `.record` field -- this kept Plan 01-25's existing `operations.test.ts` create/update assertions (`createResult.record`, `updateResult.record`) passing unchanged at runtime.
- **`safeError` classifies by `Error.name` only, never free-text content.** Three fixed codes (`LINEAR_MUTATION_TIMEOUT`/`LINEAR_MUTATION_NETWORK_ERROR`/`LINEAR_MUTATION_UNCERTAIN`) give Go a coarse-but-safe signal without ever reading `.message`/`.stack`/attached properties that could carry credentials or PII.
- **`confirmReadback` unifies "readback threw" and "readback not found"** into the same typed `"unknown"` outcome (both are equally uncertain from this transport's perspective; only Go's identity-marker discovery can resolve which actually happened).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `internal/command/linear_sync.go` needed the actual Go-side scope registration, which the plan's `files_modified` frontmatter omitted**
- **Found during:** Task 1, first `bootstrap --include linear-sync` + `test --quick --scope linear-transport-node` attempt -- the scope would resolve to nothing without a Go-side `MustDeclareNodeScope` call, exactly the same gap Plan 01-26's summary already documented and fixed for `linear-transport-errors`.
- **Issue:** The plan's acceptance criteria require "Exact scope `linear-transport-node` is registered before invocation," and the `<verify>` command directly runs `test --quick --scope linear-transport-node`. `config/commands.toml`'s scope block is prose-only; the actual self-registration lives in `internal/command/linear_sync.go`, which was not listed in this plan's `files_modified`.
- **Fix:** Added a fifth `MustDeclareNodeScope` call in `internal/command/linear_sync.go` for scope `"linear-transport-node"` (marker `TestScopeLinearTransportNode`, Command targeting both `dist/test/redact.test.js` and `dist/test/mutation.test.js` in one Node invocation), and extended `config/commands.toml`'s existing prose block to document all five registered scopes.
- **Files modified:** `internal/command/linear_sync.go`, `config/commands.toml`
- **Verification:** `powershell -NoProfile -File .\golc.ps1 bootstrap --include linear-sync` followed by `test --quick --scope linear-transport-node` exits 0 (10 passing tests). Also confirmed `test --quick --scope linear-sdk-operations`, `test --quick --scope linear-transport-pagination`, `test --quick --scope linear-transport-errors`, `check --offline`, and full `golc.ps1 test` all still pass.
- **Committed in:** `9c02b34` (Task 1 commit)

**2. [Rule 1 - Bug] `operations.test.ts` (Plan 01-25, not in this plan's `files_modified`) failed to compile after `MutationOutcome` replaced the bare `MutationResult` return shape for create/update**
- **Found during:** Task 1, first `bootstrap --include linear-sync` compile after changing `protocol.ts`/`adapter.ts`.
- **Issue:** `tsc` reported `TS2352: Conversion of type 'MutationOutcome' to type 'ReadResult' may be a mistake` at `operations.test.ts`'s existing `const readResult = (await adapter.execute(readOperation)) as ReadResult;` line -- the new discriminated `MutationOutcome` union is no longer "comparable enough" to `ReadResult` for TypeScript's `as`-cast check (the same class of `noUncheckedIndexedAccess`/`exactOptionalPropertyTypes`-strict-mode issue Plan 01-26's summary already documented and fixed twice for a different pair of casts). The two sibling casts to the narrower `MutationResult` (`createResult`, `updateResult`) still compiled cleanly and needed no change.
- **Fix:** Changed the one failing cast to `as unknown as ReadResult`, matching the exact `as unknown as X` pattern Plan 01-26 already established for this identical TS2352 class of error.
- **Files modified:** `tools/linear-sync/test/operations.test.ts`
- **Verification:** Re-ran `bootstrap --include linear-sync` (compiles cleanly) and `test --quick --scope linear-sdk-operations` (46 passing, including this file's own `TestScopeLinearSdkOperations` and `cli.ts`/`protocol.ts` supplementary tests, all unchanged in behavior).
- **Committed in:** `9c02b34` (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (1 Rule 3 blocking, 1 Rule 1 bug). Both were required for the plan's own acceptance criteria and `<verify>` command to be achievable at all -- neither is a scope expansion beyond what those already required, and both follow exact precedent Plan 01-26 already established for this same workspace.

## Issues Encountered

- This worktree had no bootstrapped `.tools/` state (fresh worktree). The main repository checkout's own `.tools/` cache (Go 1.26.5 toolchain + downloads, ~545MB, no Node/npm side) was `robocopy /MIR`'d into this worktree to skip a redundant pinned-Go-archive re-download -- safe because that cache is entirely gitignored, content-addressed, and read-only from `golc.ps1`'s perspective (same precedent Plans 01-14/01-26 already used). A fresh `bootstrap --include linear-sync` was still required and run live (Node 24.18.0 install + `npm ci` + `tsc` compile), since the source cache had no Node/npm artifacts either.
- `robocopy` needed `MSYS_NO_PATHCONV=1` in this Git-Bash environment; without it, Git Bash's automatic POSIX-to-Windows path conversion mangled the `/MIR` flag. Every `golc.ps1` invocation in this session also needed `MSYS_NO_PATHCONV=1` for the same reason.

## Known Stubs

None. `safeError`/`scanCanary`/`scanCanaryAll` are fully implemented and tested; `confirmReadback`/`createOperation`/`updateOperation`'s typed-unknown-outcome path is exercised end to end by `mutation.test.ts`'s five scenarios (including the commit-then-readback-timeout case). Real-transport wiring of a Go-side `apply.RemoteClient` implementation that spawns this Node CLI process (the "process-based Linear transport" `internal/command/linear.go` already documents as not yet existing) remains explicitly out of this plan's scope -- this plan only establishes the TypeScript-side contract that future implementation will consume.

## Threat Flags

None beyond this plan's own declared threat model (T-01-40, T-01-41, T-01-SC), which is fully mitigated as designed: `safeError`'s fixed-code-only classification and `confirmReadback`/`createOperation`/`updateOperation`'s single-attempt/no-retry discipline directly implement T-01-40's "no adapter mutation retry; Go postcondition recovery" mitigation; `scanCanary`/`scanCanaryAll` plus the allowlist-only `TransportDiagnostic` surface directly implement T-01-41's "allowlisted normalization and canary scans" mitigation. No new npm dependency was added (T-01-SC unaffected: still exactly `@linear/sdk@88.1.0` and `typescript@7.0.2`, unchanged exact lockfile). No new network, credential, or filesystem surface was introduced; `redact.ts` makes no GraphQL/SDK/HTTP call itself and reads no environment variable.

## User Setup Required

None -- everything is repository-local and offline after the one-time toolchain bootstrap already required by prior plans (verified working live in this worktree). No credentials, live Linear access, or manual configuration required to exercise any test or acceptance script in this plan.

## Next Phase Readiness

- A future Go-side `apply.RemoteClient` process adapter (spawning `tools/linear-sync/src/cli.ts`) can consume `MutationOutcome`'s `"unknown"`/`"confirmed"` discriminant directly over the existing NDJSON boundary without any further redesign of `adapter.ts`, `protocol.ts`, or `redact.ts`.
- `redact.ts`'s `safeError`/`scanCanary`/`scanCanaryAll` are available for any future producer in this workspace that needs the same allowlisted-diagnostic/canary-proof pattern (for example a future audit/logging path), matching `errors.ts`'s existing `TransportDiagnostic`-allowlist precedent.
- `mutation.test.ts`'s fixture-driven, per-scenario-subtest pattern (`loadFixture`, hostile fake SDK client, per-scenario nested subtests using the outer callback's own `TestContext`) is reusable for any future SDK-failure-shaped contract test in this workspace, matching `pagination.test.ts`/`rate-limit.test.ts`'s existing precedent.

## Self-Check: PASSED

- All four created files verified present on disk: `tools/linear-sync/src/redact.ts`, `tools/linear-sync/test/redact.test.ts`, `tools/linear-sync/test/mutation.test.ts`, `tests/fixtures/linear/mutation-uncertain.json` (all `FOUND`).
- Commit `9c02b34` (feat) verified present in `git log --oneline` on branch `worktree-agent-a4c9163aa5cdf9798`; `git diff --diff-filter=D --name-only 9c02b34~1 9c02b34` reports zero deleted files.
- The plan's exact `<verify>` command -- `powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-transport-node` -- exits 0, run live multiple times in this worktree (before and after the two auto-fixed deviations, and as part of the final regression pass): 10 passing tests.
- `test --quick --scope linear-sdk-operations` (46 passing, now including the new files via its broad glob), `test --quick --scope linear-transport-pagination` (8 passing, unchanged), `test --quick --scope linear-transport-errors` (19 passing, unchanged), `check --offline` (generate/check/build/test all with network denied), and the full `golc.ps1 test` (all Go packages + all four Node scopes) were all re-run as a final regression pass and pass cleanly. `go vet ./...` and `gofmt` report no issues.

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-21*
