---
phase: 01-offline-foundation-and-delivery-traceability
verified: 2026-07-21T09:31:06Z
status: gaps_found
score: 4/6 must-haves verified
behavior_unverified: 0
overrides_applied: 0
gaps:
  - truth: "A contributor can preview and run an idempotent reconciliation that creates or updates the intended Linear project, milestones, issues, and sub-issues without duplicating retried work (LINR-03; roadmap SC5, first clause)."
    status: failed
    reason: >
      The production "linear apply" handler (runLinearApply) calls the low-level
      apply.Apply(client, plan, mappings) entry point directly. It never calls
      apply.RunApply, apply.LoadJournal, or apply.ValidatePlanFreshness -- the
      exact functions that implement staleness rejection and journal-based
      resume. Those functions exist, are fully implemented, and are covered by
      TestScopeLinearApplyResume in internal/trace/apply/apply_test.go, but
      no production code path invokes them. Separately, the one real
      apply.RemoteClient implementation this repository ships
      (processLinearClient.ReadByMarker in internal/command/linear.go) is a
      permanent stub that always returns found=false, so
      applyUnlinkedOperation's "discover a prior successful create and adopt
      it instead of duplicating" safety path can never fire against the real
      transport either. Net effect: a contributor who re-runs
      `golc.ps1 linear apply plan-a.json --plan-id <id>` a second time
      (after a transient failure, a CI retry, or simply forgetting a plan
      file is single-use) instead of first re-running `linear preview` will
      have every not-yet-linked operation in that plan re-attempted as a
      create, producing a duplicate remote Linear object for every entity
      the first run already created successfully. No test in the repository
      exercises "apply an already-applied plan file a second time without an
      intervening preview" -- tests/acceptance/linear-transport.ps1's own
      inline comment explicitly documents that its hierarchy scenario avoids
      this exact case by re-previewing rather than re-applying. This is
      CR-01 in 01-REVIEW.md (Critical, still open -- confirmed unresolved by
      re-reading internal/command/linear.go:1385-1436 and
      internal/command/linear.go:856-858 at verification time; the review
      commit 77c0dcf is HEAD with no follow-up fix commit).
    artifacts:
      - path: internal/command/linear.go
        issue: "runLinearApply (line ~1424) calls apply.Apply(...) instead of apply.RunApply(...); processLinearClient.ReadByMarker (line 856) is a permanent `return apply.RemoteState{}, false, nil` stub."
      - path: internal/trace/apply/engine.go
        issue: "RunApply (the documented full exact-plan apply orchestration with ValidatePlanFreshness + ResumePrefix) exists and is tested, but has zero production callers."
    missing:
      - "Wire runLinearApply to call apply.RunApply (with a captured pre-apply snapshot, a loaded .journal.json via apply.LoadJournal, and os.LookupEnv) instead of the bare apply.Apply, so staleness rejection and resume-without-replay actually protect the production apply path."
      - "Implement a real processLinearClient.ReadByMarker (requires adding a description-search/list operation to tools/linear-sync/src/protocol.ts's Operation contract first) so a prior interrupted create is discovered and adopted instead of duplicated, or explicitly document in the CLI's own usage/help text that a plan file is strictly single-use and must never be re-applied without a fresh `linear preview` -- today nothing warns about this at the Go layer."
  - truth: "Linear synchronization reports ambiguity, partial GraphQL errors, pagination, and rate limiting without blocking local planning, builds, tests, or application runtime (LINR-04; roadmap SC5, second clause)."
    status: failed
    reason: >
      The ambiguity/partial-error/pagination/rate-limit scenarios the
      requirement text enumerates are genuinely implemented and tested
      (tools/linear-sync/src/errors.ts + errors.test.ts/rate-limit.test.ts,
      src/pagination.ts + pagination.test.ts). Offline build/test/generate/
      check are also unaffected in all cases, since the Node adapter is only
      ever launched by an explicit "linear" remote command. However, one
      adjacent read-failure path this workspace's own adapter.ts exposes is
      not converted to a safe reported outcome the way every sibling SDK
      call site is: readOperation (adapter.ts:198-204), the handler for
      every plain "read" action, has no try/catch, unlike createOperation,
      updateOperation, and confirmReadback (which calls the identical
      underlying readByEntity helper) -- all three of which catch and
      convert. An SDK read failure (a documented behavior for a missing,
      archived, or deleted remote object) propagates uncaught out of
      LinearSdkAdapter.execute into cli.ts's handleLine, which is also never
      wrapped in try/catch, permanently stopping the NDJSON `for await`
      reader loop for the remainder of that Node process's lifetime. Because
      internal/trace/transport/process.go's ProcessClient is one long-lived
      process reused across every Call() in a single CLI invocation (whole
      CaptureSnapshot loop, whole Apply run), every subsequent remote
      operation in that same run then hangs until the per-call deadline
      (GOLC_LINEAR_SYNC_TIMEOUT_MS, default 30s) fires -- turning one
      archived/deleted Linear object into repeated 30-second timeouts (or an
      immediate GOLC_TRANSPORT_PROCESS_EXITED) instead of the graceful
      {found:false} report ReadResult exists to express. This is CR-02 in
      01-REVIEW.md (Critical, still open -- confirmed unresolved by
      re-reading tools/linear-sync/src/adapter.ts:198-204 at verification
      time). tools/linear-sync/test/operations.test.ts's FakeLinearClient
      never throws on a miss, so this exception path has zero test coverage
      anywhere in the suite.
    artifacts:
      - path: tools/linear-sync/src/adapter.ts
        issue: "readOperation (lines 198-204) has no try/catch around readByEntity, unlike createOperation/updateOperation/confirmReadback."
    missing:
      - "Wrap readOperation's readByEntity call in try/catch and return {found:false} on any exception, matching confirmReadback's existing treatment (a missing record and an SDK exception on a missing record are already treated identically elsewhere in this file)."
      - "Add a fixture/test mirroring mutation.test.ts's HostileLinearClient but for a plain read, and a Go-side acceptance scenario in tests/acceptance/linear-transport.ps1 that deletes/archives an already-linked local ID's remote object out from under CaptureSnapshot and asserts the run still completes."
  - truth: "The requirement-to-phase mapping in REQUIREMENTS.md is current for every requirement this phase claims (Definition of Done clause)."
    status: partial
    reason: >
      LINR-01 and LINR-02 have never been checked off in REQUIREMENTS.md
      across the entire git history of this phase (confirmed via
      `git log --all -p -- .planning/REQUIREMENTS.md`), even though Plan
      01-22's own SUMMARY.md declares `requirements-completed: [LINR-01,
      LINR-02]` with substantive integration verification evidence, and the
      underlying implementation (internal/trace/catalog) is real, tested,
      and wired. Conversely, LINR-03 and LINR-04 were marked "Complete" in
      REQUIREMENTS.md by commit 6e8c88c (Plan 01-13, wave 12 of 18) -- well
      before the apply-path and read-path defects above were introduced by
      later plans (01-24, 01-15) and before 01-REVIEW.md's code review ran
      (the review commit, 77c0dcf, is the current HEAD). REQUIREMENTS.md's
      own Definition of Done states a requirement is complete only when
      "no unresolved release-blocking finding contradicts the requirement"
      -- CR-01 and CR-02 are exactly such unresolved findings against
      LINR-03/LINR-04, so the "Complete" marks are premature bookkeeping,
      not a code defect in themselves.
    artifacts:
      - path: .planning/REQUIREMENTS.md
        issue: "LINR-01/LINR-02 checkboxes and Traceability rows never updated from 'Pending' despite delivered, tested, verified implementation; LINR-03/LINR-04 marked 'Complete' despite two currently open Critical review findings that directly contradict their guarantees."
    missing:
      - "Check off LINR-01 and LINR-02 in REQUIREMENTS.md (both the checkbox list and the Traceability table) once CR-01/CR-02 below are also resolved, so the phase-closing REQUIREMENTS.md state matches the actual verified implementation."
      - "Either revert LINR-03/LINR-04 to 'Pending' until CR-01/CR-02 are fixed, or leave them 'Complete' only with an explicit linked exception noting the two open Critical findings -- silent inconsistency between the review and the requirements tracker should not ship as-is."
human_verification: []
---

# Phase 1: Offline Foundation and Delivery Traceability Verification Report

**Phase Goal:** Contributors can build and govern the project from centralized configuration and durable local identities, with Linear reconciliation that never blocks offline work. (ROADMAP.md wording: "Contributors can configure, validate, build, and trace GOLC from durable repository-owned sources without requiring Linear or secrets to be available.")
**Verified:** 2026-07-21T09:31:06Z
**Status:** gaps_found
**Re-verification:** No — initial verification

**Note on ROADMAP `Mode: mvp`:** ROADMAP.md tags this phase `Mode: mvp`, but its `Goal:` text is not in the `As a ..., I want to ..., so that ....` User Story format the MVP-mode narrowing methodology requires (`user-story.validate` returns `valid: false` against it). Given the project-wide description already calls all ten phases "MVP slices" in a looser sense, and 29 executed plans with substantial, security-relevant Linear-sync machinery are on the line, this report proceeds with full standard goal-backward verification against the roadmap Success Criteria rather than refusing outright. Flagged here as an info-level process note, not a gap.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A contributor can start at one documented root configuration and discover pinned toolchains plus setup/generation/validation/build/test/packaging/application-default/runtime-configuration entrypoints (SC1). | ✓ VERIFIED | `golc.project.toml` indexes all 6 concerns (toolchain, commands, generation, application_defaults, runtime, linear); `docs/development.md` documents the sequence; `internal/command/{build,check,config,generate,package,test,tools}.go` all self-register routes via `MustDeclareRoute`; `go build ./...` succeeds cleanly. |
| 2 | Contributors and CI run the same commands, validate each concern independently, and identify one authoritative value whenever settings are shared (SC2). | ✓ VERIFIED | `.github/workflows/check.yml` invokes `golc.ps1` (same entrypoint contributors use), `permissions: contents: read` (least-privilege); `internal/projectconfig/registry.go`/`resolve.go` implement one-owner canonical key graph with 5-layer precedence + provenance; `TestConcernPathsCannotEscapeRepository`/`TestLoadRootIndexRejectsDuplicateConcernIDs` etc. pass. |
| 3 | A clean checkout contains no secrets or machine-local values; safe examples document external names needed for optional integrations (SC3). | ✓ VERIFIED | `.env` is git-ignored and not tracked (`git ls-files` confirms); `.env.example` (via `git show HEAD:.env.example`) contains only `LINEAR_API_KEY=`/`LINEAR_TEAM_ID=` empty placeholders and an explanatory comment, no real values; `internal/security/redact.go` + `tools/linear-sync/src/redact.ts` implement a mirrored canary/pattern scan (`CanaryToken`, `forbiddenPatterns` incl. `LINEAR_API_KEY=`, `Bearer `, `sk-`, `lin_api_`); `.gitignore` also excludes `golc.local.toml` and `.tools/`. |
| 4 | Every milestone, phase, requirement, plan, and task can retain a durable local identity and complete planning context while Linear is unavailable (SC4; LINR-01/LINR-02). | ✓ VERIFIED (code) | `internal/trace/catalog/{model,id,parse,validate}.go` implement a 6-kind (`project/milestone/phase/req/plan/task`) offline ID graph built entirely from committed repository artifacts; `internal/trace/catalog/migrate.go` performs a lossless, atomic schema-1→2 migration to a credential-free `.planning/linear-map.json`; `go test ./internal/trace/catalog/... ./internal/strictjson/...` passes (`TestScopeLinearCatalog`, `TestScopeLinearMap`). See Gap 3 below: REQUIREMENTS.md's own tracking of this truth is stale, which is a documentation gap, not a code gap. |
| 5a | A contributor can preview and rerun an idempotent Linear reconciliation without duplicating retried work (LINR-03; SC5 first clause). | ✗ FAILED | See Gaps — CR-01: production `runLinearApply` calls `apply.Apply` (no staleness/resume protection) instead of `apply.RunApply`; `processLinearClient.ReadByMarker` is a permanent stub. Confirmed unresolved by direct re-read of `internal/command/linear.go` at verification time. |
| 5b | Ambiguity, partial GraphQL errors, pagination, and rate limiting are reported without blocking local planning, builds, tests, or runtime operation (LINR-04; SC5 second clause). | ✗ FAILED | Pagination/rate-limit/partial-error handling for the requirement's own enumerated scenarios is real and tested (see Artifacts table), but a directly adjacent read-exception path (`adapter.ts`'s `readOperation`) is uncaught and stalls the shared Node process for every subsequent remote operation in that run — CR-02, confirmed unresolved. Offline build/test/generate/check themselves are unaffected (separate process, never invoked from those routes). |

**Score:** 4/6 truths verified (0 present-but-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `golc.project.toml` | Complete root concern index | ✓ VERIFIED | Lists all 6 concerns with repository-relative paths; schema_version=1. |
| `config/{toolchain,commands,generation,application-defaults,runtime}.toml`, `config/integrations/linear.toml` | Independently validatable concern files | ✓ VERIFIED | All present; `internal/projectconfig/decode.go` strict-decodes each. |
| `internal/projectconfig/{registry,resolve,path,local}.go` | One-owner key graph, 5-layer resolution, containment, local writes | ✓ VERIFIED | Present, wired via `internal/command/config.go`; `go test ./internal/projectconfig/...` passes (11 tests). |
| `internal/contracts/{model,generate,normalize,linear,linear_plan}.go` | Deterministic strict JSON Schema generation/drift-check registry | ✓ VERIFIED | `RegisterSchema`/`RegisteredSchemas`/`GenerateAll`/`CheckDrift` present; `schemas/*.schema.json` (7+ files incl. `linear-map`, `linear-plan`, `linear-report`) committed. |
| `internal/bootstrap/{downloader,archive,bootstrap,cache}.go` | Checksum-verified, allowlisted, staged, atomic toolchain install | ✓ VERIFIED | Present; `go test ./internal/bootstrap/...` passes. |
| `internal/delivery/{graph,foundation}.go` | Offline-enforced command graph + deterministic foundation package | ✓ VERIFIED | `NetworkDenied`/`DenyTransport`/`GOPROXY=off` present in `graph.go`; `go test ./internal/delivery/...` passes. |
| `internal/security/redact.go`, `tools/linear-sync/src/redact.ts` | Allowlisted diagnostics + canary scan (mirrored Go/TS) | ✓ VERIFIED | Both present with matching `forbiddenPatterns`/canary token concept. |
| `internal/trace/catalog/*.go` | Offline identity/authority graph | ✓ VERIFIED | Present, tested (`catalog_test.go`, `migrate_test.go`). |
| `internal/trace/reconcile/*.go` | Canonical hash-bound preview, 3-way diff, visible ID marker | ✓ VERIFIED | Present, tested (`reconcile_test.go`). |
| `internal/trace/apply/{model,engine,guard,journal}.go` | Stale-safe resumable apply state machine | ⚠️ ORPHANED (functionally) | Fully implemented and unit-tested (`TestScopeLinearApplyResume`) but **not called by the production route** — see Gap 1 (CR-01). Exists, substantive, tested in isolation, but not wired end-to-end. |
| `internal/trace/transport/{contract,fake,process}.go` | Transport-neutral snapshot/mutation interface + real process client | ✓ VERIFIED (Go side) | Present, tested; `process.go`'s Go-side deadline/timeout handling is sound — the defect is on the Node side (see Gap 2). |
| `tools/linear-sync/src/{protocol,adapter,pagination,errors,redact,cli}.ts` | Discriminated operation contract, SDK mapping, pagination, error/rate normalization, redaction, NDJSON entrypoint | ⚠️ HOLLOW (one path) | All present and wired for `create`/`update`/`confirmReadback`; `readOperation` specifically lacks the same error handling — see Gap 2 (CR-02). |
| `.github/workflows/{check,linear-sync}.yml` | Least-privilege Windows PR CI + protected manual remote workflow | ✓ VERIFIED | `permissions: contents: read` in both; `linear-sync.yml` is `workflow_dispatch`-gated. |
| `tests/acceptance/*.ps1` | walking-skeleton, bootstrap(-node), offline, command-parity, linear-transport | ✓ VERIFIED | All 6 scripts present; `linear-transport.ps1`'s own inline comments explicitly document (and route around, not close) the CR-01 replay gap. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `internal/command/config.go` | `internal/projectconfig/local.go` | `WriteLocal`/`Explain` handlers | ✓ WIRED | Confirmed present. |
| `internal/command/generate.go` | `internal/contracts/generate.go` | `GenerateAll`/`CheckDrift` | ✓ WIRED | Confirmed present. |
| `internal/contracts/linear.go` | `internal/contracts/generate.go` | `RegisterSchema("linear-map", ...)` | ✓ WIRED | Confirmed exactly-once registration. |
| `internal/command/linear.go` | `internal/trace/catalog/parse.go` | `linear catalog` offline handler | ✓ WIRED | Confirmed. |
| `internal/command/linear.go` | `internal/trace/reconcile/diff.go` | `linear preview` handler | ✓ WIRED | Confirmed. |
| `internal/command/linear.go` | **`internal/trace/apply/engine.go` (`RunApply`, journal, freshness guard)** | "injected RemoteClientFactory" (Plan 01-24 must-have) | ✗ **NOT WIRED AS DESIGNED** | `MustDeclareRoute`/`RemoteClientFactory` symbols are present (a literal pattern grep would pass), but `runLinearApply` calls the **lower-level `apply.Apply`**, not `apply.RunApply` — the actual safety orchestration (`ValidatePlanFreshness`, `ResumePrefix`, journal) is never reached from production. This is the concrete manifestation of Gap 1 / CR-01 — a case where the documented link's *symbols* exist but the *semantic* wiring is wrong, which a pattern-presence check alone would miss. |
| `tools/linear-sync/src/adapter.ts` | `tools/linear-sync/src/redact.ts` | safe error/canary emission | ⚠️ PARTIAL | Wired for `createOperation`/`updateOperation`/`confirmReadback`; **not reached from `readOperation`**, which has no catch block to route a thrown error into a safe outcome at all (Gap 2 / CR-02). |
| `.github/workflows/check.yml` | `golc.ps1` | bootstrap/generate/check/build/test/package | ✓ WIRED | Confirmed. |
| `.github/workflows/linear-sync.yml` | `golc.ps1` | protected preview/apply + `finally` `.env` cleanup | ✓ WIRED | Confirmed present (not deeply re-audited beyond structure — CI credential materialization was in scope of the code review, not re-derived here). |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full Go module builds cleanly | `go build ./...` | exit 0, no output | ✓ PASS |
| `go vet` reports no issues | `go vet ./...` | exit 0, no output | ✓ PASS |
| Full Go test suite passes (run once, per verification constraints) | `GOPROXY=off GOFLAGS=-mod=readonly go test -count=1 ./...` | all 11 packages `ok` | ✓ PASS |
| Apply-resume/staleness test exists (enumeration only) | `go test ./internal/trace/apply/... -list '.*'` | `TestScopeLinearApplyCore`, `TestScopeLinearApplyResume` | ✓ PASS (exists) — but see Gap 1: this test exercises `RunApply` directly, not through the production `runLinearApply` handler, so its passing does not prove the production path is safe. |
| `runLinearApply` calls `apply.Apply` not `apply.RunApply` (source re-read) | `grep -n "apply\.Apply\|apply\.RunApply" internal/command/linear.go` | `results := apply.Apply(client, plan, migrated.RemoteMappings)` only | ✓ CONFIRMS GAP 1 |
| `processLinearClient.ReadByMarker` is a permanent stub (source re-read) | Read `internal/command/linear.go:856-858` | `return apply.RemoteState{}, false, nil` unconditionally | ✓ CONFIRMS GAP 1 |
| `adapter.ts`'s `readOperation` has no try/catch (source re-read) | Read `tools/linear-sync/src/adapter.ts:198-204` | no `try`/`catch` present, unlike `confirmReadback` 15 lines below it | ✓ CONFIRMS GAP 2 |
| TypeScript test suite (`tools/linear-sync`) | N/A | `node_modules` not present in this worktree (no bootstrap run here) | ? SKIP — cannot run npm test without bootstrapping; consistent with every prior plan SUMMARY's documented "no bootstrapped toolchain in this worktree" condition. TS-side coverage evidence instead comes from reading `test/*.test.ts` files directly (present: `errors`, `mutation`, `operations`, `pagination`, `rate-limit`, `redact`). |

### Probe Execution

No `scripts/*/tests/probe-*.sh` convention is used by this project; `tests/acceptance/*.ps1` fills that role but requires a bootstrapped `.tools/` toolchain this worktree does not have (consistent with every plan SUMMARY's documented substitution of direct `go build`/`go test` against the host toolchain). Skipped: no PowerShell acceptance run was executed as part of this verification; Go-level build/vet/test and direct source re-reads were used instead as the fastest reliable equivalent evidence for the two disputed code paths.

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|---|---|---|---|---|
| CONF-01 | 01-01, 01-02, 01-03, 01-04, 01-05, 01-13, 01-16, 01-17, 01-19, 01-20, 01-25, 01-28, 01-29 | Discover all entrypoints from one root config | ✓ SATISFIED | `golc.project.toml` + self-registered routes; REQUIREMENTS.md marked Complete, consistent with code. |
| CONF-02 | 01-02, 01-03, 01-18, 01-19 | Independently validatable concerns, no duplicated authoritative values | ✓ SATISFIED | `internal/projectconfig/registry.go` one-owner model; REQUIREMENTS.md marked Complete, consistent with code. |
| CONF-03 | 01-01, 01-04, 01-06, 01-07, 01-12, 01-13, 01-14, 01-15, 01-16, 01-17, 01-19, 01-20, 01-25, 01-28, 01-29 | Same commands for contributor + CI | ✓ SATISFIED | `.github/workflows/check.yml` invokes `golc.ps1`; REQUIREMENTS.md marked Complete, consistent with code. |
| CONF-04 | 01-02, 01-03, 01-07, 01-14, 01-15, 01-18, 01-26, 01-27 | Secrets/machine-local values outside committed config | ✓ SATISFIED | `.env` ignored, `.env.example` clean, redaction contract present; REQUIREMENTS.md marked Complete, consistent with code. |
| LINR-01 | 01-08, 01-09, 01-21, 01-22 | Durable local identifier for every milestone/phase/requirement/plan/task | ✓ SATISFIED (code) / ✗ NOT REFLECTED in REQUIREMENTS.md | `internal/trace/catalog` implemented and tested; REQUIREMENTS.md still shows `[ ]`/"Pending" — see Gap 3. |
| LINR-02 | 01-08, 01-09, 01-21, 01-22 | Credential-free local-ID→Linear-UUID mapping | ✓ SATISFIED (code) / ✗ NOT REFLECTED in REQUIREMENTS.md | `.planning/linear-map.json` schema-2 + `internal/trace/catalog/migrate.go`; REQUIREMENTS.md still shows `[ ]`/"Pending" — see Gap 3. |
| LINR-03 | 01-10, 01-11, 01-12, 01-13, 01-15, 01-23, 01-24, 01-25 | Idempotent preview/apply reconciliation without duplicating retried work | ✗ **BLOCKED** | REQUIREMENTS.md marks "Complete", but CR-01 (open) directly contradicts the "without duplicating retried work" guarantee in the production apply path — see Gap 1. |
| LINR-04 | 01-07, 01-10, 01-14, 01-15, 01-23, 01-24, 01-26, 01-27 | Ambiguity/partial-error/pagination/rate-limit reporting without blocking | ✗ **BLOCKED** | REQUIREMENTS.md marks "Complete"; the requirement's own named scenarios are handled and tested, but CR-02 (open) breaks the adjacent read-exception path with the same "report without blocking" intent — see Gap 2. |

No orphaned requirements: every ID REQUIREMENTS.md maps to Phase 1 (CONF-01..04, LINR-01..04) appears in at least one plan's `requirements:` frontmatter field, and no plan claims a requirement ID outside this set.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/command/linear.go` | 856-858 | `ReadByMarker` permanent `false` stub, documented in its own comment as a known limitation | 🛑 Blocker (see Gap 1) | Disables prior-success discovery for every real apply. |
| `internal/command/linear.go` | 1424 | `runLinearApply` calls `apply.Apply` instead of the documented `apply.RunApply` orchestration | 🛑 Blocker (see Gap 1) | Staleness rejection and journal resume are dead code from production's perspective. |
| `tools/linear-sync/src/adapter.ts` | 198-204 | `readOperation` missing `try`/`catch` present on every sibling SDK call site in the same file | 🛑 Blocker (see Gap 2) | Uncaught exception stalls the shared NDJSON reader process for the remainder of a run. |
| `.planning/REQUIREMENTS.md` | 133-137, 280-283 | LINR-01/LINR-02 never checked off despite delivered+tested implementation; LINR-03/LINR-04 checked off before the code review that found CR-01/CR-02 | ⚠️ Warning (see Gap 3) | Requirement tracker does not reflect actual code/verification state either direction. |

No `TODO`/`FIXME`/`XXX`/`TBD` markers found in any Go, TypeScript, TOML, or workflow file touched by this phase (`grep -rn "TODO\|FIXME\|XXX\|TBD"` across `internal/`, `tools/linear-sync/`, `.github/`, `config/`, excluding `_test.go`, returned zero matches).

### Human Verification Required

None. All findings in this report are demonstrated directly from source (build/test/vet output, direct code reads, and git history of the requirements tracker) rather than requiring visual, UX, or live-service judgment.

### Gaps Summary

Phase 1 delivers a large, well-tested offline-first foundation: centralized configuration discovery (CONF-01..04) is solid, wired, and matches REQUIREMENTS.md's own tracking. The Linear traceability catalog (LINR-01/LINR-02) is also solid and tested in code, though REQUIREMENTS.md was never updated to reflect it (Gap 3, a small documentation fix).

The phase goal's Linear-reconciliation half is where real work remains. The roadmap's Success Criterion 5 — "rerun it without duplicates" and "reported without blocking" — is **not actually true of the shipped production code path**, even though the underlying machinery that would make it true (`apply.RunApply`, journal resume, staleness rejection, per-SDK-call error containment) is implemented and unit-tested in isolation. This is precisely the "implemented and tested but not wired into the production code path" pattern the task brief warned against: `01-REVIEW.md`'s two Critical findings (CR-01, CR-02) are both still open as of this verification (re-confirmed by direct source reads, not by trusting the review or any SUMMARY.md), and REQUIREMENTS.md's "Complete" marks for LINR-03/LINR-04 predate that review by roughly a full day of subsequent plan execution.

Recommended next step: a closure plan (or amendment to a near-term plan) that (1) rewires `runLinearApply` to call `apply.RunApply` with a captured snapshot and loaded journal, (2) adds error handling to `adapter.ts`'s `readOperation` matching its sibling call sites, and (3) reconciles REQUIREMENTS.md's checkbox/table state with the actual, now-corrected implementation. None of these require new design work — the review's own **Fix** sections for CR-01 and CR-02 are concrete and ready to apply.

---

_Verified: 2026-07-21T09:31:06Z_
_Verifier: Claude (gsd-verifier)_
