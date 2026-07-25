// Launcher is the "operate" mode's Launcher + Masters composition
// (live-operation-safety-midi.md, Sketch 003 Variant A) -- shell
// restructure plan Step 7. Assigned scenes render as ScenePad grid cells;
// unassigned scenes remain visible, dimmed, and locked (never dispatch).
// The active scene's four layers stay visible in a compact strip below the
// grid. Real dispatch (dispatch.switchScene/setLayerEnabled) is completely
// independent of this surface's own assignment tracking -- assignment
// only gates whether THIS UI lets an operator reach a control; the actual
// enforcement is server-side (AuthorizeControl), per OperatorSurface.tsx's
// existing doctrine this component does not change.
//
// Group/Grand Master controls can be ASSIGNED to a surface (ControlKind
// "master") but there is no Wails-bound way to actually set a master's
// live level yet -- no such method exists on any bound service. Rather
// than build an interactive slider against a capability that doesn't
// exist (out of this round's scope per the shell restructure plan's
// explicit "no new Go services" non-goal), assigned masters are shown as
// an honest, non-interactive "assigned, not yet controllable" note.
import { useCallback } from "react";

import type { ControlRefView } from "./OperatorSurface";
import ScenePad from "./ScenePad";
import Button from "../primitives/Button/Button";
import { usePlaybackSnapshot } from "../../shell/PlaybackSnapshotContext";
import { dispatch, LAYER_KINDS, LAYER_LABELS, type LayerKind } from "../../lib/playbackDispatch";
import styles from "./Launcher.module.css";

interface LauncherProps {
  controls: ControlRefView[];
}

export default function Launcher({ controls }: LauncherProps) {
  const { state, refreshState } = usePlaybackSnapshot();

  const handleLaunch = useCallback(
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

  const sceneControls = controls.filter((control) => control.kind === "scene" && control.scene);
  const masterControls = controls.filter((control) => control.kind === "master");
  const activeScene = state?.scenes.find((scene) => scene.active) ?? null;
  const activeSceneLayerControls = activeScene
    ? controls.filter((control) => control.kind === "layer" && control.scene === activeScene.name)
    : [];

  return (
    <div className={styles.launcher}>
      <div className={styles.section}>
        <span className={styles.label}>Scenes</span>
        {sceneControls.length === 0 ? (
          <p className={styles.emptyState}>No scenes assigned to this surface yet.</p>
        ) : (
          <div className={styles.grid}>
            {sceneControls.map((control) => {
              const sceneName = control.scene ?? control.label;
              const isLive = state?.scenes.some((scene) => scene.name === sceneName && scene.active) ?? false;
              return (
                <ScenePad
                  key={sceneName}
                  name={sceneName}
                  live={isLive}
                  locked={!control.assigned}
                  onLaunch={() => void handleLaunch(sceneName)}
                />
              );
            })}
          </div>
        )}
      </div>

      <div className={styles.section}>
        <span className={styles.label}>Layers{activeScene ? ` — ${activeScene.name}` : ""}</span>
        {!activeScene ? (
          <p className={styles.emptyState}>No scene is live yet.</p>
        ) : (
          <div className={styles.layerStrip}>
            {LAYER_KINDS.map((kind) => {
              const layer = activeScene.layers.find((candidate) => candidate.kind === kind);
              const enabled = layer?.enabled ?? false;
              const control = activeSceneLayerControls.find((candidate) => candidate.layerKind === kind);
              const locked = control ? !control.assigned : false;
              return (
                <Button
                  key={kind}
                  variant={enabled ? "primary" : "secondary"}
                  aria-pressed={enabled}
                  disabled={locked}
                  title={locked ? `${LAYER_LABELS[kind]} (locked — not assigned to this surface)` : undefined}
                  onClick={() => void handleToggleLayer(activeScene.name, kind, !enabled)}
                >
                  {LAYER_LABELS[kind]}
                </Button>
              );
            })}
          </div>
        )}
      </div>

      {masterControls.length > 0 ? (
        <div className={styles.section}>
          <span className={styles.label}>Masters</span>
          <div className={styles.masterRow}>
            {masterControls.map((control) => (
              <span key={control.label} className={styles.masterChip} title="Assigned — live control not available yet">
                {control.label}
              </span>
            ))}
          </div>
        </div>
      ) : null}
    </div>
  );
}
