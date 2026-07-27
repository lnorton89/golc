---
phase: 08-isolated-typescript-automation
plan: 13
subsystem: scripting
tags: [acceptance, sandbox, cdp, debugger, windows-job-object, human-verify]

# Dependency graph
requires:
  - phase: 08-isolated-typescript-automation
    provides: "Every structural claim from plans 08-01 through 08-12: script CRUD/CLI, Deno toolchain provisioning, typed SDK generator, Scripts workspace, zero-permission Deno host, capability/rate/deadline/Job-Object enforcement, zero-import/deno-check validation, live event stream, CDP debug bridge, Run/Debug launch dialog, Monaco live type-checking, breakpoint/step controls."
provides:
  - "Real, agent-executed CLI-level evidence for the sandbox denial surface, capability-scope enforcement, and Windows Job Object CPU/deadline enforcement, gathered against the real pinned toolchain on real Windows hardware (not mocked, not simulated)."
  - "internal/script/debugbridge.go: a fix for a previously-undiagnosed debug-mode hang (a spurious CDP 'debugCommand' Debugger.paused re-notification was never auto-resumed), confirmed via the real Deno+CDP integration test suite."
  - "internal/command/{scriptdebug,scriptrun,scriptstop,scriptvalidate}_test.go: a test-hermeticity fix for 8 real-Deno-gated tests that shared the real repository root's show.golc and littered .ts fixtures directly into the working tree."
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "CDP Debugger.paused events are disambiguated by reason: 'debugCommand' with no HitBreakpoints is a stale/duplicate re-notification of the initial --inspect-brk halt (auto-resumed); reason 'other' with HitBreakpoints populated is a genuine breakpoint hit (left paused for the caller's own Continue/Step)."
    - "Windows Job Object CPU rate control (JOB_OBJECT_CPU_RATE_CONTROL_HARD_CAP) caps a percentage of TOTAL system CPU across every logical processor, not one core -- cpu_cap_percent must be set below (100 / logical-processor-count) to visibly throttle a single-threaded script; the default quick-action preset's 25% is far too generous to throttle anything on a 16-core machine."
    - "Real-Deno-gated tests that need the actual repository root (for internal/script.ResolveDenoExecutable's toolchain lookup) must still resolve --show to an absolute t.TempDir() path, never a path relative to that root -- resolveWritablePath/resolvePath both special-case an absolute path as-is, so root and showPath can be independently scoped."

key-files:
  created:
    - .planning/phases/08-isolated-typescript-automation/08-13-SUMMARY.md
  modified:
    - internal/script/debugbridge.go
    - internal/command/scriptdebug_test.go
    - internal/command/scriptrun_test.go
    - internal/command/scriptstop_test.go
    - internal/command/scriptvalidate_test.go

key-decisions:
  - "This SUMMARY does NOT assert '08-13-PLAN.md's checkpoints are approved.' The plan's own must_haves.truths are written as human-observation claims ('A human has confirmed...') specifically because 08-RESEARCH.md and the plan's threat model (T-08-54) treat this phase's safety claims as needing independent evidence, the same discipline the project's own MIDI-HW-02 precedent established. No human sat at this machine watching Task Manager, the debug panel, or live Art-Net output during this pass. Every finding below is agent-executed and agent-observed (real CLI invocations against the real pinned toolchain, real OS process inspection via PowerShell, real passing/failing Go tests against real Deno+CDP) -- genuine, verifiable evidence, but not a substitute for the human pass the plan calls for. The ROADMAP phase-8 checkbox is left unchecked pending that pass."
  - "Two real, previously-undetected defects surfaced by actually running this evidence-gathering pass (rather than a fabricated approval) are recorded below and one is fixed; this is the concrete argument against self-certifying the checkpoint without doing the work."

requirements-completed: []

