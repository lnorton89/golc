// CommandRail is the left navigation from application-shell-navigation.md's
// Focused Command Rail (Sketch 001 Variant D) -- grouped by stable user
// intent (Show/Build/Operate/Output), never by backend service/package
// name (explicit "what to avoid" in the sketch findings). Selecting a
// destination replaces the workspace + inspector; it never mutates
// playback or output.
//
// `dimmed` (09-XX guide-navigation-guard pass) visually de-emphasizes the
// whole rail while the Guided First Show overlay is open -- signalling
// that a destination click won't silently do nothing, the way it used to
// (AppShell's GuardedCommandRail wrapper intercepts onSelect and shows a
// confirm dialog instead of navigating directly). Items stay fully
// clickable while dimmed -- dimming is a visual cue only, never
// pointer-events:none, since the click itself is what triggers that
// confirm dialog.
import { useEffect, useState } from "react";
import { Collapsible } from "@base-ui/react/collapsible";
import { NAV_GROUPS, type DestinationId } from "./navigation";
import { DESTINATION_ICONS } from "./destinationIcons";
import InfoTooltip from "../components/primitives/InfoTooltip/InfoTooltip";
import CommandRailGroupToggle from "./CommandRailGroupToggle";
import NavDestinationButton from "./NavDestinationButton";
import styles from "./CommandRail.module.css";

interface CommandRailProps {
  active: DestinationId;
  onSelect: (id: DestinationId) => void;
  dimmed?: boolean;
}

export default function CommandRail({ active, onSelect, dimmed = false }: CommandRailProps) {
  const [collapsedGroups, setCollapsedGroups] = useState<Set<string>>(() => new Set());

  // A group hiding the operator's current destination would leave the
  // rail with no visible active state at all -- if navigation elsewhere
  // (e.g. a keyboard shortcut) lands on a destination inside a group the
  // operator previously collapsed, reopen just that group rather than
  // leaving it hidden.
  useEffect(() => {
    const activeGroup = NAV_GROUPS.find((group) => group.destinations.some((destination) => destination.id === active));
    if (!activeGroup || !collapsedGroups.has(activeGroup.label)) return;
    setCollapsedGroups((current) => {
      const next = new Set(current);
      next.delete(activeGroup.label);
      return next;
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active]);

  // setGroupOpen takes Collapsible.Root's own onOpenChange boolean directly
  // (rather than toggling blind off the previous Set) -- Base UI is the
  // one calling this, already knowing exactly which state the panel is
  // moving to, so there's no need to re-derive "opposite of current" the
  // way the old raw-button onClick handler had to.
  const setGroupOpen = (label: string, open: boolean) => {
    setCollapsedGroups((current) => {
      const next = new Set(current);
      if (open) {
        next.delete(label);
      } else {
        next.add(label);
      }
      return next;
    });
  };

  return (
    <nav
      className={dimmed ? `${styles.rail} ${styles.railDimmed}` : styles.rail}
      aria-label="Workspace navigation"
    >
      {NAV_GROUPS.map((group) => {
        const collapsed = collapsedGroups.has(group.label);
        return (
          <Collapsible.Root
            key={group.label}
            className={styles.group}
            open={!collapsed}
            onOpenChange={(open) => setGroupOpen(group.label, open)}
          >
            <div className={styles.groupHeader}>
              <CommandRailGroupToggle label={group.label} />
              <InfoTooltip label={`About the ${group.label} section`} text={group.description} />
            </div>
            <Collapsible.Panel className={styles.groupItems}>
              {group.destinations.map((destination) => (
                <NavDestinationButton
                  key={destination.id}
                  destination={destination}
                  icon={DESTINATION_ICONS[destination.id]}
                  isActive={destination.id === active}
                  onSelect={() => onSelect(destination.id)}
                />
              ))}
            </Collapsible.Panel>
          </Collapsible.Root>
        );
      })}
    </nav>
  );
}
