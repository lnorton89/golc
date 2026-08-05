// Small hover/focus "i" affordance placed beside a heading or nav item.
// Always a sibling of the interactive/heading element it annotates, never
// nested inside it -- CommandRail's destination buttons and Toolbar's own
// <h2> are both matched by accessible name in tests and by screen
// readers, and a nested control would fold this tooltip's text into that
// name.
//
// Rendered through a portal into document.body rather than CSS-only
// group-hover: several call sites (CommandRail.module.css's own .rail,
// Toolbar's .action row wrapping) sit inside a container with
// overflow-y: auto (browsers compute the paired overflow-x as auto too),
// which clips any absolutely positioned descendant the instant it
// crosses that container's own bounds. A portal escapes the clipped
// subtree entirely; its position is computed from the trigger's real
// on-screen rect instead.
import { useId, useLayoutEffect, useRef, useState } from "react";
import type { CSSProperties, KeyboardEvent } from "react";
import { createPortal } from "react-dom";
import styles from "./InfoTooltip.module.css";

interface InfoTooltipProps {
  label: string;
  text: string;
}

interface TooltipPosition {
  top: number;
  /** Exactly one of left/right is set -- CSS `left`/`right` are mutually
   * exclusive here, never both at once (see the flip comment below for why
   * that matters, not just for rendering the same value twice). */
  left: number | null;
  right: number | null;
  flip: boolean;
}

export default function InfoTooltip({ label, text }: InfoTooltipProps) {
  const [open, setOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const tooltipRef = useRef<HTMLSpanElement>(null);
  const [position, setPosition] = useState<TooltipPosition | null>(null);
  const tooltipId = useId();

  useLayoutEffect(() => {
    if (!open || !triggerRef.current) {
      return;
    }
    const rect = triggerRef.current.getBoundingClientRect();
    setPosition({
      top: rect.top + rect.height / 2,
      left: rect.right,
      right: null,
      flip: false,
    });
  }, [open]);

  // The CSS clamp() on .tooltip only bounds the anchor point itself, not
  // the box's actual rendered extent -- a wide tooltip anchored near the
  // right edge still overflows past 100vw, since the clamp has no way to
  // know the box's own width at CSS-authored time. This second pass runs
  // after the tooltip has actually mounted and can be measured for real:
  // if opening rightward (the default) would overflow, flip to open
  // leftward instead.
  //
  // The flip re-anchors via CSS `right` (distance from the viewport's
  // right edge) rather than keeping `left` and adding a horizontal
  // `transform: translateX(-100%)` -- a `position: fixed` box with only
  // `left` set (no `right`) always has its shrink-to-fit width computed
  // from "left to the viewport's right edge", regardless of any later
  // transform: the transform only repositions the box AFTER that width is
  // already decided, it cannot widen it back out. Near the right edge that
  // left-to-viewport-edge gap is tiny (a few px), so the box collapsed to
  // whatever the narrowest unbreakable word needed and every other word
  // wrapped onto its own line -- exactly the malformed, "every word on its
  // own line" rendering this replaces. Anchoring via `right` instead makes
  // the browser compute shrink-to-fit width from "the viewport's left edge
  // to right", which is the actually-available leftward space.
  useLayoutEffect(() => {
    if (!open || !position || position.flip || !tooltipRef.current || !triggerRef.current) {
      return;
    }
    const tooltipRect = tooltipRef.current.getBoundingClientRect();
    const margin = 8;
    if (tooltipRect.right > window.innerWidth - margin) {
      const triggerRect = triggerRef.current.getBoundingClientRect();
      setPosition({ top: position.top, left: null, right: window.innerWidth - triggerRect.left, flip: true });
    }
  }, [open, position]);

  const handleKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (event.key === "Escape") {
      setOpen(false);
    }
  };

  return (
    <span className={styles.wrapper}>
      <button
        ref={triggerRef}
        type="button"
        aria-label={label}
        aria-describedby={open ? tooltipId : undefined}
        className={styles.trigger}
        onMouseEnter={() => setOpen(true)}
        onMouseLeave={() => setOpen(false)}
        onFocus={() => setOpen(true)}
        onBlur={() => setOpen(false)}
        onKeyDown={handleKeyDown}
      >
        i
      </button>
      {open && position
        ? createPortal(
            <span
              ref={tooltipRef}
              role="tooltip"
              id={tooltipId}
              className={styles.tooltip}
              data-flip={position.flip || undefined}
              style={
                {
                  "--ds-tooltip-top": `${position.top}px`,
                  ...(position.left !== null ? { "--ds-tooltip-left": `${position.left}px` } : {}),
                  ...(position.right !== null ? { "--ds-tooltip-right": `${position.right}px` } : {}),
                } as CSSProperties
              }
            >
              {text}
            </span>,
            document.body,
          )
        : null}
    </span>
  );
}
