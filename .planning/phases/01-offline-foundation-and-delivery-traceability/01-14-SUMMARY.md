---
phase: 01-offline-foundation-and-delivery-traceability
plan: 14
subsystem: infra
tags: [nodejs, typescript, linear-sdk, pagination, relay, graphql, cursor-safety]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 13
    provides: tools/linear-sync npm workspace (protocol.ts/adapter.ts, exact-lock npm ci, pinned TypeScript compile, self-registered linear-sdk/linear-sdk-operations scopes)
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 25
    provides: cli.ts NDJSON process boundary, operations.test.ts fake-SDK hierarchy contract precedent, tsconfig rootDir "." dist/src+dist/test compile layout
provides:
  - "tools/linear-sync/src/pagination.ts: fetchAllPages -- exhausts a Relay connection to completion, tracking cursor history and failing closed (complete=false) on a null, immediately-repeated, or indirectly-looped cursor, or a defensive page ceiling"
  - "tools/linear-sync/src/adapter.ts: captureSnapshot/ConnectionQuery -- routes every connection read through fetchAllPages, marking the whole snapshot complete only once every intended connection is itself exhausted, discarding all records (even from already-completed connections) the instant any one connection anomalizes"
  - "tools/linear-sync/src/protocol.ts: Snapshot/SnapshotStatus -- transport-neutral capture outcome matching internal/trace/transport.Snapshot (Go) field names exactly; records stay empty whenever status is not \"complete\""
  - "tools/linear-sync/test/pagination.test.ts: marker TestScopeLinearTransportPagination -- fixture-driven proof of 51-object two-page exhaustion (with a page-two-only and a distinct page-one identity marker both preserved) plus three cursor-anomaly scenarios, entirely offline"
  - "tests/fixtures/linear/paginated-51.json + tests/fixtures/linear/cursor-loop.json: golden fixtures for exhaustive pagination and cursor-anomaly detection"
  - "internal/command/linear_sync.go: MustDeclareNodeScope registration for scope \"linear-transport-pagination\" (Command targets dist/test/pagination.test.js specifically, not the broad dist/test/**/*.test.js glob)"
affects: [01-26, 01-27, 01-15, linear-sync]

tech-stack:
  added: []
  patterns:
    - "fetchAllPages(fetchPage, maxPages?) is a pure, SDK-agnostic Relay pagination walker: it takes an injected async page-fetcher (real @linear/sdk connection, or a fixture-driven fake) and returns a discriminated PaginationResult (complete:true|false with a named PaginationCode), never throwing on an anomaly and never looping unboundedly -- three distinct anomaly codes (CURSOR_ANOMALY_NULL, _REPEATED, _LOOP) plus a MAX_PAGES_EXCEEDED safety stop."
    - "captureSnapshot's fail-closed aggregation: it accumulates normalized records from each ConnectionQuery in sequence but returns records:[] the moment any one connection is incomplete, discarding already-read good-connection data too -- an all-or-nothing snapshot completeness contract mirroring Go's SnapshotComplete-only-feeds-preview rule (CONTEXT D-21)."
    - "Node's node:test TestContext reentrancy hazard: awaiting t.test(...) from inside a callback that is itself an in-flight t.test(...) call on the SAME TestContext object deadlocks the whole test runner (0% CPU, no error, no timeout signal) instead of throwing -- every nested subtest loop must capture and use its own callback's TestContext parameter (e.g. async (t2) => { ... t2.test(...) ...}), never close over the outer t. This is a general node:test pitfall, not GOLC-specific, and worth carrying into any future nested-subtest test file in this workspace."
    - "Local .tools/ toolchain cache (Go 1.26.5 + downloads) is safe to robocopy /MIR between sibling worktrees of the same repository to skip a redundant pinned-archive re-download -- it is entirely gitignored, content-addressed, and read-only from golc.ps1's perspective; a fresh `bootstrap --include linear-sync` is still required per worktree to install/compile the Node/npm side (that cache was not present in the source worktree either)."

key-files:
  created:
    - tools/linear-sync/src/pagination.ts
    - tools/linear-sync/test/pagination.test.ts
    - tests/fixtures/linear/paginated-51.json
    - tests/fixtures/linear/cursor-loop.json
  modified:
    - tools/linear-sync/src/adapter.ts
    - tools/linear-sync/src/protocol.ts
    - internal/command/linear_sync.go
    - config/commands.toml

