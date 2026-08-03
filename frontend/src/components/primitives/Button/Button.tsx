// Button is the shared action primitive: primary (Signal Blue, the one
// reserved 60/30/10 accent use for primary actions), secondary (bordered,
// --panel), and destructive (--status-revoked, for Remove/Unassign-class
// actions). A native <button> underneath -- no Radix here, nothing in this
// primitive needs its composition (see shell restructure plan Step 8).
//
// `icon` is an optional leading glyph (lucide-react component reference,
// not an element -- callers pass `icon={Save}`, not `icon={<Save />}`, so
// this primitive controls size/aria-hidden consistently everywhere). Purely
// additive: every existing text-only call site is unaffected.
import { forwardRef } from "react";
import type { ButtonHTMLAttributes } from "react";
import type { LucideIcon } from "lucide-react";

import styles from "./Button.module.css";

export type ButtonVariant = "primary" | "secondary" | "destructive";
export type ButtonSize = "compact" | "default" | "target";

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  /** @deprecated Use leadingIcon for new call sites. */
  icon?: LucideIcon;
  leadingIcon?: LucideIcon;
  trailingIcon?: LucideIcon;
  loading?: boolean;
}

const VARIANT_CLASS: Record<ButtonVariant, string> = {
  primary: styles.primary,
  secondary: styles.secondary,
  destructive: styles.destructive,
};

const SIZE_CLASS: Record<ButtonSize, string> = {
  compact: styles.compact,
  default: styles.default,
  target: styles.target,
};

const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  {
    variant = "secondary",
    size = "default",
    icon: legacyIcon,
    leadingIcon,
    trailingIcon: TrailingIcon,
    loading = false,
    disabled = false,
    className,
    type = "button",
    children,
    ...rest
  },
  ref,
) {
  const LeadingIcon = leadingIcon ?? legacyIcon;
  const combinedClassName = [styles.button, VARIANT_CLASS[variant], SIZE_CLASS[size], className].filter(Boolean).join(" ");

  return (
    <button ref={ref} type={type} className={combinedClassName} disabled={disabled || loading} aria-busy={loading || undefined} {...rest}>
      {loading ? <span className={styles.spinner} aria-hidden="true" /> : null}
      {LeadingIcon ? <LeadingIcon className={styles.icon} aria-hidden="true" /> : null}
      {children}
      {TrailingIcon ? <TrailingIcon className={styles.icon} aria-hidden="true" /> : null}
    </button>
  );
});

export default Button;
