// SceneProgramming.tsx is the on-screen scene/look programming surface
// closing VERIFICATION.md Gap B[0] for PLAY-12 (06-12-PLAN.md): a show
// author creates bar-loop scenes, creates each of the four reusable look
// kinds (color theme, chase, motion preset, and a base-look preset via a
// minimal "programmer set" + "preset record" flow), enables and points
// each of a scene's four fixed layers at a reusable look, activates a
// scene, and creates reusable blend presets -- all driving the exact same
// "scene"/"theme"/"chase"/"motion"/"programmer"/"preset"/"blend" CLI
// routes internal/command/scene.go and internal/command/programming.go
// already implement and test. This is a UI-binding exercise against a
// stable backend, architecturally identical to 06-10's FixturePatch.tsx/
// FixturePatchService and 06-11's ArtnetConfig.tsx/ArtnetConfigService
// wiring.
//
// All Go-bound calls go through frontend/src/lib/wailsBridge.ts's
// ProgrammingService helpers (listProgramming/createScene/activateScene/
// setSceneLayer/createTheme/createMotion/createChase/programmerSet/
// recordPreset/createBlend) -- this file never re-declares
// `declare global` itself (the same Wave-3 post-merge collision
// FixturePatch.tsx/ArtnetConfig.tsx's own comments document) and never
// adds a second scene/look mutation path.
//
// Simplified-subset boundary (Claude's Discretion, 06-12-PLAN.md flagged
// assumption -- no CONTEXT decision covers PLAY-12): this component binds
// the core authoring path required by PLAY-12 -- create scene, create/
// record each look kind, enable+point the four scene layers, activate a
// scene, create a blend -- plus enough of "programmer set" to record a
// base-look/color preset from a picked deployment instance and a
// comma-separated "capability=value" attribute list. Rename/reorder/
// duplicate/delete for every look kind and the full programmer attribute-
// editing matrix remain the CLI's own full-fidelity path this round; the
// on-screen note below records this boundary explicitly rather than
// silently dropping the scope.
//
// State coverage (Task 3, 06-UI-SPEC.md-style backstop): initial load
// renders a skeleton placeholder; a failed bridge call's own stderr
// diagnostic surfaces verbatim in the error banner, never a silent
// failure; the scene list and each look list render an explicit empty
// state with correct singular/plural counts; and every list scrolls
// within a fixed-height panel (SceneProgramming.module.css's
// sceneScroll/lookScroll) rather than growing the window against a
// representative large show. The full create-scene -> create-each-look
// -> enable+point-each-layer -> activate click-through against a real
// golc-desktop build is queued as a human-check for end-of-phase UAT
// (workflow.human_verify_mode=end-of-phase) rather than an interactive
// mid-execution checkpoint.

import { useCallback, useEffect, useMemo, useState } from "react";

import {
  activateScene,
  assertOk,
  createBlend,
  createChase,
  createMotion,
  createScene,
  createTheme,
  errorMessage,
  listProgramming,
  offlineProgrammingView,
  programmerSet,
  recordPreset,
  setSceneLayer,
  type ProgLookView,
  type ProgPresetView,
  type ProgrammingView,
  type ProgSceneView,
} from "../../lib/wailsBridge";

import styles from "./SceneProgramming.module.css";

// LAYER_KINDS is the fixed, deterministic four-slot layer order every
// scene always carries (mirrors internal/scene/scene.go's own
// layerPriority order: base_look, color_theme, chase, motion).
const LAYER_KINDS = ["base_look", "color_theme", "chase", "motion"] as const;
type LayerKind = (typeof LAYER_KINDS)[number];

function layerKindLabel(kind: string): string {
  switch (kind) {
    case "base_look":
      return "Base Look";
    case "color_theme":
      return "Color Theme";
    case "chase":
      return "Chase";
    case "motion":
      return "Motion";
    default:
      return kind;
  }
}

function parseAttrs(raw: string): string[] {
  return raw
    .split(",")
    .map((entry) => entry.trim())
    .filter((entry) => entry.length > 0);
}

