---
phase: 03-deterministic-show-programming-and-playback
plan: 02
subsystem: programming
tags: [go, cli, uuid, fixture-capability, theme, preset, show-state]

# Dependency graph
requires:
  - phase: 03-deterministic-show-programming-and-playback
    plan: 01
    provides: programming.ProgrammerState/Touched()/TouchedAttribute (the programmer buffer a preset records from) and the show.State/validate() extension pattern
provides:
  - programming.Theme/NewTheme/RenameTheme/ValidateThemeUniqueNames (PROG-04 reusable named color theme identity)
  - programming.Preset/PresetKind/NewPreset/RecordPresetFromProgrammer/RenamePreset/ValidatePresetUniqueNames/ValidatePreset (PROG-04 kind-scoped reusable presets recorded from programmer state)
  - show.State.Themes/Presets persistence and the "theme create"/"preset record" CLI scopes
affects: [03-03, 03-04, 03-05, 03-06, 03-07]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Theme/Preset identity/construction/rename/unique-name shape copied verbatim from internal/pool/model.go (uuid.NewV7()-minted ID, GOLC_{DOMAIN}_NAME_EMPTY guard, Rename mutates only Name)"
    - "PresetKind -> allowed-capability lookup mirrors internal/pool/model.go's supportedCapabilityTypes map-of-set pattern; RecordPresetFromProgrammer filters ps.Touched() through it rather than trusting the buffer's own scope"
    - "ValidatePreset re-checks both off-kind capability membership (GOLC_PRESET_OFF_KIND_ATTRIBUTE) and the normalized [0,1] bound (GOLC_PRESET_VALUE_OUT_OF_RANGE) at Load/Save time -- the persisted buffer is never trusted blindly, mirroring programming.validateAttribute's own re-check"

key-files:
  created:
    - internal/programming/theme.go
    - internal/programming/theme_test.go
    - internal/programming/preset.go
    - internal/programming/preset_test.go
    - internal/command/theme_preset_test.go
  modified:
    - internal/show/state.go
    - internal/command/programming.go

key-decisions:
  - "ColorAssignment (Theme.Colors) is declared with an InstanceID/Value shape ready for a future scene color-theme layer to populate, but this plan's 'theme create' route only mints the named, empty-Colors container -- no RecordThemeFromProgrammer function exists in the plan's artifacts_produced list, so Colors stays empty here by design, matching pool.Pool's zero-Members-at-creation convention."
  - "preset record's --kind is validated purely through programming.RecordPresetFromProgrammer/NewPreset's own GOLC_PRESET_KIND_INVALID check, never re-derived in the CLI arg parser -- an unrecognized --kind value is a valid string until the domain layer rejects it, exactly like pool.go's --requires capability list."
  - "runPresetRecord reads from a copied *state.Programmer value (or a zero-value ProgrammerState if the buffer is nil) rather than requiring a non-nil buffer -- recording against an empty programmer state succeeds with zero captured attributes rather than erroring, matching the plan's explicit 'empty-but-valid preset' behavior spec."

requirements-completed: [PROG-04]

