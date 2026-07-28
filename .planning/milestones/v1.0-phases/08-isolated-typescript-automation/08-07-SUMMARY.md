---
phase: 08-isolated-typescript-automation
plan: 07
subsystem: scripting
tags: [go, deno, typescript, static-analysis, typecheck]

# Dependency graph
requires:
  - phase: 08-isolated-typescript-automation
    provides: "script.ResolveDenoExecutable (08-02), scriptsdk generated golc.d.ts/golc-runtime.ts + RegisteredSDKMethods/RegisteredExclusions (08-03), internal/script host.go's forbiddenDenoArgPrefixes + session.go's boundedBuffer/materialize-shim-plus-source pattern + script run CLI route shape (08-05)"
provides:
  - "internal/script/diagnostics.go: Diagnostic/ValidationResult, sortDiagnostics"
  - "internal/script/validate.go: Validate (size bound -> zero-import gate -> deno check), checkForbiddenModuleSyntax, stripCommentsAndStringLiterals, buildDenoCheckArgs, shimLineOffsetFor, parseDenoCheckDiagnostics"
  - "internal/command/scriptvalidate.go: script validate CLI route"
affects: [08-08, 08-09, 08-10, 08-11]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "internal/script/host.go's forbiddenDenoArgPrefixes list reused directly (not re-declared) to assert buildDenoCheckArgs never carries a permission-granting flag, the same list buildDenoArgs is bound by"
    - "internal/script/session.go's boundedBuffer/newBoundedBuffer reused directly for deno check's captured stdout/stderr, instead of a second bounded-buffer implementation"
    - "a small hand-rolled character-scanner (stripCommentsAndStringLiterals) strips comments/strings/template-raw content before any regex-based keyword scan runs, so the regex step can never false-positive on a comment or string, per 08-RESEARCH.md Pitfall 4's explicit instruction"

key-files:
  created:
    - internal/script/diagnostics.go
    - internal/script/diagnostics_test.go
    - internal/script/validate.go
    - internal/script/validate_test.go
    - internal/command/scriptvalidate.go
    - internal/command/scriptvalidate_test.go
  modified:
    - internal/scriptsdk/descriptors.go

key-decisions:
  - "checkForbiddenModuleSyntax flags any surviving whole-word occurrence of the reserved keywords \"import\" or \"export\" after comment/string/template-raw stripping, rather than position-anchoring to statement starts -- both keywords are reserved in TypeScript and can never legitimately appear as an identifier, so any occurrence in cleaned source is definitionally the keyword, and this also catches a dynamic import(...) or a from-re-export without a second set of position-specific rules."
  - "Template literal substitutions (`${...}`) are the one region stripCommentsAndStringLiterals never blanks: they are real, executable TypeScript, so a dynamic import hidden inside one (`` `${await import(\"evil\")}` ``) cannot bypass the gate by hiding behind backticks."
  - "Validate() materializes its own shim+source file and reads the committed internal/scriptsdk/generated/golc.d.ts from disk (root-relative) rather than sharing a literal function with session.go's Run -- byte-identical output via the same scriptsdk.RuntimeShimTS constant and concatenation formula, without modifying session.go (outside this plan's declared file list)."
  - "deno.json's compilerOptions.types is the mechanism chosen for ambient .d.ts resolution during deno check (per the plan's explicit instruction); this could not be verified against a real deno check invocation in this environment (no provisioned Deno toolchain) -- documented as an action item in deferred-items.md."

requirements-completed: [SCRP-01, SCRP-03]

