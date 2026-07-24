---
sketch: 004
name: patch-to-play-flow
question: "How should readiness and progression from fixture patch to live output be communicated?"
winner: "B"
tags: [patch, onboarding, readiness, flow, overview]
---

# Sketch 004: Patch-to-Play Flow

## Design Question

How should GOLC help an author move from fixture definitions to a handoff-ready live show while
preserving expert freedom, deterministic impact review, and explicit blockers?

## How to View

Open `.planning/sketches/004-patch-to-play-flow/index.html` in a browser.

## Variants

- **A: Workflow Rail** — persistent, non-modal readiness strip on the Overview and relevant
  workspaces; incomplete steps link directly to their focused workspace.
- **B: Guided First Show** — optional step-by-step project setup with one primary action, evidence,
  and a preview at each stage. **Selected.**
- **C: Readiness Dashboard** — status cards, blockers, warnings, physical-verification evidence,
  and launch gates optimized for experienced operators.

## What to Look For

- Is the next useful action obvious without preventing non-linear work?
- Are blockers distinguishable from warnings and optional enhancements?
- Does deterministic impact review appear at the correct point?
- Can an experienced author skip guidance and navigate directly?
- Is “ready to perform” based on explicit evidence rather than a vague progress percentage?

## Decision

Variant B was selected. GOLC should offer an optional Guided First Show flow that:

- Presents Fixtures, Patch, Program, Assign, and Verify as explicit stages.
- Gives each stage one dominant next action plus live preview/evidence.
- Keeps deterministic impact plans in preview until separately applied.
- Saves progress and allows the user to exit into direct workspace navigation at any time.
- Distinguishes blockers, warnings, and optional physical evidence.

The workflow rail and evidence dashboard remain useful supporting patterns for the normal Overview
and qualification/reporting surfaces, but they are not the primary first-show experience.
