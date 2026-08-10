// NavDestinationButton is one CommandRail nav-rail item. Every destination
// button shows its own destination.howItWorks text on hover/focus via the
// same shared, styled tooltip mechanism InfoTooltip and
// CommandRailGroupToggle's sibling tooltip both use (rather than no hover
// text at all, which is what every nav item previously had).
// `suppressible` -- unlike an InfoTooltip "i" icon (which exists solely to
// be hovered for more detail), this text pops up during ordinary
// navigation an operator brushes past constantly; NavTooltipsToggle in the
// header lets that be turned off without touching InfoTooltip's own
// icon-triggered tooltips.
//
// This stayed its own component after the Base UI migration even though
// HoverTooltip is now a component rather than a hook (so the original
// "hooks can't be called inside a .map() callback" reason no longer
// applies): CommandRail's own .map() stays a flat list of destinations
// instead of also carrying this button's tooltip/active/aria-current
// composition inline.
import type { LucideIcon } from "lucide-react";

import HoverTooltip from "../components/primitives/InfoTooltip/HoverTooltip";
import type { NavDestination } from "./navigation";
import styles from "./CommandRail.module.css";

interface NavDestinationButtonProps {
  destination: NavDestination;
  icon: LucideIcon;
  isActive: boolean;
  onSelect: () => void;
}

export default function NavDestinationButton({ destination, icon: Icon, isActive, onSelect }: NavDestinationButtonProps) {
  return (
    <HoverTooltip text={destination.howItWorks} suppressible>
      <button
        type="button"
        className={isActive ? `${styles.item} ${styles.itemActive}` : styles.item}
        aria-current={isActive ? "page" : undefined}
        onClick={onSelect}
      >
        <Icon size={15} className={styles.itemIcon} aria-hidden="true" />
        {destination.label}
      </button>
    </HoverTooltip>
  );
}
