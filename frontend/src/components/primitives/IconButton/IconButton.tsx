import { forwardRef } from "react";
import type { ButtonHTMLAttributes } from "react";
import type { LucideIcon } from "lucide-react";

import styles from "./IconButton.module.css";

export type IconButtonVariant = "neutral" | "primary" | "destructive";
/** "compact" (28px button, 16px icon) is for a small cluster of icon
 * buttons inside an already-dense row (a card header, a toolbar group)
 * where "default"'s 32px would force more line-wrapping than the row's
 * own layout intends -- not a substitute for "target" wherever a control
 * is the primary/sole way to reach an action under pressure. */
export type IconButtonSize = "compact" | "default" | "target";
/** "native" (default) disables via the real `disabled` attribute, same as
 * every other button in the app. "soft" is an opt-in for the rare case
 * where the button must stay hoverable/focusable/tabbable while inert --
 * a native `disabled` button does not receive hover/focus events in most
 * browsers, which silently swallows a `title` tooltip explaining why an
 * action is unavailable right when an operator goes looking for it. In
 * "soft" mode the caller remains responsible for making its own onClick a
 * no-op while `disabled` is true (exactly like the "native" mode's own
 * callers already must not rely on the click firing at all). */
export type IconButtonDisabledBehavior = "native" | "soft";

export interface IconButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "aria-label" | "children"> {
  icon: LucideIcon;
  label: string;
  variant?: IconButtonVariant;
  size?: IconButtonSize;
  loading?: boolean;
  disabledBehavior?: IconButtonDisabledBehavior;
}

const VARIANT_CLASS: Record<IconButtonVariant, string> = {
  neutral: styles.neutral,
  primary: styles.primary,
  destructive: styles.destructive,
};

const SIZE_CLASS: Record<IconButtonSize, string> = {
  compact: styles.compact,
  default: styles.default,
  target: styles.target,
};

const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(function IconButton(
  {
    icon: Icon,
    label,
    variant = "neutral",
    size = "target",
    loading = false,
    disabled = false,
    disabledBehavior = "native",
    className,
    type = "button",
    ...rest
  },
  ref,
) {
  if (!label.trim()) {
    throw new Error("IconButton requires a non-empty accessible label.");
  }

  const isSoftDisabled = disabled && !loading && disabledBehavior === "soft";
  const combinedClassName = [styles.button, VARIANT_CLASS[variant], SIZE_CLASS[size], className].filter(Boolean).join(" ");
  return (
    <button
      ref={ref}
      type={type}
      className={combinedClassName}
      aria-label={label}
      aria-busy={loading || undefined}
      aria-disabled={isSoftDisabled ? true : undefined}
      disabled={isSoftDisabled ? false : disabled || loading}
      {...rest}
    >
      {loading ? <span className={styles.spinner} aria-hidden="true" /> : <Icon className={styles.icon} aria-hidden="true" />}
    </button>
  );
});

export default IconButton;
