// Field is the shared labeled-input wrapper: a mono-caps label above a
// native text/number input. Select/textarea variants reuse the same label
// styling by passing children instead of relying on the built-in <input>.
import type { InputHTMLAttributes, ReactNode } from "react";

import styles from "./Field.module.css";

interface FieldProps extends InputHTMLAttributes<HTMLInputElement> {
  label: string;
  children?: ReactNode;
}

export default function Field({ label, children, id, className, ...rest }: FieldProps) {
  const inputId = id ?? `field-${label.toLowerCase().replace(/\s+/g, "-")}`;
  return (
    <label className={styles.field} htmlFor={children ? undefined : inputId}>
      <span className={styles.label}>{label}</span>
      {children ?? <input id={inputId} className={`${styles.input} ${className ?? ""}`} {...rest} />}
    </label>
  );
}
