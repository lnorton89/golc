# Deferred Items — Phase 3

Out-of-scope discoveries logged during plan execution (not fixed, per the
executor's scope-boundary rule: only issues directly caused by the current
task's changes are auto-fixed).

## 03-01

- **`TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline`
  (`internal/trace/catalog`) fails on `go test ./...`**: `GOLC_MIGRATE_DRIFT:
  .../.planning/linear-map.json does not match the canonical schema-2
  migration output`. Pre-existing, unrelated to 03-01's changes
  (`internal/programming`, `internal/show/state.go`,
  `internal/command/programming.go`) — no file this plan touches overlaps
  `internal/trace/catalog` or `.planning/linear-map.json`, and `git status`
  shows zero uncommitted changes under `.planning/` from this plan's work.
  Confirmed via `git log --oneline -- internal/trace/catalog`: last touched
  in Phase 1 (01-08/01-09/01-21), long before this phase's work began. Not
  reproduced or investigated further; flagged here for a future phase/plan
  to triage.
