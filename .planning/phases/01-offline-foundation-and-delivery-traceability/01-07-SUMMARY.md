---
phase: 01-offline-foundation-and-delivery-traceability
plan: 07
subsystem: security
tags: [go, powershell, secrets, redaction, github-actions, ci, command-parity]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 06
    provides: internal/delivery's LoadGraph/Run/RunOffline offline core graph and internal/command/check.go's self-registered "check" route
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 20
    provides: internal/delivery.BuildFoundationBundle/FoundationInventory/CanonicalManifest/EncodeManifest and the "package --foundation" route
provides:
  - internal/security.SafeDiagnostic/Redact/SetState/ScanCanary/ScanCanaryAll — the single centralized allowlisted-diagnostic and fake-secret canary-scan contract
  - .env.example — safe, empty LINEAR_API_KEY/LINEAR_TEAM_ID placeholders, committed and trackable
  - internal/command/check.go's "check --concern project" canary scan over every generated schema and the committed Linear map
  - internal/command/check.go's "check --command-parity" route — proves the Windows PR workflow matches config/commands.toml's commands.pr.steps graph and is credential/mutation-free
  - config/commands.toml's [commands.pr] policy (steps/network_steps/mutation_steps) — the single authoritative PR command graph
  - .github/workflows/check.yml — least-privilege, pull_request-only Windows CI running the exact contributor root graph
  - tests/acceptance/command-parity.ps1 — offline, credential-free acceptance for command-parity
affects: [ci, delivery, security, configuration]

tech-stack:
  added: []
  patterns:
    - "internal/security is the single centralized owner of secret-safe rendering: SafeDiagnostic never accepts a raw environment/header/config/exception object (its Fields are map[string]string only), and every field value is re-passed through Redact at render time so constructing a SafeDiagnostic directly with an unvetted value can never bypass the canary/pattern scan."
    - "ScanCanary/ScanCanaryAll operate on raw []byte so stdout/stderr captures, generated schema/map/report/manifest files, and raw ZIP archive bytes are all scanned identically — no output surface receives special-cased treatment."
    - "internal/security/redact_test.go is the external package security_test (not internal package security) because internal/command/check.go imports internal/security for its own canary scan — declaring the quick-test scope from an internal test file would close a security[test] -> command -> security import cycle, the same shape internal/delivery/delivery_test.go's package doc already documents and avoids identically."
    - "config/commands.toml's new [commands.pr] policy stays flat, pattern-matched, comma-separated scalar strings (not a TOML array/table) because internal/projectconfig's strict single-authority decoder does not decode array-valued keys — the same flat-scalar precedent config/toolchain.toml's per-tool official_host/official_path_prefix keys already establish."
    - "internal/command/check.go's single self-registered \"check\" route serves three mutually exclusive forms (--concern <name>, --offline, --command-parity) via strict in-handler flag parsing, following the same dash-word precedent generate.go/test.go already establish (router.go's route-word grammar forbids a route word beginning with a dash)."
    - "check --command-parity is the single authority that parses both config/commands.toml's commands.pr.steps and .github/workflows/check.yml's own golc.ps1 invocations; tests/acceptance/command-parity.ps1 never duplicates that inventory — it only invokes the route and asserts success, mirroring tests/acceptance/offline.ps1's Invoke-Golc/Assert-GolcSucceeded pattern."

key-files:
  created:
    - internal/security/redact.go
    - internal/security/redact_test.go
    - .env.example
    - .github/workflows/check.yml
    - tests/acceptance/command-parity.ps1
  modified:
    - internal/command/check.go
    - .gitignore
    - config/commands.toml
    - internal/projectconfig/model.go

key-decisions:
  - "CanaryToken plus a fixed forbidden-pattern list (LINEAR_API_KEY=, Bearer , sk-, lin_api_) are scanned byte-for-byte across stdout/stderr, generated schemas, the Linear map, a synthesized apply report, and a synthesized foundation manifest/ZIP — proving detection actually works (a planted token is caught) rather than only ever observing already-clean output."
  - "commands.pr.steps/network_steps/mutation_steps were added to internal/projectconfig/model.go's existing \"commands\" concern KeySpec registry (outside this plan's stated files_modified) because the strict decoder rejects any unregistered key in a concern file; see Deviations."
  - ".github/workflows/check.yml's only trigger is pull_request, with no secrets/env block referencing Linear and no Linear synchronization command anywhere in the file, satisfying D-16's PR-CI-is-never-mutation-capable boundary independently of check --command-parity's own scan."

