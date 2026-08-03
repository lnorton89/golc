// Small hover/focus "i" affordance placed beside a heading or nav item.
// Always a sibling of the interactive/heading element it annotates, never
// nested inside it -- CommandRail's destination buttons and Toolbar's own
// <h2> are both matched by accessible name in tests and by screen
// readers, and a nested control would fold this tooltip's text into that
// name.
//
// Rendered through a portal into document.body rather than CSS-only
// group-hover: several call sites (CommandRail.module.css's own .rail,
// Toolbar's .action row wrapping) sit inside a container with
// overflow-y: auto (browsers compute the paired overflow-x as auto too),
// which clips any absolutely positioned descendant the instant it
// crosses that container's own bounds. A portal escapes the clipped
// subtree entirely; its position is computed from the trigger's real
// on-screen rect instead.
import { useId, useLayoutEffect, useRef, useState } from "react";
import type { CSSProperties, KeyboardEvent } from "react";
import { createPortal } from "react-dom";
import styles from "./InfoTooltip.module.css";

interface InfoTooltipProps {
  label: string;
  text: string;
}

export default function InfoTooltip({ label, text }: InfoTooltipProps) {
  const [open, setOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const [position, setPosition] = useState<{ top: number; left: number } | null>(null);
  const tooltipId = useId();

  useLayoutEffect(() => {
    if (!open || !triggerRef.current) {
      return;
    }
    const rect = triggerRef.current.getBoundingClientRect();
    setPosition({
      top: rect.top + rect.height / 2,
      left: rect.right,
    });
  }, [open]);

  const handleKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (event.key === "Escape") {
      setOpen(false);
    }
  };

  return (
    <span className={styles.wrapper}>
      <button
        ref={triggerRef}
        type="button"
        aria-label={label}
        aria-describedby={open ? tooltipId : undefined}
        className={styles.trigger}
        onMouseEnter={() => setOpen(true)}
        onMouseLeave={() => setOpen(false)}
        onFocus={() => setOpen(true)}
        onBlur={() => setOpen(false)}
        onKeyDown={handleKeyDown}
        title={text}
      >
        i
      </button>
      {open && position
        ? createPortal(
            <span
              role="tooltip"
              id={tooltipId}
              className={styles.tooltip}
              style={{ "--tooltip-top": `${position.top}px`, "--tooltip-left": `${position.left}px` } as CSSProperties}
            >
              {text}
            </span>,
            document.body,
          )
        : null}
    </span>
  );
}