coverage:
  - id: D1
    description: "A show author can create a reusable named color theme identity-stable and duplicate-safe against other themes."
    requirement: "PROG-04"
    verification:
      - kind: unit
        ref: "internal/programming/theme_test.go#TestThemePresetNewThemeMintsID"
        status: pass
      - kind: unit
        ref: "internal/programming/theme_test.go#TestThemePresetNewThemeEmptyNameRejected"
        status: pass
      - kind: unit
        ref: "internal/programming/theme_test.go#TestThemePresetRenamePreservesID"
        status: pass
      - kind: unit
        ref: "internal/programming/theme_test.go#TestThemePresetRenameThemeEmptyNameRejected"
        status: pass
      - kind: unit
        ref: "internal/programming/theme_test.go#TestThemePresetValidateThemeUniqueNamesRejectsDuplicate"
        status: pass
      - kind: unit
        ref: "internal/programming/theme_test.go#TestThemePresetValidateThemeUniqueNamesAcceptsDistinctNames"
        status: pass
    human_judgment: false
  - id: D2
    description: "A show author can record a kind-scoped (intensity/color/position/beam) preset from the current programmer buffer, capturing only that kind's allowed capabilities -- never off-kind or untouched attributes -- with an out-of-range captured value rejected by re-validation."
    requirement: "PROG-04"
    verification:
      - kind: unit
        ref: "internal/programming/preset_test.go#TestThemePresetNewPresetValidKindMintsID"
        status: pass
      - kind: unit
        ref: "internal/programming/preset_test.go#TestThemePresetNewPresetUnknownKindRejected"
        status: pass
      - kind: unit
        ref: "internal/programming/preset_test.go#TestThemePresetRecordPresetFromProgrammerFiltersOffKind"
        status: pass
      - kind: unit
        ref: "internal/programming/preset_test.go#TestThemePresetRecordPresetFromProgrammerZeroMatchesIsValidEmptyPreset"
        status: pass
      - kind: unit
        ref: "internal/programming/preset_test.go#TestThemePresetValidatePresetRejectsOutOfRangeValue"
        status: pass
      - kind: unit
        ref: "internal/programming/preset_test.go#TestThemePresetValidatePresetRejectsOffKindAttribute"
        status: pass
      - kind: unit
        ref: "internal/programming/preset_test.go#TestThemePresetValidatePresetAcceptsValidPreset"
        status: pass
      - kind: unit
        ref: "internal/programming/preset_test.go#TestThemePresetValidatePresetUniqueNamesRejectsDuplicate"
        status: pass
    human_judgment: false
  - id: D3
    description: "'theme create'/'preset record' CLI routes persist Themes/Presets on show.State through the existing atomic Save/Load round trip; a duplicate theme name and a missing --kind are both rejected before any mutation is saved."
    requirement: "PROG-04"
    verification:
      - kind: integration
        ref: "internal/command/theme_preset_test.go#TestThemePresetRoutes"
        status: pass
      - kind: integration
        ref: "internal/command/theme_preset_test.go#TestThemePresetPresetRecordMissingKindUsage"
        status: pass
      - kind: integration
        ref: "internal/command/theme_preset_test.go#TestThemePresetPresetRecordInvalidKind"
        status: pass
      - kind: integration
        ref: "internal/command/theme_preset_test.go#TestThemePresetShowStateRoundTrip"
        status: pass
      - kind: unit
        ref: "internal/show/... (go test ./internal/show/...)"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-07-22
status: complete
---

# Phase 3 Plan 02: Reusable Themes and Presets Summary

**Identity-stable color themes and kind-filtered intensity/color/position/beam presets, recorded from the 03-01 programmer buffer and persisted on show.State through `theme create`/`preset record` CLI routes.**

## Performance

- **Duration:** ~25 min (TDD RED/GREEN task pairs, this session)
- **Tasks:** 2
- **Files modified:** 7 (5 created, 2 modified)

## Accomplishments
- `programming.Theme`/`NewTheme`/`RenameTheme`/`ValidateThemeUniqueNames` copy `internal/pool/model.go`'s identity/construction/rename/unique-name shape verbatim: a UUIDv7 ID minted once at creation, never derived from Name, never re-minted by rename, and a duplicate name always rejected with `GOLC_THEME_DUPLICATE_NAME`.
- `programming.Preset`/`PresetKind`/`NewPreset`/`RecordPresetFromProgrammer`/`RenamePreset`/`ValidatePresetUniqueNames`/`ValidatePreset` implement the four kind-scoped preset types (intensity/color/position/beam), each mapped to its own disjoint `fixture.CapabilityType` set (position = pan/tilt; beam = zoom/focus/gobo/shutter/strobe per D-04). `RecordPresetFromProgrammer` filters a `ProgrammerState`'s touched attributes down to exactly the kind's allowed capabilities -- an off-kind touched attribute is silently excluded (never captured, never an error), and a preset that ends up capturing zero attributes is still a valid, empty-but-valid preset.
- `ValidatePreset` re-checks every captured attribute against both its kind's allowed-capability membership (`GOLC_PRESET_OFF_KIND_ATTRIBUTE`) and the normalized `[0,1]` bound (`GOLC_PRESET_VALUE_OUT_OF_RANGE`) -- a hand-tampered preset can never smuggle an off-kind or out-of-range value past Load/Save.
- `show.State` gains non-omitempty `Themes`/`Presets` fields, validated through the existing single `validate()` entry point (per-preset `ValidatePreset` plus `ValidateThemeUniqueNames`/`ValidatePresetUniqueNames`). The `theme`/`preset` CLI scopes (`theme create`, `preset record`) load-mutate-save through the existing atomic `show.Save`/`show.Load` round trip, matching `pool.go`'s `runPoolCreate` shape exactly.

## Task Commits

Each task was committed atomically (TDD RED/GREEN pairs):

1. **Task 1: Theme and Preset domain types (PROG-04)** - `376d006` (test, RED) -> `f1b0105` (feat, GREEN)
2. **Task 2: Persist Themes/Presets on show.State and expose theme/preset routes (PROG-04)** - `6431ec2` (test, RED) -> `588cc31` (feat, GREEN)

**Plan metadata:** (this commit, docs: complete plan)

