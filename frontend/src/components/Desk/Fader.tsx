// Fader.tsx is one vertical DMX channel fader (Perform > Desk, a
// QLC+-style Simple Desk): a native <input type="range"> rendered vertical
// via the standard -webkit-appearance: slider-vertical/writing-mode:
// vertical-lr combination (Desk.module.css) -- there is no slider/fader
// component anywhere else in this codebase to reuse (09-RESEARCH.md
// finding), so this is a small, self-contained primitive rather than a new
// design-system component. value is always the raw 0-255 DMX byte the
// parent already resolved (either the live polled value or a locally
// tracked override) -- Fader itself never normalizes or interprets it.
import { Radio, X } from "lucide-react";
import type { LucideIcon } from "lucide-react";

import styles from "./Desk.module.css";

/** MidiLearnStatus mirrors MidiLearn.tsx's own status machine
 * (idle|listening|conflict|timeout|error) -- "idle" never reaches Fader
 * itself (Desk.tsx only ever passes a status while this channel is the one
 * actively capturing; otherwise midiLearnStatus is left undefined). */
export type MidiLearnStatus = "listening" | "conflict" | "timeout" | "error";

/** MAJOR_SCALE_TICKS are the numbered quarter/half/full reference points a
 * detailed fader's value scale shows next to its track -- a plain
 * evenly-spaced readout of the 0-255 DMX byte range every fader already
 * operates over, not tied to any particular channel's own value. Each gets
 * a visible number plus a long, thick tick line. */
const MAJOR_SCALE_TICKS = [255, 192, 128, 64, 0];

/** MINOR_SCALE_TICKS are the unlabeled in-between gradation marks a real
 * ruler shows alongside its numbered ones -- every 16 DMX values, skipping
 * whichever already have a major tick (0/64/128/192) -- rendered as a
 * shorter, thinner, fainter line with no number, purely a finer visual
 * reference for where a value between two majors actually falls. */
const MINOR_SCALE_TICKS = Array.from({ length: 16 }, (_, i) => i * 16).filter(
  (v) => !MAJOR_SCALE_TICKS.includes(v),
);

/** THUMB_TRAVEL_LENGTH_PX matches .faderInput's own thumb height (10px,
 * Desk.module.css) -- the thumb's fixed physical extent along the track's
 * travel axis. A native range input's thumb CENTER never reaches the
 * track's own 0%/100% box edges: it's inset by half the thumb's own
 * length at each end (the browser clamps thumb position so the thumb
 * itself never renders outside the track). A plain 0%-100% tick placement
 * ignores that inset and reads as visibly misaligned against the actual
 * thumb position, worst at the very top/bottom ticks (255/0) where the
 * gap between "where the tick sits" and "where the thumb actually stops"
 * is largest. */
const THUMB_TRAVEL_LENGTH_PX = 10;

/** scaleTickTop converts a 0-255 DMX byte value into the CSS top-offset
 * its tick sits at within the scale's own track-height column -- 255
 * (max) at the track's top inset, 0 (min) at its bottom inset, mirroring
 * the actual native thumb's own travel range (see THUMB_TRAVEL_LENGTH_PX)
 * instead of the track's raw 0%/100% box edges, so a tick genuinely lines
 * up with where the thumb sits at that value rather than just
 * approximating it. Mixed px+percent via calc(): the inset itself is a
 * fixed px amount regardless of this fader's own current track length,
 * while the remaining travel range scales with it. */
function scaleTickTop(value: number): string {
  const halfThumb = THUMB_TRAVEL_LENGTH_PX / 2;
  const fraction = (255 - value) / 255;
  return `calc(${halfThumb}px + ${fraction} * (100% - ${THUMB_TRAVEL_LENGTH_PX}px))`;
}

interface FaderProps {
  label: string;
  /** detailLabel is the un-abbreviated form of label (capabilityDetailLabel
   * in Desk.tsx) -- shown next to a swatch/icon only when `detailed` is
   * true, since only then is there room for it. */
  detailLabel: string;
  /** detailed, when true, means this fader's column is currently wide
   * enough (Desk.tsx's own DETAILED_MIN_FADER_WIDTH threshold against the
   * row's live --fader-width) to show its extra content: detailLabel next
   * to the swatch/icon, and a 0/64/128/192/255 value scale beside the
   * track. False keeps the original compact swatch/icon-only, scale-less
   * rendering unchanged. */
  detailed: boolean;
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
  /** touched, when true, means this channel is currently grabbed/overridden
   * (Desk.tsx's own touchedKeys -- set the moment a drag changes it,
   * cleared the moment its override is released, same lifecycle as
   * `overridden`). False renders the thumb dimmed grey instead of the
   * ordinary accent blue -- releasing an override resets a channel back
   * to this same dimmed state, not to a lingering "has been touched"
   * blue, so grey always means "not currently doing anything" at a
   * glance. */
  touched: boolean;
  onChange: (value: number) => void;
  onClear: () => void;
  /** midiMapped, when true, shows the always-visible "this channel is
   * MIDI-mapped" corner badge (outside learn mode too) -- Desk.tsx derives
   * this from whether ListDeskMappings has an entry for this channel's own
   * key. */
  midiMapped?: boolean;
  /** midiLearnMode mirrors store.ts's global toggle (MidiLearnToggle.tsx)
   * -- while true, this Fader renders the learn/remap/clear overlay
   * instead of letting a click reach the native range input. */
  midiLearnMode?: boolean;
  /** midiLearnStatus is only ever set while THIS channel is the one
   * Desk.tsx is currently capturing (its own single-capture-at-a-time
   * state, mirroring the backend's own s.learning mutual exclusion) --
   * undefined for every other fader even while midiLearnMode is true. */
  midiLearnStatus?: MidiLearnStatus;
  midiLearnMessage?: string | null;
  onMidiLearnClick?: () => void;
  onMidiRemap?: () => void;
  onMidiClear?: () => void;
  onMidiCancel?: () => void;
}

