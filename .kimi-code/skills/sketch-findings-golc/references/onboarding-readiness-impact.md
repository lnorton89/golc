# Onboarding, Readiness & Impact Review

## Design Decisions

- Use **Guided First Show** from Sketch 004 Variant B.
- Guidance is optional, saves progress, and always offers Exit Guide into direct navigation.
- Stages are Fixtures, Patch, Program, Assign, and Verify.
- Each stage has one dominant next action, concise explanation, visible evidence, and live preview.
- Patch changes remain preview-only until the user reviews and applies the deterministic impact
  plan.
- Blockers, warnings, and optional evidence are separate statuses.
- “Ready to perform” is based on explicit gates and evidence, not a generic completion percentage.
- Workflow-rail and readiness-dashboard patterns may support the normal Overview and release
  qualification, but they do not replace the selected guided flow.

## CSS Patterns

```css
.guided-flow {
  height: 100%;
  display: grid;
  grid-template-columns: 210px minmax(0, 1fr);
  gap: 7px;
}

.guide-step[aria-current="step"] {
  border-left: 3px solid var(--color-primary);
  background: var(--color-primary-soft);
}

.impact-preview {
  border: 1px solid rgba(200, 162, 75, .5);
  background: rgba(200, 162, 75, .08);
}
```

## HTML Structures

```html
<nav aria-label="First show steps">...</nav>
<section aria-labelledby="current-step-title">
  <header>...</header>
  <div class="choices-and-evidence">...</div>
  <footer>
    <button>Back</button>
    <button>Exit Guide</button>
    <button class="primary">Review Patch & Continue</button>
  </footer>
</section>
<aside aria-label="Live preview and evidence">...</aside>
```

## Interaction Contract

- The guide reads actual domain readiness; it does not own duplicate state.
- Users may visit stages non-linearly through normal navigation.
- Exiting retains completed evidence and current stage.
- A blocker prevents the transition to Perform but not editing in other workspaces.
- Warnings require acknowledgment or evidence where appropriate but do not masquerade as blockers.
- Physical MIDI evidence is optional for on-screen operation and required only for named hardware
  compatibility claims.
- Impact plans show affected groups, themes, palettes, scenes, chases, motion presets, and mappings.

## What to Avoid

- A mandatory wizard that hides normal navigation.
- Progress percentages with no explanation of unmet evidence.
- Applying structural changes when the user clicks Continue.
- Treating MIDI hardware qualification as a blocker for keyboard/on-screen playback.
- Mixing release qualification evidence into everyday error banners.

## Origin

Synthesized from Sketch 004. Full source:
`sources/004-patch-to-play-flow/index.html`.