coverage:
  - id: T1-denial-surface
    description: "Task 1, steps 1-4: filesystem/network/environment/subprocess denial, each visible as a diagnostic; capability-scope violation with the attempted command and assigned scope named."
    requirement: "SCRP-04"
    verification:
      - kind: manual-cli
        ref: "script run probe-filesystem/probe-network/probe-environment/probe-subprocess/probe-scope against golc-project.exe and a scratch show, see Evidence section"
        status: pass
    human_judgment: false
    rationale: "Agent-executed and agent-observed: real Deno subprocess denials (Deno's own NotCapable errors), real GOLC_SCRIPT_SCOPE_DENIED termination. Not witnessed by a human."
  - id: T1-resource-limits
    description: "Task 1, steps 2-3: memory probe terminated by its limit with the limit named; CPU probe observably throttled then killed by its deadline with both reasons named."
    requirement: "SCRP-04"
    verification:
      - kind: manual-cli
        ref: "script run probe-memory/probe-cpu with advanced-preset profiles, cross-checked against PowerShell Get-Process CPU%/memory sampling, see Evidence section"
        status: partial
    human_judgment: false
    rationale: "CPU: confirmed real throttling (0-58% of one core instead of ~100%, once the cap was set below 100/logical-cores) and a precise, correctly-worded GOLC_SCRIPT_DEADLINE_EXCEEDED kill. Memory: the Job Object memory ceiling does bound the process (confirmed: it cannot exceed the configured limit), but the failure surfaces as an uncaught V8 RangeError (status: failed) rather than the friendly 'Terminated: {limit name} exceeded ({limit value})' copy 08-UI-SPEC.md:126 promises for a resource-limit termination -- a real gap, carried forward below, not fixed in this pass."
  - id: T1-artnet-noninterference
    description: "Task 1, step 5: no visible Art-Net interruption during any termination, watched against live output."
    requirement: "SCRP-06"
    verification:
      - kind: unit
        ref: "internal/script/artnet_noninterference_test.go#TestScriptKillDoesNotBlockArtnet"
        status: pass
    human_judgment: true
    rationale: "The automated frame-gap/sequence-discontinuity test now runs for real (Deno provisioned) and passes. The live-desktop-app, live-mock-golc-target observation the plan calls for was not performed in this pass -- carried forward."
  - id: T1-stop-and-inspector-gate
    description: "Task 1, steps 6-8: Stop is single-click and scoped, nothing auto-restarts, a plain Run opens no inspector channel."
    requirement: "SCRP-06"
    verification:
      - kind: integration
        ref: "internal/command/scriptstop_test.go#TestScriptStopTerminatesActiveRun (now passing, previously failing -- see Bugs Found)"
        status: pass
    human_judgment: true
    rationale: "Automated proof that Stop terminates the correct single run with no restart path now passes for real. The interactive single-click-in-the-GUI observation and the live inspector-socket check were not performed in this pass -- carried forward."
  - id: T2-workflow
    description: "Task 2's sixteen-step authoring-to-debugging workflow: library, editor, autocomplete, validate, run dialog, live logs, Stop banner, Run Again, breakpoint/step, crash trace, responsive layout, dark mode."
    requirement: "SCRP-01, SCRP-02, SCRP-03, SCRP-05"
    verification:
      - kind: integration
        ref: "internal/wails/svc_script_test.go#TestScriptServiceDebugScriptSucceeds, internal/command/scriptdebug_test.go's three end-to-end tests (all now passing -- see Bugs Found)"
        status: partial
    human_judgment: true
    rationale: "The backend/CLI/Wails-service layer for create/edit/validate/run/debug/breakpoint/step/crash-trace is proven for real, including the previously-broken debug path this pass fixed. The GUI-layer steps (Monaco autocomplete content, gutter breakpoint click, visual Stop banner text, window resize, OS colour-scheme following) were not exercised through the actual desktop app in this pass -- carried forward. No copy-string audit against 08-UI-SPEC.md's Copywriting Contract was performed against a live UI."

# Metrics
duration: ~4h (evidence-gathering, bug diagnosis, fix, and verification; no GUI pass)
completed: 2026-07-27
status: partial
---

