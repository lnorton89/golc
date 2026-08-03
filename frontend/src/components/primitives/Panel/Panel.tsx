// Panel is the shared surface primitive for every workspace canvas region,
// inspector section, and stub screen (shell restructure Step 2). It is a
// pure CSS-Modules wrapper -- no Wails calls, no state -- so existing
// feature components (FixturePatch, ArtnetConfig, etc.) can adopt it by
// swapping their own ad hoc `<section className={styles.panel}>` for this
// shared one without changing any behavior.
import { forwardRef } from "react";
import type { HTMLAttributes, ReactNode } from "react";

import styles from "./Panel.module.css";

export type PanelVariant = "default" | "subdued" | "selected" | "warning" | "error";
export type PanelDensity = "default" | "compact";

export interface PanelProps extends HTMLAttributes<HTMLElement> {
  children: ReactNode;
  variant?: PanelVariant;
  density?: PanelDensity;
}

const Panel = forwardRef<HTMLElement, PanelProps>(function Panel(
  { children, className, variant = "default", density = "default", ...rest },
  ref,
) {
  const combinedClassName = className ? `${styles.panel} ${className}` : styles.panel;
  return (
    <section ref={ref} className={combinedClassName} data-variant={variant} data-density={density} {...rest}>
      {children}
    </section>
  );
});

export default Panel;
