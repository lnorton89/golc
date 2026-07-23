// OperatorSurface.tsx is the feature region for the operator-surface
// builder (D-01..D-04) and its constrained playback rendering (PLAY-03).
// This Wave 2 stub renders only a labeled placeholder plus the UI-SPEC
// loading backstop; 06-07-PLAN.md replaces this file's contents with the
// real surface list/assignment UI wired to SurfaceService, never changing
// App.tsx's mount point for this component.

import type { CSSProperties } from "react";

import { useGolcStore } from "../../store/store";

const panelStyle: CSSProperties = {
  padding: "var(--space-lg)",
  background: "var(--panel)",
  border: "1px solid var(--line)",
  borderRadius: "4px",
  color: "var(--text)",
};

export default function OperatorSurface() {
  const connectionStatus = useGolcStore((state) => state.connectionStatus);
  const loading = connectionStatus === "connecting";

  return (
    <section
      style={{ ...panelStyle, opacity: loading ? 0.5 : 1 }}
      aria-label="Operator surfaces"
      aria-busy={loading}
    >
      {loading ? "Loading operator surfaces…" : "Operator surfaces (stub)"}
    </section>
  );
}
