import { create } from "zustand";

import {
  offlineStatusSnapshot,
  type AppLogView,
  type StatusSnapshot,
} from "../lib/wailsBridge";

// maxAppLogEntries bounds the appLog slice -- distinct from (and much
// larger than) events.go's own maxStagedAppLogs, which only bounds one
// ~25ms flush tick's staging buffer. The oldest entries are dropped first
// so a long-running session's log history never grows without bound.
const maxAppLogEntries = 500;

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
  /** Bumped by OperatorSurface.tsx whenever its own CreateSurface/
   * RemoveSurface/AssignItem/UnassignItem calls change the show's operator
   * surfaces -- App.tsx mounts OperatorSurface.tsx and MidiPanel.tsx
   * permanently side by side (never as a tab that unmounts), and each owns
   * an independent SurfaceService.ListSurfaces() fetch with no shared
   * source of truth, so MidiPanel.tsx's own surface dropdown otherwise goes
   * stale the moment a surface is created/removed elsewhere on the same
   * page (only a full app restart re-fetches it). This is an invalidation
   * signal, not cached Go-pushed data, so it does not conflict with this
   * store's "never authoritative" rule above -- MidiPanel.tsx still
   * re-fetches from SurfaceService itself on every bump. */
  surfaceListVersion: number;
  bumpSurfaceListVersion: () => void;
  /** The accumulated "app:log" stream (internal/wails/events.go's
   * QueueAppLog, fed by App.logEvent/LogEvent) -- daemon-supervision,
   * hotkey-registration, and MIDI-driver lifecycle lines. Written by
   * AppLogStream.tsx (shell/, mounted unconditionally inside GlobalFrame,
   * mirroring LiveStatusBar's identical "always-mounted sole writer"
   * role): most of these lines fire during App.OnStartup, within the
   * first moments of the window opening, well before an operator has
   * necessarily navigated to the Diagnostics workspace -- if the
   * subscription lived in DiagnosticsWorkspace.tsx itself (as it
   * originally did) instead of here, every line pushed before that first
   * visit would already be gone (EventsEmit is fire-and-forget, never
   * replayed) and the workspace's log panel would appear permanently
   * empty. Bounded to maxAppLogEntries, oldest dropped first. */
  appLog: AppLogView[];
  appendAppLog: (event: AppLogView) => void;
  /** seedAppLog merges App.RecentAppLogs' backlog (fetchRecentAppLogs,
   * wailsBridge.ts) into appLog -- AppLogStream.tsx calls this once on
   * mount, covering "app:log" lines that fired (most do, during
   * App.OnStartup) before its own live subscription registered. Entries
   * already present (by seq -- a live push that arrived in the brief
   * window between subscribing and this backlog fetch resolving) are not
   * duplicated; the merged result is re-sorted by seq, since the backlog
   * and any already-live-received entries are not guaranteed to arrive in
   * a single already-ordered batch. */
  seedAppLog: (events: AppLogView[]) => void;
  clearAppLog: () => void;
  /** midiLearnMode is the global MIDI Learn toggle's own on/off state
   * (MidiLearnToggle.tsx, rendered in GlobalFrame.tsx next to
   * SafetyCluster) -- a deliberate exception to this store's "cache of
   * Go-pushed snapshots" rule (see file doc comment above): it is pure
   * client-side UI state, never pushed by the Go host, needed here only
   * because the toggle button and Desk.tsx (which reacts to it by
   * highlighting every fader) sit far apart in the component tree.
   * Mirrors surfaceListVersion's own precedent of a pragmatic
   * non-conforming field in this store. */
  midiLearnMode: boolean;
  setMidiLearnMode: (active: boolean) => void;
}

export const useGolcStore = create<GolcStoreState>((set) => ({
  connectionStatus: "connecting",
  setConnectionStatus: (status) => set({ connectionStatus: status }),
  status: offlineStatusSnapshot(),
  setStatus: (status) => set({ status }),
  surfaceListVersion: 0,
  bumpSurfaceListVersion: () => set((state) => ({ surfaceListVersion: state.surfaceListVersion + 1 })),
  appLog: [],
  appendAppLog: (event) =>
    set((state) => {
      const next = [...state.appLog, event];
      return { appLog: next.length > maxAppLogEntries ? next.slice(next.length - maxAppLogEntries) : next };
    }),
  seedAppLog: (events) =>
    set((state) => {
      // A "gap" entry (level "gap") never comes from RecentAppLogs -- only
      // ever synthesized live by events.go's flush on overflow -- but the
      // exclusion is kept here too, defensively, so a future backlog
      // source could never collide with a live-received gap's seq (always
      // 0, unset).
      const seen = new Set(state.appLog.filter((e) => e.level !== "gap").map((e) => e.seq));
      const fresh = events.filter((e) => e.level === "gap" || !seen.has(e.seq));
      if (fresh.length === 0) return {};
      const merged = [...fresh, ...state.appLog].sort((a, b) => a.seq - b.seq);
      return { appLog: merged.length > maxAppLogEntries ? merged.slice(merged.length - maxAppLogEntries) : merged };
    }),
  clearAppLog: () => set({ appLog: [] }),
  midiLearnMode: false,
  setMidiLearnMode: (active) => set({ midiLearnMode: active }),
}));
