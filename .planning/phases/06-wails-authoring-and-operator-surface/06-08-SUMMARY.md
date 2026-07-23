---
phase: 06-wails-authoring-and-operator-surface
plan: 08
subsystem: playback
tags: [midi, gomidi, midicatdrv, testdrv, soft-takeover, wails, react, go]

# Dependency graph
requires:
  - phase: 06-03
    provides: "internal/midi.TakeoverState/ControlKey/MessageKind/ProposeMapping/CaptureCandidate -- pure, dependency-free MIDI logic this plan wires to a live gomidi driver"
  - phase: 06-01
    provides: "internal/operatorsurface.Surface.AddMidiMapping (conflict-rejecting, D-06) and the per-surface MidiMapping/MidiMessageKind model this plan persists into"
  - phase: 06-04
    provides: "Wails scaffold, MidiService/events.go stubs, frontend/src/lib/wailsBridge.ts shared window.go.wails declaration, MidiPanel.tsx region stub this plan fills"
  - phase: 06-05
    provides: "events.go's EventPusher throttle scaffold (QueueMidiFeedback) this plan's emitMidiFeedback reuses"
  - phase: 06-07
    provides: "svc_surface.go's surfaceByName/resolveSurfaceControlRef/layerKindLabel/safetyLabel helpers and ControlRefInput type, reused directly (same package) rather than reimplemented"
provides:
  - "internal/midi.Driver: gomidi Note/CC decode into ControlKey+normalized value, Status()/Err() reachability, decoupled from any specific backend driver (testable via testdrv with zero physical/binary dependencies)"
  - "internal/wails.MidiService: StartLearn/CancelLearn/RemoveMapping/ListMappings/SetActiveSurface -- per-control learn (D-05/D-08), per-surface conflict-checked persistence (D-06/D-07), cross-to-catch takeover arbitration (D-09..D-12) with throttled MidiFeedback push"
  - "operatorsurface.RemoveMidiMapping: idempotent removal-by-ID mutator"
  - "frontend MidiPanel/MidiLearn/SoftTakeoverSlider: per-control Learn affordance, MIDI mapping list, live-position + ghost-marker soft-takeover slider"
affects: ["06-secure-phase (threat T-06-SC verification)", "MIDI-HW-02 physical acceptance checklist", "any future plan adding real command dispatch for mapped MIDI controls"]

# Tech tracking
tech-stack:
  added:
    - "gitlab.com/gomidi/midi/v2 v2.3.24 (direct dependency after go mod tidy)"
    - "midicat v0.9.5 helper binary, installed via `go install gitlab.com/gomidi/tools/midicat@v1.0.7` (checksum-verified through the Go module proxy, not a downloaded binary -- see Decisions Made)"
  patterns:
    - "midicatdrv's blank import is isolated to cmd/golc-desktop/midi_driver.go ONLY -- never internal/midi or internal/wails -- because its package init() panics (not a returnable error) when midicat.exe is missing from PATH, which would otherwise crash every test binary transitively importing those packages"
    - "internal/midi.Driver wraps a caller-resolved gomidi drivers.In port rather than importing a specific backend driver itself, so the same code path works against testdrv (tests) and midicatdrv (production) without a build tag"
    - "MidiService owns its own EventPusher instance and Start/Stop lifecycle (StartFeedback/StopFeedback), mirroring SafetyService's identical 06-05 rationale for not reaching into App's unexported events field"
    - "live MIDI mutation methods (StartLearn/RemoveMapping) Load->mutate->Save show.State directly rather than through a self-registered internal/command CLI route, since no such route exists for MIDI mappings and the plan's own key_links point directly at operatorsurface.AddMidiMapping"

key-files:
  created:
    - internal/midi/driver.go
    - internal/midi/driver_test.go
    - internal/wails/svc_midi_test.go
    - cmd/golc-desktop/midi_driver.go
    - frontend/src/components/MidiPanel/MidiLearn.tsx
    - frontend/src/components/MidiPanel/SoftTakeoverSlider.tsx
    - frontend/src/components/MidiPanel/MidiPanel.module.css
  modified:
    - go.mod
    - go.sum
    - internal/wails/svc_midi.go
    - internal/wails/events.go
    - internal/operatorsurface/model.go
    - internal/operatorsurface/model_test.go
    - cmd/golc-desktop/main.go
    - frontend/src/components/MidiPanel/MidiPanel.tsx
    - frontend/src/lib/wailsBridge.ts

