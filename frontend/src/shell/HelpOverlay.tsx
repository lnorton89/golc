// HelpOverlay wraps the existing KeyboardShortcuts reference panel in a
// simple custom dialog (shell restructure plan Step 8) -- toggled by '?'
// via useGlobalKeyboardWorkflow.ts, closed by Escape or backdrop click. No
// Radix dependency: nothing in this shell's actual CSS patterns needs its
// focus/roving-tabindex composition (SoftTakeoverSlider is already a fully
// custom widget with no Radix), and adding a package for exactly one
// static dialog isn't proportionate -- revisit if a future primitive
// genuinely needs it.
import { useEffect, useRef } from "react";

import KeyboardShortcuts from "../components/KeyboardShortcuts/KeyboardShortcuts";
import styles from "./HelpOverlay.module.css";

interface HelpOverlayProps {
  open: boolean;
  onClose: () => void;
}

export default function HelpOverlay({ open, onClose }: HelpOverlayProps) {
  const closeButtonRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (open) {
      closeButtonRef.current?.focus();
    }
  }, [open]);

  if (!open) {
    return null;
  }

  return (
    <div className={styles.backdrop} onClick={onClose}>
      <div
        className={styles.dialog}
        role="dialog"
        aria-modal="true"
        aria-label="Keyboard shortcuts"
        onClick={(event) => event.stopPropagation()}
      >
        <div className={styles.header}>
          <span className={styles.title}>Keyboard Shortcuts</span>
          <button type="button" ref={closeButtonRef} className={styles.close} onClick={onClose}>
            Close
          </button>
        </div>
        <div className={styles.body}>
          <KeyboardShortcuts />
        </div>
      </div>
    </div>
  );
}
