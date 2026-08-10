// ScenesLooksWorkspace is the Scene Stack (programming-scene-authoring.md,
// Sketch 002 Variant A) -- shell restructure plan Step 6. Owns every
// ProgrammingService call and all programming state (moved verbatim from
// the old flat SceneProgramming.tsx, now split into presentational
// children: SceneList (left nav), LayerRow x4 (selected scene's fixed
// layers), LookBrowser (published into the contextual inspector), and
// BarTimelinePanel (bottom evaluation panel, absorbing PlaybackControls'
// old Transport/Evaluate control). No ProgrammingService call changed.
import { useCallback, useEffect, useState, type CSSProperties } from "react";
import { Layers, Zap } from "lucide-react";

import {
  activateScene,
  assertOk,
  createBlend,
  createChase,
  createMotion,
  createScene,
  createTheme,
  deleteBlend,
  deleteChase,
  deleteMotion,
  deletePreset,
  deleteScene,
  deleteTheme,
  errorMessage,
  listProgramming,
  offlineProgrammingView,
  programmerSet,
  recordPreset,
  renameBlend,
  renameMotion,
  renamePreset,
  renameScene,
  renameTheme,
  reorderScenes,
  setSceneLayer,
  updateChase,
  type ProgChaseView,
  type ProgLookView,
  type ProgPresetView,
  type ProgrammingView,
  type ProgSceneView,
} from "../../lib/wailsBridge";

import { HOW_IT_WORKS_BY_ID } from "../../shell/navigation";
import { Button, EmptyState, ErrorState, LoadingState, ResizeHandle, SceneStack, ScrollRegion, WorkspaceFrame } from "../../design-system";
import SceneList from "../../components/SceneProgramming/SceneList";
import LayerRow, { LAYER_KINDS, type LayerKind } from "../../components/SceneProgramming/LayerRow";
import LookBrowser, { type PresetKind } from "../../components/SceneProgramming/LookBrowser";
import BarTimelinePanel from "../../components/SceneProgramming/BarTimelinePanel";
import { useInspectorSlot } from "../../shell/InspectorSlot";
import { useResizablePanel } from "../../hooks/useResizablePanel";
import styles from "./ScenesLooksWorkspace.module.css";

/** looksForKind returns the reusable-look list a given layer kind's picker
 * should source from: base_look -> presets, color_theme -> themes,
 * chase -> chases, motion -> motion presets. */
function looksForKind(kind: string, view: ProgrammingView): (ProgLookView | ProgPresetView | ProgChaseView)[] {
  switch (kind) {
    case "base_look":
      return view.presets;
    case "color_theme":
      return view.themes;
    case "chase":
      return view.chases;
    case "motion":
      return view.motions;
    default:
      return [];
  }
}