coverage:
  - id: D1
    description: "checkForbiddenModuleSyntax rejects a static import statement, a from re-export, a bare export declaration, and a dynamic import(...) expression -- including one hidden inside a template literal substitution -- each with GOLC_SCRIPT_IMPORT_FORBIDDEN and the correct line number, with zero false positives on comments/strings, before any subprocess is ever spawned."
    requirement: "SCRP-01, SCRP-03"
    verification:
      - kind: unit
        ref: "internal/script/validate_test.go#TestCheckForbiddenModuleSyntaxIgnoresCommentsAndStrings"
        status: pass
      - kind: unit
        ref: "internal/script/validate_test.go#TestCheckForbiddenModuleSyntaxDetectsStaticImport"
        status: pass
      - kind: unit
        ref: "internal/script/validate_test.go#TestCheckForbiddenModuleSyntaxDetectsDynamicImport"
        status: pass
      - kind: unit
        ref: "internal/script/validate_test.go#TestCheckForbiddenModuleSyntaxDetectsExportDeclaration"
        status: pass
      - kind: unit
        ref: "internal/script/validate_test.go#TestCheckForbiddenModuleSyntaxDetectsReexportFrom"
        status: pass
      - kind: unit
        ref: "internal/script/validate_test.go#TestCheckForbiddenModuleSyntaxDetectsImportHiddenInTemplateSubstitution"
        status: pass
      - kind: unit
        ref: "internal/script/validate_test.go#TestValidateModuleGateNeverSpawnsSubprocess"
        status: pass
      - kind: unit
        ref: "internal/script/validate_test.go#TestValidateSizeGateNeverSpawnsSubprocess"
        status: pass
    human_judgment: false
  - id: D2
    description: "deno check type-checks the materialized shim+source file against the generated golc.d.ts with --no-prompt --cached-only and no permission-granting flag, and every reported diagnostic line is corrected by the materialized shim's actual line count (never hardcoded) back into the user's own source coordinates, with an in-shim position reported under the distinct GOLC_SCRIPT_SDK_SHIM_ERROR code."
    requirement: "SCRP-01, SCRP-03"
    verification:
      - kind: unit
        ref: "internal/script/validate_test.go#TestBuildDenoCheckArgs"
        status: pass
      - kind: unit
        ref: "internal/script/validate_test.go#TestBuildDenoCheckArgsHasNoAllowFlags"
        status: pass
      - kind: unit
        ref: "internal/script/validate_test.go#TestShimLineOffsetForDerivedFromShimContent"
        status: pass
      - kind: unit
        ref: "internal/script/validate_test.go#TestParseDenoCheckDiagnosticsSubtractsShimOffset"
        status: pass
      - kind: unit
        ref: "internal/script/validate_test.go#TestParseDenoCheckDiagnosticsFlagsPositionInsideShim"
        status: pass
      - kind: unit
        ref: "internal/script/validate_test.go#TestParseDenoCheckDiagnosticsOffsetDerivedFromMaterializedFileNotHardcoded"
        status: pass
      - kind: integration
        ref: "internal/script/validate_test.go#TestValidateCleanScriptReportsZeroDiagnostics"
        status: skip
      - kind: integration
        ref: "internal/script/validate_test.go#TestValidateWrongFieldTypeReportsDiagnostic"
        status: skip
      - kind: integration
        ref: "internal/script/validate_test.go#TestValidateUnknownMethodReportsDiagnostic"
        status: skip
    human_judgment: false
  - id: D3
    description: "golc-project script validate <name> --show <path> exits 0 with {\"valid\":true,\"diagnostics\":[]} for a clean script and exits 1 with the diagnostic array for a failing one; script validate Missing --show <path> exits 1 with GOLC_SCRIPT_NOT_FOUND; a malformed invocation exits 2; \"script validate\" is itself excluded from the SDK a running script can call."
    requirement: "SCRP-01"
    verification:
      - kind: integration
        ref: "internal/command/scriptvalidate_test.go#TestScriptValidateNotFoundNeverSpawnsProcess"
        status: pass
      - kind: integration
        ref: "internal/command/scriptvalidate_test.go#TestScriptValidateShowMissingNeverSpawnsProcess"
        status: pass
      - kind: integration
        ref: "internal/command/scriptvalidate_test.go#TestScriptValidateMalformedInvocationExitsTwo"
        status: pass
      - kind: integration
        ref: "internal/command/scriptvalidate_test.go#TestScriptValidateForbiddenImportNeverSpawnsProcess"
        status: pass
      - kind: integration
        ref: "internal/command/scriptvalidate_test.go#TestScriptValidateClassifiedAsExcluded"
        status: pass
      - kind: integration
        ref: "internal/command/scriptvalidate_test.go#TestScriptValidateCleanScript"
        status: skip
      - kind: integration
        ref: "internal/command/scriptvalidate_test.go#TestScriptValidateWrongFieldTypeScript"
        status: skip
      - kind: unit
        ref: "internal/command/scriptsdk_parity_test.go#TestEveryDeclaredRouteIsClassified"
        status: pass

