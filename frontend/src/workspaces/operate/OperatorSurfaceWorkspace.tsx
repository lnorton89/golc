// OperatorSurfaceWorkspace wraps OperatorSurface.tsx, whose "operate" mode
// renders the Launcher + Masters pattern (shell restructure plan Step 7).
// Phase 13 (unified design system, 13-14-PLAN.md Task 1) retargets this
// wrapper onto the shared WorkspaceFrame pattern -- the same
// toolbar/canvas chrome every other migrated workspace wrapper now uses
// (PatchPoolsWorkspace.tsx is the closest analog: a thin wrapper around an
// unchanged feature component) -- without touching OperatorSurface.tsx's
// own mount contract or any command/dispatch path.
import { WorkspaceFrame } from "../../design-system";
import { HOW_IT_WORKS_BY_ID } from "../../shell/navigation";
import OperatorSurface from "../../components/OperatorSurface/OperatorSurface";
import styles from "./OperatorSurfaceWorkspace.module.css";

export default function OperatorSurfaceWorkspace() {
  return (
    <WorkspaceFrame
      title="Operator Surface"
      info={HOW_IT_WORKS_BY_ID["operate-operator-surface"]}
    >
      <div className={styles.canvas}>
        <OperatorSurface />
      </div>
    </WorkspaceFrame>
  );
}
