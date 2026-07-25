// LayerRow is one of a scene's four fixed layers (Base Look/Color Theme/
// Chase/Motion -- programming-scene-authoring.md's Scene Stack). Disabling
// a layer preserves and continues to display its reference (never hides
// it) -- the select's value persists regardless of the enabled toggle.
import type { ProgLookView, ProgPresetView } from "../../lib/wailsBridge";
import styles from "./LayerRow.module.css";

export type LayerKind = "base_look" | "color_theme" | "chase" | "motion";

export const LAYER_KINDS: readonly LayerKind[] = ["base_look", "color_theme", "chase", "motion"];

export function layerKindLabel(kind: string): string {
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

interface LayerRowProps {
  kind: LayerKind;
  enabled: boolean;
  refId: string;
  looks: (ProgLookView | ProgPresetView)[];
  onToggle: () => void;
  onSelectLook: (refId: string) => void;
}

export default function LayerRow({ kind, enabled, refId, looks, onToggle, onSelectLook }: LayerRowProps) {
  return (
    <li className={enabled ? styles.row : `${styles.row} ${styles.rowDisabled}`}>
      <button type="button" aria-pressed={enabled} onClick={onToggle} className={styles.toggle}>
        {layerKindLabel(kind)}
      </button>
      <select
        value={refId}
        onChange={(event) => onSelectLook(event.target.value)}
        aria-label={`${layerKindLabel(kind)} look`}
        className={styles.lookSelect}
      >
        <option value="" disabled>
          {looks.length === 0 ? "No looks available" : "Select a look…"}
        </option>
        {looks.map((look) => (
          <option key={look.id} value={look.id}>
            {look.name}
          </option>
        ))}
      </select>
    </li>
  );
}
