// Chip is the shared pill/status-indicator primitive. `tone` maps to the
// brand's semantic status vocabulary (index.css --status-*), which is
// explicitly documented as separate from the 60/30/10 color split -- never
// pass "accent" here for a routine selection state, only for the six named
// live/frame-lock/armed/revoked/blackout/offline meanings.
import type { ReactNode } from "react";

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

export default function Chip({ tone = "neutral", children }: ChipProps) {
  return <span className={`${styles.chip} ${TONE_CLASS[tone]}`}>{children}</span>;
}
