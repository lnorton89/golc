# Phase 2: Modular Fixtures and Deployments - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-21
**Phase:** 2-Modular Fixtures and Deployments
**Areas discussed:** Phase 2 interface boundary, Fixture catalog scope for v1, Pool & deployment mental model, Impact review UX for changes

---

## Phase 2 Interface Boundary

**Q: Should Phase 2 be a headless domain model + validation engine + CLI/API surface only, or include minimal in-app UI?**

| Option | Description | Selected |
|--------|-------------|----------|
| Headless only | Fixture/pool/deployment model, validation, OFL import, and impact-review logic all live behind CLI/API. No Wails UI work in Phase 2. | ✓ |
| Minimal fixture browser UI now | Add a lightweight in-app fixture catalog browser/validator UI ahead of Phase 6. | |
| You decide | | |

**Q: How should a show author create/validate a custom fixture definition day-to-day?**

| Option | Description | Selected |
|--------|-------------|----------|
| Validate-only CLI | Author hand-writes YAML and runs a `golc fixture validate <file>`-style command. | ✓ |
| Validate + scaffold CLI | Add a `golc fixture new` scaffolding command generating a starter YAML skeleton. | |
| You decide | | |

**Q: What should "share" mean for a custom fixture definition (FIXT-04) in this headless phase?**

| Option | Description | Selected |
|--------|-------------|----------|
| File-level sharing | A validated custom fixture is just a portable YAML file plus its computed identity/hash. | ✓ |
| Local library registry | Custom fixtures get added to a local personal fixture library other shows can reference by ID. | |
| You decide | | |

**Q: Should Phase 2 route fixture/pool/deployment operations through the same shared typed command model (internal/command)?**

| Option | Description | Selected |
|--------|-------------|----------|
| Route through shared command model | Operations register as commands in the same model Phase 1 established for config. | ✓ |
| Plain Go package APIs for now | Keep as ordinary Go functions/packages, adopt command model later. | |
| You decide | | |

**Notes:** No follow-up clarifications — all four questions converged on the recommended option.

---

## Fixture Catalog Scope for v1

**Q: What's the representative first-user fixture set Phase 2's validation/import pipeline must handle cleanly for v1?**

| Option | Description | Selected |
|--------|-------------|----------|
| Simple + moving heads | Static/color-changing PARs, wash fixtures, and moving-head spots/washes (intensity, color, position, beam/zoom, gobo). | ✓ |
| Simple fixtures only | Static/color-changing PARs/washes only, no pan/tilt. | |
| Everything OFL supports | Target matrix/pixel fixtures and complex multi-mode fixtures from day one. | |

**Q: For fixtures outside the v1 target set imported from OFL, what should happen?**

| Option | Description | Selected |
|--------|-------------|----------|
| Import with lossiness flagged | Import proceeds; unsupported capabilities surfaced as explicit warnings per FIXT-06. | ✓ |
| Reject at import | OFL definitions using unsupported fixture categories are rejected outright. | |
| You decide | | |

**Q: How should OFL fixture data reach GOLC given the offline-first constraint?**

| Option | Description | Selected |
|--------|-------------|----------|
| Live fetch + local cache | Import fetches from OFL online/mirror and caches/pins locally; offline after import. | ✓ |
| Vendored snapshot only | Ship a fixed, versioned OFL snapshot bundled with GOLC. | |
| You decide | | |

**Q: Should Phase 2's fixture model actively preserve GDTF-compatible data, or is GDTF fully out of scope?**

| Option | Description | Selected |
|--------|-------------|----------|
| Design for it, don't import it | Keep the canonical model capability-based/extensible for future GDTF import; no GDTF parser now. | ✓ |
| Fully out of scope for now | Don't design around GDTF at all for Phase 2. | |
| You decide | | |

**Notes:** No follow-up clarifications — all four questions converged on the recommended option.

---

## Pool & Deployment Mental Model

**Q: Is a "deployment" a saved, named mapping of pools to concrete fixtures (multiple per show), or exactly one live deployment per show?**

| Option | Description | Selected |
|--------|-------------|----------|
| Multiple named deployments | A show can hold several saved deployments (e.g. per-venue) with one marked active. | ✓ |
| Single deployment per show | A show has exactly one deployment at a time. | |
| You decide | | |

**Q: How does a "group" relate to a "pool"?**

| Option | Description | Selected |
|--------|-------------|----------|
| Groups cut across pools | A group is an independent named selection of fixtures, orthogonal to pools. | ✓ |
| Groups are subsets within a pool | A group is scoped to one logical pool. | |
| You decide | | |

**Q: When fixtures are added to a pool, should GOLC auto-assign addresses for new deployment instances, or require manual assignment?**

| Option | Description | Selected |
|--------|-------------|----------|
| Auto-assign, reviewable | GOLC proposes addresses as part of the impact plan; author can adjust before accepting. | ✓ |
| Always manual assignment | New pool members are added unaddressed; author must explicitly assign each. | |
| You decide | | |

**Q: Roughly how many fixtures/pools does a typical first-user show involve, for scale assumptions?**

| Option | Description | Selected |
|--------|-------------|----------|
| Small rig | ~10–50 fixtures across 3–8 pools. | ✓ |
| Medium rig | ~50–150 fixtures across many pools. | |
| You decide | | |

**Notes:** No follow-up clarifications — all four questions converged on the recommended option.

---

## Impact Review UX for Changes

**Q: Should the author be able to selectively accept parts of an impact plan, or is it all-or-nothing?**

| Option | Description | Selected |
|--------|-------------|----------|
| All-or-nothing | The impact plan is reviewed and either fully accepted or cancelled as a single atomic unit; "revise" reruns the review. | ✓ |
| Per-item accept/reject | Author can accept some affected items and reject/defer others, producing a partial apply. | |
| You decide | | |

**Q: Should an unmapped-but-non-hazardous capability gap block acceptance of a substitution plan, or just warn?**

| Option | Description | Selected |
|--------|-------------|----------|
| Warn, don't block | Missing/incompatible capabilities surfaced with severity but author can still accept the plan; true errors can still hard-block. | ✓ |
| Any incompatibility blocks | The plan cannot be accepted while any capability gap remains unresolved. | |
| You decide | | |

**Q: How should the impact review actually be presented/accepted given no UI in Phase 2?**

| Option | Description | Selected |
|--------|-------------|----------|
| CLI dry-run + confirm | A command computes/prints the impact plan as a dry-run; a separate apply/confirm step commits it. | ✓ |
| Single command with confirmation prompt | One CLI command shows the plan and asks for interactive y/n confirmation. | |
| You decide | | |

**Q: Should impact plans reuse Phase 1's D-18 staleness-detection pattern?**

| Option | Description | Selected |
|--------|-------------|----------|
| Reuse staleness pattern | Impact plans carry an expected show revision; apply fails safely if the revision moved. | ✓ |
| Skip staleness check | Treat plan-then-apply as effectively immediate for this single-operator workflow. | |
| You decide | | |

**Notes:** No follow-up clarifications — all four questions converged on the recommended option.

---

## Claude's Discretion

None — every question in this session converged on the recommended option; no area was left to Claude's judgment.

## Deferred Ideas

None — discussion stayed within phase scope (FIXT-01–06, POOL-01–08). No scope-creep items came up.
