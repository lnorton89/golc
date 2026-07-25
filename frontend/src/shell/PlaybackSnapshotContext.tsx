// PlaybackSnapshotContext shares ONE usePlaybackStateSnapshot() poll loop
// across every consumer that needs it -- TempoControls (persistent
// GlobalFrame), useGlobalKeyboardWorkflow (persistent, shell-level), and
// the Operate workspace Launcher from shell restructure plan Step 7.
// Without this, TempoControls and the keyboard hook (both unconditionally,
// permanently mounted together) would each run an independent 1s
// PlaybackService.GetState() poll against the exact same data.
import { createContext, useContext, type ReactNode } from "react";

import { usePlaybackStateSnapshot, type PlaybackStateSnapshot } from "../hooks/usePlaybackStateSnapshot";

const PlaybackSnapshotContext = createContext<PlaybackStateSnapshot | null>(null);

export function PlaybackSnapshotProvider({ children }: { children: ReactNode }) {
  const snapshot = usePlaybackStateSnapshot();
  return <PlaybackSnapshotContext.Provider value={snapshot}>{children}</PlaybackSnapshotContext.Provider>;
}

export function usePlaybackSnapshot(): PlaybackStateSnapshot {
  const ctx = useContext(PlaybackSnapshotContext);
  if (!ctx) {
    throw new Error("usePlaybackSnapshot must be used within a PlaybackSnapshotProvider");
  }
  return ctx;
}
