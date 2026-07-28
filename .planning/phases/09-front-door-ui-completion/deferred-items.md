# Deferred Items — Phase 9

Out-of-scope discoveries logged during plan execution (per gsd-executor's scope
boundary rule): pre-existing, unrelated to the current task's changes, not
auto-fixed.

## 09-01 (Fixture Library workspace)

Discovered while running `go test ./...` after Task 3. None of the failures
below touch `internal/fixture`, `internal/wails`, `internal/command/artnet.go`,
or any other file this plan modified — confirmed pre-existing in the worktree
before this plan's changes.

- **`internal/command` toolchain-bootstrap tests** (`TestBuildRouteCompilesTheProductionRepository`,
  `TestBuildablePackagesExcludesMagefiles`, `TestScopeCrossPlatformCI`,
  `TestScopeGreenSubprocess`, `TestScopeOfflineAcceptance`): all fail with
  `GOLC_TEST_TOOLCHAIN_MISSING` / "pinned golc-project binary not built" —
  this fresh worktree has never run `mage Bootstrap`, so `.tools/toolchains/`
  and `.tools/installs/` don't exist yet. Environment-setup gap, not a code
  regression.
- **`internal/trace/catalog` `TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline`**:
  fails with `GOLC_MIGRATE_DRIFT: .planning/linear-map.json does not match
  the canonical schema-2 migration output`. Unrelated domain (Linear
  traceability catalog); this plan never touches `.planning/linear-map.json`.