requirements-completed: [CONF-03, CONF-04, LINR-04]

coverage:
  - id: D1
    description: "internal/security centralizes allowlisted diagnostics (SafeDiagnostic/Redact/SetState) and a cross-artifact fake-secret canary scan (ScanCanary/ScanCanaryAll), wired into \"check --concern project\", and .env.example commits only safe empty LINEAR_API_KEY/LINEAR_TEAM_ID placeholders while .env stays ignored."
    requirement: CONF-04
    verification:
      - kind: unit
        ref: "internal/security/redact_test.go#TestScopeSecrets"
        status: pass
      - kind: integration
        ref: "powershell -NoProfile -File .\\golc.ps1 test --quick --scope secrets"
        status: pass
      - kind: manual_procedural
        ref: "git check-ignore .env.example (exit 1, not ignored) vs git check-ignore .env (exit 0, ignored)"
        status: pass
    human_judgment: false
  - id: D2
    description: "check --command-parity proves .github/workflows/check.yml invokes exactly config/commands.toml's commands.pr.steps graph in order and contains no Linear secret reference, remote trigger, or apply-capable command; tests/acceptance/command-parity.ps1 verifies this offline with no Linear credential present."
    requirement: LINR-04
    verification:
      - kind: integration
        ref: "powershell -NoProfile -File .\\tests\\acceptance\\command-parity.ps1"
        status: pass
      - kind: integration
        ref: "powershell -NoProfile -File .\\golc.ps1 check --command-parity"
        status: pass
    human_judgment: false

duration: ~17min
completed: 2026-07-20
status: complete
---

# Phase 1 Plan 07: Secret-Free Delivery and PR Command Parity Summary

**Centralized fake-secret canary scanning (internal/security) wired into `check --concern project`, plus a new `check --command-parity` route proving the least-privilege Windows PR workflow exactly matches `config/commands.toml`'s authoritative command graph with zero secret or mutation reachability**

## Performance

- **Duration:** ~17 min
- **Started:** 2026-07-20T22:22:25-07:00
- **Completed:** 2026-07-20T22:39:02-07:00
- **Tasks:** 2
- **Files modified:** 10 (7 created, 3 modified beyond the deviation file; 4 modified in total counting the deviation)

## Accomplishments

- `internal/security` is the new single centralized owner of secret-safe rendering and detection: `SafeDiagnostic` (a code/message/`map[string]string` fields shape that structurally cannot carry a raw environment, header, config, or exception object), `Redact`/`SetState` (never echo an underlying value), and `ScanCanary`/`ScanCanaryAll` (byte-oriented fake-secret/forbidden-pattern detection usable identically across stdout/stderr, generated schemas, maps, reports, manifests, and ZIP archive bytes).
- `internal/security/redact_test.go` (`TestScopeSecrets`) proves the scan actually detects a planted `CanaryToken` — not just that already-clean output stays clean — across real captured `check --concern project` stdout/stderr, every committed generated schema, the committed Linear map, a synthesized `apply.Report`, and a synthesized foundation manifest/ZIP built through `internal/delivery`'s real primitives.
- `internal/command/check.go`'s `check --concern project` now scans every generated schema plus the committed Linear map for fake-secret bytes before reporting success (T-01-18 mitigation).
- `.env.example` commits safe, empty `LINEAR_API_KEY`/`LINEAR_TEAM_ID` declarations with optional-remote comments only; `.gitignore`'s pre-existing `.env.*` rule was also silently matching `.env.example` itself, so `!.env.example` was added to keep it trackable (D-19).
- `config/commands.toml` gained a `[commands.pr]` policy — `steps` (the exact ordered `bootstrap,generate --check,check --offline,build,test,package --foundation` graph), `network_steps` (`bootstrap` only), and `mutation_steps` (`none`) — as the single authoritative source both the workflow and its acceptance script are checked against.
- `internal/command/check.go`'s single self-registered `check` route gained a third mutually-exclusive form, `check --command-parity`: it parses `commands.pr.steps`, parses `.github/workflows/check.yml`'s own ordered `golc.ps1` invocations, fails on the first mismatched/missing step with a stable diagnostic, and scans the workflow for a forbidden Linear secret reference, non-`pull_request` trigger, or apply-capable command (`linear apply`, `linear archive`, `linear unlink`, `linear map migrate --write`).
- `.github/workflows/check.yml` is a least-privilege Windows job triggered only by `pull_request`, `permissions: contents: read`, running exactly `bootstrap`, `generate --check`, `check --offline`, `build`, `test`, `package --foundation` through `golc.ps1` — no secret, env, or Linear reference anywhere in the file.
- `tests/acceptance/command-parity.ps1` confirms `LINEAR_API_KEY`/`LINEAR_TEAM_ID` are absent from the process environment, then invokes `check --command-parity` and asserts success — proving parity and mutation-unreachability offline, without credentials, mirroring `tests/acceptance/offline.ps1`'s `Invoke-Golc`/`Assert-GolcSucceeded` pattern exactly.

