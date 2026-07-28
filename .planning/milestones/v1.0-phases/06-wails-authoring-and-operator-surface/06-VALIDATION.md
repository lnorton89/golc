---
phase: 6
slug: wails-authoring-and-operator-surface
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-23
---

# Phase 6 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` + `-race` (existing project standard, `.planning/research/STACK.md`); no frontend test framework installed yet in this repo — Vitest/Playwright are STACK.md recommendations, not yet wired up (this is the first Wails-touching phase) |
| **Config file** | none yet — see Wave 0; new packages must declare `TestScope{PascalName}` markers (`internal/command/test.go` convention) before `./golc.ps1 test --quick --scope <name>` can target them |
| **Quick run command** | `go test ./internal/midi/... ./internal/operatorsurface/... ./internal/artnet/... ./internal/wails/... -run <Test>` (per-package/per-test; scoped `./golc.ps1 test --quick --scope <name>` wrapper available once Wave 0 declares scopes) |
| **Full suite command** | `./golc.ps1 test` (project-wide, equivalent to `go test -race ./...`) |
| **Estimated runtime** | ~30–45 seconds (consistent with Phase 1–5's project-wide suite baseline; Phase 6 adds ~4 new packages so this may grow modestly) |

---

## Sampling Rate

- **After every task commit:** Run the targeted package's `go test ./internal/<package>/... -run <Test>` (or the scoped wrapper once Wave 0 declares it)
- **After every plan wave:** Run `./golc.ps1 test`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | PLAY-01, PLAY-02 | — | Every playback action reachable via on-screen controls; a documented keyboard workflow reaches every playback action without requiring MIDI hardware | manual + smoke | Playwright/WebView2 smoke test (frontend test tooling not yet installed) | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | PLAY-03 | — / V4 Access Control | Operator surface visible-but-locked (D-04) is enforced server-side in the Go host/daemon command dispatch, never trusted from frontend-only hiding | unit | `go test ./internal/operatorsurface/... -run TestAssignment` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | PLAY-04, PLAY-05 | — / V5 Input Validation | Per-control MIDI learn (D-05) with conflict rejection (D-06), per-surface mapping scope (D-07), and cross-to-catch soft takeover restricted to continuous CC/fader controls only (D-11/D-12) | unit (`testdrv` mock driver) | `go test ./internal/midi/... -run TestLearn` / `-run TestTakeover` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | PLAY-06, PLAY-09 | — | Group masters, Grand Master, stop/release-all, and blackout flip Worker output within one Art-Net tick of the daemon safety flag being set, independent of a simulated slow/hung IPC client | integration | `go test ./internal/artnet/... -run TestSafetyOverride` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | PLAY-07 | — | Live status payload (active scene, enabled layers, BPM/bar position, controlling source, final output state) has a stable shape and is read via `Engine.CurrentFrame()`/daemon status, never a blocking poll | unit/contract + manual UAT | `go test ./internal/wails/... -run TestStatusPayload` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | PLAY-08 | — | Revoke Automation blocks and cancels queued non-manual-source commands, freezes the current look, and restores manual control even against a simulated non-responsive automation source | integration | `go test ./internal/artnet/... -run TestRevokeAutomation` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*Task IDs, Plan/Wave, and Threat Refs are TBD pending PLAN.md creation — to be backfilled by gsd-plan-checker/gsd-verifier once plans and the per-plan `<threat_model>` blocks exist, following Phase 5's own backfill precedent.*

---

## Wave 0 Requirements

- [ ] `internal/midi/` package (`driver.go`, `learn.go`, `takeover.go`) and its test files — covers PLAY-04/PLAY-05
- [ ] `internal/operatorsurface/` package (or `internal/show` extension) and its test files — covers PLAY-03
- [ ] `internal/artnet/safety.go` and its test files — covers PLAY-06/PLAY-08/PLAY-09
- [ ] `internal/wails/` package (`app.go`, `events.go`) and its test files — covers PLAY-01/PLAY-02/PLAY-07
- [ ] Frontend test tooling (Vitest/Playwright per STACK.md) — not yet installed anywhere in this repo; first Wails-touching phase, so this is expected, not a regression

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|--------------------|
| Real-hardware MIDI learn/soft-takeover against the three selected MIDI-HW-01 devices (exact button/fader semantics, bank identity, jitter) | PLAY-04, PLAY-05 | `testdrv` mock covers the learn/takeover logic; physical device behavior is gated by the still-open `MIDI-HW-02` blocker and requires the actual hardware | Follow `.planning/midi/MIDI-HW-02-CHECKLIST.md` per-device acceptance steps |
| Operator validation: information density, navigation, patch-to-playback speed vs. QLC+, constrained-surface learnability, cue-list needs, and the overall Wails/MIDI workflow | PLAY-01..09 (cross-cutting) | ROADMAP.md's Phase 6 Validation note explicitly requires human operator judgment on UX quality, not just functional correctness | Structured operator UAT session per ROADMAP.md Phase 6 "Validation" note |
| Global hotkey registration for the safety cluster (D-16) succeeding on a real Windows session without conflicting with another running app | PLAY-06, PLAY-08, PLAY-09 | `golang.design/x/hotkey`'s actual OS-level `RegisterHotKey` behavior can only be confirmed on a live Windows desktop session, not in a headless test | Launch the app, move focus to another application, verify each of the 3 safety-cluster shortcuts still fires |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
