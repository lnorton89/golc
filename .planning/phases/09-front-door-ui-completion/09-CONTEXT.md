# Phase 9: Front-Door UI Completion - Context

**Gathered:** 2026-07-27
**Status:** Ready for planning

<domain>
## Phase Boundary

A new operator can go from a fresh checkout to a patched fixture and a scene on
screen using only the UI — no CLI required for the happy path. Three concrete
gaps close this: (1) the Fixture Library workspace (currently a `ComingSoon`
stub) becomes a real browse/inspect/import UI, (2) show open/new/switch gets
on-screen controls (today the desktop app resolves exactly one fixed show path
at process startup), and (3) Guided First Show onboarding — already fully
designed in Sketch 004 Variant B — gets built.

This phase clarifies HOW to wire on-screen controls against backend routes
that already exist from Phases 2 (fixture validate/inspect/import), 5 (show
open/save/save-as/recovery), and 6 (the Wails shell, command rail, service
pattern). It does not add new backend domain capabilities — no new fixture,
pool, or show semantics beyond what Phases 2/5 already validated.

Requirements: FDUI-01, FDUI-02, FDUI-03.

</domain>

<decisions>
## Implementation Decisions

### Fixture Library Browsing
- **D-01:** Browsing combines **a local directory listing of already-imported/validated fixtures plus a separate OFL catalog search** to find and import new ones — not local-only, not OFL-only. No backend "list" route exists yet for either source; this phase adds it.
- **D-02:** Selecting a fixture (local or OFL) shows its **inspect view inline in the library workspace** (identity, provenance, validation, lossy-import warnings) with a single confirm action to commit the import — mirrors the existing CLI's validate→inspect→import pipeline as one on-screen flow, not a separate modal/wizard.
- **D-03:** The library gets **basic text search** (name/manufacturer match) — no faceted filtering, no flat-list-only. Enough to keep a growing catalog navigable without over-building for a v1 single small rig.
- **D-04:** Hand-authored custom YAML fixtures (FIXT-04) are added by **pointing at a local file (native picker or path field) and validating inline** through the same `fixture validate` pipeline, with diagnostics shown before it's added — not an in-app YAML text editor.

### Show Open/New/Switch Mechanics
- **D-05:** "Switching shows" means the **app relaunches (or respawns its daemon + reloads the webview) with the new show path** — not a true in-process live switch across all 7 services (Playback, Surface, Midi, FixturePatch, Programming, Show, Script), and not first-launch-only with no mid-session switching. Costs a brief visible restart/reload; avoids re-architecting every service's show-path binding.
- **D-06:** "New Show" is **the same mechanism as Open, pointed at a path that doesn't exist yet** — matches `main.go`'s existing comment that `show.Load` already treats a not-yet-existing file as a fresh empty show. No separate "New Show" setup flow, no new backend concept.
- **D-07:** The **app itself handles the restart/respawn** when a different show is selected — the operator never touches `GOLC_DESKTOP_SHOW` or relaunches `golc-desktop.exe` manually.

### Guided First Show Entry Point
- **D-08:** Guided First Show **auto-launches when a show has no fixtures/pools/scenes yet** (a genuinely empty/new show) — a show with existing content never auto-launches it. Safe because the flow is optional and Exit Guide is always available (locked design, `.planning/sketches/references/onboarding-readiness-impact.md`).
- **D-09:** Show open/new/switch (D-05..D-07) lives as a **new entry in the existing Show nav group**, alongside Overview and Save & Recovery — not folded into Overview's own workspace, not a new top-level entry point outside Show/Build/Operate/Output. Matches `application-shell-navigation.md`'s grouped-by-intent model.
- **D-10:** Guided First Show is **an overlay/flow reachable via auto-launch or a "Start Guide" action on Overview, not a permanent command-rail destination** — matches Sketch 004-B's HTML structure (a guided-flow section that takes over the canvas). Exiting returns to normal navigation.

### Claude's Discretion
None — every gray area discussed converged on an explicit user selection; all
recommended options were chosen.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements and roadmap
- `.planning/REQUIREMENTS.md` §"Front-Door Workflow" (FDUI-01, FDUI-02, FDUI-03) — the three locked requirements this phase's decisions satisfy.
- `.planning/ROADMAP.md` §"Phase 9: Front-Door UI Completion" — goal, dependency (Phase 8), and success criteria.
- `.planning/POST-PHASE-8-PLAN.md` §2 "Front-door workflow completion phase" — the owner-approved scope (2026-07-25) this phase's three requirements were sourced from verbatim.

### Locked design system (do not re-open without new evidence)
- `.planning/sketches/SKILL.md` — validated GOLC design system: Focused Command Rail shell, Show/Build/Operate/Output nav grouping, Paper/Ink palette, Signal Blue for selection/active state.
- `.planning/sketches/references/application-shell-navigation.md` — the exact Show/Build/Operate/Output nav grouping D-09 places the new show-management entry into; interaction contract ("selecting a command-rail destination replaces only the central workspace and inspector — navigation does not mutate show playback or output").
- `.planning/sketches/references/onboarding-readiness-impact.md` — the **fully locked** Guided First Show design (Sketch 004 Variant B): stages are Fixtures, Patch, Program, Assign, Verify; guidance is optional, saves progress, always offers Exit Guide; patch changes stay preview-only until reviewed/applied; readiness is evidence-based (blockers/warnings/optional evidence as distinct statuses), never a completion percentage. D-08/D-10 answer only WHEN/WHERE this flow is invoked — the flow's internal design itself is not up for discussion per this document's own "treat as validated, do not re-open" instruction.
- `.planning/brand/GOLC-Brand-Tokens.md` — status color vocabulary (live/frame-lock/armed/revoked/blackout/offline) already used by Phase 6; reuse for any new status indicators (e.g. fixture validation state, migration-required note).

