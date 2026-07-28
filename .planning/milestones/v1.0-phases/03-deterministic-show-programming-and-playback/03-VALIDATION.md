---
phase: 3
slug: deterministic-show-programming-and-playback
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-07-21
validated: 2026-07-22
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (stdlib), via the project's own `test` command route |
| **Config file** | none — `go test` driven by `_test.go` files; project-local scope markers follow the existing `TestScope{PascalName}` convention (`internal/command/test.go`) |
| **Quick run command** | `./golc.ps1 test --quick --scope <touched-scope>` |
| **Full suite command** | `./golc.ps1 test` |
| **Estimated runtime** | ~30 seconds (consistent with Phase 1/2 project-wide suite) |

---

## Sampling Rate

- **After every task commit:** Run `./golc.ps1 test --quick --scope <touched-scope>`
- **After every plan wave:** Run `./golc.ps1 test`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 03-01.1 | 03-01 | 1 | PROG-01 | T-03-05 / — | Selection resolves pool/group/deployment-instance/direct-fixture into a fixture-instance set | unit | `go test ./internal/programming/... -run TestSelection` | ✅ | ✅ green |
| 03-01.2 | 03-01 | 1 | PROG-02 | — | Semantic intensity/color/position/beam edit without raw DMX | unit | `go test ./internal/programming/... -run TestProgrammer` | ✅ | ✅ green |
| 03-01.2 | 03-01 | 1 | PROG-03 | — | Programmer surfaces touched attrs, values, sources, record scope | unit | `go test ./internal/programming/... -run TestProgrammerInspect` | ✅ | ✅ green |
| 03-02.1 | 03-02 | 2 | PROG-04 | — | Create/reuse themes and intensity/color/position/beam presets | unit | `go test ./internal/programming/... -run TestThemePreset` | ✅ | ✅ green |
| 03-03.1 | 03-03 | 3 | PROG-05 | — | Reusable chases with ordered steps + tempo-relative timing | unit | `go test ./internal/programming/... -run TestChase` | ✅ | ✅ green |
| 03-03.2 | 03-03 | 3 | PROG-06 | — | Reusable motion presets (position/beam only, per D-04) | unit | `go test ./internal/programming/... -run TestMotionPreset` | ✅ | ✅ green |
| 03-05.2 | 03-05 | 5 | PROG-07 | T-03-05 / — | Record/update/rename/reorder/duplicate/delete with undo/redo (D-12/13); live-active edit is ungated (D-08) | unit | `go test ./internal/command/... -run TestHistory` | ✅ | ✅ green |
| 03-04.1 / 03-06.1 | 03-04, 03-06 | 4, 5 | SCEN-01 | — | Scene loops for configured bar count against global BPM (model in 03-04; pure clock in 03-06) | unit | `go test ./internal/playback/... -run TestClockPosition` | ✅ | ✅ green |
| 03-06.2 | 03-06 | 5 | SCEN-02 | — | Numeric BPM entry | unit | `go test ./internal/command/... -run TestBPMSet` | ✅ | ✅ green |
| 03-06.1 | 03-06 | 5 | SCEN-03 | — | Tap-tempo BPM | unit | `go test ./internal/playback/... -run TestTapTempo` | ✅ | ✅ green |
| 03-04.1 | 03-04 | 4 | SCEN-04 | T-03-01 / mitigate | Exactly one active scene | unit | `go test ./internal/scene/... -run TestSingleActiveScene` | ✅ | ✅ green |
| 03-04.1 | 03-04 | 4 | SCEN-05 | — | Scene combines independently enabled layers via fixed-priority reduce | unit | `go test ./internal/scene/... -run TestLayerCombination` | ✅ | ✅ green |
| 03-07.2 | 03-07 | 6 | SCEN-06 | — | Immediate scene/layer switch (adopted at next-bar boundary, D-05/D-07); live-active edit ungated (D-08) | unit | `go test ./internal/playback/... -run TestImmediateSwitch` | ✅ | ✅ green |
| 03-04.2 | 03-04 | 4 | SCEN-07 | — | Reusable blending presets | unit | `go test ./internal/scene/... -run TestBlendPreset` | ✅ | ✅ green |
| 03-06.1 / 03-07.3 | 03-06, 03-07 | 5, 6 | SCEN-08 | — | BPM-change preserve-position-or-restart, applied uniformly to chases/motion (D-11) — primitive in 03-06's clock, wired into the running engine by 03-07 (CR-01 code-review fix) | unit | `go test ./internal/playback/... -run "TestBPMChangeEpoch\|TestEngineBPMChange"` | ✅ | ✅ green |
| 03-07.1 | 03-07 | 6 | SCEN-09 | T-03-04 / T-03-05 | Deterministic time-indexed output under simulated adapter delay/failure — byte-identical Evaluate across concurrent goroutines | unit + property | `go test ./internal/playback/... -run TestDeterministicEvaluate -race` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*Task IDs use the `{plan}.{1-based task position}` convention (STATE.md durable-ID grammar). Backfilled 2026-07-21 against the 7 committed PLAN.md files (03-01 through 03-07) after gsd-plan-checker verification passed; exact `-run` test names cross-checked against each plan's own `Test*` references. Re-verified 2026-07-22 post-execution: every command re-run directly against the built codebase (not taken on the plans' word); all pass. Two corrections found during this audit: (1) SCEN-02's command pointed at `internal/playback` but `TestBPMSet*` actually lives in `internal/command` — corrected. (2) SCEN-08 now also covers the `TestEngineBPMChange*` integration tests added by the code-review CR-01 fix (engine.go wiring `RecomputeEpoch` into `tick`), which the plan-time table predates.*

---

## Wave 0 Requirements

- [x] `internal/programming/*_test.go` — new package, no existing tests
- [x] `internal/scene/*_test.go` — new package, no existing tests
- [x] `internal/playback/*_test.go` — new package, no existing tests; includes `TestDeterministicEvaluateAcrossGoroutines` (property-style, feeds `Evaluate` the same `(plan, position)` pair via multiple goroutines under `-race`, asserts byte-identical `Frame` output) — the direct mechanical proof of SCEN-09
- [x] Framework install: none — Go `testing` used project-wide, no new framework needed

---

## Manual-Only Verifications

*None — all phase behaviors have automated verification per the map above. Windows-specific timer-resolution behavior (RESEARCH.md pitfall) is exercised indirectly through the deterministic-evaluate property test rather than requiring a separate manual step.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** verified 2026-07-22

---

## Validation Audit 2026-07-22

| Metric | Count |
|--------|-------|
| Gaps found | 1 (doc-only: SCEN-02 command pointed at wrong package) |
| Resolved | 1 (corrected in place; requirement was already covered by a passing test) |
| Escalated | 0 |

All 16 requirement rows re-run directly against the built codebase and confirmed green, including `-race` for SCEN-09. No auditor spawn needed — zero MISSING/PARTIAL requirements found on cross-reference.
