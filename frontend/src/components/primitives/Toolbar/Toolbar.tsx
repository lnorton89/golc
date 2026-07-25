// Toolbar is a workspace's fixed 42px header row (application-shell-
// navigation.md's `.workspace{grid-template-rows:42px minmax(0,1fr)}`) --
// holds the workspace title and a primary action slot. Distinct from
// PanelHeader (which labels a Panel section inside the canvas), Toolbar
// labels the whole workspace.
import type { ReactNode } from "react";

import styles from "./Toolbar.module.css";

interface ToolbarProps {
  title: string;
  action?: ReactNode;
}

export default function Toolbar({ title, action }: ToolbarProps) {
  return (
    <div className={styles.toolbar}>
      <h2 className={styles.title}>{title}</h2>
      {action ? <div className={styles.action}>{action}</div> : null}
    </div>
  );
}
