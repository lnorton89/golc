import type { CSSProperties } from "react";
import { RgbColorPicker } from "react-colorful";

import NumberStepper from "../NumberStepper/NumberStepper";
import Popover from "../Popover/Popover";
import styles from "./ColorField.module.css";

export interface RgbColor {
  /** 0-255 */
  r: number;
  /** 0-255 */
  g: number;
  /** 0-255 */
  b: number;
}

export interface ColorFieldProps {
  /** Accessible name -- also prefixes the popover's own aria-label and each
   * numeric channel field's aria-label, so a page with multiple ColorFields
   * open at different times still exposes distinguishable names. */
  label: string;
  /** Always controlled -- this app's color state always comes from/goes to
   * a live channel value (three DMX bytes), never a locally-owned draft, so
   * there is no meaningful uncontrolled mode to fall back to. */
  value: RgbColor;
  onValueChange: (value: RgbColor) => void;
  disabled?: boolean;
  /** Mirrors Field.tsx's/NumberStepper's own hideLabel -- for a caller
   * embedding this inline in a compact control cluster rather than this
   * primitive's default stacked label-above-swatch layout. The swatch
   * button's own aria-label carries the accessible name regardless. */
  hideLabel?: boolean;
}

/** clampChannel is this primitive's one and only normalization step for a
 * raw channel number -- typed text, a NumberStepper nudge, or a
 * react-colorful drag update all funnel through here before ever reaching
 * onValueChange, rather than each of the three input paths re-implementing
 * its own bounds check (this app's "never a second validation/normalize
 * implementation" convention -- see FixturePatch's/FixtureLibraryWorkspace's
 * own doc comments for the same principle applied to fixture data). A
 * fractional drag coordinate or an out-of-range typed value can never reach
 * a caller unclamped; NaN (an empty/partial typed field) resolves to 0
 * rather than propagating as NaN into a live DMX channel. */
function clampChannel(raw: number): number {
  if (!Number.isFinite(raw)) return 0;
  return Math.min(255, Math.max(0, Math.round(raw)));
}

function clampColor(color: RgbColor): RgbColor {
  return { r: clampChannel(color.r), g: clampChannel(color.g), b: clampChannel(color.b) };
}

const CHANNELS: ReadonlyArray<{ key: keyof RgbColor; caption: string; name: string }> = [
  { key: "r", caption: "R", name: "Red" },
  { key: "g", caption: "G", name: "Green" },
  { key: "b", caption: "B", name: "Blue" },
];

/**
 * ColorField is a quick-set visual convenience laid over precise
 * per-channel numeric entry -- the professional-console convention
 * (grandMA/Hog/Chamsys-style color pickers sit alongside exact channel
 * faders as a shortcut, never replacing them). It never owns a fixture's
 * actual RGBW channel state itself: a caller wires `value`/`onValueChange`
 * to whichever three channel values it already tracks (e.g. Desk's
 * color_red/color_green/color_blue fader values), so a rough pick on the
 * visual picker and a precise edit of a single channel afterward both land
 * in the exact same place.
 *
 * The swatch's own fill color is genuinely dynamic per-instance data (the
 * live RGB value being edited), not a design-system token, so it is set via
 * an inline `style` prop rather than CSS -- the same "operator-controlled
 * dynamic color as inline style, everything else through tokens" split
 * Desk.tsx's own fixtureCardInlineStyle already draws for its fixture-card
 * customization colors.
 */
export default function ColorField({ label, value, onValueChange, disabled = false, hideLabel = false }: ColorFieldProps) {
  const swatchStyle: CSSProperties = { backgroundColor: `rgb(${value.r}, ${value.g}, ${value.b})` };

  const handlePickerChange = (next: RgbColor) => {
    onValueChange(clampColor(next));
  };

  const handleChannelChange = (channel: keyof RgbColor) => (raw: string) => {
    onValueChange(clampColor({ ...value, [channel]: Number(raw) }));
  };

  return (
    <div className={styles.field}>
      {!hideLabel && <label className={styles.label}>{label}</label>}
      <Popover
        aria-label={`${label} color picker`}
        trigger={
          <button
            type="button"
            className={styles.swatch}
            style={swatchStyle}
            disabled={disabled}
            aria-label={hideLabel ? label : `${label} swatch`}
          />
        }
      >
        <div className={styles.content}>
          <div className={styles.pickerWrapper}>
            <RgbColorPicker color={value} onChange={handlePickerChange} style={{ width: "100%" }} />
          </div>
          <div className={styles.channels}>
            {CHANNELS.map(({ key, caption, name }) => (
              <div className={styles.channel} key={key}>
                <span className={styles.channelLabel} aria-hidden="true">
                  {caption}
                </span>
                <NumberStepper
                  label={`${label} ${name.toLowerCase()} channel`}
                  hideLabel
                  min={0}
                  value={String(value[key])}
                  onChange={handleChannelChange(key)}
                  disabled={disabled}
                />
              </div>
            ))}
          </div>
        </div>
      </Popover>
    </div>
  );
}
