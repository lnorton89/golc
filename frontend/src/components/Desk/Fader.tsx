// Fader.tsx is one vertical DMX channel fader (Perform > Desk, a
// QLC+-style Simple Desk): a native <input type="range"> rendered vertical
// via the standard -webkit-appearance: slider-vertical/writing-mode:
// vertical-lr combination (Desk.module.css) -- there is no slider/fader
// component anywhere else in this codebase to reuse (09-RESEARCH.md
// finding), so this is a small, self-contained primitive rather than a new
// design-system component. value is always the raw 0-255 DMX byte the
// parent already resolved (either the live polled value or a locally
// tracked override) -- Fader itself never normalizes or interprets it.
import { X } from "lucide-react";
import type { LucideIcon } from "lucide-react";

import styles from "./Desk.module.css";

interface FaderProps {
  label: string;
  /** swatch, when given (a CSS color), renders a colored circle instead of
   * a text/icon label -- the four RGBW color-mixing channels, which read
   * more directly as "the red channel" via an actual red dot than via the
   * letter "R". Takes priority over icon when both are somehow given. */
  swatch?: string;
  /** icon, when given, replaces the text label with a compact glyph
   * (DeskKey in Desk.tsx explains what each icon means) -- used for
   * capability labels too long to keep the fader column narrow
   * ("Intensity", "Strobe"), mirroring swatch's identical
   * "no per-channel free-text label exists" reasoning
   * (capabilityLabel's own doc comment in Desk.tsx). label itself is still
   * used for the accessible name/tooltip regardless. */
  icon?: LucideIcon;
  sublabel: string;
  value: number;
  overridden: boolean;
  onChange: (value: number) => void;
  onClear: () => void;
}

export default function Fader({
  label,
  swatch,
  icon: Icon,
  sublabel,
  value,
  overridden,
  onChange,
  onClear,
}: FaderProps) {
  return (
    <div className={styles.fader}>
      <span className={styles.faderValue}>{value}</span>
      <input
        className={overridden ? styles.faderInputOverridden : styles.faderInput}
        type="range"
        min={0}
        max={255}
        step={1}
        value={value}
        onChange={(event) => onChange(Number(event.target.value))}
        aria-label={`${label} (${sublabel})`}
        title={overridden ? `${label} -- manually overridden` : label}
      />
      <span className={styles.faderLabelSlot} title={label}>
        {swatch ? (
          <span className={styles.faderLabelSwatch} style={{ background: swatch }} aria-hidden="true" />
        ) : Icon ? (
          <Icon size={14} aria-hidden="true" />
        ) : (
          <span className={styles.faderLabelText}>{label}</span>
        )}
      </span>
      <span className={styles.faderSublabel} title={sublabel}>
        {sublabel}
      </span>
      <button
        type="button"
        className={styles.faderClearButton}
        onClick={onClear}
        disabled={!overridden}
        title={overridden ? "Release override back to programmed output" : "Not overridden"}
      >
        <X size={11} aria-hidden="true" />
      </button>
    </div>
  );
}
