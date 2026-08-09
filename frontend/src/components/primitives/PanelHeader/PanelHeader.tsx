// PanelHeader is the shared mono-caps label row for a Panel, matching the
// existing meta-label convention already used ad hoc across FixturePatch/
// SceneProgramming/etc.'s own CSS (11px JetBrains Mono, uppercase, --muted).
//
// `icon` is optional (lucide-react component reference, same convention as
// Button/Toolbar's own `icon` prop) -- purely additive, every existing
// call site renders identically without it.
//
// `info` (optional) renders an InfoTooltip in the same trailing,
// right-aligned .action slot `action` itself uses (info first, then
// action) rather than inline inside .labelGroup next to the label --
// mirrors Toolbar's own info/action unification (260806 screenshot
// review: this panel's "SAVE" header used to put its info icon flush
// left next to the label while the route's own title put its icon flush
// right, reading as two different conventions). An accessible name
// lookup against this panel's label text (`screen.getByText(label)`)
// stays unaffected either way, since the tooltip is a sibling of the
// label span, not nested inside it.
import { forwardRef } from "react";
import type { HTMLAttributes, ReactNode } from "react";
import type { LucideIcon } from "lucide-react";

import InfoTooltip from "../InfoTooltip/InfoTooltip";
import styles from "./PanelHeader.module.css";

export type PanelHeaderDensity = "default" | "compact";

export interface PanelHeaderProps extends HTMLAttributes<HTMLDivElement> {
  label: string;
  icon?: LucideIcon;
  info?: string;
  action?: ReactNode;
  metadata?: ReactNode;
  density?: PanelHeaderDensity;
}

const PanelHeader = forwardRef<HTMLDivElement, PanelHeaderProps>(function PanelHeader(
  { label, icon: Icon, info, action, metadata, density = "default", className, ...rest },
  ref,
) {
  const combinedClassName = className ? `${styles.header} ${className}` : styles.header;
  return (
    <div ref={ref} className={combinedClassName} data-density={density} {...rest}>
      <span className={styles.labelGroup}>
        {Icon ? <Icon className={styles.icon} aria-hidden="true" /> : null}
        <span className={styles.label}>{label}</span>
        {metadata ? <span className={styles.metadata}>{metadata}</span> : null}
      </span>
      {info || action ? (
        <div className={styles.action}>
          {info ? <InfoTooltip label={`About ${label}`} text={info} /> : null}
          {action}
        </div>
      ) : null}
    </div>
  );
});

export default PanelHeader;
