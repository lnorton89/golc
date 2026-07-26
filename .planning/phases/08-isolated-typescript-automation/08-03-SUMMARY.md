---
phase: 08-isolated-typescript-automation
plan: 03
subsystem: scripting-sdk
tags: [typescript, jsonschema, code-generation, sdk, deno, monaco]

# Dependency graph
requires:
  - phase: 07-versioned-external-control-api
    provides: show.APIKeyScope (playback/authoring/admin) and internal/contracts' generation/drift-check discipline this plan mirrors
provides:
  - internal/scriptsdk package with a self-registering SDKMethodDescriptor registry, byte-stable/drift-checked golc.d.ts and golc-runtime.ts generation
  - Every internal/command route explicitly classified as an exposed typed SDK method or an excluded-with-reason route
  - internal/command/scriptsdk_parity_test.go: a build-breaking completeness gate against the real command registry
affects: [08-05-script-host, 08-06-capability-enforcement, 08-11-monaco-editor]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Self-registering descriptor registry (mirrors internal/contracts/generate.go) generating two committed TypeScript artifacts from one source of truth instead of one JSON Schema per descriptor"
    - "Reserved-TypeScript-keyword-safe method naming (delete->remove, switch->switchScene, import->importDefinition, export->exportDocument, interface namespace->interfaces), verified with a real tsc --strict type-check"

key-files:
  created:
    - internal/scriptsdk/generate.go
    - internal/scriptsdk/descriptors.go
    - internal/scriptsdk/generate_test.go
    - internal/scriptsdk/coverage_test.go
    - internal/scriptsdk/generated/golc.d.ts
    - internal/scriptsdk/generated/golc-runtime.ts
    - internal/command/scriptsdk_parity_test.go
  modified:
    - internal/command/generate.go
    - internal/command/check.go

key-decisions:
  - "Used a table-driven registration loop (sdkMethodTable + func init()) in descriptors.go instead of 62 individually spelled `var _ = MustRegisterSDKMethod(...)` statements -- still self-registers at package init with no central switch, but keeps the enormous mechanically-derived route classification reviewable as one flat, sorted list."
  - "Renamed 6 generated method leaves/namespace segments that collided with reserved TypeScript keywords (delete, switch, import, export, interface) to remove/switchScene/importDefinition/exportDocument/interfaces -- caught by running the committed output through `tsc --noEmit --strict`, not assumed."
  - "Result types are deliberately generic (AckResult{message}, JSONResult{json}, and one dedicated APIKeyCreateResult) rather than one bespoke struct per route: most internal/command handlers return plain text, not a stable Go-typed JSON shape, so a generic result avoids inventing structure the CLI doesn't actually guarantee."
  - "Chose scope=authoring (not the general read-only default) for programmer inspect and the read-only operatorsurface routes (list/show), applying the plan's 'when ambiguous, choose the more restrictive scope' rule since both expose authoring-domain internal state to a caller."

requirements-completed: [SCRP-02]

coverage:
  - id: D1
    description: "internal/scriptsdk generates golc.d.ts (ambient global types, zero import/export) and golc-runtime.ts (stdio JSON-lines shim) byte-identically from one self-registering descriptor registry, with CheckDrift wired into generate/generate --check/check --concern project"
    requirement: "SCRP-02"
    verification:
      - kind: unit
        ref: "internal/scriptsdk/generate_test.go#TestScopeScriptsdk"
        status: pass
      - kind: unit
        ref: "internal/scriptsdk/coverage_test.go#TestScriptsdkCoverage"
        status: pass
      - kind: other
        ref: "npx -p typescript@7.0.2 tsc --noEmit --strict internal/scriptsdk/generated/golc.d.ts and golc-runtime.ts -- zero errors"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every route declared in internal/command's registry is exhaustively classified as an exposed SDK method or an excludedRoutes entry with a one-line reason (62 exposed, 22 excluded), including 'playback evaluate' explicitly excluded for frame evaluation"
    requirement: "SCRP-02"
    verification:
      - kind: unit
        ref: "internal/command/scriptsdk_parity_test.go#TestEveryDeclaredRouteIsClassified"
        status: pass
      - kind: unit
        ref: "internal/command/scriptsdk_parity_test.go#TestNoSDKMethodTargetsUndeclaredRoute"
        status: pass
    human_judgment: false

duration: 55min
completed: 2026-07-26
status: complete
---

# Phase 8 Plan 3: Typed GOLC SDK Generator Summary

