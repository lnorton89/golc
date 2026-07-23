// PlaybackControls.tsx fills 06-04-PLAN.md Task 2's stub with real
// on-screen playback controls for the complete workflow (06-06-PLAN.md
// Task 2, PLAY-01): scene selector/switch, per-layer enable/disable
// toggles, numeric BPM entry + tap-tempo, and transport/evaluate preview.
// BPM/bar readouts use JetBrains Mono (06-UI-SPEC.md Typography). Every
// control calls the matching internal/wails.PlaybackService binding
// through the `dispatch` object exported below -- the SAME functions
// frontend/src/hooks/useKeyboardWorkflow.ts's keydown handlers call
// (06-06-PLAN.md key_link), so the on-screen and documented-keyboard
// surfaces (PLAY-01/PLAY-02) can never drift out of sync.
//
// This component owns the polled PlaybackService.GetState() snapshot
// (scenes/layers/BPM) and passes the derived sceneNames/activeSceneName/
// layerEnabled/bpm into useKeyboardWorkflow -- App.tsx is never modified
// to invoke the hook itself (06-04-PLAN.md Task 2's "never edit App.tsx's
// layout or mount points" contract).

import { useCallback, useEffect, useState, type CSSProperties } from "react";

import { useGolcStore } from "../../store/store";
import { useKeyboardWorkflow } from "../../hooks/useKeyboardWorkflow";
import KeyboardShortcuts from "../KeyboardShortcuts/KeyboardShortcuts";
import styles from "./PlaybackControls.module.css";

/** WailsResult mirrors internal/wails.Result's JSON tags exactly (0
 * success, 1 command failure, 2 routing/usage/startup failure). */
export interface WailsResult {
  exitCode: number;
  stdout: string;
  stderr: string;
}

/** The exact four fixed layer kinds every scene carries (mirrors
 * internal/scene/scene.go's layerPriority order). */
export const LAYER_KINDS = ["base_look", "color_theme", "chase", "motion"] as const;
export type LayerKind = (typeof LAYER_KINDS)[number];

export const LAYER_LABELS: Record<LayerKind, string> = {
  base_look: "Base Look",
  color_theme: "Color Theme",
  chase: "Chase",
  motion: "Motion",
};

interface LayerSummary {
  kind: LayerKind;
  enabled: boolean;
  ref?: string;
}

interface SceneSummary {
  name: string;
  active: boolean;
  barsPerLoop: number;
  layers: LayerSummary[];
}

export interface PlaybackStateSummary {
  bpm: number;
  scenes: SceneSummary[];
}

interface PlaybackServiceBinding {
  SwitchScene(sceneName: string): Promise<WailsResult>;
  SetLayerEnabled(sceneName: string, kind: string, enabled: boolean): Promise<WailsResult>;
  SetBPM(bpm: number): Promise<WailsResult>;
  TapTempo(timestamps: string[]): Promise<WailsResult>;
  Evaluate(at: number): Promise<WailsResult>;
  GetState(): Promise<WailsResult>;
}

// Wails v2 injects window.go.<goPackageName>.<StructName> at runtime for
// every struct bound via cmd/golc-desktop/main.go's options.App{Bind:
// [...]} -- internal/wails.PlaybackService's Go package name is "wails".
// The `Window.go.wails` global shape itself is declared once, centrally,
// in src/lib/wailsBridge.ts (see that file's comment) -- declaring it here
// too would collide with wailsBridge.ts's declaration under TypeScript's
// declaration-merging rules for the same inline-typed `go` property.
function playbackService(): PlaybackServiceBinding | undefined {
  return typeof window !== "undefined" ? window.go?.wails?.PlaybackService : undefined;
}

const TAP_RESET_GAP_MS = 2000;
const TAP_HISTORY_MAX = 8;

/** createTapTempoRecorder returns a single shared "record one tap"
 * closure: on-screen tap-tempo clicks and the keyboard workflow's Space
 * key both feed the SAME accumulating tap buffer (reset after a >2s gap),
 * so an operator can mix mouse and keyboard taps into one coherent tempo
 * rather than each surface keeping its own disjoint buffer. Fewer than
 * two accumulated taps is a no-op (internal/command/playback.go's
 * "playback bpm tap" route itself requires at least two). */
