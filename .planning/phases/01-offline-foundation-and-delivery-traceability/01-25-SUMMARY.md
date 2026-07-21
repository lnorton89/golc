---
phase: 01-offline-foundation-and-delivery-traceability
plan: 25
subsystem: infra
tags: [nodejs, npm, typescript, linear-sdk, bootstrap, offline, powershell]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 13
    provides: tools/linear-sync npm workspace (protocol.ts/adapter.ts, exact-lock npm ci, pinned TypeScript compile, self-registered linear-sdk/linear-sdk-operations scopes)
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 29
    provides: internal/command self-registration precedent this plan reuses unchanged (no new route/scope declared)
provides:
  - "tools/linear-sync/src/cli.ts: strict NDJSON process-boundary adapter entrypoint (OperationExecutor/runCLI), no HTTP server, no reconciliation policy"
  - "tools/linear-sync/test/operations.test.ts: complete fake-SDK hierarchy contract (marker TestScopeLinearSdkOperations) -- one create/update/readback per entity against tests/fixtures/linear/hierarchy-operations.json's exact SDK method/input/output transcript"
  - "tools/linear-sync/src/ambient-node.d.ts: hand-written ambient Node typings (process, node:fs, node:test, node:assert/strict) so this workspace never needs a third npm package beyond Plan 01-12's two approved pins"
  - "tests/acceptance/bootstrap-node.ps1: clean install / network-denied no-op / isolated cache-only reinstall acceptance for bootstrap --include linear-sync"
  - "golc.ps1 Test-LinearSyncNpmInstallMatches: npm-ci skip-if-unchanged contract mirroring the existing archive-pin Test-InstalledManifestMatches, so a second bootstrap of an unchanged lock is a true zero-network no-op"
affects: [01-14, 01-26, 01-27, linear-sync]

tech-stack:
  added: []
  patterns:
    - "tsconfig.json's rootDir became \".\" (from \"src\") so one tsc invocation compiles both src/**/*.ts and test/**/*.ts into dist/src/*.js and dist/test/*.js in a single pass -- test files import compiled sibling output via relative NodeNext-style specifiers (\"../src/adapter.js\") that resolve correctly both pre-compile (against the .ts source) and post-compile (against the mirrored dist/src/ layout)."
    - "Hand-written ambient .d.ts (not @types/node) is the supply-chain-safe way to type a small, stable Node builtin surface (process, node:fs, node:test, node:assert/strict) inside a workspace whose exact package set was already blocking-checkpoint approved (Plan 01-12) -- adding @types/node would be a third package requiring a new approval gate; a local ambient declaration file needs none."
    - "Node's --test CLI flag does not auto-discover a bare directory argument on the pinned 24.18.0 build (confirmed empirically: 'node --test dist/test/' throws MODULE_NOT_FOUND); a glob pattern ('dist/test/**/*.test.js') is resolved natively by Node itself with no shell expansion required, and is the correct invocation for a registered NodeScopeRegistration.Command."
    - "The fake-SDK test client is fixture-driven, not independently computed: FakeLinearClient returns exactly the fixture's authored sdkOutput for each call and records every call in order, so the test's deepStrictEqual assertions genuinely exercise adapter.ts's own normalize()/dispatch logic (title-vs-name mapping, description defaulting, ISO timestamp conversion, per-entity SDK method selection) rather than a tautological echo."
    - "golc.ps1's npm-ci skip manifest lives inside node_modules (node_modules/.golc-npm-ci-manifest.json) rather than under .tools/manifest, so deleting node_modules always and automatically invalidates it -- there is no way for a stale skip-manifest to survive without its install."

key-files:
  created:
    - tools/linear-sync/src/cli.ts
    - tools/linear-sync/src/ambient-node.d.ts
    - tools/linear-sync/test/operations.test.ts
    - tests/fixtures/linear/hierarchy-operations.json
    - tests/acceptance/bootstrap-node.ps1
  modified:
    - golc.ps1
    - internal/command/linear_sync.go
    - tools/linear-sync/package.json
    - tools/linear-sync/tsconfig.json