**Self-registering `internal/scriptsdk` descriptor registry generating byte-stable, drift-checked `golc.d.ts` (ambient global Monaco types, zero import/export) and `golc-runtime.ts` (stdio JSON-lines shim) from every internal/command route this SDK exposes, with a build-breaking parity test guaranteeing no route is ever silently absent.**

## Performance

- **Duration:** 55 min
- **Started:** 2026-07-26T00:40:00Z
- **Completed:** 2026-07-26T01:35:00Z
- **Tasks:** 3
- **Files modified:** 9

## Accomplishments
- `internal/scriptsdk/generate.go`: `SDKMethodDescriptor` registry, `RegisterSDKMethod`/`MustRegisterSDKMethod`/`RegisterExclusion`/`MustRegisterExclusion`, `RegisteredSDKMethods`/`RegisteredExclusions`, and `GenerateInto`/`GenerateAll`/`CheckDrift` mirroring `internal/contracts`' exact generation discipline -- `CheckDrift` always generates into a disposable temp dir and never writes to the committed `internal/scriptsdk/generated/` path.
- `internal/scriptsdk/descriptors.go`: every currently-declared `internal/command` route classified -- 62 exposed as typed SDK methods (with `show.APIKeyScope` assigned and rationale documented in the package doc comment) and 22 deliberately excluded with a one-line reason each, including `playback evaluate` (frame evaluation is not script-reachable) and `artnet serve` (daemon lifecycle).
- The committed `golc.d.ts` and `golc-runtime.ts` were generated via `go run ./cmd/golc-project generate`, verified to type-check cleanly under `tsc --noEmit --strict` (both files, zero errors), and are drift-checked by `go run ./cmd/golc-project check --concern project`.
- `internal/command/generate.go` and `check.go` now call `scriptsdk.GenerateAll`/`CheckDrift` alongside the existing `contracts`/`api` calls -- one generation discipline, two (now three) registries.
- `internal/command/scriptsdk_parity_test.go`: the one file importing both `internal/command` and `internal/scriptsdk`, proving `TestEveryDeclaredRouteIsClassified` (a route in neither set fails the build, naming it) and `TestNoSDKMethodTargetsUndeclaredRoute` (an SDK route not in the real registry fails the build, naming it).

## Task Commits

Each task was committed atomically:

1. **Task 1: scriptsdk descriptor registry, renderer, and drift check** - `d3dbc7a` (feat)
2. **Task 2: Descriptor registrations, capability-surface coverage gate, and committed generated output** - `fd6acd0` (feat)
3. **Task 3: Wire scriptsdk into generate/check and add the registry parity test** - `ef534b3` (feat)

## Files Created/Modified
- `internal/scriptsdk/generate.go` - descriptor registry, TypeScript renderers, GenerateInto/GenerateAll/CheckDrift
- `internal/scriptsdk/descriptors.go` - Params/Result shapes, 62 exposed route registrations, 22 excluded-with-reason routes
- `internal/scriptsdk/generate_test.go` - Task 1 behavior tests (duplicate/invalid rejection, ordering, determinism, drift, ambient-namespace shape)
- `internal/scriptsdk/coverage_test.go` - Task 2 coverage-completeness tests (well-formedness, no dual classification, non-empty reasons, playback evaluate excluded, zero drift)
- `internal/scriptsdk/generated/golc.d.ts` - committed ambient global GOLC SDK types
- `internal/scriptsdk/generated/golc-runtime.ts` - committed stdio JSON-lines runtime shim
- `internal/command/generate.go` - wires `scriptsdk.GenerateAll`/`CheckDrift` into `generate`/`generate --check`
- `internal/command/check.go` - wires `scriptsdk.CheckDrift` into `check --concern project`'s generated-schema step
- `internal/command/scriptsdk_parity_test.go` - route-classification completeness gate against the real command registry

