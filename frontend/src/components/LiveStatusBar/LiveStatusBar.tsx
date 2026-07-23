// LiveStatusBar.tsx is the fixed-height, persistent chrome region showing
// active scene, enabled layers, BPM/bar position, controlling source, and
// final output state (PLAY-07, 06-UI-SPEC.md "Live status bar ... fixed
// height, not built from the standard scale -- treat as a locked chrome
// region"). This Wave 2 stub renders only the labeled placeholder plus the
// UI-SPEC loading backstop (a dim/skeleton state while
// useGolcStore().connectionStatus is still "connecting", i.e. before the
// Go host completes its first daemon status fetch); 06-05-PLAN.md replaces
// this file's contents with the real status projection, never changing
// App.tsx's mount point for this component.

import type { CSSProperties } from "react";

import { useGolcStore } from "../../store/store";

const barStyle: CSSProperties = {
  display: "flex",
  alignItems: "center",
  height: "48px",
  padding: "0 var(--space-md)",
  background: "var(--panel)",
  borderBottom: "1px solid var(--line)",
  fontFamily: "JetBrains Mono, ui-monospace, monospace",
  fontSize: "14px",
  color: "var(--text)",
};

export default function LiveStatusBar() {
  const connectionStatus = useGolcStore((state) => state.connectionStatus);
  const loading = connectionStatus === "connecting";

  return (
    <div
      style={{ ...barStyle, opacity: loading ? 0.5 : 1 }}
      aria-label="Live status bar"
      aria-busy={loading}
    >
      {loading ? "Connecting to playback engine…" : "Live status bar (stub)"}
    </div>
  );
}
