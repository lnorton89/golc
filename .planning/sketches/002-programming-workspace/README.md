---
sketch: 002
name: programming-workspace
question: "How should scenes, four layers, reusable looks, fixture selection, and recording coexist?"
winner: "A"
tags: [program, scenes, layers, inspector, programmer]
---

# Sketch 002: Programming Workspace

## Design Question

Which programming model best supports both fast scene assembly and precise fixture-level work
without exposing every editor at once?

## How to View

Open `.planning/sketches/002-programming-workspace/index.html` in a browser.

## Variants

- **A: Scene Stack + Inspector** — scene-led workflow with a compact layer stack, bar timeline,
  reusable-look browser, and selection-sensitive inspector. **Selected.**
- **B: Scene × Layer Matrix** — spreadsheet/launcher overview with scenes as rows and the four layer
  kinds as columns; selection opens a detail editor below.
- **C: Programmer First** — lighting-console-oriented fixture/pool selection and attribute families
  in the center, with scene recording and update actions around the programmer.

## What to Look For

- Can a new author understand how a scene points to four reusable layer looks?
- Can an experienced operator get from fixture selection to a recorded look quickly?
- Is record scope always visible before committing changes?
- Is disabled-but-referenced layer state understandable?
- Can the same design scale to dozens of scenes and many fixture pools?

## Decision

Variant A was selected. Scene selection is the primary programming context. The selected scene
exposes exactly four compact layer rows; reusable looks remain browsable without becoming the main
canvas; timeline/evaluation feedback occupies a dedicated lower panel; and the right inspector
changes with the selected scene, layer, or look.

The fixture-level programmer shown in Variant C remains a useful future drill-down pattern, but it
is not the default programming workspace.
