// MidiLearnToggle.tsx is the global MIDI Learn switch: rendered in
// GlobalFrame.tsx immediately after SafetyCluster, so it reads as one
// persistent header cluster alongside Blackout/Automation/Stop-Release-All
// despite being a plain click-to-toggle (not hold-to-confirm -- unlike
// those three, turning this on is reversible and non-destructive, so no
// confirm gesture is warranted). Toggling it on flips store.ts's
// `midiLearnMode` flag, which Desk.tsx (and, in future, any other
// MIDI-mappable view) reads to decide whether to highlight its own
// mappable controls -- this component owns no mapping logic itself, only
// the on/off switch.
import { Radio } from "lucide-react";

import { useGolcStore } from "../../store/store";
import styles from "./MidiLearnToggle.module.css";

export default function MidiLearnToggle() {
  const midiLearnMode = useGolcStore((state) => state.midiLearnMode);
  const setMidiLearnMode = useGolcStore((state) => state.setMidiLearnMode);

  return (
    <button
      type="button"
      className={styles.toggle}
      data-active={midiLearnMode || undefined}
      aria-pressed={midiLearnMode}
      aria-label={midiLearnMode ? "Exit MIDI Learn mode" : "Enter MIDI Learn mode"}
      title={
        midiLearnMode
          ? "MIDI Learn is on -- click a highlighted control, then move a MIDI control to map it"
          : "Turn on MIDI Learn to map Desk faders to MIDI controls"
      }
      onClick={() => setMidiLearnMode(!midiLearnMode)}
    >
      <Radio size={14} className={styles.icon} aria-hidden="true" />
      <span className={styles.label}>MIDI Learn</span>
    </button>
  );
}