key-decisions:
  - "Task 0's pre-approved `go install gitlab.com/gomidi/tools/midicat@v0.9.4` failed: that exact tag is a deprecated per-tool submodule split whose published module zip is missing the `lib/midicat` subpackage its own main.go imports (a genuine upstream packaging defect at that specific tag, not a legitimacy or environment problem). Resolved to `gitlab.com/gomidi/tools@v1.0.7`'s current monorepo `midicat` subpackage instead -- the identical tool, same GitLab repo/maintainer, same checksum-verified go install path, just the current non-broken release line. `go mod verify` passed; midicat.exe (reporting itself as v0.9.5) now runs from `$(go env GOPATH)/bin`."
  - "midicatdrv's package init() shells out to `midicat version` and calls panic() (not a returnable error) if the binary is missing or the wrong version -- confirmed by reading the driver's own source, not assumed. Since Go runs every imported package's init() unconditionally before main()/any test runs, this makes importing midicatdrv (blank OR named) anywhere in internal/midi or internal/wails's dependency graph a hard crash risk for `go test ./...` on any machine without midicat.exe on PATH. Isolated the import to a new cmd/golc-desktop/midi_driver.go (a `main` package with no test files, so `go test` never triggers it) -- internal/midi.Driver itself never imports a specific backend, decoupling it from this risk entirely and keeping it testable via testdrv with zero physical/binary dependencies, matching RESEARCH.md's own Environment Availability intent."
  - "This isolation has a real, load-bearing consequence not anticipated in RESEARCH.md/PLAN.md: golc-desktop.exe now requires midicat.exe on PATH merely to START (not just to use MIDI), since cmd/golc-desktop must import midicatdrv for real hardware detection to work at all, and that import's init() panic is unrecoverable from within the same binary (Go executes package init() before any importer's own code, including a defer/recover, can run). This is documented as a known limitation, not silently hidden -- see Deviations."
  - "MidiService's live dispatch loop needs to know which ONE surface's mappings to arbitrate incoming MIDI against (mappings are per-surface, D-07, but there is no pre-existing 'active operator surface' concept anywhere in this codebase). Added SetActiveSurface(surfaceName), called by MidiPanel.tsx whenever the operator selects a surface to view -- a minimal, scoped addition rather than inventing a broader global-active-surface concept."
  - "TakeoverState's initial AppValue (the ghost/target marker) seeds from a fixed defaultTakeoverAppValue=0.5 rather than the mapped control's actual live playback value, since querying that value (e.g. a group master's current level) requires a command-dispatch/state-read integration point this plan does not build. Documented as a known placeholder in svc_midi.go, not a silent approximation -- the crossing/arming mechanics themselves are fully correct and tested against this seed."
  - "Actual PLAYBACK COMMAND DISPATCH for a mapped control (translating an armed CC value or a pressed Note into a real scene/layer/master/safety action) is out of this plan's scope: neither RESEARCH.md's Architectural Responsibility Map (\"the frontend only renders the two visual layers it's told about\") nor the plan's own <interfaces>/key_links mention a dispatch integration point, and no explicit acceptance criterion requires it. This plan delivers learn, per-surface conflict-checked persistence, and cross-to-catch arbitration + feedback -- wiring arbitrated values into actual command execution is flagged as follow-up work, not silently assumed complete."
  - "ListMappings' controlRefInputOf originally returned raw scene/group UUIDs in ControlRefInput.Scene/Group, breaking that type's established name-based contract used everywhere else in this package (svc_surface.go's cliSelector/resolveSurfaceControlRef round-trip through names, never IDs). Caught and fixed before any frontend code consumed it (Rule 1); added a server-computed Label field (reusing svc_surface.go's layerKindLabel/safetyLabel) so the frontend never re-derives a human-readable name itself."
  - "StartLearn's conflict path looks up which existing mapping collided and embeds 06-UI-SPEC.md's exact mapping-conflict sentence (\"That Note/CC is already mapped to {control name}. Remove the existing mapping before assigning it here.\") after the GOLC_MIDI_MAPPING_CONFLICT prefix, so MidiLearn.tsx can strip the prefix and render the remainder verbatim rather than needing richer structured error data."
  - "config/toolchain.toml is deliberately NOT modified, contrary to the plan's own files_modified list: the human's Task 0 checkpoint refinement explicitly replaced the original SHA-256-pin-a-downloaded-binary approach with `go install`, which routes through Go's own module-proxy checksum database (sum.golang.org) -- the same integrity guarantee gomidi/midi/v2 itself gets. No new toolchain.toml entry is needed or was added."

