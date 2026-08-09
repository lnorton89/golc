// CommandRailGroupToggle is the collapsible nav-group disclosure control --
// a thin label wrapper around Base UI's Collapsible.Trigger (must render
// inside a parent Collapsible.Root, see CommandRail.tsx), which already
// supplies the aria-expanded/aria-controls disclosure contract this file
// used to hand-roll. Still its own file (Desk's FaderLearnHitArea.tsx
// precedent, see 13-13-SUMMARY.md) so CommandRail.tsx's own raw <button>
// -- the per-destination nav item, which needs aria-current landmark
// semantics no shared primitive provides -- stays the only literal
// `<button>` JSX tag in that file for DS005 purposes.
//
// One ChevronRight icon rotated 90deg via the `data-panel-open` state
// attribute Base UI sets on the trigger while open, rather than swapping
// between two lucide icons -- the idiomatic Base UI pattern (see the
// project's Collapsible docs Tailwind demo: `group-data-panel-open:rotate-90`)
// and one fewer piece of open/closed branching to keep in sync by hand.
import { Collapsible } from "@base-ui/react/collapsible";
import { ChevronRight } from "lucide-react";
import styles from "./CommandRail.module.css";

interface CommandRailGroupToggleProps {
  label: string;
}

export default function CommandRailGroupToggle({ label }: CommandRailGroupToggleProps) {
  return (
    <Collapsible.Trigger className={styles.groupToggle}>
      <ChevronRight size={12} className={styles.groupChevron} aria-hidden="true" />
      <span className={styles.groupLabel}>{label}</span>
    </Collapsible.Trigger>
  );
}
