// AppShell is the persistent top-level layout (application-shell-
// navigation.md's Focused Command Rail, Sketch 001 Variant D): a fixed
// CSS grid with a global header (which now also carries the safety
// cluster inline, see GlobalFrame.tsx), left nav, one workspace canvas,
// and a contextual right inspector. The safety row previously lived here
// as its own dedicated grid row beneath the header (icon/polish pass);
// the header-merge pass folded it directly into GlobalFrame instead --
// still unconditionally mounted, so D-13's "visible and interactive on
// every workspace, independent of daemon reachability" guarantee is
// unchanged, only its screen position moved again.
//
// Navigation-selection state lives here as plain useState, not in
// useGolcStore or Context: store.ts is documented as "cache of Go-pushed
// snapshots, never authoritative", and nav selection never round-trips
// through a Wails binding, so it doesn't belong there. CommandRail and
// WorkspaceRouter are both direct children, so plain state needs no
// prop-drilling relief.
//
// GlobalFrame (which owns LiveStatusBar and SafetyCluster) mounts
// unconditionally here, never inside WorkspaceRouter's switch -- this is
// what guarantees LiveStatusBar (the store's sole writer of
// status/connectionStatus) never unmounts regardless of which workspace
// is active.
//
// PlaybackSnapshotProvider wraps ShellBody (not AppShell's own return)
// because useGlobalKeyboardWorkflow/TempoControls/GlobalFrame all need to
// be inside the context it creates.
//
// inspectorContainer is captured once via a ref callback (ContextualInspector
// hands its DOM node up on mount) and handed to InspectorPortalProvider --
// workspaces portal content into it directly (InspectorSlot.tsx), so no
// per-publish state update ever touches ShellBody.
//
// GuidedFirstShowProvider wraps ShellCanvas (09-03-PLAN.md, D-08/D-10):
// the guide's open/exit/navigate state and its once-per-process
// auto-launch guard must survive whichever workspace WorkspaceRouter
// happens to mount/unmount, so it lives here rather than inside
// OverviewWorkspace itself. ShellCanvas renders <GuidedFirstShow /> in
// place of <WorkspaceRouter /> only while the guide is open -- the guide
// replaces the canvas alone; SafetyCluster/GlobalFrame/CommandRail/
// ContextualInspector stay mounted unconditionally exactly as before
// (application-shell-navigation.md's interaction contract).
import { useState } from "react";

import GlobalFrame from "./GlobalFrame";
import CommandRail from "./CommandRail";
import ContextualInspector from "./ContextualInspector";
import WorkspaceRouter from "./WorkspaceRouter";
import HelpOverlay from "./HelpOverlay";
import ConfirmModal from "../components/primitives/ConfirmModal/ConfirmModal";
import { InspectorPortalProvider } from "./InspectorSlot";
import { PlaybackSnapshotProvider } from "./PlaybackSnapshotContext";
import { useGlobalKeyboardWorkflow } from "./useGlobalKeyboardWorkflow";
import { DEFAULT_DESTINATION, type DestinationId } from "./navigation";
import { GuidedFirstShowProvider, useGuidedFirstShow } from "../workspaces/show/GuidedFirstShow/GuidedFirstShowContext";
import GuidedFirstShow from "../workspaces/show/GuidedFirstShow/GuidedFirstShow";
import styles from "./AppShell.module.css";

function ShellCanvas({ active }: { active: DestinationId }) {
  const { open } = useGuidedFirstShow();
  return open ? <GuidedFirstShow /> : <WorkspaceRouter active={active} />;
}

// GuardedCommandRail intercepts CommandRail's own onSelect while the
// Guided First Show is open: previously a destination click while
// guiding silently did nothing (activeDestination changed underneath the
// still-open guide overlay, with zero visible effect) -- guide-navigation
// -guard pass. Now the rail visually dims (CommandRail's own `dimmed`
// prop) and a click prompts a ConfirmModal instead of navigating
// straight away; confirming calls the same `navigateTo` a stage's own
// primary action already uses to hand off to a real workspace, so
// leaving-via-nav-click and leaving-via-a-stage-action behave identically.
function GuardedCommandRail({ active }: { active: DestinationId }) {
  const { open, navigateTo } = useGuidedFirstShow();
  const [pendingDestination, setPendingDestination] = useState<DestinationId | null>(null);

  return (
    <>
      <CommandRail
        active={active}
        dimmed={open}
        onSelect={(destination) => {
          if (open) {
            setPendingDestination(destination);
          } else {
            navigateTo(destination);
          }
        }}
      />
      {pendingDestination && (
        <ConfirmModal
          title="Leave the guide?"
          message="You're leaving the Guided First Show before finishing. Your progress is kept, and you can resume from Overview later."
          confirmLabel="Leave Guide"
          cancelLabel="Stay in Guide"
          onConfirm={() => {
            const destination = pendingDestination;
            setPendingDestination(null);
            navigateTo(destination);
          }}
          onCancel={() => setPendingDestination(null)}
        />
      )}
    </>
  );
}

function ShellBody() {
  const [activeDestination, setActiveDestination] = useState<DestinationId>(DEFAULT_DESTINATION);
  const [inspectorContainer, setInspectorContainer] = useState<HTMLDivElement | null>(null);
  const { helpOpen, closeHelp } = useGlobalKeyboardWorkflow();

  return (
    <div className={styles.appShell}>
      <div className={styles.header}>
        <GlobalFrame />
      </div>
      <GuidedFirstShowProvider activeDestination={activeDestination} onNavigate={setActiveDestination}>
        <div className={styles.rail}>
          <GuardedCommandRail active={activeDestination} />
        </div>
        <main className={styles.main}>
          <InspectorPortalProvider container={inspectorContainer}>
            <ShellCanvas active={activeDestination} />
          </InspectorPortalProvider>
        </main>
      </GuidedFirstShowProvider>
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
