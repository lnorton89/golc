import { useId, useState } from "react";
import type { LucideIcon } from "lucide-react";
import { Toggle } from "@base-ui/react/toggle";
import { ToggleGroup as BaseToggleGroup } from "@base-ui/react/toggle-group";

import Button from "../Button/Button";
import styles from "./ToggleGroup.module.css";

export interface ToggleGroupOption {
  value: string;
  label: string;
  /** Optional leading glyph, passed straight through to the composed
   * Button's own leadingIcon -- same lucide-react component-reference
   * convention Menu's MenuItem already uses (`icon={Sun}`, not
   * `icon={<Sun />}`), for a call site like Settings' Mode row (light/
   * dark/system) whose existing Button-based markup carries one per
   * option. */
  icon?: LucideIcon;
  disabled?: boolean;
}

export interface ToggleGroupProps {
  label?: string;
  options: ReadonlyArray<ToggleGroupOption>;
  value?: string;
  defaultValue?: string;
  onValueChange?: (value: string) => void;
  disabled?: boolean;
  "aria-label"?: string;
}

/** ToggleGroup is a labeled set of mutually-exclusive options rendered as a
 * row of buttons, built on Base UI's unstyled ToggleGroup + Toggle -- the
 * button-styled sibling to RadioGroup's radio-dot rendering, for a "My
 * Library" / "Open Fixture Library"-shaped choice where the surrounding
 * screen's existing visual language is Button's primary/secondary variants,
 * not radio dots. Each Toggle composes the shared Button primitive
 * directly via Base UI's `render` prop (confirmed empirically: Base UI
 * merges its own onClick/aria-pressed/data-pressed/tabindex/ref onto the
 * rendered element, so Button keeps owning 100% of the visual chrome --
 * same reasoning as this session's other primitives reusing Button rather
 * than reinventing its chrome). Base UI owns press semantics and
 * roving-tabindex arrow-key navigation between options; this primitive
 * only decides which variant each Button-in-disguise renders with.
 *
 * `label` is optional (unlike RadioGroup's, which is required) because a
 * real call site -- FixtureLibraryWorkspace's source toggle -- has no
 * visible heading of its own, only an aria-label on the button row itself;
 * when `label` is supplied (e.g. Settings' "Mode"/"Theme" rows, which
 * already render their own visible heading beside the button row) it wires
 * up the same aria-labelledby pattern RadioGroup uses.
 *
 * Base UI surprise (confirmed empirically against the rendered DOM, not
 * documented plainly): even with `multiple` left at its default `false`,
 * ToggleGroup lets an operator click the only pressed option and deselect
 * it to an empty array -- a plain radio input can't do this, but Base UI
 * models ToggleGroup's value as `string[]` regardless of single/multiple
 * mode, and single-select mode only enforces "at most one," not "exactly
 * one." This primitive's own contract (like RadioGroup's) promises exactly
 * one option selected at all times, so onValueChange below ignores an
 * empty result -- the clicked option's own Button snaps back to pressed on
 * the next render instead of leaving the group with nothing selected. */
export default function ToggleGroup({
  label,
  options,
  value,
  defaultValue,
  onValueChange,
  disabled = false,
  "aria-label": ariaLabel,
}: ToggleGroupProps) {
  const generatedId = useId();
  const labelId = `toggle-group-${generatedId}-label`;
  const isControlled = value !== undefined;
  const [internalValue, setInternalValue] = useState(defaultValue);
  const currentValue = isControlled ? value : internalValue;

  const handleValueChange = (next: string[]) => {
    if (next.length === 0) return; // deselect-to-empty guard -- see doc comment above
    const nextValue = next[0];
    if (!isControlled) setInternalValue(nextValue);
    onValueChange?.(nextValue);
  };

  const group = (
    <BaseToggleGroup
      className={styles.options}
      value={currentValue !== undefined ? [currentValue] : []}
      onValueChange={handleValueChange}
      disabled={disabled}
      aria-label={ariaLabel}
      aria-labelledby={!ariaLabel && label ? labelId : undefined}
    >
      {options.map((option) => (
        <Toggle
          key={option.value}
          value={option.value}
          disabled={disabled || option.disabled}
          render={<Button variant={currentValue === option.value ? "primary" : "secondary"} leadingIcon={option.icon} />}
        >
          {option.label}
        </Toggle>
      ))}
    </BaseToggleGroup>
  );

  if (!label) return group;

  return (
    <div className={styles.root}>
      <span id={labelId} className={styles.label}>
        {label}
      </span>
      {group}
    </div>
  );
}
