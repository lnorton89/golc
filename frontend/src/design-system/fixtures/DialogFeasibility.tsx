import { createPortal } from "react-dom";
import { useEffect, useRef, useState } from "react";

import SafetyCluster from "../../components/SafetyCluster/SafetyCluster";
import ConfirmDialog from "../../components/primitives/ConfirmDialog/ConfirmDialog";
import Dialog from "../../components/primitives/Dialog/Dialog";

/**
 * A deliberately small, test-only surface for verifying the public dialog
 * contract in both Chromium and the packaged WebView2 runtime. It is mounted
 * only by App's `?e2e=dialog-feasibility` route.
 */
export default function DialogFeasibility() {
  const [allowedOpen, setAllowedOpen] = useState(false);
  const [blockedOpen, setBlockedOpen] = useState(false);
  const allowedTriggerRef = useRef<HTMLButtonElement>(null);
  const blockedTriggerRef = useRef<HTMLButtonElement>(null);
  const safeActionRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    allowedTriggerRef.current?.focus();
  }, []);

  return (
    <main aria-label="Dialog feasibility fixture">
      <h1>Dialog feasibility</h1>
      <button ref={allowedTriggerRef} type="button" onClick={() => setAllowedOpen(true)}>
        Open allowed dialog
      </button>
      <button ref={blockedTriggerRef} type="button" onClick={() => setBlockedOpen(true)}>
        Open blocked dialog
      </button>
      {createPortal(
        <div style={{ position: "fixed", top: "var(--ds-spacing-space4)", right: "var(--ds-spacing-space4)", zIndex: "var(--ds-stacking-emergency-overlay)" }}>
          <SafetyCluster />
        </div>,
        document.body,
      )}

      <Dialog
        open={allowedOpen}
        title="Review fixture patch"
        description="Review this non-destructive fixture change before applying it."
        initialFocusRef={safeActionRef}
        onClose={() => setAllowedOpen(false)}
      >
        <button ref={safeActionRef} type="button" onClick={() => setAllowedOpen(false)}>
          Keep editing
        </button>
        <button type="button" onClick={() => setAllowedOpen(false)}>
          Apply review
        </button>
        {createPortal(
          <div data-testid="nested-portal-content" role="status">
            Nested portal content
          </div>,
          document.body,
        )}
      </Dialog>

      <ConfirmDialog
        open={blockedOpen}
        title="Discard fixture changes?"
        message="This fixture keeps its dismissal policy explicit."
        cancelLabel="Keep fixture"
        confirmLabel="Discard fixture"
        onCancel={() => setBlockedOpen(false)}
        onConfirm={() => setBlockedOpen(false)}
        closeOnEscape={false}
        closeOnBackdrop={false}
      />
    </main>
  );
}
