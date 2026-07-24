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
import { dispatch, LAYER_KINDS, LAYER_LABELS, type LayerKind, type PlaybackStateSummary } from "../../lib/playbackDispatch";
import KeyboardShortcuts from "../KeyboardShortcuts/KeyboardShortcuts";
import styles from "./PlaybackControls.module.css";

export type { WailsResult, LayerKind, PlaybackStateSummary } from "../../lib/playbackDispatch";
export { dispatch, LAYER_KINDS, LAYER_LABELS } from "../../lib/playbackDispatch";

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
