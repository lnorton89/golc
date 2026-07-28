---
phase: 03-deterministic-show-programming-and-playback
verified: 2026-07-22T05:23:50Z
status: passed
score: 5/5 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification: false
resolved_gaps:
  - truth: "REQUIREMENTS.md accurately reflects PROG-04 completion status"
    status: resolved
    resolved_at: 2026-07-22T05:30:00Z
    resolution: >
      Checked off `- [x] **PROG-04**` and updated its Traceability row to
      "Complete" in .planning/REQUIREMENTS.md (commit b416ee8). The
      underlying requirement was already satisfied in code and covered by
      passing tests — this closed the doc-tracking omission from 03-02's
      docs commit (f12e46a), which updated SUMMARY.md but not
      REQUIREMENTS.md.
---

# Phase 3: Deterministic Show Programming and Playback Verification Report

**Phase Goal:** Show authors can program complete tempo-aware looks and run them through a headless engine whose output is deterministic under adapter delay or failure.
**Verified:** 2026-07-22T05:23:50Z
**Status:** passed (one documentation-only gap found and resolved — see Gaps Summary)
**Re-verification:** No — initial verification, but incorporates confirmation of a prior code-review fix cycle (03-REVIEW.md -> 03-REVIEW-FIX.md, CR-01 + WR-01/02/03)

## Goal Achievement

### Observable Truths (Success Criteria 1-5)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Show author can select pools/groups/instances/fixtures; set semantic intensity/color/position/beam/fixture-specific attributes; inspect touched values, sources, record scope | VERIFIED | `internal/programming/selection.go` (Resolve), `programmer.go` (Set/Touched), CLI `programmer set`/`programmer inspect` in `internal/command/programming.go`. `go test ./internal/programming/... ./internal/command/...` green; PROG-01 adjacency/empty/ordering/dangling-ref tests and PROG-02/03 range/capability rejection tests all pass. |
| 2 | Show author can create/reuse themes, attribute presets, tempo-relative chases, semantic motion presets; record/update/rename/reorder/duplicate/delete with undo/redo | VERIFIED | `internal/programming/theme.go`, `preset.go`, `chase.go`, `motion.go`, `history.go`. `TestHistoryRecordUndoRedoRoundTrip`, `TestHistoryMixedObjectTypeSingleGlobalStack` (single whole-session stack, D-12), `TestHistoryUndoEmptyBoundaryNoCrash`/`TestHistoryRedoNoTailBoundaryNoCrash` all pass. CRUD verbs (`chase update`/`rename`/`reorder`/`duplicate`/`delete`, etc.) exist in `internal/command/programming.go`. |
| 3 | Show author can assemble a scene as configured bar loop with independently enabled base-look/color-theme/chase/motion layers plus reusable blending presets | VERIFIED | `internal/scene/scene.go` (bar-loop config, SCEN-01 model), `layer.go` (4 independently-enabled layers, D-01..D-03), `blend.go` (SCEN-07 blend presets). `TestSingleActiveScene*` (SCEN-04), layer/blend tests all pass. Fixed-priority reduce (`scene.ReduceLayers`, D-02, no HTP) verified by `internal/playback/evaluate_test.go`. |
| 4 | Operator can enter/tap global BPM, switch the one active scene or any layer immediately, choose whether a BPM change preserves musical position or restarts the loop | VERIFIED (post-fix) | `internal/playback/clock.go` (`Position`, `RecomputeEpoch`), CLI `playback bpm set`/`playback bpm tap` (`internal/command/playback.go`). **CR-01 closure independently re-verified**: `internal/playback/engine.go:143-155` `tick()` now calls `RecomputeEpoch(pending.PreserveOnBPMChange, plan.BPM, pending.BPM, plan.BarsPerLoop, e.loopStart, now)` whenever the plan adopted at a bar-boundary crossing carries a different BPM than the plan it replaces -- read directly from the file, not taken from the fix commit's own claim. `TestEngineBPMChangePreservesPosition`/`TestEngineBPMChangeRestartsAtBarZero` in `internal/playback/engine_test.go` exercise this through `e.StageEdit` + `e.tick` (the real adoption path used by `Start`'s goroutine), not by calling `RecomputeEpoch` directly, and assert the resulting `BarIndex`/`BeatFraction` against what the old-BPM plan would have reported at the same instant. Both tests independently re-run and pass (`go test ./internal/playback/... -run 'TestEngineBPMChangePreservesPosition|TestEngineBPMChangeRestartsAtBarZero' -v`). SCEN-06 immediate-switch-at-next-bar-boundary covered by `internal/playback/engine_test.go` switch tests. |
| 5 | Deterministic playback harness produces same time-indexed results under UI/persistence/scripts/API/LLM slowness or restart; adopts only complete valid show plans at safe boundaries | VERIFIED | `internal/playback/compile.go` (`Compile`, all-or-nothing, D-06), `evaluate.go` (`Evaluate`, pure function), `engine.go` (atomic-pointer publish, next-bar adoption, reject-and-keep-last-valid). `TestDeterministicEvaluateSameArgs`/`TestDeterministicEvaluateAcrossGoroutines` pass under `-race`. Structural isolation confirmed by direct import inspection: `internal/playback`, `internal/scene`, `internal/programming` import none of `internal/command` or any adapter package (only a doc-comment cross-reference, not a Go import, appears in `clock.go`). |

