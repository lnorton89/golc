# Deferred Items — Phase 5

Out-of-scope discoveries surfaced during plan execution, logged rather than fixed (execution scope boundary).

## 05-01

- **`internal/trace/catalog` — `TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline` fails with `GOLC_MIGRATE_DRIFT: .planning/linear-map.json does not match the canonical schema-2 migration output`.**
  Confirmed unrelated to this plan: `internal/trace/catalog` has zero dependency on `internal/show`, `.planning/linear-map.json` has no uncommitted diff in this worktree, and prior commits (`5d76a5f`, `4285645`, `9d44376`) show this exact drift has recurred and been fixed after several earlier phases — a known, pre-existing, recurring catalog-sync issue independent of the Phase 5 SQLite storage work. Not fixed here; left for a dedicated catalog-regeneration fix outside this plan's scope.
