# Phase 1: Offline Foundation and Delivery Traceability - Context

**Gathered:** 2026-07-17
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase establishes GOLC's contributor-facing foundation: one discoverable project command, centralized but logically separated configuration, reproducible offline-capable development after bootstrap, durable repository-owned planning identities, and safe reconciliation with Linear. It does not implement lighting-domain behavior, application UI, playback, Art-Net, scripting, or AI capabilities.

</domain>

<decisions>
## Implementation Decisions

### Contributor Setup Flow
- **D-01:** A clean Windows checkout is prepared through one bootstrap command that installs or downloads pinned project-local tools where practical and then verifies the complete environment.
- **D-02:** After one successful bootstrap, core generation, validation, build, and test operations must work offline from pinned local caches. Network-only operations such as dependency refresh and Linear sync fail clearly without breaking offline work.
- **D-03:** Contributors use one repository command with clear subcommands for at least bootstrap, check, generate, build, test, package, and Linear operations rather than discovering ecosystem-specific commands.
- **D-04:** Tool and dependency updates are explicit. An update command produces reviewable manifest and lockfile changes; bootstrap never silently upgrades pinned versions.

### Configuration Hierarchy
- **D-05:** A small machine-readable manifest at the repository root is the central configuration index. It points to logically separated configuration files organized by concern rather than becoming a monolithic settings file.
- **D-06:** Override precedence is committed defaults, then user-level configuration, untracked project-local configuration, environment variables, and finally command-line flags.
- **D-07:** The effective configuration is inspectable, including the source layer that supplied each resolved value.
- **D-08:** Stable generated schemas, public contracts, and generated types needed for review or downstream consumers are committed. Caches, temporary generation output, and machine-specific artifacts are ignored.
- **D-09:** Configuration validation is strict: unknown keys, duplicate authority, invalid values, and unresolved references fail immediately. Deprecated keys warn and provide migration guidance.
- **D-10:** Local development and CI invoke the same repository commands and validate each concern independently while retaining one authoritative value for shared settings.

### Linear Authority and Conflicts
- **D-11:** Repository artifacts own scope, durable local IDs, requirement text, and roadmap phase structure. Linear owns operational execution fields: status, assignee, priority, estimate, and completion timestamps.
- **D-12:** Linear comments and discussion remain in Linear and are not mirrored into repository planning artifacts.
- **D-13:** If both sides changed the same mapped field since the last synchronization, that item is blocked. The tool shows a field-by-field conflict preview and requires explicit resolution; neither side wins automatically.
- **D-14:** Stable local and Linear UUID identities never change during renames. Renames update display text only.
- **D-15:** Removal is never mirrored as an automatic deletion. It requires an explicit reviewed archive or unlink action.

### Linear Synchronization Lifecycle
- **D-16:** Mutating synchronization runs only through explicit repository commands at planning or execution milestones. Pull-request CI performs read-only drift checks and must not mutate Linear.
- **D-17:** Mutation is a two-step operation: preview creates a deterministic reconciliation plan, and a separate apply command executes that exact plan.
- **D-18:** Apply rejects a preview if relevant repository or Linear state changed after the preview was produced.
- **D-19:** The repository commits `.env.example` with all supported variables and safe placeholders. The real local `.env` remains untracked.
- **D-20:** CI creates an ephemeral `.env` from its protected secret store and removes it after the job. Secret values must never appear in previews, logs, errors, mapping files, or committed artifacts.
- **D-21:** Synchronization failures, ambiguity, pagination, partial GraphQL errors, and rate limits are reported without blocking local planning, builds, tests, or application runtime.

