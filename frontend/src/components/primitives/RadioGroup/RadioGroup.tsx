import { useId } from "react";
import { Radio } from "@base-ui/react/radio";
import { RadioGroup as BaseRadioGroup } from "@base-ui/react/radio-group";

import styles from "./RadioGroup.module.css";

export interface RadioGroupOption {
  value: string;
  label: string;
  disabled?: boolean;
}

export interface RadioGroupProps {
  label: string;
  options: ReadonlyArray<RadioGroupOption>;
  value?: string;
  defaultValue?: string;
  onValueChange?: (value: string) => void;
  disabled?: boolean;
  "aria-label"?: string;
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
}: RadioGroupProps) {
  const generatedId = useId();
  const labelId = `radio-group-${generatedId}-label`;
  const isControlled = value !== undefined;

  return (
    <div className={styles.root}>
      <span id={labelId} className={styles.label}>
        {label}
      </span>
      <BaseRadioGroup
        className={styles.options}
        value={isControlled ? value : undefined}
        defaultValue={isControlled ? undefined : defaultValue}
        disabled={disabled}
        aria-label={ariaLabel}
        aria-labelledby={ariaLabel ? undefined : labelId}
        onValueChange={(next) => onValueChange?.(next as string)}
      >
        {options.map((option) => (
          <label key={option.value} className={styles.option} data-disabled={disabled || option.disabled ? "true" : undefined}>
            <Radio.Root className={styles.radio} value={option.value} disabled={disabled || option.disabled}>
              <Radio.Indicator className={styles.dot} />
            </Radio.Root>
            <span className={styles.optionLabel}>{option.label}</span>
          </label>
        ))}
      </BaseRadioGroup>
    </div>
  );
}
