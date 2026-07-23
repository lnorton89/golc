// SafetyCluster.tsx is the persistent, visually distinct safety-cluster
// region (D-13/D-15): Blackout / Revoke Automation / Stop-Release-All, in
// a fixed screen position present on every view (authoring, programming,
// playback alike). This Wave 2 stub renders the three labeled
// placeholders only -- 06-05-PLAN.md replaces this file's contents with
// the real hold-to-confirm (D-14) controls wired to SafetyService, never
// changing App.tsx's mount point for this component.
//
// D-13 also means this region must remain visible even when the daemon is
// unreachable (the on-screen path is one of two independent triggers into
// the same daemon override state, per 06-RESEARCH.md Pitfall 1 -- the
// other being the OS-level hotkeys internal/wails/hotkey.go registers).
// This stub therefore never gates its own rendering on connection status.

import type { CSSProperties } from "react";

const clusterStyle: CSSProperties = {
  display: "flex",
  gap: "var(--space-md)",
  padding: "var(--space-md)",
  border: "2px solid var(--status-revoked)",
  borderRadius: "4px",
  background: "var(--panel)",
};

const controlStyle: CSSProperties = {
  minHeight: "64px",
  minWidth: "160px",
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  color: "var(--ink)",
  fontFamily: "Archivo, system-ui, sans-serif",
  fontWeight: 600,
  border: "1px solid var(--line)",
  borderRadius: "4px",
  background: "var(--page)",
};

export default function SafetyCluster() {
  return (
    <div style={clusterStyle} aria-label="Safety cluster">
      <div style={controlStyle}>Hold to Blackout</div>
      <div style={controlStyle}>Hold to Revoke Automation</div>
      <div style={controlStyle}>Hold to Stop / Release All</div>
    </div>
  );
}
