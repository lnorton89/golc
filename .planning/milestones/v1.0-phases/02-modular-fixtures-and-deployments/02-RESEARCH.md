# Phase 2: Modular Fixtures and Deployments - Research

**Researched:** 2026-07-21
**Domain:** Fixture-definition modeling/validation (YAML + OFL import), logical fixture pools, deployment mapping, and atomic impact-review workflows — headless Go domain model + CLI
**Confidence:** MEDIUM

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Phase 2 Interface Boundary**
- **D-01:** Phase 2 is headless — domain model, validation engine, and CLI/API surface only. No fixture-editor or pool-management UI in this phase; that's Phase 6 (Wails).
- **D-02:** Custom fixture authoring is validate-only: the author hand-writes YAML (guided by schema/docs) and runs a `golc fixture validate <file>`-style CLI command. No scaffold/generator command in Phase 2.
- **D-03:** "Share" (FIXT-04) means file-level sharing — a validated custom fixture is a portable YAML file plus its computed identity/hash. No registry, upload, or discovery mechanism in Phase 2.
- **D-04:** Fixture/pool/deployment operations route through the same shared typed command model (`internal/command`) that Phase 1 established, so Phase 6 (UI) and Phase 7 (external API) can expose these operations later without rework.

**Fixture Catalog Scope for v1**
- **D-05:** Representative first-user fixture set: simple/color-changing PARs and wash fixtures, plus moving-head spots/washes — intensity, color, position, beam/zoom, gobo capabilities. No pixel/matrix fixture support required for v1.
- **D-06:** OFL fixtures outside the v1 target set (pixel/matrix, exotic multi-mode) still import through the same normalization pipeline; unsupported/lossy capabilities are surfaced as explicit warnings per FIXT-06 rather than rejected outright.
- **D-07:** OFL data reaches GOLC via live fetch (from OFL online or a local mirror the user points at) plus a local cache; once a fixture is imported and pinned (FIXT-05), the show is fully usable offline. Only fetching *new* fixtures needs connectivity.
- **D-08:** The canonical fixture model should be designed to be GDTF-friendly/extensible (capability-based, not hard-wired to OFL's shape) so GDTF import could be added later without a schema rewrite. No GDTF parser or import path is built in Phase 2.

**Pool & Deployment Mental Model**
- **D-09:** A "deployment" is a saved, named mapping of logical pools to concrete fixture instances/addresses. A show can hold multiple named deployments (e.g., per venue), with one marked active at a time.
- **D-10:** A "group" is an independent, cross-pool named selection concept — orthogonal to pools, which exist purely to abstract fixture count/identity.
- **D-11:** When fixtures are added to a pool, GOLC auto-assigns proposed universe/address for the new deployment instances as part of the impact plan; the author sees and can adjust these before accepting. Not fully manual.
- **D-12:** Scale assumption for design/performance: small rig, ~10–50 fixtures across 3–8 pools per typical first-user show. Impact review and pool operations can be synchronous/simple — no need to design for large-venue scale in Phase 2.

**Impact Review UX for Changes**
- **D-13:** Impact plans (pool changes, fixture substitution) are reviewed and accepted/cancelled as a single atomic unit — no per-item partial accept within one plan. "Revise" means changing the underlying request and re-running the review.
- **D-14:** Capability gaps in fixture substitution (POOL-07) use a severity taxonomy: missing/incompatible/unsupported capabilities are surfaced as warnings the author can knowingly accept past (never silently approximated, but not automatically blocking). True structural errors can still hard-block separately from warnings.
- **D-15:** The impact review is presented as a CLI dry-run (human-readable output, with a JSON/machine-readable option) followed by a separate apply/confirm step — mirroring familiar infra-as-code plan/apply UX.
- **D-16:** Impact plans reuse Phase 1's staleness-detection pattern (plan carries an expected show revision; apply fails safely with a clear message if the revision moved since the plan was computed) rather than skipping revision checking.

### Claude's Discretion
None — every gray area discussed converged on the recommended option; no "you decide" selections were made in this session.

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope. No scope-creep items came up; all four discussed areas were clarifications of how to implement what's already in FIXT-01–06 and POOL-01–08.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| FIXT-01 | Load fixture definitions authored in a documented, versioned YAML schema | OFL's own JSON-Schema-with-semver-`version` pattern (see Standard Stack/Code Examples) is a proven precedent; reuse `invopop/jsonschema` (already a dependency) to generate the schema from one canonical Go struct, matching the existing `schemas/*.schema.json` convention. |
| FIXT-02 | Reject duplicate keys, ambiguous YAML constructs, invalid ranges, unsupported semantics with actionable diagnostics | `go.yaml.in/yaml/v4`'s `WithUniqueKeys()`/`WithKnownFields()` (both default-true under `WithV4Defaults()`) give duplicate-key and unknown-field rejection for free; range/semantics validation is GOLC-owned logic following the existing `GOLC_{DOMAIN}_{CONDITION}` diagnostic-code convention. |
| FIXT-03 | Import an OFL definition through GOLC's canonical validation/normalization pipeline | OFL fixture-format.md + capability-types.md fully documented (Architecture Patterns, Code Examples); import must land in the *same* internal typed model FIXT-01 produces, then run the *same* validation path. |
| FIXT-04 | Create, edit, validate, and share a custom YAML fixture definition | D-02/D-03 scope this to CLI validate + file-level sharing; no new research beyond FIXT-01/02 tooling. |
| FIXT-05 | Pin each fixture by stable identity, schema version, content revision, hash | Reuse the exact `internal/trace/apply` + `internal/strictjson.CanonicalEncode` pattern already proven in this repo (sha256 over canonical JSON, integrity-checked on every read) — see Don't Hand-Roll. |
| FIXT-06 | Inspect source, provenance, validation result, unsupported/lossy import details before use | Provenance record shape modeled directly on OFL's own manufacturer/fixture-key/RDM metadata plus GOLC's own warning list (see Runtime Provenance model in Architecture Patterns). |
| POOL-01 | Define a logical pool of compatible fixtures independent of count/addresses/hardware | Domain modeling only — no external precedent needed; see Architecture Patterns § Pool/Deployment model. |
| POOL-02 | Create a deployment mapping logical pools to concrete instances/modes/universes/addresses | See D-09/D-11 and Architecture Patterns; auto-assign next-free-address as part of the impact plan. |
| POOL-03 | Impact review on pool fixture add/remove covering all dependents | Reuse the `internal/trace/apply` plan/report/status shape (`StatusCompleted/Noop/Pending/Blocked`) as the structural model for a pool impact plan. |
| POOL-04 | Configurable propagation behavior, review-before-apply stays default | Config toggle in the pool-operation command, defaulting to review-required; see Architecture Patterns. |
| POOL-05 | Reviewed pool update applies atomically | Reuse `ValidatePlanIntegrity`/`ValidatePlanFreshness` two-gate pattern before any mutation; single-transaction apply. |
| POOL-06 | Replace fixture model by mapping shared semantic capabilities | OFL capability-type taxonomy (Architecture Patterns) is the concrete vocabulary two fixture definitions are diffed against. |
| POOL-07 | Replacement review identifies missing/incompatible/unsupported capabilities, never silently approximates | D-14 severity taxonomy; no external fixture-library models a hazard/severity flag (Common Pitfalls) — this is GOLC-owned design. |
| POOL-08 | Accept, revise, or cancel a pool/substitution impact plan before it changes the show | D-13/D-15/D-16; dry-run/apply CLI split modeled on Terraform's plan/apply precedent (Code Examples). |

</phase_requirements>

## Summary

Phase 2 has two structurally different halves that share one substrate: a **fixture-catalog half** (parse/validate/pin YAML fixture definitions, optionally imported from Open Fixture Library) and a **pool/deployment half** (map logical, quantity-independent fixture groups onto concrete addressed instances, and review every downstream effect of a change before applying it). Both halves are pure domain/CLI work — no persistence engine, no UI — and both must register through `internal/command` exactly like Phase 1's config/build/test commands did, so Phase 6 (Wails) and Phase 7 (API) get these operations for free later.

The fixture-catalog half has strong, directly reusable external precedent: Open Fixture Library (OFL) is a mature, MIT-licensed, semver-schema-versioned JSON format with a well-documented capability-type taxonomy (Intensity, ColorIntensity, Pan/Tilt, BeamAngle, Zoom, WheelSlot/Gobo, Prism, etc.) that maps almost directly onto D-05's v1 target set (PARs, washes, moving-head spot/wash). GDTF (DIN SPEC 15800) is the second, geometry-plus-attribute-graph standard that D-08 asks the canonical model to stay compatible with, without importing it yet. Neither format defines an explicit "hazard" or safety-severity field — POOL-07's missing/incompatible/unsupported severity taxonomy is a GOLC-original design with no schema to borrow.

The pool/deployment half has no useful *external* precedent (this is GOLC's own domain concept) but very strong *internal* precedent: Phase 1's `internal/trace/apply` package already implements the exact plan/apply shape POOL-03 through POOL-08 need — a canonically-hashed, integrity-checked, freshness-checked plan object; a per-operation result status enum (`completed`/`noop`/`pending`/`blocked`); and a hard separation between the read-only preview path and the single explicit mutation path. Phase 2 should not re-invent this pattern; it should generalize/reuse it for pool-impact and fixture-substitution plans.

**Primary recommendation:** Model one canonical, capability-based Go `FixtureDefinition` type (independent of OFL's JSON shape) that both the YAML-load path (FIXT-01/02) and the OFL-import path (FIXT-03) normalize into; generate its JSON Schema with the already-vendored `invopop/jsonschema` the same way `schemas/*.schema.json` is generated today; parse YAML with `go.yaml.in/yaml/v4`'s `WithUniqueKeys()+WithKnownFields()` Options API (promote it from an indirect to a direct dependency and bump past the currently-pinned `v4.0.0-rc.2`); and build pool/deployment impact review as a direct generalization of `internal/trace/apply`'s plan-integrity/plan-freshness/atomic-apply contract rather than a new mechanism.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| YAML fixture parsing/strict decoding (FIXT-01/02) | Domain model (Go, `internal/fixture`) | — | Pure parse/validate logic; no UI or network involved. |
| OFL fetch + cache (FIXT-03/D-07) | Domain model (Go, `internal/fixture`) | CLI/API surface | Network fetch is an explicit, user-triggered action gated behind connectivity; caching keeps the show offline-usable afterward. |
| Fixture identity/hash pinning (FIXT-05) | Domain model | Show persistence (Phase 5) | Phase 2 computes and carries the hash; Phase 5 is what ultimately persists it inside a `.golc` file. |
| Pool/deployment domain model (POOL-01/02) | Domain model | — | Logical pools and concrete deployments are pure show-state concepts with no timing or I/O dependency. |
| Impact-plan computation (POOL-03/06/07) | Domain model | — | Deterministic, synchronous computation over in-memory show state (D-12: small-rig scale). |
| Impact-plan apply (POOL-05/08) | Domain model | Command model (`internal/command`) | Apply must be atomic and revision-checked exactly like Phase 1's Linear apply; the command layer is only the routing/dispatch shell around it. |
| CLI dry-run/apply UX (D-15) | CLI/API surface (`internal/command`) | Domain model | The command layer renders/serializes the plan; the domain layer computes it — same split Phase 1 uses for `linear preview`/`linear apply`. |
| Future UI/API exposure (Phase 6/7) | Frontend Server / API tier | Domain model | Out of scope for Phase 2, but D-04 requires today's commands to be shaped so those tiers can call them unchanged later. |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `go.yaml.in/yaml/v4` | pin to latest available `v4.0.0-rcN` (currently `rc.2` indirect in go.mod; `rc.6` is available upstream — verify exact pin at execution time with `go list -m -versions go.yaml.in/yaml/v4`) | Strict YAML 1.2-subset parsing for fixture definitions (FIXT-01/02) | Official continuation of the `gopkg.in/yaml.v2/v3` lineage (now hosted at `github.com/yaml/go-yaml`), already an indirect dependency of this repo via `invopop/jsonschema`. Its v4 "Options API" (`WithUniqueKeys()`, `WithKnownFields()`) gives duplicate-key rejection and unknown-field rejection out of the box, both defaulting to `true` under `WithV4Defaults()`. [CITED: github.com/yaml/go-yaml docs/options.md] |
| `github.com/invopop/jsonschema` | v0.14.0 (already a direct dependency) | Generate `schemas/fixture.schema.json` from one canonical Go `FixtureDefinition` struct, exactly like `schemas/golc-project.schema.json` and the `config-*.schema.json` files are generated today | Reusing the existing single-authority-struct-to-schema pattern keeps fixture schema generation consistent with the rest of the repo's centralized-configuration philosophy (CONF-01/02) instead of hand-authoring a second, independently-drifting JSON Schema. [VERIFIED: go.mod] |
| `internal/command` (Phase 1, this repo) | n/a (in-repo) | Route registration for `fixture validate`, `fixture import`, `pool …`, `deployment …` CLI commands | D-04 explicitly requires this; `MustDeclareRoute`/`MustDeclareScope` is the established self-registration contract every command file in this repo already follows. [VERIFIED: internal/command/router.go] |
| `internal/strictjson` (Phase 1, this repo) | n/a (in-repo) | `CanonicalEncode` for computing the sha256 content hash behind FIXT-05's pinning, and for hashing pool/substitution impact plans (POOL-05/08) | Already proven, tested (idempotent, sorted-key, LF-terminated) canonical-JSON encoder used by `internal/trace/apply` for exactly this kind of plan-hash binding. [VERIFIED: internal/strictjson/decode.go] |
| `github.com/google/uuid` | v1.6.0 (recommended in AGENTS.md stack research; not yet in go.mod — add when this phase's plans first need durable entity IDs) | UUIDv7 identities for fixture definitions, pools, deployments, and fixture instances | Matches the project-wide ID convention already declared in AGENTS.md's stack research for "fixtures, groups, scenes, chases…". [CITED: AGENTS.md GSD:stack section] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Go standard `net/http` + `context` | Go 1.26.5 stdlib | OFL live-fetch client (D-07) | A live fetch is a small, occasional, user-triggered GET against `open-fixture-library.org` or a user-configured mirror URL; no HTTP client library is justified for this narrow, cancelable, timeout-bounded need. |
| Go standard `crypto/sha256` | Go 1.26.5 stdlib | Content-hash computation feeding FIXT-05 | Already the primitive `internal/strictjson`/`internal/trace/reconcile` build on; no new hashing library needed. |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `go.yaml.in/yaml/v4`'s Options API for duplicate-key/unknown-field rejection | Hand-rolled YAML pre-scan (regex/line-scan for duplicate keys) | Strictly worse: reinvents a security-relevant parser feature the library ships and defaults on; higher bug surface, no test coverage inherited from upstream. |
| One canonical capability-based Go fixture model | Deserializing directly into a struct shaped like OFL's JSON (channels/modes as OFL defines them) | Blocks D-08 (GDTF-friendly extensibility) and D-06 (surfacing lossy/unsupported OFL constructs as warnings rather than either silently dropping or rejecting them) — OFL's shape is channel/mode-first, not capability-first, and would need a second reshaping layer for GDTF later anyway. |
| Generalizing `internal/trace/apply`'s plan/apply contract for pool/substitution impact plans | A brand-new impact-plan mechanism specific to fixtures | Would duplicate a proven, already-tested integrity/freshness/atomic-apply pattern for no benefit; also risks the two mechanisms drifting in subtly incompatible ways (e.g. different staleness semantics), which D-16 explicitly warns against. |

**Installation:**
```bash
# go.yaml.in/yaml/v4: verify the current latest rc/stable at execution time, then:
go get go.yaml.in/yaml/v4@<verified-version>

# github.com/google/uuid: only when the first plan needs durable entity IDs
go get github.com/google/uuid@v1.6.0
```

**Version verification:** Before writing any code, re-run:
```bash
go list -m -versions go.yaml.in/yaml/v4
go list -m -versions github.com/google/uuid
```
`go.yaml.in/yaml/v4` was observed at this research date to have released through `v4.0.0-rc.6` while this repo's `go.sum` pins `v4.0.0-rc.2` (indirect). Confirm the exact rc/stable to pin against at plan-execution time — do not assume `rc.2`'s Options API surface is unchanged in later rcs without checking `docs/options.md` in the tag being pinned.

## Package Legitimacy Audit

> The `gsd-tools query package-legitimacy check` seam only supports `npm`/`pypi`/`crates` ecosystems; this phase's dependencies are Go modules, so that automated check does not apply. Verification below was performed manually against the Go module proxy and the packages' own repositories.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|--------------|---------|-------------|
| `go.yaml.in/yaml/v4` | Go module proxy (`proxy.golang.org`) | New module path (2025), but a direct continuation of `gopkg.in/yaml.v2/v3` by the same/successor maintainers, moved to the CNCF-adjacent `go.yaml.in` namespace | n/a (Go modules don't report weekly downloads) | github.com/yaml/go-yaml | OK | Approved — already an indirect dependency of this exact repo (`go list -m all` shows `v4.0.0-rc.2`); promote to direct. |
| `github.com/invopop/jsonschema` | Go module proxy | Established, already a direct dependency (v0.14.0) | n/a | github.com/invopop/jsonschema | OK | Approved — already vetted and in use by this repo's own config-schema generation. |
| `github.com/google/uuid` | Go module proxy | Long-established (Google-maintained) | n/a | github.com/google/uuid | OK | Approved — already recommended in AGENTS.md's stack research; add only when first needed. |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

*No new third-party packages beyond what AGENTS.md's stack research and this repo's existing `go.mod` already establish are required for Phase 2.*

## Architecture Patterns

### System Architecture Diagram

```
                         ┌───────────────────────────┐
                         │   golc CLI (internal/     │
                         │   command routes: fixture │
                         │   *, pool *, deployment *)│
                         └────────────┬──────────────┘
                                      │ Request{Route, Args, Root}
                                      ▼
     ┌───────────────────────────────────────────────────────────┐
     │              internal/fixture (domain model)               │
     │                                                             │
     │  YAML file ──► strict decode ──► canonical                 │
     │  (FIXT-01/02)   (go.yaml.in/yaml/v4:                        │
     │                  WithUniqueKeys+WithKnownFields)             │
     │                        │                                    │
     │  OFL JSON ─────► OFL→canonical normalizer ─────┐            │
     │  (fetch/cache,   (FIXT-03; same target type)    │            │
     │   D-07)                                          ▼            │
     │                                    FixtureDefinition (capability-  │
     │                                    based: intensity/color/position/│
     │                                    beam/gobo — GDTF-friendly, D-08)│
     │                        │                                    │
     │                        ▼                                    │
     │              Validation + Provenance record                 │
     │        (source, schema_version, content hash, revision,     │
     │         validation result, lossy/unsupported warnings)       │
     │              (FIXT-05/FIXT-06)                               │
     └───────────────────────────┬─────────────────────────────────┘
                                  │ pinned FixtureDefinition
                                  ▼
     ┌───────────────────────────────────────────────────────────┐
     │           internal/pool + internal/deployment               │
     │                                                             │
     │  Logical Pool (POOL-01) ──► add/remove fixture request      │
     │        │                            │                       │
     │        │                            ▼                       │
     │        │                 Impact-Plan Builder                │
     │        │           (walks groups, themes, palettes,         │
     │        │            scenes, chases, motion presets,         │
     │        │            controller mappings; auto-proposes      │
     │        │            next-free universe/address, D-11)       │
     │        │                            │                       │
     │        │                            ▼                       │
     │        │              ImpactPlan{expected_revision,          │
     │        │                 plan_id=sha256(canonical),          │
     │        │                 operations[], warnings[], errors[]} │
     │        │                            │                       │
     │        │              ┌─────────────┴─────────────┐         │
     │        │              ▼                            ▼         │
     │        │      dry-run render                 apply gate      │
     │        │   (human text / --json)      (ValidatePlanIntegrity, │
     │        │                                ValidatePlanFreshness,│
     │        │                                atomic single-tx      │
     │        │                                Apply; D-13/15/16)   │
     │        ▼                                                     │
     │  Deployment (POOL-02): pool → concrete fixture instance,     │
     │  mode, universe, address                                     │
     │                                                             │
     │  Fixture substitution (POOL-06/07): same Impact-Plan shape,  │
     │  diffed by shared semantic capability instead of pool size   │
     └───────────────────────────────────────────────────────────┘
```

### Recommended Project Structure
```
internal/
├── fixture/               # FIXT-01..06: canonical model, YAML strict decode,
│                          # OFL fetch/normalize, provenance, identity/hash
│   ├── model.go           # capability-based FixtureDefinition + Capability types
│   ├── decode.go          # go.yaml.in/yaml/v4 strict decode + diagnostics
│   ├── ofl/                # OFL-specific JSON shape + normalizer + fetch/cache
│   ├── provenance.go       # source/schema_version/revision/hash/validation record
│   └── identity.go         # sha256(strictjson.CanonicalEncode(...)) pinning
├── pool/                  # POOL-01,03,04,05: logical pools + impact-plan engine
│   ├── model.go            # Pool, PoolFixture
│   ├── impact.go           # dependents walk (groups/themes/.../controller maps)
│   └── plan.go             # ImpactPlan integrity/freshness/apply (mirrors
│                            # internal/trace/apply's guard.go pattern)
├── deployment/            # POOL-02: pool → concrete instance/mode/address mapping
│   └── model.go
└── substitution/          # POOL-06,07,08: capability-diff based fixture replacement
    └── plan.go             # reuses pool's ImpactPlan shape
schemas/
└── fixture.schema.json     # generated from internal/fixture.FixtureDefinition via
                            # invopop/jsonschema, same pattern as existing schemas
```

### Pattern 1: Canonical capability-based fixture model, not OFL's channel/mode shape
**What:** Define one Go type (`FixtureDefinition` with `Capabilities []Capability`, each capability typed by a small enum — Intensity, Color, Position(Pan/Tilt), Beam(Zoom/Focus), Gobo, etc.) that both the hand-authored-YAML path and the OFL-import path normalize into. Never let downstream pool/programming code see OFL's `channels`/`modes`/`wheels` shape directly.
**When to use:** Always — this is the FIXT-01/03 boundary. It is what makes D-08 (GDTF-friendly extensibility) achievable without a schema rewrite later, since GDTF is also attribute/geometry-based rather than channel-index-based.
**Example (illustrative target shape, not yet in this repo):**
```go
// Source: OFL docs/fixture-format.md + docs/capability-types.md
// (github.com/OpenLightingProject/open-fixture-library)
type CapabilityType string

const (
    CapIntensity  CapabilityType = "intensity"
    CapColor      CapabilityType = "color"
    CapPan        CapabilityType = "pan"
    CapTilt       CapabilityType = "tilt"
    CapZoom       CapabilityType = "zoom"
    CapFocus      CapabilityType = "focus"
    CapGobo       CapabilityType = "gobo"
    // ... extend per D-05's v1 target set; OFL's ~30 types are the
    // superset reference, not all required for v1.
)

type Capability struct {
    Type    CapabilityType
    Range   [2]float64 // normalized 0..1, not raw DMX — keeps the model
                       // protocol-agnostic (Art-Net today, GDTF/other later)
    Comment string
}
```

### Pattern 2: OFL import through the same normalization + validation pipeline (FIXT-03/D-06)
**What:** The OFL importer parses OFL's own JSON shape into an intermediate representation, then maps it onto `FixtureDefinition`. Capabilities OFL supports that v1's capability enum does not yet model are captured as explicit `LossyImportWarning` entries on the provenance record — never dropped silently and never rejected outright (per D-06).
**When to use:** Every OFL import, including fixtures outside the v1 target set (pixel/matrix, exotic multi-mode).
**Example:**
```go
// Source: OFL docs/fixture-format.md (schema `version` field, semver:
// MAJOR = breaking, MINOR = old-still-valid) — github.com/OpenLightingProject/open-fixture-library
type OFLImportResult struct {
    Definition FixtureDefinition
    Provenance Provenance // source="ofl", ofl_key, ofl_schema_version, fetched_at
    Warnings   []LossyImportWarning // e.g. "pixel matrix channel group not modeled in v1"
}
```

### Pattern 3: Impact plan as a generalization of `internal/trace/apply` (POOL-03..08)
**What:** A pool/substitution impact plan carries `schema_version`, an `expected_revision` (the show revision it was computed against), a `plan_id = sha256(strictjson.CanonicalEncode(planBody))`, an ordered list of affected-dependent operations, and separate `warnings`/`errors` lists. Before any apply: recompute `plan_id` from the plan's own bytes (integrity) and recompute the *current* impact plan from current show state and compare `plan_id` (freshness) — reject on mismatch with a clear "re-run review" message, exactly like `ValidatePlanIntegrity`/`ValidatePlanFreshness` do today for Linear sync plans.
**When to use:** Every pool add/remove (POOL-03/05) and every fixture-substitution review (POOL-06/07/08).
**Example:**
```go
// Source: internal/trace/apply/guard.go (this repo, Phase 1)
func ValidatePlanFreshness(plan ImpactPlan, currentShow ShowState) error {
    fresh, err := BuildImpactPlan(currentShow, plan.Request)
    if err != nil {
        return fmt.Errorf("GOLC_POOL_PLAN_STALE: recomputing failed: %v", err)
    }
    if fresh.PlanID != plan.PlanID {
        return fmt.Errorf(
            "GOLC_POOL_PLAN_STALE: plan %s no longer matches current show state (recomputed %s); re-run review",
            plan.PlanID, fresh.PlanID)
    }
    return nil
}
```

### Pattern 4: Dry-run/apply CLI split (D-15), Terraform-precedented
**What:** `golc pool update <pool> --add ... --remove ...` computes and prints the impact plan (human-readable by default, `--json` for machine-readable) and writes nothing. A separate `golc pool apply <plan-file-or-id>` performs `ValidatePlanIntegrity` → `ValidatePlanFreshness` → single-transaction apply. Never let one invocation both compute and mutate.
**When to use:** All pool and substitution operations (POOL-04/05/08).
**Example:** Terraform's `terraform plan -out=tfplan` then `terraform apply tfplan` is the reference UX shape; GOLC's `linear preview`/`linear apply` split (already implemented in this repo for Linear sync) is the closer, in-repo precedent to copy verbatim for command structure.

### Anti-Patterns to Avoid
- **Letting pool/programming code depend on OFL's channel/mode indices directly:** Breaks D-08 and makes every future fixture-substitution comparison channel-number-based instead of capability-based, which POOL-06/07 explicitly forbid ("never silently approximated" only holds if the comparison is semantic).
- **Building a second staleness/plan-hash mechanism for fixtures instead of reusing `internal/trace/apply`'s:** Two independently-evolving "is this plan still valid?" implementations in one codebase is exactly the kind of drift D-16 calls out as a risk.
- **Treating "hazard" capabilities as something to detect from imported fixture data:** Neither OFL nor GDTF ship a hazard/safety field (see Common Pitfalls) — don't build a parser for a field that doesn't exist; any hazard/severity signal in GOLC is domain logic GOLC defines itself (D-14), not something extracted from a fixture file.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Duplicate-key / unknown-field YAML rejection (FIXT-02) | A custom YAML pre-scanner or line-based duplicate-key detector | `go.yaml.in/yaml/v4`'s `WithUniqueKeys()` + `WithKnownFields()` | Both ship as tested, default-on Options-API behaviors in the library already indirectly vendored in this repo; hand-rolling duplicates a security-relevant parser feature. |
| JSON Schema authoring for the fixture format (FIXT-01) | Hand-written `schemas/fixture.schema.json` maintained independently of the Go struct | `invopop/jsonschema` generation from the canonical `FixtureDefinition` Go struct | This repo already solved "one struct, one generated schema, zero drift" for `golc-project.schema.json`/`config-*.schema.json`; a hand-authored fixture schema would immediately be a second, divergent source of truth. |
| Content-hash pinning / staleness detection (FIXT-05, POOL-05/08) | A bespoke fixture-hash or impact-plan-hash scheme | `internal/strictjson.CanonicalEncode` + `crypto/sha256`, following `internal/trace/apply`'s `recomputePlanID`/`ValidatePlanIntegrity`/`ValidatePlanFreshness` pattern | Already implemented, tested (idempotent, deterministic, sorted-key canonical JSON) in this exact repo for an analogous problem (Linear sync plan staleness) — reuse rather than reinvent. |
| OFL fixture format parsing | A hand-rolled OFL JSON reader built from guesswork | The documented OFL fixture format (`docs/fixture-format.md`) and capability-types reference (`docs/capability-types.md`) at github.com/OpenLightingProject/open-fixture-library | The format is fully documented with a versioned schema; reading the spec once and generating a typed intermediate struct from it is far more reliable than reverse-engineering example files. |

**Key insight:** Every hard problem in this phase — strict parsing, schema generation, content-addressed pinning, atomic reviewed apply — already has either an external, well-documented library (OFL format + go.yaml.in/yaml/v4) or an internal, already-tested pattern (`internal/strictjson`, `internal/trace/apply`) to reuse. The only genuinely novel design work is the capability-based canonical fixture model itself and the pool/deployment domain concepts (POOL-01/02) — both of which are pure GOLC domain modeling with no external schema to defer to.

## Common Pitfalls

### Pitfall 1: Assuming OFL fixture data carries a permissive-but-distinct data license (e.g. CC-BY-SA) separate from the MIT-licensed application code
**What goes wrong:** Teams sometimes assume a data-heavy open project like OFL splits code (MIT) from data (Creative Commons), and build an attribution/redistribution workflow around that assumption.
**Why it happens:** Many open-data projects do use that split; OFL does not — the whole repository, including `fixtures/*.json`, is registered as a single MIT-licensed GitHub repository.
**How to avoid:** Treat OFL fixture definitions as MIT-licensed for import/redistribution purposes; confirm at implementation time against the actual `LICENSE` file in whichever OFL mirror/fork the user points GOLC at (D-07 allows a user-configured mirror, which could in principle carry different terms).
**Warning signs:** A CONTRIBUTING/legal review step that asks "do we need CC-BY-SA attribution for imported fixtures?" — the answer for upstream OFL is no, but re-verify for any non-default mirror URL.

### Pitfall 2: Modeling fixture-substitution capability gaps as a hazard/safety concept borrowed from the fixture library
**What goes wrong:** Spending research/design time looking for how OFL or GDTF flag "hazardous" or safety-critical attributes (strobe, UV, lasers) to reuse for POOL-07's severity taxonomy.
**Why it happens:** The roadmap's phase research note explicitly calls out "hazardous attributes" as an open research area, implying such a field might exist upstream.
**How to avoid:** Confirmed via primary-source review of OFL's full capability-types documentation: no such field exists in either OFL or GDTF. POOL-07's missing/incompatible/unsupported severity taxonomy (D-14) is a GOLC-original design decision, not an import from fixture metadata. Any "this is a strobe/UV capability, warn the operator" behavior is GOLC application logic keyed off `CapabilityType`, not a flag read from fixture data.
**Warning signs:** A plan task that says "parse OFL's hazard field" — there is no such field to parse.

### Pitfall 3: Letting `go.yaml.in/yaml/v4`'s pre-release (`rc.N`) status surprise-break the strict-decode API between plan-time and execution-time
**What goes wrong:** Planning against `v4.0.0-rc.2` (this repo's currently-pinned indirect version) and discovering at execution time that the Options API (`WithUniqueKeys`/`WithKnownFields`) shipped or changed shape in a later rc, or that `rc.2` itself predates that API.
**Why it happens:** The module is still in release-candidate status (`v4.0.0-rc.1` through `rc.6` observed at research time); rc-to-rc API changes are more likely than in a stable v4.0.0.
**How to avoid:** Re-run `go list -m -versions go.yaml.in/yaml/v4` and check `docs/options.md` in the exact tag being pinned before writing FIXT-02's decode code; pin an exact version (not a floating rc) in `go.mod`.
**Warning signs:** `go build` failures on `yaml.WithUniqueKeys`/`yaml.WithKnownFields` not existing, or `go.sum` showing a different rc than what was researched.

## Code Examples

Verified patterns from official/primary sources:

### Strict YAML decode rejecting duplicate keys and unknown fields
```go
// Source: github.com/yaml/go-yaml docs/options.md (go.yaml.in/yaml/v4)
loader, err := yaml.NewLoader(reader,
    yaml.WithKnownFields(), // defaults to true; fails on unmodeled keys
    yaml.WithUniqueKeys(),  // defaults to true; fails on duplicate mapping keys
)
if err != nil {
    return err
}
var def FixtureDefinition
if err := loader.Load(&def); err != nil {
    // e.g. "admin: false / admin: true" -> duplicate key error
    return fmt.Errorf("GOLC_FIXTURE_YAML_INVALID: %v", err)
}
```

### OFL schema-version convention to mirror for FIXT-01's own YAML schema
```markdown
<!-- Source: docs/fixture-format.md, github.com/OpenLightingProject/open-fixture-library -->
The schema files have a `version` property. Every time the schema is updated,
this version needs to be incremented using semantic versioning:
1. MAJOR version when you make incompatible schema changes
   (old fixtures are not valid with the new schema anymore).
2. MINOR version ... (old fixtures are still valid with the new schema,
   new fixtures aren't valid with the old schema).
```

### Plan integrity + freshness gates to generalize for pool/substitution impact plans
```go
// Source: internal/trace/apply/guard.go (this repo)
func ValidatePlanIntegrity(plan reconcile.Plan) error {
    if plan.SchemaVersion != reconcile.SchemaVersion {
        return fmt.Errorf("GOLC_APPLY_PLAN_SCHEMA: ...")
    }
    recomputed, err := recomputePlanID(plan)
    if err != nil { return err }
    if recomputed != plan.PlanID {
        return fmt.Errorf("GOLC_APPLY_PLAN_HASH: plan_id %q does not match its own recomputed canonical hash %q; the plan bytes were altered after hashing", plan.PlanID, recomputed)
    }
    return nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `gopkg.in/yaml.v2`/`v3` as the de facto Go YAML library | `go.yaml.in/yaml/v4` under the `github.com/yaml/go-yaml` org, with a new Options/Load-Dump API alongside the classic Decoder/Encoder API | v4 line, still release-candidate at research time (`rc.1`–`rc.6` observed) | Strict decoding (`WithKnownFields`/`WithUniqueKeys`) is now a first-class, default-on option rather than something bolted on via `UnmarshalStrict` — simplifies FIXT-02's implementation. |
| GDTF pre-standardization (vendor-specific device description formats) | GDTF is now officially recognized as DIN SPEC 15800 (v1.2, 2022), backed by MA Lighting, Robe, Vectorworks | 2021–2022 | Strengthens the case for D-08's capability/geometry-friendly canonical model — GDTF is now a real interoperability target, not a vendor experiment. |

**Deprecated/outdated:**
- Treating OFL as JSON-only for downstream consumption: OFL's own website already exports many console-native formats from its JSON source — GOLC importing directly from the canonical JSON (rather than an already-lossy exported format) is the correct approach and matches D-07's "OFL online or a local mirror" framing.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | GDTF's attribute/geometry model is accurately summarized from general web search (not a primary-source spec read) as "hierarchical Geometry + logical Attribute graph" | Standard Stack, Architecture Patterns (D-08 rationale) | If GDTF's actual structure differs materially, the "capability-based, not hard-wired to OFL's shape" design guidance for D-08 could still be directionally right but the specific geometry/attribute vocabulary used when GDTF import is eventually built (post-v1) may need revision. Low risk to Phase 2 itself since no GDTF parser is built now. |
| A2 | The set of OFL-cataloged small-venue-representative fixtures (generic RGB PARs, LED washes, moving-head spot/wash) was confirmed via general web search, not by enumerating specific OFL fixture keys to pin as the v1 test/acceptance corpus | Code Examples, Common Pitfalls, phase requirement FIXT-03 support | The planner should pick 3–6 *specific* OFL fixture keys (e.g. by browsing open-fixture-library.org/categories) during planning or Wave 0, rather than treating this research's general claim as sufficient to select an acceptance corpus. |
| A3 | `go.yaml.in/yaml/v4`'s Options API (`WithUniqueKeys`, `WithKnownFields`) is present in the currently go.sum-pinned `v4.0.0-rc.2`, not only in later rcs | Standard Stack, Common Pitfalls #3 | If `rc.2` predates the Options API, FIXT-02's implementation needs a version bump before this pattern works; verify with `go list -m -versions` + reading `docs/options.md` from the exact pinned tag before writing decode code (flagged explicitly as a pitfall above). |

## Open Questions

1. **Exact set of OFL fixture keys to pin as the v1 acceptance/test corpus**
   - What we know: D-05 names the *categories* (simple/color-changing PARs, washes, moving-head spot/wash); OFL's catalog has many manufacturer entries per category.
   - What's unclear: Which specific fixture keys (e.g. a specific Chauvet/ADJ/generic PAR, a specific moving-head spot) become the fixed test/acceptance corpus referenced by the roadmap's "physical validation corpus" research note.
   - Recommendation: The planner should select 3–6 concrete OFL fixture keys (favoring well-known, stable, "generic"-manufacturer entries where available, to minimize churn if a specific vendor's definition changes) during plan-writing, and pin them by content hash (FIXT-05) as soon as they're imported in tests.

2. **Whether OFL's live REST API (`/api/v1`) or direct raw-JSON-file fetch from the fixture repository is the intended D-07 "live fetch" mechanism**
   - What we know: OFL's documented REST API (`docs/rest-api.md`) exposes search, manufacturer listing, and fixture-editor/import endpoints, but no documented direct "GET one fixture's canonical JSON by key" endpoint in the v1.0 API spec reviewed.
   - What's unclear: The stable, intended way to fetch one fixture's OFL JSON by manufacturer/fixture key for D-07's live-fetch-plus-cache flow — likely either a raw GitHub content fetch (`fixtures/<man>/<key>.json`) or a website download URL, not the REST API's search/import endpoints.
   - Recommendation: Confirm the exact fetch URL pattern during planning (e.g. by inspecting one fixture's "Download" button behavior on open-fixture-library.org, or the plugin export mechanism in `docs/plugins.md`) before committing to an HTTP client implementation.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All of Phase 2 (pure Go domain/CLI code) | ✓ | go1.26.5 windows/amd64 | — |
| Network access to open-fixture-library.org | FIXT-03/D-07 OFL live fetch | ✓ (verified: HTTP 200 at research time) | — | D-07 already designs for offline-after-pin; a local mirror is the documented fallback if connectivity is unavailable at a given site. |
| `go.yaml.in/yaml/v4` at a version with the Options API | FIXT-02 strict decode | ✓ available on module proxy (rc.1–rc.6 observed) | verify exact version at execution time | If the Options API is absent in the version ultimately pinned, fall back to the classic `Decoder.KnownFields(true)` method plus a custom duplicate-key check (present in the library per multiple sources, but confirm against the pinned tag). |
| `github.com/invopop/jsonschema` | FIXT-01 schema generation | ✓ (already a direct dependency, v0.14.0) | v0.14.0 | — |

**Missing dependencies with no fallback:** none identified.
**Missing dependencies with fallback:** none currently missing — flagged above only as a version-drift contingency.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package + this repo's own `internal/command` "test" route wrapper |
| Config file | none — driven by `golc.project.toml`/`config/commands.toml` (`go_version = "ref:toolchain.go.version"`) |
| Quick run command | `golc test --quick` (project-wide `go vet` sanity gate, <=30s budget per 01-VALIDATION.md) |
| Full suite command | `golc test` (bare) — every project Go package's tests via the pinned toolchain |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FIXT-01 | Load a valid YAML fixture definition | unit | `go test ./internal/fixture/... -run TestLoad` | ❌ Wave 0 |
| FIXT-02 | Reject duplicate keys / unknown fields / invalid ranges with actionable diagnostics | unit (table-driven, one case per rejection class) | `go test ./internal/fixture/... -run TestDecodeRejects` | ❌ Wave 0 |
| FIXT-03 | Import an OFL fixture through the canonical pipeline | unit + fixture-corpus | `go test ./internal/fixture/ofl/... -run TestImport` | ❌ Wave 0 |
| FIXT-04 | Validate a hand-authored custom fixture via CLI | integration (CLI route) | `go test ./internal/command/... -run TestFixtureValidateRoute` | ❌ Wave 0 |
| FIXT-05 | Stable identity/hash pinning survives re-read | unit (round-trip) | `go test ./internal/fixture/... -run TestIdentityHashStable` | ❌ Wave 0 |
| FIXT-06 | Provenance/warning inspection surfaces lossy import details | unit | `go test ./internal/fixture/... -run TestProvenance` | ❌ Wave 0 |
| POOL-01/02 | Define pool, map to deployment instances/addresses | unit | `go test ./internal/pool/... ./internal/deployment/...` | ❌ Wave 0 |
| POOL-03/04/05 | Impact review + configurable propagation + atomic apply | unit + property (`pgregory.net/rapid`, per AGENTS.md stack research) | `go test ./internal/pool/... -run TestImpactPlan` | ❌ Wave 0 |
| POOL-06/07 | Capability-based substitution, severity taxonomy | unit | `go test ./internal/substitution/... -run TestCapabilityDiff` | ❌ Wave 0 |
| POOL-08 | Accept/revise/cancel plan before it changes the show | integration (CLI dry-run/apply route pair) | `go test ./internal/command/... -run TestPoolApplyRoute` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `golc test --quick`
- **Per wave merge:** `golc test`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/fixture/` package + tests — no fixture domain code exists yet (greenfield per STATE.md integration-points note)
- [ ] `internal/pool/`, `internal/deployment/`, `internal/substitution/` packages + tests — greenfield
- [ ] A small local OFL fixture fixture-corpus under `tests/fixtures/ofl/` (a handful of real, pinned OFL JSON files) so FIXT-03 tests don't depend on live network access
- [ ] Decide and pin the specific `go.yaml.in/yaml/v4` version before writing FIXT-02 tests (see Open Questions / Pitfall 3)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|----------------|---------|-------------------|
| V2 Authentication | no | Phase 2 is headless/local CLI-only; no authentication surface introduced. |
| V3 Session Management | no | No session concept in this phase. |
| V4 Access Control | no | Single-operator local tool; no multi-actor access control in this phase. |
| V5 Input Validation | yes | Strict YAML decode (`go.yaml.in/yaml/v4` `WithKnownFields`/`WithUniqueKeys`) + explicit range/semantics validation with actionable `GOLC_FIXTURE_*` diagnostic codes for every fixture field; strict JSON handling (`internal/strictjson.DecodeStrict`) for any plan/report artifacts. |
| V6 Cryptography | yes (narrow) | `crypto/sha256` (stdlib) via `internal/strictjson.CanonicalEncode` for content-hash identity/pinning — never hand-roll a hash function; sha256 is sufficient here (integrity/identity, not a security boundary against a motivated attacker). |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|----------------------|
| Malicious/malformed OFL fixture JSON (or a compromised user-configured OFL mirror per D-07) injecting unexpected structure | Tampering | Route OFL import through the *same* strict-decode + canonical-model validation pipeline as hand-authored YAML (FIXT-03 requirement); never trust fetched JSON to be well-formed. |
| A show file or hand-crafted plan artifact with a tampered `plan_id`/hash, replayed to bypass freshness checking | Tampering | `ValidatePlanIntegrity`-style self-hash recomputation (already proven in `internal/trace/apply/guard.go`) before any apply — reject on any mismatch. |
| Server-Side Request Forgery via a user-supplied "local mirror" URL for OFL fetch (D-07) pointing at an internal/unintended network target | Tampering / Information Disclosure | Treat the mirror URL as untrusted input: validate scheme (http/https only), consider an explicit opt-in/allowlist step before fetching from a non-default host, and apply request timeouts/size limits. |
| Duplicate-key YAML "key override" attacks (a second `admin:`/similarly-named key silently overriding an earlier one) | Tampering | `WithUniqueKeys()` (default true) rejects this at parse time — explicitly documented by the library as "a security feature that prevents key override attacks." |

## Sources

### Primary (HIGH confidence)
- `go.mod`, `go list -m -versions`, `go list -m all` (this repo, run directly) — confirmed exact currently-pinned `go.yaml.in/yaml/v4` version (`v4.0.0-rc.2`, indirect) and upstream version availability through `rc.6`.
- `internal/command/router.go`, `internal/trace/apply/guard.go`, `internal/trace/apply/model.go`, `internal/strictjson/decode.go`, `internal/projectconfig/decode.go` (this repo) — direct read of established route-registration, plan-integrity/freshness, and canonical-encoding patterns to reuse.
- GitHub license API (`gh api repos/OpenLightingProject/open-fixture-library/license`) — confirmed MIT for the whole OFL repository including fixture data files.

### Secondary (MEDIUM confidence)
- `docs/fixture-format.md`, `docs/capability-types.md`, `docs/rest-api.md`, `README.md` — fetched directly from `github.com/OpenLightingProject/open-fixture-library` via `gh api repos/.../contents/...` (raw primary-source documentation, not AI-summarized search results). [CITED: github.com/OpenLightingProject/open-fixture-library]
- `docs/options.md`, `yaml.go` — fetched directly from `github.com/yaml/go-yaml` (the repository backing the `go.yaml.in/yaml/v4` module path) via `gh api`. [CITED: github.com/yaml/go-yaml]
- AGENTS.md `<!-- GSD:stack-start -->` section (this repo) — prior stack research already establishing Go 1.26.5, `github.com/google/uuid` v1.6.0, `pgregory.net/rapid` for property tests, etc.

### Tertiary (LOW confidence)
- WebSearch results on GDTF/DIN SPEC 15800 general structure (not a primary spec document read) — see Assumptions Log A1.
- WebSearch results on representative small-venue OFL fixture examples (not a specific fixture-key enumeration) — see Assumptions Log A2.
- WebSearch results on general content-addressable-hash/lockfile conventions and Terraform's plan/apply UX — used only to corroborate patterns already directly observed in this repo's own code (`internal/trace/apply`) and in OFL's own docs.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — `go.yaml.in/yaml/v4` Options API and `invopop/jsonschema` usage confirmed by direct primary-source read (`gh api`) and by this repo's own `go.mod`/existing schema-generation code.
- Architecture: MEDIUM — the fixture-catalog half is grounded in directly-read OFL documentation; the pool/deployment half is grounded in directly-read internal precedent (`internal/trace/apply`), but the pool/deployment domain model itself is original design work with no external validation.
- Pitfalls: MEDIUM — the OFL-license and no-hazard-field findings are primary-source-confirmed; the yaml-rc-version-drift pitfall is a forward-looking risk flag, not yet-observed breakage.

**Research date:** 2026-07-21
**Valid until:** 2026-08-20 (30 days — stable domain, but re-verify the `go.yaml.in/yaml/v4` pinned version and OFL mirror/license status at plan-execution time given the library's pre-release status)