### Existing backend routes this phase wires against (do not duplicate)
- `internal/command/fixture.go` — self-registered `fixture validate <file>`, `fixture inspect <file>`, `fixture import --ofl <manufacturer>/<key>|--ofl-file <path> --out <path>` routes (Phase 2, FIXT-01..06). All three take an explicit file path; none currently *lists* fixtures — D-01's directory/OFL browsing needs a new route or a Wails-service-level directory scan, to be resolved by research/planning.
- `internal/wails/svc_fixturepatch.go` (`FixturePatchService`) — existing pool/deployment Wails bindings; `AddPoolMemberPreview` already takes a `stableKey`/`contentHash`, i.e. it assumes the fixture is already identified/pinned — confirms the Fixture Library workspace's browsing/import job is genuinely new, not a duplicate of existing pool-creation UI.
- `internal/show` (Phase 5) — `show.Load`/`Save`/`SaveAs` and the recovery-point detect/accept/discard flow (SHOW-01..04) that D-05/D-06's open/new mechanics reuse.
- `cmd/golc-desktop/main.go` — `showPathEnvName` (`GOLC_DESKTOP_SHOW`), `defaultShowFileName`, and the single fixed `cfg.ShowPath` every one of the 7 constructed services binds to at startup; this is the exact architectural constraint D-05 works around by relaunching rather than live-switching.

No user-referenced ADRs/specs beyond the project's own planning docs and the sketch findings above came up during discussion — no additional canonical docs to add.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `frontend/src/workspaces/show/SaveRecoveryWorkspace.tsx` — existing Save/Save-As/recovery-point UI pattern (Toolbar, Panel, ScrollRegion, EmptyState primitives) that D-09's new Open/New/Switch entry should follow structurally; its own doc comment already flags "this workspace does not bind show open... there is no 'open a different show' flow to wire yet" as the exact gap this phase closes.
- `frontend/src/workspaces/build/*` (e.g. `ScriptsWorkspace.tsx` per Phase 8 D-16) — established library-workspace pattern (list view + inspector) that D-01/D-02's Fixture Library browse/inspect UI should follow, replacing the `ComingSoon` stub.
- `frontend/src/lib/wailsBridge.ts` — existing typed bridge functions (`saveShow`, `saveShowAs`, `detectRecoveryPoints`, `diagnoseShow`, etc.) as the pattern for whatever new bridge functions D-01 (fixture list/search) and D-05/D-07 (open/new/relaunch) require.
- `internal/wails/app.go` — existing supervised-daemon-spawn/dial/orphan-cleanup lifecycle (Phase 6/7 precedent); D-05/D-07's app-handles-the-restart mechanism should extend this existing supervision rather than inventing a second process-lifecycle model.

### Established Patterns
- `{DOMAIN}_{CONDITION}` diagnostic code convention (e.g. `GOLC_SHOW_STATE_INVALID`, `GOLC_TRANSPORT_ADAPTER_MISSING`) — any new diagnostics (e.g. fixture-list-source-unreadable, show-path-invalid-on-switch) should follow the same naming convention.
- Self-registering `internal/command` routes — the pattern any new backend route (e.g. a fixture-list route) should follow, per Phase 1-8 precedent.
- Wails service-per-feature pattern (`svc_fixturepatch.go`, `svc_show.go`, etc.) — D-01's fixture browsing and D-05's open/new mechanics should each get their own service method(s) following this existing structure, not a new binding pattern.

### Integration Points
- No backend route currently lists fixtures (by directory or OFL search) — D-01 requires new code, likely in `internal/command/fixture.go` (a new `fixture list` route) plus a Wails-service wrapper, or a Wails-only directory scan if a CLI-parity route isn't needed. Research/planning should decide.
- No "switch/relaunch with new show path" mechanism exists yet in `internal/wails/app.go` or `cmd/golc-desktop/main.go` — D-05/D-07 requires new code to trigger and coordinate an app-level restart with a different `ShowPath`.
- Guided First Show (D-08/D-10) has no existing frontend component (`grep -ri "guided|onboard" frontend/src` returns 0 hits per `POST-PHASE-8-PLAN.md`) — entirely new frontend surface, though its design is fully specified by the locked sketch reference.

</code_context>

<specifics>
## Specific Ideas

- The Fixture Library and show-switching gray areas both surfaced a real architectural finding during discussion (not from the user, from code scouting): `cmd/golc-desktop/main.go` constructs all 7 Wails services against one fixed `ShowPath` read once at startup, and no fixture-listing route exists anywhere in the backend. Both decisions (D-01, D-05) were made with this constraint explicit, choosing the lower-risk path (add a list route; relaunch-based switching) over a larger architectural change (live-switching every service's bound show path).
- Guided First Show's own internal design (stages, optional, evidence-based) was treated as already decided — the user was not asked to re-litigate it, only to decide the two things the locked sketch reference leaves open: trigger condition and nav placement.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. All three areas (Fixture Library browsing, show open/new/switch mechanics, Guided First Show entry point) were clarifications of how to implement FDUI-01/02/03, not new capabilities.

</deferred>

---

*Phase: 9-Front-Door UI Completion*
*Context gathered: 2026-07-27*
