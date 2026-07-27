// Toolbar is a workspace's fixed 42px header row (application-shell-
// navigation.md's `.workspace{grid-template-rows:42px minmax(0,1fr)}`) --
// holds the workspace title and a primary action slot. Distinct from
// PanelHeader (which labels a Panel section inside the canvas), Toolbar
// labels the whole workspace.
//
// `icon` (optional, lucide-react component reference) mirrors each
// workspace's own command-rail glyph (shell/destinationIcons.tsx) so the
// nav item and its workspace read as the same destination. It renders
// outside the <h2>, so the heading's accessible name (asserted verbatim by
// AppShell.navigation.test.tsx against each nav label) is unaffected.
import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";

import styles from "./Toolbar.module.css";

interface ToolbarProps {
  title: string;
  icon?: LucideIcon;
  action?: ReactNode;
}

export default function Toolbar({ title, icon: Icon, action }: ToolbarProps) {
  return (
    <div className={styles.toolbar}>
      <span className={styles.titleGroup}>
        {Icon ? <Icon size={16} className={styles.icon} aria-hidden="true" /> : null}
        <h2 className={styles.title}>{title}</h2>
      </span>
      {action ? <div className={styles.action}>{action}</div> : null}
    </div>
  );
}