key-decisions:
  - "adapter.ts gains captureSnapshot(connections)/ConnectionQuery as a new exported capability (not wired into cli.ts's NDJSON boundary yet) -- protocol.ts's own docstring boundary keeps identity discovery/reconciliation policy in Go (internal/trace/reconcile); captureSnapshot only exhausts and normalizes, matching internal/trace/transport.Transport.CaptureSnapshot's exact contract shape so a later Node process-boundary plan (01-27) can wire it through cli.ts without a redesign."
  - "protocol.ts's SnapshotStatus declares the full Go-matching six-value vocabulary (complete/incomplete/partial/cursor_anomaly/ambiguous/rate_limited) now, even though this plan's captureSnapshot only ever produces \"complete\" or \"cursor_anomaly\" -- matches this file's existing precedent of declaring exhaustive vocabularies before every producer exists (e.g. OperationAction already included \"create\"/\"update\" long before every call site used them), and Plan 01-26 (errors.ts) is expected to be the next producer for \"partial\"/\"rate_limited\" without needing to touch this type again."
  - "internal/command/linear_sync.go (not listed in the plan's files_modified frontmatter) was modified to add the actual MustDeclareNodeScope registration for \"linear-transport-pagination\" -- config/commands.toml's own established pattern in this file is prose documentation only (its comment explicitly says the scope pair is 'not as machine-readable keys here'); the real self-registration that makes `test --quick --scope linear-transport-pagination` resolve to a Command lives in Go, exactly where the existing \"linear-sdk-operations\" scope is registered. Without this the plan's own <verify> command could never pass. See Deviations."
  - "The new scope's registered Command targets dist/test/pagination.test.js specifically (a new linearSyncNodeTestFilePagination constant), not the shared dist/test/**/*.test.js glob \"linear-sdk-operations\" uses -- so a scoped `test --quick --scope linear-transport-pagination` run exercises exactly TestScopeLinearTransportPagination and nothing else, while the full \"linear-sdk-operations\" glob still picks up pagination.test.ts too (both scopes pass; this was verified live)."

requirements-completed: [CONF-04, LINR-03, LINR-04]

coverage:
  - id: D1
    description: "fetchAllPages exhausts a 51-object two-page Relay connection, records page counts, and reaches the exact page-two-only GOLC identity footer (plus a distinct page-one footer, proving neither marker is lost or collapsed)"
    requirement: LINR-03
    verification:
      - kind: unit
        ref: "tools/linear-sync/test/pagination.test.ts: 'fetchAllPages exhausts both pages of 51 objects and finds the exact page-two footer'"
        status: pass
    human_judgment: false
  - id: D2
    description: "Repeated, null, and indirectly-looped cursors all produce fetchAllPages complete=false with a distinct named PaginationCode, never looping unboundedly or throwing"
    requirement: LINR-04
    verification:
      - kind: unit
        ref: "tools/linear-sync/test/pagination.test.ts: 'cursor anomalies (repeated, null, indirect loop) produce complete=false and block identity decisions' (3 nested scenario subtests)"
        status: pass
    human_judgment: false
  - id: D3
    description: "captureSnapshot marks the whole snapshot complete only once every intended connection is exhausted, and blocks the entire snapshot (discarding already-read good-connection records) when any one connection has a cursor anomaly"
    requirement: LINR-04
    verification:
      - kind: unit
        ref: "tools/linear-sync/test/pagination.test.ts: 'captureSnapshot marks the whole snapshot complete only once every intended connection is exhausted' and 'captureSnapshot blocks the entire snapshot when any one intended connection has a cursor anomaly'"
        status: pass
    human_judgment: false
  - id: D4
    description: "Exact Node scope linear-transport-pagination is registered in the authoritative command graph (config/commands.toml prose + internal/command/linear_sync.go MustDeclareNodeScope) before invocation, with matching marker TestScopeLinearTransportPagination, and the registered scope exits 0"
    requirement: CONF-04
    verification:
      - kind: e2e
        ref: "powershell -NoProfile -File .\\golc.ps1 test --quick --scope linear-transport-pagination"
        status: pass
    human_judgment: false

duration: ~2h
completed: 2026-07-21
status: complete
---

