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
  setSceneLayer,
  updateChase,
  type ProgChaseView,
  type ProgLookView,
  type ProgPresetView,
  type ProgrammingView,
  type ProgSceneView,
} from "../../lib/wailsBridge";

import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import { HOW_IT_WORKS_BY_ID } from "../../shell/navigation";
import ScrollRegion from "../../components/primitives/ScrollRegion/ScrollRegion";
import SceneList from "../../components/SceneProgramming/SceneList";
import LayerRow, { LAYER_KINDS, type LayerKind } from "../../components/SceneProgramming/LayerRow";
import LookBrowser, { type PresetKind } from "../../components/SceneProgramming/LookBrowser";
import BarTimelinePanel from "../../components/SceneProgramming/BarTimelinePanel";
import { useInspectorSlot } from "../../shell/InspectorSlot";
import { useResizablePanel } from "../../hooks/useResizablePanel";
import ResizeHandle from "../../components/primitives/ResizeHandle/ResizeHandle";
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
    try {
      const result = await setSceneLayer(scene.name, kind, refId, true);
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
      if (selectedSceneName === oldName) {
        setSelectedSceneName(newName);
      }
      await refresh();
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
    <div className={styles.workspace}>
      {inspectorPortal}
      <Toolbar title="Scenes & Looks" icon={Layers} info={HOW_IT_WORKS_BY_ID["build-scenes-looks"]} />
      <div className={styles.canvas}>
        {loading ? (
          <p className={styles.loading}>Loading scene programming…</p>
        ) : (
          <>
            {error ? <p className={styles.errorText}>{error}</p> : null}
            <div
              className={styles.layout}
              style={{ "--scenelist-width": `${sceneListPanel.size}px` } as CSSProperties}
            >
              <div className={styles.sceneListColumn}>
                <SceneList
                  scenes={view.scenes}
                  selectedName={selectedSceneName}
                  onSelect={setSelectedSceneName}
                  onCreate={handleCreateScene}
                  onRename={(oldName, newName) => void handleRenameScene(oldName, newName)}
                  onDelete={(name) => void handleDeleteScene(name)}
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
                      {selectedScene.active ? (
                        <span className={styles.activeChip}>LIVE</span>
                      ) : (
                        <button
                          type="button"
                          className={styles.activateButton}
                          onClick={() => void handleActivateScene(selectedScene.name)}
                        >
                          <Zap size={13} aria-hidden="true" />
                          Activate
                        </button>
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
                  <p className={styles.emptyState}>
                    <Layers size={14} aria-hidden="true" />
                    Create a scene to start pointing its layers at reusable looks.
                  </p>
                )}

                <BarTimelinePanel activeSceneName={selectedScene?.name ?? null} />
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
