---
phase: 07-versioned-external-control-api
plan: 10
subsystem: api
tags: [api, requirements, roadmap, coverage-gate, documentation, tdd]

# Dependency graph
requires:
  - phase: 07-versioned-external-control-api
    provides: internal/api's capability-coverage gate (coverage_test.go), the 07-09 committed exclusion set, and 07-VERIFICATION.md's Gap 1 finding
provides:
  - "ROADMAP.md Phase 7 Success Criterion #1 rewritten to name the delivered /v1 capability breadth plus a named deferral owner"
  - "REQUIREMENTS.md API-01 narrowed to the delivered, coverage-gated subset; new EXTN-05 (v1.x) requirement filed as the deferral owner"
  - "coverage_test.go's three deferred exclusion reason constants (Art-Net, mutation, read future-work) now name EXTN-05, mechanically enforced by a new test"
affects: [phase-completion, gsd-secure-phase, future-EXTN-05-planning]

tech-stack:
  added: []
  patterns: ["Deferral-by-named-requirement: instead of leaving future work as unattributed prose, file it as an explicit v1.x requirement id and assert the pointer mechanically in the code that would otherwise silently drift"]

key-files:
  created: []
  modified:
    - .planning/ROADMAP.md
    - .planning/REQUIREMENTS.md
    - internal/api/coverage_test.go

key-decisions:
  - "API-01's claim was narrowed to exactly what /v1 delivers today (config/show inspect, pool create, api-key lifecycle, atomic batch, revisioned events) rather than wiring more routes to satisfy the original broader claim -- matches the developer's chosen route per 07-VERIFICATION.md gap 1 and the plan's explicit non-goal (zero new REST operations)."
  - "EXTN-05 was filed under v1.x 'Extended Control and Effects' (not a new numbered ROADMAP phase) so the v1 Traceability table's 88/88 coverage counts remain unchanged, matching the existing EXTN-01..04 convention of deliberate exclusion from that table."
  - "The API-01 concurrency probe ('if interrupted or run in parallel, what is guaranteed?') stays out of scope for this round, per the plan's own [ASSUMED] note -- it concerns concurrent invocation of many wired capabilities, and only one mutating domain (pool create) is wired; it belongs to EXTN-05 when breadth actually lands."

requirements-completed: [API-01]

coverage:
  - id: D1
    description: "ROADMAP.md Phase 7 Success Criterion #1 names the delivered /v1 capability breadth and the EXTN-05 deferral owner for the remaining domains"
    requirement: "API-01"
    verification:
      - kind: other
        ref: "grep -n 'Success Criteria' -A 6 .planning/ROADMAP.md"
        status: pass
    human_judgment: false
  - id: D2
    description: "REQUIREMENTS.md API-01 text narrowed to the coverage-gated subset; new EXTN-05 (v1.x) requirement filed under Extended Control and Effects"
    requirement: "API-01"
    verification:
      - kind: other
        ref: "grep -n 'EXTN-05' .planning/REQUIREMENTS.md"
        status: pass
    human_judgment: false
  - id: D3
    description: "coverage_test.go's three deferred future-work reason constants (Art-Net, mutation, read) each name EXTN-05; new TestFutureWorkExclusionsNameDeferralOwner asserts the pointer mechanically and that permanent categories do not claim one"
    requirement: "API-01"
    verification:
      - kind: unit
        ref: "internal/api/coverage_test.go#TestFutureWorkExclusionsNameDeferralOwner"
        status: pass
      - kind: unit
        ref: "internal/api/coverage_test.go#TestCapabilityCoverage"
        status: pass
      - kind: unit
        ref: "internal/api/coverage_test.go#TestNoPendingRoutes"
        status: pass
    human_judgment: false

duration: ~15min
completed: 2026-07-25
status: complete
---

# Phase 7 Plan 10: Re-scope API-01 to delivered /v1 breadth Summary

**Narrowed API-01's "every capability" claim to what /v1 actually delivers (config/show inspect, pool create, api-key lifecycle, atomic batch, revisioned events) and filed the remaining show-domain/Art-Net breadth plus the Wails/HTTP parity check as a new named requirement, EXTN-05 (v1.x), enforced mechanically in coverage_test.go so the deferral pointer cannot be silently dropped.**

## Performance

- **Duration:** ~15 min
- **Tasks:** 2 completed
- **Files modified:** 3 (.planning/ROADMAP.md, .planning/REQUIREMENTS.md, internal/api/coverage_test.go)

## Accomplishments