### the agent's Discretion
- Select the implementation technology and internal name for the single repository command, provided its user-facing subcommands and behavior match the locked decisions.
- Choose exact root-manifest and concern-file names, schemas, and directory layout while preserving one machine-readable index and independent validation.
- Choose cache locations, download verification mechanics, and retry/backoff details for transient network operations.
- Choose the serialization format and hashing strategy for deterministic Linear preview plans.
- Choose the CI provider configuration and how its protected secret is materialized as an ephemeral `.env`.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Scope and Requirements
- `.planning/PROJECT.md` — Project core value, centralized-configuration constraint, Linear-from-inception decision, offline requirement, and secret-handling boundary.
- `.planning/ROADMAP.md` § Phase 1 — Phase goal, eight mapped requirements, success criteria, and phase boundary.
- `.planning/REQUIREMENTS.md` § Centralized Configuration and § Linear Traceability — Atomic `CONF-01` through `CONF-04` and `LINR-01` through `LINR-04` requirements.
- `.planning/STATE.md` — Current phase, pending Linear remote mappings, and project-wide blockers/constraints.

### Research and Technical Direction
- `.planning/research/SUMMARY.md` § Linear Delivery Traceability and § Phase 0/1 roadmap implications — Reconciled repository/Linear authority model, offline invariant, and recommended delivery hierarchy.
- `.planning/research/STACK.md` § Development and Delivery Tools and § Linear from Day One — Pinned toolchain direction, official Linear SDK recommendation, and reconciled Project/Milestone/Issue mapping.
- `AGENTS.md` — Generated GOLC project constraints, current stack recommendations, Linear hierarchy, and workflow enforcement for all downstream agents.

### Existing Configuration and Identity Seed
- `.planning/config.json` — Existing GSD workflow configuration; the Phase 1 project configuration must coexist clearly without confusing it for application runtime configuration.
- `.planning/linear-map.json` — Credential-free schema-1 seed containing stable local project/milestone IDs and pending remote mappings; extend or migrate deliberately rather than inventing remote UUIDs.

No phase-specific SPEC.md or ADR exists for Phase 1. The files above and the locked decisions in this context are authoritative.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `.planning/linear-map.json`: Provides the initial credential-free local identity and pending-mapping shape that Phase 1 can validate, extend, or migrate.
- `.planning/config.json`: Demonstrates an existing machine-readable configuration concern that the root index must distinguish from future application and developer configuration.
- `.gitignore`: Already excludes `.env`; Phase 1 must add a safe committed `.env.example` without exposing the existing local file.
- `AGENTS.md`: Supplies pinned stack and workflow guidance that the project command and configuration validation should respect.

### Established Patterns
- The repository is greenfield and has no implementation source, build system, package manifests, or codebase maps yet. Phase 1 establishes the first conventions.
- Repository planning files are complete offline and use durable local IDs. Remote Linear UUIDs are optional mappings, never local identity.
- Secrets are external to Git. An existing untracked `.env` was detected but its contents were intentionally not inspected.
- Planning documents are committed atomically with GSD-managed commands and use explicit traceability rather than title-based identity.

### Integration Points
- A new root manifest becomes the index for developer tooling, application defaults, runtime configuration, schemas, generation, validation, builds, tests, packaging, and integration-specific concerns.
- The single repository command becomes the supported interface used by contributors and CI.
- Linear reconciliation tooling connects `.planning/linear-map.json`, roadmap/requirements/plan artifacts, the official Linear API/SDK, preview artifacts, and read-only CI drift checks.
- `.env.example`, `.gitignore`, protected CI secrets, and ephemeral CI `.env` creation form the credential boundary.

</code_context>

<specifics>
## Specific Ideas

- The single repository command should present cohesive subcommands such as `bootstrap`, `check`, `generate`, `build`, `test`, `package`, and `linear`, regardless of the underlying implementation technology.
- A Linear apply operation should consume the exact preview artifact the user reviewed and fail closed when that preview is stale.
- Effective configuration inspection should make precedence understandable instead of merely printing final values.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within Phase 1 scope.

</deferred>

---

*Phase: 01-offline-foundation-and-delivery-traceability*
*Context gathered: 2026-07-17*
