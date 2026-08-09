// Menu is the shared dropdown action-list primitive: a "..." overflow menu
// on a list row, a right-click-style action list, anywhere a trigger needs
// to open a small list of discrete actions. Built on Base UI's Menu (the
// same unstyled-primitive approach Dialog/Tabs already use) -- Base UI owns
// open/close state, focus management, and the roving-tabindex keyboard
// navigation (ArrowUp/ArrowDown/Home/End/typeahead) internally; this
// wrapper only supplies the item contract and design-system styling.
//
// `trigger` is typed ReactNode (per this primitive's contract) but must
// actually be a single ReactElement that forwards its ref and spreads
// unknown props onto its own DOM node -- the same composition requirement
// Base UI's `render` prop always has (see Dialog/Tabs conversion notes).
// IconButton (this repo's icon-only button primitive) already satisfies
// that and is the expected default trigger for an overflow menu.
import { Menu as BaseMenu } from "@base-ui/react/menu";
import type { LucideIcon } from "lucide-react";
import type { ReactElement, ReactNode } from "react";

import styles from "./Menu.module.css";

export interface MenuItem {
  id: string;
  label: string;
  icon?: LucideIcon;
  onSelect: () => void;
  disabled?: boolean;
  /** Visually flags a destructive action (e.g. "Delete"), mirrors Button's
   * destructive variant -- reuses the same --ds-action-destructive token. */
  destructive?: boolean;
}

export interface MenuProps {
  trigger: ReactNode;
  items: readonly MenuItem[];
  /** Accessible name for the menu itself if `trigger` alone doesn't supply
   * one. Base UI labels the popup via the trigger's own accessible name by
   * default (aria-labelledby); this only needs to be set when that default
   * isn't enough (e.g. a trigger whose own label is purely visual). */
  "aria-label"?: string;
}

export default function Menu({ trigger, items, "aria-label": ariaLabel }: MenuProps) {
  return (
    <BaseMenu.Root>
      <BaseMenu.Trigger render={trigger as ReactElement} />
      <BaseMenu.Portal>
        <BaseMenu.Positioner className={styles.positioner} sideOffset={4} align="start">
          <BaseMenu.Popup
            className={styles.popup}
            aria-label={ariaLabel}
            // Base UI labels the popup via aria-labelledby pointing at the
            // trigger by default, and aria-labelledby always wins over
            // aria-label in accessible-name computation when both are
            // present -- so an explicit `aria-label` here would otherwise
            // be silently shadowed by that default. Spreading the key (not
            // just passing `undefined` as its value) is required: Base UI's
            // own prop-merging only skips its internal default when the key
            // is present in props at all, not merely non-undefined.
            {...(ariaLabel ? { "aria-labelledby": undefined } : {})}
          >
            {items.map((item) => {
              const Icon = item.icon;
              return (
                <BaseMenu.Item
                  key={item.id}
                  className={item.destructive ? `${styles.item} ${styles.destructive}` : styles.item}
                  disabled={item.disabled}
                  data-destructive={item.destructive ? "true" : undefined}
                  onClick={item.onSelect}
                >
                  {Icon ? <Icon className={styles.icon} aria-hidden="true" /> : null}
                  {item.label}
                </BaseMenu.Item>
              );
            })}
          </BaseMenu.Popup>
        </BaseMenu.Positioner>
      </BaseMenu.Portal>
    </BaseMenu.Root>
  );
}
