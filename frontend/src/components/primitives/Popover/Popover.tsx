// Generic anchored floating-content primitive: click-triggered, and free to
// hold arbitrary rich/interactive content (an inline settings panel
// anchored to a button, say). InfoTooltip is this app's OTHER
// floating-content mechanism, but it is hover/focus-triggered and
// text-only -- a distinct pattern from this one, not a subset of it -- so
// this primitive is a separate component rather than an InfoTooltip option.
//
// A lighter-weight sibling of Dialog, not a second modal: it never blocks
// or traps focus in the rest of the page the way Dialog's backdrop scrim
// does. Base UI's Popover.Root defaults its own `modal` prop to `false`,
// which is exactly this non-blocking behavior, so it's left unset here
// rather than passed explicitly. Dismissible the same two ways Dialog is
// (Escape, an outside click) but always so -- unlike Dialog, this contract
// has no closeOnEscape/closeOnBackdrop opt-outs, since a non-modal surface
// that resists its own dismissal would be a confusing, half-modal hybrid.
import { Popover as BasePopover } from "@base-ui/react/popover";
import { isValidElement, useState, type ReactNode } from "react";

import styles from "./Popover.module.css";

export interface PopoverProps {
  /** The element that opens the popover. Base UI's `render` composition
   * merges the open/close behavior directly onto this element (the same
   * "compose, don't wrap" mechanism Dialog would reach for if it had a
   * built-in trigger) instead of nesting it inside a second <button> --
   * passing an existing <button> here doesn't produce nested interactive
   * elements. Falls back to rendering as children of Base UI's own default
   * <button> when the value isn't a single element (e.g. plain text). */
  trigger: ReactNode;
  children: ReactNode;
  /** Accessible name for the popover content, for when the content itself
   * doesn't already supply one (no heading inside, say). */
  "aria-label"?: string;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  side?: "top" | "right" | "bottom" | "left";
}

export default function Popover({
  trigger,
  children,
  "aria-label": ariaLabel,
  open,
  onOpenChange,
  side = "bottom",
}: PopoverProps) {
  // Controlled/uncontrolled fallback: `open` present at all (even
  // `open={false}`) means the caller owns the state; absent means this
  // component tracks it itself, the same shape most of this app's own
  // controllable primitives use.
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const isControlled = open !== undefined;
  const openState = isControlled ? open : uncontrolledOpen;

  const handleOpenChange = (next: boolean) => {
    if (!isControlled) setUncontrolledOpen(next);
    onOpenChange?.(next);
  };

  return (
    <BasePopover.Root open={openState} onOpenChange={handleOpenChange}>
      {isValidElement(trigger) ? (
        <BasePopover.Trigger render={trigger} />
      ) : (
        <BasePopover.Trigger>{trigger}</BasePopover.Trigger>
      )}
      <BasePopover.Portal>
        <BasePopover.Positioner className={styles.positioner} side={side} sideOffset={8}>
          <BasePopover.Popup className={styles.popup} aria-label={ariaLabel}>
            {children}
          </BasePopover.Popup>
        </BasePopover.Positioner>
      </BasePopover.Portal>
    </BasePopover.Root>
  );
}
