# Deferred Items — Phase 04 (Observable Art-Net Live Output)

Out-of-scope discoveries surfaced during plan execution that were logged
here rather than fixed, per the executor's scope-boundary rule (only
auto-fix issues directly caused by the current task's changes).

## Plan 01

- **`internal/trace/catalog` — `TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline`
  fails pre-existing, unrelated to this plan's changes.**
  `go test ./internal/trace/catalog/...` reports:
  `GOLC_MIGRATE_DRIFT: .planning/linear-map.json does not match the
  canonical schema-2 migration output`. Verified via `git stash` before any
  Plan 01 edits that this failure exists on the base commit
  (`8c1adfd`) independent of the fixture/artnet model changes made in this
  plan. Not touched — out of scope for 04-01.
