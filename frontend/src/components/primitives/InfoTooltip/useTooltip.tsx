// useTooltip is the shared hover/focus tooltip mechanism behind
// InfoTooltip's "i" disclosure icon AND any other element that wants the
// same styled, portal-rendered tooltip attached directly to itself
// (CommandRail's own nav destination buttons, for one) -- so every hover
// affordance in the app renders through the same styled box instead of
// some using this and others falling back to the browser's own unstyled
// native `title` tooltip.
import { useId, useLayoutEffect, useRef, useState } from "react";
import type { CSSProperties, KeyboardEvent, ReactNode, RefObject } from "react";
import { createPortal } from "react-dom";
import { useNavTooltipsEnabled } from "../../../hooks/useNavTooltipsEnabled";
import styles from "./InfoTooltip.module.css";

const EDGE_MARGIN = 8;
// A floor on the computed max-width, below which text becomes illegible
// regardless of which side has "more" room -- e.g. a trigger sitting only
// a few px from BOTH edges (a narrow nav rail item) has neither side
// genuinely usable; better to render at a readable minimum and let the
// box slightly touch/pass the tighter edge than collapse to a few px wide
// with every word wrapping onto its own line (the exact bug this whole
// mechanism replaces).
const MIN_USABLE_WIDTH = 180;

interface TooltipPosition {
  top: number;
  /** Exactly one of left/right is set -- CSS `left`/`right` are mutually
   * exclusive, never both at once: this is a real anchor-side choice, not
   * just two equivalent ways to say the same position. */
  left: number | null;
  right: number | null;
  maxWidth: number;
  flip: boolean;
}

export interface UseTooltipResult<T extends HTMLElement> {
  /** Attach to the trigger element itself (the thing that shows the
   * tooltip on hover/focus) -- an "i" icon button for InfoTooltip, or a
   * nav button/any other control directly for every other call site. */
  triggerRef: RefObject<T | null>;
  /** Spread onto the trigger element: aria-describedby plus every
   * open/close event handler. */
  triggerProps: {
    "aria-describedby": string | undefined;
    onMouseEnter: () => void;
    onMouseLeave: () => void;
    onFocus: () => void;
    onBlur: () => void;
    onKeyDown: (event: KeyboardEvent) => void;
  };
  /** Render this anywhere in the trigger's own tree -- it's a portal, so
   * its DOM placement doesn't matter, only that it's rendered at all. */
  tooltipNode: ReactNode;
}

export interface UseTooltipOptions {
  /** True for tooltips attached directly to an already-labeled control the
   * operator brushes past during ordinary use (a nav destination button,
   * say) rather than deliberately hovering to ask "what is this" (an
   * InfoTooltip "i" icon, which has no other purpose and is never
   * suppressed) -- gated behind the header's nav-tooltips preference
   * (lib/navTooltips.ts) so an operator who finds constant hover text
   * annoying during normal navigation can turn just this class off. */
  suppressible?: boolean;
}

export function useTooltip<T extends HTMLElement = HTMLElement>(
  text: string,
  options?: UseTooltipOptions,
): UseTooltipResult<T> {
  const suppressible = options?.suppressible ?? false;
  // Always called (rules of hooks) -- cheap, and irrelevant when
  // suppressible is false since navTooltipsEnabled is never consulted below.
  const navTooltipsEnabled = useNavTooltipsEnabled();
  const [hovered, setHovered] = useState(false);
  const open = hovered && (!suppressible || navTooltipsEnabled);
  const triggerRef = useRef<T>(null);
  const [position, setPosition] = useState<TooltipPosition | null>(null);
  const tooltipId = useId();

  // A single, geometry-only pass -- no "render once, measure the tooltip's
  // own box, maybe re-render" two-pass dance. That approach shipped two
  // separate real bugs: (1) flipping via `left` + `transform:
  // translateX(-100%)` collapsed to the narrowest unbreakable word, since
  // a position:fixed box's shrink-to-fit width is computed from its anchor
  // to the viewport edge BEFORE any transform is applied; (2) even a
  // `right`-anchored flip could still push the box off the OPPOSITE edge
  // when the trigger sat close to both edges at once (a narrow nav rail
  // item), because deciding "flip or don't" from only one edge's overflow
  // never asked whether the other side actually had more room to begin
  // with. Computing both sides' available space up front, from the
  // trigger's own rect alone, and handing the browser a real numeric
  // max-width for whichever side wins sidesteps both bugs at once: the
  // box is told exactly how much room it has before it ever renders, so
  // it wraps within that budget instead of guessing and correcting after
  // the fact.
  useLayoutEffect(() => {
    if (!open || !triggerRef.current) {
      return;
    }
    const rect = triggerRef.current.getBoundingClientRect();
    const spaceRight = window.innerWidth - rect.right - EDGE_MARGIN;
    const spaceLeft = rect.left - EDGE_MARGIN;
    const flip = spaceLeft > spaceRight;
    const availableSpace = flip ? spaceLeft : spaceRight;
    setPosition({
      top: rect.top + rect.height / 2,
      left: flip ? null : rect.right,
      right: flip ? window.innerWidth - rect.left : null,
      maxWidth: Math.max(availableSpace, MIN_USABLE_WIDTH),
      flip,
    });
  }, [open]);

  const handleKeyDown = (event: KeyboardEvent) => {
    if (event.key === "Escape") {
      setHovered(false);
    }
  };

  const tooltipNode =
    open && position
      ? createPortal(
          <span
            role="tooltip"
            id={tooltipId}
            className={styles.tooltip}
            data-flip={position.flip || undefined}
            style={
              {
                "--ds-tooltip-top": `${position.top}px`,
                "--ds-tooltip-max-width": `${position.maxWidth}px`,
                ...(position.left !== null ? { "--ds-tooltip-left": `${position.left}px` } : {}),
                ...(position.right !== null ? { "--ds-tooltip-right": `${position.right}px` } : {}),
              } as CSSProperties
            }
          >
            {text}
          </span>,
          document.body,
        )
      : null;

  return {
    triggerRef,
    triggerProps: {
      "aria-describedby": open ? tooltipId : undefined,
      onMouseEnter: () => setHovered(true),
      onMouseLeave: () => setHovered(false),
      onFocus: () => setHovered(true),
      onBlur: () => setHovered(false),
      onKeyDown: handleKeyDown,
    },
    tooltipNode,
  };
}