# Metrics
duration: ~50min
completed: 2026-07-26
status: complete
---

# Phase 8 Plan 7: Script Validation (Zero-Import Gate + Type Check) Summary

**SCRP-01's validate verb: a hand-rolled comment/string-aware structural zero-import gate followed by a `deno check` type-check against the generated SDK, with every diagnostic mapped back into the author's own source line numbers via a computed (never hardcoded) shim offset.**

## Performance

- **Duration:** ~50 min
- **Tasks:** 2 completed
- **Files modified:** 7 (6 created, 1 modified)

## Accomplishments
- `internal/script/diagnostics.go` declares `Diagnostic`/`ValidationResult`/`Severity` with a stable, snake_case JSON shape and `sortDiagnostics`, which orders every diagnostic by `(Line, Column, Code)` so `script validate`'s output is byte-stable across repeated runs of the identical script.
- `internal/script/validate.go`'s `checkForbiddenModuleSyntax` rejects a static import, a `from "..."` re-export, a bare `export` declaration, and a dynamic `import(...)` expression -- including one hidden inside a template literal substitution (`` `${await import("evil")}` ``) -- with zero false positives on comments or strings, via a hand-rolled character scanner (`stripCommentsAndStringLiterals`) rather than a regex over raw text.
- `Validate` composes the size bound, the zero-import gate, and (only if both pass) a `deno check` invocation with `check --no-prompt --cached-only <path>` and no permission-granting flag, sharing `host.go`'s `forbiddenDenoArgPrefixes` list and `session.go`'s `boundedBuffer`. Every `deno check` diagnostic line is corrected by a shim-line offset computed from the materialized shim's actual content (`shimLineOffsetFor`), never a hardcoded constant, with an in-shim position reported under the distinct `GOLC_SCRIPT_SDK_SHIM_ERROR` code.
- `internal/command/scriptvalidate.go` wires `script validate <name> --show <path>` through `script.Validate`: exit 0 with `{"valid":true,"diagnostics":[]}` for a clean script, exit 1 with the diagnostic array for a failing one, exit 2 on a malformed invocation.
- `internal/scriptsdk/descriptors.go` classifies `script validate` as an excluded route (script lifecycle control), keeping `TestEveryDeclaredRouteIsClassified` green.

## Task Commits

Each task was committed atomically as a TDD RED/GREEN pair:

1. **Task 1: Structural zero-import gate and the diagnostic model**
   - `5d2e6dd` (test) - RED: failing coverage for Diagnostic/ValidationResult and checkForbiddenModuleSyntax
   - `817b11c` (feat) - GREEN: internal/script/diagnostics.go + validate.go's module-gate half
2. **Task 2: deno check type validation with source-coordinate mapping, and the `script validate` route**
   - `a636880` (test) - RED: failing coverage for buildDenoCheckArgs/parseDenoCheckDiagnostics/Validate/script validate route
   - `15f9730` (feat) - GREEN: validate.go's Validate/buildDenoCheckArgs/shimLineOffsetFor/parseDenoCheckDiagnostics, internal/command/scriptvalidate.go, internal/scriptsdk/descriptors.go exclusion

**Plan metadata:** committed by the wave orchestrator after merge (worktree mode: this agent does not update STATE.md/ROADMAP.md).

_Both tasks have a separate RED (test) commit before their GREEN (feat) commit, per the plan's `tdd="true"` gate; no REFACTOR commit was needed._

