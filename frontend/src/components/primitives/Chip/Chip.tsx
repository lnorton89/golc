// Chip is the shared pill/status-indicator primitive. `tone` maps to the
// brand's semantic status vocabulary (index.css --status-*), which is
// explicitly documented as separate from the 60/30/10 color split -- never
// pass "accent" here for a routine selection state, only for the six named
// live/frame-lock/armed/revoked/blackout/offline meanings.
//
// Each non-neutral tone renders a matching small glyph (icon/polish pass)
// so status is never conveyed by color alone -- neutral stays icon-less,
// it carries no semantic meaning to reinforce.
import type { ReactNode } from "react";
import { CircleDot, CircleCheck, CircleAlert, CircleX, CircleSlash, CircleDashed, type LucideIcon } from "lucide-react";

import styles from "./Chip.module.css";

export type ChipTone = "neutral" | "live" | "frame-lock" | "armed" | "revoked" | "blackout" | "offline";

interface ChipProps {
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
    <span className={`${styles.chip} ${TONE_CLASS[tone]}`}>
      {Icon ? <Icon size={11} className={styles.icon} aria-hidden="true" /> : null}
      {children}
    </span>
  );
}
