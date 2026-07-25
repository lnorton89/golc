// LookBrowser is the Scene Stack's reusable-look browser (programming-
// scene-authoring.md), published into the shell's contextual inspector via
// useInspectorSlot rather than living inline in the workspace canvas. Only
// one create-form is open at a time (toggled by category button) --
// "Permanently displaying every look-creation form" is the sketch's own
// explicit "what to avoid".
import { useState } from "react";

import type { ProgrammingView } from "../../lib/wailsBridge";
import Button from "../primitives/Button/Button";
import PanelHeader from "../primitives/PanelHeader/PanelHeader";
import ScrollRegion from "../primitives/ScrollRegion/ScrollRegion";
import styles from "./LookBrowser.module.css";

type FormKind = "theme" | "motion" | "chase" | "preset" | "blend";
export type PresetKind = "intensity" | "color" | "position" | "beam";

interface LookBrowserProps {
  view: ProgrammingView;
  onCreateTheme: (name: string) => void;
  onCreateMotion: (name: string) => void;
  onCreateChase: (name: string, unit: "bar" | "beat", stepDuration: number) => void;
  onCreateBlend: (name: string, duration: number, curve: string) => void;
  onRecordPreset: (instanceId: string, attrs: string[], kind: PresetKind, name: string) => void;
  presetLoading: boolean;
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

  const toggle = (kind: FormKind) => setActiveForm((current) => (current === kind ? null : kind));

  const looksTotal = view.themes.length + view.chases.length + view.motions.length + view.presets.length;

  return (
    <div className={styles.browser}>
      <PanelHeader label="Looks" />

      <p className={styles.countSummary}>
        {looksTotal === 0
          ? "No looks yet — create one below, then point a scene layer at it."
          : `${pluralize(view.themes.length, "theme")}, ${pluralize(view.chases.length, "chase")}, ${pluralize(view.motions.length, "motion preset")}, ${pluralize(view.presets.length, "base-look preset")}`}
      </p>

      <div className={styles.categoryRow}>
        <Button variant={activeForm === "theme" ? "primary" : "secondary"} onClick={() => toggle("theme")}>
          + Theme
        </Button>
        <Button variant={activeForm === "motion" ? "primary" : "secondary"} onClick={() => toggle("motion")}>
          + Motion
        </Button>
        <Button variant={activeForm === "chase" ? "primary" : "secondary"} onClick={() => toggle("chase")}>
          + Chase
        </Button>
        <Button variant={activeForm === "preset" ? "primary" : "secondary"} onClick={() => toggle("preset")}>
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
            {view.themes.map((look) => (
              <li key={`theme-${look.id}`} className={styles.lookRow}>
                <span className={styles.lookKind}>Theme</span>
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
                <span className={styles.lookKind}>Preset ({preset.kind})</span>
                <span title={preset.name}>{preset.name}</span>
              </li>
            ))}
          </ul>
        ) : null}
      </ScrollRegion>

      <div className={styles.divider} />

      <PanelHeader
        label="Blend Presets"
        action={
          <Button variant={activeForm === "blend" ? "primary" : "secondary"} onClick={() => toggle("blend")}>
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
        <p className={styles.emptyState}>No blend presets yet.</p>
      ) : (
        <ul className={styles.rows} aria-label="Blend list">
          {view.blends.map((blend) => (
            <li key={blend.id} className={styles.lookRow}>
              <span title={blend.name}>{blend.name}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
