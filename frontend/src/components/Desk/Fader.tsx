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

import styles from "./Desk.module.css";

interface FaderProps {
  label: string;
  sublabel: string;
  value: number;
  overridden: boolean;
  onChange: (value: number) => void;
  onClear: () => void;
}

export default function Fader({ label, sublabel, value, overridden, onChange, onClear }: FaderProps) {
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
      <span className={styles.faderLabel} title={label}>
        {label}
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
