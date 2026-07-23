// LiveStatusBar.tsx is the fixed-height, persistent chrome region showing
// active scene, enabled layers, BPM/bar position, controlling source, and
// final output state (PLAY-07, 06-UI-SPEC.md "Live status bar ... fixed
// height, not built from the standard scale -- treat as a locked chrome
// region"). 06-05-PLAN.md Task 2 fills this Wave 2 stub: on mount it
// fetches an authoritative baseline via fetchSafetyStatus, subscribes to
// the Go host's throttled "status:update" push (onStatusUpdate,
// wailsBridge.ts) to stay current, and re-queries fetchSafetyStatus again
// if no push arrives within STATUS_GAP_MS -- the store's `status` slice is
// therefore always a cache of the Go-pushed/fetched snapshot, never
// authoritative on its own (06-RESEARCH.md anti-pattern: "Treating Wails
// EventsEmit as ... source of truth"). When no scene is active (or the
// daemon is unreachable), every field renders an explicit idle value
// ("No active scene", "--") rather than a blank/undefined one (PLAY-07
// idle edge, D-04 "visible not hidden"); scene/layer names truncate with
// ellipsis at a fixed column width with the full name on hover via the
// native `title` attribute, and this bar's own height never grows to
// accommodate a long name (06-UI-SPEC.md overflow rule).

import { useEffect, type CSSProperties } from "react";

import { useGolcStore } from "../../store/store";
import {
  fetchSafetyStatus,
  onStatusUpdate,
  type StatusSnapshot,
} from "../../lib/wailsBridge";
import styles from "./LiveStatusBar.module.css";

// STATUS_GAP_MS bounds how long LiveStatusBar waits with no "status:update"
// push before re-querying fetchSafetyStatus directly -- several push
// cadences (internal/wails events.go's own eventsTickInterval/
// statusPollInterval, 25ms), not one, so ordinary scheduler jitter never
// falsely triggers a re-query (mirrors internal/artnet/health.go's
// frameStaleAfter "several ticks, not one" convention).
const STATUS_GAP_MS = 2000;

// STATUS_COLOR_VAR maps the daemon's fixed controllingSource/outputState
// vocabulary (06-UI-SPEC.md Status Vocabulary: live/frame-lock/armed/
// revoked/blackout/offline) to this app's brand CSS custom properties
// (index.css). An unrecognized value (should never happen against a
// well-behaved daemon) falls back to --muted rather than an undefined/
// blank color.
const STATUS_COLOR_VAR: Record<string, string> = {
  live: "var(--status-live)",
  "frame-lock": "var(--status-frame-lock)",
  armed: "var(--status-armed)",
  revoked: "var(--status-revoked)",
  blackout: "var(--status-blackout)",
  offline: "var(--status-offline)",
};

function statusColor(value: string): string {
  return STATUS_COLOR_VAR[value] ?? "var(--muted)";
}

function StatusChip({ label, value }: { label: string; value: string }) {
  const color = statusColor(value);
  return (
    <span
      className={styles.chip}
      style={{ "--chip-color": color } as CSSProperties}
      title={`${label}: ${value}`}
    >
      <span className={styles.chipDot} aria-hidden="true" />
      {value}
    </span>
  );
}

export default function LiveStatusBar() {
  const status = useGolcStore((state) => state.status);
  const setStatus = useGolcStore((state) => state.setStatus);
  const connectionStatus = useGolcStore((state) => state.connectionStatus);
  const setConnectionStatus = useGolcStore(
    (state) => state.setConnectionStatus,
  );

  useEffect(() => {
    let cancelled = false;
    let lastUpdateAt = Date.now();

    const applySnapshot = (snapshot: StatusSnapshot) => {
      if (cancelled) return;
      lastUpdateAt = Date.now();
      setStatus(snapshot);
      setConnectionStatus(snapshot.reachable ? "connected" : "unreachable");
    };

    fetchSafetyStatus().then(applySnapshot);
    const unsubscribe = onStatusUpdate(applySnapshot);

    const gapCheck = window.setInterval(() => {
      if (Date.now() - lastUpdateAt > STATUS_GAP_MS) {
        fetchSafetyStatus().then(applySnapshot);
      }
    }, STATUS_GAP_MS);

    return () => {
      cancelled = true;
      unsubscribe();
      window.clearInterval(gapCheck);
    };
  }, [setStatus, setConnectionStatus]);

  const loading = connectionStatus === "connecting";
  const sceneName = status.active ? status.sceneName || "Unnamed scene" : "No active scene";
  const layersText =
    status.active && status.enabledLayers.length > 0
      ? status.enabledLayers.join(", ")
      : status.active
        ? "No layers enabled"
        : "--";
  const bpmText = status.active ? status.bpm.toFixed(0) : "--";
  const barText = status.active
    ? `${status.barIndex + 1}.${Math.floor(status.beatFraction * 100)
        .toString()
        .padStart(2, "0")}`
    : "--";

  return (
    <div
      className={styles.bar}
      aria-label="Live status bar"
      aria-busy={loading}
      style={{ opacity: loading ? 0.5 : 1 }}
    >
      <div className={styles.field}>
        <span className={styles.fieldLabel}>Scene</span>
        <span
          className={`${styles.fieldValue} ${styles.truncate}`}
          title={sceneName}
        >
          {sceneName}
        </span>
      </div>

      <div className={styles.field}>
        <span className={styles.fieldLabel}>Layers</span>
        <span
          className={`${styles.fieldValue} ${styles.layersValue}`}
          title={layersText}
        >
          {layersText}
        </span>
      </div>

      <div className={styles.field}>
        <span className={styles.fieldLabel}>BPM</span>
        <span className={styles.fieldValue}>{bpmText}</span>
      </div>

      <div className={styles.field}>
        <span className={styles.fieldLabel}>Bar</span>
        <span className={styles.fieldValue}>{barText}</span>
      </div>

      <div className={styles.spacer} />

      {!status.reachable && (
        <span className={styles.unreachableCopy}>
          Can&rsquo;t reach the playback engine. GOLC will try to reconnect
          automatically — Blackout and Stop/Release-All remain available.
        </span>
      )}

      <StatusChip label="Source" value={status.controllingSource} />
      <StatusChip label="Output" value={status.outputState} />
    </div>
  );
}
