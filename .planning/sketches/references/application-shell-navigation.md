# Application Shell & Navigation

## Design Decisions

- Use the **Focused Command Rail** selected in Sketch 001 Variant D.
- Left navigation is grouped by stable user intent:
  - Show: Overview; Save & Recovery.
  - Build: Fixture Library; Patch & Pools; Scenes & Looks.
  - Operate: Operator Surface; MIDI Mapping.
  - Output: Art-Net; Diagnostics.
- The central canvas contains one workspace. A contextual inspector occupies the right edge.
- A compact top frame contains show identity/save state, transport, BPM, bar/beat position,
  controlling source, active scene, and output health.
- The bottom safety region remains fixed and contains Stop/Release-All, Revoke Automation, and
  Blackout. It is visually distinct without making the entire header red.
- At compact desktop widths, inspectors may collapse into a drawer, but the command rail, live
  truth, and safety actions remain available.

## CSS Patterns

```css
.app-shell {
  height: 100vh;
  display: grid;
  grid-template:
    52px minmax(0, 1fr) 48px /
    186px minmax(0, 1fr) 258px;
  overflow: hidden;
}

.workspace {
  min-width: 0;
  min-height: 0;
  display: grid;
  grid-template-rows: 42px minmax(0, 1fr);
}

.canvas {
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}
```

All scrolling occurs inside bounded panels, not on the application body.

## HTML Structures

```html
<div class="app-shell">
  <header class="global-frame">...</header>
  <nav class="command-rail">...</nav>
  <main class="workspace">...</main>
  <aside class="contextual-inspector">...</aside>
  <footer class="safety-cluster">...</footer>
</div>
```

## Interaction Contract

- Selecting a command-rail destination replaces the central workspace and inspector.
- Navigation does not mutate show playback or output.
- Keyboard focus cycles global frame → navigation → canvas → inspector.
- Active navigation uses Signal Blue plus a structural indicator such as an inset border.
- Save/offline/live states use text plus shape or status indicators, never color alone.

## What to Avoid

- Icon-only primary navigation; it obscured the product's domain organization.
- A permanent live-deck sidebar on every authoring screen; it reduced canvas width excessively.
- Mounting every feature region in one page-length column.
- Repeating daemon-unreachable banners inside every feature panel.
- Treating backend service/package names as the information architecture.

## Origin

Synthesized from Sketch 001. Full source:
`sources/001-workspace-shell/index.html`.

