import { useId } from "react";
import { Radio } from "@base-ui/react/radio";
import { RadioGroup as BaseRadioGroup } from "@base-ui/react/radio-group";

import styles from "./RadioGroup.module.css";

export interface RadioGroupOption {
  value: string;
  label: string;
  disabled?: boolean;
  /** A decorative color swatch shown before the label (a palette's own
   * accent color, say) -- only meaningful with `layout="wrap"` (see
   * RadioGroupProps.layout); ignored by the default "stacked" layout. */
  swatch?: string;
}

export interface RadioGroupProps {
  label: string;
  options: ReadonlyArray<RadioGroupOption>;
  value?: string;
  defaultValue?: string;
  onValueChange?: (value: string) => void;
  disabled?: boolean;
  "aria-label"?: string;
  /** "stacked" (default): a vertical list of radio-dot-plus-label rows,
   * each option's own `<label>` supplying its accessible name. "wrap": a
   * wrapping row of pill-shaped options (each option's own Radio.Root IS
   * the clickable pill, selection shown via its filled/outlined state
   * rather than a separate dot) -- for a small set of visually distinct
   * choices like a color-palette picker, where a vertical dot-list would
   * both look wrong and bury the swatches' whole point (comparing several
   * of them at a glance). */
  layout?: "stacked" | "wrap";
  /** Visually hides the group's own `label` (via the standard ds-sr-only
   * technique) while keeping it as the group's accessible name -- for a
   * caller that already renders its own visible heading above the group
   * (matching a sibling control's heading treatment exactly, say) and
   * would otherwise end up with the label rendered twice. */
  hideLabel?: boolean;
}

/** RadioGroup is a labeled set of mutually-exclusive options built on Base
 * UI's unstyled RadioGroup + Radio. Each Radio.Root renders as a <span
 * role="radio"> that receives keyboard focus directly (confirmed against
 * the rendered DOM), with a hidden native <input type="radio"> carrying
 * the real form value -- arrow-key movement between options is native
 * roving-tabindex behavior Base UI wires up itself, not something this
 * primitive re-implements. Wrapping each option's Radio.Root in a <label>
 * auto-wires that option's own accessible name; the group's own visible
 * heading is wired to the root via aria-labelledby (Base UI's documented
 * pattern for labeling a RadioGroup as a whole, since RadioGroup has no
 * built-in visible-label slot the way Checkbox/Switch's own wrapping
 * <label> covers a single control). */
export default function RadioGroup({
  label,
  options,
  value,
  defaultValue,
  onValueChange,
  disabled = false,
  "aria-label": ariaLabel,
  layout = "stacked",
  hideLabel = false,
}: RadioGroupProps) {
  const generatedId = useId();
  const labelId = `radio-group-${generatedId}-label`;
  const isControlled = value !== undefined;

  return (
    <div className={styles.root}>
      <span id={labelId} className={hideLabel ? `${styles.label} ds-sr-only` : styles.label}>
        {label}
      </span>
      <BaseRadioGroup
        className={styles.options}
        data-layout={layout}
        value={isControlled ? value : undefined}
        defaultValue={isControlled ? undefined : defaultValue}
        disabled={disabled}
        aria-label={ariaLabel}
        aria-labelledby={ariaLabel ? undefined : labelId}
        onValueChange={(next) => onValueChange?.(next as string)}
      >
        {options.map((option) =>
          layout === "wrap" ? (
            <Radio.Root
              key={option.value}
              className={styles.pill}
              value={option.value}
              disabled={disabled || option.disabled}
            >
              {option.swatch ? <span className={styles.swatch} style={{ background: option.swatch }} aria-hidden="true" /> : null}
              <span className={styles.optionLabel}>{option.label}</span>
            </Radio.Root>
          ) : (
            <label key={option.value} className={styles.option} data-disabled={disabled || option.disabled ? "true" : undefined}>
              <Radio.Root className={styles.radio} value={option.value} disabled={disabled || option.disabled}>
                <Radio.Indicator className={styles.dot} />
              </Radio.Root>
              <span className={styles.optionLabel}>{option.label}</span>
            </label>
          ),
        )}
      </BaseRadioGroup>
    </div>
  );
}
