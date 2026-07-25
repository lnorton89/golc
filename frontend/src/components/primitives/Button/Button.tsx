// Button is the shared action primitive: primary (Signal Blue, the one
// reserved 60/30/10 accent use for primary actions), secondary (bordered,
// --panel), and destructive (--status-revoked, for Remove/Unassign-class
// actions). A native <button> underneath -- no Radix here, nothing in this
// primitive needs its composition (see shell restructure plan Step 8).
import type { ButtonHTMLAttributes } from "react";

import styles from "./Button.module.css";

export type ButtonVariant = "primary" | "secondary" | "destructive";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
}

const VARIANT_CLASS: Record<ButtonVariant, string> = {
  primary: styles.primary,
  secondary: styles.secondary,
  destructive: styles.destructive,
};

export default function Button({ variant = "secondary", className, type = "button", ...rest }: ButtonProps) {
  const combinedClassName = className
    ? `${styles.button} ${VARIANT_CLASS[variant]} ${className}`
    : `${styles.button} ${VARIANT_CLASS[variant]}`;
  return <button type={type} className={combinedClassName} {...rest} />;
}
