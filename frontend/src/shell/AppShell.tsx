// AppShell is the persistent top-level layout (application-shell-
// navigation.md's Focused Command Rail, Sketch 001 Variant D): a fixed
// CSS grid with a global header, a fixed safety row directly beneath it,
// left nav, one workspace canvas, and a contextual right inspector. The
// safety row was moved here from a bottom footer (icon/polish pass) --
// still full-width and unconditionally mounted, so D-13's "visible and
// interactive on every workspace, independent of daemon reachability"
// guarantee is unchanged, only its screen position moved.
//
// Navigation-selection state lives here as plain useState, not in
// useGolcStore or Context: store.ts is documented as "cache of Go-pushed
// snapshots, never authoritative", and nav selection never round-trips
// through a Wails binding, so it doesn't belong there. CommandRail and
// WorkspaceRouter are both direct children, so plain state needs no
// prop-drilling relief.
//
// SafetyCluster and GlobalFrame's LiveStatusBar mount unconditionally here,
// never inside WorkspaceRouter's switch -- this is what guarantees
// LiveStatusBar (the store's sole writer of status/connectionStatus) never
// unmounts regardless of which workspace is active.
//
// PlaybackSnapshotProvider wraps ShellBody (not AppShell's own return)
// because useGlobalKeyboardWorkflow/TempoControls/GlobalFrame all need to
// be inside the context it creates.
//
// inspectorContainer is captured once via a ref callback (ContextualInspector
// hands its DOM node up on mount) and handed to InspectorPortalProvider --
// workspaces portal content into it directly (InspectorSlot.tsx), so no
// per-publish state update ever touches ShellBody.
import { useState } from "react";

import SafetyCluster from "../components/SafetyCluster/SafetyCluster";
import GlobalFrame from "./GlobalFrame";
import CommandRail from "./CommandRail";
import ContextualInspector from "./ContextualInspector";
import WorkspaceRouter from "./WorkspaceRouter";
import HelpOverlay from "./HelpOverlay";
import { InspectorPortalProvider } from "./InspectorSlot";
import { PlaybackSnapshotProvider } from "./PlaybackSnapshotContext";
import { useGlobalKeyboardWorkflow } from "./useGlobalKeyboardWorkflow";
import { DEFAULT_DESTINATION, type DestinationId } from "./navigation";
import styles from "./AppShell.module.css";

function ShellBody() {
  const [activeDestination, setActiveDestination] = useState<DestinationId>(DEFAULT_DESTINATION);
  const [inspectorContainer, setInspectorContainer] = useState<HTMLDivElement | null>(null);
  const { helpOpen, closeHelp } = useGlobalKeyboardWorkflow();

  return (
    <div className={styles.appShell}>
      <div className={styles.header}>
        <GlobalFrame />
      </div>
      <div className={styles.safety}>
        <SafetyCluster />
      </div>
      <div className={styles.rail}>
        <CommandRail active={activeDestination} onSelect={setActiveDestination} />
      </div>
      <main className={styles.main}>
        <InspectorPortalProvider container={inspectorContainer}>
          <WorkspaceRouter active={activeDestination} />
        </InspectorPortalProvider>
      </main>
      <div className={styles.inspector}>
        <ContextualInspector onContainerReady={setInspectorContainer} />
      </div>
      <HelpOverlay open={helpOpen} onClose={closeHelp} />
    </div>
  );
}

export default function AppShell() {
  return (
    <PlaybackSnapshotProvider>
      <ShellBody />
    </PlaybackSnapshotProvider>
  );
}
