// EmptyState is a projection-only composition for a bounded empty region.
// The legacy children form remains supported so existing workspace messages
// can migrate independently; new callers provide a heading/body/action.
import type { ReactNode } from "react";
import { Inbox, type LucideIcon } from "lucide-react";

import styles from "./EmptyState.module.css";

interface EmptyStateProps {
  icon?: LucideIcon;
  children?: ReactNode;
  heading?: string;
  body?: ReactNode;
  action?: ReactNode;
}

export default function EmptyState({ icon: Icon = Inbox, children, heading, body, action }: EmptyStateProps) {
  if (!heading) {
    return (
      <p className={styles.legacy}>
        <Icon size={14} className={styles.icon} aria-hidden="true" />
        {children}
      </p>
    );
  }

  return (
    <section className={styles.emptyState}>
      <Icon size={20} className={styles.icon} aria-hidden="true" />
      <div className={styles.copy}>
        <h2 className={styles.heading}>{heading}</h2>
        {body ? <p className={styles.body}>{body}</p> : null}
        {action ? <div className={styles.action}>{action}</div> : null}
      </div>
    </section>
  );
}
