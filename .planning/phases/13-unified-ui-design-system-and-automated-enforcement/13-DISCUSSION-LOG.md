# Phase 13: Unified UI Design System and Automated Enforcement - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-02
**Phase:** 13-unified-ui-design-system-and-automated-enforcement
**Areas discussed:** Visual direction, migration breadth, component ownership, enforcement, exceptions, verification

---

## Visual Direction

| Option | Description | Selected |
|--------|-------------|----------|
| Preserve and formalize | Keep the validated Paper/Ink console language and remove drift | ✓ |
| Redesign the product | Establish an unrelated new visual language | |

**User's choice:** Auto-selected the recommended preserve-and-formalize path from existing locked brand and sketch decisions.
**Notes:** The reported problem is inconsistency, not dissatisfaction with the approved brand direction.

---

## Migration Breadth

| Option | Description | Selected |
|--------|-------------|----------|
| Migrate all reachable UI | End the phase with one system across the current app | ✓ |
| New code only | Leave existing inconsistencies in place | |
| Selected workspaces only | Improve visible hotspots while retaining two systems | |

**User's choice:** Auto-selected full current-app migration.
**Notes:** This directly matches the request to unify the app rather than merely prepare future primitives.

---

## Component Ownership

| Option | Description | Selected |
|--------|-------------|----------|
| Semantic tokens plus typed primitives | Centralize shared visuals and interaction states | ✓ |
| Tokens only | Continue hand-building controls in feature CSS | |
| Component library only | Allow unrestricted raw visual values in feature styles | |

**User's choice:** Auto-selected semantic tokens plus typed primitives.
**Notes:** Feature CSS retains layout and domain-visualization responsibility.

---

## Enforcement

| Option | Description | Selected |
|--------|-------------|----------|
| Static checks plus tests and CI | Fail locally and in CI with actionable diagnostics | ✓ |
| Documentation only | Rely on agent discipline | |
| Visual snapshots only | Detect appearance drift after rendering | |

**User's choice:** Auto-selected layered enforcement.
**Notes:** The checker must start green and cannot hide known violations in an open-ended baseline.

---

## Exceptions

| Option | Description | Selected |
|--------|-------------|----------|
| Audited exception manifest | Record file, rule, rationale, and review condition | ✓ |
| Inline suppression comments | Allow local bypasses without central review | |
| No exceptions | Force all domain visuals into generic primitives | |

**User's choice:** Auto-selected an audited exception manifest.
**Notes:** This preserves intentional locked values such as the approved onboarding grid while keeping drift visible.

---

## Verification

| Option | Description | Selected |
|--------|-------------|----------|
| Policy, unit, accessibility, and visual layers | Cover authoring-time drift and rendered regressions | ✓ |
| Unit tests only | Validate APIs without appearance coverage | |
| Manual review only | Depend on screenshots and human memory | |

**User's choice:** Auto-selected layered verification.
**Notes:** Representative light/dark states should be stable and bounded enough for CI.

## Claude's Discretion

- Exact checker implementation and file decomposition.
- Exact component API names and migration order.

## Deferred Ideas

None.
