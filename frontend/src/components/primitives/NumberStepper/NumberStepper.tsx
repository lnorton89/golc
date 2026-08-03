import { forwardRef } from "react";
import { ChevronUp, ChevronDown } from "lucide-react";

import styles from "./NumberStepper.module.css";

export interface NumberStepperProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  min?: number;
  placeholder?: string;
  disabled?: boolean;
  description?: string;
}

/** NumberStepper is a labeled numeric field with a compact nudge affordance
 * (+/-1 per click) alongside direct typing and the input's native keyboard
 * stepping. The nudge buttons are excluded from the tab order
 * (tabIndex={-1}): they are a pointer-only convenience layered on the
 * field, not an independent keyboard control. */
const NumberStepper = forwardRef<HTMLInputElement, NumberStepperProps>(function NumberStepper(
  { label, value, onChange, min = 1, placeholder, disabled = false, description },
  ref,
) {
  const step = (delta: number) => {
    const parsed = Number(value) || 0;
    onChange(String(Math.max(min, Math.round(parsed + delta))));
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
