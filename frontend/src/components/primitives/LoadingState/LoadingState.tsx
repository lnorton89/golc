import type { ReactNode } from "react";
import { LoaderCircle } from "lucide-react";

import styles from "./LoadingState.module.css";

export type LoadingStateVariant = "inline" | "panel" | "list-row";

interface LoadingStateProps {
  label: string;
  variant?: LoadingStateVariant;
  children?: ReactNode;
}

export default function LoadingState({ label, variant = "inline", children }: LoadingStateProps) {
  return (
    <div className={`${styles.wrapper} ${styles[variant]}`} aria-busy="true">
      {children}
      <div className={styles.status} role="status" aria-live="polite" aria-busy="true" aria-label={label}>
        <LoaderCircle className={styles.spinner} size={16} aria-hidden="true" />
        <span>{label}</span>
      </div>
    </div>
  );
}
