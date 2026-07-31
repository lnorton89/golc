// ListRow is the shared row primitive for every list in the shell: surface
// list, mapping list, pool/deployment list, scene list. Renders as a button
// when onSelect is provided (keyboard-addressable selection), otherwise as
// a plain row. `locked` never dispatches onSelect -- mirrors the D-04
// "visible but locked, never hidden" convention already established by
// OperatorSurface.tsx's operate-mode rendering.
//
// `icon` is optional (same lucide-react component-reference convention as
// Button/PanelHeader/Toolbar) -- every existing call site is unaffected.
//
// `actions` is an optional trailing slot (e.g. rename/delete buttons)
// rendered outside the selectable button/row so its own clicks never
// trigger onSelect -- purely additive, every existing call site (which
// never passes it) is unaffected.
import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";

import styles from "./ListRow.module.css";

interface ListRowProps {
  label: string;
  icon?: LucideIcon;
  meta?: ReactNode;
  selected?: boolean;
  locked?: boolean;
  onSelect?: () => void;
  actions?: ReactNode;
}

export default function ListRow({
  label,
  icon: Icon,
  meta,
  selected = false,
  locked = false,
  onSelect,
  actions,
}: ListRowProps) {
  const className = [
    styles.row,
    selected ? styles.selected : "",
    locked ? styles.locked : "",
  ]
    .filter(Boolean)
    .join(" ");

  const content = (
    <>
      {Icon ? <Icon size={14} className={styles.icon} aria-hidden="true" /> : null}
      <span className={styles.label}>{label}</span>
      {meta ? <span className={styles.meta}>{meta}</span> : null}
    </>
  );

  if (!onSelect) {
    return (
      <div className={className} aria-disabled={locked} title={label}>
        {content}
        {actions}
      </div>
    );
  }

  return (
    <>
      <button
        type="button"
        className={className}
        aria-pressed={selected}
        aria-disabled={locked}
        disabled={locked}
        title={label}
        onClick={onSelect}
      >
        {content}
      </button>
      {actions}
    </>
  );
}
