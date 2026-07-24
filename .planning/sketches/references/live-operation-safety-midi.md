# Live Operation, Safety & MIDI

## Design Decisions

- Use **Launcher + Masters** from Sketch 003 Variant A as the default operator surface.
- Assigned scenes render as large random-access pads with name, layer count, loop length, and
  keyboard mapping.
- Unassigned scenes remain visible, reduced in emphasis, labeled Locked, and disabled.
- The active scene uses Signal Blue plus explicit LIVE text.
- The active scene's four layers remain visible in a compact lower strip.
- Group masters and Grand Master are adjacent to the launcher.
- The live-state panel continuously shows active scene, bar/beat, source, output health, selected
  surface, and MIDI connection.
- Soft pickup shows:
  - Blue fill/handle following the physical fader.
  - A fixed gold target marker for the live application value.
  - Plain-language movement direction.
- The hardware-bank design remains available for MIDI mapping and diagnostics, not as the default
  performance screen.

## CSS Patterns

```css
.scene-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(120px, 1fr));
  gap: 6px;
}

.scene-pad[aria-current="true"] {
  background: rgba(27, 68, 217, .32);
  border-color: var(--color-primary);
  box-shadow: inset 0 3px var(--color-primary);
}

.scene-pad[aria-disabled="true"] {
  opacity: .38;
  filter: saturate(.35);
}

.pickup-target {
  position: absolute;
  width: 2px;
  background: var(--color-armed);
}
```

## Interaction Contract

- Scene pads launch through shared commands; the UI waits for projected state to mark a scene live.
- Locked pads remain focusable only if needed to explain why they are locked; they never dispatch.
- Buttons act immediately on MIDI Note input; continuous controls require cross-to-catch.
- MIDI learn is per assigned control and per operator surface.
- Mapping conflicts are rejected until the old mapping is explicitly removed.
- Blackout, Revoke Automation, and Stop/Release-All remain independent local-priority actions.
- Hold-to-confirm gives immediate press progress and cancels cleanly on pointer/key release.

## What to Avoid

- Hiding unassigned show scope entirely.
- A cue-stack-only performance model; ordered cues may be an optional surface later.
- Requiring MIDI hardware to complete normal playback.
- Showing hardware position without the live pickup target.
- Treating a red global header as a permanent safety signal.
- Reporting failed MIDI safety/master dispatch only in server logs; implementation should provide
  an operator-visible failure indication.

## Origin

Synthesized from Sketch 003. Full source:
`sources/003-performance-workspace/index.html`.

