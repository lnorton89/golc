// HoverTooltip is the shared hover/focus tooltip mechanism behind
// InfoTooltip's "i" disclosure icon AND any other element that wants the
// same styled tooltip attached directly to itself (CommandRail's nav
// destination buttons, via NavDestinationButton.tsx) -- so every hover
// affordance in the app renders through the same styled box instead of
// some using this and others falling back to the browser's own unstyled
// native `title` tooltip.
//
// This replaces the hand-rolled useTooltip hook, which computed its own
// viewport geometry in a useLayoutEffect and portaled a position:fixed box
// with top/left/right/max-width set inline. That code existed because it
// had to solve viewport-edge collision by hand, and it did so only after
// shipping two separate real bugs (see this file's git history and the
// e2e spec): a flipped box anchored via `left` + translateX(-100%)
// collapsed to its narrowest unbreakable word, and a flip decision made
// from one edge's overflow alone could push a trigger that was near BOTH
// edges off-screen. Base UI's positioner solves both structurally with
// Floating UI's flip/shift middleware, so the geometry is no longer ours
// to get wrong.
//
// What is deliberately preserved from the old implementation:
//
//   - Opening toward the inline-end (right in LTR) and flipping to the
//     inline-start only when that side is genuinely roomier. `side` +
//     Base UI's default flip collision avoidance is exactly that rule,
//     now computed by Floating UI rather than by comparing spaceLeft and
//     spaceRight ourselves.
//
//   - The MIN_USABLE_WIDTH floor. A trigger sitting a few px from BOTH
//     viewport edges has no genuinely usable side; the old code chose to
//     render at a readable minimum and let the box slightly pass the
//     tighter edge rather than collapse to a few px wide with every word
//     on its own line. InfoTooltip.module.css keeps that as
//     `max-width: max(<floor>, var(--available-width))`.
//
//   - The ENTIRE accessible association. Base UI 1.7's tooltip parts set
//     no ARIA whatsoever -- TooltipPopup.js and TooltipTrigger.js contain
//     no aria-* attributes at all, and the popup gets neither a role nor
//     an id. Verified in a real browser, not just jsdom: before this was
//     wired by hand, hovering a trigger produced a popup with `id=""` and
//     left `aria-describedby` null on the trigger, so a screen-reader user
//     got an anonymous box with no link to the control it described. The
//     role, the id, and the open-scoped aria-describedby below are
//     therefore all load-bearing, not decoration -- they reproduce exactly
//     what the retired hook wired via useId().
//
//   - pointer-events: none on the popup, so the tooltip can never sit
//     between the operator and a control they are trying to click.
//
//   - `suppressible`, backed by the header's nav-tooltips preference
//     (lib/navTooltips.ts). True for tooltips attached to an
//     already-labeled control an operator brushes past during ordinary
//     navigation (a nav destination button); false for an InfoTooltip "i"
//     icon, which has no purpose other than being hovered and so is never
//     suppressed. Implemented via Base UI's own `disabled`, which stops
//     the tooltip opening at all rather than rendering it invisibly.
import { Tooltip as BaseTooltip } from "@base-ui/react/tooltip";
import { useId, useState } from "react";
import type { ReactElement } from "react";

import { useNavTooltipsEnabled } from "../../../hooks/useNavTooltipsEnabled";
import styles from "./InfoTooltip.module.css";

/** TOOLTIP_GAP_PX is both the offset from the trigger and the minimum gap
 * kept from a viewport edge -- the old implementation's single EDGE_MARGIN
 * constant, which served both roles. */
const TOOLTIP_GAP_PX = 8;

export interface HoverTooltipProps {
  /** The tooltip's text. */
  text: string;
  /** See the `suppressible` note in this file's doc comment. */
  suppressible?: boolean;
  /** The trigger element. Base UI merges its own trigger props (open/close
   * handlers, aria-describedby) into this element rather than wrapping it,
   * so the caller keeps full ownership of the tag, className, and ref. */
  children: ReactElement;
}

export default function HoverTooltip({ text, suppressible = false, children }: HoverTooltipProps) {
  // Always called (rules of hooks) -- cheap, and irrelevant when
  // suppressible is false, since the preference is not consulted below.
  const navTooltipsEnabled = useNavTooltipsEnabled();
  const disabled = suppressible && !navTooltipsEnabled;

  // Open state is tracked here only so aria-describedby can be scoped to
  // it. Pointing the trigger at an id that is not in the document while
  // closed would be a dangling reference; the retired hook scoped it the
  // same way. Base UI still owns every decision about WHEN to open --
  // this just mirrors what it decided.
  const [open, setOpen] = useState(false);
  const tooltipId = useId();

  return (
    // delay/closeDelay 0 preserve the old hook's behaviour exactly: it
    // opened on the mouseenter/focus event itself with no timer. A desk
    // operator scanning the rail should not have to hold still to find out
    // what a control does.
    //
    // The Provider lives here, per tooltip, rather than once at the app
    // root. Its only job is supplying that shared delay, and its grouping
    // behaviour ("adjacent tooltips open instantly once one is open") is
    // definitionally moot at zero delay -- so a single global instance
    // would buy nothing while making HoverTooltip depend on app-level
    // setup that every standalone component test would then have to
    // reproduce.
    <BaseTooltip.Provider delay={0} closeDelay={0}>
      <BaseTooltip.Root disabled={disabled} open={open} onOpenChange={setOpen}>
        <BaseTooltip.Trigger aria-describedby={open ? tooltipId : undefined} render={children} />
        <BaseTooltip.Portal>
          <BaseTooltip.Positioner
            side="inline-end"
            align="center"
            sideOffset={TOOLTIP_GAP_PX}
            collisionPadding={TOOLTIP_GAP_PX}
            // fallbackAxisSide: "none" constrains placement to the inline
            // axis, so this can only ever resolve to inline-end or
            // inline-start -- never top/bottom.
            //
            // Base UI's tooltip default is the opposite ('end': flip freely
            // to any axis), and it visibly changed behaviour here: a
            // CommandRail nav item's tooltip has long sentence text whose
            // max-content width exceeds the room beside the rail, so
            // Floating UI abandoned the inline axis and dropped the tooltip
            // BELOW the nav item. The hand-rolled implementation had no
            // such fallback -- it always opened beside the trigger and
            // capped max-width to the room actually available, which is the
            // behaviour the max-width rule in InfoTooltip.module.css still
            // provides. Caught by design-system.tooltip-overflow.spec.ts's
            // left-edge case.
            collisionAvoidance={{ fallbackAxisSide: "none" }}
            className={styles.positioner}
          >
            <BaseTooltip.Popup id={tooltipId} role="tooltip" className={styles.tooltip}>
              {text}
            </BaseTooltip.Popup>
          </BaseTooltip.Positioner>
        </BaseTooltip.Portal>
      </BaseTooltip.Root>
    </BaseTooltip.Provider>
  );
}
