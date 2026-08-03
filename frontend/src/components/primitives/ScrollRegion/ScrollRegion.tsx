// ScrollRegion is the concrete implementation of application-shell-
// navigation.md's hard rule: "All scrolling occurs inside bounded panels,
// not on the application body." Every list in every workspace should sit
// inside one of these rather than letting its parent grow unbounded.
import type { HTMLAttributes, ReactNode } from "react";

import styles from "./ScrollRegion.module.css";

type ScrollDirection = "vertical" | "horizontal" | "both";

interface ScrollRegionProps extends Omit<HTMLAttributes<HTMLDivElement>, "role"> {
  children: ReactNode;
  direction?: ScrollDirection;
}

export default function ScrollRegion({ children, className, direction = "vertical", "aria-label": ariaLabel, "aria-labelledby": ariaLabelledBy, tabIndex, ...rest }: ScrollRegionProps) {
  const combinedClassName = [styles.region, styles[direction], className].filter(Boolean).join(" ");
  const isNamed = Boolean(ariaLabel || ariaLabelledBy);
  return (
    <div
      {...rest}
      aria-label={ariaLabel}
      aria-labelledby={ariaLabelledBy}
      className={combinedClassName}
      data-scroll-direction={direction}
      role={isNamed ? "region" : undefined}
      tabIndex={tabIndex ?? (isNamed ? 0 : undefined)}
    >
      {children}
    </div>
  );
}
