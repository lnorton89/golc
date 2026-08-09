// Select is the dropdown-choice primitive: a thin wrapper around Base UI's
// unstyled Select (see Dialog.tsx/Tabs.tsx for the same wrapping convention
// this repo already established). Composition follows Base UI's documented
// canonical nesting -- Root > (Label +) Trigger(Value, Icon), Portal >
// Positioner > Popup > List > Item(ItemText, ItemIndicator) -- there is no
// Select.Content node despite that being a common pattern in other
// compound-component libraries.
import { Select as BaseSelect } from "@base-ui/react/select";
import { Check, ChevronsUpDown } from "lucide-react";

import styles from "./Select.module.css";

export interface SelectOption {
  value: string;
  label: string;
  disabled?: boolean;
}

export interface SelectProps {
  label: string;
  options: readonly SelectOption[];
  value?: string;
  defaultValue?: string;
  onValueChange?: (value: string) => void;
  placeholder?: string;
  disabled?: boolean;
  /** hideLabel drops the visible <Select.Label> line, mirroring Field.tsx's
   * hideLabel -- the trigger's own aria-label keeps the accessible name
   * regardless, for a caller embedding this inline in a single-row control
   * cluster rather than this primitive's default label-above-control
   * block layout. */
  hideLabel?: boolean;
}

export default function Select({
  label,
  options,
  value,
  defaultValue,
  onValueChange,
  placeholder,
  disabled = false,
  hideLabel = false,
}: SelectProps) {
  const items = options.map((option) => ({ value: option.value, label: option.label }));

  return (
    <BaseSelect.Root
      items={items}
      value={value}
      defaultValue={defaultValue}
      disabled={disabled}
      onValueChange={(next) => {
        if (typeof next === "string") onValueChange?.(next);
      }}
    >
      <div className={styles.field}>
        {hideLabel ? null : <BaseSelect.Label className={styles.label}>{label}</BaseSelect.Label>}
        <BaseSelect.Trigger className={styles.trigger} disabled={disabled} aria-label={hideLabel ? label : undefined}>
          <BaseSelect.Value className={styles.value} placeholder={placeholder} />
          <BaseSelect.Icon className={styles.triggerIcon}>
            <ChevronsUpDown aria-hidden="true" className={styles.chevron} />
          </BaseSelect.Icon>
        </BaseSelect.Trigger>
      </div>
      <BaseSelect.Portal>
        <BaseSelect.Positioner className={styles.positioner} sideOffset={4} align="start">
          <BaseSelect.Popup className={styles.popup}>
            <BaseSelect.List className={styles.list}>
              {options.map((option) => (
                <BaseSelect.Item key={option.value} className={styles.item} value={option.value} disabled={option.disabled}>
                  <BaseSelect.ItemText className={styles.itemText}>{option.label}</BaseSelect.ItemText>
                  <BaseSelect.ItemIndicator className={styles.itemIndicator}>
                    <Check aria-hidden="true" className={styles.checkIcon} />
                  </BaseSelect.ItemIndicator>
                </BaseSelect.Item>
              ))}
            </BaseSelect.List>
          </BaseSelect.Popup>
        </BaseSelect.Positioner>
      </BaseSelect.Portal>
    </BaseSelect.Root>
  );
}
