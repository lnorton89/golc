// PanelHeader is the shared mono-caps label row for a Panel, matching the
// existing meta-label convention already used ad hoc across FixturePatch/
// SceneProgramming/etc.'s own CSS (11px JetBrains Mono, uppercase, --muted).
import type { ReactNode } from "react";

import styles from "./PanelHeader.module.css";

interface PanelHeaderProps {
  label: string;
  action?: ReactNode;
}

export default function PanelHeader({ label, action }: PanelHeaderProps) {
  return (
    <div className={styles.header}>
      <span className={styles.label}>{label}</span>
      {action ? <div className={styles.action}>{action}</div> : null}
    </div>
  );
}