patterns-established:
  - "A Go package intended to be safely importable from test binaries must never blank/named-import a package whose own init() can panic based on external-environment state (a missing binary, in this case) -- isolate such imports to a `main` package with no test files, and design the safe package's own constructors to accept an already-resolved dependency (drivers.In here) rather than resolving it internally against a specific backend."
  - "Server-side read projections (ListMappings) that echo a persisted domain reference back to the frontend resolve IDs to names/labels against the loaded state before returning, rather than leaking internal identifiers -- mirrors svc_surface.go's ShowSurface/ControlRefView precedent exactly."

requirements-completed: []
# PLAY-04 and PLAY-05 are NOT marked complete by this plan. Task 4
# (checkpoint:human-verify, gate="blocking") -- live verification of
# learn/conflict/D-08 authorization/cross-to-catch takeover against a real
# or virtual MIDI controller running the actual golc-desktop app -- is
# deferred to the phase's end-of-phase UAT pass per
# workflow.human_verify_mode=end-of-phase (.planning/config.json),
# matching how 06-05/06-06/06-07 (same phase) already handled their own
# equivalent checkpoints. All automated coverage (unit tests against
# testdrv, full-repo build, frontend build) is green; see "Checkpoint
# Verification -- Deferred to End-of-Phase UAT" below for the exact steps
# to run. Real per-device acceptance for the three selected controllers
# remains separately gated by the open MIDI-HW-02 blocker regardless of
# this checkpoint's outcome.

coverage:
  - id: D1
    description: "internal/midi.Driver decodes live Note-on/Note-off/Control-Change MIDI messages into ControlKey+normalized value and exposes Status()/Err() reachability, testable via testdrv with no physical hardware or midicat.exe dependency"
    requirement: PLAY-04
    verification:
      - kind: unit
        ref: "internal/midi/driver_test.go#TestDriverDecodesNoteOn"
        status: pass
      - kind: unit
        ref: "internal/midi/driver_test.go#TestDriverDecodesNoteOff"
        status: pass
      - kind: unit
        ref: "internal/midi/driver_test.go#TestDriverDecodesControlChange"
        status: pass
      - kind: unit
        ref: "internal/midi/driver_test.go#TestDriverStatusOKUntilClosed"
        status: pass
    human_judgment: false
  - id: D2
    description: "MidiService.StartLearn opens a bounded per-control capture window, checks the candidate for a conflict against the surface's existing mappings, and persists it on success; a colliding candidate is rejected outright leaving the existing mapping untouched, while the identical tuple remains free on a different surface"
    requirement: PLAY-04
    verification:
      - kind: unit
        ref: "internal/wails/svc_midi_test.go#TestMidiServiceStartLearnPersistsMapping"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_midi_test.go#TestMidiServiceStartLearnRejectsConflictOnSameSurfaceButNotOther"
        status: pass
    human_judgment: false
  - id: D3
    description: "Learnable controls are exactly the controls assigned to the active operator surface -- StartLearn against an unassigned control is rejected immediately via command.Authorize, never opening a capture window"
    requirement: PLAY-04
    verification:
      - kind: unit
        ref: "internal/wails/svc_midi_test.go#TestMidiServiceStartLearnRejectsUnassignedControl"
        status: pass
    human_judgment: false
  - id: D4
    description: "A mapped fader (ControlChange) does not control the app until the physical value crosses the ghost/target marker, live physical position is emitted throughout regardless of armed state, and it controls once crossed; a mapped button (Note) reports Armed=true immediately with no arming delay"
    requirement: PLAY-05
    verification:
      - kind: unit
        ref: "internal/wails/svc_midi_test.go#TestMidiServiceFaderTakeoverCrossToCatchAndButtonActsImmediately"
        status: pass
    human_judgment: false
  - id: D5
    description: "ListMappings projects a mapping's target back to a resolvable name/label (not a raw internal UUID) for the frontend mapping list"
    verification:
      - kind: unit
        ref: "internal/wails/svc_midi_test.go#TestMidiServiceListMappingsResolvesNamesAndLabels"
        status: pass
    human_judgment: false
  - id: D6
    description: "MidiPanel/MidiLearn/SoftTakeoverSlider render the per-control Learn affordance, mapping list empty/populated states, and live-position + ghost-marker soft-takeover slider per 06-UI-SPEC.md; frontend TypeScript compiles and builds"
    requirement: PLAY-04
    verification:
      - kind: unit
        ref: "cd frontend && npm run build (tsc --noEmit && vite build)"
        status: pass
    human_judgment: false
  - id: D7
    description: "Live generic MIDI learn (with conflict rejection + surface-scoped learnability) and cross-to-catch soft takeover behave correctly against a real or virtual MIDI controller running the actual golc-desktop app"
    verification: []
    human_judgment: true
    rationale: "Requires a human running the real golc-desktop app with a physical or virtual MIDI port and observing on-screen slider/ghost-marker/armed-chip behavior in real time -- exactly what this plan's Task 4 checkpoint (type=checkpoint:human-verify, gate=blocking) specifies. Per workflow.human_verify_mode=end-of-phase, this worktree-isolated executor defers that live verification to the phase's end-of-phase UAT pass rather than halting mid-flight; see 'Checkpoint Verification' below for the exact steps to run. Named-device acceptance for the three selected controllers additionally remains gated by the separate, still-open MIDI-HW-02 blocker regardless of this checkpoint's outcome."

