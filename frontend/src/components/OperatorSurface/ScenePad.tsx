// ScenePad is one cell of the Launcher + Masters scene grid (live-
// operation-safety-midi.md, Sketch 003 Variant A): a large random-access
// launch target. Assigned scenes are interactive; unassigned/locked scenes
// render dimmed and NEVER dispatch (D-04 "visible but locked, never
// hidden" -- the lock is enforced server-side by AuthorizeControl, this
// dimmed/disabled rendering is a UI affordance only, matching
// OperatorSurface.tsx's own existing doctrine).
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
