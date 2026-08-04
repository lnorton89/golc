// CommandRailGroupToggle is the collapsible nav-group disclosure control,
// extracted to its own file (Desk's FaderLearnHitArea.tsx precedent, see
// 13-13-SUMMARY.md) so CommandRail.tsx's own raw <button> -- the
// per-destination nav item, which needs aria-current landmark semantics
// no shared primitive provides -- is the only DS005 "styled native
// control" diagnostic the checker finds there. Two raw <button> elements
// in one file always produce byte-identical DS005 diagnostic values
// ("button"); the checker's exception mechanism can only resolve a match
// to exactly one diagnostic per rule+path, so the two could never be
// excepted individually without this split (see
// design-system/exceptions.json).
import { ChevronDown, ChevronRight } from "lucide-react";
import styles from "./CommandRail.module.css";

interface CommandRailGroupToggleProps {
  label: string;
  collapsed: boolean;
  panelId: string;
  onToggle: () => void;
}

export default function CommandRailGroupToggle({ label, collapsed, panelId, onToggle }: CommandRailGroupToggleProps) {
  return (
    <button type="button" className={styles.groupToggle} aria-expanded={!collapsed} aria-controls={panelId} onClick={onToggle}>
      {collapsed ? (
        <ChevronRight size={12} className={styles.groupChevron} aria-hidden="true" />
      ) : (
        <ChevronDown size={12} className={styles.groupChevron} aria-hidden="true" />
      )}
      <span className={styles.groupLabel}>{label}</span>
    </button>
  );
}
