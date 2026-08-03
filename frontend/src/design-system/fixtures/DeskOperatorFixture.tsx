// DeskOperatorFixture.tsx (Plan 13-34 Task 1, UI-SPEC-VISUAL-MATRIX): a
// deterministic, browser-reachable e2e-only fixture mounting the real
// Operator Surface "operate" mode Launcher (LIVE/locked scene pads, the
// active scene's layer strip, and Masters) directly adjacent to the real
// Desk projected-fader grid, with the persistent SafetyCluster visible
// below both -- UI-SPEC's Required reference matrix names this exact
// combined state ("Active LIVE pad, locked pad, masters, visible safety
// controls" plus this plan's own "projected faders") for the
// "Desk / Operator Surface" row. No single existing navigable destination
// renders both surfaces on one screen: OperatorSurfaceWorkspace (Operate)
// and DeskWorkspace (Perform) are two separate WorkspaceRouter
// destinations (see workspaces/perform/DeskWorkspace.tsx and
// workspaces/operate/OperatorSurfaceWorkspace.tsx). Mirrors the
// established ?e2e=... fixture-route precedent (DialogFeasibility.tsx,
// DesignSystemGallery.tsx, EmergencyFallbackFixture.tsx) rather than
// inventing a second mechanism -- every child here is the real production
// component (Launcher/Desk/SafetyCluster), not a re-implementation; only
// the deterministic Wails seed and connectionStatus are supplied by the
// spec file that mounts this route (frontend/e2e/
// design-system.visual-live-editors.spec.ts), exactly like every other
// fixture in this directory keeps data-seeding out of the component tree
// itself.
import { useEffect } from "react";

import { useGolcStore } from "../../store/store";
import { PlaybackSnapshotProvider } from "../../shell/PlaybackSnapshotContext";
import Panel from "../../components/primitives/Panel/Panel";
import PanelHeader from "../../components/primitives/PanelHeader/PanelHeader";
import Launcher from "../../components/OperatorSurface/Launcher";
import type { ControlRefView } from "../../components/OperatorSurface/OperatorSurface";
import Desk from "../../components/Desk/Desk";
import SafetyCluster from "../../components/SafetyCluster/SafetyCluster";

// FIXTURE_CONTROLS seeds one assigned, currently-live scene ("Opening
// Look", all four layers assigned) and one unassigned/locked scene
// ("Interlude"), plus one assigned Grand Master -- Launcher.tsx derives
// live/locked purely from PlaybackSnapshotContext's own active-scene state
// (seeded by the spec file's mocked PlaybackService.GetState) crossed with
// each control's own `assigned` flag here, exactly the same derivation the
// real OperatorSurfaceWorkspace uses.
export const FIXTURE_CONTROLS: ControlRefView[] = [
  { kind: "scene", scene: "Opening Look", label: "Opening Look", assigned: true },
  { kind: "scene", scene: "Interlude", label: "Interlude", assigned: false },
  { kind: "layer", scene: "Opening Look", layerKind: "base_look", label: "Base Look", assigned: true },
  { kind: "layer", scene: "Opening Look", layerKind: "color_theme", label: "Color Theme", assigned: true },
  { kind: "layer", scene: "Opening Look", layerKind: "chase", label: "Chase", assigned: true },
  { kind: "layer", scene: "Opening Look", layerKind: "motion", label: "Motion", assigned: true },
  { kind: "master", masterKind: "grand", label: "Grand Master", assigned: true },
];

export default function DeskOperatorFixture() {
  // usePlaybackStateSnapshot only polls PlaybackService.GetState while
  // connectionStatus === "connected" -- every real navigable destination
  // reaches "connected" via LiveStatusBar's own FetchStatus effect, which
  // this standalone fixture deliberately doesn't mount (it isn't part of
  // AppShell's persistent chrome). Setting it directly here is the
  // fixture-local equivalent, mirroring how Vitest's own test suites set
  // this same store field directly rather than mounting LiveStatusBar.
  useEffect(() => {
    useGolcStore.getState().setConnectionStatus("connected");
  }, []);

  return (
    <main
      aria-label="Desk and Operator Surface fixture"
      style={{
        display: "flex",
        flexDirection: "column",
        minHeight: "100vh",
        boxSizing: "border-box",
        gap: "var(--ds-spacing-space2)",
        padding: "var(--ds-spacing-space2)",
      }}
    >
      {/* A compact heading (not the default browser h1 sizing/margins):
          this fixture's own captured content (safety controls, LIVE/
          locked pads, masters, projected faders) needs every available
          pixel of the 720px regression viewport height -- the page title
          itself doesn't. */}
      <h1 style={{ margin: 0, fontSize: "var(--ds-typography-font-size-heading)" }}>Desk / Operator Surface</h1>
      {/* SafetyCluster sits first, immediately below the heading -- the
          combined Launcher+Desk content below is taller than the 720px
          regression viewport, so a safety cluster placed after it would
          scroll out of the captured screenshot (toHaveScreenshot captures
          only the viewport, not the full page). Mirrors GlobalFrame's own
          real persistent-header placement (top of the screen, always
          reachable) rather than inventing a bottom-anchored position. */}
      <SafetyCluster />
      <PlaybackSnapshotProvider>
        {/* Stacked (not side by side): a two-column split squeezes Desk's
            own projected-fader grid below its real minimum content width
            at the 900px compact-width regression viewport, overflowing
            the document horizontally. Stacking vertically instead gives
            each real production component its own full page width -- the
            exact width it renders at when reached through its own real
            navigable destination -- while still satisfying UI-SPEC's
            "adjacent" requirement (vertically adjacent, not necessarily
            side by side). density="compact" trims Panel's own padding so
            more of Desk's projected faders below fit inside the viewport. */}
        <Panel aria-label="Operator surface preview" density="compact">
          <PanelHeader label="Operator Surface — Booth (Preview as Operator)" density="compact" />
          <Launcher controls={FIXTURE_CONTROLS} />
        </Panel>
        <Desk />
      </PlaybackSnapshotProvider>
    </main>
  );
}
