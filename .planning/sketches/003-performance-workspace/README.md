---
sketch: 003
name: performance-workspace
question: "What constrained surface best supports reliable handoff and live operation?"
winner: "A"
tags: [perform, operator, scenes, masters, midi, safety]
---

# Sketch 003: Performance Workspace

## Design Question

What should a show author hand to another operator so scene changes, layer overrides, masters,
MIDI pickup, and safety actions remain obvious under live pressure?

## How to View

Open `.planning/sketches/003-performance-workspace/index.html` in a browser.

## Variants

- **A: Launcher + Masters** — random-access scene launcher with large scene cells, compact active
  layers, group masters, and a persistent live-state inspector. **Selected.**
- **B: Cue Stack** — ordered previous/current/next cue list with a dominant GO action, suitable for
  repeatable shows and operators who should follow a sequence.
- **C: Hardware Bank** — eight channel strips and eight launch pads arranged like a generic MIDI
  control bank, with visible pickup targets and bank navigation.

## What to Look For

- Can a substitute operator identify the active scene and next safe action immediately?
- Are assigned and locked controls distinguishable without hiding show scope?
- Is MIDI soft pickup understandable without reading documentation?
- Are live output, controlling source, timing, and safety continuously legible?
- Does the layout fit both mouse/keyboard operation and a small physical controller?

## Decision

Variant A was selected as the default constrained operator surface. Assigned scenes are large,
random-access launch targets; unassigned scenes remain visible and locked; the active scene's four
layers occupy a compact lower strip; group and grand masters remain adjacent; MIDI pickup direction
and target are shown inline; and the right live-state panel continuously exposes timing, source,
output health, surface identity, and controller connection.

The hardware-bank treatment from Variant C remains a useful specialized MIDI mapping/diagnostic
view, but is not the default handoff surface.
