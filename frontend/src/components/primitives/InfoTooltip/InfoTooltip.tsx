// Small hover/focus "i" affordance placed beside a heading or nav item.
// Always a sibling of the interactive/heading element it annotates, never
// nested inside it -- CommandRail's destination buttons and Toolbar's own
// <h2> are both matched by accessible name in tests and by screen
// readers, and a nested control would fold this tooltip's text into that
// name.
//
// The actual open/close state, positioning, viewport-edge flip, and
// portal rendering all live in useTooltip.tsx -- shared with every other
// element in the app that wants this same styled tooltip attached
// directly to itself, rather than the browser's own unstyled native
// `title` tooltip (CommandRail's own nav destination buttons, for one).
// This component is just the dedicated "i" disclosure icon flavor of
// that shared mechanism.
import styles from "./InfoTooltip.module.css";
import { useTooltip } from "./useTooltip";

interface InfoTooltipProps {
  label: string;
  text: string;
}

export default function InfoTooltip({ label, text }: InfoTooltipProps) {
  const { triggerRef, triggerProps, tooltipNode } = useTooltip<HTMLButtonElement>(text);

  return (
    <span className={styles.wrapper}>
      <button ref={triggerRef} type="button" aria-label={label} className={styles.trigger} {...triggerProps}>
        i
      </button>
      {tooltipNode}
    </span>
  );
}
