// EmptyState is the shared "nothing here yet" row: a muted icon (defaults
// to Inbox) plus a message, replacing the many near-identical
// `<p className={styles.emptyState}>...</p>` paragraphs that were
// duplicated per-workspace before the icon/polish pass. Deliberately no
// "create" call-to-action baked in -- callers that want one already render
// their own create button elsewhere (PanelHeader's action slot, a
// dedicated toolbar), so this stays a pure, reusable message row.
import type { ReactNode } from "react";
import { Inbox, type LucideIcon } from "lucide-react";

import styles from "./EmptyState.module.css";

interface EmptyStateProps {
  icon?: LucideIcon;
  children: ReactNode;
}

export default function EmptyState({ icon: Icon = Inbox, children }: EmptyStateProps) {
  return (
    <p className={styles.emptyState}>
      <Icon size={14} className={styles.icon} aria-hidden="true" />
      {children}
    </p>
  );
}
