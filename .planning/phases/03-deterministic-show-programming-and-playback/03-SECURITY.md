---
phase: 03
slug: deterministic-show-programming-and-playback
status: verified
threats_open: 0
asvs_level: 1
created: 2026-07-22
---

# Phase 03 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| show file → domain model (`show.Load`) | Hand-editable `show.State` JSON crosses into the programming/scene/playback domains; nothing is trusted before whole-State validation passes. | Programmer buffer, themes/presets, chases/motion presets, scenes/layers/blend presets, tempo |
| CLI args → programmer/tempo mutation | Untrusted `--attr`/`--instance`/BPM/tap values cross into attribute editing and global tempo. | Semantic attribute values, BPM/tap timestamps |
| CLI args → object mutation (rename/reorder/duplicate/delete) | Untrusted mutation targets and index permutations cross into State mutation. | Object names, reorder index lists |
| programmer buffer → preset record | Captured attribute values re-validated at record time, not trusted from the persisted buffer. | Captured intensity/color/position/beam values |
| show file → compiled plan (`Compile`) | A hand-editable show document crosses into the engine at Compile; all-or-nothing validation rejects any partial/invalid plan before it can reach live output. | Full show.State → CompiledPlan |
| engine tick loop ↔ adapters (UI/persistence/scripts/API/LLM) | The real-time isolation boundary PROJECT.md names; every crossing is a non-blocking atomic Load/Store — no adapter can block or backpressure the tick loop. | Frame, CompiledPlan (atomic pointers only) |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-03-01 (03-01) | Tampering | `show.State.Programmer` buffer (dangling instance ref) | medium | mitigate | `ValidateProgrammer` extends `show.validate()` — `internal/programming/programmer.go:141` | closed |
| T-03-05 (03-01) | Tampering | programmer attribute value (intensity/color/position/beam) | high | mitigate | Validated against `fixture.Capability.Range` [0,1] and `SupportedCapabilityTypes`; `GOLC_PROGRAMMER_VALUE_OUT_OF_RANGE`/`GOLC_PROGRAMMER_CAPABILITY_UNSUPPORTED` — `internal/programming/programmer.go:89-99` | closed |
| T-03-01 (03-02) | Tampering | duplicate/empty theme-preset names | medium | mitigate | `ValidateThemeUniqueNames`/`ValidatePresetUniqueNames` extend `show.validate()` — `internal/programming/theme.go:64`, `preset.go:143` | closed |
| T-03-05 (03-02) | Tampering | preset captured attribute values | high | mitigate | `ValidatePreset` re-checks every captured value against [0,1] — `internal/programming/preset.go:163` | closed |
| T-03-01 (03-03) | Tampering | duplicate/empty chase-motion names | medium | mitigate | Unique-name validators extend `show.validate()` — `internal/programming/motion.go:138` | closed |
| T-03-02 (03-03) | Denial of Service | chase step count | high | mitigate | `maxChaseSteps=256` ceiling; `GOLC_CHASE_TOO_MANY_STEPS` — `internal/programming/chase.go:35,88-89` | closed |
| T-03-05 (03-03) | Tampering | motion-preset capability scope / keyframe values | high | mitigate | `ValidateMotionPreset` enforces D-04 position/beam-only scope + [0,1] bound — `internal/programming/motion.go:95` | closed |
| T-03-01a (03-04) | Tampering | scene layer reference (scene → theme/preset/chase/motion) | high | mitigate | `ValidateLayerReferences` extends `show.validate()` — `internal/scene/scene.go:261` | closed |
| T-03-01b (03-04) | Tampering | multiple active scenes | medium | mitigate | `ValidateSingleActiveScene` rejects >1 active scene (SCEN-04) — `internal/scene/scene.go:216` | closed |
| T-03-02 (03-04) | Denial of Service | scene bar-loop count | high | mitigate | `maxBarsPerLoop=1024` ceiling; `GOLC_SCENE_BARS_INVALID` — `internal/scene/scene.go:51,146-147` | closed |
| T-03-01 (03-05) | Tampering | duplicate/dangling names after rename/duplicate/delete | high | mitigate | Every mutating verb re-runs `show.validate()` through `show.Save` | closed |
| T-03-05 (03-05) | Tampering | reorder index permutation | medium | mitigate | `reorderChaseSteps` rejects a non-permutation order; `GOLC_CHASE_USAGE` — `internal/command/programming.go:1309-1327` | closed |
| T-03-04 (03-06) | Denial of Service (against playback) | clock position source | high | mitigate | `Position` is a pure function of monotonic elapsed time, no accumulated state, no adapter call — `internal/playback/clock.go` | closed |
| T-03-05 (03-06) | Tampering | BPM / tap-tempo input | high | mitigate | `ValidateBPM`/`TapTempo` reject non-positive/non-finite/out-of-band/<2-tap/zero-interval input; `maxBPM=999`; `GOLC_PLAYBACK_BPM_INVALID`/`GOLC_PLAYBACK_TAP_INVALID` — `internal/playback/clock.go:41,109,116-117,133` | closed |
| T-03-03 (03-07) | Tampering / Integrity | partially-valid compiled plan reaching the live engine | high | mitigate | `Compile` is all-or-nothing; `StageEdit` sets `pendingPlan` only on a fully valid compile, `GOLC_PLAYBACK_PLAN_INVALID` otherwise, never touching `activePlan` (D-06) — `internal/playback/compile.go`, `engine.go:173` | closed |
| T-03-04 (03-07) | Denial of Service (against playback) | adapter stalling/blocking the engine tick loop | high | mitigate | Structural isolation confirmed: `internal/playback` imports no `internal/command`/adapter package; all engine reads/writes use `atomic.Pointer` Load/Store, never blocking — `internal/playback/engine.go:67-70` | closed |
| T-03-05 (03-07) | Tampering | out-of-range attribute values reaching a Frame | high | mitigate | `Compile` re-validates every attribute value against [0,1] during flatten; an out-of-range value fails the whole compile — `internal/playback/compile.go` | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above `workflow.security_block_on` (high) count toward `threats_open`*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|--------------|------|
| T-03-SC | Tampering — package installs (npm/pip/cargo) | No new third-party packages introduced across any of the 7 plans in this phase (Go stdlib + already-verified `github.com/google/uuid`, confirmed via `git diff go.mod go.sum` showing zero changes); the package legitimacy gate does not apply. Asserted independently in each of 03-01 through 03-07's `<threat_model>` block. | GSD secure-phase (retroactive, plan-time disposition) | 2026-07-22 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-07-22 | 18 (17 mitigate + 1 consolidated accept) | 18 | 0 | Claude (orchestrator, L1 grep-depth verification — ASVS level 1, short-circuit per register_authored_at_plan_time: true) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-07-22
