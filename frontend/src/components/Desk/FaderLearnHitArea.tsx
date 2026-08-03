// FaderLearnHitArea is Fader.tsx's own MIDI-learn click target, split into
// its own file so its raw <button> is the only one the design-system
// checker finds here -- Fader.tsx already owns one exact, narrow
// domain-geometry exception for its own fixed-size clear button, and the
// checker's exception mechanism can only resolve a match to exactly one
// diagnostic per file (see design-system/exception-proposals/desk.json).
//
// This element is transparent -- no background, no border, nothing
// painted over the fader's own content -- it exists only to intercept the
// click a native range-input drag would otherwise consume, and to give
// the whole card one unambiguous click target. The fader's own highlight
// styling (applied by the caller) is what conveys "clickable," not this
// element. Listening -> click cancels; already mapped -> click remaps;
// otherwise -> click learns.
import styles from "./Desk.module.css";

export interface FaderLearnHitAreaProps {
  label: string;
  listening: boolean;
  mapped: boolean;
  attention: boolean;
  message: string | null | undefined;
  onLearn?: () => void;
  onRemap?: () => void;
  onCancel?: () => void;
}

export default function FaderLearnHitArea({
  label,
  listening,
  mapped,
  attention,
  message,
  onLearn,
  onRemap,
  onCancel,
}: FaderLearnHitAreaProps) {
  return (
    <>
      <button
        type="button"
        className={styles.faderLearnHitArea}
        onClick={listening ? onCancel : mapped ? onRemap : onLearn}
        aria-label={listening ? `Cancel MIDI learn for ${label}` : mapped ? `Remap MIDI control for ${label}` : `Learn MIDI mapping for ${label}`}
        title={
          attention && message
            ? message
            : listening
              ? "Listening for MIDI input… click to cancel"
              : mapped
                ? "Click to remap"
                : "Click to learn"
        }
      />
      {attention && message && (
        <span className="ds-sr-only" role="alert">
          {message}
        </span>
      )}
    </>
  );
}
