// LookBrowser is the Scene Stack's reusable-look browser (programming-
// scene-authoring.md), published into the shell's contextual inspector via
// useInspectorSlot rather than living inline in the workspace canvas. Only
// one create-form is open at a time (toggled by category button) --
// "Permanently displaying every look-creation form" is the sketch's own
// explicit "what to avoid".
import { useState } from "react";
import { Plus, Check, X, Pencil, Trash2, Sparkles, Palette } from "lucide-react";

import type { ProgChaseView, ProgLookView, ProgPresetView, ProgrammingView } from "../../lib/wailsBridge";
import Button from "../primitives/Button/Button";
import PanelHeader from "../primitives/PanelHeader/PanelHeader";
import ScrollRegion from "../primitives/ScrollRegion/ScrollRegion";
import EmptyState from "../primitives/EmptyState/EmptyState";
import styles from "./LookBrowser.module.css";

type FormKind = "theme" | "motion" | "chase" | "preset" | "blend";
export type PresetKind = "intensity" | "color" | "position" | "beam";

/** RenameKind covers every look kind that only needs a plain rename +
 * delete (everything except Chase, which gets its own richer edit form
 * below since it also carries stepUnit/stepDuration). */
type RenameKind = "theme" | "motion" | "preset" | "blend";

interface LookBrowserProps {
  view: ProgrammingView;
  onCreateTheme: (name: string) => void;
  onCreateMotion: (name: string) => void;
  onCreateChase: (name: string, unit: "bar" | "beat", stepDuration: number) => void;
  onCreateBlend: (name: string, duration: number, curve: string) => void;
  onRecordPreset: (instanceId: string, attrs: string[], kind: PresetKind, name: string) => void;
  presetLoading: boolean;
  onRenameTheme: (oldName: string, newName: string) => void;
  onDeleteTheme: (name: string) => void;
  onRenameMotion: (oldName: string, newName: string) => void;
  onDeleteMotion: (name: string) => void;
  onRenamePreset: (oldName: string, newName: string) => void;
  onDeletePreset: (name: string) => void;
  onUpdateChase: (name: string, newName: string, unit: string, stepDuration: number) => void;
  onDeleteChase: (name: string) => void;
  onRenameBlend: (oldName: string, newName: string) => void;
  onDeleteBlend: (name: string) => void;
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

export default function LookBrowser({
  view,
  onCreateTheme,
  onCreateMotion,
  onCreateChase,
  onCreateBlend,
  onRecordPreset,
  presetLoading,
  onRenameTheme,
  onDeleteTheme,
  onRenameMotion,
  onDeleteMotion,
  onRenamePreset,
  onDeletePreset,
  onUpdateChase,
  onDeleteChase,
  onRenameBlend,
  onDeleteBlend,
}: LookBrowserProps) {
  const [activeForm, setActiveForm] = useState<FormKind | null>(null);

  // Local per-form input state -- kept minimal since only one form is ever
  // mounted/visible at a time.
  const [themeName, setThemeName] = useState("");
  const [motionName, setMotionName] = useState("");
  const [chaseName, setChaseName] = useState("");
  const [chaseUnit, setChaseUnit] = useState<"bar" | "beat">("bar");
  const [chaseStepDuration, setChaseStepDuration] = useState("1");
  const [blendName, setBlendName] = useState("");
  const [blendDuration, setBlendDuration] = useState("1");
  const [blendCurve, setBlendCurve] = useState("linear");
  const [presetInstanceId, setPresetInstanceId] = useState("");
  const [presetKind, setPresetKind] = useState<PresetKind>("intensity");
  const [presetAttrs, setPresetAttrs] = useState("");
  const [presetName, setPresetName] = useState("");

  // Rename state, shared across theme/motion/preset/blend (RenameKind) --
  // keyed by "<kind>-<id>" so only one row across every category is ever
  // in edit mode at a time, mirroring the single-active-form discipline
  // above.
  const [renamingKey, setRenamingKey] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState("");

  // Chase gets its own richer edit state (name + unit + step-duration),
  // since "chase update" covers more than a plain rename.
  const [editingChaseId, setEditingChaseId] = useState<string | null>(null);
  const [chaseEditName, setChaseEditName] = useState("");
  const [chaseEditUnit, setChaseEditUnit] = useState<"bar" | "beat">("bar");
  const [chaseEditStepDuration, setChaseEditStepDuration] = useState("1");

  const toggle = (kind: FormKind) => setActiveForm((current) => (current === kind ? null : kind));

  const renameHandlers: Record<RenameKind, (oldName: string, newName: string) => void> = {
    theme: onRenameTheme,
    motion: onRenameMotion,
    preset: onRenamePreset,
    blend: onRenameBlend,
  };
  const deleteHandlers: Record<RenameKind, (name: string) => void> = {
    theme: onDeleteTheme,
    motion: onDeleteMotion,
    preset: onDeletePreset,
    blend: onDeleteBlend,
  };

  const handleStartRename = (kind: RenameKind, look: ProgLookView | ProgPresetView) => {
    setRenamingKey(`${kind}-${look.id}`);
    setRenameValue(look.name);
  };

  const handleSaveRename = (kind: RenameKind, currentName: string) => {
    const trimmed = renameValue.trim();
    if (trimmed === "" || trimmed === currentName) {
      setRenamingKey(null);
      return;
    }
    renameHandlers[kind](currentName, trimmed);
    setRenamingKey(null);
  };

  const handleDelete = (kind: RenameKind, name: string, label: string) => {
    if (window.confirm(`Delete ${label} "${name}"?`)) {
      deleteHandlers[kind](name);
    }
  };

  const handleStartEditChase = (chase: ProgChaseView) => {
    setEditingChaseId(chase.id);
    setChaseEditName(chase.name);
    setChaseEditUnit(chase.stepUnit === "beat" ? "beat" : "bar");
    setChaseEditStepDuration(String(chase.stepDuration));
  };

  const handleSaveEditChase = (originalName: string) => {
    const trimmedName = chaseEditName.trim();
    const stepDuration = Number.parseFloat(chaseEditStepDuration);
    if (Number.isNaN(stepDuration) || stepDuration <= 0) {
      return;
    }
    onUpdateChase(originalName, trimmedName === originalName ? "" : trimmedName, chaseEditUnit, stepDuration);
    setEditingChaseId(null);
  };

  const handleDeleteChase = (name: string) => {
    if (window.confirm(`Delete chase "${name}"?`)) {
      onDeleteChase(name);
    }
  };

  const looksTotal = view.themes.length + view.chases.length + view.motions.length + view.presets.length;

  return (
    <div className={styles.browser}>
      <PanelHeader label="Looks" icon={Sparkles} />

      <p className={styles.countSummary}>
        {looksTotal === 0
          ? "No looks yet — create one below, then point a scene layer at it."
          : `${pluralize(view.themes.length, "theme")}, ${pluralize(view.chases.length, "chase")}, ${pluralize(view.motions.length, "motion preset")}, ${pluralize(view.presets.length, "base-look preset")}`}
      </p>

      <div className={styles.categoryRow}>
        <Button variant={activeForm === "theme" ? "primary" : "secondary"} icon={Plus} onClick={() => toggle("theme")}>
          + Theme
        </Button>
        <Button variant={activeForm === "motion" ? "primary" : "secondary"} icon={Plus} onClick={() => toggle("motion")}>
          + Motion
        </Button>
        <Button variant={activeForm === "chase" ? "primary" : "secondary"} icon={Plus} onClick={() => toggle("chase")}>
          + Chase
        </Button>
        <Button variant={activeForm === "preset" ? "primary" : "secondary"} icon={Plus} onClick={() => toggle("preset")}>
          + Preset
        </Button>
      </div>

      {activeForm === "theme" ? (
        <div className={styles.form}>
          <input
            className={styles.input}
            type="text"
            value={themeName}
            placeholder="Color theme name"
            aria-label="New color theme name"
            onChange={(event) => setThemeName(event.target.value)}
          />
          <Button
            variant="primary"
            icon={Check}
            onClick={() => {
              if (themeName.trim() === "") return;
              onCreateTheme(themeName.trim());
              setThemeName("");
              setActiveForm(null);
            }}
          >
            Create Theme
          </Button>
        </div>
      ) : null}

      {activeForm === "motion" ? (
        <div className={styles.form}>
          <input
            className={styles.input}
            type="text"
            value={motionName}
            placeholder="Motion preset name"
            aria-label="New motion preset name"
            onChange={(event) => setMotionName(event.target.value)}
          />
          <Button
            variant="primary"
            icon={Check}
            onClick={() => {
              if (motionName.trim() === "") return;
              onCreateMotion(motionName.trim());
              setMotionName("");
              setActiveForm(null);
            }}
          >
            Create Motion
          </Button>
        </div>
      ) : null}

      {activeForm === "chase" ? (
        <div className={styles.form}>
          <input
            className={styles.input}
            type="text"
            value={chaseName}
            placeholder="Chase name"
            aria-label="New chase name"
            onChange={(event) => setChaseName(event.target.value)}
          />
          <div className={styles.row}>
            <select
              className={styles.inputNarrow}
              value={chaseUnit}
              aria-label="Chase step unit"
              onChange={(event) => setChaseUnit(event.target.value as "bar" | "beat")}
            >
              <option value="bar">bar</option>
              <option value="beat">beat</option>
            </select>
            <input
              className={styles.inputNarrow}
              type="number"
              min={0}
              step="any"
              value={chaseStepDuration}
              aria-label="Chase step duration"
              onChange={(event) => setChaseStepDuration(event.target.value)}
            />
          </div>
          <Button
            variant="primary"
            icon={Check}
            onClick={() => {
              const stepDuration = Number.parseFloat(chaseStepDuration);
              if (chaseName.trim() === "" || Number.isNaN(stepDuration)) return;
              onCreateChase(chaseName.trim(), chaseUnit, stepDuration);
              setChaseName("");
              setActiveForm(null);
            }}
          >
            Create Chase
          </Button>
        </div>
      ) : null}

      {activeForm === "preset" ? (
        <div className={styles.form}>
          <select
            className={styles.input}
            value={presetInstanceId}
            aria-label="Fixture instance"
            onChange={(event) => setPresetInstanceId(event.target.value)}
          >
            <option value="" disabled>
              {view.instances.length === 0 ? "No deployment instances available" : "Select a fixture instance…"}
            </option>
            {view.instances.map((instance) => (
              <option key={instance.id} value={instance.id}>
                {instance.label}
              </option>
            ))}
          </select>
          <select
            className={styles.inputNarrow}
            value={presetKind}
            aria-label="Preset kind"
            onChange={(event) => setPresetKind(event.target.value as PresetKind)}
          >
            <option value="intensity">intensity</option>
            <option value="color">color</option>
            <option value="position">position</option>
            <option value="beam">beam</option>
          </select>
          <input
            className={styles.input}
            type="text"
            value={presetAttrs}
            placeholder="capability=value, comma-separated"
            aria-label="Attribute assignments"
            onChange={(event) => setPresetAttrs(event.target.value)}
          />
          <input
            className={styles.input}
            type="text"
            value={presetName}
            placeholder="Preset name"
            aria-label="Preset name"
            onChange={(event) => setPresetName(event.target.value)}
          />
          <Button
            variant="primary"
            icon={Check}
            disabled={presetLoading}
            onClick={() => {
              const attrs = parseAttrs(presetAttrs);
              if (presetName.trim() === "" || presetInstanceId === "" || attrs.length === 0) return;
              onRecordPreset(presetInstanceId, attrs, presetKind, presetName.trim());
              setPresetName("");
              setPresetAttrs("");
              setActiveForm(null);
            }}
          >
            {presetLoading ? "Recording…" : "Record Preset"}
          </Button>
        </div>
      ) : null}

      <ScrollRegion className={styles.list}>
        {looksTotal > 0 ? (
          <ul className={styles.rows} aria-label="Look list">
            {view.themes.map((look) =>
              renamingKey === `theme-${look.id}` ? (
                <li key={`theme-${look.id}`} className={styles.renameRow}>
                  <input
                    className={styles.input}
                    type="text"
                    value={renameValue}
                    aria-label="Theme name"
                    autoFocus
                    onChange={(event) => setRenameValue(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter") handleSaveRename("theme", look.name);
                      if (event.key === "Escape") setRenamingKey(null);
                    }}
                  />
                  <Button variant="secondary" icon={Check} onClick={() => handleSaveRename("theme", look.name)} aria-label="Save" />
                  <Button variant="secondary" icon={X} onClick={() => setRenamingKey(null)} aria-label="Cancel" />
                </li>
              ) : (
                <li key={`theme-${look.id}`} className={styles.lookRow}>
                  <span className={styles.lookKind}>Theme</span>
                  <span title={look.name}>{look.name}</span>
                  <span className={styles.rowActions}>
                    <Button variant="secondary" icon={Pencil} onClick={() => handleStartRename("theme", look)} aria-label={`Rename ${look.name}`} />
                    <Button variant="destructive" icon={Trash2} onClick={() => handleDelete("theme", look.name, "theme")} aria-label={`Delete ${look.name}`} />
                  </span>
                </li>
              ),
            )}
            {view.chases.map((chase) =>
              editingChaseId === chase.id ? (
                <li key={`chase-${chase.id}`} className={styles.renameRow}>
                  <input
                    className={styles.input}
                    type="text"
                    value={chaseEditName}
                    aria-label="Chase name"
                    autoFocus
                    onChange={(event) => setChaseEditName(event.target.value)}
                  />
                  <select
                    className={styles.inputNarrow}
                    value={chaseEditUnit}
                    aria-label="Chase step unit"
                    onChange={(event) => setChaseEditUnit(event.target.value as "bar" | "beat")}
                  >
                    <option value="bar">bar</option>
                    <option value="beat">beat</option>
                  </select>
                  <input
                    className={styles.inputNarrow}
                    type="number"
                    min={0}
                    step="any"
                    value={chaseEditStepDuration}
                    aria-label="Chase step duration"
                    onChange={(event) => setChaseEditStepDuration(event.target.value)}
                  />
                  <Button variant="secondary" icon={Check} onClick={() => handleSaveEditChase(chase.name)} aria-label="Save" />
                  <Button variant="secondary" icon={X} onClick={() => setEditingChaseId(null)} aria-label="Cancel" />
                </li>
              ) : (
                <li key={`chase-${chase.id}`} className={styles.lookRow}>
                  <span className={styles.lookKind}>Chase</span>
                  <span title={chase.name}>{chase.name}</span>
                  <span className={styles.rowActions}>
                    <Button variant="secondary" icon={Pencil} onClick={() => handleStartEditChase(chase)} aria-label={`Edit ${chase.name}`} />
                    <Button variant="destructive" icon={Trash2} onClick={() => handleDeleteChase(chase.name)} aria-label={`Delete ${chase.name}`} />
                  </span>
                </li>
              ),
            )}
            {view.motions.map((look) =>
              renamingKey === `motion-${look.id}` ? (
                <li key={`motion-${look.id}`} className={styles.renameRow}>
                  <input
                    className={styles.input}
                    type="text"
                    value={renameValue}
                    aria-label="Motion preset name"
                    autoFocus
                    onChange={(event) => setRenameValue(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter") handleSaveRename("motion", look.name);
                      if (event.key === "Escape") setRenamingKey(null);
                    }}
                  />
                  <Button variant="secondary" icon={Check} onClick={() => handleSaveRename("motion", look.name)} aria-label="Save" />
                  <Button variant="secondary" icon={X} onClick={() => setRenamingKey(null)} aria-label="Cancel" />
                </li>
              ) : (
                <li key={`motion-${look.id}`} className={styles.lookRow}>
                  <span className={styles.lookKind}>Motion</span>
                  <span title={look.name}>{look.name}</span>
                  <span className={styles.rowActions}>
                    <Button variant="secondary" icon={Pencil} onClick={() => handleStartRename("motion", look)} aria-label={`Rename ${look.name}`} />
                    <Button variant="destructive" icon={Trash2} onClick={() => handleDelete("motion", look.name, "motion preset")} aria-label={`Delete ${look.name}`} />
                  </span>
                </li>
              ),
            )}
            {view.presets.map((preset) =>
              renamingKey === `preset-${preset.id}` ? (
                <li key={`preset-${preset.id}`} className={styles.renameRow}>
                  <input
                    className={styles.input}
                    type="text"
                    value={renameValue}
                    aria-label="Preset name"
                    autoFocus
                    onChange={(event) => setRenameValue(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter") handleSaveRename("preset", preset.name);
                      if (event.key === "Escape") setRenamingKey(null);
                    }}
                  />
                  <Button variant="secondary" icon={Check} onClick={() => handleSaveRename("preset", preset.name)} aria-label="Save" />
                  <Button variant="secondary" icon={X} onClick={() => setRenamingKey(null)} aria-label="Cancel" />
                </li>
              ) : (
                <li key={`preset-${preset.id}`} className={styles.lookRow}>
                  <span className={styles.lookKind}>Preset ({preset.kind})</span>
                  <span title={preset.name}>{preset.name}</span>
                  <span className={styles.rowActions}>
                    <Button variant="secondary" icon={Pencil} onClick={() => handleStartRename("preset", preset)} aria-label={`Rename ${preset.name}`} />
                    <Button variant="destructive" icon={Trash2} onClick={() => handleDelete("preset", preset.name, "preset")} aria-label={`Delete ${preset.name}`} />
                  </span>
                </li>
              ),
            )}
          </ul>
        ) : null}
      </ScrollRegion>

      <div className={styles.divider} />

      <PanelHeader
        label="Blend Presets"
        icon={Palette}
        action={
          <Button variant={activeForm === "blend" ? "primary" : "secondary"} icon={Plus} onClick={() => toggle("blend")}>
            + Blend
          </Button>
        }
      />

      {activeForm === "blend" ? (
        <div className={styles.form}>
          <input
            className={styles.input}
            type="text"
            value={blendName}
            placeholder="Blend name"
            aria-label="New blend name"
            onChange={(event) => setBlendName(event.target.value)}
          />
          <div className={styles.row}>
            <input
              className={styles.inputNarrow}
              type="number"
              min={0}
              step="any"
              value={blendDuration}
              aria-label="Blend duration (bars)"
              onChange={(event) => setBlendDuration(event.target.value)}
            />
            <select
              className={styles.inputNarrow}
              value={blendCurve}
              aria-label="Blend curve"
              onChange={(event) => setBlendCurve(event.target.value)}
            >
              <option value="linear">linear</option>
              <option value="ease_in">ease_in</option>
              <option value="ease_out">ease_out</option>
            </select>
          </div>
          <Button
            variant="primary"
            icon={Check}
            onClick={() => {
              const duration = Number.parseFloat(blendDuration);
              if (blendName.trim() === "" || Number.isNaN(duration)) return;
              onCreateBlend(blendName.trim(), duration, blendCurve.trim());
              setBlendName("");
              setActiveForm(null);
            }}
          >
            Create Blend
          </Button>
        </div>
      ) : null}

      {view.blends.length === 0 ? (
        <EmptyState icon={Palette}>No blend presets yet.</EmptyState>
      ) : (
        <ul className={styles.rows} aria-label="Blend list">
          {view.blends.map((blend) =>
            renamingKey === `blend-${blend.id}` ? (
              <li key={blend.id} className={styles.renameRow}>
                <input
                  className={styles.input}
                  type="text"
                  value={renameValue}
                  aria-label="Blend name"
                  autoFocus
                  onChange={(event) => setRenameValue(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter") handleSaveRename("blend", blend.name);
                    if (event.key === "Escape") setRenamingKey(null);
                  }}
                />
                <Button variant="secondary" icon={Check} onClick={() => handleSaveRename("blend", blend.name)} aria-label="Save" />
                <Button variant="secondary" icon={X} onClick={() => setRenamingKey(null)} aria-label="Cancel" />
              </li>
            ) : (
              <li key={blend.id} className={styles.lookRow}>
                <span title={blend.name}>{blend.name}</span>
                <span className={styles.rowActions}>
                  <Button variant="secondary" icon={Pencil} onClick={() => handleStartRename("blend", blend)} aria-label={`Rename ${blend.name}`} />
                  <Button variant="destructive" icon={Trash2} onClick={() => handleDelete("blend", blend.name, "blend preset")} aria-label={`Delete ${blend.name}`} />
                </span>
              </li>
            ),
          )}
        </ul>
      )}
    </div>
  );
}
