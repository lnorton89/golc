---
phase: 8
slug: isolated-typescript-automation
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-25
---

# Phase 8 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` package, table-driven, `_test.go` suffix (backend); Vitest 4.x (frontend, already configured in `frontend/package.json`) |
| **Config file** | Go: none beyond `go.mod`/`mage Test`. Frontend: inline in `vite.config.ts` (no separate `vitest.config.*`) |
| **Quick run command** | `go test ./internal/script/... ./internal/scriptsdk/... ./internal/show/... -run <TestName>` (backend); `npm --prefix frontend run test -- ScriptsWorkspace` (frontend) |
| **Full suite command** | `mage Test` (backend); `npm --prefix frontend run build` (frontend — includes `tsc --noEmit && vitest run && vite build`) |
| **Estimated runtime** | Not yet measured — `internal/script`, `internal/scriptsdk`, and the frontend Scripts workspace are all Wave 0 (do not exist yet) |

---

## Sampling Rate

- **After every task commit:** `go test ./internal/script/...` (or the specific changed package); `npm --prefix frontend run test -- <changed workspace>` for frontend changes
- **After every plan wave:** `mage Test` (full backend suite); `npm --prefix frontend run build` (full frontend suite, includes the runtime-error smoke gate)
- **Before `/gsd-verify-work`:** Full suite green, plus the manual-only verifications below (Windows Job Object hard-cap, real debugger attach) — these are the parts least amenable to pure unit testing
- **Max feedback latency:** 60s (targeted `go test`/`vitest` run per task commit)

---

## Per-Task Verification Map

*Task IDs are assigned by the planner (Step 8) — this map records the requirement → test-command contract the planner's tasks must satisfy. Plan/Wave columns are filled in once PLAN.md files exist.*

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | SCRP-01 | — | Create/edit/validate/run/stop/debug lifecycle works end to end | integration (Go) | `go test ./internal/script/... -run TestScriptLifecycle` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SCRP-01 | — | Editor create/edit/validate UI round-trip | component (Vitest) | `npm --prefix frontend run test -- ScriptsWorkspace` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SCRP-02 | — | Generated SDK is byte-stable and drift-checked | unit (Go) | `go test ./internal/scriptsdk/... -run TestGenerateDrift` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SCRP-02 | — | A script cannot reach raw DMX/frame evaluation through the SDK | integration (Go) | `go test ./internal/script/... -run TestSDKNoRawDMXRoute` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SCRP-03 | T-08-01 | Zero Deno permission flags are ever passed for a script run | unit (Go) | `go test ./internal/script/... -run TestDenoCommandLineHasNoAllowFlags` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SCRP-03 | T-08-01 | A script attempting filesystem/network/env/subprocess access is denied at the OS/runtime level | integration (Go, spawns real pinned Deno) | `go test ./internal/script/... -run TestDenoDenialSurface` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SCRP-04 | — | Capability/deadline/rate/resource profile is saved per-script and pre-filled at run time | integration (Go) | `go test ./internal/show/... -run TestScriptProfilePersistence` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SCRP-04 | T-08-03 | Exceeding a deadline/rate/resource limit produces immediate hard termination with a logged reason (D-08) | integration (Go, Windows-only) | `go test ./internal/script/... -run TestLimitOverrunHardKill` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SCRP-05 | — | Structured logs/diagnostics/source-mapped stack traces/command outcomes stream live | integration (Go) | `go test ./internal/script/... -run TestLiveLogStreaming` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SCRP-06 | T-08-04 | A runaway/crashed/blocked script can be terminated without affecting a concurrently running Art-Net frame loop | integration (Go, existing Art-Net test harness) | `go test ./internal/script/... -run TestScriptKillDoesNotBlockArtnet` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | D-01/D-02 | T-08-02 | Debug-mode-only inspector: no CDP server exists during a plain Run | unit (Go) | `go test ./internal/script/... -run TestNoInspectorOutsideDebugMode` | ❌ W0 | ⬜ pending |

---

## Wave 0 Requirements

- [ ] `internal/script/` package does not exist yet — every listed backend test file is new.
- [ ] `internal/scriptsdk/` generator package does not exist yet.
- [ ] `internal/show/scripts.go` (Script entity + persistence, D-17) does not exist yet.
- [ ] `config/toolchain.toml` has no `[toolchain.deno]` entry yet — a pinned, checksum-verified Deno binary must be available before any real (non-mocked) `TestDenoDenialSurface`-style integration test can run.
- [ ] `frontend/src/workspaces/build/ScriptsWorkspace.tsx` and a Monaco integration do not exist; `monaco-editor` is not yet a frontend dependency (blocked on the `checkpoint:human-verify` the Package Legitimacy Audit's SUS verdict requires).
- [ ] No existing test harness exercises "kill a subprocess while the Art-Net frame loop is running and assert no frame miss" — needs a new fixture; confirm exact reuse points against existing Art-Net test infrastructure during planning.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Windows Job Object CPU/memory hard-cap actually enforces limits under real load | SCRP-04 | OS-level kernel enforcement cannot be fully simulated in a unit test | Run a script with a tight memory/CPU limit on real Windows hardware; confirm hard kill at the configured threshold and no leak beyond the cap |
| Real debugger attach/breakpoint/step-through works end-to-end via Monaco + CDP | SCRP-01 (debug), D-01/D-02 | CDP protocol plus editor UI interaction needs a human driving an actual debug session | Set a breakpoint in the Monaco gutter, launch Debug mode, confirm execution pauses at the breakpoint and local variables are inspectable |
| `monaco-editor` package legitimacy/version confirmation before install | D-15 | Package Legitimacy Audit flagged `monaco-editor`'s latest version SUS (too-new heuristic) — requires explicit human sign-off before `npm install` | Human confirms the exact pinned `monaco-editor` minor version against the npm registry before installation, per `08-RESEARCH.md`'s Package Legitimacy Audit |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
