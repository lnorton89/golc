// LiveStatusBar.tsx is the fixed-height, persistent chrome region showing
// active scene, enabled layers, BPM/bar position, controlling source, and
// final output state (PLAY-07, 06-UI-SPEC.md "Live status bar ... fixed
// height, not built from the standard scale -- treat as a locked chrome
// region"). On mount it fetches an authoritative baseline via
// fetchSafetyStatus, subscribes to the Go host's throttled "status:update"
// push (onStatusUpdate, wailsBridge.ts) to stay current, and re-queries
// fetchSafetyStatus again if no push arrives within STATUS_GAP_MS -- the
// store's `status` slice is therefore always a cache of the Go-pushed/
// fetched snapshot, never authoritative on its own (06-RESEARCH.md
// anti-pattern: "Treating Wails EventsEmit as ... source of truth"). When
// no scene is active (or the daemon is unreachable), every field renders
// the same explicit idle value ("--") rather than a blank/undefined one
// (PLAY-07 idle edge, D-04 "visible not hidden"); scene/layer names
// truncate with ellipsis at a fixed column width with the full name on
// hover via the native `title` attribute, and this bar's own height never
// grows to accommodate a long name (06-UI-SPEC.md overflow rule).
//
// 13-15 design-system migration: Source/Output now render through the
// shared `Chip` primitive (its per-tone icon already satisfies "status is
// non-color-only" -- ChipTone's vocabulary is byte-identical to this
// bridge's own controllingSource/outputState strings) instead of a local,
// color-only StatusChip.

import { useEffect } from "react";
import { TriangleAlert } from "lucide-react";

import { useGolcStore } from "../../store/store";
import {
  fetchSafetyStatus,
  onStatusUpdate,
  type StatusSnapshot,
} from "../../lib/wailsBridge";
import Chip, { type ChipTone } from "../primitives/Chip/Chip";
import styles from "./LiveStatusBar.module.css";

// STATUS_GAP_MS bounds how long LiveStatusBar waits with no "status:update"
// push before re-querying fetchSafetyStatus directly -- several push
// cadences (internal/wails events.go's own eventsTickInterval/
// statusPollInterval, 25ms), not one, so ordinary scheduler jitter never
// falsely triggers a re-query (mirrors internal/artnet/health.go's
// frameStaleAfter "several ticks, not one" convention).
const STATUS_GAP_MS = 2000;

// BEATS_PER_BAR mirrors internal/playback/clock.go's fixed 4/4 time
// signature constant (beatsPerBar) -- beatFraction is a fraction of one
// bar, so this converts it to a 1-based beat number within that bar.
const BEATS_PER_BAR = 4;

// KNOWN_TONES is the daemon's fixed controllingSource/outputState
// vocabulary (06-UI-SPEC.md Status Vocabulary: live/frame-lock/armed/
// revoked/blackout/offline), byte-identical to Chip's own ChipTone union.
// An unrecognized value (should never happen against a well-behaved
// daemon) falls back to "neutral" rather than an undefined/blank tone.
const KNOWN_TONES: ReadonlySet<string> = new Set([
  "live",
  "frame-lock",
  "armed",
  "revoked",
  "blackout",
  "offline",
]);

function toChipTone(value: string): ChipTone {
  return (KNOWN_TONES.has(value) ? value : "neutral") as ChipTone;
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
  const sceneName = status.active ? status.sceneName || "Unnamed scene" : "--";
  const layersText =
    status.active && status.enabledLayers.length > 0
      ? status.enabledLayers.join(", ")
      : status.active
        ? "No layers enabled"
        : "--";
  const barText = status.active
    ? `${status.barIndex + 1}:${Math.floor(status.beatFraction * BEATS_PER_BAR) + 1}`
    : "--";

  return (
    <div
      className={styles.bar}
      aria-label="Live status bar"
      aria-busy={loading}
      style={{ opacity: loading ? 0.5 : 1 }}
    >
      <div className={styles.metric}>
        <span className={styles.metricLabel}>Scene</span>
        <span
          className={`${styles.metricValue} ${styles.truncate}`}
          title={sceneName}
        >
          {sceneName}
        </span>
      </div>

      <div className={styles.metric}>
        <span className={styles.metricLabel}>Layers</span>
        <span
          className={`${styles.metricValue} ${styles.layersValue}`}
          title={layersText}
        >
          {layersText}
        </span>
      </div>

      <div className={styles.metric}>
        <span className={styles.metricLabel}>Bar</span>
        <span className={styles.metricValue}>{barText}</span>
      </div>

      <div className={styles.spacer} />

      {!status.reachable && (
        <span className={styles.unreachableCopy}>
          <TriangleAlert size={14} className={styles.unreachableIcon} aria-hidden="true" />
          Can&rsquo;t reach the playback engine. GOLC will try to reconnect
          automatically — Blackout and Stop/Release-All remain available.
        </span>
      )}

      <span className={styles.statusChip} title={`Source: ${status.controllingSource}`}>
        <Chip tone={toChipTone(status.controllingSource)}>{status.controllingSource}</Chip>
      </span>
      <span className={styles.statusChip} title={`Output: ${status.outputState}`}>
        <Chip tone={toChipTone(status.outputState)}>{status.outputState}</Chip>
      </span>
    </div>
  );
}
