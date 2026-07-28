# Phase 6: Wails Authoring and Operator Surface - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-23
**Phase:** 6-Wails Authoring and Operator Surface
**Areas discussed:** Operator surface builder, MIDI learn & conflicts, Soft takeover feedback, Safety control placement

---

## Operator Surface Builder

**Clarification requested:** The user first asked what "constrained operator surface" meant. Answered: a stripped-down playback view a show author hands to a less-experienced operator, containing only assigned scenes/layers/masters/safety controls, while the operator always sees active scene, layers, BPM/bar position, controlling source, and final output state (PLAY-03/07).

| Option | Description | Selected |
|--------|-------------|----------|
| Assign from full view | Toggle "add to this operator surface" on items directly from the existing authoring view; no new screen | ✓ |
| Dedicated builder screen | Separate screen listing every assignable item with explicit add/remove per named surface | |
| Drag-and-drop canvas | Author spatially arranges tiles/faders on a canvas mimicking the final layout | |

**User's choice:** Assign from full view

| Option | Description | Selected |
|--------|-------------|----------|
| One per show | Single constrained surface per show | |
| Multiple named surfaces | A show can define several named surfaces, switchable | ✓ |

**User's choice:** Multiple named surfaces

| Option | Description | Selected |
|--------|-------------|----------|
| Individual items only | Author picks specific scenes/layers/masters one at a time | ✓ |
| Items plus groups | Author can assign whole categories at once with individual overrides | |

**User's choice:** Individual items only

| Option | Description | Selected |
|--------|-------------|----------|
| Fully hidden | Unassigned items don't appear at all | |
| Visible but locked | Shown grayed out/disabled, visible but not interactable | ✓ |

**User's choice:** Visible but locked

**Notes:** No further questions — moved to next area.

---

## MIDI Learn & Conflicts

| Option | Description | Selected |
|--------|-------------|----------|
| Per-control learn button | Each mappable control has its own "Learn" affordance | ✓ |
| Global learn mode | Toggle a mode, then click each on-screen control in turn | |
| MIDI activity monitor | Panel lists incoming messages; user assigns from the list | |

**User's choice:** Per-control learn button

| Option | Description | Selected |
|--------|-------------|----------|
| Warn and confirm | Show a reassign confirmation dialog | |
| Block until unmapped | Reject the new mapping until the old one is explicitly removed | ✓ |
| Silent overwrite | New mapping replaces the old one immediately | |

**User's choice:** Block until unmapped

| Option | Description | Selected |
|--------|-------------|----------|
| Per operator surface | Each named surface has its own independent MIDI mapping | ✓ |
| Global to the show | One mapping applies everywhere regardless of active surface | |

**User's choice:** Per operator surface

| Option | Description | Selected |
|--------|-------------|----------|
| Whatever's on the surface | Any control assigned to a named surface is automatically MIDI-learnable | ✓ |
| Fixed safe list | Only a defined set of playback primitives are ever MIDI-mappable | |

**User's choice:** Whatever's on the surface

**Notes:** No further questions — moved to next area.

---

## Soft Takeover Feedback

| Option | Description | Selected |
|--------|-------------|----------|
| Ghost/shadow marker | A second faint marker shows the physical fader's live position alongside the app-value marker | ✓ |
| Locked/pulsing state | Fader shows a pulsing border/label until caught up, no raw position exposed | |
| Numeric mismatch readout | Text shows physical vs. app value until they match | |

**User's choice:** Ghost/shadow marker

| Option | Description | Selected |
|--------|-------------|----------|
| Cross to catch (pickup) | Physical value must cross/pass through the app's current value | ✓ |
| Proximity threshold | Physical takes over once within a small tolerance, without crossing | |

**User's choice:** Cross to catch (pickup)

| Option | Description | Selected |
|--------|-------------|----------|
| Stays at app value | Visible slider stays at the true app value; physical movement ignored until catch-up | |
| Follows physical live | Slider tracks the physical fader's raw position live, shown in a distinct "not armed" state | ✓ |

**User's choice:** Follows physical live

**Notes:** Combined with the earlier "ghost/shadow marker" pick, this resolves to: the visible slider tracks the physical fader's live position, and a separate ghost/target marker shows the fixed app value it must cross — captured together as D-09/D-10 in CONTEXT.md.

| Option | Description | Selected |
|--------|-------------|----------|
| Faders/CC only | Soft takeover is fader-specific; Note/button controls act immediately | ✓ |
| All mapped controls | Apply takeover-style debounce even to buttons | |

**User's choice:** Faders/CC only

**Notes:** No further questions — moved to next area.

---

## Safety Control Placement

| Option | Description | Selected |
|--------|-------------|----------|
| Global bar, every screen | Persistent strip on every screen, not just playback | ✓ |
| Playback/operator view only | Visible only in the live playback or operator-surface view | |

**User's choice:** Global bar, every screen

| Option | Description | Selected |
|--------|-------------|----------|
| Hold-to-confirm | Press and hold ~500ms-1s before it fires | ✓ |
| Single immediate click | No confirmation step at all | |
| Two-step arm then confirm | First click arms, second click within a short window executes | |

**User's choice:** Hold-to-confirm

| Option | Description | Selected |
|--------|-------------|----------|
| Dedicated safety cluster | Grouped together in one visually distinct area, fixed screen position | ✓ |
| Placed near relevant context | Each control sits near what it affects | |

**User's choice:** Dedicated safety cluster

| Option | Description | Selected |
|--------|-------------|----------|
| Fixed, global, unmodifiable | Default keybinding works anywhere, cannot be rebound | ✓ |
| Global but customizable | Same reachability, but user can rebind | |

**User's choice:** Fixed, global, unmodifiable

**Notes:** No further questions — this was the last area. User confirmed ready for context after a final "explore more gray areas vs. ready for context" check.

---

## Claude's Discretion

None — every gray area discussed converged on an explicit user selection.

## Deferred Ideas

None — discussion stayed within phase scope.
