# Phase 2: Modular Fixtures and Deployments - Context

**Gathered:** 2026-07-21
**Status:** Ready for planning

<domain>
## Phase Boundary

Show authors can build a trustworthy semantic fixture catalog (load, create, edit, validate, import from OFL, and share versioned YAML fixture definitions) and adapt logical fixture pools to concrete deployments through explicit, atomic, reviewable impact changes — including semantic fixture substitution. This phase is headless: domain model, validation engine, and CLI/API surface only. No Wails UI work happens in Phase 2; the fixture editor and pool-management UI arrive in Phase 6.

Requirements: FIXT-01 through FIXT-06, POOL-01 through POOL-08.

</domain>

<decisions>
## Implementation Decisions

### Phase 2 Interface Boundary
- **D-01:** Phase 2 is headless — domain model, validation engine, and CLI/API surface only. No fixture-editor or pool-management UI in this phase; that's Phase 6 (Wails).
- **D-02:** Custom fixture authoring is validate-only: the author hand-writes YAML (guided by schema/docs) and runs a `golc fixture validate <file>`-style CLI command. No scaffold/generator command in Phase 2.
- **D-03:** "Share" (FIXT-04) means file-level sharing — a validated custom fixture is a portable YAML file plus its computed identity/hash. No registry, upload, or discovery mechanism in Phase 2.
- **D-04:** Fixture/pool/deployment operations route through the same shared typed command model (`internal/command`) that Phase 1 established, so Phase 6 (UI) and Phase 7 (external API) can expose these operations later without rework. This matches PROJECT.md's constraint that UI, scripts, API, and LLM must converge on shared domain commands.