## Files Created/Modified
- `internal/programming/theme.go` - `Theme`/`ColorAssignment` types, `NewTheme`/`RenameTheme`/`ValidateThemeUniqueNames` (PROG-04)
- `internal/programming/theme_test.go` - identity minting, empty-name rejection, rename-preserves-ID, duplicate/distinct-name validation coverage
- `internal/programming/preset.go` - `Preset`/`PresetKind`/`PresetAttribute` types, `presetKindCapabilities` mapping, `NewPreset`/`RecordPresetFromProgrammer`/`RenamePreset`/`ValidatePresetUniqueNames`/`ValidatePreset` (PROG-04)
- `internal/programming/preset_test.go` - kind-validation, off-kind-filter, empty-preset-is-valid, out-of-range/off-kind rejection, duplicate-name coverage
- `internal/show/state.go` - added `Themes []programming.Theme` (`json:"themes"`) and `Presets []programming.Preset` (`json:"presets"`) non-omitempty fields; extended `validate()` with per-preset `ValidatePreset` iteration plus `ValidateThemeUniqueNames`/`ValidatePresetUniqueNames`
- `internal/command/programming.go` - added `theme`/`preset` scopes with `theme create`/`preset record` routes: arg parsing, `programming.NewTheme`/`RecordPresetFromProgrammer` calls, save, and success-line output
- `internal/command/theme_preset_test.go` - end-to-end route coverage (theme create + preset record round trip with off-kind filtering verified through the CLI, duplicate-theme-name rejection, missing/invalid `--kind` rejection, direct show.State Themes/Presets round-trip)

## Decisions Made
- `ColorAssignment` (`Theme.Colors`) is declared with an `InstanceID`/`Value` shape ready for a future scene color-theme layer (03-04) to populate, but this plan's `theme create` route only mints the named, empty-`Colors` container. The plan's `artifacts_produced` list has no `RecordThemeFromProgrammer` function, so `Colors` stays empty by design at this plan's scope, mirroring `pool.Pool`'s zero-`Members`-at-creation convention.
- `preset record`'s `--kind` value is passed through as a raw string into `programming.PresetKind` and validated purely by `programming.NewPreset`/`RecordPresetFromProgrammer`'s own `GOLC_PRESET_KIND_INVALID` check -- the CLI arg parser never re-derives kind validity, exactly like `pool.go`'s `--requires` capability-list handling.
- `runPresetRecord` reads from a copied `*state.Programmer` value (or a zero-value `ProgrammerState` when the buffer is nil) rather than requiring a non-nil buffer first -- recording against an empty/absent programmer state succeeds with zero captured attributes rather than erroring, consistent with the plan's explicit "empty-but-valid preset" behavior spec.

## Deviations from Plan

None - plan executed exactly as written. `ValidatePresetUniqueNames` unit coverage and a few additional negative-path tests (empty-name rejection for both types, distinct-name acceptance, invalid-kind-at-CLI-layer) were added beyond the plan's literal behavior bullets as directly implied acceptance-criteria coverage, not as scope changes.

## Issues Encountered
- `go test ./...` (full repo suite) still reports the same pre-existing, unrelated failure documented in `03-01-SUMMARY.md` and `deferred-items.md`: `TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline` in `internal/trace/catalog` (`GOLC_MIGRATE_DRIFT` against `.planning/linear-map.json`). Confirmed out of this plan's scope: none of this plan's files (`internal/programming/theme.go`/`preset.go`, `internal/show/state.go`, `internal/command/programming.go`) overlap `internal/trace/catalog` or `.planning/linear-map.json`. All three of this plan's own scoped verification commands pass cleanly: `go test ./internal/programming/... -run TestThemePreset`, `go test ./internal/command/... -run TestThemePreset`, `go test ./internal/show/...`. No new deferred-items.md entry added since this is the identical, already-logged 03-01 issue.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- `programming.Theme`/`Preset`/`PresetKind` are ready for 03-03 (chases) and 03-04 (motion presets, scene color-theme/base-look layers) to reference by ID.
- `show.State.Themes`/`Presets` establishes the exact field/`validate()`-extension pattern for the remaining Phase 3 object types (`Chases`, `MotionPresets`, `Scenes`, `BlendPresets`).
- Blocker/concern: the pre-existing `internal/trace/catalog` linear-map drift failure (see Issues Encountered / `deferred-items.md`) remains untriaged and should be picked up by a future plan/phase.

---
*Phase: 03-deterministic-show-programming-and-playback*
*Completed: 2026-07-22*

## Self-Check: PASSED

- Verified all 7 created/modified files exist on disk (internal/programming/{theme,preset}.go + tests, internal/command/theme_preset_test.go, internal/command/programming.go, internal/show/state.go).
- Verified all 4 commit hashes exist in git log --oneline --all (376d006, f1b0105, 6431ec2, 588cc31).
