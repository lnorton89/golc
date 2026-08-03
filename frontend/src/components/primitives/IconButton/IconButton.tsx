import { forwardRef } from "react";
import type { ButtonHTMLAttributes } from "react";
import type { LucideIcon } from "lucide-react";

import styles from "./IconButton.module.css";

export type IconButtonVariant = "neutral" | "primary" | "destructive";
export type IconButtonSize = "default" | "target";

export interface IconButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "aria-label" | "children"> {
  icon: LucideIcon;
  label: string;
  variant?: IconButtonVariant;
  size?: IconButtonSize;
  loading?: boolean;
}

const VARIANT_CLASS: Record<IconButtonVariant, string> = {
  neutral: styles.neutral,
  primary: styles.primary,
  destructive: styles.destructive,
};

const SIZE_CLASS: Record<IconButtonSize, string> = {
  default: styles.default,
  target: styles.target,
};

const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(function IconButton(
  { icon: Icon, label, variant = "neutral", size = "target", loading = false, disabled = false, className, type = "button", ...rest },
  ref,
) {
  if (!label.trim()) {
    throw new Error("IconButton requires a non-empty accessible label.");
  }

  const combinedClassName = [styles.button, VARIANT_CLASS[variant], SIZE_CLASS[size], className].filter(Boolean).join(" ");
  return (
    <button
      ref={ref}
      type={type}
      className={combinedClassName}
      aria-label={label}
      aria-busy={loading || undefined}
      disabled={disabled || loading}
      {...rest}
    >
      {loading ? <span className={styles.spinner} aria-hidden="true" /> : <Icon className={styles.icon} aria-hidden="true" />}
    </button>
  );
});

export default IconButton;
