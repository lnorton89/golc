# Phase 9: Front-Door UI Completion - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-27
**Phase:** 09-front-door-ui-completion
**Areas discussed:** Fixture Library browsing, Show open/new/switch mechanics, Guided First Show entry point

---

## Fixture Library Browsing

### How should a user discover fixtures to browse/import?

| Option | Description | Selected |
|--------|-------------|----------|
| Local directory + OFL search | List already-imported fixtures from a project-local directory plus a separate OFL catalog search to import new ones | ✓ |
| Local directory only | Only list already-imported fixtures; OFL import stays CLI-only | |
| OFL catalog search only | Focus on discovering/importing from OFL; skip a local browser | |

**User's choice:** Local directory + OFL search
**Notes:** No backend "list" route exists for either source yet — this phase adds it.

### Import flow once a fixture is found

| Option | Description | Selected |
|--------|-------------|----------|
| Inline inspect-then-commit | Selecting a fixture shows inspect view in the library workspace; single confirm commits the import | ✓ |
| Separate import modal/dialog | Browsing and importing are distinct steps via a dedicated dialog | |

**User's choice:** Inline inspect-then-commit
**Notes:** Mirrors the existing CLI validate→inspect→import pipeline as one on-screen flow.

### Search/filter

| Option | Description | Selected |
|--------|-------------|----------|
| Basic text search only | Filter by name/manufacturer text match | ✓ |
| Faceted filter | Multiple filter dimensions | |
| No search — flat list | Plain scrollable list | |

**User's choice:** Basic text search only

### Custom YAML fixtures (FIXT-04)

| Option | Description | Selected |
|--------|-------------|----------|
| Drop file + validate inline | Point at a local .yaml file, validate inline via the fixture validate pipeline | ✓ |
| Paste/edit YAML in-app | A text editor panel for writing/pasting YAML directly | |

**User's choice:** Drop file + validate inline

---

## Show Open/New/Switch Mechanics

### How should switching shows actually work?

| Option | Description | Selected |
|--------|-------------|----------|
| Relaunch with new path | App relaunches (or respawns daemon + reloads webview) with the new show path | ✓ |
| True in-process live switch | Re-point every one of the 7 services' show path in-process, no restart | |
| First-launch picker only | No mid-session switching, only a picker before the app loads | |

**User's choice:** Relaunch with new path
**Notes:** Discovered during code scouting that `cmd/golc-desktop/main.go` constructs all 7 services (Playback, Surface, Midi, FixturePatch, Programming, Show, Script) against one fixed `ShowPath` read once at startup — a live in-process switch would be a real architectural change, not UI wiring.

### New Show mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| Same mechanism as Open, new path | "New Show" is Open pointed at a not-yet-existing path — matches `show.Load`'s existing behavior | ✓ |
| Dedicated "New Show" flow with setup | A separate on-screen flow for creating a new show | |

**User's choice:** Same mechanism as Open, new path

### Who handles the restart?

| Option | Description | Selected |
|--------|-------------|----------|
| App handles it | Selecting a different show triggers the app to restart/respawn itself with the new path | ✓ |
| Operator relaunches manually | UI records a preference; operator closes/reopens the app themselves | |

**User's choice:** App handles it

---

## Guided First Show Entry Point

### Auto-launch or menu-triggered?

| Option | Description | Selected |
|--------|-------------|----------|
| Auto-launch for empty/new show | Opening a show with no fixtures/pools/scenes auto-launches the guide; existing content never auto-launches it | ✓ |
| Menu-triggered only | Always available from a menu, never opens itself | |
| Both, with one-time dismissal | Auto-launches once; if exited, not offered again automatically | |

**User's choice:** Auto-launch for empty/new show
**Notes:** Safe because the flow (per `.planning/sketches/references/onboarding-readiness-impact.md`, already locked and not re-discussed) is optional and Exit Guide is always available.

### Nav placement for show open/new/switch

| Option | Description | Selected |
|--------|-------------|----------|
| Existing Show group, new entry | Add a new destination alongside Overview and Save & Recovery | ✓ |
| Folded into Overview | Open/New/Switch actions live as buttons within Overview itself | |
| New top-level entry point | A dedicated launcher/welcome screen outside Show/Build/Operate/Output | |

**User's choice:** Existing Show group, new entry

### Guided First Show's own nav destination

| Option | Description | Selected |
|--------|-------------|----------|
| Overlay/flow, no dedicated nav entry | Reached via auto-launch or a "Start Guide" button on Overview; matches Sketch 004-B's HTML structure | ✓ |
| Dedicated nav destination | Its own entry in the command rail | |

**User's choice:** Overlay/flow, no dedicated nav entry

---

## Claude's Discretion

None — every gray area discussed converged on an explicit user selection; all recommended options were chosen.

## Deferred Ideas

None — discussion stayed within phase scope.