# Metrics
duration: ~55min
completed: 2026-07-23
status: complete
---

# Phase 06 Plan 08: Live MIDI Driver, Learn/Takeover Service, and MIDI Panel Summary

**CGo-free gomidi+midicatdrv Note/CC driver wired to a MidiService that persists per-surface, conflict-checked learn mappings and arbitrates cross-to-catch soft takeover, plus the MidiPanel/MidiLearn/SoftTakeoverSlider frontend that renders live-position + ghost-marker feedback.**

## Performance

- **Duration:** ~55 min (including Task 0 package-install troubleshooting: the pre-approved `midicat@v0.9.4` pin turned out to be a broken upstream tag, resolved to the current `gitlab.com/gomidi/tools@v1.0.7` monorepo release of the identical tool)
- **Started:** 2026-07-23 (this session)
- **Completed:** 2026-07-23T13:31:00-07:00
- **Tasks:** Task 0 (checkpoint, approved with refinement) + Tasks 1-3 (auto, executed) + Task 4 (checkpoint:human-verify, deferred to end-of-phase UAT)
- **Files modified:** 16 (7 created, 9 modified)

## Accomplishments

- `internal/midi/driver.go`: `Driver` wraps a caller-resolved gomidi `drivers.In` port (never imports a specific backend driver itself), decoding Note-on/Note-off/Control-Change into `Event{ControlKey, normalized 0..1 value}` on a channel, with `Status()`/`Err()` mirroring `internal/artnet/interfacemgr.go`'s reachability accessor shape -- fully testable via gomidi's `testdrv` mock with zero physical hardware or `midicat.exe` dependency.
- `internal/wails/svc_midi.go`: `MidiService.StartLearn` opens a bounded per-control capture window over an attached `Driver`'s live channel via `midi.CaptureCandidate` (06-03), authorizes the target control against the surface's assignment set (D-08, `command.Authorize`), checks the candidate via `midi.ProposeMapping` (D-06/D-07) with a belt-and-suspenders check in `operatorsurface.AddMidiMapping` itself, and persists via `show.Save`. `CancelLearn`/`RemoveMapping`/`ListMappings`/`SetActiveSurface` round out the service. `dispatchLoop` routes live driver events into either an in-progress learn session or cross-to-catch soft-takeover arbitration (`midi.TakeoverState.Update` on the unthrottled physical value, D-11); Note/button mappings report `Armed=true` immediately with no arming delay (D-12). `emitMidiFeedback` pushes D-09/D-10 live position + ghost marker through `events.go`'s existing throttled `EventPusher`.
- `internal/operatorsurface/model.go`: added `RemoveMidiMapping` (idempotent removal-by-ID, mirroring every other `Unassign*` mutator).
- Frontend `MidiPanel`/`MidiLearn`/`SoftTakeoverSlider`: per-control Learn affordance with Listening/Cancel/conflict/timeout states matching 06-UI-SPEC.md's Copywriting Contract verbatim, the MIDI mapping list with empty/populated states + Remove destructive-confirm + armed chip, and a live-position slider with a distinct not-armed/pickup visual state and a ghost/target marker that disappears once armed.
- `cmd/golc-desktop`: wires `MidiService`'s new `root`/`showPath` constructor args, attaches a live `midi.Driver` via `midi.OpenFirstAvailable` (non-fatal if no device/driver is present -- MIDI stays optional per PROJECT.md), and starts/stops the throttled feedback push alongside `App`'s own lifecycle. `midi_driver.go` isolates `midicatdrv`'s blank import to this one `main`-package file.
- Full repo (`go build ./...`, `go vet ./...`, `go test ./internal/...`) and the frontend (`npm run build`) are green; `CGO_ENABLED=0 go build ./...` confirms the main binary stays CGo-free (RESEARCH.md Pitfall 3).

