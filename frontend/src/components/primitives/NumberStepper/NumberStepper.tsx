import { forwardRef } from "react";
import type { FocusEventHandler, KeyboardEventHandler } from "react";
import { NumberField } from "@base-ui/react/number-field";
import { ChevronUp, ChevronDown } from "lucide-react";

import styles from "./NumberStepper.module.css";

export interface NumberStepperProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  min?: number;
  /** step is the nudge amount applied per click (default 1) -- callers
   * needing sub-integer precision (e.g. TempoControls' 0.1 BPM nudge)
   * pass an explicit fractional step; Base UI's NumberField owns the
   * actual increment/decrement math and keeps it drift-free internally. */
  step?: number;
  placeholder?: string;
  disabled?: boolean;
  description?: string;
  /** onBlur/onKeyDown are optional pass-throughs onto the underlying
   * input -- additive, opt-in hooks for a caller that layers its own
   * commit-on-Enter/Escape/blur semantics on top of this primitive's
   * typing/nudge affordance (TempoControls' click-to-edit BPM field)
   * without duplicating the compact input+spinner markup this primitive
   * already owns. */
  onBlur?: FocusEventHandler<HTMLInputElement>;
  onKeyDown?: KeyboardEventHandler<HTMLInputElement>;
  /** hideLabel drops the visible <label> line and narrows the input to a
   * compact fixed width -- for a caller embedding this field inline in a
   * single-row, fixed-height toolbar (TempoControls' persistent-header BPM
   * field) rather than this primitive's default stacked block-form layout,
   * which is taller/wider than a 52px chrome row can hold. The input's own
   * `aria-label` (always set, below) keeps the accessible name regardless. */
  hideLabel?: boolean;
}

/** NumberStepper is a labeled numeric field with a compact nudge affordance
 * (+/-step per click, default 1) alongside direct typing and the input's
 * native keyboard stepping. The nudge buttons are excluded from the tab
 * order (tabIndex={-1}): they are a pointer-only convenience layered on
 * the field, not an independent keyboard control.
 *
 * Built on Base UI's NumberField, whose engine works in `number | null`
 * rather than this primitive's own `string` contract -- the empty string
 * (used by optional "Auto" fields like ProjectFixtures' starting
 * universe/address) round-trips to/from `null` at this boundary so neither
 * consumer needs to change. `format` disables thousands grouping (Base
 * UI's default would otherwise render e.g. "1,234" for a universe number)
 * and caps display rounding at 6 fraction digits -- matching the previous
 * hand-rolled nudge math's own 1e6 precision floor -- so a caller typing a
 * precise fractional value never sees it visually truncated on blur. */
const NUMBER_FORMAT: Intl.NumberFormatOptions = { useGrouping: false, maximumFractionDigits: 6 };

const NumberStepper = forwardRef<HTMLInputElement, NumberStepperProps>(function NumberStepper(
  { label, value, onChange, min = 1, step: stepAmount = 1, placeholder, disabled = false, description, onBlur, onKeyDown, hideLabel = false },
  ref,
) {
  const parsed = value === "" ? null : Number(value);
  const numericValue = parsed === null || Number.isNaN(parsed) ? null : parsed;

  return (
    <div className={styles.field} data-compact={hideLabel ? "true" : undefined}>
      {!hideLabel && <label className={styles.label}>{label}</label>}
      <NumberField.Root
        className={styles.controls}
        min={min}
        step={stepAmount}
        format={NUMBER_FORMAT}
        value={numericValue}
        disabled={disabled}
        onValueChange={(next) => onChange(next === null ? "" : String(next))}
      >
        <NumberField.Group className={styles.group}>
          <NumberField.Input
            ref={ref}
            className={styles.input}
            placeholder={placeholder}
            aria-label={label}
            onBlur={onBlur}
            onKeyDown={onKeyDown}
          />
          <span className={styles.spinner}>
            <NumberField.Increment className={styles.stepControl} tabIndex={-1} aria-label={`Increase ${label.toLowerCase()}`}>
              <ChevronUp className={styles.spinnerIcon} aria-hidden="true" />
            </NumberField.Increment>
            <NumberField.Decrement className={styles.stepControl} tabIndex={-1} aria-label={`Decrease ${label.toLowerCase()}`}>
              <ChevronDown className={styles.spinnerIcon} aria-hidden="true" />
            </NumberField.Decrement>
          </span>
        </NumberField.Group>
      </NumberField.Root>
      {description ? <div className={styles.description}>{description}</div> : null}
    </div>
  );
});

export default NumberStepper;
