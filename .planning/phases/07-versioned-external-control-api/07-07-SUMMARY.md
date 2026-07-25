---
phase: 07-versioned-external-control-api
plan: 07
subsystem: api
tags: [api, audit, sqlite, redaction, security, single-writer]

# Dependency graph
requires:
  - phase: 07-versioned-external-control-api
    provides: "07-05: internal/api/observer.go's MutationEvent + RegisterMutationObserver/fireMutationObservers seam; 07-06: atomic /v1/batch fires one MutationEvent per committed sub-request after the single aggregated Save"
provides:
  - "internal/show: audit_log append-only table (createTablesSQL) + AuditRecord/WriteAuditRecord/QueryAuditLog, reusing openStore's single-writer machinery and stageRecoveryPoint's transaction discipline"
  - "internal/api/audit.go: RegisterAuditObserver(root, showPath) -- a redacting post-mutation observer that writes exactly one audit_log row per attempted mutation (success/failure/rejected/dry_run/idempotent_replay/batch sub-event), with credential-shaped fields stripped before serialization"
affects: [07-09-versioned-external-control-api]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Audit writes reuse internal/show's existing openStore/stageRecoveryPoint transaction discipline (db.Begin/tx.Exec/tx.Commit) -- never a second, uncoordinated sql.Open against the .golc file; enforced by a source-grep test (TestAuditWriterNeverOpensASecondConnection)."
    - "Strip-before-write redaction (A5): internal/api/audit.go builds redacted_details from a credential-shaped-flag-name check (--token/--secret/--key/--password) PLUS internal/security.Redact's centrally-owned forbidden-token pattern scan (Bearer headers, api-key prefixes) on every arg value, before the row is ever serialized -- never stored raw and redacted at read time."
    - "RegisterAuditObserver(root, showPath) is an exported wiring entrypoint, not a package-init self-registration: root/showPath are per-*Server values only known at daemon-construction time, so a future startup-wiring plan (07-09) calls it once per *Server, mirroring 07-08's own SSE observer wiring. No edit to mutate.go/batch.go/server.go was needed or made."

key-files:
  created:
    - internal/show/audit.go
    - internal/show/audit_test.go
    - internal/api/audit.go
    - internal/api/audit_test.go
  modified:
    - internal/show/schema.go

key-decisions:
  - "The audit observer's outcome column is a direct pass-through of MutationEvent.Outcome (no relabeling): 07-05/07-06 already fire \"success\"/\"failure\"/\"dry_run\"/\"idempotent_replay\", and the pipeline's own scope/revision rejections (403/412) both report outcome \"failure\" today, not a distinct \"rejected\" value. audit_log.outcome carries no CHECK constraint (07-RESEARCH.md Pattern 5's DDL draft), so this plan does not invent a \"rejected\" mapping that the observer seam does not actually produce -- the behavior tests instead prove the pass-through is correct for whatever outcome string a caller supplies (including a literal \"rejected\", tested directly), rather than asserting a specific mapping mutate.go/batch.go were never asked to implement."
  - "Redaction operates on MutationEvent.Args (the flat CLI-style argv slice the observer seam actually carries), not a raw HTTP JSON body -- MutationEvent has no body field. A \"--<flag>\" arg whose name contains key/token/secret/password redacts its following value; internal/security.Redact (already the repo's single centralized redaction authority, redact.go) additionally scans every arg value for Bearer-header/api-key-prefix patterns regardless of position, covering the Authorization-header-value case even though auth.go never actually puts a raw Authorization value into a MutationEvent (Actor is always the key id, never the raw token)."
  - "Audit-write failures are logged (GOLC_API_AUDIT_WRITE_FAILED to stderr) and swallowed, never propagated: the write happens strictly after mutate.go's own pipeline has already committed its outcome, so a failing audit write must not fail or reverse the mutation it is recording, and must not crash the daemon process that also owns deterministic playback/Art-Net output -- mirrors server.go's own isolated-background-goroutine doctrine for a post-bind Serve failure."

patterns-established:
  - "Source-grep regression test (TestAuditWriterNeverOpensASecondConnection) as a lightweight enforcement mechanism for an architectural invariant (single-writer discipline) that a normal behavioral test cannot directly observe."

requirements-completed: [API-06]

