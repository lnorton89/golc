// usePlaybackStateSnapshot extracts PlaybackControls.tsx's original
// GetState() polling loop (shell restructure Step 5) so every consumer
// (TempoControls in the persistent GlobalFrame, and the Operate workspace
// Launcher once Step 7 lands) shares the same poll cadence/shape rather
// than each hand-rolling its own. Only polls while the daemon is actually
// connected (WR-03 fix, preserved verbatim from the original component).
import { useCallback, useEffect, useState } from "react";

import { useGolcStore } from "../store/store";
import { dispatch, type PlaybackStateSummary } from "../lib/playbackDispatch";

const STATE_POLL_INTERVAL_MS = 1000;

export interface PlaybackStateSnapshot {
  state: PlaybackStateSummary | undefined;
  refreshState: () => Promise<void>;
}

export function usePlaybackStateSnapshot(): PlaybackStateSnapshot {
  const connectionStatus = useGolcStore((store) => store.connectionStatus);
  const [state, setState] = useState<PlaybackStateSummary | undefined>(undefined);

  const refreshState = useCallback(async () => {
    const next = await dispatch.getState();
    if (next) {
      setState(next);
    }
  }, []);

  useEffect(() => {
    if (connectionStatus !== "connected") {
      return;
    }
    void refreshState();
    const interval = window.setInterval(() => {
      void refreshState();
    }, STATE_POLL_INTERVAL_MS);
    return () => window.clearInterval(interval);
  }, [refreshState, connectionStatus]);

  return { state, refreshState };
}
