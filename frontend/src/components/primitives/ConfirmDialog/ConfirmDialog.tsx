import { useRef, type ReactNode } from "react";

import Button from "../Button/Button";
import Dialog from "../Dialog/Dialog";
import styles from "./ConfirmDialog.module.css";

export interface ConfirmDialogProps {
  open: boolean;
  title: ReactNode;
  message: ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  destructive?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
  closeOnEscape?: boolean;
  closeOnBackdrop?: boolean;
}

/**
 * A confirmation is an explicit decision surface. It never owns the command it
 * precedes: hold-to-confirm safety commands retain their own direct, local path.
 */
export default function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel = "Confirm",
  cancelLabel = "Cancel",
  destructive = false,
  onConfirm,
  onCancel,
  closeOnEscape = true,
  closeOnBackdrop = true,
}: ConfirmDialogProps) {
  const cancelRef = useRef<HTMLButtonElement>(null);

  return (
    <Dialog
      open={open}
      title={title}
      description={message}
      initialFocusRef={cancelRef}
      onClose={onCancel}
      closeOnEscape={closeOnEscape}
      closeOnBackdrop={closeOnBackdrop}
      role={destructive ? "alertdialog" : "dialog"}
    >
      <div className={styles.actions}>
        <Button ref={cancelRef} variant="secondary" onClick={onCancel}>
          {cancelLabel}
        </Button>
        <Button variant={destructive ? "destructive" : "primary"} onClick={onConfirm}>
          {confirmLabel}
        </Button>
      </div>
    </Dialog>
  );
}
