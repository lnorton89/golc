---
phase: 05-durable-shows-and-recovery
plan: 04
subsystem: database
tags: [sqlite, pragma-integrity-check, json-export, cli, database-sql, go]

# Dependency graph
requires:
  - phase: 05-durable-shows-and-recovery (05-01)
    provides: internal/show.openStore/Load/Save/LoadForRead (SQLite-backed .golc store), checkpointAndClose
provides:
  - internal/show/diagnose.go: Diagnose/DiagnosticReport combining PRAGMA integrity_check with structural validate() (via LoadForRead)
  - internal/command/show_diagnose.go: "show diagnose" and "show export" CLI routes on the existing "show" scope
affects: [05-05]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Diagnose reads SchemaVersion/Revision directly from show_meta (readMeta) independent of whether the structural check succeeds, so a structurally-invalid file still reports what schema_version/revision it claims to be"
    - "show export prints strictjson.CanonicalEncode(state) directly -- never buildShowInspectView's allowlisted show-inspect projection -- to satisfy D-13's 'same shape as today's canonical State JSON' requirement"
    - "Corruption test technique: pad a State with many pool members so the saved .golc spans multiple SQLite pages, then overwrite only the file's tail quarter with garbage -- well past page 1's header/sqlite_master schema -- so openStore's PRAGMA/application_id door checks still succeed and PRAGMA integrity_check gets a chance to walk into (and report on) the corrupted region"

key-files:
  created:
    - internal/show/diagnose.go
    - internal/show/diagnose_test.go
    - internal/command/show_diagnose.go
    - internal/command/show_diagnose_test.go
  modified: []

key-decisions:
  - "Diagnose calls LoadForRead as a second, separate connection for the structural check rather than reusing decodeAndValidate on the same db handle the integrity_check ran against -- keeps the file-level and structural checks independent (one query-level failure in either does not abort the other) and reuses LoadForRead's existing D-10 newer-than-supported tolerance verbatim instead of re-implementing it"
  - "DiagnosticReport.StructuralError is a string (not the error/string ambiguity the plan left open) so show_diagnose.go's JSON output stays a plain deterministic document with no encoding/json error-marshaling edge cases"
  - "show diagnose/show export share one GOLC_SHOW_DIAGNOSE_USAGE arg-parsing code (both routes take only --show <path>), mirroring deployment.go's precedent of one shared usage code across routes declared in the same file"

patterns-established:
  - "Diagnose is the reference shape for any future .golc read-only inspection command: open once for the file-level SQLite check, then delegate the structural check to the existing Load/LoadForRead entry point rather than re-decoding/re-validating inline"

requirements-completed: [SHOW-06]

coverage:
  - id: D1
    description: "show diagnose runs both PRAGMA integrity_check (file/page corruption) and structural show.validate() and prints a combined DiagnosticReport"
    requirement: SHOW-06
    verification:
      - kind: unit
        ref: "internal/show/diagnose_test.go#TestDiagnoseHealthyFile"
        status: pass
      - kind: unit
        ref: "internal/command/show_diagnose_test.go#TestShowDiagnoseHealthyExitZero"
        status: pass
    human_judgment: false
  - id: D2
    description: "show diagnose on a corrupted .golc reports the corruption (non-'ok' integrity_check lines and/or a failed structural validate) rather than crashing or reporting healthy"
    requirement: SHOW-06
    verification:
      - kind: unit
        ref: "internal/show/diagnose_test.go#TestDiagnoseFileCorruption"
        status: pass
      - kind: unit
        ref: "internal/command/show_diagnose_test.go#TestShowDiagnoseCorruptExitOne"
        status: pass
      - kind: unit
        ref: "internal/show/diagnose_test.go#TestDiagnoseStructurallyInvalid"
        status: pass
    human_judgment: false
  - id: D3
    description: "show export prints the versioned human-readable JSON byte-identical to strictjson.CanonicalEncode(State) -- the full canonical document, not show inspect's allowlisted projection -- and round-trips back into a fresh .golc via Load"
    requirement: SHOW-06
    verification:
      - kind: unit
        ref: "internal/command/show_diagnose_test.go#TestShowExportMatchesCanonicalEncode"
        status: pass
    human_judgment: false
  - id: D4
    description: "show diagnose and show export tolerate a newer-than-supported schema_version via LoadForRead and never rewrite the file; neither runs automatically on show open"
    requirement: SHOW-06
    verification:
      - kind: unit
        ref: "internal/command/show_diagnose_test.go#TestShowExportTooNewReadOnly"
        status: pass
    human_judgment: false
  - id: D5
    description: "PRAGMA integrity_check + validate() over a representative show completes well under a second at this app's scale (05-RESEARCH.md Pitfall 4), confirmed empirically rather than assumed"
    verification:
      - kind: unit
        ref: "internal/show/diagnose_test.go#TestDiagnoseCompletesUnderOneSecond"
        status: pass
    human_judgment: false

duration: 15min
completed: 2026-07-23
status: complete
---

# Phase 5 Plan 4: On-Demand Show Diagnostics and JSON Export Summary

**`show diagnose` combines SQLite `PRAGMA integrity_check` with the existing structural `show.validate()` into one report; `show export` prints the full canonical State document byte-identical to `strictjson.CanonicalEncode`, both read-only and D-10-tolerant of a newer-than-supported schema_version.**

## Performance