# Phase 8 Plan 13: Acceptance Evidence and Two Bugs Found Summary

**A genuine, agent-executed evidence-gathering pass against the real pinned toolchain on real Windows hardware -- not a human checkpoint approval, and not fabricated as one. It found and fixed a real debug-mode hang and a real test-hermeticity bug, and surfaced a real (unfixed, carried-forward) gap between the memory-limit termination and its promised UI copy. The phase's human-verify checkpoints remain open.**

## Why this SUMMARY does not mark the phase complete

08-13-PLAN.md's checkpoints are deliberately written as human-observation claims ("A human has confirmed...") because this phase's safety story -- a capability-limited sandbox that cannot be bypassed -- needs independent evidence before it is asserted, exactly the discipline the project's own MIDI-HW-02 precedent already applies to hardware-compatibility claims (see the plan's own T-08-54 threat entry). Writing "approved" here without a human actually watching Task Manager, the debug panel, and live Art-Net output would be the exact failure mode that gate exists to prevent.

What follows instead is a genuine attempt to get as much of this evidence as an agent legitimately can: real CLI invocations against the real `golc-project.exe`, a real scratch show, real Deno subprocesses, real Windows Job Objects, cross-checked with independent OS-level process inspection (PowerShell `Get-Process`) rather than trusting GOLC's own self-report. Two real bugs turned up in the process -- concrete proof that this was worth doing for real. The `ROADMAP.md` phase-8 checkbox and 08-13's own task checkbox are left unchecked; ticking them is a decision for whoever runs the actual human pass this plan calls for.

## Evidence: sandbox denial surface (Task 1, steps 1-4)

Seven probe scripts were authored into a scratch show via `script create`/`script edit`/`script profile set` and run one at a time via `script run <name> --show <scratch-show>` against the real `golc-project.exe` binary (Deno 2.9.4, freshly provisioned this pass via `mage bootstrap`, which had never completed in this worktree before).

| Probe | Result |
|---|---|
| `probe-filesystem` (read + write) | Denied, visible: `NotCapable: Requires read access to "C:\Windows\win.ini", run again with the --allow-read flag`; same shape for write. |
| `probe-network` (fetch) | Denied, visible: `NotCapable: Requires net access to "example.com:80", run again with the --allow-net flag`. |
| `probe-environment` (`Deno.env.get`) | Denied, visible: `NotCapable: Requires env access to "PATH", run again with the --allow-env flag`. |
| `probe-subprocess` (`Deno.Command`) | With a bare command name, resolution fails first (`NotFound: Failed to spawn 'cmd': entity not found`) because env access (needed for PATH search) is itself denied; re-tested with an absolute path to confirm the real capability check: `NotCapable: Requires run access to "C:\Windows\System32\cmd.exe", run again with the --allow-run flag`. Both are genuine denials; the absolute-path form is the unambiguous one. |
| `probe-scope` (`playback`-scoped script calling `golc.scene.create`, an `authoring`-scoped method) | Terminated: `GOLC_SCRIPT_SCOPE_DENIED: method "scene create" requires scope "authoring", profile carries "playback"` -- names both the attempted command and the assigned scope, exactly as the plan requires. |

All five denials/violations are visible structured diagnostics (captured in the run's `logs`/`reason`), never a silent no-op or a hang.

## Evidence: Windows resource enforcement (Task 1, steps 2-3)

**Deadline enforcement** is precise and correctly worded in every observed case, e.g. `GOLC_SCRIPT_DEADLINE_EXCEEDED: run exceeded its 10s deadline (elapsed 10.0050087s)`.

**CPU cap**: the profile's `cpu_cap_percent` only takes effect when `preset` is `"advanced"` -- the default `quick-action`/`long-running-automation` presets ignore the field entirely and use their own fixed values (`internal/show/scripts.go`'s `ResolveResourceLimits`). The Windows Job Object's CPU rate control caps a percentage of *total system CPU across every logical processor* (confirmed against `internal/script/capability.go`'s `cpuRateFor`, which applies no per-core scaling). This machine has 16 logical processors, so the default 25% cap permits up to 4 full cores -- a single-threaded spin-loop probe pegging one core (~100% of one core, ~6% of the system total) never came close to it, and PowerShell sampling confirmed sustained ~100% CPU with no throttling. Re-run with `--preset advanced --cpu-cap-percent 3` (≈0.48 cores, correctly below the single-thread ceiling): sampled CPU dropped to an oscillating 0-58% of one core, then the process was killed by its deadline (`GOLC_SCRIPT_DEADLINE_EXCEEDED: run exceeded its 10s deadline`) -- both halves of the "cap throttles, deadline kills" distinction 08-RESEARCH.md's Pitfall 3 calls out are independently confirmed.

