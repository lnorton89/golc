---
status: complete
phase: 03-deterministic-show-programming-and-playback
source: [03-01-SUMMARY.md, 03-02-SUMMARY.md, 03-03-SUMMARY.md, 03-04-SUMMARY.md, 03-05-SUMMARY.md, 03-06-SUMMARY.md, 03-07-SUMMARY.md]
started: 2026-07-22T05:45:21Z
updated: 2026-07-22T05:52:00Z
---

## Current Test

[testing complete]

## Tests

### 1. A show author can resolve a mixed pool/group/deployment-instance/direct-fixture selection into a deterministic, deduped set of concrete fixture instances, with dangling references rejected rather than silently dropped.
expected: covered by internal/programming/selection_test.go (13 tests)
result: pass
source: automated
coverage_id: D1 (03-01)

### 2. A show author can set semantic intensity/color/position/beam attributes on resolved instances as normalized [0,1] values (never raw DMX), with out-of-range values and unsupported capabilities rejected; the programmer buffer reports every touched attribute with value/source/record scope, stable order, no phantoms.
expected: covered by internal/programming/programmer_test.go (8 tests)
result: pass
source: automated
coverage_id: D2 (03-01)

### 3. `golc programmer set/inspect/clear` routes an author's CLI edit through selection resolution and attribute validation, persisting the buffer on show.State through the existing atomic Save/Load round trip.
expected: covered by internal/command/programming_test.go (4 tests) + internal/show tests. Live-verified: `programmer set --instance <id> --attr intensity=0.8` and `--attr color=0.3` both succeeded; `programmer inspect` reported both touched attributes with source=manual.
result: pass
source: automated
coverage_id: D3 (03-01)

### 4. A show author can create a reusable named color theme, identity-stable and duplicate-safe against other themes.
expected: covered by internal/programming/theme_test.go (6 tests). Live-verified: `theme create "Sunset"` succeeded.
result: pass
source: automated
coverage_id: D1 (03-02)

### 5. A show author can record a kind-scoped (intensity/color/position/beam) preset from the current programmer buffer, capturing only that kind's allowed capabilities, with an out-of-range captured value rejected by re-validation.
expected: covered by internal/programming/preset_test.go (8 tests). Live-verified: `preset record "Warm Wash" --kind intensity` captured 1 attribute from the programmer buffer.
result: pass
source: automated
coverage_id: D2 (03-02)

### 6. `theme create`/`preset record` CLI routes persist Themes/Presets on show.State through the existing atomic Save/Load round trip; a duplicate theme name and a missing --kind are both rejected before any mutation is saved.
expected: covered by internal/command/theme_preset_test.go (4 tests) + internal/show tests
result: pass
source: automated
coverage_id: D3 (03-02)

### 7. A show author can create a reusable chase with ordered steps and tempo-relative (bar- or beat-relative) step timing; step order is never reordered/deduped/randomized.
expected: covered by internal/programming/chase_test.go + internal/command/chase_motion_test.go (4 tests). Live-verified: `chase create "Sweep" --unit bar --step-duration 1` succeeded.
result: pass
source: automated
coverage_id: D1 (03-03)

### 8. A show author can create a reusable motion preset built only from position/beam semantic capabilities; a color or gobo/color-wheel capability is always rejected.
expected: covered by internal/programming/motion_test.go + internal/command/chase_motion_test.go (4 tests). Live-verified: `motion create "Pan Sweep"` succeeded.
result: pass
source: automated
coverage_id: D2 (03-03)

### 9. Chases/MotionPresets persist through show.Load/Save with unique-name enforcement, surfaced as chase create / motion create CLI routes.
expected: covered by internal/command/chase_motion_test.go (2 tests)
result: pass
source: automated
coverage_id: D3 (03-03)

### 10. Scene model with configured bar-loop length and four independently enabled, independently selectable layers (base-look/color-theme/chase/motion).
expected: covered by internal/scene/scene_test.go + internal/command/scene_test.go (2 tests). Live-verified: `scene create "Verse" --bars 4` succeeded.
result: pass
source: automated
coverage_id: D1 (03-04)

### 11. Fixed-priority layer reduce (base-look < color-theme < chase < motion), order-not-magnitude, disabled layers contribute nothing, per-layer selection scoping.
expected: covered by internal/scene/layer_test.go (4 tests)
result: pass
source: automated
coverage_id: D2 (03-04)

### 12. Exactly-one-active-scene invariant enforced at the domain model and through show.Save's whole-State validation.
expected: covered by internal/scene/scene_test.go + internal/command/scene_test.go (3 tests). Live-verified: after `scene activate "Verse"` then `playback switch "Chorus"`, raw show.json shows Verse.active=false and Chorus.active=true -- exactly one active scene at all times.
result: pass
source: automated
coverage_id: D3 (03-04)

### 13. Reusable blend presets describing transitions between scene/layer states, with duration/curve boundary validation and duplicate-name rejection.
expected: covered by internal/scene/blend_test.go + internal/command/scene_test.go (4 tests). Live-verified: `blend create "Quick Fade" --duration-bars 1` succeeded.
result: pass
source: automated
coverage_id: D4 (03-04)

### 14. Scenes/BlendPresets/Tempo persist on show.State through Load/Save, with layer-reference integrity (dangling reference rejection) enforced at the single validate() entry point.
expected: covered by internal/command/scene_test.go (2 tests)
result: pass
source: automated
coverage_id: D5 (03-04)