function createTapTempoRecorder() {
  let taps: string[] = [];
  let lastTapAt = 0;

  return async function recordTap(): Promise<WailsResult | undefined> {
    const now = Date.now();
    if (taps.length > 0 && now - lastTapAt > TAP_RESET_GAP_MS) {
      taps = [];
    }
    lastTapAt = now;
    taps.push(new Date(now).toISOString());
    if (taps.length > TAP_HISTORY_MAX) {
      taps = taps.slice(taps.length - TAP_HISTORY_MAX);
    }
    if (taps.length < 2) {
      return undefined;
    }
    return playbackService()?.TapTempo(taps);
  };
}

const recordTap = createTapTempoRecorder();

/** dispatch is the single source of truth for every playback action.
 * PlaybackControls' own on-screen controls below AND
 * useKeyboardWorkflow's keydown handlers both call these exact functions
 * -- never a second, parallel implementation -- so the on-screen surface
 * (PLAY-01) and the documented keyboard surface (PLAY-02) always reach
 * the identical workflow. */
export const dispatch = {
  async switchScene(sceneName: string): Promise<WailsResult | undefined> {
    return playbackService()?.SwitchScene(sceneName);
  },
  async setLayerEnabled(sceneName: string, kind: LayerKind, enabled: boolean): Promise<WailsResult | undefined> {
    return playbackService()?.SetLayerEnabled(sceneName, kind, enabled);
  },
  async setBPM(bpm: number): Promise<WailsResult | undefined> {
    return playbackService()?.SetBPM(bpm);
  },
  recordTap,
  async evaluate(at: number): Promise<WailsResult | undefined> {
    return playbackService()?.Evaluate(at);
  },
  async getState(): Promise<PlaybackStateSummary | undefined> {
    const result = await playbackService()?.GetState();
    if (!result || result.exitCode !== 0 || !result.stdout) {
      return undefined;
    }
    return JSON.parse(result.stdout) as PlaybackStateSummary;
  },
};

const STATE_POLL_INTERVAL_MS = 1000;

const panelStyle: CSSProperties = {
  opacity: 1,
};