**Score:** 5/5 truths verified. 0 behavior-unverified.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/programming/selection.go` | PROG-01 resolution | VERIFIED | 6.4KB, substantive, wired via `internal/command/programming.go` |
| `internal/programming/programmer.go` | PROG-02/03 programmer state | VERIFIED | 6.3KB, wired |
| `internal/programming/theme.go` / `preset.go` | PROG-04 themes/presets | VERIFIED | 2.6KB/6.9KB, wired via `theme create`/`preset record` CLI routes, persisted on `show.State` |
| `internal/programming/chase.go` / `motion.go` | PROG-05/06 chases/motion presets | VERIFIED | 5.4KB/5.9KB, wired |
| `internal/programming/history.go` | PROG-07 undo/redo | VERIFIED | 4.4KB, single whole-session stack confirmed by test |
| `internal/scene/scene.go` / `layer.go` / `blend.go` | SCEN-01/04/05/07 scene model | VERIFIED | 12.3KB/3.8KB/4.2KB, wired via `internal/command/scene.go` |
| `internal/playback/clock.go` | SCEN-01/02/03/08 musical clock | VERIFIED | 8.4KB, `RecomputeEpoch` confirmed called by `engine.go` (see truth 4) |
| `internal/playback/compile.go`, `evaluate.go`, `engine.go`, `frame.go` | SCEN-06/09 compiler/evaluator/engine | VERIFIED | All substantive and cross-wired; `engine.go` doc comments updated post-fix to describe the recompute behavior accurately |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `command/programming.go` programmer routes | `programming.ProgrammerState` | mutation -> `show.State.Programmer` persisted via `show.Save` | WIRED | Confirmed by passing integration tests in `internal/command` |
| `command/scene.go` scene/layer routes | `scene.NewScene`/`SetLayer` | -> `show.State.Scenes`/`BlendPresets`/`Tempo` via `show.Save` | WIRED | `TestSceneLayerSetPreservesSelectionWhenOmitted` (WR-03 fix) independently re-verified passing |
| `command/playback.go` `playback evaluate` | `playback.Compile`/`Evaluate` | `--at` decomposed via `positionFromAt`, wrapped mod `BarsPerLoop` | WIRED | WR-01 fix (`positionFromAt` now takes `barsPerLoop` and wraps) confirmed in `internal/command/playback.go:293-300`; `TestPlaybackEvaluateWrapsBarIndexModuloBarsPerLoop` re-run and passes |
| `playback.Engine.tick` | `RecomputeEpoch` | BPM-differs-from-active-plan branch at bar-boundary crossing | WIRED (post CR-01 fix) | Directly read in `engine.go:143-155`; not reachable before the fix (confirmed absent in pre-fix review), present and exercised now |
| `internal/playback` | `internal/command` (adapters) | must NOT import | NOT IMPORTED (correct) | `grep` across `internal/playback`, `internal/scene`, `internal/programming` finds zero Go imports of `internal/command`; only a doc-comment textual reference in `clock.go` |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|---|---|---|---|---|
| PROG-01 | 03-01 | Select fixtures by pool/group/deployment instance/direct | SATISFIED | `selection.go` + tests |
| PROG-02 | 03-01 | Edit semantic attributes without raw DMX | SATISFIED | `programmer.go` + tests |
| PROG-03 | 03-01 | Programmer shows touched attrs/values/sources/scope | SATISFIED | `programmer inspect` route + tests |
| PROG-04 | 03-02 | Reusable themes/intensity/color/position/beam presets | SATISFIED (code) / **REQUIREMENTS.md not updated** | `theme.go`/`preset.go` + tests all pass; but `.planning/REQUIREMENTS.md` still shows `[ ]`/"Pending" -- see Gaps |
| PROG-05 | 03-03 | Reusable chases, ordered steps, tempo-relative timing | SATISFIED | `chase.go` + tests |
| PROG-06 | 03-03 | Reusable motion presets, position/beam only | SATISFIED | `motion.go` + tests |
| PROG-07 | 03-05 | Record/update/rename/reorder/duplicate/delete + undo/redo | SATISFIED | `history.go` + CRUD routes + tests |
| SCEN-01 | 03-04/03-06 | Scene loops configured bars against global BPM | SATISFIED | `scene.go` + `clock.go` + tests |
| SCEN-02 | 03-06 | Numeric BPM entry | SATISFIED | `playback bpm set` + tests |
| SCEN-03 | 03-06 | Tap-tempo BPM | SATISFIED | `playback bpm tap` + tests |
| SCEN-04 | 03-04 | Exactly one active scene | SATISFIED | `TestSingleActiveScene*` |
| SCEN-05 | 03-04 | Independently enabled layers | SATISFIED | `layer.go` + `ReduceLayers` tests |
| SCEN-06 | 03-07 | Immediate scene/layer switch (at next bar boundary) | SATISFIED | `engine.go` `SwitchScene`/`StageEdit` + tests |
| SCEN-07 | 03-04 | Reusable blending presets | SATISFIED | `blend.go` + tests |
| SCEN-08 | 03-06/03-07 | BPM-change preserve-or-restart | SATISFIED (post CR-01 fix, independently re-verified) | `RecomputeEpoch` wired in `engine.go`; `TestEngineBPMChangePreservesPosition`/`RestartsAtBarZero` pass |
| SCEN-09 | 03-07 | Deterministic under adapter delay/failure | SATISFIED | `TestDeterministicEvaluate*` pass under `-race`; structural isolation confirmed |

No orphaned requirements: every ID mapped to Phase 3 in `.planning/REQUIREMENTS.md`'s Traceability table (PROG-01..07, SCEN-01..09) appears in exactly one plan's `requirements:` frontmatter field, and every plan's declared requirements appear in REQUIREMENTS.md's Phase 3 mapping.

### Anti-Patterns Found

None. `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` grep across all phase-touched source files (`internal/programming`, `internal/scene`, `internal/playback`, `internal/command/programming.go`, `scene.go`, `playback.go`) returned zero matches.

### Code Review Fix Verification (independent re-check, not taken on the fix commit's word)

| Finding | File(s) | Independently Verified | Test Re-Run |
|---|---|---|---|
| CR-01 (BLOCKER): RecomputeEpoch never wired into Engine.tick | `internal/playback/engine.go` | YES — read `engine.go:136-160` directly; `tick()` now branches on `pending.BPM != plan.BPM` and calls `RecomputeEpoch(pending.PreserveOnBPMChange, ...)` before recomputing `pos` | `TestEngineBPMChangePreservesPosition`, `TestEngineBPMChangeRestartsAtBarZero` — both PASS, exercised through `StageEdit`+`tick`, not a direct `RecomputeEpoch` call |
| WR-01: `playback evaluate --at` didn't wrap BarIndex | `internal/command/playback.go` | YES — `positionFromAt(at float64, barsPerLoop int)` wraps and normalizes negative remainder | `TestPlaybackEvaluateWrapsBarIndexModuloBarsPerLoop` — PASS |
| WR-02: `Position` panics on `barsPerLoop <= 0` | `internal/playback/clock.go` | YES — `if barsPerLoop <= 0 { barsPerLoop = 1 }` guard present | `TestClockPositionNonPositiveBarsPerLoopDoesNotPanic` — PASS |
| WR-03: `scene layer set` wiped Selection on partial update | `internal/command/scene.go` | YES — `has*` flags + `targetScene.LayerByKind` carry-forward logic present at `runSceneLayerSet` | `TestSceneLayerSetPreservesSelectionWhenOmitted` — PASS |
| IN-01/IN-02 (INFO, out of scope) | `internal/playback/evaluate.go` | Confirmed left unfixed as declared (`fix_scope: critical_warning` excludes Info) | N/A — no regression introduced |

### Full Test Suite

`go build ./...` — clean.
`go test ./...` — all packages pass except the single pre-existing, phase-unrelated failure:
```
--- FAIL: TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline
    GOLC_MIGRATE_DRIFT: .../.planning/linear-map.json does not match the canonical schema-2 migration output
