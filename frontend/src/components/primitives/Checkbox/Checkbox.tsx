import { Checkbox as BaseCheckbox } from "@base-ui/react/checkbox";
import { Check, Minus } from "lucide-react";

import styles from "./Checkbox.module.css";

export interface CheckboxProps {
  label: string;
  checked?: boolean;
  defaultChecked?: boolean;
  onCheckedChange?: (checked: boolean) => void;
  disabled?: boolean;
  /** indeterminate renders the mixed-state dash instead of a checkmark
   * (e.g. a "select all" checkbox when only some rows in the group are
   * selected) -- purely visual, independent of checked/defaultChecked,
   * matching Base UI's own Checkbox.Root `indeterminate` prop. */
  indeterminate?: boolean;
  /** hideLabel drops the visible text beside the box -- mirrors Slider's/
   * NumberStepper's hideLabel convention for a caller embedding this
   * control where the box alone (with its accessible name carried by
   * aria-label instead) fits a compact layout better than the default
   * box+label row. */
  hideLabel?: boolean;
}

/** Checkbox is a tri-state (checked/unchecked/indeterminate) selection
 * control built on Base UI's unstyled Checkbox. Base UI's Checkbox.Root
 * renders as a <span role="checkbox"> that receives focus directly
 * (verified against the rendered DOM, not assumed) alongside a visually
 * hidden native <input type="checkbox"> that carries the real form value
 * -- wrapping it in a <label> auto-wires the visible text as its
 * accessible name via aria-labelledby, the same pattern Base UI's own
 * docs use for a single labeled checkbox. */
export default function Checkbox({
  label,
  checked,
  defaultChecked,
  onCheckedChange,
  disabled = false,
  indeterminate = false,
  hideLabel = false,
}: CheckboxProps) {
  const isControlled = checked !== undefined;

  const box = (
    <BaseCheckbox.Root
      className={styles.box}
      checked={isControlled ? checked : undefined}
      defaultChecked={isControlled ? undefined : (defaultChecked ?? false)}
      indeterminate={indeterminate}
      disabled={disabled}
      aria-label={hideLabel ? label : undefined}
      onCheckedChange={(next) => onCheckedChange?.(next)}
    >
      <BaseCheckbox.Indicator className={styles.indicator}>
        {indeterminate ? <Minus className={styles.icon} aria-hidden="true" /> : <Check className={styles.icon} aria-hidden="true" />}
      </BaseCheckbox.Indicator>
    </BaseCheckbox.Root>
  );

  if (hideLabel) return box;

  return (
    <label className={styles.root}>
      {box}
      <span className={styles.label}>{label}</span>
    </label>
  );
}
