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

## 09-02 (Show open/new/switch: supervised self-relaunch)

Re-confirmed while running `go test ./...` in this plan's own (separate)
worktree, after Task 3: the same five `internal/command` toolchain-bootstrap
tests fail with the identical `GOLC_TEST_TOOLCHAIN_MISSING` /
"pinned golc-project binary not built" errors — this worktree has also
never run `mage Bootstrap`. None of the five touch `internal/wails`,
`cmd/golc-desktop/main.go`, or any other file this plan modified. Not
auto-fixed (out of scope, pre-existing, environmental, matches 09-01's
identical finding above). `internal/trace/catalog`'s
`TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline` passed
in this worktree's run (no `.planning/linear-map.json` drift here), so it is
not re-logged.

## 09-07 (Custom-YAML fixture add: PickFixtureFile / PreviewFile)

Re-confirmed while running `go test ./...` in this plan's own worktree, after
Task 2: the same five `internal/command` toolchain-bootstrap tests
(`TestBuildRouteCompilesTheProductionRepository`,
`TestBuildablePackagesExcludesMagefiles`, `TestScopeCrossPlatformCI`,
`TestScopeGreenSubprocess`, `TestScopeOfflineAcceptance`) fail with
`GOLC_TEST_TOOLCHAIN_MISSING` — this worktree has never run
`mage Bootstrap` either. None touch `internal/wails` or any other file this
plan modified.

Additionally, this run surfaced `link.exe: resize output file failed: There
is not enough space on the disk` for `internal/scriptsdk`, `internal/bootstrap`,
`internal/security`, `internal/api`, `internal/strictjson`,
`internal/trace/apply`, and `internal/trace/catalog`'s own test binaries —
the host filesystem (`C:`) was at 178MB free when this plan ran `go test
./...`. A host-disk-capacity condition, not a code regression; none of the
affected packages are touched by this plan. `internal/wails` itself (this
plan's own package) built and linked its test binary successfully and passed
in full.
