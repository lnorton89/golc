import { Switch as BaseSwitch } from "@base-ui/react/switch";

import styles from "./Switch.module.css";

export interface SwitchProps {
  label: string;
  checked?: boolean;
  defaultChecked?: boolean;
  onCheckedChange?: (checked: boolean) => void;
  disabled?: boolean;
  /** hideLabel drops the visible text beside the track -- mirrors
   * Checkbox's/Slider's own hideLabel convention for a caller embedding
   * this control where the track alone (with its accessible name carried
   * by aria-label instead) fits a compact layout better than the default
   * track+label row. This is a genuine drop-in upgrade for this app's
   * existing ad-hoc "Button styled as a toggle with data-active/
   * aria-pressed" pattern (NavTooltipsToggle.tsx) -- a real switch role
   * with native on/off semantics, for any future toggle that doesn't need
   * that pattern's icon+soft-disabled specifics. */
  hideLabel?: boolean;
}

/** Switch is a binary on/off toggle built on Base UI's unstyled Switch.
 * Switch.Root renders as a <span role="switch"> that receives keyboard
 * focus directly (confirmed against the rendered DOM, same as Checkbox's
 * Root) alongside a visually hidden native <input type="checkbox">, and
 * unlike Slider.Thumb, Switch.Thumb sets no positioning styles of its own
 * -- this primitive's own CSS owns the thumb's slide entirely via a
 * data-checked-keyed transform. Wrapping the whole thing in a <label>
 * auto-wires the visible text as the accessible name via
 * aria-labelledby, the same pattern used for Checkbox. */
export default function Switch({
  label,
  checked,
  defaultChecked,
  onCheckedChange,
  disabled = false,
  hideLabel = false,
}: SwitchProps) {
  const isControlled = checked !== undefined;

  const track = (
    <BaseSwitch.Root
      className={styles.track}
      checked={isControlled ? checked : undefined}
      defaultChecked={isControlled ? undefined : (defaultChecked ?? false)}
      disabled={disabled}
      aria-label={hideLabel ? label : undefined}
      onCheckedChange={(next) => onCheckedChange?.(next)}
    >
      <BaseSwitch.Thumb className={styles.thumb} />
    </BaseSwitch.Root>
  );

  if (hideLabel) return track;

  return (
    <label className={styles.root}>
      {track}
      <span className={styles.label}>{label}</span>
    </label>
  );
}
