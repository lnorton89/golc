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
// other workspace with nothing for it to do. Navigating away from a
// supported destination while learn mode is on turns it back off
// automatically (the effect below) -- there is no "still on, but nothing
// to map" state to strand an operator in, so `disabled` here only ever
// needs to gate turning it ON.
import { useEffect } from "react";
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

  // Leaving Desk (or any future MIDI-learnable view) while learn mode is
  // on turns it off -- WorkspaceRouter unmounts Desk.tsx itself on
  // navigate-away regardless (its own doc comment: "a plain switch, not
  // keep-mounted-hidden"), so its highlighted faders are already gone the
  // instant this fires; this just keeps the global toggle's own on/off
  // state truthful to that, rather than leaving it silently "on" for a
  // view with nothing left to highlight.
  useEffect(() => {
    if (!supportedHere && midiLearnMode) {
      setMidiLearnMode(false);
    }
  }, [supportedHere, midiLearnMode, setMidiLearnMode]);

  const disabled = !supportedHere;

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
