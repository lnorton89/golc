// CommandRail is the left navigation from application-shell-navigation.md's
// Focused Command Rail (Sketch 001 Variant D) -- grouped by stable user
// intent (Show/Build/Operate/Output), never by backend service/package
// name (explicit "what to avoid" in the sketch findings). Selecting a
// destination replaces the workspace + inspector; it never mutates
// playback or output.
import { NAV_GROUPS, type DestinationId } from "./navigation";
import styles from "./CommandRail.module.css";

interface CommandRailProps {
  active: DestinationId;
  onSelect: (id: DestinationId) => void;
}

export default function CommandRail({ active, onSelect }: CommandRailProps) {
  return (
    <nav className={styles.rail} aria-label="Workspace navigation">
      {NAV_GROUPS.map((group) => (
        <div key={group.label} className={styles.group}>
          <span className={styles.groupLabel}>{group.label}</span>
          {group.destinations.map((destination) => {
            const isActive = destination.id === active;
            return (
              <button
                key={destination.id}
                type="button"
                className={isActive ? `${styles.item} ${styles.itemActive}` : styles.item}
                aria-current={isActive ? "page" : undefined}
                onClick={() => onSelect(destination.id)}
              >
                {destination.label}
              </button>
            );
          })}
        </div>
      ))}
    </nav>
  );
}