# Phase 1 Plan 14: Exhaust Every Linear Page and Block Incomplete Identity Discovery Summary

**A cursor-safe `fetchAllPages` Relay-pagination walker plus `captureSnapshot`'s all-or-nothing multi-connection completeness contract, proven against a 51-object two-page fixture and three cursor-anomaly fixtures under a newly self-registered `linear-transport-pagination` Node scope**

## Performance

- **Duration:** ~2h (including a ~35min from-scratch worktree Node/Go toolchain bootstrap and diagnosing a node:test reentrancy deadlock)
- **Started:** 2026-07-21T05:25:00Z (approx.)
- **Completed:** 2026-07-21T07:31:04Z
- **Tasks:** 1
- **Files modified:** 8 (4 created, 4 modified)

## Accomplishments

- `tools/linear-sync/src/pagination.ts`'s `fetchAllPages<TNode>(fetchPage, maxPages?)` walks an injected Relay-style page fetcher from `cursor=null` until `hasNextPage=false`, returning every node across every page. It fails closed (`complete: false` with a named `PaginationCode`, never a thrown exception or an unbounded loop) the instant a page reports `hasNextPage=true` with a null/empty `endCursor`, repeats the cursor just requested, or returns a cursor already seen earlier in the same walk — plus a defensive 1000-page ceiling (`LINEAR_PAGINATION_MAX_PAGES_EXCEEDED`) against a hostile or buggy transport that never terminates.
- `tools/linear-sync/src/adapter.ts` gained `ConnectionQuery`/`captureSnapshot`: `captureSnapshot` exhausts every given connection through `fetchAllPages` and normalizes every node, marking the whole `Snapshot` `"complete"` only once **every** intended connection has itself finished — the first connection with a cursor anomaly blocks the entire snapshot and discards every record already read (including from connections that already completed), so a hidden later page can never spoof absence or uniqueness for an identity decision. Multiple records carrying distinct GOLC identity markers are preserved individually, never deduplicated here (that decision stays in Go's `internal/trace/reconcile`). `LinearEntityHandle` is now exported so this connection-read path and its tests share the exact same normalization shape a single-entity read already produces.
- `tools/linear-sync/src/protocol.ts` gained `Snapshot`/`SnapshotStatus`, matching `internal/trace/transport.Snapshot` (Go) field names exactly (`status`, `reason?`, `records`) — the full six-value Go-matching status vocabulary is declared now even though this plan's `captureSnapshot` only ever produces `"complete"` or `"cursor_anomaly"`; `"partial"`/`"rate_limited"` are Plan 01-26's to produce against this same shape.
- `tools/linear-sync/test/pagination.test.ts` registers `TestScopeLinearTransportPagination` and proves, entirely offline against fixture-driven fake page fetchers: (1) `fetchAllPages` exhausts both pages of a 51-object fixture, reaching the page-two-only footer and a distinct page-one footer; (2) three cursor-anomaly scenarios (`repeated_cursor`, `null_cursor`, `indirect_loop`) each produce the exact expected `PaginationCode`; (3) `captureSnapshot` returns `"complete"` with all 51 normalized records (both markers preserved) for one fully-exhausted connection; (4) `captureSnapshot` returns `"cursor_anomaly"` with zero records when a second connection in the same call anomalizes, even though the first connection in that same call fully completed.
- `tests/fixtures/linear/paginated-51.json` (schemaVersion 1, page size 50, one node on page one and one node on page two each carrying a distinct GOLC identity footer) and `tests/fixtures/linear/cursor-loop.json` (schemaVersion 1, three named cursor-anomaly scenarios) are the golden fixtures this test authors against.
- `internal/command/linear_sync.go` gained the actual `MustDeclareNodeScope` registration for scope `"linear-transport-pagination"` (Command: `node --test dist/test/pagination.test.js`, targeting only the pagination test file, distinct from `"linear-sdk-operations"`'s broader glob), and `config/commands.toml`'s existing prose-documentation block was extended to list all three now-registered Linear SDK workspace scopes.
- Live end-to-end verification: `powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-transport-pagination` exits 0 (8 passing subtests); `test --quick --scope linear-sdk-operations` (17 passing, including the new file via its broad glob), `test --quick` (Go vet), `check --offline`, `generate --check`, and full `golc.ps1 test` (all Go packages + both Node scopes) were all re-run as regression gates and pass cleanly.

## Task Commits

Each task was committed atomically:

1. **Task 1: Exhaust every page and block incomplete identity discovery** - `0d69631` (feat)

**Plan metadata:** committed with this summary

## Files Created/Modified

- `tools/linear-sync/src/pagination.ts` - `fetchAllPages`, `PageInfo`, `ConnectionPage`, `FetchPage`, `PaginationResult`/`PaginationComplete`/`PaginationIncomplete`, `PaginationCode`, `DEFAULT_MAX_PAGES`.
- `tools/linear-sync/src/adapter.ts` - `ConnectionQuery`, `captureSnapshot`; exported `LinearEntityHandle`.
- `tools/linear-sync/src/protocol.ts` - `SnapshotStatus`, `Snapshot`.
- `tools/linear-sync/test/pagination.test.ts` - `TestScopeLinearTransportPagination` fixture-driven contract (4 top-level assertions, 3 nested cursor-anomaly scenarios).
- `tests/fixtures/linear/paginated-51.json` - 51-object two-page fixture with two distinct GOLC identity markers.
- `tests/fixtures/linear/cursor-loop.json` - repeated/null/indirect-loop cursor anomaly scenarios.
- `internal/command/linear_sync.go` - `MustDeclareNodeScope` registration for `"linear-transport-pagination"`; new `linearSyncNodeTestFilePagination` constant and `linearSyncNodeTestCommandPagination`/`resolveLinearSyncProjectRoot` helpers (the latter factored out of the pre-existing `linearSyncNodeTestCommand` to avoid duplicating the `GOLC_PROJECT_ROOT` resolution logic).
- `config/commands.toml` - extended the existing Linear SDK workspace scopes prose block to document all three registered scopes.

## Decisions Made

See frontmatter `key-decisions` for the full list. Highlights:

- **`captureSnapshot` is a new adapter.ts capability, not wired into `cli.ts`'s NDJSON boundary yet.** It matches `internal/trace/transport.Transport.CaptureSnapshot`'s exact contract shape (`Snapshot`/`SnapshotStatus` field names mirror Go's `internal/trace/transport.Snapshot` byte-for-byte) so a later Node process-boundary plan can wire it through `cli.ts` without a redesign — this plan's scope is the transport-vocabulary and pagination-safety layer, not the process boundary.
- **`internal/command/linear_sync.go` was modified despite not being in the plan's `files_modified` frontmatter** — a Rule 3 (blocking) fix. `config/commands.toml`'s existing scope-documentation block is explicitly prose-only (its own comment states the scope pair is "not as machine-readable keys here"); the actual self-registration that resolves `test --quick --scope linear-transport-pagination` to a runnable Command lives in Go via `MustDeclareNodeScope`, exactly matching the precedent the pre-existing `"linear-sdk-operations"` scope already establishes in this same file. Without this change the plan's own `<verify>` command could never pass — this is not a scope expansion, it is the mechanism required to satisfy the plan's own acceptance criteria ("`config/commands.toml` registers exact Node scope `linear-transport-pagination` through the Plan 17 registry contract before invocation").
- **The new scope's Command targets `dist/test/pagination.test.js` directly**, not the shared `dist/test/**/*.test.js` glob `"linear-sdk-operations"` uses, so a scoped run of `linear-transport-pagination` exercises exactly `TestScopeLinearTransportPagination` and nothing else. Both scopes were verified to pass independently and together (the broad glob still discovers `pagination.test.ts` too).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `internal/command/linear_sync.go` needed the actual Go-side scope registration, which the plan's `files_modified` frontmatter omitted**
- **Found during:** Task 1, reading `internal/command/test.go`'s registered-scope dispatcher per `<read_first>` and comparing against `config/commands.toml`'s existing prose-only scope documentation pattern.
- **Issue:** The plan's acceptance criteria require `config/commands.toml` to register scope `linear-transport-pagination` "through the Plan 17 registry contract before invocation," and the `<verify>` command directly runs `test --quick --scope linear-transport-pagination`. But `config/commands.toml`'s existing scope block is prose documentation only (verified against its own comment); the actual `MustDeclareNodeScope` self-registration that makes a scope name resolve to a runnable Command lives in `internal/command/linear_sync.go`, which was not listed in this plan's `files_modified`. Without touching it, the scope literally does not exist and the plan's own verify command fails with `GOLC_TEST_SCOPE_NO_MARKERS`.
- **Fix:** Added a second `MustDeclareNodeScope` call in `internal/command/linear_sync.go` for scope `"linear-transport-pagination"` (marker `TestScopeLinearTransportPagination`, Command targeting `dist/test/pagination.test.js`), factored the shared `GOLC_PROJECT_ROOT` resolution out of the pre-existing `linearSyncNodeTestCommand` into a new `resolveLinearSyncProjectRoot` helper reused by both scopes, and extended `config/commands.toml`'s existing prose block to document all three scopes.
- **Files modified:** `internal/command/linear_sync.go`, `config/commands.toml`
- **Verification:** `powershell -NoProfile -File .\golc.ps1 bootstrap --include linear-sync` (rebuilds `golc-project.exe` from the changed Go source) followed by `test --quick --scope linear-transport-pagination` exits 0 (8 passing subtests). Also confirmed `test --quick --scope linear-sdk-operations`, `test --quick`, `check --offline`, `generate --check`, and full `golc.ps1 test` all still pass.
- **Committed in:** `0d69631` (Task 1 commit)

**2. [Rule 1 - Bug] `node:test` TestContext reentrancy deadlock in the original cursor-anomaly nested-subtest loop**
- **Found during:** Task 1, first live run of `test --quick --scope linear-transport-pagination` — the process hung indefinitely at ~0% CPU with no error or timeout after the first subtest passed.
- **Issue:** The initial draft of `pagination.test.ts`'s cursor-anomaly subtest used `await t.test("cursor anomalies...", async () => { ... for (scenario) { await t.test(\`scenario: ${scenario.name}\`, ...) } })` — the inner `t.test(...)` call closed over the *outer* `test(...)` callback's `t` parameter rather than capturing its own callback's `TestContext`. Awaiting `t.test(...)` from inside a callback that is itself an in-flight `t.test(...)` call on the exact same `TestContext` object deadlocks Node's test runner (confirmed by isolating `fetchAllPages` itself in a plain standalone script — it returns correctly and near-instantly for all three scenarios — and then reproducing the deadlock in a minimal `node:test` file with the same reentrant `t` pattern; capturing the callback's own context parameter, e.g. `async (t2) => { ... t2.test(...) }`, fixed it immediately).
- **Fix:** Changed the cursor-anomaly subtest's callback signature to capture its own `TestContext` (`async (scenarioContext) => { ... }`) and issue every nested scenario subtest against `scenarioContext.test(...)` instead of the outer `t`. Added an explanatory code comment documenting the hazard for future nested-subtest test files in this workspace.
- **Files modified:** `tools/linear-sync/test/pagination.test.ts`
- **Verification:** `test --quick --scope linear-transport-pagination` now exits 0 in ~160ms (8 passing subtests, no hang); re-run three times to confirm no flakiness.
- **Committed in:** `0d69631` (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (1 Rule 3 blocking, 1 Rule 1 bug). Both were required for the plan's own acceptance criteria and `<verify>` command to be achievable at all — neither is a scope expansion beyond what those already required.

## Issues Encountered

- This worktree had no bootstrapped `.tools/` state (fresh worktree). The main repository checkout's own `.tools/` cache (Go 1.26.5 toolchain + downloads, ~545MB, no Node/npm side) was `robocopy /MIR`'d into this worktree to skip a redundant pinned-Go-archive re-download — safe because that cache is entirely gitignored, content-addressed, and read-only from `golc.ps1`'s perspective. A fresh `bootstrap --include linear-sync` was still required and run live (Node 24.18.0 install + `npm ci` + `tsc` compile), since the source cache had no Node/npm artifacts either.
- The `internal/command/linear_sync.go` Go-side registration required rebuilding `golc-project.exe` via `golc.ps1 bootstrap --include linear-sync` (the compiled CLI binary is only rebuilt during bootstrap, not on every `golc.ps1` invocation) before the new scope name would resolve at all — this is expected/documented dispatcher behavior, not a bug, but is worth noting for anyone editing `internal/command/*.go` mid-plan: a bare `golc.ps1 build`/`golc.ps1 test` does not pick up Go source changes to the CLI itself.
- The `node:test` reentrancy deadlock (Deviation 2) took real diagnostic effort to isolate: process CPU stayed near 0% with the process fully "Responding" per `Get-Process`, no error/timeout surfaced, and the hang only reproduced through the actual `node --test <file>` invocation (a bare standalone script calling the same `fetchAllPages` logic directly, and a from-scratch minimal 3-level-nested `node:test` file using correctly-captured contexts, both completed instantly) — narrowing it to the specific reentrant-`t` pattern required a targeted minimal repro with console.log tracing.

## Known Stubs

None. `captureSnapshot` is fully implemented and tested but intentionally not yet wired into `cli.ts`'s NDJSON process boundary — that wiring is explicitly deferred to the Node process-boundary plan (01-27) per this plan's own scope (`protocol.ts`/`adapter.ts`/`pagination.ts` only; `cli.ts` is not in `files_modified`), matching the "Next Phase Readiness" pattern the prior plan (01-25) already established for this exact extension point.

## Threat Flags

None beyond this plan's own declared threat model (T-01-39, T-01-SC), which is fully mitigated as designed: `fetchAllPages`'s cursor-history tracking and fail-closed anomaly codes directly implement T-01-39's "exhaust pages, cursor anomaly block, completeness diagnostics" mitigation, and `captureSnapshot`'s all-or-nothing multi-connection completeness contract (proven by the "blocks the entire snapshot" test) extends that same mitigation to the multi-connection case the threat register's "Linear pagination -> snapshot" trust boundary describes. No new network, credential, or filesystem surface was introduced; `pagination.ts` makes no GraphQL/SDK call itself (structurally verified: it only calls the injected `fetchPage` parameter).

## User Setup Required

None — everything is repository-local and offline after the one-time toolchain bootstrap already required by prior plans (verified working live in this worktree, including a from-scratch Node/npm install). No credentials, live Linear access, or manual configuration required to exercise any test or acceptance script in this plan.

## Next Phase Readiness

- Plan 01-26 (data-plus-errors/rate limits) can extend `protocol.ts`'s `Snapshot`/`SnapshotStatus` in place to produce `"partial"`/`"rate_limited"` statuses without changing the shape; its own `<read_first>` already names `pagination.ts`'s completeness diagnostics as required reading.
- Plan 01-27 (Node process boundary) can wire `adapter.ts`'s `captureSnapshot`/`ConnectionQuery` through `cli.ts`'s NDJSON loop, and construct real `ConnectionQuery.fetchPage` implementations against actual `@linear/sdk` connection methods (e.g. `client.issues()`, `client.projectMilestones()`), without redesigning either `pagination.ts` or the `Snapshot` contract.
- `tools/linear-sync/test/pagination.test.ts`'s fixture-driven `fetchPageFromFixturePages` pattern and the `paginated-51.json`/`cursor-loop.json` fixture shapes are reusable for any future Relay-connection-shaped fixture test in this workspace.
- The `node:test` reentrant-`TestContext` hazard documented above (tech-stack patterns) is worth carrying forward into 01-26/01-27's own test files, which will likely also need nested per-scenario subtests.

## Self-Check: PASSED

- All four created files verified present on disk: `tools/linear-sync/src/pagination.ts`, `tools/linear-sync/test/pagination.test.ts`, `tests/fixtures/linear/paginated-51.json`, `tests/fixtures/linear/cursor-loop.json` (all `FOUND`).
- Commit `0d69631` (feat) verified present in `git log --oneline` on branch `worktree-agent-a6c0c38922a37a2c6`; `git diff --diff-filter=D --name-only 0d69631~1 0d69631` reports zero deleted files.
- The plan's exact `<verify>` command — `powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-transport-pagination` — exits 0, run live multiple times in this worktree (including before/after the Rule 1 deadlock fix, and as part of the final regression pass).
- `go vet ./...` (via `test --quick`), `gofmt` (no findings on `internal/command/linear_sync.go`), `check --offline` (generate/check/build/test all with network denied), `generate --check` (zero drift), and the full `golc.ps1 test` (all 10 Go packages + both `linear-sdk-operations` and `linear-transport-pagination` Node scopes, 17+8 passing Node subtests) were all re-run as a final regression pass and pass cleanly.

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-21*