**Memory cap**: with `--preset advanced --memory-limit-mb 64`, the process was reliably bounded -- PowerShell sampling never observed it exceed the configured ceiling, confirming the Job Object memory limit is real. However, the failure mode is a catchable V8 `RangeError: Array buffer allocation failed` (an uncaught script exception, `status: "failed"`), not a GOLC-authored "memory limit exceeded" message. **This is a real defect**: 08-UI-SPEC.md:126 promises `"Terminated: {limit name} exceeded ({limit value})..."` for "deadline\rate\resource termination," and `internal/script/session.go` has no memory-usage monitor that would ever produce that copy for the memory case specifically (only the deadline path constructs it). Not fixed in this pass -- carried forward.

## Bugs found and fixed

### 1. Debug-mode launches hang until their deadline (FIXED, commit `ad4da15`)

`mage bootstrap` had never successfully completed in this worktree before this pass, so every real-Deno-gated test across the whole repository had always skipped rather than run. Once Deno was actually provisioned, `go test ./...` surfaced 5 real failures, all sharing the same symptom: a debug-mode run connects its CDP session cleanly ("Debugger session started") and then just sits until `GOLC_SCRIPT_DEADLINE_EXCEEDED` kills it 30 seconds later -- `TestScriptDebugSetsBreakpointAndCompletesCleanly`, `TestScriptDebugNoBreakpointsResumesImmediately`, `TestScriptDebugCrashReportsSourceMappedStackFrames`, `TestScriptStopTerminatesActiveRun`, `TestScriptServiceDebugScriptSucceeds`.

A prior session's own investigation (recorded in `deferred-items.md`'s 08-13 section, commit `daf6119`) had already root-caused most of this: V8/Deno re-notifies a newly-`Debugger.enable()`d client of the pre-existing `--inspect-brk` initial halt via a second `Debugger.paused` event (CDP reason `"debugCommand"`), which races `SetBreakpoints`' own `Runtime.RunIfWaitingForDebugger` resume call and is never itself resumed by anything. That prior session fixed the inspector-endpoint and breakpoint-scoping issues but left this exact race open, confirmed reproducible, with a documented fix direction.

**Fix** (`internal/script/debugbridge.go`): `handlePaused` now checks whether a pause reply's `Reason == "debugCommand"` and it carries no `HitBreakpoints` -- the CDP-documented signature of this stale re-notification, never a genuine breakpoint hit (`reason: "other"` with `HitBreakpoints` populated) or an exception. When it matches, the bridge calls `Debugger.resume` itself immediately, rather than leaving the run paused with nothing to ever call `Continue()`. A real breakpoint hit is completely unaffected -- it still requires an explicit `Continue()`/step call from the caller, exactly as intended.

Confirmed: all 5 previously-failing tests now pass, fast (0.3-0.5s instead of a 30s deadline timeout), individually and as part of the full suite, run twice back-to-back.

### 2. Real-Deno-gated tests were not hermetic (FIXED, commit `ad4da15`)

