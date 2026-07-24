# GOLC UI Sketch Manifest

## Design Direction

GOLC should feel like a focused live-production instrument: dense without being cramped, calm
while idle, unmistakable when state changes, and fast enough to operate from muscle memory. The
application is divided by operator intent rather than backend subsystem. Patch, Program, Perform,
and Setup are persistent workspaces around a shared transport/status frame. Signal Blue indicates
selection and live/manual control; other color is reserved for semantic state, warnings, and
safety. The visual language borrows the task-oriented views, contextual inspectors, compact
panels, and uninterrupted transport of Ableton Live and Bitwig Studio without imitating either
product's branding.

## Reference Points

- Ableton Live 12: separate Session and Arrangement views, uninterrupted transport, compact
  persistent control/status bars, keyboard-first navigation.
- Bitwig Studio: task-curated Arrange/Mix/Edit views, contextual inspector panels, optional access
  panels, display profiles.
- GOLC brand system: warm Paper/Ink palette, Signal Blue accent, Archivo + JetBrains Mono,
  semantic live/frame-lock/armed/revoked/blackout/offline colors.
- Existing GOLC domain: fixture pools and deployments, scene layers and reusable looks, operator
  surfaces, MIDI learn, Art-Net status, independent safety actions.

## Sketches

| # | Name | Design Question | Winner | Tags |
|---|------|-----------------|--------|------|
| 001 | workspace-shell | How should operators move between Patch, Program, Perform, and Setup without losing live context? | D — Focused Command Rail | layout, navigation, chrome, workflow |
| 002 | programming-workspace | How should scenes, layers, reusable looks, selection, and editing coexist? | A — Scene Stack + Inspector | program, scenes, inspector |
| 003 | performance-workspace | What constrained surface best supports reliable handoff and live operation? | pending | perform, operator, midi |
| 004 | patch-to-play-flow | How should readiness and progression from fixture patch to live output be communicated? | pending | patch, onboarding, flow |
