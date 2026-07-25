// ListRow is the shared row primitive for every list in the shell: surface
// list, mapping list, pool/deployment list, scene list. Renders as a button
// when onSelect is provided (keyboard-addressable selection), otherwise as
// a plain row. `locked` never dispatches onSelect -- mirrors the D-04
// "visible but locked, never hidden" convention already established by
// OperatorSurface.tsx's operate-mode rendering.
import type { ReactNode } from "react";

import styles from "./ListRow.module.css";

interface ListRowProps {
  label: string;
  meta?: ReactNode;
  selected?: boolean;
  locked?: boolean;
  onSelect?: () => void;
}

export default function ListRow({ label, meta, selected = false, locked = false, onSelect }: ListRowProps) {
  const className = [
    styles.row,
    selected ? styles.selected : "",
    locked ? styles.locked : "",
  ]
    .filter(Boolean)
    .join(" ");

  if (!onSelect) {
    return (
      <div className={className} aria-disabled={locked} title={label}>
        <span className={styles.label}>{label}</span>
        {meta ? <span className={styles.meta}>{meta}</span> : null}
      </div>
    );
  }

  return (
    <button
      type="button"
      className={className}
      aria-pressed={selected}
      aria-disabled={locked}
      disabled={locked}
      title={label}
      onClick={onSelect}
    >
      <span className={styles.label}>{label}</span>
      {meta ? <span className={styles.meta}>{meta}</span> : null}
    </button>
  );
}
