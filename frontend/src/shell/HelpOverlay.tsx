// HelpOverlay wraps the existing KeyboardShortcuts reference panel in the
// shared, packaged-proven Dialog primitive (13-06's Chromium- and
// packaged-WebView2-verified focus-trap/Escape/backdrop/portal contract)
// instead of a hand-rolled backdrop+dialog pair -- toggled by '?' via
// useGlobalKeyboardWorkflow.ts, closed by Escape, an allowed backdrop
// click, or the Close button. KeyboardShortcuts itself has no focusable
// descendants, so Dialog's own "first focusable element" default lands on
// Close automatically, matching this overlay's previous explicit
// close-button auto-focus.
import { Button, Dialog } from "../design-system";
import KeyboardShortcuts from "../components/KeyboardShortcuts/KeyboardShortcuts";
import styles from "./HelpOverlay.module.css";

interface HelpOverlayProps {
  open: boolean;
  onClose: () => void;
}

export default function HelpOverlay({ open, onClose }: HelpOverlayProps) {
  return (
    <Dialog open={open} title="Keyboard shortcuts" onClose={onClose}>
      <KeyboardShortcuts />
      <div className={styles.actions}>
        <Button variant="secondary" onClick={onClose}>
          Close
        </Button>
      </div>
    </Dialog>
  );
}
