---
phase: 07-versioned-external-control-api
plan: 01
subsystem: infra
tags: [api, go-modules, supply-chain, dependencies, chi, huma, rate-limiting]

# Dependency graph
requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    provides: internal/bootstrap's offline-forced GOPROXY=off/-mod=readonly build/test path and its project-local .tools/cache/go-mod module cache layout
provides:
  - github.com/go-chi/chi/v5 v5.3.1 pinned as a direct go.mod dependency (HTTP router, D-04)
  - github.com/danielgtaylor/huma/v2 v2.39.0 pinned as a direct go.mod dependency (OpenAPI-3.1-from-Go framework, D-03)
  - golang.org/x/time v0.15.0 pinned as a direct go.mod dependency (per-key token-bucket rate limiter)
  - project-local offline module cache (.tools/cache/go-mod) warmed with the new modules so every later Phase 7 plan's mage build/test runs resolve them with GOPROXY=off
affects: [07-02-versioned-external-control-api, 07-03-versioned-external-control-api, 07-04-versioned-external-control-api, 07-05-versioned-external-control-api, 07-09-versioned-external-control-api]

# Tech tracking
tech-stack:
  added: [github.com/go-chi/chi/v5 v5.3.1, github.com/danielgtaylor/huma/v2 v2.39.0, golang.org/x/time v0.15.0]
  patterns:
    - "Go module pin procedure: go get with normal network access (never inside the offline-forced bootstrap path), then warm .tools/cache/go-mod explicitly via `GOMODCACHE=.tools/cache/go-mod go mod download all` before mage's GOPROXY=off targets will resolve the new modules"
    - "Force a require into go.mod's direct block by hand-editing when no application code imports the module yet (go get/go mod tidy would otherwise classify it indirect since nothing references it) -- deliberate for a pin-only plan that adds zero application code"

key-files:
  created: []
  modified: [go.mod, go.sum]

key-decisions:
  - "Moved all three new requires into go.mod's direct (non -indirect) block by manual edit rather than running go mod tidy, because no application code imports them yet in this plan and go mod tidy would otherwise remove or re-classify them as indirect/unused"
  - "Ran go get with the default GOMODCACHE (network access, Task 2's deliberate one-time online step) then separately warmed the project-local .tools/cache/go-mod (gitignored) via go mod download all, since mage testquick forces GOMODCACHE=.tools/cache/go-mod + GOPROXY=off (internal/bootstrap/cache.go's OfflineEnvironment / internal/command/test.go's projectGoEnvironment) and would not otherwise find the newly pinned modules"

patterns-established:
  - "Pattern: Go module pin plans in this repo run go get online, hand-verify go mod graph for unexpected transitives, then must independently warm .tools/cache/go-mod before the offline mage path can see the new modules -- this second step is not covered by 07-RESEARCH.md's grounded procedure and should be folded into future pin plans/RESEARCH updates"

requirements-completed: []  # No requirement is functionally complete after this plan -- API-01/API-02/API-05 each span multiple Phase 7 plans (07-02..07-09); this plan only pins the module dependencies with zero application code, so none of them are checked off here.

coverage:
  - id: D1
    description: "chi v5.3.1, huma/v2 v2.39.0, and golang.org/x/time v0.15.0 pinned as direct go.mod/go.sum requires at the human-verified versions"
    verification:
      - kind: other
        ref: "git diff go.mod (direct require block) + go mod verify"
        status: pass
    human_judgment: false
  - id: D2
    description: "go build ./... and mage testquick both pass offline (GOPROXY=off, .tools/cache/go-mod) with the enlarged module graph, and go mod graph shows no unexpected mandatory third-party transitive from chi/huma/x-time in the actual build (only existing wails/echo transitive deps bumped patch/minor versions via MVS)"
    verification:
      - kind: other
        ref: "go build ./... (exit 0); mage testquick (go vet passed, exit 0); go mod verify (all modules verified)"
        status: pass
    human_judgment: false

