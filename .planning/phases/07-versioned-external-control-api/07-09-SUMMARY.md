---
phase: 07-versioned-external-control-api
plan: 09
subsystem: api
tags: [api, openapi, huma, drift, deprecation, typed-errors, docs, audit]

# Dependency graph
requires:
  - phase: 07-versioned-external-control-api
    provides: "07-02: RegisterOperation self-registration seam; 07-04: keys.go; 07-05: mutate.go's serialized pipeline + observer.go's MutationEvent seam; 07-06: batch.go; 07-07: audit.go's RegisterAuditObserver (unwired); 07-08: events.go's SSE"
provides:
  - "internal/api/generate.go: GenerateOpenAPI/CheckOpenAPIDrift, mirroring internal/contracts' generate+CheckDrift discipline (D-03) -- the published, byte-stable, drift-checked OpenAPI 3.1 contract"
  - "docs/api/openapi.json: the committed generated contract covering every currently self-registered /v1 operation"
  - "docs/api/COMPATIBILITY.md: D-02's versioning/breaking-change/deprecation-window/header policy + curl client examples"
  - "internal/api/deprecation.go: MarkOperationDeprecated + DeprecationMiddleware (scaffolded, unwired -- no operation is deprecated yet)"
  - "internal/api.RegisterAuditObserver now wired into NewServer -- every real mutation leaves an audit trail in production (closes 07-07's flagged gap)"
affects: []  # last plan in Phase 7

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "generate.go reflects the full Huma API from router.go's operationRegistrations against a throwaway zero-value *Server (no listener bound) to obtain humaAPI.OpenAPI(), then serializes it via encoding/json's own deterministic map-key-sorting + huma.OpenAPI's ordered-field MarshalJSON -- no extra normalization pass was needed (contrast internal/contracts' NormalizeSchema), proven by TestOpenAPIDeterministic."
    - "Common typed-error responses (400/401/403/412/429) are declared by post-processing the generated *huma.OpenAPI document's Responses maps in generate.go, not by editing every individual operation's huma.Operation{Errors: [...]} in translate.go/mutate.go/batch.go/keys.go -- these codes originate from shared middleware (auth, rate-limit) and the mutation pipeline, not any single handler's own success-path Go code, so declaring them at the document-generation layer keeps every existing operation-registration file untouched."
    - "Raw JSON has no comment syntax, so the GENERATED...DO NOT EDIT marker is carried in two machine/human-visible places: Info.Description and an \"x-generated\" vendor extension."
    - "internal/command/generate.go and check.go (not magefiles/magefile.go) are the actual wiring point for 'mage generate/check targets include the api OpenAPI': mage's generate/generatecheck targets already route through internal/delivery's MageTarget->route mapping into the single 'generate'/'generate --check' CLI route, so adding api.GenerateOpenAPI/CheckOpenAPIDrift calls there automatically reaches every mage entrypoint with zero magefile.go edits."

key-files:
  created:
    - internal/api/generate.go
    - internal/api/generate_test.go
    - internal/api/deprecation.go
    - internal/api/deprecation_test.go
    - docs/api/openapi.json
    - docs/api/COMPATIBILITY.md
  modified:
    - internal/command/generate.go
    - internal/command/check.go
    - internal/api/coverage_test.go
    - internal/api/server.go

key-decisions:
  - "Coverage-closure scope boundary (must_haves truth NOT fully met -- see \"Known Gaps\" below): this plan did NOT wire the ~67 remaining show-domain/Art-Net/dev-tooling routes into new REST operations. 07-05-SUMMARY.md explicitly named two acceptable closing paths for this plan -- \"its own wave of route-by-route wiring\" or \"an explicit acknowledgment that this remains a documented, deliberate scope boundary\" -- and this plan takes the second, since (a) this plan's own declared files_modified scope names no new per-domain operation files, (b) each remaining mutation route needs its own bespoke Huma input struct mapping JSON body fields to CLI flags (mutate.go's \"pool create\" precedent -- no generic passthrough exists), and (c) ~67 such routes is a multi-plan effort, not a single closing task. Instead, every exclusion reason was rewritten to be honest and individually categorized (permanent vs. future-milestone-work), and TestNoPendingRoutes was added to mechanically guard against a blank/placeholder reason ever creeping in."
  - "RegisterAuditObserver(root, showPath) is wired into NewServer (server.go), not left for a future plan: 07-07-SUMMARY.md explicitly flagged this as an unwired gap and named 07-09 as the plan that should close it. D-07 guarantees exactly one *Server per daemon process, so production gets exactly one registration. Verified safe against this package's own test suite (which constructs many *Server values per process, accumulating harmless swallowed-failure observers in observer.go's global registry) including under -race."
  - "The 400/401/403/412/429 typed-error responses are declared generically on every generated operation (generate.go post-processing the OpenAPI document) rather than by editing huma.Operation{Errors: [...]} in each of translate.go/mutate.go/batch.go/keys.go: these five codes are produced by shared cross-cutting middleware (auth, rate-limit) and the mutation pipeline, not by any single handler's own success-path return type, so every operation can genuinely encounter all five regardless of its own specific Go code path."
  - "CheckOpenAPIDrift never writes a disposable temp directory (unlike internal/contracts.CheckDrift): renderOpenAPI generates entirely in memory, so there is no intermediate filesystem write to clean up -- a stronger form of the same 'never touches committed bytes' guarantee."

