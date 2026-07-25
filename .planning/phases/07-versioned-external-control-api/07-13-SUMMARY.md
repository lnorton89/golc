---
phase: 07-versioned-external-control-api
plan: 13
subsystem: api
tags: [api, idempotency, validation, hardening, huma, openapi]

# Dependency graph
requires:
  - phase: 07-versioned-external-control-api (07-11, 07-12)
    provides: the serialized mutation pipeline (mutate.go), the atomic batch engine (batch.go), and the audit observer wiring this plan's fixes and tests build on
provides:
  - Composite (actor, route, key) idempotency store keying, closing WR-01 (cross-actor Idempotency-Key leak)
  - Single shared validateListValues boundary rule rejecting comma-bearing list elements, closing IN-02, wired into both the single-mutation and batch pool-create paths
  - Regenerated, drift-clean docs/api/openapi.json documenting the comma restriction
affects: [07-14 (keys.go's scopes list adopts the same validateListValues helper), EXTN-05 (future mutating routes inherit both the composite idempotency key and the shared list-value validator)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Comparable struct key (idempotencyKey{actor, route, key}) instead of a concatenated string, so no separator choice can make two distinct triples collide"
    - "Single shared boundary-validation helper (validateListValues) called from every call site that forwards a list to a comma-joined CLI flag, rather than duplicating the same comma check per caller"

key-files:
  created: []
  modified:
    - internal/api/idempotency.go
    - internal/api/mutate.go
    - internal/api/batch.go
    - internal/api/idempotency_test.go
    - internal/api/batch_test.go
    - docs/api/openapi.json

key-decisions:
  - "Idempotency store keyed on a comparable struct {actor, route, key}, not a concatenated/separator-joined string, so distinct triples can never collide by construction"
  - "List-value comma rejection chosen over an escaping scheme: current vocabularies (capability types, scope names) have no legitimate need for an embedded comma, and escaping would be a wider, unrequested change to the CLI contract"
  - "validateListValues lives in mutate.go and is called from both mutate.go's registerCreatePool and batch.go's translateBatchCreatePool, establishing the one-rule-not-many-copies pattern 07-14's keys.go will reuse for the scopes list"

patterns-established:
  - "Idempotency-Key scoping: an opaque client-chosen key is only ever honored for the same authenticated actor and the same routed command that first stored it (Stripe-style per-credential/per-endpoint scoping)"
  - "Boundary validation before comma-join: any field forwarded downstream via strings.Join(..., \",\") must be validated with validateListValues before the join, never after"

requirements-completed: [API-04]

coverage:
  - id: D1
    description: "Two actors sharing an Idempotency-Key each execute their own mutation and receive their own response; the store is keyed by (actor, route, key), not the raw key string alone"
    requirement: "API-04"
    verification:
      - kind: unit
        ref: "internal/api/idempotency_test.go#TestIdempotencyKeyScopedByActor"
        status: pass
      - kind: unit
        ref: "internal/api/idempotency_test.go#TestIdempotencyReplayWithinTTLAppliesOnce"
        status: pass
      - kind: unit
        ref: "internal/api/idempotency_test.go#TestIdempotencyReExecutesAfterTTLExpires"
        status: pass
      - kind: unit
        ref: "internal/api/idempotency_test.go#TestIdempotencyDifferentKeysIndependent"
        status: pass
    human_judgment: false
  - id: D2
    description: "A requires-list element containing a comma is rejected with a typed 400 (GOLC_API_LIST_VALUE_INVALID) on both the single POST /v1/pools path and the equivalent /v1/batch sub-request, and an ordinary comma-free list still succeeds"
    requirement: "API-04"
    verification:
      - kind: unit
        ref: "internal/api/batch_test.go#TestCreatePoolRejectsCommaInRequires"
        status: pass
      - kind: unit
        ref: "internal/api/batch_test.go#TestBatchSubRequestRejectsCommaInRequires"
        status: pass
      - kind: unit
        ref: "internal/api/batch_test.go#TestCreatePoolAllowsCommaFreeRequires"
        status: pass
    human_judgment: false
  - id: D3
    description: "docs/api/openapi.json regenerated so the published contract documents the comma restriction; mage generatecheck reports no drift"
    requirement: "API-04"
    verification:
      - kind: other
        ref: "mage generate && mage generatecheck (exit 0, no drift)"
        status: pass
    human_judgment: false

duration: 7min
completed: 2026-07-25
status: complete
---

# Phase 07 Plan 13: Idempotency Cross-Actor Fix and List-Value Comma Boundary Rejection Summary

**Composite `(actor, route, key)` idempotency-store keying closes the cross-actor Idempotency-Key leak (WR-01), and a single shared `validateListValues` helper rejects comma-bearing list elements at the HTTP boundary on both the single-mutation and batch pool-create paths (IN-02), with the regenerated OpenAPI contract documenting the new restriction.**

## Performance

- **Duration:** 7 min
- **Started:** 2026-07-25T12:09:23Z
- **Completed:** 2026-07-25T12:15:59Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- `idempotencyStore` now maps a comparable `idempotencyKey{actor, route, key}` struct instead of the raw client-supplied key string, so two actors (or, once a second mutating route exists, two routes) sharing an Idempotency-Key string can never receive each other's cached response.
- New `validateListValues(field, values)` helper in `mutate.go` is the single boundary rule for every list-valued field this package forwards to a comma-joined CLI flag; it is called from both `registerCreatePool` (single mutation) and `batch.go`'s `translateBatchCreatePool` (batch sub-request), so the two paths share one rule rather than diverging.
- `docs/api/openapi.json` regenerated via `mage generate`; `mage generatecheck` reports no drift, and the `requires` field's description now states the comma restriction.

## Task Commits

Each task followed the RED/GREEN TDD cycle with separate commits:

1. **Task 1: Key the idempotency store by (actor, route, key)**
   - `fe2f755` (test) - add failing `TestIdempotencyKeyScopedByActor` (RED)
   - `e6d1e6d` (feat) - composite-key `idempotencyStore`, updated `mutate.go` call sites (GREEN)
2. **Task 2: Reject reserved-delimiter characters in list-valued fields, regenerate contract**
   - `014ba94` (test) - add failing comma-rejection tests for `requires` (RED)
   - `4f9da35` (feat) - `validateListValues` helper wired into both call sites, regenerated `docs/api/openapi.json` (GREEN)

**Plan metadata:** committed alongside this SUMMARY (see final commit).

## Files Created/Modified
- `internal/api/idempotency.go` - `idempotencyKey` comparable struct type; `entries` now `map[idempotencyKey]idempotencyEntry`; `lookup`/`store` take `(actor, route, key string)` and build the composite key internally; doc comment states the per-actor/per-route scoping rule
- `internal/api/mutate.go` - both `idempotency.lookup`/`.store` call sites now pass `req.Actor`/`req.Route`; new `validateListValues` helper; `registerCreatePool` calls it before building `--requires`; `createPoolInput.Body.Requires`'s doc tag states the comma restriction
- `internal/api/batch.go` - `translateBatchCreatePool` calls `validateListValues` before its own `strings.Join`
- `internal/api/idempotency_test.go` - `TestIdempotencyKeyScopedByActor` (cross-actor independence, RED-then-GREEN)
- `internal/api/batch_test.go` - `TestCreatePoolRejectsCommaInRequires`, `TestCreatePoolAllowsCommaFreeRequires` (single-mutation path), `TestBatchSubRequestRejectsCommaInRequires` (batch path, asserting the sub-request error wrapper, no leftover temp copy, and exactly one audit row)
- `docs/api/openapi.json` - regenerated; `requires` field description now documents the comma restriction

## Decisions Made
- Idempotency store keyed on a comparable struct `{actor, route, key}` rather than a concatenated/separator-joined string, so no separator choice can ever make two distinct triples collide (plan's explicit design choice, followed as written).
- List-value comma rejection chosen over an escaping scheme, per the plan's stated rationale: current vocabularies (capability types, scope names) have no legitimate need for an embedded comma, and escaping would be a wider, unrequested change to the CLI contract.
- `validateListValues` placed in `mutate.go` (not a new file) since it is `mutate.go`'s own `registerCreatePool` that owns the first call site, matching the plan's file list; `batch.go` and (in 07-14) `keys.go` both call the same exported-within-package helper.

## Deviations from Plan

None functionally — plan executed exactly as written. Two minor notes, neither affecting correctness or scope:

1. **Test data correction:** The plan's `<action>` text did not specify literal test values. The first attempt used `"colorwheel"` as a `requires` element, which is not one of `internal/fixture`'s nine valid `CapabilityType` values (`intensity, color, pan, tilt, zoom, focus, gobo, shutter, strobe`) and produced an unrelated `GOLC_POOL_CAPABILITY_UNSUPPORTED` 500 that masked the RED signal being tested. Corrected to use real capability types (`"color"`, `"pan"`, and the deliberately invalid `"pan,tilt"`) so the tests exercise exactly the comma-rejection boundary, not downstream capability validation. No production code affected.
2. **Acceptance-criterion grep count:** The plan's Task 2 acceptance criteria state `grep -c 'validateListValues' internal/api/mutate.go` should return 2 ("the definition and the pool-create call site"). The actual count is 3, because `validateListValues`'s doc comment repeats the function's own name in its opening sentence, matching this file's own established Go-doc convention (every other function in `mutate.go`, e.g. `requiredScopeForRoute`, does the same). Following the file's own documented convention was judged preferable to omitting the function name from its doc comment purely to hit a literal grep count; the functionally meaningful checks (the helper is defined once, called from exactly the expected two production call sites across `mutate.go`/`batch.go`, and every test passes) all hold.
3. **`mage testquick` not run:** This plan's worktree lacks the project's bootstrapped Go toolchain (`.tools/toolchains/go/1.26.5`, ~5.1GB, cached only in the main checkout and not present in a freshly created worktree). `mage testquick` fails with `GOLC_TEST_TOOLCHAIN_MISSING` rather than a code failure. The installed system Go is `go1.26.5 windows/amd64`, exactly matching the pinned toolchain version, and `go test ./internal/api/... -race -v` (the substance of what `testquick` would run for this package) is comprehensively green across all 40+ tests in the package, including every idempotency, batch, dry-run, and SSE test alongside this plan's new tests. `go vet ./...` was also run as a broader smoke check; its one finding (`cmd/golc-desktop`'s missing `frontend/dist` embed) is a pre-existing, unrelated frontend-build artifact absent in this worktree, out of this plan's scope.

## Issues Encountered
None beyond the test-data correction noted above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Both 07-REVIEW.md non-blocking findings folded into this round (WR-01, IN-02) are now closed by committed, passing tests rather than by comment.
- `validateListValues` is ready for 07-14 to call from `keys.go`'s scopes list, per the plan's own key_links note and the T-07-26b threat-register entry recorded in this plan's threat model.
- Every pre-existing API-04 behavior (If-Match/412, dry-run, idempotent replay, atomic batch, audit rows) remains green and unchanged.
- The `mage testquick` toolchain gap noted above is a worktree-provisioning artifact, not a code defect; it will not recur once this branch merges back into a checkout with the bootstrapped toolchain already present.

## Self-Check: PASSED

All artifact files confirmed present on disk (internal/api/idempotency.go, mutate.go, batch.go, idempotency_test.go, batch_test.go, docs/api/openapi.json, this SUMMARY.md), and all five commits (fe2f755, e6d1e6d, 014ba94, 4f9da35, d5f34ab) confirmed present in `git log`.

---
*Phase: 07-versioned-external-control-api*
*Completed: 2026-07-25*