key-decisions:
  - "internal/delivery/graph.go and internal/bootstrap/bootstrap_test.go (both declared in this plan's files_modified frontmatter) were deliberately left byte-unchanged: neither delivery.OfflineEnvironment nor any Go-level bootstrap primitive is involved in tools/linear-sync's Node/npm provisioning, which lives entirely in golc.ps1 (PowerShell) and tools/linear-sync's own TypeScript. check --offline's \"test\" step only ever runs 'test --quick' (go vet, Go-only); it never reaches the linear-sdk-operations Node scope. No genuine correctness gap required touching either file -- matches the precedent Plan 01-29's summary set for declared-but-unneeded files."
  - "tsconfig.json's rootDir changed from \"src\" to \".\" (and include gained test/**/*.ts) so a single tsc invocation produces both dist/src/*.js and dist/test/*.js. This was a required Rule 3 (blocking) fix: the plan's own acceptance criteria require test/operations.test.ts to compile to a location the pre-registered linear-sdk-operations scope's Command can run, and there was no other way to get dist/test/ populated without either a second tsconfig or this rootDir change. Single-config was chosen over a second tsconfig.test.json to avoid two independently-drifting build configs."
  - "golc.ps1's dist output checks (bootstrap and the new npm-ci skip check) moved from dist/protocol.js and dist/adapter.js to dist/src/protocol.js, dist/src/adapter.js, dist/src/cli.js, and dist/test/operations.test.js -- required consequence of the rootDir change above, verified end-to-end against a real clean bootstrap."
  - "internal/command/linear_sync.go's registered Command changed from ['node', '--test', 'dist/test/'] to ['node', '--test', 'dist/test/**/*.test.js'] -- a Rule 1 (bug) fix discovered empirically: the pinned Node 24.18.0 build's --test flag throws MODULE_NOT_FOUND on a bare directory positional argument (verified directly against the extracted node.exe, both with and without a trailing slash), but resolves a glob pattern natively without shell expansion. Without this fix the plan's own <verify> command ('test --quick --scope linear-sdk-operations') could never pass."
  - "A hand-written ambient-node.d.ts supplies process/node:fs/node:test/node:assert:strict typings instead of adding @types/node as a devDependency -- keeps this workspace's approved package set at exactly the two pins Plan 01-12's blocking checkpoint approved (@linear/sdk@88.1.0, typescript@7.0.2), avoiding a new package-approval gate that would have turned this autonomous plan into a checkpoint plan."
  - "The fake-SDK hierarchy fixture (tests/fixtures/linear/hierarchy-operations.json) is the single source of truth both the fake client's return values and the test's expected-record assertions are checked against, authored by hand for all 5 entity kinds x {create, update, read} x exact SDK call transcript (5 SDK calls per entity: create, readback, update, readback, explicit read)."

requirements-completed: [CONF-01, CONF-03, LINR-03]

coverage:
  - id: D1
    description: "A clean first bootstrap --include linear-sync installs Node, runs exact-lock npm ci, and compiles src+test to dist/src and dist/test"
    requirement: CONF-01
    verification:
      - kind: e2e
        ref: "powershell -NoProfile -File .\\tests\\acceptance\\bootstrap-node.ps1 (Stage 1, run against a repository with prior Node state removed)"
        status: pass
    human_judgment: false
  - id: D2
    description: "An immediate second bootstrap of an unchanged lock is a true zero-network no-op: npm ci is never invoked, node_modules/dist/pins remain byte-identical"
    requirement: CONF-01
    verification:
      - kind: e2e
        ref: "powershell -NoProfile -File .\\tests\\acceptance\\bootstrap-node.ps1 (Stage 2: asserts the exact 'npm ci not invoked' diagnostic and byte-identical directory inventories)"
        status: pass
    human_judgment: false
  - id: D3
    description: "An isolated temporary prefix (never reusing existing node_modules) reinstalls and recompiles entirely from the warmed npm cache with npm ci --offline and a poisoned registry"
    requirement: CONF-03
    verification:
      - kind: e2e
        ref: "powershell -NoProfile -File .\\tests\\acceptance\\bootstrap-node.ps1 (Stage 3)"
        status: pass
    human_judgment: false
  - id: D4
    description: "cli.ts's runCLI accepts strict NDJSON discriminated requests only, decodes each line through protocol.ts's strict decodeOperation, and contains no HTTP server/listener/reconciliation-policy surface"
    requirement: LINR-03
    verification:
      - kind: unit
        ref: "tools/linear-sync/test/operations.test.ts: 'cli.ts runCLI decodes strict NDJSON...' and 'cli.ts contains no HTTP server or listener surface'"
        status: pass
    human_judgment: false
  - id: D5
    description: "operations.test.ts defines marker TestScopeLinearSdkOperations matching the exact pre-registered linear-sdk-operations scope and asserts one create/update/readback per entity against the exact SDK method/input/output transcript"
    requirement: LINR-03
    verification:
      - kind: e2e
        ref: "powershell -NoProfile -File .\\golc.ps1 test --quick --scope linear-sdk-operations"
        status: pass
    human_judgment: false

