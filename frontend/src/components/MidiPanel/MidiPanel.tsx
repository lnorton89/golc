// MidiPanel.tsx is the feature region for generic MIDI Note/CC learn and
// soft-takeover feedback (PLAY-04/05, D-05..D-12). This Wave 2 stub
// renders only a labeled placeholder plus the UI-SPEC loading backstop;
// 06-08-PLAN.md replaces this file's contents with the real learn/mapping
// UI wired to MidiService, never changing App.tsx's mount point for this
// component.

import type { CSSProperties } from "react";

import { useGolcStore } from "../../store/store";

const panelStyle: CSSProperties = {
  padding: "var(--space-lg)",
  background: "var(--panel)",
  border: "1px solid var(--line)",
  borderRadius: "4px",
  color: "var(--text)",
};

export default function MidiPanel() {
  const connectionStatus = useGolcStore((state) => state.connectionStatus);
  const loading = connectionStatus === "connecting";

  return (
    <section
      style={{ ...panelStyle, opacity: loading ? 0.5 : 1 }}
      aria-label="MIDI mappings"
      aria-busy={loading}
    >
      {loading ? "Loading MIDI mappings…" : "MIDI mappings (stub)"}
    </section>
  );
}