## Task Commits

1. **Task 1: Enforce safe examples and secret-free output** - `512ed73` (feat)
2. **Task 2: Prove root-command parity and prohibit PR mutation** - `5cc7dd5` (feat)

**Plan metadata:** committed with this summary

## Files Created/Modified

- `internal/security/redact.go` - `SafeDiagnostic`, `Redact`, `SetState`, `ScanCanary`, `ScanCanaryAll`, `CanaryToken`, `CanaryViolation`.
- `internal/security/redact_test.go` - `TestScopeSecrets`: canary/pattern detection, `SetState`/`Redact`/`SafeDiagnostic.String` allowlisting, and zero-leak proofs across real command output, committed schemas/map, a synthesized report, and a synthesized foundation manifest/ZIP.
- `.env.example` - Safe, empty `LINEAR_API_KEY`/`LINEAR_TEAM_ID` declarations with optional-remote comments.
- `.gitignore` - Added `!.env.example` so the safe example stays trackable beside the still-ignored `.env`/`.env.*`.
- `internal/command/check.go` - `runProjectCheck` now scans generated schemas + the Linear map for canary violations (`canaryScanSources`); the `check` route gained `--command-parity` (`runCheckCommandParity`, `parsePRStepPolicy`, `extractWorkflowSteps`, `compareStepSequences`) alongside `--concern`/`--offline`.
- `config/commands.toml` - Added `[commands.pr]` (`steps`, `network_steps`, `mutation_steps`).
- `.github/workflows/check.yml` - New least-privilege, `pull_request`-only Windows PR workflow.
- `tests/acceptance/command-parity.ps1` - New offline, credential-free command-parity acceptance script.
- `internal/projectconfig/model.go` - Registered `commands.pr.steps`/`commands.pr.network_steps`/`commands.pr.mutation_steps` in the `commands` concern's `KeySpec` registry (deviation; see below).

## Decisions Made

See `key-decisions` in the frontmatter above for: (1) the canary/forbidden-pattern scan design and why it proves actual detection rather than only observing clean output, (2) registering the new PR-policy keys in `internal/projectconfig/model.go`, and (3) the workflow's single `pull_request` trigger with no Linear reference anywhere in the file.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Registered `commands.pr.*` keys in `internal/projectconfig/model.go`**
- **Found during:** Task 2 (config/commands.toml's `[commands.pr]` policy)
- **Issue:** `internal/projectconfig`'s strict single-authority concern decoder (`decode.go`) rejects any key present in a concern file that is not explicitly registered in that concern's `KeySpec` map — the plan's stated `files_modified` list for this task did not include `internal/projectconfig/model.go`, but adding `[commands.pr]` to `config/commands.toml` without registering its three new keys would make every `check --concern project` run (and thus `check --offline`, and thus every quick-test-gated task) fail closed with `GOLC_CONFIG_UNKNOWN_KEY`.
- **Fix:** Added `commands.pr.steps`, `commands.pr.network_steps`, and `commands.pr.mutation_steps` to the existing `commands` concern's `Keys` map, each with a new flat pattern (`prStepListPattern`, `prStepNamesPattern`) following the exact same flat-scalar precedent every other concern (and `config/toolchain.toml`'s per-tool keys specifically) already establishes — no new concern, no array/table value, no change to the single-authority model itself.
- **Files modified:** `internal/projectconfig/model.go`
- **Verification:** `check --concern project`, `check --offline`, `generate --check`, and the full `go test ./...` suite all pass after the change; `check --command-parity` and `tests/acceptance/command-parity.ps1` both exit 0.
- **Committed in:** `5cc7dd5` (Task 2 commit)