- **Duration:** 15 min
- **Started:** 2026-07-22T22:38:00-07:00 (approx.)
- **Completed:** 2026-07-22T22:53:00-07:00
- **Tasks:** 2 (RED/GREEN pairs collapsed per task; both tasks written test-and-implementation together and iterated to green)
- **Files modified:** 4 (all created)

## Accomplishments
- `internal/show/diagnose.go`: `Diagnose`/`DiagnosticReport` running the full (not `quick_check`) `PRAGMA integrity_check` for file-level corruption detection, plus a reused `LoadForRead` call for the structural check (D-11) -- never reimplements `validate()`. `SchemaVersion`/`Revision` are read from `show_meta` independent of whether the structural check succeeds, so a structurally-invalid file still reports its claimed schema_version/revision.
- `internal/command/show_diagnose.go`: registers `show diagnose` (exit 0 healthy / exit 1 on any file-level or structural issue, never a crash) and `show export` (prints `strictjson.CanonicalEncode(state)` directly, the full document, not `show inspect`'s allowlisted projection) on the already-declared `show` scope -- no duplicate scope registration.
- Empirically confirmed (via a corruption test that genuinely trips real `PRAGMA integrity_check` findings, verified with a temporary debug run before the final commit) that injected page-level corruption surfaces as descriptive `FileLevelIssues` text, not a query-level crash.
- Empirically confirmed `Diagnose` completes well under one second on a representative saved show (05-RESEARCH.md Pitfall 4's scale assumption, not merely assumed).
- `go test ./...` (full suite) passes.

## Task Commits

Each task was committed atomically:

1. **Task 1: Diagnose (PRAGMA integrity_check + structural validate)** - `fd51008` (feat)
2. **Task 2: show diagnose / show export CLI routes** - `1a14f94` (feat)

**Plan metadata:** pending (this commit)

## Files Created/Modified
- `internal/show/diagnose.go` - `DiagnosticReport`, `Diagnose(root, path string) (DiagnosticReport, error)`
- `internal/show/diagnose_test.go` - `TestDiagnoseHealthyFile`, `TestDiagnoseStructurallyInvalid`, `TestDiagnoseFileCorruption`, `TestDiagnoseCompletesUnderOneSecond`
- `internal/command/show_diagnose.go` - `runShowDiagnose`, `runShowExport`, `parseShowDiagnoseArgs`, `parseShowExportArgs`, shared `parseShowPathArg`
- `internal/command/show_diagnose_test.go` - `TestShowExportMatchesCanonicalEncode`, `TestShowDiagnoseHealthyExitZero`, `TestShowDiagnoseCorruptExitOne`, `TestShowExportTooNewReadOnly`

## Decisions Made
- **Diagnose opens a second connection via `LoadForRead` for the structural check** rather than calling `decodeAndValidate` on the same `db` handle the integrity_check query used. This keeps the two checks independent (a file-level query issue never prevents the structural check from running, and vice versa) and reuses `LoadForRead`'s existing D-10 "newer-than-supported tolerated" behavior verbatim instead of duplicating that branch logic inside `diagnose.go`.
- **`DiagnosticReport.StructuralError` is a plain `string`**, resolving the plan frontmatter's `error`/`string` ambiguity in favor of `string` -- keeps `show diagnose`'s JSON output a deterministic plain document (`encoding/json` cannot marshal a bare `error` interface usefully) with an `omitempty` tag so a healthy report has no `structural_error` key at all.
- **`show diagnose`/`show export` share one `GOLC_SHOW_DIAGNOSE_USAGE` code** for their identical `--show <path>`-only argument shape, mirroring `deployment.go`'s existing precedent (`GOLC_DEPLOYMENT_USAGE` shared across `deployment create`/`deployment activate`/`show inspect`) of one usage code per file rather than per route.
- **Corruption-injection technique for tests**: pad a `State` with ~500 pool members so the saved `.golc` spans multiple SQLite pages, then overwrite only the file's tail quarter with `0xFF` bytes -- well past page 1's header and `sqlite_master` schema. This lets `openStore`'s PRAGMA setup and `application_id` door check still succeed (so `Diagnose` doesn't hard-fail at the open step) while `PRAGMA integrity_check` still walks into the corrupted region and reports real findings (confirmed via a temporary debug run before finalizing the test: actual output included lines like `Tree 4 page 4 cell 0: invalid page number 4294967295` and multiple `Page N: never used` entries -- genuine corruption detection, not a synthetic pass).

## Deviations from Plan

None - plan executed exactly as written. The `<action>` blocks' explicit code-example shape (RESEARCH.md's Diagnose example) was followed with one refinement already covered above (using `LoadForRead` via a second connection rather than inlining `decodeAndValidate` on the integrity-check connection) -- this is a design detail within the task's own discretion, not a deviation from a locked requirement.

## Issues Encountered
None. The corruption-injection technique (tail-quarter garbage overwrite) worked on the first implementation attempt and was verified empirically (not just trusted) via a temporary debug log before being finalized and removed.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- SHOW-06 is fully delivered: `show diagnose` and `show export` give operators an on-demand, read-only troubleshooting/interchange surface, independent of the other Phase 5 plans' recovery (05-02), migration/backup (05-03) work.
- `internal/show.Diagnose` and the `show diagnose`/`show export` routes are stable additions any later plan can build on (e.g., a future `show import` command reusing `show export`'s exact output shape as its input format, noted as an unconfirmed follow-on in 05-CONTEXT.md's Specific Ideas -- not built here, out of this plan's scope).
- No blockers for 05-05 or the phase-level wave merge.

---
*Phase: 05-durable-shows-and-recovery*
*Completed: 2026-07-23*
