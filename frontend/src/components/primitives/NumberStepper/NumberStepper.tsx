import { forwardRef } from "react";
import type { FocusEventHandler, KeyboardEventHandler } from "react";
import { ChevronUp, ChevronDown } from "lucide-react";

import styles from "./NumberStepper.module.css";

export interface NumberStepperProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  min?: number;
  /** step is the nudge amount applied per click (default 1) -- callers
   * needing sub-integer precision (e.g. TempoControls' 0.1 BPM nudge)
   * pass an explicit fractional step; rounding uses a fixed-precision
   * factor so repeated nudges never drift from floating-point error. */
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
}

/** NumberStepper is a labeled numeric field with a compact nudge affordance
 * (+/-step per click, default 1) alongside direct typing and the input's
 * native keyboard stepping. The nudge buttons are excluded from the tab
 * order (tabIndex={-1}): they are a pointer-only convenience layered on
 * the field, not an independent keyboard control. */
const NumberStepper = forwardRef<HTMLInputElement, NumberStepperProps>(function NumberStepper(
  { label, value, onChange, min = 1, step: stepAmount = 1, placeholder, disabled = false, description, onBlur, onKeyDown },
  ref,
) {
  const step = (direction: 1 | -1) => {
    const parsed = Number(value) || 0;
    const next = Math.max(min, Math.round((parsed + direction * stepAmount) * 1e6) / 1e6);
    onChange(String(next));
  };

  return (
    <div className={styles.field}>
      <label className={styles.label}>{label}</label>
      <span className={styles.controls} data-disabled={disabled ? "true" : undefined}>
        <input
          ref={ref}
          className={styles.input}
          type="number"
          min={min}
          value={value}
          disabled={disabled}
          placeholder={placeholder}
          aria-label={label}
          onChange={(event) => onChange(event.target.value)}
          onBlur={onBlur}
          onKeyDown={onKeyDown}
        />
        <span className={styles.spinner}>
          <button
            type="button"
            className={styles.stepControl}
            tabIndex={-1}
            disabled={disabled}
            aria-label={`Increase ${label.toLowerCase()}`}
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => step(1)}
          >
            <ChevronUp className={styles.spinnerIcon} aria-hidden="true" />
          </button>
          <button
            type="button"
            className={styles.stepControl}
            tabIndex={-1}
            disabled={disabled}
            aria-label={`Decrease ${label.toLowerCase()}`}
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => step(-1)}
          >
            <ChevronDown className={styles.spinnerIcon} aria-hidden="true" />
          </button>
        </span>
      </span>
      {description ? <div className={styles.description}>{description}</div> : null}
    </div>
  );
});

export default NumberStepper;
