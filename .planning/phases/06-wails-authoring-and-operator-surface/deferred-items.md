# Deferred Items

Pre-existing, out-of-scope issues discovered during phase execution but not fixed (per executor SCOPE BOUNDARY rule: only auto-fix issues directly caused by the current task's changes).

## From Plan 06-02 execution

Discovered while running `go test -race ./...` as a broader sanity check after Task 3 (06-02 touches only `internal/artnet/*` and `internal/command/artnet*.go` — confirmed via `git diff --stat <base>..HEAD -- internal/trace` returning empty, i.e. zero overlap with these failures).

- **`internal/trace/catalog` — `TestScopeLinearCatalog` / `TestScopeLinearMap` fail**: `BuildCatalog: GOLC_CATALOG_ID_INVALID: requirement key "TBD" does not match the KEY-NN grammar`. Three subtests of `TestScopeLinearCatalog` and one subtest of `TestScopeLinearMap` fail against the real repository catalog. Unrelated to Art-Net safety overrides; last touched in Phase 1 commits, not this phase.
- **`internal/trace/transport` — `TestScopeTraceTransportProcess` flaky under `-race`**: a genuine data race between `ProcessClient.terminate`'s `exec.Cmd.Wait()` and `ProcessClient.safeFailureSummary()`'s concurrent read of the same field (`internal/trace/transport/process.go` lines ~217/252). Reproducible via `go test -race ./internal/trace/transport/...`. Unrelated to Art-Net safety overrides; last touched in Phase 1 commits, not this phase.

Both should be triaged and fixed in a dedicated `internal/trace` maintenance plan, not folded into Phase 6 UI/operator-surface work.
