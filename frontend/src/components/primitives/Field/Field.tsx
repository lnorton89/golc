import { cloneElement, forwardRef, isValidElement, useId } from "react";
import type { AriaAttributes, InputHTMLAttributes, ReactElement, ReactNode } from "react";

import styles from "./Field.module.css";

type FieldControlProps = {
  id?: string;
  className?: string;
  disabled?: boolean;
  required?: boolean;
  "aria-describedby"?: string;
  "aria-invalid"?: AriaAttributes["aria-invalid"];
  "data-multiline"?: string;
};

export interface FieldProps extends InputHTMLAttributes<HTMLInputElement> {
  label: string;
  description?: ReactNode;
  error?: ReactNode;
  children?: ReactNode;
  /** hideLabel drops the visible stacked <label> line -- for a caller
   * embedding this field inline in a single-row control cluster (a Button
   * beside it expecting the same natural height, e.g. SaveRecoveryWorkspace's
   * Save/"Save As destination path"/Save As row) rather than this
   * primitive's default label-above-input block layout, which is taller
   * than a plain button and forces every sibling in a stretched flex row
   * to match that height. The control's own `aria-label` (added below)
   * keeps the accessible name regardless. */
  hideLabel?: boolean;
}

function joinIds(...ids: Array<string | undefined>) {
  const joined = ids.filter(Boolean).join(" ");
  return joined || undefined;
}

const Field = forwardRef<HTMLInputElement, FieldProps>(function Field(
  { label, description, error, children, id, className, disabled, required, hideLabel = false, "aria-describedby": ariaDescribedBy, "aria-invalid": ariaInvalid, ...rest },
  ref,
) {
  const generatedId = useId();
  const controlId = id ?? `field-${generatedId}`;
  const descriptionId = description ? `${controlId}-description` : undefined;
  const errorId = error ? `${controlId}-error` : undefined;
  const describedBy = joinIds(ariaDescribedBy, descriptionId, errorId);
  const invalid = error ? true : ariaInvalid;

  const child = isValidElement(children) ? (children as ReactElement<FieldControlProps>) : null;
  const control = child
    ? cloneElement(child, {
        id: controlId,
        className: [styles.input, child.props.className].filter(Boolean).join(" "),
        disabled: disabled ?? child.props.disabled,
        required: required ?? child.props.required,
        "aria-describedby": joinIds(child.props["aria-describedby"], describedBy),
        "aria-invalid": invalid,
        ...(hideLabel ? { "aria-label": label } : {}),
        // Marks a <textarea> child so Field.module.css's [data-multiline]
        // rule (taller min-height, resizable) applies -- an explicit
        // attribute rather than a `textarea.input` tag selector, so any
        // future non-<textarea> multiline control could opt in too.
        ...(child.type === "textarea" ? { "data-multiline": "" } : {}),
      })
    : (
        <input
          ref={ref}
          id={controlId}
          className={[styles.input, className].filter(Boolean).join(" ")}
          disabled={disabled}
          required={required}
          aria-describedby={describedBy}
          aria-invalid={invalid}
          aria-label={hideLabel ? label : undefined}
          {...rest}
        />
      );

  return (
    <div className={styles.field} data-invalid={error ? "true" : undefined}>
      {!hideLabel && (
        <label className={styles.label} htmlFor={controlId}>
          {label}
          {required ? <span className={styles.required} aria-hidden="true"> *</span> : null}
        </label>
      )}
      {control}
      {description ? <div id={descriptionId} className={styles.description}>{description}</div> : null}
      {error ? <div id={errorId} className={styles.error} role="alert">{error}</div> : null}
    </div>
  );
});

export default Field;
