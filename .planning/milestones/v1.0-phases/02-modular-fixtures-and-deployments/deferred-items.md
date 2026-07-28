# Phase 02 Deferred Items

Out-of-scope discoveries logged during plan execution per the executor's
scope-boundary rule (only auto-fix issues directly caused by the current
task's changes; pre-existing failures in unrelated files are logged here,
not fixed).

## 02-01

- **`go test ./...` full-suite failure unrelated to this plan's files:**
  `internal/trace/catalog` `TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline`
  fails with `GOLC_MIGRATE_DRIFT: .planning/linear-map.json does not match
  the canonical schema-2 migration output`. This is pre-existing: `git
  status --short .planning/` shows no changes from this plan, and the
  failure is in `internal/trace/catalog` (Phase 1 Linear sync), not
  `internal/fixture`, `internal/command/fixture.go`, or
  `internal/contracts/fixture.go` (this plan's `files_modified`). Likely
  cause: `.planning/linear-map.json` was committed before later Phase 2
  planning artifacts (docs, plans) were added, so the migration it encodes
  has drifted from the current `.planning/` tree. Needs a `linear map
  migrate --write` regeneration in a dedicated fix, out of this plan's
  scope.
