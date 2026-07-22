---
phase: 3
slug: deterministic-show-programming-and-playback
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-21
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
| TBD | TBD | TBD | PROG-01 | T-03-05 / — | Selection resolves pool/group/deployment-instance/direct-fixture into a fixture-instance set | unit | `go test ./internal/programming/... -run TestSelection` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | PROG-02 | — | Semantic intensity/color/position/beam edit without raw DMX | unit | `go test ./internal/programming/... -run TestProgrammer` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | PROG-03 | — | Programmer surfaces touched attrs, values, sources, record scope | unit | `go test ./internal/programming/... -run TestProgrammerInspect` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | PROG-04 | — | Create/reuse themes and intensity/color/position/beam presets | unit | `go test ./internal/programming/... -run TestThemePreset` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | PROG-05 | — | Reusable chases with ordered steps + tempo-relative timing | unit | `go test ./internal/programming/... -run TestChase` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | PROG-06 | — | Reusable motion presets (position/beam only, per D-04) | unit | `go test ./internal/programming/... -run TestMotionPreset` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | PROG-07 | T-03-05 / — | Record/update/rename/reorder/duplicate/delete with undo/redo (D-12/13) | unit | `go test ./internal/programming/... -run TestHistory` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SCEN-01 | — | Scene loops for configured bar count against global BPM | unit | `go test ./internal/playback/... -run TestClockPosition` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SCEN-02 | — | Numeric BPM entry | unit | `go test ./internal/playback/... -run TestBPMSet` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SCEN-03 | — | Tap-tempo BPM | unit | `go test ./internal/playback/... -run TestTapTempo` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SCEN-04 | — | Exactly one active scene | unit | `go test ./internal/scene/... -run TestSingleActiveScene` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SCEN-05 | — | Scene combines independently enabled layers | unit | `go test ./internal/scene/... -run TestLayerCombination` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SCEN-06 | — | Immediate scene/layer switch | unit | `go test ./internal/playback/... -run TestImmediateSwitch` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SCEN-07 | — | Reusable blending presets | unit | `go test ./internal/scene/... -run TestBlendPreset` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SCEN-08 | — | BPM-change preserve-position-or-restart | unit | `go test ./internal/playback/... -run TestBPMChangeEpoch` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SCEN-09 | T-03-04 / T-03-05 | Deterministic time-indexed output under simulated adapter delay/failure | unit + property | `go test ./internal/playback/... -run TestDeterministicEvaluate` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*Task/Plan/Wave IDs are TBD until the planner assigns them — the plan-checker or a manual pass should backfill these columns once PLAN.md files exist.*

---

## Wave 0 Requirements

- [ ] `internal/programming/*_test.go` — new package, no existing tests
- [ ] `internal/scene/*_test.go` — new package, no existing tests
- [ ] `internal/playback/*_test.go` — new package, no existing tests; must include a property-style test that feeds `Evaluate` the same `(plan, position)` pair many times (and via multiple goroutines) and asserts byte-identical `Frame` output — the direct mechanical proof of SCEN-09
- [ ] Framework install: none — Go `testing` is already in use project-wide, no new framework needed

---

## Manual-Only Verifications

*None — all phase behaviors have automated verification per the map above. Windows-specific timer-resolution behavior (RESEARCH.md pitfall) is exercised indirectly through the deterministic-evaluate property test rather than requiring a separate manual step.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