coverage:
  - id: D1
    description: "audit_log table + WriteAuditRecord/QueryAuditLog round-trip all fields including null revisions; append-only with strictly increasing ids"
    requirement: "API-06"
    verification:
      - kind: unit
        ref: "internal/show/audit_test.go#TestWriteAuditRecordRoundTrip"
        status: pass
      - kind: unit
        ref: "internal/show/audit_test.go#TestWriteAuditRecordSequentialIDsIncrease"
        status: pass
      - kind: unit
        ref: "internal/show/audit_test.go#TestWriteAuditRecordNullResultingRevision"
        status: pass
      - kind: unit
        ref: "internal/show/audit_test.go#TestAuditWriterNeverOpensASecondConnection"
        status: pass
    human_judgment: false
  - id: D2
    description: "A successful mutation writes exactly one audit_log row with actor=key id, source=http, correlation id, outcome=success, and the resulting revision"
    requirement: "API-06"
    verification:
      - kind: unit
        ref: "internal/api/audit_test.go#TestAuditObserverRecordsSuccessOutcome"
        status: pass
    human_judgment: false
  - id: D3
    description: "A failed mutation and a rejected mutation each write exactly one audit_log row with a null resulting_revision"
    requirement: "API-06"
    verification:
      - kind: unit
        ref: "internal/api/audit_test.go#TestAuditObserverRecordsFailureAndRejectedOutcomes"
        status: pass
    human_judgment: false
  - id: D4
    description: "A dry-run writes exactly one audit_log row with outcome dry_run and a null resulting_revision"
    requirement: "API-06"
    verification:
      - kind: unit
        ref: "internal/api/audit_test.go#TestAuditObserverRecordsDryRunOutcome"
        status: pass
    human_judgment: false
  - id: D5
    description: "redacted_details never contains a raw credential-named-flag value or a raw Authorization/bearer-shaped value, while preserving non-sensitive detail"
    requirement: "API-06"
    verification:
      - kind: unit
        ref: "internal/api/audit_test.go#TestAuditObserverRedactsSensitiveFields"
        status: pass
    human_judgment: false
  - id: D6
    description: "Repeated per-sub-request mutation events (the atomic-batch shape) each write their own audit row, not one row for the whole batch"
    requirement: "API-06"
    verification:
      - kind: unit
        ref: "internal/api/audit_test.go#TestAuditObserverBatchSubEventsEachWriteOneRow"
        status: pass
    human_judgment: false

# Metrics
duration: 35min
completed: 2026-07-25
status: complete
---

# Phase 7 Plan 7: Redacted Audit Trail for API Mutations Summary

**Append-only `audit_log` SQLite table plus a strip-before-write redacting observer that writes exactly one accountable, secret-free row per attempted API mutation (success, failure, rejected, dry-run, and each atomic-batch sub-event).**

## Performance

- **Duration:** ~35 min
- **Completed:** 2026-07-25
- **Tasks:** 2/2
- **Files modified:** 5 (4 created, 1 modified)