requirements-completed: [API-02]

coverage:
  - id: D1
    description: "OpenAPI 3.1 document generation is deterministic and byte-stable across runs; the drift check is read-only and never rewrites committed bytes"
    requirement: "API-02"
    verification:
      - kind: unit
        ref: "internal/api/generate_test.go#TestOpenAPIDeterministic"
        status: pass
      - kind: unit
        ref: "internal/api/generate_test.go#TestOpenAPIDrift"
        status: pass
    human_judgment: false
  - id: D2
    description: "The generated document covers every /v1 operation (queries, mutations, batch, keys) and declares 400/401/403/412/429 typed error responses"
    requirement: "API-02"
    verification:
      - kind: unit
        ref: "internal/api/generate_test.go#TestOpenAPIDocumentsEveryOperation"
        status: pass
    human_judgment: false
  - id: D3
    description: "The committed document self-identifies as generated (GENERATED...DO NOT EDIT marker, both human- and machine-visible)"
    requirement: "API-02"
    verification:
      - kind: unit
        ref: "internal/api/generate_test.go#TestOpenAPIGeneratedMarker"
        status: pass
    human_judgment: false
  - id: D4
    description: "MarkOperationDeprecated sets the OpenAPI Deprecated flag and attaches Sunset/Link metadata; the header helper produces the exact Deprecation/Sunset/Link header set the compatibility policy documents, normalized to UTC"
    requirement: "API-02"
    verification:
      - kind: unit
        ref: "internal/api/deprecation_test.go#TestMarkOperationDeprecatedSetsOpenAPIFlag"
        status: pass
      - kind: unit
        ref: "internal/api/deprecation_test.go#TestDeprecationHeadersForSunsetAndLink"
        status: pass
      - kind: unit
        ref: "internal/api/deprecation_test.go#TestDeprecationHeadersUseUTC"
        status: pass
    human_judgment: false
  - id: D5
    description: "Every registered route is either a REST operation or an individually-reasoned exclusion, never silently unmapped; no exclusion reason is a blank/placeholder string"
    requirement: "API-01"
    verification:
      - kind: integration
        ref: "internal/api/coverage_test.go#TestCapabilityCoverage"
        status: pass
      - kind: unit
        ref: "internal/api/coverage_test.go#TestNoPendingRoutes"
        status: pass
    human_judgment: false
  - id: D6
    description: "Every real mutation writes an audit trail in production (RegisterAuditObserver wired into NewServer, not just available as an unwired seam)"
    requirement: "API-06"
    verification:
      - kind: integration
        ref: "internal/api tests (audit_test.go's existing suite) pass unchanged with the new NewServer wiring, including under -race"
        status: pass
    human_judgment: false

# Metrics
duration: 90min
completed: 2026-07-25
status: complete
---

# Phase 7 Plan 9: OpenAPI 3.1 Contract, Compatibility Policy, and Coverage-Gate Closure Summary

**The API's capstone contract: a byte-stable, drift-checked OpenAPI 3.1 document generated directly from the Go handler structs (mirroring `internal/contracts`), a published compatibility/deprecation policy with a working Deprecation/Sunset header mechanism, `RegisterAuditObserver` finally wired into production, and an honestly re-documented capability-coverage exclusion set.**

## Performance

- **Duration:** ~90 min
- **Completed:** 2026-07-25
- **Tasks:** 2/2
- **Files modified:** 10 (6 created, 4 modified)

