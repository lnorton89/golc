import { TriangleAlert } from "lucide-react";

import styles from "./ErrorState.module.css";

export type ErrorStateVariant = "inline" | "panel";

interface ErrorStateProps {
  heading: string;
  message: string;
  variant?: ErrorStateVariant;
  retryLabel?: string;
  onRetry?: () => void;
  diagnostic?: string;
}

export default function ErrorState({
  heading,
  message,
  variant = "inline",
  retryLabel,
  onRetry,
  diagnostic,
}: ErrorStateProps) {
  return (
    <section className={`${styles.errorState} ${styles[variant]}`} role="alert">
      <TriangleAlert className={styles.icon} size={20} aria-hidden="true" data-testid="error-icon" />
      <div className={styles.copy}>
        <h2 className={styles.heading}>{heading}</h2>
        <p className={styles.message}>{message}</p>
        {onRetry && retryLabel ? (
          <button type="button" className={styles.retry} onClick={onRetry}>
            {retryLabel}
          </button>
        ) : null}
        {diagnostic ? (
          <details className={styles.diagnostic}>
            <summary>Technical details</summary>
            <pre>{diagnostic}</pre>
          </details>
        ) : null}
      </div>
    </section>
  );
}