## Files Created/Modified
- `internal/script/diagnostics.go` - `Severity`/`Diagnostic`/`ValidationResult`, `sortDiagnostics`
- `internal/script/diagnostics_test.go` - stable JSON shape + sort-order coverage
- `internal/script/validate.go` - `checkSourceSize`, `checkForbiddenModuleSyntax`, `stripCommentsAndStringLiterals`, `Validate`, `buildDenoCheckArgs`, `shimLineOffsetFor`, `parseDenoCheckDiagnostics`, `redactLines`
- `internal/script/validate_test.go` - module-gate false-positive/detection coverage, shim-offset math, real-Deno-gated type-check coverage (skips cleanly when `.tools/toolchains/deno/` is unprovisioned)
- `internal/command/scriptvalidate.go` - `script validate` route, view projection
- `internal/command/scriptvalidate_test.go` - not-found/show-missing/malformed/forbidden-import/excluded-from-scriptsdk coverage plus real-Deno-gated clean/wrong-field-type coverage
- `internal/scriptsdk/descriptors.go` - classifies `"script validate"` in `excludedRouteTable`

## Decisions Made
- `checkForbiddenModuleSyntax` flags any surviving whole-word `import`/`export` occurrence after comment/string/template-raw stripping (word-boundary regex over cleaned text), rather than statement-position anchoring -- both are reserved TypeScript keywords that can never be a real identifier, so this single rule covers static imports, dynamic imports, bare exports, and `from`-re-exports without separate position-specific logic.
- `stripCommentsAndStringLiterals` deliberately does **not** blank a template literal's `${...}` substitution content -- it is real, executable code, so a dynamic import hidden inside one cannot bypass the gate by hiding behind backticks (verified by `TestCheckForbiddenModuleSyntaxDetectsImportHiddenInTemplateSubstitution`).
- `Validate` re-materializes the shim+source file independently in `internal/script/validate.go` (using the same `scriptsdk.RuntimeShimTS` constant and `shim + "\n" + source` formula `session.go`'s `Run` uses) rather than extracting a shared helper into `session.go` -- `session.go` is not in this plan's declared `files_modified` list, and the two call sites already produce byte-identical output by construction since both concatenate the identical embedded constant the same way.
- `deno.json`'s `compilerOptions.types` was chosen as the ambient-`.d.ts` resolution mechanism for `deno check`, per the plan's explicit instruction. This could not be verified against a real `deno check` invocation in this environment (no provisioned Deno toolchain) -- logged as an action item in `deferred-items.md`, with the single call site to change (`denoCheckConfig` in `validate.go`) named if it needs to switch to a triple-slash reference comment instead.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2/3 - Missing critical / blocking] Classified "script validate" in internal/scriptsdk/descriptors.go, outside the plan's declared files_modified list**
- **Found during:** Task 2, while implementing the `script validate` CLI route
- **Issue:** The plan's frontmatter `files_modified` list omits `internal/scriptsdk/descriptors.go`, but the plan's own Task 2 action text explicitly requires classifying `"script validate"` in `scriptsdk`'s `excludedRoutes`, and the plan's own verification step (`TestEveryDeclaredRouteIsClassified`) fails the build without it -- a newly declared route with no SDK classification is a hard build-breaking gap by design (`scriptsdk_parity_test.go`'s stated purpose).
- **Fix:** Added `"script validate": "script lifecycle control; a script must not validate or introspect other scripts through the SDK"` to `excludedRouteTable`.
- **Files modified:** internal/scriptsdk/descriptors.go
- **Commit:** `15f9730`

---

**Total deviations:** 1 auto-fixed (1 missing critical/blocking, required by the plan's own explicit action text and verification step)
**Impact on plan:** Necessary for correctness against the plan's own stated behavior and verification; the `files_modified` list omission appears to be an oversight in the plan document, not an intentional scope boundary. No scope creep beyond what the plan's own Task 2 action text already specified.

## Issues Encountered
- Same five pre-existing toolchain-bootstrap failures as 08-01/08-03/08-05 (`TestBuildRouteCompilesTheProductionRepository`, `TestBuildablePackagesExcludesMagefiles`, `TestScopeCrossPlatformCI`, `TestScopeGreenSubprocess`, `TestScopeOfflineAcceptance`), unrelated to this plan's changes. `go test ./internal/script/... ./internal/scriptsdk/...` is fully green; `go test ./internal/command/...` is green except these five pre-existing failures.
- This worktree's `.tools/toolchains/deno/` is a partial/unverified install (same condition 08-05 recorded: `GOLC_DENO_TOOLCHAIN_MISSING: verified install does not match pin`), so every test spawning a real `deno check` subprocess skips cleanly rather than running for real: `internal/script`'s `TestValidateCleanScriptReportsZeroDiagnostics`, `TestValidateWrongFieldTypeReportsDiagnostic`, `TestValidateUnknownMethodReportsDiagnostic`, and `internal/command`'s `TestScriptValidateCleanScript`, `TestScriptValidateWrongFieldTypeScript`. The plan's acceptance criteria ask for real transcripts of `golc-project script validate` against a clean and a deliberately broken script to be recorded here -- this could not be captured in this environment. The zero-subprocess-spawn guarantee for both gates (size bound, zero-import) is independently proven in this environment without any mocking (`TestValidateModuleGateNeverSpawnsSubprocess`/`TestValidateSizeGateNeverSpawnsSubprocess` run against a real, unprovisioned root and assert no `GOLC_SCRIPT_DENO_MISSING` error surfaces), and the shim-offset math is independently unit-tested against hand-crafted fixture text mirroring `deno check`'s documented diagnostic format -- but that fixture format, and the `deno.json` ambient-types mechanism, were not confirmed against a live `deno check` process. Full details and the required re-verification steps are logged in `.planning/phases/08-isolated-typescript-automation/deferred-items.md` (08-07 section).

## User Setup Required

None - no external service configuration required. (Re-running the Deno-gated tests requires `mage Bootstrap` with a matching Deno 2.9.4 pin, as documented in deferred-items.md.)

## Next Phase Readiness
- `internal/script`'s `Validate`/`Diagnostic`/`ValidationResult` and `script validate` are the validation surface 08-11's editor integration renders inline diagnostics from (D-15's Monaco integration), and the diagnostic count feeds the UI-SPEC's "This script has {N} error(s). Fix them before running." summary row.
- Before relying on this plan's validate verb in production, re-run `go test ./internal/script/... ./internal/command/... -count=1` on a machine where `mage Bootstrap` (matching Deno 2.9.4 pin) has completed, and confirm the `deno check` diagnostic-parsing regexes and the `deno.json` ambient-types mechanism actually match that Deno version's real output -- both are documented, single-call-site changes if they need adjustment (`.planning/phases/08-isolated-typescript-automation/deferred-items.md`, 08-07 section).
- No blockers for 08-08/08-09/08-10/08-11.

## Self-Check: PASSED

- FOUND: internal/script/diagnostics.go
- FOUND: internal/script/diagnostics_test.go
- FOUND: internal/script/validate.go
- FOUND: internal/script/validate_test.go
- FOUND: internal/command/scriptvalidate.go
- FOUND: internal/command/scriptvalidate_test.go
- FOUND: internal/scriptsdk/descriptors.go (modified)
- FOUND: .planning/phases/08-isolated-typescript-automation/08-07-SUMMARY.md
- FOUND: .planning/phases/08-isolated-typescript-automation/deferred-items.md (08-07 section appended)
- FOUND commit: 5d2e6dd (test: diagnostic model + zero-import gate RED)
- FOUND commit: 817b11c (feat: diagnostic model + zero-import gate GREEN)
- FOUND commit: a636880 (test: deno check validation + script validate route RED)
- FOUND commit: 15f9730 (feat: deno check validation + script validate route GREEN)
- `go build ./internal/... ./cmd/golc-project/...`: PASS
- `go vet ./internal/... ./cmd/golc-project/...`: PASS
- `go test ./internal/script/... ./internal/scriptsdk/...`: PASS
- `go test ./internal/command/...`: PASS (5 pre-existing unrelated failures + 5 real-Deno-gated skips, both logged in deferred-items.md)
- `grep -c 'cached-only' internal/script/validate.go`: 3 (>= 1 required)

---
*Phase: 08-isolated-typescript-automation*
*Completed: 2026-07-26*