## Accomplishments
- Built `internal/api/generate.go`: `GenerateOpenAPI`/`CheckOpenAPIDrift` mirror `internal/contracts/generate.go`'s generate+CheckDrift discipline exactly (D-03) -- reflecting the full Huma API from every self-registered operation (`router.go`'s `operationRegistrations`) into a canonical, byte-stable OpenAPI 3.1 document, with a `GENERATED ... DO NOT EDIT` marker carried in both `Info.Description` and an `x-generated` vendor extension (raw JSON has no comments).
- Every generated operation declares `400`/`401`/`403`/`412`/`429` typed-error responses via a document-level post-processing pass (`declareCommonErrors`), rather than editing every individual operation-registration file -- these codes come from shared cross-cutting middleware (auth, rate-limit) and the mutation pipeline, not any single handler's own success-path type.
- Generated and committed `docs/api/openapi.json`: covers all 8 currently self-registered operations across 7 paths (`/v1/config/{concern}`, `/v1/show`, `/v1/pools`, `/v1/batch`, `/v1/keys`, `/v1/keys/{id}`, `/v1/events`).
- Wired the api contract into `internal/command/generate.go`'s `runGenerate` and `check.go`'s `runProjectCheck` (both already called `contracts.CheckDrift`/`GenerateAll`) -- since mage's `generate`/`generatecheck` targets already route through `internal/delivery`'s `MageTarget`->route mapping into this exact CLI route, no `magefiles/magefile.go` edit was needed to satisfy "mage generate/check targets include the api OpenAPI."
- Wrote `docs/api/COMPATIBILITY.md`: D-02's full policy (URL-path versioning, breaking-change definition, 180-day deprecation window, `Deprecation`/`Sunset`/`Link` response-header signals, the complete typed-error table) plus working `curl` examples for a query, an `If-Match` mutation, a dry-run, a batch, a key mint, and opening/resuming the SSE event stream.
- Built `internal/api/deprecation.go`: `MarkOperationDeprecated` (sets `op.Deprecated` + stashes `DeprecationInfo` on `op.Metadata`) and `DeprecationMiddleware` (emits the three headers for a marked operation's responses) -- scaffolded and unit-tested now since no `/v1` operation is deprecated yet.
- **Closed 07-07's flagged production gap:** `RegisterAuditObserver(root, showPath)` is now called from `NewServer`, so every real API mutation leaves an audit trail in production. Verified safe against the full `internal/api` suite (including `-race`) despite the package's own tests constructing many `*Server` values per test binary.
- Re-documented `coverage_test.go`'s exclusion set: renamed every `*Deferred` reason to `*FutureWork` (the old "deferred to a later 07-0x plan" wording is stale now that this is Phase 7's final plan) and rewrote each reason string to honestly distinguish permanent exclusions (daemon-lifecycle, local-process-launch, dev-tooling) from explicit future-milestone work. Added `TestNoPendingRoutes`.

## Task Commits

1. **Task 1: Generate + commit the OpenAPI 3.1 contract with a byte-stable drift check** - `76fa154` (feat)
2. **Task 2: Compatibility/deprecation policy + headers + final capability-coverage closure** - `44cbd36` (docs: COMPATIBILITY.md + deprecation.go), `3f5a843` (fix: audit-observer wiring + coverage re-documentation)

**Plan metadata:** SUMMARY.md commit follows this file; STATE.md/ROADMAP.md/REQUIREMENTS.md intentionally left untouched -- this plan ran as a parallel worktree agent, and the orchestrator owns those writes centrally after the wave completes.

## Files Created/Modified
- `internal/api/generate.go` - `GenerateOpenAPI`, `CheckOpenAPIDrift`, `buildOpenAPIDocument`, `declareCommonErrors`, `pathItemOperations`
- `internal/api/generate_test.go` - `TestOpenAPIDeterministic`, `TestOpenAPIDrift`, `TestOpenAPIDocumentsEveryOperation`, `TestOpenAPIGeneratedMarker`
- `internal/api/deprecation.go` - `DeprecationInfo`, `MarkOperationDeprecated`, `DeprecationMiddleware`, `deprecationHeadersFor`
- `internal/api/deprecation_test.go` - `TestMarkOperationDeprecatedSetsOpenAPIFlag`, `TestDeprecationInfoForUnmarkedOperation`, `TestDeprecationHeadersForSunsetAndLink`, `TestDeprecationHeadersUseUTC`
- `docs/api/openapi.json` - committed, generated OpenAPI 3.1 contract
- `docs/api/COMPATIBILITY.md` - D-02's compatibility/deprecation policy + client examples
- `internal/command/generate.go` - wired `api.GenerateOpenAPI`/`CheckOpenAPIDrift` into the `generate`/`generate --check` route
- `internal/command/check.go` - wired `api.CheckOpenAPIDrift` into `check --concern project`'s generated-artifact drift check
- `internal/api/coverage_test.go` - renamed/rewrote exclusion-reason categories, added `TestNoPendingRoutes`
- `internal/api/server.go` - `NewServer` now calls `RegisterAuditObserver(root, showPath)`

