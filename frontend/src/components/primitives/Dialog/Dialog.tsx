import { Dialog as BaseDialog } from "@base-ui/react/dialog";
import { useEffect, useRef, type ReactNode, type RefObject } from "react";

import styles from "./Dialog.module.css";

export interface DialogProps {
  open: boolean;
  title: ReactNode;
  description?: ReactNode;
  children: ReactNode;
  onClose: () => void;
  /** The least-destructive action to focus when the dialog opens. */
  initialFocusRef?: RefObject<HTMLElement | null>;
  closeOnEscape?: boolean;
  closeOnBackdrop?: boolean;
  role?: "dialog" | "alertdialog";
}

export default function Dialog({
  open,
  title,
  description,
  children,
  onClose,
  initialFocusRef,
  closeOnEscape = true,
  closeOnBackdrop = true,
  role = "dialog",
}: DialogProps) {
  // finalFocus has to be tracked explicitly (not Base UI's automatic
  // trigger-return-focus) because this Dialog is always controlled from the
  // outside -- there is no Dialog.Trigger element for Base UI to remember as
  // the thing to refocus on close.
  const returnFocusRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (!open) return;
    const activeElement = document.activeElement;
    returnFocusRef.current = activeElement instanceof HTMLElement ? activeElement : null;
  }, [open]);

  return (
    <BaseDialog.Root
      open={open}
      onOpenChange={(next, eventDetails) => {
        if (next) return;
        if (eventDetails.reason === "escape-key" && !closeOnEscape) {
          eventDetails.cancel();
          return;
        }
        if (eventDetails.reason === "outside-press" && !closeOnBackdrop) {
          eventDetails.cancel();
          return;
        }
        onClose();
      }}
    >
      <BaseDialog.Portal>
        <BaseDialog.Backdrop className={styles.backdrop} data-testid="dialog-backdrop" />
        <BaseDialog.Popup
          className={styles.dialog}
          role={role}
          // Base UI enforces modality via `inert` on background content
          // rather than setting aria-modal itself -- added explicitly since
          // not every assistive-tech pairing treats `inert` as sufficient on
          // its own, and this was the existing contract's own ARIA guarantee.
          aria-modal="true"
          initialFocus={initialFocusRef}
          finalFocus={returnFocusRef}
        >
          <div className={styles.content}>
            <BaseDialog.Title className={styles.title}>{title}</BaseDialog.Title>
            {description ? <BaseDialog.Description className={styles.description}>{description}</BaseDialog.Description> : null}
            <div className={styles.body}>{children}</div>
          </div>
        </BaseDialog.Popup>
      </BaseDialog.Portal>
    </BaseDialog.Root>
  );
}
