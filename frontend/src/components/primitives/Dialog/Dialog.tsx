import { createPortal } from "react-dom";
import { useEffect, useId, useRef, type KeyboardEvent, type ReactNode, type RefObject } from "react";

import styles from "./Dialog.module.css";

const FOCUSABLE_SELECTOR = [
  "a[href]",
  "button:not([disabled])",
  "input:not([disabled])",
  "select:not([disabled])",
  "textarea:not([disabled])",
  "[tabindex]:not([tabindex='-1'])",
].join(",");

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

function getFocusableElements(container: HTMLElement): HTMLElement[] {
  return Array.from(container.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter(
    (element) => !element.hasAttribute("disabled") && element.getAttribute("aria-hidden") !== "true",
  );
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
  const dialogRef = useRef<HTMLDivElement>(null);
  const returnFocusRef = useRef<HTMLElement | null>(null);
  const titleId = useId();
  const descriptionId = useId();

  useEffect(() => {
    if (!open) return;

    const activeElement = document.activeElement;
    returnFocusRef.current = activeElement instanceof HTMLElement ? activeElement : null;

    const dialog = dialogRef.current;
    const focusTarget = initialFocusRef?.current ?? (dialog ? getFocusableElements(dialog)[0] : null) ?? dialog;
    focusTarget?.focus();

    return () => {
      returnFocusRef.current?.focus();
    };
  }, [initialFocusRef, open]);

  if (!open) return null;

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === "Escape" && closeOnEscape) {
      event.preventDefault();
      onClose();
      return;
    }

    if (event.key !== "Tab") return;

    const dialog = dialogRef.current;
    if (!dialog) return;
    const focusableElements = getFocusableElements(dialog);
    if (focusableElements.length === 0) {
      event.preventDefault();
      dialog.focus();
      return;
    }

    const first = focusableElements[0];
    const last = focusableElements[focusableElements.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  };

  return createPortal(
    <div
      className={styles.backdrop}
      data-testid="dialog-backdrop"
      onMouseDown={(event) => {
        if (closeOnBackdrop && event.target === event.currentTarget) onClose();
      }}
    >
      <div
        ref={dialogRef}
        className={styles.dialog}
        role={role}
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={description ? descriptionId : undefined}
        tabIndex={-1}
        onKeyDown={handleKeyDown}
      >
        <div className={styles.content}>
          <h2 id={titleId} className={styles.title}>{title}</h2>
          {description ? <div id={descriptionId} className={styles.description}>{description}</div> : null}
          <div className={styles.body}>{children}</div>
        </div>
      </div>
    </div>,
    document.body,
  );
}
