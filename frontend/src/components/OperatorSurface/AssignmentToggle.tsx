// AssignmentToggle.tsx is the in-place, per-item "add to this operator
// surface" control (06-07-PLAN.md Task 2, D-01/D-03): a plain checkbox
// shown directly on one scene/layer/master/safety control's row in
// OperatorSurface.tsx's authoring view -- never a separate builder screen,
// and never a group/category-level bulk-assign control (there is no
// "select all" affordance anywhere in this component tree). The checkbox
// state directly reflects the control's real assignment membership
// (`assigned`), and `onToggle` always issues the exact opposite of the
// current membership; because AssignItem/UnassignItem are themselves
// idempotent no-ops server-side (PLAY-03), re-toggling an already-assigned
// item can never desync the checkbox from real membership.

import styles from "./OperatorSurface.module.css";

interface AssignmentToggleProps {
  label: string;
  assigned: boolean;
  onToggle: () => void;
  disabled?: boolean;
}

export default function AssignmentToggle({
  label,
  assigned,
  onToggle,
  disabled = false,
}: AssignmentToggleProps) {
  return (
    <label className={styles.toggleRow} title={label}>
      <input
        type="checkbox"
        checked={assigned}
        disabled={disabled}
        onChange={onToggle}
        aria-label={`Add ${label} to this operator surface`}
      />
      <span className={styles.toggleLabel}>{label}</span>
    </label>
  );
}
