---
phase: 9
slug: front-door-ui-completion
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-27
---

# Phase 9 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Seeded from
> `09-RESEARCH.md`'s Validation Architecture section; Task ID/Plan/Wave columns are
> filled in once `/gsd-plan-phase` produces PLAN.md files (this phase has no plans yet).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework (Go)** | `go test` (stdlib `testing`), existing project-wide convention |
| **Framework (frontend)** | Vitest ^4.1.10 + `@testing-library/react` ^16.3.2 |
| **Config file** | `frontend/vite.config.ts` (Vitest config colocated with Vite config — no separate `vitest.config.ts`) |
| **Quick run command (Go)** | `go test ./internal/fixture/... ./internal/wails/... -run <TestName>` |
| **Quick run command (frontend)** | `npm --prefix frontend test -- <TestFile>` (Vitest run mode) |
| **Full suite command** | `go test ./...` and `npm --prefix frontend run build` (runs `tsc --noEmit && vitest run && vite build`) |
| **Estimated runtime** | ~30-60s Go, ~30-60s frontend build (unchanged from Phase 8 baseline) |

---

## Sampling Rate

- **After every task commit:** Run the touched stack's quick command (`go test ./internal/fixture/... ./internal/wails/...` or `npm --prefix frontend test -- <touched file>`)
- **After every plan wave:** Run `go test ./...` and `npm --prefix frontend run build`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~60 seconds

---

## Per-Task Verification Map

*Plans: TBD — this table is populated once `/gsd-plan-phase 9` creates PLAN.md files and assigns task IDs/waves. Requirement → expected-test mapping from research, in the interim:*

| Requirement | Behavior | Test Type | Automated Command | File Exists |
|-------------|----------|-----------|--------------------|-------------|
| FDUI-01 | Local fixture directory listing decodes/pins every `.yaml`/`.yml` file, skips non-fixture files | unit (Go) | `go test ./internal/fixture/... -run TestListDirectory` | ❌ Wave 0 — new `internal/fixture/directory_test.go` |
| FDUI-01 | `FixtureLibraryService.ListLocal`/`SearchOFL`/`Import` project correctly and degrade gracefully when bridge unavailable | unit (Go + frontend) | `go test ./internal/wails/... -run TestFixtureLibraryService` / `npm --prefix frontend test -- FixtureLibraryWorkspace` | ❌ Wave 0 — new `svc_fixturelibrary_test.go`; existing `FixtureLibraryWorkspace.test.tsx` only asserts the `ComingSoon` stub and must be rewritten |
| FDUI-02 | `App.RelaunchWithShow` spawns a new process with the correct `GOLC_DESKTOP_SHOW` env override, via an injectable spawn func (mirrors `app_test.go`'s existing `spawnFunc` test-double pattern) | unit (Go) | `go test ./internal/wails/... -run TestRelaunchWithShow` | ❌ Wave 0 — extend existing `internal/wails/app_test.go` |
| FDUI-02 | `ShowsWorkspace.tsx` renders current-show path, Open/New actions, and relaunch transient/error copy | unit (frontend) | `npm --prefix frontend test -- ShowsWorkspace` | ❌ Wave 0 — new `ShowsWorkspace.test.tsx` |
| FDUI-03 | Guided First Show auto-launches only when show has zero fixtures/pools/scenes, never on a show with existing content | unit (frontend) | `npm --prefix frontend test -- GuidedFirstShow` | ❌ Wave 0 — new `GuidedFirstShow.test.tsx` |
| FDUI-03 | Guided First Show stage status (blocker/warning/evidence) derives from live backend reads, never a separately persisted progress flag | unit (frontend, mocked bridge) | `npm --prefix frontend test -- GuidedFirstShow` | ❌ Wave 0 — same file, additional cases |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky — all rows start ⬜ pending until plans/tasks exist.*

---

## Wave 0 Requirements

- [ ] `internal/fixture/directory_test.go` — covers the extracted `ListDirectory` function (FDUI-01)
- [ ] `internal/wails/svc_fixturelibrary_test.go` — covers `ListLocal`/`SearchOFL`/`Import` (FDUI-01)
- [ ] `internal/wails/app_test.go` extension — covers `RelaunchWithShow`'s spawn-args and `Quit`-call sequencing via the existing injectable `dialFunc`/`spawnFunc` test-double pattern (FDUI-02)
- [ ] `frontend/src/workspaces/show/ShowsWorkspace.test.tsx` — new file (FDUI-02)
- [ ] `frontend/src/workspaces/show/GuidedFirstShow/GuidedFirstShow.test.tsx` — new file (FDUI-03)
- [ ] `frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx` — must be rewritten; current version only asserts the `ComingSoon` stub's static text and will fail/be meaningless once the real workspace lands (FDUI-01)

---

## Manual-Only Verifications

*None identified by research.* All phase behaviors have automated verification. Visual quality of the Guided First Show flow (stage transitions, readiness panel styling) is covered by the `09-UI-SPEC.md` contract and standard UI review, not tracked here as a separate manual-only behavior.

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
