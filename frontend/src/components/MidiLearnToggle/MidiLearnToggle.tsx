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
//
// MIDI_LEARN_SUPPORTED_DESTINATIONS is the closed set of destinations that
// actually have anything MIDI-learnable today (only Desk's faders) -- this
// button is mounted unconditionally in GlobalFrame (same as SafetyCluster),
// so without this check it would sit there invitingly clickable on every
// other workspace with nothing for it to do. A route not in this set only
// blocks turning learn mode ON from there, not off: if it's already on
// (e.g. the operator turned it on while on Desk, then navigated away
// without exiting first), the toggle stays clickable so there is always a
// way back out from wherever they end up, never a state only reachable by
// returning to Desk first.
import { Radio } from "lucide-react";

import { useGolcStore } from "../../store/store";
import type { DestinationId } from "../../shell/navigation";
import styles from "./MidiLearnToggle.module.css";

const MIDI_LEARN_SUPPORTED_DESTINATIONS: ReadonlySet<DestinationId> = new Set<DestinationId>(["perform-desk"]);

interface MidiLearnToggleProps {
  activeDestination: DestinationId;
}

export default function MidiLearnToggle({ activeDestination }: MidiLearnToggleProps) {
  const midiLearnMode = useGolcStore((state) => state.midiLearnMode);
  const setMidiLearnMode = useGolcStore((state) => state.setMidiLearnMode);

  const supportedHere = MIDI_LEARN_SUPPORTED_DESTINATIONS.has(activeDestination);
  const disabled = !supportedHere && !midiLearnMode;

  return (
    <button
      type="button"
      className={styles.toggle}
      data-active={midiLearnMode || undefined}
      aria-pressed={midiLearnMode}
      aria-disabled={disabled}
      aria-label={midiLearnMode ? "Exit MIDI Learn mode" : "Enter MIDI Learn mode"}
      title={
        disabled
          ? "MIDI Learn isn't available on this view -- switch to Perform > Desk to map faders"
          : midiLearnMode
            ? "MIDI Learn is on -- click a highlighted control, then move a MIDI control to map it. Press Esc to exit."
            : "Turn on MIDI Learn to map Desk faders to MIDI controls"
      }
      onClick={disabled ? undefined : () => setMidiLearnMode(!midiLearnMode)}
    >
      <Radio size={14} className={styles.icon} aria-hidden="true" />
      <span className={styles.label}>MIDI Learn</span>
    </button>
  );
}