**2. [Rule 1 - Bug] Fixed `.gitignore` silently un-tracking `.env.example`**
- **Found during:** Task 1 (`.env.example`)
- **Issue:** The pre-existing `.gitignore` rule `.env.*` (intended to match variants like `.env.local`) also matched the literal committed safe example `.env.example` itself, which would have made `git add .env.example` silently fail to stage it — directly violating D-19 ("the repository commits `.env.example`").
- **Fix:** Added `!.env.example` immediately after the `.env`/`.env.*` rules to re-include it, matching this plan's own `Artifacts this phase produces` description ("Modified `.gitignore`: retains `.env` exclusion and explicitly keeps `.env.example` trackable").
- **Files modified:** `.gitignore`
- **Verification:** `git check-ignore .env.example` exits 1 (not ignored); `git check-ignore .env` exits 0 (ignored); `git ls-files` confirms `.env.example` is tracked after commit.
- **Committed in:** `512ed73` (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug)
**Impact on plan:** Both auto-fixes were necessary for the plan's own explicitly stated artifacts/acceptance criteria to actually pass. No scope creep — no new concern, table, service, or architecture was introduced.

## Issues Encountered

- The worktree's `.tools/` toolchain cache did not exist at task start (gitignored, worktree-local); `golc.ps1 bootstrap` was run once to provision it (network was reachable in this environment) before any `golc.ps1` subcommand could execute. `golc.ps1 bootstrap` was re-run once after each Go source change in this plan, since `golc.ps1`'s dispatcher always delegates to the previously-built `.tools/installs/golc_project/bin/golc-project.exe` and only `bootstrap` rebuilds it from current source.

## Known Stubs

None - both `check --command-parity` and the canary scan integration in `check --concern project` run against real repository artifacts (the actual committed `.github/workflows/check.yml`, `config/commands.toml`, generated schemas, and `.planning/linear-map.json`), not placeholders or mocked data.

## User Setup Required

None - both tasks are pure offline Go/PowerShell operations over the already-bootstrapped repository-local toolchain; no credential or external service is involved. `tests/acceptance/command-parity.ps1` explicitly asserts no Linear credential is present before it runs.

## Next Phase Readiness

- `internal/security.SafeDiagnostic`/`Redact`/`SetState`/`ScanCanary`/`ScanCanaryAll` are stable, importable primitives any later plan needing safe diagnostic rendering or a fake-secret scan (for example the Linear transport/apply plans later in this phase) can reuse directly instead of re-implementing masking logic.
- `check --command-parity` and `tests/acceptance/command-parity.ps1` are ready for the CI provider to actually invoke `.github/workflows/check.yml` on a real pull request; the workflow itself has not yet been exercised by GitHub Actions in this session (no PR was opened), only its parity/content contract was verified locally.
- `01-VALIDATION.md`'s `01-07-01`/`01-07-02` per-task verification rows are both green: `golc.ps1 test --quick --scope secrets` and `tests/acceptance/command-parity.ps1` each exit 0.

## Self-Check: PASSED

- All created files verified present on disk: `internal/security/redact.go`, `internal/security/redact_test.go`, `.env.example`, `.github/workflows/check.yml`, `tests/acceptance/command-parity.ps1` (all `FOUND`); `git ls-files` confirms each is tracked.
- Commits `512ed73` and `5cc7dd5` verified present in `git log --oneline --all`; `git diff --diff-filter=D --name-only` against each commit's parent reports zero deleted files for both.
- `powershell -NoProfile -File .\golc.ps1 test --quick --scope secrets` exits 0 (marker `TestScopeSecrets` found and passing).
- `powershell -NoProfile -File .\tests\acceptance\command-parity.ps1` exits 0.
- `powershell -NoProfile -File .\golc.ps1 check --command-parity`, `check --concern project`, `check --offline`, `generate --check`, and the full `golc.ps1 test` suite (every package `ok`) all exit 0 after both tasks.
- `powershell -NoProfile -File .\tests\acceptance\offline.ps1 -Mode core` and `-Mode package` both remain green, confirming Task 2's changes did not regress the existing offline/foundation acceptance scripts.
- `git check-ignore -v` confirms `.env.example` is NOT ignored while `.env` remains ignored.

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-20*