export default function ScenesLooksWorkspace() {
  const [view, setView] = useState<ProgrammingView>(offlineProgrammingView());
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedSceneName, setSelectedSceneName] = useState<string | null>(null);
  const [presetLoading, setPresetLoading] = useState(false);
  const sceneListPanel = useResizablePanel({
    min: 160,
    max: 400,
    defaultSize: 205,
    storageKey: "golc.sceneListWidth",
    edge: "end",
  });

  const refresh = useCallback(async (): Promise<void> => {
    try {
      const next = await listProgramming();
      setView(next);
      setError(null);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  // Keep the selection valid: default to the active scene once data loads,
  // and drop a selection that no longer exists (e.g. after a CLI-side
  // delete outside this session).
  useEffect(() => {
    if (selectedSceneName && view.scenes.some((scene) => scene.name === selectedSceneName)) {
      return;
    }
    const activeScene = view.scenes.find((scene) => scene.active) ?? view.scenes[0];
    setSelectedSceneName(activeScene?.name ?? null);
  }, [view, selectedSceneName]);

  const handleCreateScene = (name: string, bars: number) => {
    void (async () => {
      try {
        const result = await createScene(name, bars);
        assertOk(result, "CreateScene");
        await refresh();
        setSelectedSceneName(name);
      } catch (err) {
        setError(errorMessage(err));
      }
    })();
  };

  const handleActivateScene = async (name: string) => {
    try {
      const result = await activateScene(name);
      assertOk(result, "ActivateScene");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleToggleLayer = async (scene: ProgSceneView, kind: LayerKind) => {
    const layer = scene.layers.find((candidate) => candidate.kind === kind);
    const nextEnabled = !(layer?.enabled ?? false);
    try {
      const result = await setSceneLayer(scene.name, kind, "", nextEnabled);
      assertOk(result, "SetSceneLayer");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleSelectLayerLook = async (scene: ProgSceneView, kind: LayerKind, refId: string) => {
    if (refId === "") {
      return;
    }
    // Preserve the layer's current enabled state rather than hard-coding
    // `true`. The look picker stays live whether or not the layer is
    // toggled on (LayerRow.tsx's own contract), so on the LIVE scene
    // pre-staging a different chase for a disabled Chase layer used to
    // switch it on and start contributing to output -- a visible-on-stage
    // side effect from what reads as a staging action.
    const currentLayer = scene.layers.find((candidate) => candidate.kind === kind);
    const enabled = currentLayer?.enabled ?? false;
    try {
      const result = await setSceneLayer(scene.name, kind, refId, enabled);
      assertOk(result, "SetSceneLayer");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleCreateTheme = async (name: string) => {
    try {
      const result = await createTheme(name);
      assertOk(result, "CreateTheme");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleCreateMotion = async (name: string) => {
    try {
      const result = await createMotion(name);
      assertOk(result, "CreateMotion");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleCreateChase = async (name: string, unit: "bar" | "beat", stepDuration: number) => {
    try {
      const result = await createChase(name, unit, stepDuration);
      assertOk(result, "CreateChase");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleCreateBlend = async (name: string, duration: number, curve: string) => {
    try {
      const result = await createBlend(name, duration, curve);
      assertOk(result, "CreateBlend");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleRecordPreset = async (instanceId: string, attrs: string[], kind: PresetKind, name: string) => {
    setPresetLoading(true);
    try {
      const setResult = await programmerSet([instanceId], attrs);
      assertOk(setResult, "ProgrammerSet");
      const recordResult = await recordPreset(name, kind);
      assertOk(recordResult, "RecordPreset");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setPresetLoading(false);
    }
  };

  const handleRenameScene = async (oldName: string, newName: string) => {
    try {
      const result = await renameScene(oldName, newName);
      assertOk(result, "RenameScene");
      // Order matters: setting the selection BEFORE the refresh committed
      // a render where selectedSceneName was the new name while
      // view.scenes still held the old one, so the selection-validity
      // effect above saw no match and overwrote the selection with the
      // active/first scene. handleCreateScene already does it this way
      // round; this handler was the outlier.
      await refresh();
      if (selectedSceneName === oldName) {
        setSelectedSceneName(newName);
      }
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleDeleteScene = async (name: string) => {
    try {
      const result = await deleteScene(name);
      assertOk(result, "DeleteScene");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  // Translates SceneList's dragged name order into the 0-based index
  // permutation "scene reorder" expects, against view.scenes' own current
  // (pre-drag) order -- SceneList only ever hands back names, never
  // indices, since it has no idea what the server-side order actually is
  // once its own local `order` state has drifted from the `scenes` prop.
  // Reports whether the reorder was actually accepted, so SceneList can
  // roll its optimistic local order back on rejection -- its own reset
  // effect deliberately only fires when the scene name set changes, which
  // a failed reorder never does.
  const handleReorderScenes = async (orderedNames: string[]): Promise<boolean> => {
    const originalIndexByName = new Map(view.scenes.map((scene, index) => [scene.name, index]));
    const order = orderedNames.map((name) => originalIndexByName.get(name) ?? -1);
    if (order.length !== view.scenes.length || order.includes(-1)) {
      return false;
    }
    try {
      const result = await reorderScenes(order);
      assertOk(result, "ReorderScenes");
      await refresh();
      return true;
    } catch (err) {
      setError(errorMessage(err));
      return false;
    }
  };

  const handleRenameTheme = async (oldName: string, newName: string) => {
    try {
      const result = await renameTheme(oldName, newName);
      assertOk(result, "RenameTheme");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleDeleteTheme = async (name: string) => {
    try {
      const result = await deleteTheme(name);
      assertOk(result, "DeleteTheme");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleRenameMotion = async (oldName: string, newName: string) => {
    try {
      const result = await renameMotion(oldName, newName);
      assertOk(result, "RenameMotion");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleDeleteMotion = async (name: string) => {
    try {
      const result = await deleteMotion(name);
      assertOk(result, "DeleteMotion");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleRenamePreset = async (oldName: string, newName: string) => {
    try {
      const result = await renamePreset(oldName, newName);
      assertOk(result, "RenamePreset");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleDeletePreset = async (name: string) => {
    try {
      const result = await deletePreset(name);
      assertOk(result, "DeletePreset");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleUpdateChase = async (name: string, newName: string, unit: string, stepDuration: number) => {
    try {
      const result = await updateChase(name, newName, unit, stepDuration);
      assertOk(result, "UpdateChase");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleDeleteChase = async (name: string) => {
    try {
      const result = await deleteChase(name);
      assertOk(result, "DeleteChase");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleRenameBlend = async (oldName: string, newName: string) => {
    try {
      const result = await renameBlend(oldName, newName);
      assertOk(result, "RenameBlend");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleDeleteBlend = async (name: string) => {
    try {
      const result = await deleteBlend(name);
      assertOk(result, "DeleteBlend");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const inspectorPortal = useInspectorSlot(
    <LookBrowser
      view={view}
      onCreateTheme={(name) => void handleCreateTheme(name)}
      onCreateMotion={(name) => void handleCreateMotion(name)}
      onCreateChase={(name, unit, stepDuration) => void handleCreateChase(name, unit, stepDuration)}
      onCreateBlend={(name, duration, curve) => void handleCreateBlend(name, duration, curve)}
      onRecordPreset={(instanceId, attrs, kind, name) => void handleRecordPreset(instanceId, attrs, kind, name)}
      presetLoading={presetLoading}
      onRenameTheme={(oldName, newName) => void handleRenameTheme(oldName, newName)}
      onDeleteTheme={(name) => void handleDeleteTheme(name)}
      onRenameMotion={(oldName, newName) => void handleRenameMotion(oldName, newName)}
      onDeleteMotion={(name) => void handleDeleteMotion(name)}
      onRenamePreset={(oldName, newName) => void handleRenamePreset(oldName, newName)}
      onDeletePreset={(name) => void handleDeletePreset(name)}
      onUpdateChase={(name, newName, unit, stepDuration) => void handleUpdateChase(name, newName, unit, stepDuration)}
      onDeleteChase={(name) => void handleDeleteChase(name)}
      onRenameBlend={(oldName, newName) => void handleRenameBlend(oldName, newName)}
      onDeleteBlend={(name) => void handleDeleteBlend(name)}
    />,
  );

  const selectedScene = view.scenes.find((scene) => scene.name === selectedSceneName) ?? null;

  return (
    <WorkspaceFrame
      title="Scenes & Looks"
      info={HOW_IT_WORKS_BY_ID["build-scenes-looks"]}
    >
      {inspectorPortal}
      <div className={styles.canvas}>
        {loading ? (
          <LoadingState label="Loading scene programming…" variant="panel" />
        ) : (
          <>
            {error ? <ErrorState heading="Scene programming unavailable" message={error} /> : null}
            <div
              className={styles.layout}
              style={{ "--ds-scenelist-width": `${sceneListPanel.size}px` } as CSSProperties}
            >
              <div className={styles.sceneListColumn}>
                <SceneStack
                  scenes={view.scenes.map((scene) => ({
                    id: scene.name,
                    name: scene.name,
                    status: scene.active ? "live" : "neutral",
                    label: scene.active ? "LIVE" : `${scene.barsPerLoop}bar`,
                  }))}
                />
                <SceneList
                  scenes={view.scenes}
                  selectedName={selectedSceneName}
                  onSelect={setSelectedSceneName}
                  onCreate={handleCreateScene}
                  onRename={(oldName, newName) => void handleRenameScene(oldName, newName)}
                  onDelete={(name) => void handleDeleteScene(name)}
                  onReorder={handleReorderScenes}
                />
                <ResizeHandle
                  edge="end"
                  label="Resize scene list"
                  isResizing={sceneListPanel.isResizing}
                  onPointerDown={sceneListPanel.handlePointerDown}
                  onDoubleClick={sceneListPanel.resetSize}
                />
              </div>

              <div className={styles.mainColumn}>
                {selectedScene ? (
                  <>
                    <div className={styles.sceneHeader}>
                      <span className={styles.sceneName} title={selectedScene.name}>
                        {selectedScene.name}
                      </span>
                      {/* The active scene's LIVE status is already surfaced by
                          SceneStack, immediately to the left in the same
                          viewport; this header only needs an action when the
                          selected scene is NOT yet the live one. */}
                      {selectedScene.active ? null : (
                        <Button
                          variant="secondary"
                          leadingIcon={Zap}
                          onClick={() => void handleActivateScene(selectedScene.name)}
                        >
                          Activate
                        </Button>
                      )}
                    </div>
                    <ScrollRegion>
                      <ul className={styles.layerList} aria-label={`${selectedScene.name} layers`}>
                        {LAYER_KINDS.map((kind) => {
                          const layer = selectedScene.layers.find((candidate) => candidate.kind === kind);
                          return (
                            <LayerRow
                              key={kind}
                              kind={kind}
                              enabled={layer?.enabled ?? false}
                              refId={layer?.ref ?? ""}
                              looks={looksForKind(kind, view)}
                              onToggle={() => void handleToggleLayer(selectedScene, kind)}
                              onSelectLook={(refId) => void handleSelectLayerLook(selectedScene, kind, refId)}
                            />
                          );
                        })}
                      </ul>
                    </ScrollRegion>
                  </>
                ) : (
                  <EmptyState icon={Layers}>Create a scene to start pointing its layers at reusable looks.</EmptyState>
                )}

                <BarTimelinePanel activeSceneName={selectedScene?.name ?? null} />
              </div>
            </div>
          </>
        )}
      </div>
    </WorkspaceFrame>
  );
}