function pluralize(count: number, noun: string): string {
  return `${count} ${noun}${count === 1 ? "" : "s"}`;
}

/** looksForKind returns the reusable-look list a given layer kind's picker
 * should source from: base_look -> presets, color_theme -> themes,
 * chase -> chases, motion -> motion presets. */
function looksForKind(
  kind: string,
  view: ProgrammingView,
): (ProgLookView | ProgPresetView)[] {
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

export default function SceneProgramming() {
  const [view, setView] = useState<ProgrammingView>(offlineProgrammingView());
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [newSceneName, setNewSceneName] = useState("");
  const [newSceneBars, setNewSceneBars] = useState("4");

  const [newThemeName, setNewThemeName] = useState("");
  const [newMotionName, setNewMotionName] = useState("");
  const [newChaseName, setNewChaseName] = useState("");
  const [newChaseUnit, setNewChaseUnit] = useState<"bar" | "beat">("bar");
  const [newChaseStepDuration, setNewChaseStepDuration] = useState("1");

  const [newBlendName, setNewBlendName] = useState("");
  const [newBlendDuration, setNewBlendDuration] = useState("1");
  const [newBlendCurve, setNewBlendCurve] = useState("linear");

  const [presetInstanceId, setPresetInstanceId] = useState("");
  const [presetAttrs, setPresetAttrs] = useState("");
  const [presetName, setPresetName] = useState("");
  const [presetKind, setPresetKind] = useState<
    "intensity" | "color" | "position" | "beam"
  >("intensity");
  const [presetLoading, setPresetLoading] = useState(false);

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

  const handleCreateScene = async () => {
    const trimmed = newSceneName.trim();
    const bars = Number.parseInt(newSceneBars, 10);
    if (trimmed === "" || Number.isNaN(bars)) {
      return;
    }
    try {
      const result = await createScene(trimmed, bars);
      assertOk(result, "CreateScene");
      setNewSceneName("");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
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
    const layer = scene.layers.find((l) => l.kind === kind);
    const nextEnabled = !(layer?.enabled ?? false);
    try {
      const result = await setSceneLayer(scene.name, kind, "", nextEnabled);
      assertOk(result, "SetSceneLayer");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleSelectLayerLook = async (
    scene: ProgSceneView,
    kind: LayerKind,
    refId: string,
  ) => {
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

  const handleCreateTheme = async () => {
    const trimmed = newThemeName.trim();
    if (trimmed === "") return;
    try {
      const result = await createTheme(trimmed);
      assertOk(result, "CreateTheme");
      setNewThemeName("");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleCreateMotion = async () => {
    const trimmed = newMotionName.trim();
    if (trimmed === "") return;
    try {
      const result = await createMotion(trimmed);
      assertOk(result, "CreateMotion");
      setNewMotionName("");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleCreateChase = async () => {
    const trimmed = newChaseName.trim();
    const stepDuration = Number.parseFloat(newChaseStepDuration);
    if (trimmed === "" || Number.isNaN(stepDuration)) return;
    try {
      const result = await createChase(trimmed, newChaseUnit, stepDuration);
      assertOk(result, "CreateChase");
      setNewChaseName("");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleCreateBlend = async () => {
    const trimmed = newBlendName.trim();
    const duration = Number.parseFloat(newBlendDuration);
    if (trimmed === "" || Number.isNaN(duration)) return;
    try {
      const result = await createBlend(trimmed, duration, newBlendCurve.trim());
      assertOk(result, "CreateBlend");
      setNewBlendName("");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleRecordPreset = async () => {
    const trimmedName = presetName.trim();
    const attrs = parseAttrs(presetAttrs);
    if (trimmedName === "" || presetInstanceId === "" || attrs.length === 0) {
      return;
    }
    setPresetLoading(true);
    try {
      const setResult = await programmerSet([presetInstanceId], attrs);
      assertOk(setResult, "ProgrammerSet");
      const recordResult = await recordPreset(trimmedName, presetKind);
      assertOk(recordResult, "RecordPreset");
      setPresetName("");
      setPresetAttrs("");
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setPresetLoading(false);
    }
  };

  const scenes = view.scenes;
  const looksTotal = useMemo(
    () =>
      view.themes.length +
      view.presets.length +
      view.chases.length +
      view.motions.length,
    [view],
  );

  return (
    <section
      className={styles.panel}
      aria-label="Scene programming"
      aria-busy={loading}
    >
      <h2 className={styles.sectionHeading}>Scene &amp; Look Programming</h2>
      <p className={styles.subsetNote}>
        This surface covers create/point/activate for scenes and looks and a
        minimal preset-recording flow. Renaming, reordering, duplicating,
        deleting, and the full programmer attribute matrix remain available
        from the CLI this round.
      </p>

      {loading ? (
        <div className={styles.skeleton}>Loading scene programming…</div>
      ) : (
        <>
          {error && <p className={styles.errorText}>{error}</p>}

          {/* Scenes */}
          <div className={styles.subsection}>
            <h3 className={styles.subsectionHeading}>Scenes</h3>
            <div className={styles.createRow}>
              <input
                className={styles.createInput}
                type="text"
                value={newSceneName}
                placeholder="New scene name"
                onChange={(event) => setNewSceneName(event.target.value)}
                aria-label="New scene name"
              />
              <input
                className={styles.createInputNarrow}
                type="number"
                min={1}
                value={newSceneBars}
                onChange={(event) => setNewSceneBars(event.target.value)}
                aria-label="Bars per loop"
              />
              <button
                type="button"
                className={styles.primaryButton}
                onClick={() => void handleCreateScene()}
              >
                Create Scene
              </button>
            </div>

            {scenes.length === 0 ? (
              <div className={styles.emptyState}>
                <p className={styles.emptyHeading}>No scenes yet</p>
                <p className={styles.emptyBody}>
                  Create a bar-loop scene, then point its four layers at
                  reusable looks below.
                </p>
              </div>
            ) : (
              <>
                <p className={styles.countSummary}>
                  {pluralize(scenes.length, "scene")}
                </p>
                <ul className={styles.sceneScroll} aria-label="Scene list">
                  {scenes.map((scene) => (
                    <li key={scene.name} className={styles.row}>
                      <div className={styles.rowHeader}>
                        <span className={styles.rowName} title={scene.name}>
                          {scene.name}
                        </span>
                        <span className={styles.rowCounts}>
                          {scene.barsPerLoop} bar
                          {scene.barsPerLoop === 1 ? "" : "s"}
                        </span>
                        {scene.active ? (
                          <span className={styles.activeChip}>Active</span>
                        ) : (
                          <button
                            type="button"
                            className={styles.secondaryButton}
                            onClick={() => void handleActivateScene(scene.name)}
                          >
                            Activate
                          </button>
                        )}
                      </div>
                      <ul className={styles.layerList}>
                        {LAYER_KINDS.map((kind) => {
                          const layer = scene.layers.find((l) => l.kind === kind);
                          const looks = looksForKind(kind, view);
                          return (
                            <li key={kind} className={styles.layerRow}>
                              <button
                                type="button"
                                className={
                                  layer?.enabled
                                    ? styles.layerToggleOn
                                    : styles.layerToggleOff
                                }
                                onClick={() => void handleToggleLayer(scene, kind)}
                                aria-pressed={layer?.enabled ?? false}
                              >
                                {layerKindLabel(kind)}
                              </button>
                              <select
                                className={styles.lookSelect}
                                value={layer?.ref ?? ""}
                                onChange={(event) =>
                                  void handleSelectLayerLook(
                                    scene,
                                    kind,
                                    event.target.value,
                                  )
                                }
                                aria-label={`${layerKindLabel(kind)} look for ${scene.name}`}
                              >
                                <option value="" disabled>
                                  {looks.length === 0
                                    ? "No looks available"
                                    : "Select a look…"}
                                </option>
                                {looks.map((look) => (
                                  <option key={look.id} value={look.id}>
                                    {look.name}
                                  </option>
                                ))}
                              </select>
                            </li>
                          );
                        })}
                      </ul>
                    </li>
                  ))}
                </ul>
              </>
            )}
          </div>

          {/* Looks */}
          <div className={styles.subsection}>
            <h3 className={styles.subsectionHeading}>Looks</h3>

            {looksTotal === 0 ? (
              <div className={styles.emptyState}>
                <p className={styles.emptyHeading}>No looks yet</p>
                <p className={styles.emptyBody}>
                  Create a color theme, chase, motion preset, or base-look
                  preset below, then point a scene layer at it above.
                </p>
              </div>
            ) : (
              <p className={styles.countSummary}>
                {pluralize(view.themes.length, "theme")},{" "}
                {pluralize(view.chases.length, "chase")},{" "}
                {pluralize(view.motions.length, "motion preset")},{" "}
                {pluralize(view.presets.length, "base-look preset")}
              </p>
            )}

            <div className={styles.lookCreateGrid}>
              <div className={styles.createRow}>
                <input
                  className={styles.createInput}
                  type="text"
                  value={newThemeName}
                  placeholder="New color theme name"
                  onChange={(event) => setNewThemeName(event.target.value)}
                  aria-label="New color theme name"
                />
                <button
                  type="button"
                  className={styles.primaryButton}
                  onClick={() => void handleCreateTheme()}
                >
                  Create Theme
                </button>
              </div>

              <div className={styles.createRow}>
                <input
                  className={styles.createInput}
                  type="text"
                  value={newMotionName}
                  placeholder="New motion preset name"
                  onChange={(event) => setNewMotionName(event.target.value)}
                  aria-label="New motion preset name"
                />
                <button
                  type="button"
                  className={styles.primaryButton}
                  onClick={() => void handleCreateMotion()}
                >
                  Create Motion
                </button>
              </div>

              <div className={styles.createRow}>
                <input
                  className={styles.createInput}
                  type="text"
                  value={newChaseName}
                  placeholder="New chase name"
                  onChange={(event) => setNewChaseName(event.target.value)}
                  aria-label="New chase name"
                />
                <select
                  className={styles.createInputNarrow}
                  value={newChaseUnit}
                  onChange={(event) =>
                    setNewChaseUnit(event.target.value as "bar" | "beat")
                  }
                  aria-label="Chase step unit"
                >
                  <option value="bar">bar</option>
                  <option value="beat">beat</option>
                </select>
                <input
                  className={styles.createInputNarrow}
                  type="number"
                  min={0}
                  step="any"
                  value={newChaseStepDuration}
                  onChange={(event) => setNewChaseStepDuration(event.target.value)}
                  aria-label="Chase step duration"
                />
                <button
                  type="button"
                  className={styles.primaryButton}
                  onClick={() => void handleCreateChase()}
                >
                  Create Chase
                </button>
              </div>

              <div className={styles.presetForm}>
                <p className={styles.presetFormHeading}>
                  Record a base-look / attribute preset
                </p>
                <div className={styles.createRow}>
                  <select
                    className={styles.createInput}
                    value={presetInstanceId}
                    onChange={(event) => setPresetInstanceId(event.target.value)}
                    aria-label="Fixture instance"
                  >
                    <option value="" disabled>
                      {view.instances.length === 0
                        ? "No deployment instances available"
                        : "Select a fixture instance…"}
                    </option>
                    {view.instances.map((instance) => (
                      <option key={instance.id} value={instance.id}>
                        {instance.label}
                      </option>
                    ))}
                  </select>
                  <select
                    className={styles.createInputNarrow}
                    value={presetKind}
                    onChange={(event) =>
                      setPresetKind(
                        event.target.value as
                          | "intensity"
                          | "color"
                          | "position"
                          | "beam",
                      )
                    }
                    aria-label="Preset kind"
                  >
                    <option value="intensity">intensity</option>
                    <option value="color">color</option>
                    <option value="position">position</option>
                    <option value="beam">beam</option>
                  </select>
                </div>
                <div className={styles.createRow}>
                  <input
                    className={styles.createInput}
                    type="text"
                    value={presetAttrs}
                    placeholder="capability=value, comma-separated (e.g. intensity=0.8)"
                    onChange={(event) => setPresetAttrs(event.target.value)}
                    aria-label="Attribute assignments"
                  />
                  <input
                    className={styles.createInput}
                    type="text"
                    value={presetName}
                    placeholder="Preset name"
                    onChange={(event) => setPresetName(event.target.value)}
                    aria-label="Preset name"
                  />
                  <button
                    type="button"
                    className={styles.primaryButton}
                    disabled={presetLoading}
                    onClick={() => void handleRecordPreset()}
                  >
                    {presetLoading ? "Recording…" : "Record Preset"}
                  </button>
                </div>
              </div>
            </div>

            {(view.themes.length > 0 ||
              view.chases.length > 0 ||
              view.motions.length > 0 ||
              view.presets.length > 0) && (
              <ul className={styles.lookScroll} aria-label="Look list">
                {view.themes.map((look) => (
                  <li key={`theme-${look.id}`} className={styles.lookRow}>
                    <span className={styles.lookKind}>Color Theme</span>
                    <span title={look.name}>{look.name}</span>
                  </li>
                ))}
                {view.chases.map((look) => (
                  <li key={`chase-${look.id}`} className={styles.lookRow}>
                    <span className={styles.lookKind}>Chase</span>
                    <span title={look.name}>{look.name}</span>
                  </li>
                ))}
                {view.motions.map((look) => (
                  <li key={`motion-${look.id}`} className={styles.lookRow}>
                    <span className={styles.lookKind}>Motion</span>
                    <span title={look.name}>{look.name}</span>
                  </li>
                ))}
                {view.presets.map((preset) => (
                  <li key={`preset-${preset.id}`} className={styles.lookRow}>
                    <span className={styles.lookKind}>
                      Preset ({preset.kind})
                    </span>
                    <span title={preset.name}>{preset.name}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>

          {/* Blends */}
          <div className={styles.subsection}>
            <h3 className={styles.subsectionHeading}>Blend Presets</h3>
            <div className={styles.createRow}>
              <input
                className={styles.createInput}
                type="text"
                value={newBlendName}
                placeholder="New blend name"
                onChange={(event) => setNewBlendName(event.target.value)}
                aria-label="New blend name"
              />
              <input
                className={styles.createInputNarrow}
                type="number"
                min={0}
                step="any"
                value={newBlendDuration}
                onChange={(event) => setNewBlendDuration(event.target.value)}
                aria-label="Blend duration (bars)"
              />
              <select
                className={styles.createInputNarrow}
                value={newBlendCurve}
                onChange={(event) => setNewBlendCurve(event.target.value)}
                aria-label="Blend curve"
              >
                <option value="linear">linear</option>
                <option value="ease_in">ease_in</option>
                <option value="ease_out">ease_out</option>
              </select>
              <button
                type="button"
                className={styles.primaryButton}
                onClick={() => void handleCreateBlend()}
              >
                Create Blend
              </button>
            </div>

            {view.blends.length === 0 ? (
              <div className={styles.emptyState}>
                <p className={styles.emptyHeading}>No blend presets yet</p>
                <p className={styles.emptyBody}>
                  Create a blend preset to describe transitions between scene
                  and layer states.
                </p>
              </div>
            ) : (
              <>
                <p className={styles.countSummary}>
                  {pluralize(view.blends.length, "blend preset")}
                </p>
                <ul className={styles.lookScroll} aria-label="Blend list">
                  {view.blends.map((blend) => (
                    <li key={blend.id} className={styles.lookRow}>
                      <span title={blend.name}>{blend.name}</span>
                    </li>
                  ))}
                </ul>
              </>
            )}
          </div>
        </>
      )}
    </section>
  );
}
