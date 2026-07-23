// PlaybackControls.tsx is the feature region for on-screen playback (scene
// switch, layer enable/disable, BPM/tap tempo, transport -- PLAY-01) and,
// eventually, the documented in-webview keyboard workflow (PLAY-02). This
// Wave 2 stub renders only a labeled placeholder plus the UI-SPEC loading
// backstop; 06-06-PLAN.md replaces this file's contents with the real
// controls wired to PlaybackService, never changing App.tsx's mount point
// for this component.

import type { CSSProperties } from "react";

import { useGolcStore } from "../../store/store";

const panelStyle: CSSProperties = {
  padding: "var(--space-lg)",
  background: "var(--panel)",
  border: "1px solid var(--line)",
  borderRadius: "4px",
  color: "var(--text)",
};

export default function PlaybackControls() {
  const connectionStatus = useGolcStore((state) => state.connectionStatus);
  const loading = connectionStatus === "connecting";

  return (
    <section
      style={{ ...panelStyle, opacity: loading ? 0.5 : 1 }}
      aria-label="Playback controls"
      aria-busy={loading}
    >
      {loading ? "Loading playback controls…" : "Playback controls (stub)"}
    </section>
  );
}
