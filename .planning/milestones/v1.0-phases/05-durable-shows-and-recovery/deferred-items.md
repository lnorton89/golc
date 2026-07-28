# Deferred Items — Phase 5

Out-of-scope discoveries surfaced during plan execution, logged rather than fixed (execution scope boundary).

## 05-01

- **`internal/trace/catalog` — `TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline` fails with `GOLC_MIGRATE_DRIFT: .planning/linear-map.json does not match the canonical schema-2 migration output`.**
  Confirmed unrelated to this plan: `internal/trace/catalog` has zero dependency on `internal/show`, `.planning/linear-map.json` has no uncommitted diff in this worktree, and prior commits (`5d76a5f`, `4285645`, `9d44376`) show this exact drift has recurred and been fixed after several earlier phases — a known, pre-existing, recurring catalog-sync issue independent of the Phase 5 SQLite storage work. Fixed by regenerating `.planning/linear-map.json` (`linear map migrate --write`) during this phase's Wave 1 gate, commit `c70523b`.

## Post-execution (during code-review/fix pass)

- **`internal/trace/catalog` — `TestScopeLinearCatalog`/`TestScopeLinearMap` fail with `GOLC_CATALOG_ID_INVALID: requirement key "TBD" does not match the KEY-NN grammar`.**
  Introduced by a concurrent, unrelated session that added `### Phase 11: Telemetry, Usage Statistics, and Auto Crash Submission Pipeline` to `.planning/ROADMAP.md` as a draft skeleton (commit `7c22168`) with a placeholder `**Requirements:** TBD` line — Phase 11 has not been broken down into real requirement IDs yet (`Plans: 0 plans`, `- [ ] TBD (run /gsd-plan-phase 11 to break down)`). A malformed sub-issue in the same line (`**Requirements**: TBD`, colon outside the bold markers, breaking the catalog parser's line-match entirely) was fixed here since it was a pure formatting typo. The underlying "TBD" placeholder is a legitimate content gap in Phase 11's still-in-progress planning, not something to fabricate requirement IDs for from outside that context — left for whoever runs `/gsd-plan-phase 11` (or a roadmap requirements pass) to resolve. Confirmed zero relationship to Phase 5 or `internal/show`.