export default function PlaybackControls() {
  const connectionStatus = useGolcStore((state) => state.connectionStatus);
  const loading = connectionStatus === "connecting";

  const [state, setState] = useState<PlaybackStateSummary | undefined>(undefined);
  const [bpmInput, setBpmInput] = useState("");
  const [evaluateAt, setEvaluateAt] = useState("0");
  const [previewOutput, setPreviewOutput] = useState("");
  const [showShortcuts, setShowShortcuts] = useState(false);

  const refreshState = useCallback(async () => {
    const next = await dispatch.getState();
    if (next) {
      setState(next);
      setBpmInput(String(next.bpm));
    }
  }, []);

  // WR-03 fix: only poll while the bridge/daemon is actually connected --
  // previously this ran unconditionally, so a disconnected daemon meant
  // dispatch.getState() resolved to undefined every second forever with no
  // backoff, the one outlier in this codebase that never changed cadence/
  // state for a disconnected daemon (LiveStatusBar's STATUS_GAP_MS re-query
  // and SafetyService's throttled push both do). Re-running this effect on
  // every connectionStatus change also means polling resumes immediately
  // (not up to STATE_POLL_INTERVAL_MS late) the moment the daemon
  // reconnects, rather than waiting for the next already-scheduled tick.
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

  const activeScene = state?.scenes.find((s) => s.active) ?? state?.scenes[0];
  const activeSceneName = activeScene?.name ?? null;
  const layerEnabled: Record<string, boolean> = {};
  for (const layer of activeScene?.layers ?? []) {
    layerEnabled[layer.kind] = layer.enabled;
  }
  const sceneNames = state?.scenes.map((s) => s.name) ?? [];
  const bpm = state?.bpm ?? 0;

  useKeyboardWorkflow({ sceneNames, activeSceneName, layerEnabled, bpm });

  const handleSwitchScene = useCallback(
    async (sceneName: string) => {
      await dispatch.switchScene(sceneName);
      await refreshState();
    },
    [refreshState],
  );

  const handleToggleLayer = useCallback(
    async (sceneName: string, kind: LayerKind, enabled: boolean) => {
      await dispatch.setLayerEnabled(sceneName, kind, enabled);
      await refreshState();
    },
    [refreshState],
  );

  const handleSetBpm = useCallback(async () => {
    const parsed = Number(bpmInput);
    if (!Number.isFinite(parsed)) {
      return;
    }
    await dispatch.setBPM(parsed);
    await refreshState();
  }, [bpmInput, refreshState]);

  const handleTap = useCallback(async () => {
    await dispatch.recordTap();
    await refreshState();
  }, [refreshState]);

  const handleEvaluate = useCallback(async () => {
    const parsed = Number(evaluateAt);
    if (!Number.isFinite(parsed)) {
      return;
    }
    const result = await dispatch.evaluate(parsed);
    setPreviewOutput(result?.stdout || result?.stderr || "");
  }, [evaluateAt]);

  return (
    <section
      className={styles.panel}
      style={{ ...panelStyle, opacity: loading ? 0.5 : 1 }}
      aria-label="Playback controls"
      aria-busy={loading}
    >
      {loading ? (
        "Loading playback controls…"
      ) : (
        <>
          <h2 className={styles.heading}>Playback</h2>

          <div className={styles.section}>
            <span className={styles.label}>Tempo</span>
            <div className={styles.row}>
              <input
                className={styles.bpmInput}
                type="number"
                min={1}
                step="0.1"
                aria-label="BPM"
                value={bpmInput}
                onChange={(event) => setBpmInput(event.target.value)}
              />
              <button type="button" className={styles.button} onClick={() => void handleSetBpm()}>
                Set BPM
              </button>
              <button type="button" className={styles.button} onClick={() => void handleTap()}>
                Tap Tempo
              </button>
              <span className={styles.readout}>{bpm} BPM</span>
            </div>
          </div>

          <div className={styles.section}>
            <span className={styles.label}>Scenes</span>
            {sceneNames.length === 0 ? (
              <span className={styles.emptyState}>No scenes yet — create one from the authoring view.</span>
            ) : (
              <div className={styles.sceneList}>
                {state?.scenes.map((scene) => (
                  <button
                    key={scene.name}
                    type="button"
                    className={scene.active ? styles.sceneButtonActive : styles.sceneButton}
                    aria-pressed={scene.active}
                    onClick={() => void handleSwitchScene(scene.name)}
                  >
                    {scene.name}
                  </button>
                ))}
              </div>
            )}
          </div>

          <div className={styles.section}>
            <span className={styles.label}>Layers{activeSceneName ? ` — ${activeSceneName}` : ""}</span>
            {!activeScene ? (
              <span className={styles.emptyState}>Switch to a scene to control its layers.</span>
            ) : (
              <div className={styles.layerGrid}>
                {LAYER_KINDS.map((kind) => {
                  const layer = activeScene.layers.find((l) => l.kind === kind);
                  const enabled = layer?.enabled ?? false;
                  return (
                    <button
                      key={kind}
                      type="button"
                      className={enabled ? styles.layerToggleOn : styles.layerToggle}
                      aria-pressed={enabled}
                      onClick={() => void handleToggleLayer(activeScene.name, kind, !enabled)}
                    >
                      {LAYER_LABELS[kind]}
                    </button>
                  );
                })}
              </div>
            )}
          </div>

          <div className={styles.section}>
            <span className={styles.label}>Transport / Evaluate</span>
            <div className={styles.row}>
              <input
                className={styles.bpmInput}
                type="number"
                step="0.01"
                aria-label="Evaluate position (bar.beatfraction)"
                value={evaluateAt}
                onChange={(event) => setEvaluateAt(event.target.value)}
              />
              <button type="button" className={styles.buttonPrimary} onClick={() => void handleEvaluate()}>
                Evaluate
              </button>
            </div>
            {previewOutput && <pre className={styles.previewOutput}>{previewOutput}</pre>}
          </div>

          <div className={styles.section}>
            <div className={styles.row}>
              <button
                type="button"
                className={styles.button}
                aria-expanded={showShortcuts}
                onClick={() => setShowShortcuts((current) => !current)}
              >
                {showShortcuts ? "Hide Keyboard Shortcuts" : "Keyboard Shortcuts"}
              </button>
            </div>
            {showShortcuts && <KeyboardShortcuts />}
          </div>
        </>
      )}
    </section>
  );
}
