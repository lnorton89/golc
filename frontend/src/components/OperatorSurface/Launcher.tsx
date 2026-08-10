// Launcher is the "operate" mode's Launcher + Masters composition
// (live-operation-safety-midi.md, Sketch 003 Variant A) -- shell
// restructure plan Step 7, retargeted onto the shared design system
// (unified design system phase, 13-14-PLAN.md Task 2). Assigned scenes
// render as ScenePad grid cells; unassigned scenes remain visible, dimmed,
// and locked (never dispatch). The active scene's four layers stay
// visible in a compact strip below the grid, now composed from the
// shared Button primitive (via the public design-system barrel instead of
// a direct primitive import). Real dispatch
// (dispatch.switchScene/setLayerEnabled) is completely independent of
// this surface's own assignment tracking -- assignment only gates whether
// THIS UI lets an operator reach a control; the actual enforcement is
// server-side (AuthorizeControl), per OperatorSurface.tsx's existing
// doctrine this component does not change.
//
// Group/Grand Master controls can be ASSIGNED to a surface (ControlKind
// "master") but there is no Wails-bound way to actually set a master's
// live level yet -- no such method exists on any bound service. Rather
// than build an interactive slider against a capability that doesn't
// exist (out of this round's scope per the shell restructure plan's
// explicit "no new Go services" non-goal), assigned masters now render
// through the shared LauncherMasters pattern -- a disabled, compact
// "Not controllable" button per assigned master -- the same honest,
// non-interactive treatment the prior local masterChip markup gave, just
// composed from the shared pattern instead of a locally reinvented class.
import { useCallback, useState } from "react";
import { Layers, Play } from "lucide-react";

import type { ControlRefView } from "./OperatorSurface";
import ScenePad from "./ScenePad";
import { Button, EmptyState, ErrorState, LauncherMasters } from "../../design-system";
import { usePlaybackSnapshot } from "../../shell/PlaybackSnapshotContext";
import { dispatch, LAYER_KINDS, LAYER_LABELS, type LayerKind, type WailsResult } from "../../lib/playbackDispatch";
import styles from "./Launcher.module.css";

interface LauncherProps {
  controls: ControlRefView[];
}

/** failureMessage turns a dispatch result into operator-facing copy, or
 * null when the dispatch succeeded. An absent result means the bridge
 * isn't bound at all (wailsBridge's documented degraded contract), which
 * is itself a failure worth showing on this surface. */
function failureMessage(result: WailsResult | undefined, action: string): string | null {
  if (!result) {
    return `${action} failed: not connected to the GOLC host.`;
  }
  if (result.exitCode === 0) {
    return null;
  }
  return `${action} failed: ${result.stderr.trim() || `exit code ${result.exitCode}`}`;
}

export default function Launcher({ controls }: LauncherProps) {
  const { state, refreshState } = usePlaybackSnapshot();
  // This is the surface handed to a player mid-show, so a rejected launch
  // (daemon unreachable, or a server-side AuthorizeControl rejection under
  // D-04) must never look like nothing happened. Both handlers used to
  // discard the returned WailsResult entirely and the component rendered
  // no error surface of any kind.
  const [error, setError] = useState<string | null>(null);
  // pendingLayers gates a layer button while its own toggle is in flight:
  // the target state is computed from a snapshot that only refreshes once
  // a second, so a rapid second click read the same pre-toggle value and
  // re-sent the same target.
  const [pendingLayers, setPendingLayers] = useState<ReadonlySet<string>>(new Set());

  const handleLaunch = useCallback(
    async (sceneName: string) => {
      const result = await dispatch.switchScene(sceneName);
      setError(failureMessage(result, `Launching ${sceneName}`));
      await refreshState();
    },
    [refreshState],
  );

  const handleToggleLayer = useCallback(
    async (sceneName: string, kind: LayerKind, enabled: boolean) => {
      setPendingLayers((current) => new Set(current).add(kind));
      try {
        const result = await dispatch.setLayerEnabled(sceneName, kind, enabled);
        setError(failureMessage(result, `${LAYER_LABELS[kind]} toggle`));
        await refreshState();
      } finally {
        setPendingLayers((current) => {
          const next = new Set(current);
          next.delete(kind);
          return next;
        });
      }
    },
    [refreshState],
  );

  const sceneControls = controls.filter((control) => control.kind === "scene" && control.scene);
  const masterControls = controls.filter((control) => control.kind === "master");
  const activeScene = state?.scenes?.find((scene) => scene.active) ?? null;
  const activeSceneLayerControls = activeScene
    ? controls.filter((control) => control.kind === "layer" && control.scene === activeScene.name)
    : [];

  return (
    <div className={styles.launcher}>
      {error ? <ErrorState heading="Dispatch failed" message={error} /> : null}

      <div className={styles.section}>
        <span className={styles.label}>Scenes</span>
        {sceneControls.length === 0 ? (
          <EmptyState icon={Layers}>No scenes assigned to this surface yet.</EmptyState>
        ) : (
          <div className={styles.grid}>
            {sceneControls.map((control) => {
              const sceneName = control.scene ?? control.label;
              const isLive = state?.scenes?.some((scene) => scene.name === sceneName && scene.active) ?? false;
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
          <EmptyState icon={Play}>No scene is live yet.</EmptyState>
        ) : (
          <div className={styles.layerStrip}>
            {LAYER_KINDS.map((kind) => {
              const layer = activeScene.layers.find((candidate) => candidate.kind === kind);
              const enabled = layer?.enabled ?? false;
              const control = activeSceneLayerControls.find((candidate) => candidate.layerKind === kind);
              // Absent control entry => locked, matching the scene-pad
              // path above and this component's own "unassigned … locked
              // (never dispatch)" contract. This is NOT unreachable
              // defensive code: ShowSurface enumerates all four layer
              // kinds for every scene *in the show file at fetch time*
              // (svc_surface.go's surfaceLayerKindOrder), while
              // `activeScene` comes from usePlaybackSnapshot's separate 1s
              // poll -- so a scene created after this surface's detail was
              // fetched (another session, the CLI, an SDK script) is live
              // with no layer controls listed for it, and every layer
              // button used to render enabled and dispatch.
              const locked = !control?.assigned;
              return (
                <Button
                  key={kind}
                  variant={enabled ? "primary" : "secondary"}
                  aria-pressed={enabled}
                  disabled={locked || pendingLayers.has(kind)}
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
          <LauncherMasters
            masters={masterControls.map((control) => ({
              id: control.label,
              name: control.label,
              value: "Not controllable",
              disabled: true,
            }))}
          />
        </div>
      ) : null}
    </div>
  );
}