### 15. Session-only whole-session linear undo/redo history (programming.History): Record/Undo/Redo with redo-tail truncation, round-trip idempotency, empty-boundary no-crash errors.
expected: covered by internal/programming/history_test.go (5 tests)
result: pass
source: automated
coverage_id: D1 (03-05)

### 16. Full record/update/rename/reorder/duplicate/delete CLI surface across theme/preset/chase/motion/scene, persisting through show.Save.
expected: covered by internal/command/history_test.go#TestHistoryRoutes
result: pass
source: automated
coverage_id: D2 (03-05)

### 17. CRUD verbs succeed against an object referenced by (or, for scene duplicate, being) the currently-active scene with no pause/detach/lock precondition (D-08).
expected: covered by internal/command/history_test.go#TestHistoryLiveActiveEdit
result: pass
source: automated
coverage_id: D3 (03-05)

### 18. Pure musical-position clock: Position(now, bpm, barsPerLoop, loopStart) advances BarIndex/BeatFraction from monotonic elapsed time, wraps via modulo barsPerLoop, deterministic across repeated calls and concurrent goroutines, exact-boundary sample attributed to the new bar (floor semantics).
expected: covered by internal/playback/clock_test.go (4 tests)
result: pass
source: automated
coverage_id: D1 (03-06)

### 19. Operator can set global BPM by entering a numeric value via `playback bpm set <bpm> --show <path>`; re-setting the current value is idempotent; a non-numeric or <=0 value is rejected; a missing argument is rejected (exit 2).
expected: covered by internal/command/playback_bpm_test.go (5 tests). Live-verified: `playback bpm set 120` succeeded (GOLC_PLAYBACK_BPM_SET: 120).
result: pass
source: automated
coverage_id: D2 (03-06)

### 20. Operator can set global BPM through tap tempo via `playback bpm tap --at <ts>... --show <path>`; fewer than two taps is rejected and does not persist a change.
expected: covered by internal/playback/clock_test.go + internal/command/playback_bpm_test.go (6 tests). Live-verified: two taps 0.5s apart (`--at 2026-01-01T00:00:00.000Z --at 2026-01-01T00:00:00.500Z`) computed exactly 120 BPM (GOLC_PLAYBACK_BPM_TAP: 120); a single-tap attempt was correctly rejected with GOLC_PLAYBACK_TAP_INVALID.
result: pass
source: automated
coverage_id: D3 (03-06)

### 21. A global BPM change either preserves the active loop's musical position (RecomputeEpoch preserve=true keeps BarIndex/BeatFraction identical across the change) or restarts the loop at bar 0 (preserve=false).
expected: covered by internal/playback/clock_test.go#TestBPMChangeEpochPreservesPosition and #TestBPMChangeEpochRestartsAtBarZero, plus the engine-level integration tests TestEngineBPMChangePreservesPosition/TestEngineBPMChangeRestartsAtBarZero added by the code-review CR-01 fix (Engine.tick now wires RecomputeEpoch into the real adoption path, independently re-verified by the phase verifier).
result: pass
source: automated
coverage_id: D4 (03-06)

### 22. Compile(show.State) validates every scene->layer->theme/preset/chase/motion-preset reference and every resolved attribute value, flattening the single active scene into an immutable CompiledPlan all-or-nothing -- never a partial plan, never mutates its input State.
expected: covered by internal/playback/compile_test.go (6 tests)
result: pass
source: automated
coverage_id: D1 (03-07)

### 23. Evaluate(plan, pos) is a pure fixed-priority layer reduce producing byte-identical Frames across repeated and concurrent calls with the same (plan, pos) -- the mechanical proof of SCEN-09.
expected: covered by internal/playback/evaluate_test.go (6 tests, including -race). Live-verified: `playback evaluate --at 0.0` run twice against the same show file produced byte-identical output both times (`GOLC_PLAYBACK_EVALUATE: bar=0 beat_fraction=0 instances=0`).
result: pass
source: automated
coverage_id: D2 (03-07)

### 24. The engine adopts a staged edit or scene switch atomically at the next bar boundary (never mid-bar), keeps running the last valid plan on a rejected edit, requires no lock/pause/detach precondition, and is delay-deterministic and race-clean under concurrent CurrentFrame reads.
expected: covered by internal/playback/engine_test.go (6 tests, including -race). Note: the real-time Engine tick loop is not yet wired into a live CLI/adapter route in this phase (confirmed: no `internal/command` route constructs an Engine) -- this is proven at the engine-object level via the automated suite, matching the code review's own finding. Wiring the engine into a running process is Phase 4 (Art-Net output) / Phase 6 (Wails UI) scope.
result: pass
source: automated
coverage_id: D3 (03-07)

### 25. `playback evaluate --at <bar>.<beatfraction> [--json] --show <path>` compiles the active scene and prints the deterministic Frame; a show with no active scene or an invalid compile exits non-zero. `playback switch <scene> --show <path>` activates a scene and saves; an unknown scene is rejected.
expected: covered by internal/command/playback_engine_test.go (7 tests). Live-verified: `playback switch "Chorus"` activated Chorus and deactivated Verse (confirmed via raw show.json); `playback evaluate --at 0.0` printed a deterministic summary line.
result: pass
source: automated
coverage_id: D4 (03-07)

## Summary

total: 25
passed: 25
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

None.
