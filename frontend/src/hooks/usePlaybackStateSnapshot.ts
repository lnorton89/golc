// usePlaybackStateSnapshot extracts PlaybackControls.tsx's original
// GetState() polling loop (shell restructure Step 5) so every consumer
// (TempoControls in the persistent GlobalFrame, and the Operate workspace
// Launcher once Step 7 lands) shares the same poll cadence/shape rather
// than each hand-rolling its own. Only polls while the daemon is actually
// connected (WR-03 fix, preserved verbatim from the original component).
//
// The loop is now Query's refetchInterval rather than a hand-rolled
// window.setInterval. `enabled` reproduces the connection gate exactly: a
// disabled query neither fetches nor schedules, and it keeps whatever it
// last cached, so disconnecting still leaves the last known state on screen
// instead of blanking the transport readout.
import { useQuery } from "@tanstack/react-query";

import { useGolcStore } from "../store/store";
import { queryKeys } from "../lib/queryKeys";
import { dispatch, type PlaybackStateSummary } from "../lib/playbackDispatch";

const STATE_POLL_INTERVAL_MS = 1000;

export interface PlaybackStateSnapshot {
  state: PlaybackStateSummary | undefined;
  refreshState: () => Promise<void>;
}

export function usePlaybackStateSnapshot(): PlaybackStateSnapshot {
  const connectionStatus = useGolcStore((store) => store.connectionStatus);

  const query = useQuery({
    queryKey: queryKeys.playback.snapshot(),
    // dispatch.getState() answers undefined for every failure mode (no
    // bound service, non-zero exit, empty stdout). The original loop
    // treated that as "keep the last known state" via `if (next)`, never
    // overwriting a good snapshot with a blank one. Throwing here
    // reproduces exactly that: a rejected fetch leaves `data` holding the
    // previous success, so the readout freezes on the last real value
    // rather than flickering to empty on one dropped poll.
    queryFn: async () => {
      const next = await dispatch.getState();
      if (!next) throw new Error("GOLC_PLAYBACK_STATE_UNAVAILABLE");
      return next;
    },
    enabled: connectionStatus === "connected",
    refetchInterval: STATE_POLL_INTERVAL_MS,
  });

  return {
    state: query.data,
    refreshState: async () => {
      await query.refetch();
    },
  };
}
