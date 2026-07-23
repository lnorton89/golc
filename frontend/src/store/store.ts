import { create } from "zustand";

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
// loading/error UI-SPEC state depends on.

export type ConnectionStatus = "connecting" | "connected" | "unreachable";

export interface GolcStoreState {
  /** Whether the Go host has completed its first daemon status fetch
   * (06-UI-SPEC.md loading backstop: lists/status regions render a
   * skeleton/dim placeholder until this flips to "connected"). */
  connectionStatus: ConnectionStatus;
  setConnectionStatus: (status: ConnectionStatus) => void;
}

export const useGolcStore = create<GolcStoreState>((set) => ({
  connectionStatus: "connecting",
  setConnectionStatus: (status) => set({ connectionStatus: status }),
}));
