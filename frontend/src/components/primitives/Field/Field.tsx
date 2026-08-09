import { forwardRef, isValidElement, useId } from "react";
import type { InputHTMLAttributes, ReactElement, ReactNode } from "react";
import { Field as BaseField } from "@base-ui/react/field";

import styles from "./Field.module.css";

type FieldControlProps = {
  className?: string;
  disabled?: boolean;
  required?: boolean;
  "aria-describedby"?: string;
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
  // Base UI's Field.Description/Field.Error auto-generate their own ids and
  // append them to Field.Control's aria-describedby via context (verified
  // empirically -- see the now-deleted scratch test this conversion used),
  // so only a caller-supplied describedby (and, for the cloned-child branch,
  // any describedby already on that child) needs to be threaded through by
  // hand here.
  const invalid = error ? true : ariaInvalid;
  const hasError = Boolean(error);

  const child = isValidElement(children) ? (children as ReactElement<FieldControlProps>) : null;
  const control = child
    ? (
        <BaseField.Control
          id={controlId}
          className={[styles.input, child.props.className].filter(Boolean).join(" ")}
          disabled={disabled ?? child.props.disabled}
          required={required ?? child.props.required}
          aria-describedby={joinIds(ariaDescribedBy, child.props["aria-describedby"])}
          aria-invalid={invalid}
          {...(hideLabel ? { "aria-label": label } : {})}
          // Marks a <textarea> child so Field.module.css's [data-multiline]
          // rule (taller min-height, resizable) applies -- an explicit
          // attribute rather than a `textarea.input` tag selector, so any
          // future non-<textarea> multiline control could opt in too.
          {...(child.type === "textarea" ? { "data-multiline": "" } : {})}
          render={child}
        />
      )
    : (
        <BaseField.Control
          ref={ref}
          id={controlId}
          className={[styles.input, className].filter(Boolean).join(" ")}
          disabled={disabled}
          required={required}
          aria-describedby={ariaDescribedBy}
          aria-invalid={invalid}
          aria-label={hideLabel ? label : undefined}
          {...rest}
        />
      );

  return (
    <BaseField.Root className={styles.field} invalid={hasError} data-invalid={hasError ? "true" : undefined}>
      {!hideLabel && (
        <BaseField.Label className={styles.label}>
          {label}
          {required ? <span className={styles.required} aria-hidden="true"> *</span> : null}
        </BaseField.Label>
      )}
      {control}
      {description ? <BaseField.Description className={styles.description}>{description}</BaseField.Description> : null}
      {error ? (
        <BaseField.Error className={styles.error} match={hasError} role="alert">
          {error}
        </BaseField.Error>
      ) : null}
    </BaseField.Root>
  );
});

export default Field;
