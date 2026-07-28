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

## 03-03

- Same `TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline`
  drift reproduced again on `go test ./...` after 03-03's changes
  (`internal/programming/chase.go`, `internal/programming/motion.go`,
  `internal/show/state.go`, `internal/command/programming.go`). Every
  package this plan touches passes independently
  (`go test ./internal/programming/... ./internal/command/... ./internal/show/...`
  green); the drift is isolated to `internal/trace/catalog` /
  `.planning/linear-map.json`, outside this plan's `files_modified` list.
  No new fix attempted — same pre-existing condition already logged under
  03-01.

## 03-04

- Same `TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline`
  drift reproduced again on `go test ./...` after 03-04's changes
  (`internal/scene/scene.go`, `internal/scene/layer.go`,
  `internal/scene/blend.go`, `internal/show/state.go`,
  `internal/command/scene.go`). Every package this plan touches passes
  independently (`go test ./internal/scene/... ./internal/command/...
  ./internal/show/...` green, plus a full `go build ./...` and
  `go vet ./...`); the drift is isolated to `internal/trace/catalog` /
  `.planning/linear-map.json`, outside this plan's `files_modified` list.
  No new fix attempted — same pre-existing condition already logged under
  03-01/03-03.

## 03-05

- Same `TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline`
  drift reproduced again on `go test ./...` after 03-05's changes
  (`internal/programming/history.go`, `internal/programming/history_test.go`,
  `internal/command/programming.go`, `internal/command/history_test.go`).
  Every package this plan touches passes independently
  (`go test ./internal/programming/... -run TestHistory`,
  `go test ./internal/command/... -run TestHistory`, and
  `go test ./internal/show/...` all green, plus a full `go build ./...`
  and `go vet ./...` clean); the drift is isolated to
  `internal/trace/catalog` / `.planning/linear-map.json`, outside this
  plan's `files_modified` list. No new fix attempted — same pre-existing
  condition already logged under 03-01/03-03/03-04.

## 03-06

- Same `TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline`
  drift reproduced again on `go test ./...` after 03-06's changes
  (`internal/playback/clock.go`, `internal/playback/clock_test.go`,
  `internal/command/playback.go`, `internal/command/playback_bpm_test.go`).
  `go test ./internal/playback/... ./internal/command/...
  ./internal/show/...` all green, plus a full `go build ./...` and
  `go vet ./...` clean; `git status --short .planning/linear-map.json`
  shows zero uncommitted changes from this plan's work. Drift remains
  isolated to `internal/trace/catalog` / `.planning/linear-map.json`,
  outside this plan's `files_modified` list — likely reflects concurrent
  worktree-agent activity elsewhere in this wave. No new fix attempted;
  same pre-existing condition already logged under 03-01/03-03/03-04.

## 03-07

- Same `TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline`
  drift reproduced again on `go test ./...` after 03-07's changes
  (`internal/playback/frame.go`, `internal/playback/compile.go`,
  `internal/playback/evaluate.go`, `internal/playback/engine.go`, plus
  their `_test.go` files, and `internal/command/playback.go`/
  `internal/command/playback_engine_test.go`). `go test
  ./internal/playback/... -race`, `go test ./internal/command/... -run
  TestPlayback`, and `go test ./internal/show/...` are all green, plus a
  full `go build ./...` and `go vet ./...` clean; `git status --short
  .planning/linear-map.json` shows zero uncommitted changes from this
  plan's work. Drift remains isolated to `internal/trace/catalog` /
  `.planning/linear-map.json`, outside this plan's `files_modified` list.
  No new fix attempted; same pre-existing condition already logged under
  03-01/03-03/03-04/03-05/03-06.
