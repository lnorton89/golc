// Toolbar is a workspace's header row -- 42px by default, but only a
// min-height (Toolbar.module.css): a workspace with a crowded action
// slot (ScriptsWorkspace's New Script/Save/Delete/Validate/Run/Debug/Stop
// row) wraps onto extra lines rather than clipping or scrolling, and
// every .workspace is a flex column with .canvas as `flex: 1;
// min-height: 0` below this, so growing here never overlaps or breaks
// anything. Holds the workspace title and a primary action slot. Distinct
// from PanelHeader (which labels a Panel section inside the canvas),
// Toolbar labels the whole workspace.
//
// `icon` (optional, lucide-react component reference) mirrors each
// workspace's own command-rail glyph (shell/destinationIcons.tsx) so the
// nav item and its workspace read as the same destination. It renders
// outside the <h2>, so the heading's accessible name (asserted verbatim by
// AppShell.navigation.test.tsx against each nav label) is unaffected.
//
// `info` (optional) renders an InfoTooltip -- also outside the <h2>, same
// reasoning -- showing this destination's own howItWorks copy
// (shell/navigation.ts's HOW_IT_WORKS_BY_ID, sourced from
// desktopViews.json). Omitted entirely by callers that pass no `info`, so
// existing Toolbar.test.tsx's "renders no buttons without an action"
// assertion is unaffected.
import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";

import InfoTooltip from "../InfoTooltip/InfoTooltip";
import styles from "./Toolbar.module.css";

interface ToolbarProps {
  title: string;
  icon?: LucideIcon;
  info?: string;
  action?: ReactNode;
}

export default function Toolbar({ title, icon: Icon, info, action }: ToolbarProps) {
  return (
    <div className={styles.toolbar}>
      <span className={styles.titleGroup}>
        {Icon ? <Icon size={16} className={styles.icon} aria-hidden="true" /> : null}
        <h2 className={styles.title}>{title}</h2>
        {info ? <InfoTooltip label={`How ${title} works`} text={info} /> : null}
      </span>
      {action ? <div className={styles.action}>{action}</div> : null}
    </div>
  );
}
