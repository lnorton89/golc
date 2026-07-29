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
import { NAV_GROUPS, type DestinationId } from "./navigation";
import { DESTINATION_ICONS } from "./destinationIcons";
import styles from "./CommandRail.module.css";

interface CommandRailProps {
  active: DestinationId;
  onSelect: (id: DestinationId) => void;
  dimmed?: boolean;
}

export default function CommandRail({ active, onSelect, dimmed = false }: CommandRailProps) {
  return (
    <nav
      className={dimmed ? `${styles.rail} ${styles.railDimmed}` : styles.rail}
      aria-label="Workspace navigation"
    >
      {NAV_GROUPS.map((group) => (
        <div key={group.label} className={styles.group}>
          <span className={styles.groupLabel}>{group.label}</span>
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
      ))}
    </nav>
  );
}
