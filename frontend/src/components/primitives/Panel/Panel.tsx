// Panel is the shared surface primitive for every workspace canvas region,
// inspector section, and stub screen (shell restructure Step 2). It is a
// pure CSS-Modules wrapper -- no Wails calls, no state -- so existing
// feature components (FixturePatch, ArtnetConfig, etc.) can adopt it by
// swapping their own ad hoc `<section className={styles.panel}>` for this
// shared one without changing any behavior.
import type { HTMLAttributes, ReactNode } from "react";

import styles from "./Panel.module.css";

interface PanelProps extends HTMLAttributes<HTMLElement> {
  children: ReactNode;
}

export default function Panel({ children, className, ...rest }: PanelProps) {
  const combinedClassName = className ? `${styles.panel} ${className}` : styles.panel;
  return (
    <section className={combinedClassName} {...rest}>
      {children}
    </section>
  );
}
