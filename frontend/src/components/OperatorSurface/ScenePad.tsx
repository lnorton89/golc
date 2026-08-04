// ScenePad is one cell of the Launcher + Masters scene grid (live-
// operation-safety-midi.md, Sketch 003 Variant A): a large random-access
// launch target. Assigned scenes are interactive; unassigned/locked scenes
// render dimmed and NEVER dispatch (D-04 "visible but locked, never
// hidden" -- the lock is enforced server-side by AuthorizeControl, this
// dimmed/disabled rendering is a UI affordance only, matching
// OperatorSurface.tsx's own existing doctrine).
//
// This remains a raw native <button> (registered as a narrow DS005 domain
// exception, design-system/exceptions.json)
// rather than the shared Button primitive: a launch-pad grid cell's
// stacked name/LIVE/Locked-tag content and fixed 88px minimum height are
// domain-specific launcher-grid geometry, not the single-line label
// grammar Button owns -- there is no launch-pad/grid-cell primitive
// elsewhere in this codebase to reuse (same reasoning as Desk's own
// FaderLearnHitArea/faderClearButton DS005 exceptions).
import styles from "./ScenePad.module.css";

interface ScenePadProps {
  name: string;
  live: boolean;
  locked: boolean;
  onLaunch: () => void;
}

export default function ScenePad({ name, live, locked, onLaunch }: ScenePadProps) {
  return (
    <button
      type="button"
      className={styles.pad}
      aria-current={live ? "true" : undefined}
      aria-disabled={locked}
      disabled={locked}
      title={name}
      onClick={onLaunch}
    >
      <span className={styles.name}>{name}</span>
      {live ? <span className={styles.liveTag}>LIVE</span> : null}
      {locked ? <span className={styles.lockedTag}>Locked</span> : null}
    </button>
  );
}
