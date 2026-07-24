---
sketch: 001
name: workspace-shell
question: "How should operators move between Patch, Program, Perform, and Setup without losing live context?"
winner: "D"
tags: [layout, navigation, chrome, workflow]
---

# Sketch 001: Workspace Shell

## Design Question

How should GOLC divide major tasks while keeping show identity, transport, live truth, connection
health, and emergency controls continuously available?

## How to View

Open `.planning/sketches/001-workspace-shell/index.html` in a browser.

## Variants

- **A: Task Rail + Inspector** — Bitwig-inspired left workspace rail, central job canvas,
  contextual right inspector, compact global header, fixed safety footer.
- **B: Author / Perform Dual View** — Ableton-inspired primary duality with Patch and Setup as
  supporting drawers; strongest separation between building and operating.
- **C: Command Deck** — Dense split console with a permanent navigator and live deck; fastest
  cross-domain visibility but highest information-density risk.
- **D: Focused Command Rail** — Synthesis of A's focused central workspace and contextual
  inspector with C's grouped Show / Build / Operate / Output navigation. **Selected.**

## Decision

Variant D was selected. It preserves a single focused workspace canvas and contextual inspector
while giving the left navigation a durable domain grouping:

- Show: overview, save, and recovery.
- Build: fixture library, patch/pools, scenes/looks.
- Operate: operator surfaces and MIDI mappings.
- Output: Art-Net and diagnostics.

At compact desktop widths, the selected scene's four layers use a 2×2 grid so the grouped
navigation and inspector do not force horizontal clipping.

## What to Look For

- Can you identify the current job and live state in under a second?
- Does changing workspaces feel safe while a show is running?
- Are emergency controls available without dominating the entire interface?
- Is the central canvas large enough for future patch grids, scene matrices, and operator surfaces?
- Does the inspector/panel model make routine editing feel discoverable?