```
Independently confirmed out of Phase 3 scope: `git log --oneline -- internal/trace/catalog` shows the package's last touch was Phase 1 (`4a6a73d feat(01-09)`, `f9cf5d9 test(01-09)`), and no Phase 3 plan's `files_modified` list overlaps `internal/trace/catalog` or `.planning/linear-map.json`. This failure does not count against Phase 3 verification.

### Human Verification Required

None. All success criteria are fully verifiable through code inspection and automated tests; the phase is explicitly headless (no UI to visually inspect).

### Gaps Summary

One gap found, documentation-only: `.planning/REQUIREMENTS.md` was never updated to check off PROG-04 or mark its Traceability row "Complete," even though PROG-04 is fully implemented and tested (03-02-SUMMARY.md `requirements-completed: [PROG-04]`, all `internal/programming/theme*`/`preset*` tests passing). Every other plan in this phase has a matching REQUIREMENTS.md-updating docs commit; 03-02's docs commit (`f12e46a`) omitted this step.

**Resolved:** Fixed directly (commit `b416ee8`) — PROG-04 checked off and its Traceability row updated to "Complete" in `.planning/REQUIREMENTS.md`. No functional gap ever existed; the phase's goal is fully achieved.

---

_Verified: 2026-07-22T05:23:50Z_
_Verifier: Claude (gsd-verifier)_
