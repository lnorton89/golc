# Deferred Items — Phase 08 Isolated TypeScript Automation

## Pre-existing, out-of-scope toolchain bootstrap failures (found during 08-03)

While running `go test ./internal/command/... ./internal/scriptsdk/...` after
wiring `internal/scriptsdk` into `generate`/`check` (08-03-PLAN.md Task 3),
five pre-existing tests failed because this worktree has never run
`mage Bootstrap` (`.tools/toolchains` and `.tools/installs` do not exist):

- `TestBuildRouteCompilesTheProductionRepository`
- `TestBuildablePackagesExcludesMagefiles`
- `TestScopeCrossPlatformCI/Mage_tests_cross-compile_for_every_configured_contributor_platform`
- `TestScopeGreenSubprocess`
- `TestScopeOfflineAcceptance`

All five require the pinned Go toolchain and/or a pre-built
`golc-project.exe` under `.tools/`, neither of which this fresh worktree
checkout has provisioned. This is an environment-setup precondition for the
whole `internal/command` test suite, unrelated to any change 08-03 made
(confirmed: `.tools/toolchains` and `.tools/installs` are absent from disk
entirely, not stale/mismatched). Out of scope for this plan's task-level
auto-fix rules — resolving it means running `mage Bootstrap`, a
repository-wide provisioning step, not a fix to 08-03's own files.

`go test ./internal/scriptsdk/...` (the package this plan owns) is fully
green. `go run ./cmd/golc-project generate` and `go run ./cmd/golc-project
check --concern project` (which do not require the pinned toolchain, only
the local `go` already on PATH) both exit 0 with zero drift.