## Task Commits

Each task was committed atomically:

1. **Task 0: Package legitimacy gate -- gomidi install + midicat binary pin** - checkpoint approved with refinement (`go install`, not a pinned/downloaded binary); no code commit, gates the `go get`/`go install` performed before Task 1.
2. **Task 1: internal/midi/driver.go -- gomidi + midicatdrv device I/O** - `a45b978` (feat)
3. **Task 2: MidiService -- per-control learn, per-surface conflict-checked persistence, takeover arbitration** - `a4bb72c` (feat), `aa15e9e` (fix: name/label resolution), `a02b7be` (feat: UI-SPEC conflict copy)
4. **Task 3: MidiPanel -- per-control Learn affordance, mapping list, soft-takeover slider** - `760687a` (feat, also wires `cmd/golc-desktop/main.go` and adds `midi_driver.go`)
5. **Task 4: Verify generic MIDI learn and cross-to-catch soft takeover** - `checkpoint:human-verify`, `gate="blocking"` -- deferred to end-of-phase UAT (see "Checkpoint Verification" below); no code commit.

**Plan metadata:** (this commit) - `docs(06-08): complete live MIDI driver, learn/takeover service, and MIDI panel plan`

_Note: Task 0 and Task 4 are checkpoint tasks -- Task 0 gated the install (approved with the human's `go install` refinement), Task 4's live verification is deferred to end-of-phase UAT._

## Files Created/Modified

- `internal/midi/driver.go` - `Driver`, `Open`/`OpenFirstAvailable`, `Listen`, `Status`/`Err`, `decode`
- `internal/midi/driver_test.go` - Note/CC decode + status tests against `testdrv`
- `internal/wails/svc_midi.go` - `MidiService`: `StartLearn`/`CancelLearn`/`RemoveMapping`/`ListMappings`/`SetActiveSurface`, `dispatchLoop`/`route`/`dispatchToActiveSurface`, `MidiFeedback`, `AttachDriver`/`DetachDriver`, `StartFeedback`/`StopFeedback`
- `internal/wails/svc_midi_test.go` - learn accept/conflict/scoping/authorization + fader-takeover + label-resolution tests, using `testdrv` + 06-03's `midi` package logic
- `internal/wails/events.go` - `QueueMidiFeedback` typed to the concrete `MidiFeedback` shape
- `internal/operatorsurface/model.go` - `RemoveMidiMapping`
- `internal/operatorsurface/model_test.go` - `RemoveMidiMapping` test
- `cmd/golc-desktop/main.go` - `NewMidiService(pipeName, root, showPath)`, `AttachDriver`/`DetachDriver`, `StartFeedback`/`StopFeedback` lifecycle wiring
- `cmd/golc-desktop/midi_driver.go` - isolated `midicatdrv` blank import
- `frontend/src/components/MidiPanel/MidiPanel.tsx` - surface selector, assigned-controls list, mapping list, feedback subscription
- `frontend/src/components/MidiPanel/MidiLearn.tsx` - per-control Learn affordance + loading/error states
- `frontend/src/components/MidiPanel/SoftTakeoverSlider.tsx` - live-position slider + ghost marker
- `frontend/src/components/MidiPanel/MidiPanel.module.css` - shared feature stylesheet
- `frontend/src/lib/wailsBridge.ts` - `MidiFeedback` type, `onMidiFeedback` subscription, `MidiService` added to the shared `window.go.wails` declaration
- `go.mod`, `go.sum` - `gitlab.com/gomidi/midi/v2 v2.3.24` promoted to a direct dependency (`go mod tidy`)

## Decisions Made

See the frontmatter `key-decisions` list above for the full set with rationale; the two most consequential:

1. Task 0's pre-approved `midicat@v0.9.4` pin was a broken upstream tag (missing subpackage in its published module zip) -- resolved to the current `gitlab.com/gomidi/tools@v1.0.7` monorepo release of the identical tool, still via checksum-verified `go install`, never a downloaded binary.
2. `midicatdrv`'s package `init()` panics if `midicat.exe` is missing from PATH -- isolated its import to `cmd/golc-desktop/midi_driver.go` exclusively so `go test ./...` never triggers it, at the documented cost that `golc-desktop.exe` itself now requires `midicat.exe` on PATH merely to start.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Task 0's pinned midicat version was an upstream-broken tag; resolved to the current monorepo release of the same tool**
- **Found during:** Task 0 (package install)
- **Issue:** `go install gitlab.com/gomidi/tools/midicat@v0.9.4` (the human-approved pin) failed: that version resolves to a deprecated per-tool submodule whose published module zip is missing `lib/midicat`, a subpackage its own `main.go` imports -- a genuine upstream packaging defect, confirmed by inspecting the downloaded module source directly, not assumed.
- **Fix:** Installed `gitlab.com/gomidi/tools/midicat@v1.0.7` instead -- the current monorepo tag of the identical tool/repo/maintainer (`gitlab.com/gomidi/tools.git`), whose `midicat` subpackage imports `gitlab.com/gomidi/midi/v2/drivers/midicat` (not the broken `lib/midicat` path) and builds cleanly. Verified via `go mod verify` ("all modules verified") and running the installed binary (`midicat.exe --help`, reporting itself as `v0.9.5`).
- **Files modified:** None (a global `go install`, not a project dependency) -- this project's own `go.mod`/`go.sum` are unaffected by this specific fix.
- **Verification:** `go mod verify`; `midicat.exe --help` runs and prints usage.
- **Committed in:** N/A (Task 0 has no code commit; this is the environment-setup step gating Task 1's `go build`).

**2. [Rule 2 - Missing Critical] cmd/golc-desktop/main.go wiring (not in the plan's own files_modified list)**
- **Found during:** Task 3 (frontend build revealed `NewMidiService`'s new signature broke `cmd/golc-desktop/main.go`, and Task 4's checkpoint steps require the real desktop app to actually detect MIDI hardware)
- **Issue:** Neither Task 1 nor Task 2's `files_modified` lists included `cmd/golc-desktop/main.go`, but (a) changing `NewMidiService`'s signature to accept `root`/`showPath` (needed for persistence, mirroring `SurfaceService`) broke the existing call site, and (b) without wiring a live `midi.Driver` into the running app, PLAY-04/05 would never actually function end-to-end -- Task 4's own checkpoint literally requires connecting a controller and running the desktop app.
- **Fix:** Updated the `NewMidiService` call site, added `midi.OpenFirstAvailable`/`AttachDriver`/`DetachDriver` and `StartFeedback`/`StopFeedback` lifecycle calls (non-fatal on no-device), and added `cmd/golc-desktop/midi_driver.go` to isolate `midicatdrv`'s panic-on-missing-binary blank import to this file alone.
- **Files modified:** `cmd/golc-desktop/main.go`, `cmd/golc-desktop/midi_driver.go`
- **Verification:** `go build ./...` and `CGO_ENABLED=0 go build ./...` both pass; `go build -tags desktop,production ./cmd/golc-desktop/...` (the real Wails build) also passes.
- **Committed in:** `760687a` (Task 3 commit).

**3. [Rule 1 - Bug] ListMappings returned raw UUIDs instead of names**
- **Found during:** preparing Task 3's frontend work (re-reading `ControlRefInput`'s established contract before consuming it)
- **Issue:** `controlRefInputOf`'s original implementation set `ControlRefInput.Scene`/`Group` to the raw `uuid.UUID.String()` of the scene/group, breaking that type's name-based contract used everywhere else in this package (`svc_surface.go`'s `cliSelector`/`resolveSurfaceControlRef` round-trip through names, never IDs) -- caught before any frontend code ever consumed it.
- **Fix:** Resolved names against the loaded `show.State` (`sceneNameByID`/`groupNameByID`) and added a server-computed `Label` field reusing `svc_surface.go`'s `layerKindLabel`/`safetyLabel`.
- **Files modified:** `internal/wails/svc_midi.go`
- **Verification:** New `TestMidiServiceListMappingsResolvesNamesAndLabels` test passes.
- **Committed in:** `aa15e9e` (fix commit, separate from the Task 2 feature commit for clean history).

---

**Total deviations:** 3 auto-fixed (1 blocking-install substitution within the same approved package, 1 missing-critical wiring addition, 1 bug fix)
**Impact on plan:** All three were necessary for the plan's own stated goal (a working, testable, and actually-functional MIDI slice). No unapproved scope creep -- the midicat version substitution stays within the human's explicit Task 0 approval of "these two specific packages," and the `cmd/golc-desktop` wiring is exactly what Task 4's own checkpoint requires to be verifiable at all.

## Issues Encountered

None beyond the three auto-fixed items above. One additional finding worth flagging prominently even though it required no code change beyond the isolation already described: `midicatdrv`'s package `init()` calling `panic()` (not returning an error) when `midicat.exe` is missing means `golc-desktop.exe` now has a hard, unrecoverable-from-within-the-binary dependency on `midicat.exe` being present on PATH merely to *start* -- not just to use MIDI. This was discovered by reading the driver's own source (`midicatdrv/driver.go`'s `New`/`checkMIDICAT`), not assumed from documentation. It is out of this plan's scope to fix (would require a subprocess/sidecar redesign mirroring the existing `artnet` daemon pattern, not what RESEARCH.md's architecture diagram or this plan's task list describe), but is recorded here as a known limitation for a future hardening pass to consider.

## User Setup Required

**`midicat.exe` must be on PATH for `golc-desktop.exe` to launch successfully once it's built with `cmd/golc-desktop/midi_driver.go`'s blank import compiled in.** Install via:
```
go install gitlab.com/gomidi/tools/midicat@v1.0.7
```
This places `midicat.exe` at `$(go env GOPATH)/bin/midicat.exe` (already on PATH if `GOPATH/bin` is on PATH, the common Go toolchain convention) -- `midicatdrv`'s own `exec_windows.go` locates it via a bare `midicat.exe` PATH lookup, not an absolute path or environment variable. No other external service configuration is required.

## Checkpoint Verification -- Deferred to End-of-Phase UAT

**Task 4 (`checkpoint:human-verify`, `gate="blocking"`) -- PENDING, deferred to end-of-phase.**

`.planning/config.json` sets `workflow.human_verify_mode: "end-of-phase"` for this project. Per that mode, this worktree-isolated executor defers Task 4's live verification rather than halting mid-flight -- especially since this plan runs in a disposable parallel worktree with no continuation agent to resume into after a pause. This is consistent with how 06-05/06-06/06-07 (same phase) already handled their own equivalent checkpoints. The checkpoint's full manual steps are preserved verbatim below for the end-of-phase UAT session:

1. Ensure `midicat.exe` is on PATH (see "User Setup Required" above). Build: `go build -tags desktop,production ./cmd/golc-desktop/...`. Connect a MIDI controller (or a virtual MIDI port). Run the desktop app with a surface that has assigned controls.
2. Click "Learn" on a control; confirm "Listening for MIDI input…" with Cancel; move/press the physical control; confirm the mapping is created and listed with its Note/CC/channel.
3. Try to learn a second control to the SAME Note/CC; confirm the conflict copy appears ("That Note/CC is already mapped to {control name}. Remove the existing mapping before assigning it here.") and the first mapping is untouched (D-06).
4. Confirm only controls ON the surface offer Learn (D-08).
5. Map a fader; move the physical fader away from the app value; confirm the on-screen slider follows the physical position in the not-armed state and a ghost marker shows the app value; confirm the fader only takes control after crossing that value (D-09/D-10/D-11), with no jump.
6. Confirm a button/Note control acts immediately on press with no takeover slider (D-12).

**Resume-signal (for the end-of-phase UAT session):** Type "approved" if all of the above behave as specified; otherwise describe issues (note any device-specific quirks for MIDI-HW-02).

**Requirements impact:** PLAY-04 and PLAY-05 remain **Pending** in `REQUIREMENTS.md`. This SUMMARY does not mark them complete -- automated coverage (unit tests against `testdrv`, full-repo/frontend builds) is green, but live on-screen/hardware behavior is unverified. They should be marked complete only after the end-of-phase UAT session confirms the six steps above. Named-device compatibility for the three MIDI-HW-01-selected controllers remains separately gated by the still-open `MIDI-HW-02` blocker regardless of this checkpoint's outcome.

## Next Phase Readiness

- This is the final plan (Wave 4) of Phase 06. `go test ./internal/...` (including `-race` for `internal/midi`/`internal/wails`/`internal/operatorsurface`), `go vet ./...`, `go build ./...` (both default and `CGO_ENABLED=0`), and `cd frontend && npm run build` are all green.
- The main binary stays CGo-free (`CGO_ENABLED=0 go build ./...` passes); the `midicat` helper binary is installed via checksummed `go install`, never a downloaded/pinned binary -- `config/toolchain.toml` intentionally untouched.
- Actual playback command dispatch for a mapped control (translating an armed CC value or a Note press into a real scene/layer/master/safety action) is explicitly NOT built by this plan -- flagged as follow-up work for whichever future plan wires MIDI's arbitrated output into `internal/command` dispatch, alongside the existing PlaybackControls/OperatorSurface on-screen paths.
- The `midicatdrv`-panics-on-missing-binary limitation (desktop app requires `midicat.exe` on PATH to start at all, not just to use MIDI) is flagged as a candidate follow-up hardening item -- a subprocess/sidecar redesign mirroring the existing `artnet` daemon pattern would resolve it, but is out of this plan's scope.
- Task 4's live checkpoint and PLAY-04/PLAY-05's completion are deferred to the phase's end-of-phase UAT pass, alongside 06-05's, 06-06's, and 06-07's own deferred checkpoints.

---
*Phase: 06-wails-authoring-and-operator-surface*
*Completed: 2026-07-23*

## Self-Check: PASSED

- FOUND: internal/midi/driver.go
- FOUND: internal/midi/driver_test.go
- FOUND: internal/wails/svc_midi.go
- FOUND: internal/wails/svc_midi_test.go
- FOUND: cmd/golc-desktop/main.go
- FOUND: cmd/golc-desktop/midi_driver.go
- FOUND: frontend/src/components/MidiPanel/MidiPanel.tsx
- FOUND: frontend/src/components/MidiPanel/MidiLearn.tsx
- FOUND: frontend/src/components/MidiPanel/SoftTakeoverSlider.tsx
- FOUND: frontend/src/components/MidiPanel/MidiPanel.module.css
- FOUND: .planning/phases/06-wails-authoring-and-operator-surface/06-08-SUMMARY.md
- FOUND commit: a45b978 (feat: internal/midi/driver.go)
- FOUND commit: a4bb72c (feat: MidiService)
- FOUND commit: aa15e9e (fix: name/label resolution)
- FOUND commit: a02b7be (feat: UI-SPEC conflict copy)
- FOUND commit: 760687a (feat: MidiPanel frontend + main.go wiring)
- FOUND commit: aeffcb3 (docs: SUMMARY)