## Decisions Made
- Table-driven registration (`sdkMethodTable` + `func init()`) instead of 62 individually spelled `var _ = MustRegisterSDKMethod(...)` statements -- see key-decisions above.
- Reserved-TypeScript-keyword collisions (`delete`, `switch`, `import`, `export`, `interface`) renamed to `remove`/`switchScene`/`importDefinition`/`exportDocument`/`interfaces` and verified with a real `tsc --strict` run rather than assumed safe.
- Generic `AckResult`/`JSONResult` result shapes instead of one bespoke struct per route.
- `programmer inspect` and `operatorsurface list`/`show` classified as `authoring` (not the general read-only `playback` default) per the plan's "choose the more restrictive scope when ambiguous" rule.
- `show open`/`show save-as` classified as `authoring` (mutating routes not explicitly named in the plan's authoring parenthetical, but consistent with `show save`); `show inspect`/`show diagnose` classified as `playback` (pure read-only queries).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Renamed 6 generated method leaves/namespace segments colliding with reserved TypeScript keywords**
- **Found during:** Task 2, while verifying the freshly generated `golc.d.ts`/`golc-runtime.ts` against a real TypeScript compiler
- **Issue:** The mechanical Route -> Method mapping produced `function delete(...)`, `function switch(...)`, `function import(...)`, `function export(...)`, and `namespace interface { ... }` -- all illegal, since `delete`/`switch`/`import`/`export`/`interface` are reserved TypeScript identifiers. `tsc --noEmit --strict` failed with syntax errors on the first generation attempt.
- **Fix:** Renamed the affected `Method` dot-paths in `descriptors.go` (`scene/chase/motion/theme/preset delete` -> `.remove`, `playback switch` -> `.switchScene`, `fixture import` -> `.importDefinition`, `show export` -> `.exportDocument`, `artnet interface list` -> `artnet.interfaces.list`), each with an inline comment explaining why, then regenerated.
- **Files modified:** `internal/scriptsdk/descriptors.go`, `internal/scriptsdk/generated/golc.d.ts`, `internal/scriptsdk/generated/golc-runtime.ts`
- **Verification:** `npx -p typescript@7.0.2 tsc --noEmit --strict` on both committed generated files now exits 0 with zero errors.
- **Committed in:** `fd6acd0` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Necessary for the generated SDK to actually be valid TypeScript -- caught by real compiler verification rather than left latent. No scope creep.

## Issues Encountered
- `go test ./internal/command/...` (full package) shows 5 pre-existing failures in this worktree unrelated to this plan's changes: `TestBuildRouteCompilesTheProductionRepository`, `TestBuildablePackagesExcludesMagefiles`, `TestScopeCrossPlatformCI/Mage_tests_cross-compile_for_every_configured_contributor_platform`, `TestScopeGreenSubprocess`, `TestScopeOfflineAcceptance` -- all require a bootstrapped pinned toolchain/pre-built `golc-project.exe` under `.tools/`, which this fresh worktree checkout has never provisioned (`mage Bootstrap` was never run). Confirmed unrelated to scriptsdk (the missing directories predate this plan's changes). Logged to `.planning/phases/08-isolated-typescript-automation/deferred-items.md` rather than fixed, per the scope-boundary rule (fixing it means running a repository-wide provisioning step, not editing this plan's files). `go test ./internal/scriptsdk/...` and the two targeted parity tests are fully green; `go run ./cmd/golc-project generate`/`check --concern project` (which only need the local `go` on PATH) both exit 0 with zero drift.
- `router_test.go`'s `"routerfixture echo"` fixture route (a pre-existing, intentional self-registration proof, reachable only inside a `go test` binary) initially failed `TestEveryDeclaredRouteIsClassified`. Added it to a documented `testOnlyRoutes` allowlist in `scriptsdk_parity_test.go` rather than classifying it in `descriptors.go` (it is not a real production route reachable from `cmd/golc-project`'s actual binary).

## User Setup Required

None - no external service configuration required. (Deno toolchain provisioning is explicitly deferred to a later Phase 8 plan per 08-RESEARCH.md's Environment Availability table -- this plan only generates the type surface, it does not run a script.)

## Next Phase Readiness
- The generated `golc.d.ts`/`golc-runtime.ts` and per-method `show.APIKeyScope` metadata are ready for 08-05 (script host wiring, which injects `golc-runtime.ts` alongside a sandboxed script) and 08-06 (host-side capability enforcement, which reads the required scope from generated metadata) to consume directly.
- 08-11 (Monaco editor integration) can load the committed `golc.d.ts` for live type-checking/autocomplete without any further generation work.
- No blockers. The `sdkMethodTable`/`excludedRouteTable` pattern in `descriptors.go` is the single place a future command-registry route addition must be classified -- `TestEveryDeclaredRouteIsClassified` fails loudly if it is not.

---
*Phase: 08-isolated-typescript-automation*
*Completed: 2026-07-26*

## Self-Check: PASSED

All 9 task-created/modified files plus this SUMMARY.md and deferred-items.md verified present on disk. All 3 task commit hashes (`d3dbc7a`, `fd6acd0`, `ef534b3`) verified present in `git log --oneline --all`.