duration: 15min
completed: 2026-07-24
status: complete
---

# Phase 7 Plan 1: Pin chi, huma/v2, and golang.org/x/time Summary

**Pinned chi v5.3.1, huma/v2 v2.39.0, and golang.org/x/time v0.15.0 as direct go.mod/go.sum dependencies after a blocking human supply-chain checkpoint, with no application code added -- offline build/test path re-verified green.**

## Performance

- **Duration:** ~15 min (Task 2 execution; Task 1's human checkpoint approval elapsed separately, async)
- **Completed:** 2026-07-24T22:58:57-07:00
- **Tasks:** 2/2 (Task 1 checkpoint:human-verify approved by user; Task 2 auto)
- **Files modified:** 2 (go.mod, go.sum)

## Accomplishments
- Task 1 (blocking-human checkpoint): human reviewed all three modules' pkg.go.dev pages and approved chi v5.3.1, huma/v2 v2.39.0, and golang.org/x/time v0.15.0 as direct dependencies of this offline-pinned repository. Resume signal received: "approved".
- Task 2: ran `go get github.com/go-chi/chi/v5@v5.3.1`, `go get github.com/danielgtaylor/huma/v2@v2.39.0`, `go get golang.org/x/time@v0.15.0` with normal network access (default GOPROXY, outside the offline-forced bootstrap path).
- Hand-edited go.mod to move all three into the direct `require (...)` block (they landed in the `// indirect` block after `go get` because no application code imports them yet in this pin-only plan).
- Reviewed `go mod graph | grep -E "go-chi/chi|danielgtaylor/huma|golang.org/x/time"`: huma/v2's own go.mod declares many optional router-adapter dependencies (gin, fiber, mux, httprouter, bunrouter, echo/v5) but none of those are pulled into the actual pruned build list -- the only concrete changes to the real (indirect) require block are version bumps of existing wails-transitive deps already present before this plan (echo v4.13.3->v4.15.2, gommon v0.4.2->v0.5.0, mattn/go-colorable v0.1.13->v0.1.14, mattn/go-isatty v0.0.20->v0.0.22, golang.org/x/crypto v0.51.0->v0.52.0, golang.org/x/net v0.54.0->v0.55.0), satisfying the "no unexpected third-party transitive dependency" acceptance criterion.
- Warmed the project-local offline module cache (`.tools/cache/go-mod`, gitignored) via `GOMODCACHE=.tools/cache/go-mod GOPROXY=https://proxy.golang.org,direct go mod download all` so the offline `mage testquick`/`mage build` path (which forces `GOMODCACHE=.tools/cache/go-mod` + `GOPROXY=off`, per `internal/bootstrap/cache.go`'s `OfflineEnvironment` and `internal/command/test.go`'s `projectGoEnvironment`) can resolve the new modules without network.
- Verified `go build ./...` succeeds and `mage testquick` (go vet -tags mage ./...) passes fully offline; `go mod verify` reports "all modules verified".
- Confirmed `git diff --name-only` shows only `go.mod` and `go.sum` changed -- no file under `internal/` was created or modified, matching the plan's acceptance criteria.

## Task Commits

1. **Task 1: Supply-chain verification of chi, huma/v2, and golang.org/x/time (T-07-SC)** - checkpoint:human-verify, gate="blocking-human" -- no code change, approved via user's "approved" resume signal, no commit (nothing to commit for a checkpoint task).
2. **Task 2: Pin the three modules into go.mod/go.sum (online, one-time)** - `a2fe325` (feat)

**Plan metadata:** SUMMARY.md commit pending (this file); STATE.md/ROADMAP.md intentionally left untouched by this plan per explicit dispatch instruction.

## Files Created/Modified
- `go.mod` - added chi v5.3.1, huma/v2 v2.39.0, golang.org/x/time v0.15.0 to the direct require block; bumped four existing indirect wails/echo-transitive deps (echo, gommon, mattn/go-colorable, mattn/go-isatty) plus golang.org/x/crypto and golang.org/x/net via MVS resolution
- `go.sum` - added verified hash entries for the three new modules and their actually-needed transitive closure

## Decisions Made
- Moved the three new requires into go.mod's direct block by hand-editing (not `go mod tidy`), since this plan deliberately adds zero application code that imports them -- `go mod tidy` would otherwise strip or re-indirect them. Chose this over adding a throwaway blank-import file, since the plan explicitly prohibits any application code addition this plan.
- Independently warmed `.tools/cache/go-mod` after the `go get` step, since the default `go get` (network-enabled, no offline env override) downloaded into the user's global `GOMODCACHE` (`C:\Users\Lawrence\go\pkg\mod`), not the project-local offline cache directory `mage testquick`/`mage build` actually reads from. This gap wasn't explicit in 07-RESEARCH.md's grounded pin procedure and is called out as a pattern for future pin plans.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Warmed the project-local offline module cache before mage testquick would pass**
- **Found during:** Task 2 (running the `<verify>` step, `go build ./... && mage testquick`)
- **Issue:** `mage testquick` failed with `module lookup disabled by GOPROXY=off` (attempting to fetch `github.com/mattn/go-isatty v0.0.22`) because the offline-forced environment (`internal/bootstrap/cache.go`'s `OfflineEnvironment`, `internal/command/test.go`'s `projectGoEnvironment`) points `GOMODCACHE` at the repo-local `.tools/cache/go-mod`, which the plain `go get` calls (using the default global `GOMODCACHE`) never populated.
- **Fix:** Ran `GOMODCACHE="$(pwd)/.tools/cache/go-mod" GOPROXY=https://proxy.golang.org,direct go mod download all` (still online, still outside the offline-forced path) to warm the project-local cache with the same go.sum-verified modules.
- **Files modified:** none (cache directory only; `.tools/` is gitignored)
- **Verification:** Re-ran `mage testquick` -- `go vet -tags mage ./...` passed, exit 0; `go build ./...` succeeded; `go mod verify` (run under `GOFLAGS=-mod=readonly GOPROXY=off GOMODCACHE=.tools/cache/go-mod`) reported "all modules verified".
- **Committed in:** n/a (cache warm produces no tracked file changes; go.mod/go.sum committed in `a2fe325`)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Necessary to satisfy the plan's own `<verify>` step (`go build ./... && mage testquick`) exactly as written; no scope creep, no application code touched.

## Issues Encountered
None beyond the offline-cache-warm deviation documented above.

## User Setup Required
None - no external service configuration required. (The `go get` network access in Task 2 was a one-time deliberate contributor action per plan design, already completed in this session.)

## Next Phase Readiness
- go.mod/go.sum are pinned, offline-verifiable, and committed (`a2fe325`); every remaining Phase 7 plan (07-02 through 07-09) can now import `github.com/go-chi/chi/v5`, `github.com/danielgtaylor/huma/v2`, and `golang.org/x/time` and build/test fully offline via `mage testquick`/`mage build`.
- No requirement (API-01/API-02/API-05) is functionally complete yet -- this plan is purely the dependency-pin foundation; REQUIREMENTS.md intentionally left unmarked (see `requirements-completed: []` above and rationale in frontmatter).
- Note for future pin plans / 07-RESEARCH.md maintainers: the grounded "Pinning a new Go dependency in this repo" procedure (07-RESEARCH.md lines 132-140) does not mention that `.tools/cache/go-mod` must be separately warmed after `go get` -- this plan's Task 2 needed an extra `go mod download all` against that GOMODCACHE before `mage testquick` would pass. Worth folding into that procedure text for the next contributor who pins a dependency.

---
*Phase: 07-versioned-external-control-api*
*Completed: 2026-07-24*

## Self-Check: PASSED
- FOUND: go.mod
- FOUND: go.sum
- FOUND: .planning/phases/07-versioned-external-control-api/07-01-SUMMARY.md
- FOUND: commit a2fe325
