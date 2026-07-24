# Programming & Scene Authoring

## Design Decisions

- Use **Scene Stack + Inspector** from Sketch 002 Variant A.
- A scene list is the primary programming navigator.
- The selected scene exposes exactly four compact layer rows:
  Base Look, Color Theme, Chase, and Motion.
- Each row shows enabled/disabled state, referenced reusable look, and one meaningful summary value.
- Disabling a layer preserves and continues to display its reference.
- Reusable looks remain in a compact adjacent browser, not mixed into the scene list.
- A lower timeline/evaluation panel shows bar-relative layer behavior and the evaluated position.
- The right inspector edits the selected scene, layer, look, fixture selection, or impact item.
- Record scope and touched values are visible before recording or updating a look.

## CSS Patterns

```css
.program-layout {
  height: 100%;
  display: grid;
  grid-template:
    minmax(0, 1fr) 170px /
    205px minmax(0, 1fr) 190px;
  gap: 7px;
}

.layer-row {
  display: grid;
  grid-template-columns: 82px 1fr 62px 28px;
  align-items: center;
  min-height: 48px;
  border-left: 3px solid var(--color-primary);
}

.layer-row[data-enabled="false"] {
  border-left-color: var(--color-border-strong);
  opacity: .72;
}
```

## HTML Structures

```html
<section aria-label="Scenes">...</section>
<section aria-label="Selected scene layers">
  <button data-layer-kind="base-look">...</button>
  <button data-layer-kind="color-theme">...</button>
  <button data-layer-kind="chase">...</button>
  <button data-layer-kind="motion">...</button>
</section>
<aside aria-label="Reusable looks">...</aside>
<section aria-label="Bar timeline preview">...</section>
```

## Interaction Contract

- Scene selection changes layer rows, timeline, and inspector together.
- Selecting a layer changes only contextual detail; it does not toggle it.
- The enable switch is a separate, keyboard-addressable control.
- Evaluation at a bar position previews through shared Go commands.
- Updating a reusable look opens deterministic impact review when multiple scenes reference it.
- Fixture-level programming may drill into a programmer panel, but it is not the default workspace.

## What to Avoid

- A scene × layer spreadsheet as the only programming view; it gives overview but weak editing
  hierarchy.
- Making the fixture programmer the default landing surface for ordinary scene assembly.
- Permanently displaying every look-creation form.
- Hiding disabled layer references.
- Using scene color decoratively without a defined show-authoring color model.

## Origin

Synthesized from Sketch 002. Full source:
`sources/002-programming-workspace/index.html`.

