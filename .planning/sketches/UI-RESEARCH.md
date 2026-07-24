# GOLC UI Research

**Date:** 2026-07-23  
**Scope:** Information architecture and interaction patterns for the post-implementation Phase 6
UI remediation.

## Executive Finding

The current interface is not primarily suffering from missing polish. It lacks a task model.
`frontend/src/App.tsx` permanently mounts playback, operator surfaces, fixture patching, Art-Net
configuration, scene programming, and MIDI mapping in a single vertical document. The Phase 6
UI-SPEC defines tokens, copy, and component states but does not define primary navigation,
workspace ownership, contextual selection, panel hierarchy, or a patch-to-play workflow.

GOLC should use four task workspaces:

1. **Patch** — fixtures, pools, deployments, addresses, and deterministic impact review.
2. **Program** — scenes, four layer kinds, reusable looks, selection, and record scope.
3. **Perform** — scene launch, layer overrides, group/grand masters, constrained operator surface,
   MIDI pickup, and immediate live feedback.
4. **Setup** — Art-Net, MIDI devices/mappings, show settings, diagnostics, and application settings.

Transport, controlling source, output health, active scene, BPM/bar position, Blackout, Revoke
Automation, and Stop/Release-All remain visible across workspaces.

## Evidence From the Existing Application

- `App.tsx` has one `main` flex column and mounts all six large feature regions concurrently.
- Feature panels independently repeat headings, create rows, empty states, and scrolling lists.
- The default shell uses 32px gaps around panels intended for dense operational use.
- Offline diagnostics are repeated in the live status bar, operator surface, and Art-Net panel.
- Setup-only controls compete visually with playback controls.
- Scene programming mixes scene creation, look creation, chase configuration, preset recording,
  and blend creation in one panel rather than following the current selection.
- The UI-SPEC names Radix UI and Lucide, but `frontend/package.json` contains neither dependency.
  Current components are custom React + CSS Modules.
- The Phase 6 UAT explicitly did not assess pixel-level aesthetics, hierarchy, or workflow speed.

## Reference Pattern: Ableton Live

Useful patterns:

- Session View and Arrangement View serve different mental models while transport continues.
- A compact persistent control bar carries global time and playback state.
- Selection drives a detail area instead of every editor being open at once.
- Launchable content is spatial, predictable, and keyboard/MIDI addressable.
- The same project state is presented through alternate task views; changing views does not alter
  what is playing.

GOLC translation:

- Program and Perform are alternate views over the same show, not separate authorities.
- A scene/layer launcher belongs in Perform; deep editing belongs in Program.
- Workspace changes must never interrupt playback or output.

## Reference Pattern: Bitwig Studio

Useful patterns:

- Arrange, Mix, and Edit are curated panel compositions for particular jobs.
- Contextual Inspector panels expose properties for the current selection.
- Optional access panels hold browsers, project data, and mappings without taking over the canvas.
- Panels may be resized or hidden while the central job remains obvious.
- Display profiles acknowledge that a live tool may be used on different monitor arrangements.

GOLC translation:

- Patch, Program, Perform, and Setup each choose a central canvas and supporting panels.
- One contextual inspector prevents repeated inline forms and supports progressive disclosure.
- A future display-profile model can add a dedicated operator monitor without changing domain
  commands.

## Design Principles

1. **One primary job per workspace.** Setup does not share the Perform canvas.
2. **Selection reveals detail.** Do not render every form for every entity simultaneously.
3. **Live truth is persistent.** Active scene, output, source, timing, and safety survive navigation.
4. **Color carries state.** Signal Blue is selection/live; red is not decorative.
5. **Density comes from hierarchy.** Smaller spacing alone will not fix the current UI.
6. **Keyboard paths mirror visual paths.** Workspace, panel, list, and inspector focus are explicit.
7. **Author and operator views share commands.** They differ in affordance and authorization only.
8. **Offline and degraded state are consolidated.** One global connection indicator links to detail.
9. **Risky edits have a review surface.** Pool and fixture changes open a deterministic impact plan.
10. **The frontend projects state.** No proposed interaction makes React authoritative over timing.

## Proposed Information Architecture

```text
Global frame
├── Show identity / save state
├── Transport: play, BPM, tap, bar.beat
├── Live truth: scene, source, output health
├── Workspace switcher: Patch / Program / Perform / Setup
└── Safety cluster: Blackout / Revoke Automation / Stop-Release-All

Patch
├── Fixture/pool browser
├── Deployment patch canvas
└── Selection inspector + impact review drawer

Program
├── Scene/layer navigator
├── Programming canvas
├── Reusable-look browser
└── Selection/record-scope inspector

Perform
├── Scene launcher
├── Active-layer controls
├── Group + grand masters
└── Operator surface / MIDI pickup feedback

Setup
├── Art-Net
├── MIDI
├── Show/application settings
└── Diagnostics
```

## Anti-Patterns to Avoid

- A scrolling dashboard containing every capability.
- Navigation organized by Go package or service name.
- Multiple competing offline/error banners.
- Permanent create forms above every list.
- Hidden playback state while editing patch or settings.
- A red-filled global header that visually implies constant emergency.
- Modal dialogs for routine selection changes.
- Colorful scene tiles without a semantic show-authoring color model.

## Implementation Implications

- Refactor `App.tsx` into a shell/router before restyling individual panels.
- Preserve current Wails services and shared commands; move components rather than rewriting domain
  operations.
- Introduce a common panel, toolbar, list-row, chip, field, button, and inspector vocabulary.
- Radix should be evaluated only for behavior-heavy primitives (dialogs, popovers, menus, tabs);
  installing it is an implementation decision, not a visual direction.
- Add visual regression fixtures with realistic populated shows, not only empty/offline states.

## Sources

- Ableton Live 12 Manual: Session View,
  https://www.ableton.com/en/live-manual/12/session-view/
- Ableton Live 12 Manual: Live Concepts,
  https://www.ableton.com/en/live-manual/12/live-concepts/
- Ableton Live 12 Manual: Keyboard Shortcuts,
  https://www.ableton.com/en/manual/live-keyboard-shortcuts/
- Bitwig Studio User Guide: A Musical Swiss Army Knife,
  https://www.bitwig.com/userguide/latest/a_musical_swiss_army_knife/
- Bitwig Studio User Guide: The Edit View,
  https://www.bitwig.com/userguide/latest/the_edit_view/
- Bitwig Studio User Guide: The Clip Launcher,
  https://www.bitwig.com/userguide/latest/the_clip_launcher/

