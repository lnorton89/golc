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
import { ChevronDown, ChevronRight } from "lucide-react";
import { NAV_GROUPS, type DestinationId } from "./navigation";
import { DESTINATION_ICONS } from "./destinationIcons";
import InfoTooltip from "../components/primitives/InfoTooltip/InfoTooltip";
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

  const toggleGroup = (label: string) => {
    setCollapsedGroups((current) => {
      const next = new Set(current);
      if (next.has(label)) {
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
        const panelId = `nav-group-${group.label}`;
        return (
          <div key={group.label} className={styles.group}>
            <div className={styles.groupHeader}>
              <button
                type="button"
                className={styles.groupToggle}
                aria-expanded={!collapsed}
                aria-controls={panelId}
                onClick={() => toggleGroup(group.label)}
              >
                {collapsed ? (
                  <ChevronRight size={12} className={styles.groupChevron} aria-hidden="true" />
                ) : (
                  <ChevronDown size={12} className={styles.groupChevron} aria-hidden="true" />
                )}
                <span className={styles.groupLabel}>{group.label}</span>
              </button>
              <InfoTooltip label={`About the ${group.label} section`} text={group.description} />
            </div>
            {collapsed ? null : (
              <div id={panelId} className={styles.groupItems}>
                {group.destinations.map((destination) => {
                  const isActive = destination.id === active;
                  const Icon = DESTINATION_ICONS[destination.id];
                  return (
                    <button
                      key={destination.id}
                      type="button"
                      className={isActive ? `${styles.item} ${styles.itemActive}` : styles.item}
                      aria-current={isActive ? "page" : undefined}
                      onClick={() => onSelect(destination.id)}
                    >
                      <Icon size={15} className={styles.itemIcon} aria-hidden="true" />
                      {destination.label}
                    </button>
                  );
                })}
              </div>
            )}
          </div>
        );
      })}
    </nav>
  );
}
