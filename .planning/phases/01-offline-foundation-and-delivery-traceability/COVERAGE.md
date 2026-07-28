# Phase 01 — Linear API Capability Coverage Matrix

**Produced:** 2026-07-21 (gap-closure round, plans 01-30..01-32)
**Integration:** Linear GraphQL API via the official `@linear/sdk`, isolated in the `tools/linear-sync` Node adapter and driven from Go through `internal/trace/transport` (NDJSON process transport).
**Policy:** Full API Coverage by Default — every capability the codebase exercises is `INTEGRATE` unless an explicit one-line reason justifies `OPT-OUT`.

This matrix enumerates the Linear API capability surface actually used or exposed by this repository across all 29 executed plans plus this gap-closure round. It is not a plan of new capabilities; the gap-closure round adds no new Linear capability, it repairs the wiring/robustness of two already-integrated ones (apply-with-resume, read-failure containment).

| Capability | Disposition | Where | Reason / Notes |
|-----------|-------------|-------|----------------|
| Preview (read-only reconciliation plan) | INTEGRATE | `internal/trace/reconcile/*`, `internal/command/linear.go` (`runLinearPreview*`) | Deterministic hash-bound preview built from a captured snapshot; never mutates. |
| Snapshot capture (read-back of already-linked entities) | INTEGRATE | `processLinearClient.CaptureSnapshot`, `tools/linear-sync/src/adapter.ts` (`captureSnapshot`) | Targeted read-by-UUID of every entity with a recorded mapping; feeds preview + freshness. |
| Read by immutable UUID | INTEGRATE | `ReadByUUID` (Go), `readByEntity`/`readOperation` (TS) | Single-entity `project`/`projectMilestone`/`issue` accessor. **CR-02 (Gap 2) hardens the read-failure path of `readOperation` in plan 01-31.** |
| Apply / Create | INTEGRATE | `processLinearClient.Create`, `createOperation` (TS) | Single create mutation + mandatory readback; typed `unknown` on failure, zero auto-retry. |
| Apply / Update | INTEGRATE | `processLinearClient.Update`, `updateOperation` (TS) | Single update mutation guarded by `expectedUpdatedAt`; typed `unknown` on failure. |
| Apply orchestration with staleness rejection + journal resume | INTEGRATE | `apply.RunApply`, `ValidatePlanFreshness`, `LoadJournal`, `ResumePrefix` | Implemented + unit-tested; **CR-01 (Gap 1) wires it into the production `runLinearApply` route in plan 01-30** so D-18 staleness rejection and D-21 resume actually protect production. |
| List / paginate (Relay connection walk) | INTEGRATE | `tools/linear-sync/src/pagination.ts` (`fetchAllPages`), `adapter.ts` (`captureSnapshot`) | All connection reads route through bounded page-walk with cursor-anomaly detection. |
| Rate-limit handling | INTEGRATE | `tools/linear-sync/src/errors.ts` (`normalizeRateLimit`), `adapter.ts` (`probeRateLimit`) | Rate-limit signal blocks the whole snapshot with a reported `rate_limited` status without blocking offline work (D-21). |
| Partial GraphQL error handling (data+errors on HTTP 200) | INTEGRATE | `errors.ts` (`normalizeGraphQLResult`), `adapter.ts` (`probeGraphQLResult`) | Non-empty `errors` array blocks the snapshot with a reported `partial` status; never spoofs absence/uniqueness. |
| Secret redaction / canary scan on all diagnostics | INTEGRATE | `internal/security/redact.go`, `tools/linear-sync/src/redact.ts` (`safeError`, `scanCanary`) | Mirrored Go/TS forbidden-pattern + canary scan; no credential byte reaches any output (D-20). |
| Archive / unlink (explicit reviewed removal preview) | INTEGRATE | `writeArchivePreview`, `internal/trace/reconcile` (`ArchivePreview`) | Removal is never an automatic deletion; produces an explicit reviewable preview (D-15). |
| Read-only PR drift check (CI) | INTEGRATE | `.github/workflows/check.yml`, `linear-sync.yml` | PR CI performs read-only drift checks only; mutation is `workflow_dispatch`-gated (D-16). |
| Bounded read retry on transient server_error | OPT-OUT | `errors.ts` (`decideRetry`, tested but unwired — WR-01) | Out of scope for this gap-closure round. WR-01 is a Warning, not a release blocker; the retry policy is implemented + unit-tested but not wired. Deferred to a follow-up; no LINR-03/LINR-04 truth depends on it. |
| Search / list-by-marker (discover a prior interrupted create) | OPT-OUT | `processLinearClient.ReadByMarker` (permanent stub) | The NDJSON `Operation` contract exposes no description-search/list action. A real implementation requires a cross-language `protocol.ts` contract extension (larger scope). Compensating control shipped in plan 01-30: `ValidatePlanFreshness` (D-18) rejects a stale re-apply loudly and the CLI documents single-use plans. Full `ReadByMarker` remains a documented follow-up (WR-05). |
| Webhooks / server-sent events | OPT-OUT | — | Not part of Phase 1 offline-first traceability scope; no requirement (CONF/LINR) references push delivery. |
| OAuth / remote-access auth surface | OPT-OUT | — | Credentials are external (`.env`, ephemeral CI secret, D-19/D-20); interactive OAuth flows are out of Phase 1 scope. |

**Summary:** 12 capabilities INTEGRATE, 4 OPT-OUT (each with a one-line reason). The two gap-closure fixes (CR-01, CR-02) both repair already-`INTEGRATE` capabilities; the only capability directly touched by an `OPT-OUT` decision this round is `ReadByMarker`, whose absence is compensated by the now-wired `ValidatePlanFreshness` staleness guard.