duration: ~2h30min
completed: 2026-07-21
status: complete
---

# Phase 1 Plan 25: Contract-Test the Hierarchy and Prove Offline Repeat/Cache-Only Bootstrap Summary

**A strict NDJSON CLI, a fixture-driven fake-SDK hierarchy contract test covering all five Linear entity kinds, and a three-stage bootstrap-node acceptance script proving clean install / zero-network second bootstrap / isolated cache-only reinstall — plus the Rule 1/3 fixes (Node's `--test` glob requirement, the rootDir/dist-layout change, and the npm-ci skip manifest) required to make all of it pass end to end**

## Performance

- **Duration:** ~2h30min
- **Started:** 2026-07-21T04:30:00Z (approx.)
- **Completed:** 2026-07-21T07:01:33Z
- **Tasks:** 1
- **Files modified:** 9 (5 created, 4 modified)

## Accomplishments

- `tools/linear-sync/src/cli.ts` is the strict NDJSON process-boundary entrypoint: `runCLI(input, output, executor)` reads newline-delimited JSON `Operation`s, strictly decodes each through `protocol.ts`'s `decodeOperation` (a malformed line fails the whole run closed, naming the exact line number via `CliProtocolError`), executes it through an injected `OperationExecutor`, and writes one NDJSON result line per operation. It never imports an HTTP/net module and never opens a listener; `main()` (the real process entrypoint, only invoked when the compiled file is the process's own entry script) wires a real `LinearSdkAdapter` from `LINEAR_API_KEY` and is never reached by `operations.test.ts`, which injects its own fake executor.
- `tools/linear-sync/src/ambient-node.d.ts` hand-declares just enough of `process`, `node:fs`, `node:test`, and `node:assert/strict` for this workspace's actual code, so no third npm package (`@types/node`) is ever needed beyond Plan 01-12's two blocking-checkpoint-approved pins.
- `tools/linear-sync/test/operations.test.ts` registers `TestScopeLinearSdkOperations` (the exact marker `config/commands.toml`/`internal/command/linear_sync.go` pre-registered as scope `linear-sdk-operations`) and, for every one of the five entity kinds (`project`, `project_milestone`, `parent_issue`, `requirement_issue`, `task_subissue`), runs a create → update → read sequence through `LinearSdkAdapter` against an in-memory `FakeLinearClient`, asserting both the normalized record at each step and the exact 5-call SDK transcript (`createX`, readback, `updateX`, readback, explicit read) match `tests/fixtures/linear/hierarchy-operations.json` byte-for-byte. The fake client is fixture-driven (returns exactly the fixture's authored `sdkOutput`), so the assertions genuinely exercise `adapter.ts`'s own field-mapping/dispatch logic rather than tautologically echoing input. Three supplementary tests cover `cli.ts`'s NDJSON round-trip, its structural absence of any HTTP-server surface, and a `protocol.ts` strict-decoding regression guard.
- `tests/fixtures/linear/hierarchy-operations.json` is the golden transcript all five entities' create/update inputs, fake SDK outputs, normalized records, and exact SDK call sequences are authored against.
- `tests/acceptance/bootstrap-node.ps1` runs three stages against the real repository under test: **Stage 1** removes any prior Node toolchain/`node_modules`/`dist`/npm-cache state and proves a genuinely clean `bootstrap --include linear-sync`; **Stage 2** reruns the identical command against the unchanged lock and asserts the exact `"npm ci not invoked"` skip diagnostic plus byte-identical `node_modules`/`dist` directory inventories; **Stage 3** copies only `package.json`/`package-lock.json`/`tsconfig.json`/`src/`/`test/` (never `node_modules`) into a brand-new temporary directory and proves `npm ci --offline` there succeeds using only the shared warmed npm cache, with a poisoned `NPM_CONFIG_REGISTRY` as defense-in-depth against a silently-ignored `--offline` flag, then compiles the result with the pinned `tsc`. All three stages, plus the plan's own exact `<verify>` command (`bootstrap-node.ps1` then `test --quick --scope linear-sdk-operations`), were run live end-to-end in this worktree and pass.
- `golc.ps1` gained `Test-LinearSyncNpmInstallMatches` (mirroring the existing `Test-InstalledManifestMatches` archive-pin skip contract): a recorded manifest inside `node_modules` matching the current `package.json`/`package-lock.json` hashes plus every expected `dist/` output present means `Invoke-GolcBootstrapLinearSync` returns immediately without invoking `npm ci` or `tsc` at all — a true zero-network no-op, not merely a fast reinstall.

## Task Commits

Each task was committed atomically:

1. **Task 1: Contract-test the hierarchy and prove offline repeat bootstrap** - `4cc8b1a` (feat)

**Plan metadata:** committed with this summary

## Files Created/Modified

- `tools/linear-sync/src/cli.ts` - Strict NDJSON adapter entrypoint (`runCLI`, `OperationExecutor`, `CliProtocolError`, `main`).
- `tools/linear-sync/src/ambient-node.d.ts` - Hand-written ambient Node typings avoiding an `@types/node` dependency.
- `tools/linear-sync/test/operations.test.ts` - Complete fake-SDK hierarchy contract plus cli.ts/protocol.ts supplementary coverage.
- `tests/fixtures/linear/hierarchy-operations.json` - Golden 5-entity create/update/read SDK-call transcript.
- `tests/acceptance/bootstrap-node.ps1` - Clean install / network-denied no-op / isolated cache-only reinstall acceptance.
- `golc.ps1` - `Test-LinearSyncNpmInstallMatches` npm-ci skip contract; updated `dist/src`/`dist/test` output-path checks.
- `internal/command/linear_sync.go` - Registered Node scope Command switched to the `dist/test/**/*.test.js` glob (Node's bare-directory `--test` argument does not auto-discover on the pinned build).
- `tools/linear-sync/package.json` - `test` script updated to the same glob for consistency.
- `tools/linear-sync/tsconfig.json` - `rootDir: "."`, `include` extended to `test/**/*.ts`, `types: []`.

## Decisions Made

See frontmatter `key-decisions` for the full list. Highlights:

- **`internal/delivery/graph.go` and `internal/bootstrap/bootstrap_test.go` were left unchanged** — both are declared in this plan's `files_modified` frontmatter, but neither is involved in tools/linear-sync's Node/npm provisioning (which lives entirely in `golc.ps1` and TypeScript), and no genuine correctness gap required touching either. `check --offline`'s `test` step only ever runs `test --quick` (go vet, Go-only) and never reaches the Node scope.
- **`tsconfig.json`'s `rootDir` changed from `"src"` to `"."`** so one `tsc` invocation compiles both `src/**/*.ts` and `test/**/*.ts` into `dist/src/*.js` and `dist/test/*.js` — required for the plan's own acceptance criteria (the pre-registered scope needs `dist/test/` populated) to be achievable at all.
- **`internal/command/linear_sync.go`'s registered Command changed to a glob** (`dist/test/**/*.test.js` instead of a bare `dist/test/`) after empirically confirming the pinned Node 24.18.0 build throws `MODULE_NOT_FOUND` on a bare directory positional argument to `--test`, but resolves a glob natively.
- **A hand-written ambient-node.d.ts avoids adding `@types/node`** as a third package, keeping this workspace within Plan 01-12's exact two-package blocking-checkpoint approval and this plan fully autonomous (no new checkpoint required).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `tsconfig.json`'s `rootDir: "src"` could never produce `dist/test/` output**
- **Found during:** Task 1, wiring `test/operations.test.ts` to compile
- **Issue:** With `rootDir: "src"` and `include: ["src/**/*.ts"]`, TypeScript rejects any file outside `src/` (including `test/operations.test.ts`) with a "file is not under 'rootDir'" error; the pre-registered `linear-sdk-operations` scope requires `dist/test/*.test.js` to exist.
- **Fix:** Changed `rootDir` to `"."` and `include` to `["src/**/*.ts", "test/**/*.ts"]`, producing `dist/src/*.js` and `dist/test/*.js` from one `tsc` invocation. Updated `golc.ps1`'s dist-output existence checks (both the bootstrap success check and the new npm-ci skip check) from `dist/protocol.js`/`dist/adapter.js` to `dist/src/protocol.js`/`dist/src/adapter.js`/`dist/src/cli.js`/`dist/test/operations.test.js`.
- **Files modified:** `tools/linear-sync/tsconfig.json`, `golc.ps1`
- **Verification:** Live `bootstrap --include linear-sync` run produces all four expected files; `go build ./...`, `go test ./...`, `check --offline`, `generate --check` all pass.
- **Committed in:** `4cc8b1a` (Task 1 commit)

**2. [Rule 1 - Bug] `internal/command/linear_sync.go`'s registered Node scope Command used a bare directory argument Node's pinned build rejects**
- **Found during:** Task 1, first live run of `test --quick --scope linear-sdk-operations`
- **Issue:** `node --test dist/test/` (with or without a trailing slash) throws `Error: Cannot find module '...\dist\test'` / `MODULE_NOT_FOUND` on the pinned Node 24.18.0 Windows build (confirmed by invoking the extracted `node.exe` directly, isolating the failure from any Go/PowerShell layer). Node's `--test` flag does not auto-discover test files inside a bare directory positional argument on this build; it resolves a glob pattern (`dist/test/**/*.test.js`) natively instead.
- **Fix:** Changed the registered `Command` in `linear_sync.go` from `["node", "--test", "dist/test/"]` to `["node", "--test", "dist/test/**/*.test.js"]` (a named constant, `linearSyncNodeTestGlob`), and updated `tools/linear-sync/package.json`'s `test` script identically for consistency.
- **Files modified:** `internal/command/linear_sync.go`, `tools/linear-sync/package.json`
- **Verification:** `powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-sdk-operations` exits 0 with all 9 subtests passing; also confirmed as part of the full `golc.ps1 test` suite and `check --offline`.
- **Committed in:** `4cc8b1a` (Task 1 commit)

**3. [Rule 2 - Missing critical functionality] `golc.ps1` had no mechanism to skip `npm ci` on an unchanged lock, so a "network-denied second bootstrap" would have actually attempted `npm ci` every time**
- **Found during:** Task 1 design, before writing `bootstrap-node.ps1`
- **Issue:** The plan's own acceptance criteria require "second bootstrap with network denied is no-op" and "records zero network calls." Prior to this fix, `Invoke-GolcBootstrapLinearSync` unconditionally ran `npm ci` on every bootstrap invocation regardless of whether `tools/linear-sync`'s lock had changed — the exact-lock discipline (D-04) was present, but the zero-network-on-repeat discipline (D-02) was not, unlike the parallel Go-module and archive-pin bootstrap paths, which already skip their own network-reaching steps when nothing changed.
- **Fix:** Added `Test-LinearSyncNpmInstallMatches`, mirroring the existing `Test-InstalledManifestMatches` archive-pin skip contract: a manifest recorded inside `node_modules` (invalidated automatically if `node_modules` is ever deleted) matching the current `package.json`/`package-lock.json` hashes, plus the pinned `tsc` and every expected `dist/` output already present, makes `Invoke-GolcBootstrapLinearSync` return immediately without invoking `npm ci` or `tsc`.
- **Files modified:** `golc.ps1`
- **Verification:** `tests/acceptance/bootstrap-node.ps1` Stage 2 (live, three separate full runs during this plan) confirms the exact skip diagnostic and byte-identical `node_modules`/`dist` inventories across a second bootstrap of the unchanged lock.
- **Committed in:** `4cc8b1a` (Task 1 commit)

---

**Total deviations:** 3 auto-fixed (1 Rule 3 blocking, 1 Rule 1 bug, 1 Rule 2 missing critical functionality). All three were required for the plan's own acceptance criteria and `<verify>` command to be achievable at all — none is a scope expansion beyond what those already required.

## Issues Encountered

- This worktree had no bootstrapped `.tools/` state (fresh worktree, matching the pattern noted in Plans 01-13/01-29). The full `<verify>` sequence required a real from-scratch download of Go 1.26.5 and Node 24.18.0 over live network access; both completed successfully. `tests/acceptance/bootstrap-node.ps1` was run four separate times end-to-end during this plan (once initially, once after the Rule 1/3 fixes, once as the plan's exact `<verify>` command, and once as a final regression check alongside the full `golc.ps1 test`/`check --offline`/`generate --check`/`build --scope linear-sdk`/`tests/acceptance/bootstrap.ps1`/`tests/acceptance/offline.ps1 -Mode core`/`tests/acceptance/command-parity.ps1` gates) — all passed cleanly every time.
- Node's `--test` bare-directory behavior (Deviation 2 above) was not discoverable from documentation alone; it required directly invoking the extracted, pinned `node.exe` outside of any Go/PowerShell wrapper to isolate the failure to Node itself before deciding on the glob fix.

## Known Stubs

None. `cli.ts`'s `main()` (the real `LinearSdkAdapter`/`LINEAR_API_KEY` process entrypoint) is fully wired but never exercised by any test in this plan — it requires a real Linear API key and is intentionally out of this plan's offline scope, matching CONTEXT D-21 ("synchronization failures... are reported without blocking local planning, builds, tests, or application runtime") and the Manual-Only Verification row in `01-VALIDATION.md` for a reviewed real Linear preview/apply/replay.

## Threat Flags

None beyond this plan's own declared threat model (T-01-36, T-01-37, T-01-38), which is fully mitigated as designed: the isolated-prefix `npm ci --offline` acceptance (T-01-36) proves cache completeness independent of any existing install; the fixture-driven fake-SDK contract test (T-01-37) exercises `adapter.ts`'s exact per-entity method dispatch and field mapping; and the exact 5-call-per-entity transcript assertion (T-01-38) proves every hierarchy operation's recorded outcome and sequence completeness. `cli.ts` introduces no new network surface (structurally verified: no `node:http`/`node:https`/`node:net` import, asserted by a dedicated test).

## User Setup Required

None — everything is repository-local and offline after the one-time toolchain bootstrap already required by prior plans (verified working live in this worktree). No credentials, live Linear access, or manual configuration required to exercise any test or acceptance script in this plan.

## Next Phase Readiness

- Plan 01-14 (pagination) and Plan 01-26/01-27 (transport errors/rate limits/Node process boundary) can extend `protocol.ts`/`adapter.ts`/`cli.ts` in place; `cli.ts`'s `OperationExecutor` interface and `runCLI` NDJSON loop are ready to be driven by a real subprocess from Go's `internal/trace/transport` without any redesign.
- `tests/fixtures/linear/hierarchy-operations.json` and `operations.test.ts`'s `FakeLinearClient` pattern are reusable for any future Node-side fixture-driven contract test in this workspace.
- `golc.ps1`'s `Test-LinearSyncNpmInstallMatches` skip contract and `tests/acceptance/bootstrap-node.ps1`'s three-stage pattern are now standing, verified gates any later plan touching `tools/linear-sync`'s dependencies can rely on.

## Self-Check: PASSED

- All five created files verified present on disk: `tools/linear-sync/src/cli.ts`, `tools/linear-sync/src/ambient-node.d.ts`, `tools/linear-sync/test/operations.test.ts`, `tests/fixtures/linear/hierarchy-operations.json`, `tests/acceptance/bootstrap-node.ps1` (all `FOUND`).
- Commit `4cc8b1a` (feat) verified present in `git log --oneline` on branch `worktree-agent-a5a8d649a5423bb96`; `git diff --diff-filter=D --name-only 4cc8b1a~1 4cc8b1a` reports zero deleted files.
- The plan's exact `<verify>` command — `powershell -NoProfile -File .\tests\acceptance\bootstrap-node.ps1` then `powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-sdk-operations` — both exit 0, run live end-to-end multiple times in this worktree.
- `go build ./...`, `go vet ./...`, `gofmt -l internal/command/linear_sync.go` (no findings), and the full `golc.ps1 test` (Go + the linear-sdk-operations Node scope) all exit 0/clean. `golc.ps1 check --offline`, `golc.ps1 generate --check`, `golc.ps1 build --scope linear-sdk`, `tests/acceptance/bootstrap.ps1`, `tests/acceptance/offline.ps1 -Mode core`, and `tests/acceptance/command-parity.ps1` were all re-run as a final regression pass and pass cleanly.

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-21*
