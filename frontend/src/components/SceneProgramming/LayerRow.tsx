// LayerRow is one of a scene's four fixed layers (Base Look/Color Theme/
// Chase/Motion -- programming-scene-authoring.md's Scene Stack). Disabling
// a layer preserves and continues to display its reference (never hides
// it) -- the select's value persists regardless of the enabled toggle.
import type { ProgLookView, ProgPresetView } from "../../lib/wailsBridge";
import { Button, Field } from "../../design-system";
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
      <Button variant={enabled ? "primary" : "secondary"} aria-pressed={enabled} onClick={onToggle}>
        {layerKindLabel(kind)}
      </Button>
      <Field label={`${layerKindLabel(kind)} look`}>
        <select value={refId} onChange={(event) => onSelectLook(event.target.value)}>
          <option value="" disabled>
            {looks.length === 0 ? "No looks available" : "Select a look…"}
          </option>
          {looks.map((look) => (
            <option key={look.id} value={look.id}>
              {look.name}
            </option>
          ))}
        </select>
      </Field>
    </li>
  );
}
