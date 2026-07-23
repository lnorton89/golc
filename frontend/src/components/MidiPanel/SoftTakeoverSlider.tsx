// SoftTakeoverSlider.tsx renders the D-09/D-10/D-11 live-position slider
// with a ghost/target marker for one continuous CC/fader MIDI mapping.
// While not armed, the fill tracks the live physical position in a
// distinct pickup visual state (muted/translucent) and a fixed ghost/
// target marker (Signal Blue accent) shows the app's current value the
// physical fader must cross; once armed (feedback.armed), the fill
// switches to the armed status color and tracks the controlling value,
// and the ghost marker disappears (armed means physical === appValue,
// making a separate marker redundant). Only continuous CC/fader mappings
// render this component -- Note/button mappings render an armed chip
// only (D-12, see MidiPanel.tsx), never a takeover slider.
//
// This component is purely presentational: it receives the latest
// MidiFeedback for its own mapping as a prop (MidiPanel.tsx owns the
// "midi:feedback" EventsOn subscription and keys pushes by mappingId) and
// never calls a Wails binding itself.

import styles from "./MidiPanel.module.css";
import type { MidiFeedback } from "../../lib/wailsBridge";

interface SoftTakeoverSliderProps {
  feedback?: MidiFeedback;
}

function clampPercent(value: number): number {
  if (Number.isNaN(value)) return 0;
  return Math.min(100, Math.max(0, value * 100));
}

export default function SoftTakeoverSlider({ feedback }: SoftTakeoverSliderProps) {
  const armed = feedback?.armed ?? false;
  const physicalPct = clampPercent(feedback?.physical ?? 0);
  const ghostPct = clampPercent(feedback?.appValue ?? 0);

  return (
    <div className={styles.takeoverRow}>
      <div
        className={`${styles.takeoverTrack} ${armed ? styles.takeoverArmed : styles.takeoverPickup}`}
        role="slider"
        aria-label="Soft-takeover fader position"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={Math.round(physicalPct)}
      >
        <div className={styles.takeoverFill} style={{ width: `${physicalPct}%` }} />
        {!armed && (
          <div
            className={styles.takeoverGhost}
            style={{ left: `${ghostPct}%` }}
            title="Target: the app's current value the fader must cross"
          />
        )}
      </div>
      <span className={`${styles.armedChip} ${armed ? styles.armedChipOn : styles.armedChipOff}`}>
        {armed ? "Armed" : "Not armed"}
      </span>
    </div>
  );
}
