// PanelHeader is the shared mono-caps label row for a Panel, matching the
// existing meta-label convention already used ad hoc across FixturePatch/
// SceneProgramming/etc.'s own CSS (11px JetBrains Mono, uppercase, --muted).
//
// `icon` is optional (lucide-react component reference, same convention as
// Button/Toolbar's own `icon` prop) -- purely additive, every existing
// call site renders identically without it.
import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";

import styles from "./PanelHeader.module.css";

interface PanelHeaderProps {
  label: string;
  icon?: LucideIcon;
  action?: ReactNode;
}

export default function PanelHeader({ label, icon: Icon, action }: PanelHeaderProps) {
  return (
    <div className={styles.header}>
      <span className={styles.labelGroup}>
        {Icon ? <Icon size={13} className={styles.icon} aria-hidden="true" /> : null}
        <span className={styles.label}>{label}</span>
      </span>
      {action ? <div className={styles.action}>{action}</div> : null}
    </div>
  );
}