export default function Fader({
  label,
  detailLabel,
  detailed,
  swatch,
  icon: Icon,
  sublabel,
  value,
  overridden,
  touched,
  onChange,
  onClear,
  midiMapped,
  midiLearnMode,
  midiLearnStatus,
  midiLearnMessage,
  onMidiLearnClick,
  onMidiRemap,
  onMidiClear,
  onMidiCancel,
}: FaderProps) {
  const inputClassName = overridden
    ? styles.faderInputOverridden
    : touched
      ? styles.faderInput
      : `${styles.faderInput} ${styles.faderInputUntouched}`;
  // detailShiftClass nudges just the value badge + track (never the
  // label/sublabel/clear-button below) right when detailed: the scale
  // ruler hangs off the track's own LEFT edge (position:absolute,
  // .faderScale), which visually pulls this whole top group left of
  // where the label content below -- which has no such asymmetric
  // element -- reads as centered. Shifting only these two elements
  // re-centers the group's own visual footprint (badge+track+scale)
  // without disturbing the vertical value/track/label/sublabel alignment
  // .faderTrackRow's own centering already guarantees.
  const detailShiftClass = detailed ? styles.faderDetailShift : "";
  return (
    <div className={styles.fader}>
      <span className={`${styles.faderValue} ${detailShiftClass}`}>{value}</span>
      <div className={`${styles.faderTrackRow} ${detailShiftClass}`}>
        {detailed && (
          <div className={styles.faderScale} aria-hidden="true">
            {MINOR_SCALE_TICKS.map((tick) => (
              <span
                key={tick}
                className={styles.faderScaleMinorTick}
                style={{ top: scaleTickTop(tick) }}
              />
            ))}
            {MAJOR_SCALE_TICKS.map((tick) => (
              <span key={tick} className={styles.faderScaleMajorTick} style={{ top: scaleTickTop(tick) }}>
                <span className={styles.faderScaleTickNumber}>{tick}</span>
                <span className={styles.faderScaleTickLine} />
              </span>
            ))}
          </div>
        )}
        <input
          className={inputClassName}
          type="range"
          min={0}
          max={255}
          step={1}
          value={value}
          onChange={(event) => onChange(Number(event.target.value))}
          aria-label={`${label} (${sublabel})`}
          title={overridden ? `${label} -- manually overridden` : label}
        />
      </div>
      <span className={styles.faderLabelSlot} title={label}>
        {swatch ? (
          <span className={styles.faderLabelSwatch} style={{ background: swatch }} aria-hidden="true" />
        ) : Icon ? (
          <Icon size={14} aria-hidden="true" />
        ) : (
          <span className={styles.faderLabelText}>{label}</span>
        )}
        {(swatch || Icon) && detailed && <span className={styles.faderLabelDetail}>{detailLabel}</span>}
      </span>
      <span className={styles.faderSublabel} title={sublabel}>
        {sublabel}
      </span>
      <button
        type="button"
        className={styles.faderClearButton}
        onClick={overridden ? onClear : undefined}
        aria-disabled={!overridden}
        title={overridden ? "Release override back to programmed output" : "Not overridden"}
      >
        <X size={11} aria-hidden="true" />
      </button>
      {midiLearnMode ? (
        <div className={styles.faderLearnOverlay}>
          {midiLearnStatus === "listening" ? (
            <>
              <span className={styles.faderLearnListening} role="status" aria-live="polite">
                Listening…
              </span>
              <button type="button" className={styles.faderLearnActionButton} onClick={onMidiCancel}>
                Cancel
              </button>
            </>
          ) : midiMapped ? (
            <div className={styles.faderLearnActions}>
              <button
                type="button"
                className={styles.faderLearnActionButton}
                onClick={onMidiRemap}
                aria-label={`Remap MIDI control for ${label}`}
              >
                Remap
              </button>
              <button
                type="button"
                className={`${styles.faderLearnActionButton} ${styles.faderLearnClearButton}`}
                onClick={onMidiClear}
                aria-label={`Clear MIDI mapping for ${label}`}
              >
                Clear
              </button>
            </div>
          ) : (
            <button
              type="button"
              className={styles.faderLearnButton}
              onClick={onMidiLearnClick}
              aria-label={`Learn MIDI mapping for ${label}`}
            >
              Learn
            </button>
          )}
          {midiLearnMessage && (midiLearnStatus === "conflict" || midiLearnStatus === "timeout" || midiLearnStatus === "error") && (
            <p className={styles.faderLearnMessage} role="alert">
              {midiLearnMessage}
            </p>
          )}
        </div>
      ) : (
        midiMapped && (
          <span className={styles.faderLearnBadge} title={`${label} is MIDI-mapped`} aria-hidden="true">
            <Radio size={9} />
          </span>
        )
      )}
    </div>
  );
}