- ROADMAP.md Phase 7 Success Criterion #1 now describes the concrete, delivered `/api/v1` capability set (config concern inspect, show inspect, pool creation, api-key create/list/revoke, atomic batch, revisioned SSE events) dispatched through the same internal/command route registry the UI uses, and names EXTN-05 (v1.x) as the deferral owner for the remaining show domains (scene, chase, motion, theme, preset, blend, deployment, operatorsurface, playback, programmer, fixture-import, show open/save) and Art-Net runtime control, plus the Wails-versus-HTTP outcome-parity check.
- REQUIREMENTS.md API-01's bullet text is narrowed to match (same claim, same deferral pointer to EXTN-05), and a new `EXTN-05` bullet was appended to the v1.x "Extended Control and Effects" list, immediately after EXTN-04, in the same unchecked-box-free bullet shape.
- internal/api/coverage_test.go's three deferred future-work exclusion reason constants (`reasonArtnetFutureWork`, `reasonMutationFutureWork`, `reasonReadFutureWork`) each now append a trailing clause naming EXTN-05 as the requirement that owns closing that category; `reasonDevTooling` and the two permanent categories (`reasonDaemonLifecycle`, `reasonLocalProcessLaunch`) were deliberately left untouched. The `buildExcludedRoutes` doc comment was updated to replace its prior open-ended "a future milestone could wire" framing with the now-decided EXTN-05 ownership, while preserving the existing permanent-versus-deferred distinction.
- New test `TestFutureWorkExclusionsNameDeferralOwner` asserts (by sampling one route per category from the live `excludedRoutes` map, not by re-declaring constants) that each of the three deferred categories names EXTN-05, and that the two permanent categories do not — so a future bulk edit can never silently flatten the distinction or drop the pointer. Confirmed via the TDD RED/GREEN cycle: the test failed against the unmodified reason strings (RED, committed separately), then passed once the EXTN-05 clauses were added (GREEN).
- `TestCapabilityCoverage`'s never-both/never-neither/stale-exclusion checks and `TestNoPendingRoutes`' placeholder scan both still pass unchanged, so the narrowed claim remains mechanically enforced, not merely prose.
- Confirmed `api.RegisteredRoutes()` (internal/api/router.go) already returns its covered-route set via `sort.Strings`, satisfying the plan's ordering must-have with no code change required.

## Task Commits

Each task was committed atomically:

1. **Task 1: Narrow API-01 in ROADMAP.md + REQUIREMENTS.md and file EXTN-05 as the named deferral owner** - `6b6f9f6` (docs)
2. **Task 2: Point coverage_test.go's future-work exclusions at EXTN-05 and assert the pointer mechanically**
   - RED: `f9e55ec` (test) — failing `TestFutureWorkExclusionsNameDeferralOwner` against unmodified reason strings
   - GREEN: `cf3f033` (feat) — EXTN-05 clauses added to the three deferred reason constants + doc comment update; test passes

_TDD task produced two commits (test -> feat); no refactor commit was needed._

## Files Created/Modified

- `.planning/ROADMAP.md` - Phase 7 Success Criterion #1 rewritten to the delivered scope with named EXTN-05 deferral
- `.planning/REQUIREMENTS.md` - API-01 text narrowed; new EXTN-05 (v1.x) bullet appended after EXTN-04
- `internal/api/coverage_test.go` - three deferred reason constants + doc comment name EXTN-05; new `TestFutureWorkExclusionsNameDeferralOwner`

## Decisions Made

- Re-scoped API-01 rather than wiring additional routes, per the plan's explicit objective and the developer's chosen route out of 07-VERIFICATION.md gap 1.
- Filed EXTN-05 under v1.x "Extended Control and Effects" (not a new ROADMAP phase), keeping it deliberately absent from the Traceability table alongside EXTN-01..04, so the 88/88 v1 coverage counts are unchanged.
- Left the API-01 Traceability row (`| API-01 | Phase 7 | Pending |`) untouched — phase-completion flow owns flipping it, not this gap-closure plan.
- Did not touch `.planning/linear-map.json`; editing API-01's requirement text changes that catalog entity's display string, and `go test ./internal/trace/catalog/... -run TestScopeLinearMap` was independently re-confirmed still failing with `GOLC_MIGRATE_DRIFT` in this worktree, consistent with 07-VERIFICATION.md's documentation of this as pre-existing and unrelated to Phase 7 code. No action taken per the plan's explicit "out of scope" instruction.

## Deviations from Plan

None - plan executed exactly as written. Both tasks matched their described actions; no auto-fixes, blockers, or architectural questions arose.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Gap 1 from 07-VERIFICATION.md is closed: ROADMAP Success Criterion #1 and REQUIREMENTS API-01 now read as one consistent, delivered-scope claim, and the deferred breadth has a named, mechanically-enforced owner (EXTN-05) rather than an unattributed "future work" label.
- 07-11-PLAN.md and 07-12-PLAN.md (gap wave 1 / wave 9, closing verification gaps 2 and 3) are unaffected by this plan's changes and remain ready to execute independently.
- Known, pre-existing, out-of-scope failure carried forward unchanged: `go test ./internal/trace/catalog/... -run TestScopeLinearMap` fails with `GOLC_MIGRATE_DRIFT` against `.planning/linear-map.json` (documented in 07-VERIFICATION.md, unrelated to Phase 7 code, not repaired by this plan).

---
*Phase: 07-versioned-external-control-api*
*Completed: 2026-07-25*

## Self-Check: PASSED

- FOUND: .planning/phases/07-versioned-external-control-api/07-10-SUMMARY.md
- FOUND: 6b6f9f6 (docs: narrow API-01 + file EXTN-05)
- FOUND: f9e55ec (test: RED, failing deferral-pointer test)
- FOUND: cf3f033 (feat: GREEN, EXTN-05 clauses added)
- FOUND: dbef739 (docs: SUMMARY.md commit)
