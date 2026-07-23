import { create } from "zustand";

import {
  offlineStatusSnapshot,
  type StatusSnapshot,
} from "../lib/wailsBridge";

// store.ts is the Zustand cache of Go-pushed snapshots (06-RESEARCH.md
// Recommended Project Structure: "store/ -- Zustand: cache of Go-pushed
// snapshots, never authoritative"). Every field here is a projection of
// state the Go host last pushed via runtime.EventsEmit
// (internal/wails/events.go's throttled pushStatus scaffold) -- this
// store is never the source of truth for playback/safety state, and no
// action here should mutate application state without a corresponding
// Go-bound call (06-RESEARCH.md Pitfall 1 / Anti-Pattern: "Treating Wails
// EventsEmit as ... source of truth").
//
// 06-05 (status bar/safety), 06-06 (playback), 06-07 (operator surface),
// and 06-08 (MIDI) each add their own slice to this store; this scaffold
// declares only the shared "daemon connection" status every slice's
// loading/error UI-SPEC state depends on. 06-05-PLAN.md Task 2 adds the
// `status` slice below: PLAY-07's live status projection, written by
// LiveStatusBar.tsx's own EventsOn subscription + FetchStatus gap-query
// (never written directly by a component render), and read by
// LiveStatusBar.tsx (and, in a later plan, SafetyCluster.tsx's own
// active/idle visual state).

export type ConnectionStatus = "connecting" | "connected" | "unreachable";

export interface GolcStoreState {
  /** Whether the Go host has completed its first daemon status fetch
   * (06-UI-SPEC.md loading backstop: lists/status regions render a
   * skeleton/dim placeholder until this flips to "connected"). */
  connectionStatus: ConnectionStatus;
  setConnectionStatus: (status: ConnectionStatus) => void;
  /** The most recently received PLAY-07 status projection -- a cache of
   * the Go host's last throttled "status:update" push (or the last
   * FetchStatus gap-query result), never authoritative on its own
   * (06-RESEARCH.md anti-pattern). Starts at the same explicit
   * offline/idle projection FetchStatus itself falls back to, so a
   * component reading this before the first update/fetch resolves still
   * sees explicit idle values, never undefined. */
  status: StatusSnapshot;
  setStatus: (status: StatusSnapshot) => void;
}

export const useGolcStore = create<GolcStoreState>((set) => ({
  connectionStatus: "connecting",
  setConnectionStatus: (status) => set({ connectionStatus: status }),
  status: offlineStatusSnapshot(),
  setStatus: (status) => set({ status }),
}));
