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
  /**
   * ARIA role, fully decoupled from `destructive` (mirrors Dialog.tsx's own
   * `role` prop). `destructive` only ever controls the confirm button's
   * visual variant; a confirmation can need `alertdialog` semantics -- e.g.
   * an interruption that risks losing in-progress context -- without being a
   * destructive/red-styled action, and vice versa. Defaults to
   * `destructive`'s prior implied role so existing callers are unaffected.
   */
  role?: "dialog" | "alertdialog";
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
  role,
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
      role={role ?? (destructive ? "alertdialog" : "dialog")}
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
