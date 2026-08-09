// ScrollRegion is the concrete implementation of application-shell-
// navigation.md's hard rule: "All scrolling occurs inside bounded panels,
// not on the application body." Every list in every workspace should sit
// inside one of these rather than letting its parent grow unbounded.
//
// Built on Base UI's ScrollArea (@base-ui/react/scroll-area), which always
// draws its own custom scrollbar Thumb/Track -- there is no "native
// scrollbar" mode.
//
// The old component was a single div that was simultaneously (a) the flex
// item its parent's layout sizes/caps (flex/min-height/max-height) and
// (b) the actual scrolling box a consumer pads/arranges its own children
// within (padding, display:flex+gap, centering). Base UI's Root (the flex
// item that participates in the surrounding layout) and Viewport (the
// element that actually scrolls) are two different nodes, so a single
// incoming `className` cannot mechanically land on "the right one
// property-by-property" -- e.g. AppLogPanel's `.stream` needs its
// max-height on Root (or the region ends up empty dead space below a
// capped Viewport), while KeyboardShortcuts' `.scrollArea` needs its
// `display:flex;flex-direction:column;gap` on Viewport (to arrange the
// children actually passed to ScrollRegion) -- and some consumer classes
// (`.inspectBody`, `.scrollArea`) mix both in one class. The className is
// therefore applied to *both* Root and Viewport: sizing properties
// (flex/min/max-*) are redundant-but-harmless when duplicated, and
// content-arrangement properties (display/gap/align-items) only do
// anything on Viewport (Root has a single child, so flex arrangement
// there is inert). The one property that genuinely doubles up harmfully
// is `padding` (an inset on both nested boxes stacks instead of
// cancelling out), so Root's padding is force-zeroed via inline style
// (which always wins over an external class) -- Viewport still renders
// the real padding, so the visual inset is unchanged, just no longer
// doubled.
import { ScrollArea } from "@base-ui/react/scroll-area";
import type { HTMLAttributes, ReactNode } from "react";

import styles from "./ScrollRegion.module.css";

type ScrollDirection = "vertical" | "horizontal" | "both";

interface ScrollRegionProps extends Omit<HTMLAttributes<HTMLDivElement>, "role"> {
  children: ReactNode;
  direction?: ScrollDirection;
}

export default function ScrollRegion({ children, className, direction = "vertical", "aria-label": ariaLabel, "aria-labelledby": ariaLabelledBy, tabIndex, ...rest }: ScrollRegionProps) {
  const rootClassName = [styles.root, className].filter(Boolean).join(" ");
  const viewportClassName = [styles.viewport, className].filter(Boolean).join(" ");
  const isNamed = Boolean(ariaLabel || ariaLabelledBy);

  // Base UI's Viewport sets its own `overflow: scroll` inline (both axes,
  // always -- there's no prop to restrict it to one axis). Restricting
  // scrolling to a single axis therefore has to win the same inline-style
  // specificity battle: a longhand `overflow-x`/`overflow-y` declared
  // after the shorthand in the same style attribute wins for that axis.
  // Base UI merges a consumer-supplied `style` prop in after its own, so
  // this reliably overrides it (verified via a throwaway DOM dump, not
  // assumed from docs).
  const overflowOverride =
    direction === "vertical" ? { overflowX: "hidden" as const } : direction === "horizontal" ? { overflowY: "hidden" as const } : undefined;

  return (
    <ScrollArea.Root className={rootClassName} style={{ padding: 0 }}>
      <ScrollArea.Viewport
        {...rest}
        aria-label={ariaLabel}
        aria-labelledby={ariaLabelledBy}
        className={viewportClassName}
        data-scroll-direction={direction}
        role={isNamed ? "region" : undefined}
        style={overflowOverride}
        tabIndex={tabIndex ?? (isNamed ? 0 : undefined)}
      >
        {children}
      </ScrollArea.Viewport>
      {direction !== "horizontal" && (
        <ScrollArea.Scrollbar className={styles.scrollbar} orientation="vertical">
          <ScrollArea.Thumb className={styles.thumb} />
        </ScrollArea.Scrollbar>
      )}
      {direction !== "vertical" && (
        <ScrollArea.Scrollbar className={styles.scrollbar} orientation="horizontal">
          <ScrollArea.Thumb className={styles.thumb} />
        </ScrollArea.Scrollbar>
      )}
      {direction === "both" && <ScrollArea.Corner className={styles.corner} />}
    </ScrollArea.Root>
  );
}
