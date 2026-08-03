import type { ReactNode } from "react";
import { CircleAlert, CircleCheck, CircleDashed, CircleDot, CircleSlash, CircleX, type LucideIcon } from "lucide-react";

import styles from "./Chip.module.css";

export type ChipTone = "neutral" | "live" | "frame-lock" | "armed" | "revoked" | "blackout" | "offline";

export interface ChipProps {
  tone?: ChipTone;
  children: ReactNode;
}

const TONE_CLASS: Record<ChipTone, string> = {
  neutral: styles.neutral,
  live: styles.live,
  "frame-lock": styles.frameLock,
  armed: styles.armed,
  revoked: styles.revoked,
  blackout: styles.blackout,
  offline: styles.offline,
};

const TONE_ICON: Record<ChipTone, LucideIcon | null> = {
  neutral: null,
  live: CircleDot,
  "frame-lock": CircleCheck,
  armed: CircleAlert,
  revoked: CircleX,
  blackout: CircleSlash,
  offline: CircleDashed,
};

export default function Chip({ tone = "neutral", children }: ChipProps) {
  const Icon = TONE_ICON[tone];
  return (
    <span
      className={`${styles.chip} ${TONE_CLASS[tone]}`}
      data-status={tone}
      role={tone === "neutral" ? undefined : "status"}
      aria-label={tone === "neutral" ? undefined : typeof children === "string" ? children : `${tone} status`}
    >
      {Icon ? <Icon className={styles.icon} aria-hidden="true" /> : null}
      {children}
    </span>
  );
}