## Accomplishments
- Added the `audit_log` table to `internal/show/schema.go`'s `createTablesSQL`, following `recovery_points`' append-only/`AUTOINCREMENT`/`IF NOT EXISTS` convention exactly (id, occurred_at, actor, source, correlation_id, route, expected_revision, resulting_revision, outcome, status_code, redacted_details).
- Built `internal/show/audit.go`: `AuditRecord` (nullable revisions via `sql.NullInt64`), `WriteAuditRecord` (insert, reusing `openStore` + `stageRecoveryPoint`'s exact `db.Begin`/`tx.Exec`/`tx.Commit` transaction discipline -- never a second `sql.Open`), and `QueryAuditLog` (occurred_at/id-ordered reader).
- Built `internal/api/audit.go`: `RegisterAuditObserver(root, showPath)` wires a redacting observer onto `07-05`'s `RegisterMutationObserver` seam. For every `MutationEvent`, it builds one `show.AuditRecord` (actor/source/correlation/route/revisions/outcome/status) and a `redacted_details` JSON built by `redactArgs`, which strips a credential-named flag's following value (`--token`, `--secret`, `--key`, `--password` -- case-insensitive substring match) and additionally runs every arg value through `internal/security.Redact` (this repo's already-centralized forbidden-token scanner: Bearer headers, `sk-`/`lin_api_` prefixes) before the row is ever serialized.
- Proved the single-writer invariant with a source-grep regression test (`TestAuditWriterNeverOpensASecondConnection`) rather than relying only on code review to keep `audit.go` from later regressing into a direct `sql.Open`.
- Verified one audit row per attempted mutation across every outcome the seam produces (`success`, `failure`, `rejected`, `dry_run`) plus the atomic-batch shape (N `fireMutationObservers` calls -> N rows, never one row per batch).

## Task Commits

1. **Task 1: audit_log table + internal/show audit writer/query (single-writer discipline)** - RED `f11fa94` (test), GREEN `ba5675f` (feat)
2. **Task 2: Redacting post-mutation audit observer (every mutation, incl. failures, dry-runs, batches)** - RED `82aaa70` (test), GREEN `b4c714e` (feat)

**Plan metadata:** SUMMARY.md commit pending (this file); STATE.md/ROADMAP.md/REQUIREMENTS.md intentionally left untouched -- this plan ran as a parallel worktree agent, and the orchestrator owns those writes centrally after the wave completes.

## Files Created/Modified
- `internal/show/schema.go` - `audit_log` table added to `createTablesSQL`; package doc comment updated from "four small tables" to "five"
- `internal/show/audit.go` - `AuditRecord`, `WriteAuditRecord`, `QueryAuditLog`
- `internal/show/audit_test.go` - `TestWriteAuditRecordRoundTrip`, `TestWriteAuditRecordSequentialIDsIncrease`, `TestWriteAuditRecordNullResultingRevision`, `TestAuditWriterNeverOpensASecondConnection`
- `internal/api/audit.go` - `redactArgs`/`isSensitiveFlagName`/`buildRedactedDetails`, `auditObserver`, `RegisterAuditObserver`
- `internal/api/audit_test.go` - `TestAuditObserverRecordsSuccessOutcome`, `TestAuditObserverRecordsFailureAndRejectedOutcomes`, `TestAuditObserverRecordsDryRunOutcome`, `TestAuditObserverRedactsSensitiveFields`, `TestAuditObserverBatchSubEventsEachWriteOneRow`

## Decisions Made
- Outcome pass-through (no relabeling to a "rejected" value the seam does not itself produce) -- see `key-decisions` in frontmatter.
- Redaction targets `MutationEvent.Args` (the actual seam shape) rather than a hypothetical raw JSON body, reusing `internal/security.Redact` as the value-pattern layer instead of hand-rolling a second Bearer-detection implementation.
- Audit-write failures are logged and swallowed, never propagated to the mutation pipeline -- see `key-decisions` in frontmatter.

## Deviations from Plan

None - plan executed exactly as written. `RegisterAuditObserver` is an additive exported entrypoint (not listed by name in the plan's action text, which only names "a post-mutation observer... registered via 07-05's RegisterMutationObserver"), required for the observer to actually be attachable with a concrete `root`/`showPath` and for tests to exercise it deterministically -- this is the natural, minimal shape of "registers via the seam," not a scope expansion.

## Issues Encountered
- `mage testquick` fails in this worktree with `GOLC_TEST_TOOLCHAIN_MISSING` (pinned Go toolchain binary not bootstrapped) -- the same pre-existing environment limitation `07-04-SUMMARY.md`/`07-05-SUMMARY.md` already documented, unrelated to this plan's changes. This plan's own stated verification commands (`go test ./internal/show/... -run ...`, `go test ./internal/api/... -run ...`) ran directly and are fully green, including the broader `go test ./internal/api/... ./internal/show/...` and `go test -race ./internal/api/... ./internal/show/...` sweeps and `go build ./internal/...`.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `internal/show.WriteAuditRecord`/`QueryAuditLog` and `internal/api.RegisterAuditObserver` are stable, general-purpose seams: any future daemon-startup wiring plan can call `api.RegisterAuditObserver(server.root, server.showPath)` once per `*Server` with zero further plumbing (mirrors `07-05-SUMMARY.md`'s own "07-08 attach to with zero further plumbing" precedent).
- **Not yet wired into production startup:** this plan's `files_modified` scope deliberately excluded `server.go`/`router.go` (avoiding a merge collision with the parallel 07-08 SSE-observer plan sharing the same wave) -- no daemon process currently calls `RegisterAuditObserver`. A future plan (likely 07-09, which `07-05-SUMMARY.md` already flags as the "close capability coverage" plan) needs to add that one call at `*Server` construction time before every real mutation actually leaves an on-disk audit trail in production.
- The redaction field list stays narrow (credential-shaped names only, `[ASSUMED]` per 07-RESEARCH.md Open Question 3) -- available for discuss-phase/UAT to widen later if a real sensitive field surfaces beyond key/token/secret/password.

---
*Phase: 07-versioned-external-control-api*
*Completed: 2026-07-25*
