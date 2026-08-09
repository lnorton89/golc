import { Slider as BaseSlider } from "@base-ui/react/slider";

import styles from "./Slider.module.css";

export interface SliderProps {
  label: string;
  value?: number;
  defaultValue?: number;
  onValueChange?: (value: number) => void;
  min?: number;
  max?: number;
  step?: number;
  disabled?: boolean;
  /** hideLabel drops the visible <Slider.Label> line -- mirrors Field.tsx's
   * hideLabel prop for a caller embedding this control inline in a compact
   * row rather than this primitive's default label-above-track layout. The
   * thumb's own aria-label (set below) keeps the accessible name regardless. */
  hideLabel?: boolean;
}

/** Slider is a continuous-drag numeric input built on Base UI's unstyled
 * Slider -- the drag-native counterpart to NumberStepper's click/type
 * affordance, for values (fixture intensity, DMX-channel-style 0-255 or
 * 0-100% ranges) where dragging along a track feels more natural than
 * nudging a discrete stepper. */
export default function Slider({
  label,
  value,
  defaultValue,
  onValueChange,
  min = 0,
  max = 100,
  step = 1,
  disabled = false,
  hideLabel = false,
}: SliderProps) {
  const isControlled = value !== undefined;

  return (
    <BaseSlider.Root
      className={styles.root}
      value={isControlled ? value : undefined}
      defaultValue={isControlled ? undefined : (defaultValue ?? min)}
      min={min}
      max={max}
      step={step}
      disabled={disabled}
      onValueChange={(next) => onValueChange?.(next as number)}
    >
      {!hideLabel && <BaseSlider.Label className={styles.label}>{label}</BaseSlider.Label>}
      <BaseSlider.Control className={styles.control}>
        <BaseSlider.Track className={styles.track}>
          <BaseSlider.Indicator className={styles.indicator} />
          <BaseSlider.Thumb className={styles.thumb} aria-label={hideLabel ? label : undefined} />
        </BaseSlider.Track>
      </BaseSlider.Control>
    </BaseSlider.Root>
  );
}
