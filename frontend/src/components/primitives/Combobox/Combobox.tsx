import { Combobox as BaseCombobox } from "@base-ui/react/combobox";
import { useId } from "react";
import { ChevronDown } from "lucide-react";

import { fuzzyMatches } from "../../../lib/fuzzySearch";
import styles from "./Combobox.module.css";

export interface ComboboxOption {
  value: string;
  label: string;
  disabled?: boolean;
}

export interface ComboboxProps {
  label: string;
  options: readonly ComboboxOption[];
  value?: string;
  defaultValue?: string;
  onValueChange?: (value: string) => void;
  placeholder?: string;
  disabled?: boolean;
  hideLabel?: boolean;
  emptyMessage?: string;
}

export default function Combobox({
  label,
  options,
  value,
  defaultValue,
  onValueChange,
  placeholder,
  disabled = false,
  hideLabel = false,
  emptyMessage = "No matches",
}: ComboboxProps) {
  const generatedId = useId();
  const controlId = `combobox-${generatedId}`;

  // Base UI's Combobox.Root selects/compares whole item objects, not the
  // bare id strings this primitive's own contract deals in -- resolving
  // `value`/`defaultValue` to the matching ComboboxOption here (looked up
  // from the very same `options` array Root's `items` prop and the List's
  // render-prop items come from) keeps that resolved option referentially
  // identical to what Root sees elsewhere, so no isItemEqualToValue
  // override is needed for selection/highlight to match up correctly.
  // Only one of value/defaultValue is ever forwarded to Root, mirroring a
  // native <input>'s own controlled-vs-uncontrolled split -- passing both
  // at once would leave Root's controlled/uncontrolled mode ambiguous.
  const controlledValueProps =
    value !== undefined
      ? { value: options.find((option) => option.value === value) ?? null }
      : { defaultValue: options.find((option) => option.value === defaultValue) ?? null };

  return (
    <div className={styles.field}>
      {!hideLabel && (
        <label htmlFor={controlId} className={styles.label}>
          {label}
        </label>
      )}
      <BaseCombobox.Root
        items={options}
        itemToStringLabel={(option: ComboboxOption | null) => option?.label ?? ""}
        // Deliberately narrower than Base UI's default match (which
        // stringifies the whole item): matched against `label` only, so a
        // fixture's internal id/value never becomes an accidental filter
        // hit.
        //
        // fuzzyMatches rather than the former .includes(): a transposed
        // character no longer empties the list. Ranking is NOT applied
        // here and cannot be through this API -- Base UI's `filter` is a
        // per-option yes/no predicate, so options keep their given order.
        // A caller that needs best-match-first should rank its own
        // `options` array with fuzzySearch before passing it in.
        filter={(option: ComboboxOption, query: string) => fuzzyMatches(option.label, query)}
        onValueChange={(nextOption: ComboboxOption | null) => onValueChange?.(nextOption?.value ?? "")}
        {...controlledValueProps}
      >
        <div className={styles.control} data-disabled={disabled ? "" : undefined}>
          <BaseCombobox.Input
            id={controlId}
            className={styles.input}
            placeholder={placeholder}
            disabled={disabled}
            aria-label={hideLabel ? label : undefined}
          />
          <BaseCombobox.Trigger className={styles.trigger} disabled={disabled} aria-label={`Show ${label} options`}>
            <ChevronDown className={styles.icon} aria-hidden="true" />
          </BaseCombobox.Trigger>
        </div>
        <BaseCombobox.Portal>
          <BaseCombobox.Positioner className={styles.positioner} sideOffset={4}>
            <BaseCombobox.Popup className={styles.popup}>
              <BaseCombobox.Empty className={styles.empty}>{emptyMessage}</BaseCombobox.Empty>
              <BaseCombobox.List className={styles.list}>
                {(option: ComboboxOption) => (
                  <BaseCombobox.Item key={option.value} value={option} disabled={option.disabled} className={styles.item}>
                    {option.label}
                  </BaseCombobox.Item>
                )}
              </BaseCombobox.List>
            </BaseCombobox.Popup>
          </BaseCombobox.Positioner>
        </BaseCombobox.Portal>
      </BaseCombobox.Root>
    </div>
  );
}
