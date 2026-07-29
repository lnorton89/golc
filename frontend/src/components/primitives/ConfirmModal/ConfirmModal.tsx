// ConfirmModal is a small, generic yes/no confirmation dialog -- the same
// custom-dialog pattern shell/HelpOverlay.tsx already established (no
// Radix dependency; backdrop click + Escape both cancel). First caller is
// AppShell's guide-navigation guard (leaving the Guided First Show
// without finishing it), but this primitive carries no guide-specific
// copy or logic -- any future "are you sure?" prompt should reuse this
// rather than hand-rolling another backdrop+dialog pair.
//
// Cancel autofocuses via the native `autoFocus` HTML attribute (passed
// straight through Button's own ...rest spread) rather than a ref --
// Button.tsx is a plain function component, not wrapped in forwardRef.
import { useEffect } from "react";

import Button from "../Button/Button";
import styles from "./ConfirmModal.module.css";

interface ConfirmModalProps {
  title: string;
  message: string;
  confirmLabel?: string;
  cancelLabel?: string;
  onConfirm: () => void;
  onCancel: () => void;
}

export default function ConfirmModal({
  title,
  message,
  confirmLabel = "Confirm",
  cancelLabel = "Cancel",
  onConfirm,
  onCancel,
}: ConfirmModalProps) {
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onCancel();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onCancel]);

  return (
    <div className={styles.backdrop} onClick={onCancel}>
      <div
        className={styles.dialog}
        role="alertdialog"
        aria-modal="true"
        aria-label={title}
        onClick={(event) => event.stopPropagation()}
      >
        <h3 className={styles.title}>{title}</h3>
        <p className={styles.message}>{message}</p>
        <div className={styles.actions}>
          <Button variant="secondary" autoFocus onClick={onCancel}>
            {cancelLabel}
          </Button>
          <Button variant="primary" onClick={onConfirm}>
            {confirmLabel}
          </Button>
        </div>
      </div>
    </div>
  );
}
