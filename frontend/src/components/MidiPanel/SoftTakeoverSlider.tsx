// SoftTakeoverSlider.tsx renders the D-09/D-10/D-11 live-position visual
// track with a ghost/target marker for one continuous CC/fader MIDI
// mapping. While not armed, the fill tracks the live physical position in
// a distinct pickup visual state (muted/translucent) and a fixed ghost/
// target marker (Signal Blue accent) shows the app's current value the
// physical fader must cross; once armed (feedback.armed), the fill
// switches to the armed status color and tracks the controlling value,
// and the ghost marker disappears (armed means physical === appValue,
// making a separate marker redundant). Only continuous CC/fader mappings
// render this component -- Note/button mappings render a Chip only
// (D-12, see MidiPanel.tsx), never a takeover slider.
//
// This component is purely presentational: it receives the latest
// MidiFeedback for its own mapping as a prop (MidiPanel.tsx owns the
// "midi:feedback" EventsOn subscription and keys pushes by mappingId) and
// never calls a Wails binding itself.
//
// Phase 13 Plan 28: the visual track/fill/ghost marker is unavoidable
// domain geometry (design-system/exceptions.json
// has the one shorthand exception it still needs) and stays decorative
// (aria-hidden) rather than the misleading former role="slider" -- a
// non-interactive element with no keyboard support was never a real ARIA
// slider to begin with. The shared design-system MidiPickup pattern now
// owns the actual accessible status announcement (physical/target/armed),
// adopting D-05's "public actions/statuses" requirement instead of a
// second, redundant local chip.
import { MidiPickup } from "../../design-system";
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
        aria-hidden="true"
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
      <MidiPickup value={Math.round(physicalPct)} target={Math.round(ghostPct)} pickedUp={armed} />
    </div>
  );
}