## Decisions Made
- Coverage-closure scope boundary, RegisterAuditObserver wiring, generic typed-error declaration, and in-memory drift checking -- see `key-decisions` in the frontmatter for full rationale on each.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical Functionality] Wired `RegisterAuditObserver` into `NewServer`**
- **Found during:** Pre-task review of 07-07-SUMMARY.md (explicitly flagged by the orchestrator's own prompt as a known gap to check)
- **Issue:** 07-07 built a complete redacting audit observer (`internal/api/audit.go`'s `RegisterAuditObserver`) but deliberately never called it from production code, to avoid a merge collision with the parallel 07-08 SSE plan sharing the same wave. No daemon process called it, so no real mutation left an audit trail.
- **Fix:** Added one call, `RegisterAuditObserver(server.root, server.showPath)`, at the end of `NewServer` (server.go). D-07 guarantees exactly one `*Server` per daemon process, so production registers exactly once.
- **Files modified:** `internal/api/server.go`
- **Verification:** Full `internal/api` suite (72 tests including all of `audit_test.go`) passes unchanged, including under `-race`; no performance degradation observed despite every test file's own `NewServer` calls now each registering one (mostly harmless, swallowed-failure) audit observer for the remainder of the test binary.
- **Committed in:** `3f5a843`

**2. [Rule 3 - Blocking Issue] Wired the api contract into `internal/command/generate.go`/`check.go` instead of `magefiles/magefile.go`**
- **Found during:** Task 1, investigating how mage's `generate`/`generatecheck` targets actually reach `contracts.GenerateAll`/`CheckDrift`
- **Issue:** The plan's stated files list named `magefiles/magefile.go` as needing an edit to satisfy "mage generate/check targets include the api OpenAPI." Investigation showed mage's `Generate()`/`GenerateCheck()` functions already route generically through `internal/delivery.LookupMageTarget` into the single `"generate"`/`"generate --check"` CLI route (`internal/command/generate.go`'s `runGenerate`) -- the actual call site of `contracts.GenerateAll`/`CheckDrift`. Editing `magefile.go` directly would have had no effect (or required duplicating the routing logic); the correct wiring point is one layer down.
- **Fix:** Added `api.GenerateOpenAPI`/`CheckOpenAPIDrift` calls to `internal/command/generate.go`'s `runGenerate` and `internal/command/check.go`'s `runProjectCheck` (which already called the equivalent `contracts.*` functions for the same reason). No import cycle: `internal/command` already imports `internal/api` (via `artnet.go`'s `apiCommandExecutor` adapter, established in 07-02).
- **Files modified:** `internal/command/generate.go`, `internal/command/check.go`
- **Verification:** `go build ./internal/...` green (no import cycle); `internal/command`'s test suite shows only the 5 pre-existing `GOLC_TEST_TOOLCHAIN_MISSING` failures already documented in 07-04/07-05/07-07-SUMMARY.md (this worktree has not run `mage Bootstrap`), unrelated to this change.
- **Committed in:** `76fa154`

---

**Total deviations:** 2 auto-fixed (1 missing critical functionality, 1 blocking issue). No architectural changes (Rule 4) were needed.
**Impact on plan:** Both were necessary to fulfill the plan's own stated acceptance criteria and the orchestrator's explicit known-gap check; neither expands scope beyond what those criteria require.

## Known Gaps

**Capability-coverage closure is NOT fully achieved** (must_haves truth #5: "the exclusion set now contains only permanent, documented exclusions ... with no remaining pending/unexposed routes"). After this plan, `coverage_test.go`'s exclusion set still contains ~67 routes across four categories:

| Category | Count | Status |
|---|---|---|
| `reasonDevTooling` | 16 | Permanent (local-checkout-only commands, never meaningful over remote HTTP) |
| `reasonDaemonLifecycle` | 1 (`artnet serve`) | Permanent (IS the process hosting `/v1`) |
| `reasonLocalProcessLaunch` | 1 (`run`) | Permanent (local GUI-launch action) |
| `reasonArtnetFutureWork` | 10 | **Not permanent** -- future milestone work |
| `reasonMutationFutureWork` | 42 | **Not permanent** -- future milestone work |
| `reasonReadFutureWork` | 9 | **Not permanent** -- future milestone work |

Only the first three categories (18 routes) are genuinely permanent, matching the plan's stated target of "daemon-lifecycle and interactive routes." The remaining 61 routes (Art-Net runtime, show-domain mutations, show-domain reads) are honestly re-labeled as future-milestone work rather than falsely claimed closed, but were **not wired into new REST operations** by this plan.

**Why:** 07-05-SUMMARY.md explicitly anticipated this tension and named two acceptable paths for 07-09 to take -- "its own wave of route-by-route wiring" or "an explicit acknowledgment that this remains a documented, deliberate scope boundary." This plan takes the second path, because:
1. This plan's own declared `files_modified` scope (`generate.go`, `generate_test.go`, `deprecation.go`, `coverage_test.go`, `docs/api/*`) names no new per-domain operation files.
2. Each of the 42 remaining mutation routes needs its own bespoke Huma input struct mapping JSON body fields to that route's specific CLI flags -- `mutate.go`'s `"pool create"` registration is ~30 lines of route-specific code with no generic passthrough available (verified by reading `mutate.go` directly). Wiring 42 more is a multi-plan effort, not a single closing task.
3. Several of the 9 read routes additionally need dedicated design work beyond a `RegisterOperation` call (they emit plain-text stdout, not JSON, or accept a client-supplied filesystem path -- the same blocker 07-02-SUMMARY.md already identified for these exact routes).

**Recommendation:** A future milestone should either (a) run a dedicated route-by-route wiring phase reusing `mutate.go`'s proven pipeline, prioritizing the highest-value show-domain mutations first, or (b) explicitly re-scope API-01's "every capability" claim to the show/pool/key/batch/event domains this phase actually completed, formally deferring Art-Net runtime control and the remaining show-domain routes to a named future phase in ROADMAP.md.

## Issues Encountered
- `internal/api/observer.go` (a pre-existing file from 07-05, never touched by this plan) fails `gofmt -l` due to a one-line whitespace-alignment issue (`mutationObservers    []func(...)` vs. `mutationObservers   []func(...)`) introduced in commit `1952bf9`. Out of this plan's scope per the executor's own scope-boundary rule (pre-existing issue in a file this plan never modifies); not fixed, flagged here for a future cleanup pass.
- `mage testquick`/full `go test ./internal/command/...` show the same 5 pre-existing `GOLC_TEST_TOOLCHAIN_MISSING`/`golc-project binary not built` failures already documented in 07-04/07-05/07-07-SUMMARY.md (this worktree has not run `mage Bootstrap`) -- confirmed unrelated to any file this plan touches; `go build ./...` (whole repo, excluding the pre-existing unbuilt Wails frontend) is green.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The OpenAPI contract (`docs/api/openapi.json`), its drift check (`generate --check`, `check --concern project`, `mage generatecheck`), and the compatibility/deprecation policy (`docs/api/COMPATIBILITY.md`) are the stable, general-purpose foundation for any future `/v2` migration: `internal/api/deprecation.go`'s `MarkOperationDeprecated`/`DeprecationMiddleware` are ready to apply the moment a breaking change actually ships.
- `internal/api.RegisterAuditObserver` is now live in production (`NewServer`) -- every future mutation-adding route (the deferred 42 above) automatically gets an audit trail with zero further plumbing, the same "attach with zero further plumbing" guarantee 07-05/07-07/07-08-SUMMARY.md already established for the observer seam.
- **Phase 7 (Versioned External Control API) is now complete as far as this plan's own scope reaches**, but API-01's "every public GOLC capability" claim is only fully true for the show/pool/key/batch/event domains this phase actually wired -- see "Known Gaps" above for the honest accounting of what remains for a future milestone.

---
*Phase: 07-versioned-external-control-api*
*Completed: 2026-07-25*

## Self-Check: PASSED
- FOUND: internal/api/generate.go
- FOUND: internal/api/generate_test.go
- FOUND: internal/api/deprecation.go
- FOUND: internal/api/deprecation_test.go
- FOUND: docs/api/openapi.json
- FOUND: docs/api/COMPATIBILITY.md
- FOUND: internal/command/generate.go
- FOUND: internal/command/check.go
- FOUND: internal/api/coverage_test.go
- FOUND: internal/api/server.go
- FOUND: commit 76fa154
- FOUND: commit 44cbd36
- FOUND: commit 3f5a843