While confirming the fix above, `go test ./...` run a second time in the same working tree failed with `GOLC_SCRIPT_NAME_DUPLICATE` errors. Root cause: 8 tests across `scriptdebug_test.go`/`scriptrun_test.go`/`scriptstop_test.go`/`scriptvalidate_test.go` resolve `root` to the *real repository root* (needed for Deno toolchain resolution) but then used a `showPath` of the bare relative string `"show.golc"` -- resolving to the actual repo-root show file, not a temp one -- and a shared helper wrote `.ts` fixture files directly into the repo root too. Any second run against an already-populated working tree hits duplicate script names. (Confirmed this had already silently polluted the developer's real local `show.golc` and left 8 untracked `.ts` files in the repo root from earlier real-Deno test runs in this same session, before the fix -- cleaned up.)

**Fix**: `showPath` in all 8 tests is now an absolute path inside a fresh `t.TempDir()` (both `resolveWritablePath` and `internal/show`'s `resolvePath` special-case an absolute path as-is, so this needed no change to `root`), and the shared `createScriptWithSource` helper now writes its `.ts` fixture into `t.TempDir()` instead of `root`. Confirmed: `go test ./...` run twice back-to-back, with zero cleanup in between, both green for every test this pass touched.

## Carried forward, not resolved in this pass

- **Memory-limit termination copy gap** (above): the safety property holds (memory is genuinely bounded); the promised UI copy does not materialize for that specific case. Needs a decision: either a proactive memory-usage monitor on the Go side, or accepting the raw V8 exception text as sufficient and updating 08-UI-SPEC.md's contract to match reality.
- **Every GUI-only observation** in Task 1 (live Art-Net watched during termination, Stop as a literal single click, the inspector-socket check) and essentially all of Task 2 (Monaco autocomplete/typecheck-underline content, gutter breakpoint click, the Stop banner's transient/persistent copy sequence, Run Again re-opening the dialog, responsive layout, dark-mode following, the Copywriting Contract audit) -- none of this was exercised through the real desktop app (`mage Run`) in this pass. The backend/CLI/Wails-service layer underneath every one of these is now proven for real (including the debug path this pass specifically fixed), which meaningfully de-risks a GUI pass, but does not substitute for one.
- **Pre-existing, unrelated test flakiness**: `TestScopeOfflineAcceptance` and a handful of config-validation tests were observed to fail intermittently during this session while a concurrent, unrelated session was also committing work to this same repository (a `mage Dev` hot-reload feature, commits `2e278dd`/`7af2ad1`). Not investigated further -- out of this plan's scope, and `TestScopeOfflineAcceptance` was already on record as a pre-existing toolchain-bootstrap-sensitive failure in 08-06-SUMMARY.md.

## Next steps

1. A human (or a carefully supervised, screenshot-verified computer-use pass, if the operator prefers) runs `mage Run` and works through 08-13-PLAN.md's Task 1 steps 5-8 and Task 2's sixteen steps against a live Art-Net target.
2. Decide on the memory-limit banner gap (fix vs. spec update).
3. Once both checkpoints are genuinely confirmed, update this SUMMARY's `resume-signal` outcome, check off 08-13 and Phase 8 in `ROADMAP.md`, and update `STATE.md`'s Current Position.

## Self-Check

- FOUND: internal/script/debugbridge.go (fix applied, committed `ad4da15`)
- FOUND: internal/command/scriptdebug_test.go, scriptrun_test.go, scriptstop_test.go, scriptvalidate_test.go (hermeticity fix applied, committed `ad4da15`)
- FOUND commit: `ad4da15` (fix: resume the spurious CDP debugCommand pause and make script tests hermetic)
- `go build ./...`: PASS
- `go test ./internal/script/... ./internal/command/... ./internal/wails/...`: PASS (run twice back-to-back, both clean)
- Working tree left clean of test-run pollution (stray show.golc scripts and .ts fixtures removed)
- NOT DONE: human/GUI checkpoint approval for either Task 1 or Task 2 -- ROADMAP.md phase 8 checkbox intentionally left unchecked

---
*Phase: 08-isolated-typescript-automation*
*Status: evidence gathered, two bugs found and fixed, checkpoints still open*
