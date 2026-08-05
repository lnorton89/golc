// NavDestinationButton is one CommandRail nav-rail item. Extracted to its
// own component (rather than inline in CommandRail's own .map()) because
// useTooltip is a hook -- hooks can't be called inside a loop/callback,
// only at a component's own top level, and every destination button now
// shows its own destination.howItWorks text on hover/focus via the same
// shared, styled tooltip mechanism InfoTooltip and CommandRailGroupToggle's
// sibling tooltip both use (rather than no hover text at all, which is
// what every nav item previously had). `suppressible: true` -- unlike an
// InfoTooltip "i" icon (which exists solely to be hovered for more detail),
// this text pops up during ordinary navigation an operator brushes past
// constantly; NavTooltipsToggle in the header lets that be turned off
// without touching InfoTooltip's own icon-triggered tooltips.
import type { LucideIcon } from "lucide-react";

import { useTooltip } from "../components/primitives/InfoTooltip/useTooltip";
import type { NavDestination } from "./navigation";
import styles from "./CommandRail.module.css";

interface NavDestinationButtonProps {
  destination: NavDestination;
  icon: LucideIcon;
  isActive: boolean;
  onSelect: () => void;
}

export default function NavDestinationButton({ destination, icon: Icon, isActive, onSelect }: NavDestinationButtonProps) {
  const { triggerRef, triggerProps, tooltipNode } = useTooltip<HTMLButtonElement>(destination.howItWorks, { suppressible: true });

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        className={isActive ? `${styles.item} ${styles.itemActive}` : styles.item}
        aria-current={isActive ? "page" : undefined}
        onClick={onSelect}
        {...triggerProps}
      >
        <Icon size={15} className={styles.itemIcon} aria-hidden="true" />
        {destination.label}
      </button>
      {tooltipNode}
    </>
  );
}
