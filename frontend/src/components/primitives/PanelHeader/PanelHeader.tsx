// PanelHeader is the shared mono-caps label row for a Panel, matching the
// existing meta-label convention already used ad hoc across FixturePatch/
// SceneProgramming/etc.'s own CSS (11px JetBrains Mono, uppercase, --muted).
//
// `icon` is optional (lucide-react component reference, same convention as
// Button/Toolbar's own `icon` prop) -- purely additive, every existing
// call site renders identically without it.
//
// `info` (optional) renders an InfoTooltip inside .labelGroup, after the
// label span -- same reasoning as Toolbar's own `info`: an accessible
// name lookup against this panel's label text (`screen.getByText(label)`)
// stays unaffected since the tooltip is a sibling, not nested inside it.
import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";

import InfoTooltip from "../InfoTooltip/InfoTooltip";
import styles from "./PanelHeader.module.css";

interface PanelHeaderProps {
  label: string;
  icon?: LucideIcon;
  info?: string;
  action?: ReactNode;
}

export default function PanelHeader({ label, icon: Icon, info, action }: PanelHeaderProps) {
  return (
    <div className={styles.header}>
      <span className={styles.labelGroup}>
        {Icon ? <Icon size={13} className={styles.icon} aria-hidden="true" /> : null}
        <span className={styles.label}>{label}</span>
        {info ? <InfoTooltip label={`About ${label}`} text={info} /> : null}
      </span>
      {action ? <div className={styles.action}>{action}</div> : null}
    </div>
  );
}
