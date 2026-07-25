---
phase: quick-delete-golc-ps1
plan: 260724-x7n
subsystem: developer-tooling
tags: [powershell-removal, mage, ci, cli, cleanup]

requires:
  - phase: quick-260723-vj8
    provides: The nonblocking cross-platform Mage CI observation matrix and the six-target Mage graph
  - phase: quick-260724-w3f
    provides: Real, all-green cross-platform evidence that the Mage-driven graph works before cutting the PowerShell shim over
provides:
  - golc.ps1 (811 lines) deleted; Mage (magefiles/magefile.go) is the sole contributor entrypoint
  - commands.entrypoint removed from config, the strict schema registry, the delivery graph, the generated JSON Schema, and MCP projections
  - check.yml and linear-sync.yml rewritten to Mage/the pinned CLI binary, with a new closed-world exception in the command-parity checker for Mage's own provisioning step
  - tests/acceptance/*.ps1 (2,651 lines) individually investigated: 4 scripts deleted as redundant, 2 genuine coverage gaps ported to new Go tests, 1 script kept (architecturally requires a real Node process) with its golc.ps1 dependency fixed
  - A live Claude Code Stop hook that shelled out to golc.ps1 found and fixed before it could silently break
affects: [developer-entrypoint, ci, powershell-removal, documentation]

tech-stack:
  added: []
  patterns: [investigate-then-decide acceptance-test triage (delete/port/keep, never mechanical translation), narrow exact-string-match exceptions to otherwise-strict closed-world parsers, real-subprocess smoke tests alongside in-process unit tests]

key-files:
  created:
    - internal/command/offline_acceptance_test.go
    - internal/command/green_subprocess_test.go
    - internal/command/linear_sync_workflow_test.go
  deleted:
    - golc.ps1
    - tests/acceptance/bootstrap.ps1
    - tests/acceptance/bootstrap-node.ps1
    - tests/acceptance/offline.ps1
    - tests/acceptance/walking-skeleton.ps1
  modified:
    - .claude/settings.json
    - .github/workflows/check.yml
    - .github/workflows/linear-sync.yml
    - .gitignore
    - README.md
    - cmd/golc-project/main.go
    - config/commands.toml
    - config/toolchain.toml
    - docs/development.md
    - docs/reference/README.md
    - docs/reference/artnet-ipc.md
    - docs/reference/delivery.md
    - internal/bootstrap/cache.go
    - internal/command/build.go
    - internal/command/check.go
    - internal/command/check_test.go
    - internal/command/linear.go
    - internal/command/package.go
    - internal/command/test.go
    - internal/contracts/model.go
    - internal/delivery/delivery_test.go
    - internal/delivery/foundation.go
    - internal/delivery/graph.go
    - internal/docgen/docgen.go
    - internal/projectconfig/model.go
    - internal/security/redact_test.go
    - internal/trace/transport/process.go
    - internal/trace/transport/process_test.go
    - magefiles/magefile_test.go
    - schemas/config-commands.schema.json
    - tests/acceptance/linear-transport.ps1
    - tests/golden/foundation-manifest.json
    - tools/golc-mcp/README.md
    - tools/golc-mcp/commands.go
    - tools/golc-mcp/config.go
    - tools/golc-mcp/docs.go
    - tools/golc-mcp/mage.go
    - tools/golc-mcp/main.go
    - tools/golc-mcp/protocol_test.go
    - tools/golc-mcp/schemas.go
    - tools/golc-mcp/testscopes.go

key-decisions:
  - "internal/contracts/model.go's CommandsBlock turned out to be a second, independently-maintained schema-generation struct (distinct from internal/projectconfig/model.go's KeySpec validation registry) that also declared an entrypoint field -- discovered only because regenerating schemas/config-commands.schema.json after removing the KeySpec still showed the stale field, forcing a second removal site to be found."
  - "tests/acceptance/*.ps1 was triaged script-by-script by cross-referencing each one's exact behavioral assertions (error codes, stdout markers) against the existing Go test suite via targeted grep, not mechanically translated. This surfaced genuine gaps (offline.ps1 -Mode core, walking-skeleton.ps1 -Mode green) worth porting and confirmed the rest were truly redundant -- a mechanical port would have produced ~2,600 lines of duplicate Go test ceremony for behavior already covered."
  - "linear-transport.ps1 is kept as PowerShell rather than ported or deleted: it drives the real, compiled tools/linear-sync/dist/src/cli.js through the real Go<->Node process transport, which cannot be replicated in Go without spawning a real Node process running the real compiled adapter. Only its golc.ps1 dependency (the Invoke-Golc helper) was fixed."
  - "check.yml's strict command-parity checker (every workflow run: line must exactly match 'run: mage <Target>') would have rejected the new Mage-install run: line as an unknown executable shape. Rather than loosening the parser generally, added one narrow, exact-string-match exception (prMageInstallInvocation) that must appear at most once and strictly before any target dispatch -- preserving the closed-world guarantee for everything else."
  - ".claude/settings.json's Stop hook (a live, functional quick-test gate, not documentation) shelled out to golc.ps1 and would have silently broken the moment the file was deleted. Found via a repo-wide grep sweep before deletion, not after a failure; repointed at the pinned CLI binary and verified the replacement command actually runs."

requirements-completed: [CONF-01, CONF-02, CONF-03]

coverage:
  - id: D1
    description: "golc.ps1 is deleted and no live automation (CI, hooks) depends on it."
    requirement: CONF-01
    verification:
      - kind: unit
        ref: "go build/go vet/go test ./... with golc.ps1 actually absent"
        status: pass
      - kind: manual
        ref: "repo-wide golc.ps1 grep sweep, every remaining hit individually reviewed"
        status: pass
    human_judgment: true
  - id: D2
    description: "commands.entrypoint is removed from every layer that declared it, not just golc.ps1 itself."
    requirement: CONF-02
    verification:
      - kind: unit
        ref: "go test ./internal/projectconfig ./internal/delivery ./internal/contracts ./tools/golc-mcp -count=1"
        status: pass
    human_judgment: false
  - id: D3
    description: "check.yml and linear-sync.yml run exclusively through Mage/the pinned CLI binary, and command-parity validation accounts for Mage's own provisioning step."
    requirement: CONF-03
    verification:
      - kind: unit
        ref: "go test ./internal/command -run TestScopeCommandParity -count=1"
        status: pass
      - kind: manual
        ref: "gh workflow run linear-sync.yml --ref master (linear-drift job)"
        status: pass
    human_judgment: true

duration: single session, four sequential tasks
completed: 2026-07-24
status: complete
---

# Quick Plan 260724-x7n: Delete golc.ps1 and Retire Every Reference (PowerShell Removal Step 7) Summary

**golc.ps1 (811 lines) and 2,146 lines of redundant PowerShell acceptance tests deleted; 2 genuine Go-coverage gaps closed; 1 script kept where Go architecturally can't replace it; a live Stop-hook dependency found and fixed before it could break.**

## Performance

- **Tasks:** 4
- **Files modified:** 49 (5 deleted outright, 3 created, 41 modified)
- **Completed:** 2026-07-24

## Accomplishments

- Removed `commands.entrypoint` from every layer that declared it: `config/commands.toml`, `internal/projectconfig/model.go`'s strict KeySpec registry, `internal/delivery/graph.go`'s `CommandInventory`/foundation-ZIP packaging, `internal/contracts/model.go`'s independently-maintained JSON-Schema-generation struct (a second authority discovered mid-task), and `tools/golc-mcp`'s MCP projection — regenerated `schemas/config-commands.schema.json` and `docs/reference/*` with confirmed zero drift.
- Rewrote `check.yml` (which had never actually executed in its Mage-based form — no install step existed) to provision Mage via the same checksum-pinned script `cross-platform-mage.yml` uses, extending the strict command-parity checker with one narrow exception for the provisioning step.
- Rewrote `linear-sync.yml`'s both jobs from `golc.ps1` invocations to `mage Bootstrap` plus direct pinned-CLI-binary calls for the routes with no fixed-argument Mage target equivalent.
- Investigated all five `tests/acceptance/*.ps1` scripts (2,651 lines) individually against existing Go coverage rather than mechanically porting: deleted 4 as redundant (2,146 lines), ported 2 genuine gaps to new Go tests (`offline_acceptance_test.go`, `green_subprocess_test.go`), added a third new test (`linear_sync_workflow_test.go`) for a structural gap found along the way, and kept `linear-transport.ps1` as PowerShell (it drives the real compiled Node Linear-sync adapter, which no Go test can replace) after fixing its `golc.ps1` dependency.
- Found and fixed a live functional dependency before it could silently break: `.claude/settings.json`'s Stop hook shelled out to `golc.ps1`.
- Deleted `golc.ps1` itself and swept every remaining production doc comment, runtime error message, and `.gitignore` comment across the codebase.
- Rewrote `README.md` and `docs/development.md`'s full contributor walkthrough for the Mage-based flow.

## Task Commits

1. **Task 1-4 (single commit): delete golc.ps1 and retire every reference** - `f32fdf1`

## Files Created/Modified

See `key-files` in the frontmatter above for the complete list (3 created, 5 deleted, 41 modified).

## Decisions Made

See `key-decisions` in the frontmatter above.

## Deviations from Plan

- No upfront detailed plan existed for this work (unlike 260723-vj8's format); this PLAN.md/SUMMARY.md pair is reconstructed after the fact to preserve this repository's `.planning/quick/` traceability convention. The actual work proceeded task-by-task as described, verified locally after each, in a single combined commit.

## Issues Encountered

- `internal/contracts/model.go` turned out to be a second, independently-maintained schema-generation authority for `commands.entrypoint`, not caught until schema regeneration still showed the stale field after the first (KeySpec-registry) removal.
- The strict command-parity checker's closed-world `run:` line validation initially had no way to accept the new Mage-install provisioning step, requiring a deliberately narrow (not general) exception.

## Verification

- `go build`, `go vet`, `go test ./... -count=1` (normal and `GITHUB_EVENT_NAME=pull_request`) - PASS, with `golc.ps1` actually deleted.
- `go run ./cmd/golc-project check --command-parity` - PASS against the real, rewritten `check.yml`.
- `gh workflow run linear-sync.yml --ref master` (`linear-drift` job) - PASS, real credential-free run.
- Repo-wide `golc.ps1` grep sweep - every remaining hit individually reviewed and confirmed intentional.

## Known Stubs

None.

## User Setup Required

None.

## Next Phase Readiness

- PowerShell-removal Steps 0-8 are all complete. Step 9 (final byte-for-byte verification, including triggering `check.yml` for its first-ever real run) is the last step — see `260724-y2t`.

## Self-Check: PASSED

- All created/deleted/modified key files match the actual working tree.
- Commit `f32fdf1` exists on `master` and is pushed to `origin/master`.
- Full `go test ./...` passes with `golc.ps1` actually absent from the filesystem.

---
*Plan: 260724-x7n*
*Completed: 2026-07-24*
