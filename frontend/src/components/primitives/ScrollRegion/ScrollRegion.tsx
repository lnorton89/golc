// ScrollRegion is the concrete implementation of application-shell-
// navigation.md's hard rule: "All scrolling occurs inside bounded panels,
// not on the application body." Every list in every workspace should sit
// inside one of these rather than letting its parent grow unbounded.
import type { HTMLAttributes, ReactNode } from "react";

import styles from "./ScrollRegion.module.css";

interface ScrollRegionProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
}

export default function ScrollRegion({ children, className, ...rest }: ScrollRegionProps) {
  const combinedClassName = className ? `${styles.region} ${className}` : styles.region;
  return (
    <div className={combinedClassName} {...rest}>
      {children}
    </div>
  );
}