### Fixture Catalog Scope for v1
- **D-05:** Representative first-user fixture set: simple/color-changing PARs and wash fixtures, plus moving-head spots/washes — intensity, color, position, beam/zoom, gobo capabilities. No pixel/matrix fixture support required for v1.
- **D-06:** OFL fixtures outside the v1 target set (pixel/matrix, exotic multi-mode) still import through the same normalization pipeline; unsupported/lossy capabilities are surfaced as explicit warnings per FIXT-06 rather than rejected outright.
- **D-07:** OFL data reaches GOLC via live fetch (from OFL online or a local mirror the user points at) plus a local cache; once a fixture is imported and pinned (FIXT-05), the show is fully usable offline. Only fetching *new* fixtures needs connectivity — consistent with the project's offline-first constraint.
- **D-08:** The canonical fixture model should be designed to be GDTF-friendly/extensible (capability-based, not hard-wired to OFL's shape) so GDTF import could be added later without a schema rewrite. No GDTF parser or import path is built in Phase 2.

### Pool & Deployment Mental Model
- **D-09:** A "deployment" is a saved, named mapping of logical pools to concrete fixture instances/addresses. A show can hold multiple named deployments (e.g., per venue), with one marked active at a time.
- **D-10:** A "group" is an independent, cross-pool named selection concept (e.g., "all wash fixtures" spanning multiple pools) — orthogonal to pools, which exist purely to abstract fixture count/identity. Matches PROG-01's "select by pool, group, deployment instance, or direct selection" phrasing.
- **D-11:** When fixtures are added to a pool, GOLC auto-assigns proposed universe/address for the new deployment instances as part of the impact plan (e.g., next free slot); the author sees and can adjust these before accepting. Not fully manual.
- **D-12:** Scale assumption for design/performance: small rig, ~10–50 fixtures across 3–8 pools per typical first-user show (club/church/school scale). Impact review and pool operations can be synchronous/simple — no need to design for large-venue scale in Phase 2.

### Impact Review UX for Changes
- **D-13:** Impact plans (pool changes, fixture substitution) are reviewed and accepted/cancelled as a single atomic unit — no per-item partial accept within one plan. "Revise" means changing the underlying pool/substitution request and re-running the review, not picking-and-choosing within a computed plan. This matches POOL-05's atomic-apply guarantee.
- **D-14:** Capability gaps in fixture substitution (POOL-07) use a severity taxonomy: missing/incompatible/unsupported capabilities are surfaced as warnings the author can knowingly accept past (never silently approximated, but not automatically blocking). True structural errors can still hard-block separately from warnings.
- **D-15:** The impact review is presented as a CLI dry-run (human-readable output, with a JSON/machine-readable option) followed by a separate apply/confirm step — mirroring familiar infra-as-code plan/apply UX. This sets up cleanly for Phase 6's UI to call the same underlying commands later.
- **D-16:** Impact plans reuse Phase 1's staleness-detection pattern (D-18/D-21: plan carries an expected show revision; apply fails safely with a clear message if the revision moved since the plan was computed) rather than skipping revision checking.

### Claude's Discretion
None — every gray area discussed converged on the recommended option; no "you decide" selections were made in this session.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project-level requirements and roadmap
- `.planning/PROJECT.md` — Core value, requirements, constraints (esp. pool propagation, semantic substitution, offline-first, shared command model), and Key Decisions table.
- `.planning/REQUIREMENTS.md` §Fixture Definitions, §Pools and Deployments — FIXT-01–06 and POOL-01–08 requirement text and Traceability mapping to Phase 2.
- `.planning/ROADMAP.md` §Phase 2: Modular Fixtures and Deployments — Goal, success criteria, and the research note flagging canonical fixture semantics, pool propagation rules, representative first-user fixtures, OFL snapshot/licensing, GDTF preservation, hazardous attributes, and physical validation corpus as open research areas for this phase.

### Prior-phase precedent this phase should follow
- `.planning/STATE.md` §Accumulated Context → Decisions — Phase 1 decisions this phase must stay consistent with, notably: "UI, persistence, scripts, API, LLM, and Linear never own or block deterministic playback or Art-Net timing" and the D-18 staleness / D-21 journal-resume pattern referenced in D-16 above.

No user-referenced ADRs/specs beyond the project's own planning docs came up during discussion — no additional canonical docs to add.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/command` (Phase 1): shared typed command-registration model with `MustDeclareScope` — Phase 2 fixture/pool/deployment operations should register here (D-04) rather than as plain Go functions, so Phase 6/7 can expose them without rework.
- `go.yaml.in/yaml/v4` (already an indirect dependency via `invopop/jsonschema`): can be used directly for YAML fixture parsing, including duplicate-key rejection consistent with FIXT-02.
- `schemas/*.schema.json` + `internal/projectconfig` "Strict concern decoding" pattern (Spec-driven: DefaultSpec registry, typed cross-references via `ref:<canonical.key>`): precedent for how GOLC currently does schema-validated, strictly-decoded structured config — a plausible model for fixture-schema validation, though fixture definitions are YAML-authored rather than TOML.
- `internal/trace/apply` D-18 staleness check + D-21 journal-resume (Phase 1 Linear apply): the concrete pattern D-16 says to reuse for impact-plan revision checking.

### Established Patterns
- Strict decoding with actionable diagnostics is a repo-wide convention already (`golc.local.toml is re-validated strictly on every read... same stable diagnostics as rejected writes"` — STATE.md). FIXT-02's "actionable diagnostics" requirement should follow this existing convention rather than introducing a new error-reporting style.
- Deprecation/validation outcome codes follow a `{DOMAIN}_{CONDITION}` naming convention (e.g., `CFG_DEPRECATED_KEY`, `GOLC_ROUTE_SCOPE_UNDECLARED`) — fixture/pool validation diagnostics should likely follow the same code-naming convention for consistency.

### Integration Points
- No fixture, pool, or deployment code exists yet — `internal/` currently only has `bootstrap`, `command`, `contracts`, `delivery`, `projectconfig`, `security`, `strictjson`, `trace/*`. Phase 2 is greenfield for this domain and should establish its own package(s) (e.g., `internal/fixture`, `internal/pool`/`internal/deployment`) following the existing package-per-concern layout.

</code_context>

<specifics>
## Specific Ideas

- Impact review UX should feel like infra-as-code plan/apply (D-15) — dry-run first, explicit apply/confirm second.
- Severity taxonomy for capability gaps: missing / incompatible / unsupported, all warn-not-block by default (D-14).
- Small-rig scale target (~10–50 fixtures, 3–8 pools) should inform any performance/complexity tradeoffs in pool/impact-review design (D-12) — don't over-engineer for large venues.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. No scope-creep items came up; all four discussed areas were clarifications of how to implement what's already in FIXT-01–06 and POOL-01–08.

</deferred>

---

*Phase: 2-Modular Fixtures and Deployments*
*Context gathered: 2026-07-21*
