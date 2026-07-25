// OperatorSurfaceWorkspace wraps OperatorSurface.tsx, whose "operate" mode
// now renders the Launcher + Masters pattern (shell restructure plan Step
// 7). An earlier interim version of this file also rendered a standalone
// scene-quick-switch section (from Step 5, before Launcher existed) --
// that's retired now: showing it alongside OperatorSurface's own Launcher
// would duplicate scene-switching UI the moment a surface is in operate
// mode.
import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import OperatorSurface from "../../components/OperatorSurface/OperatorSurface";
import styles from "../workspace.module.css";

export default function OperatorSurfaceWorkspace() {
  return (
    <div className={styles.workspace}>
      <Toolbar title="Operator Surface" />
      <div className={styles.canvas}>
        <OperatorSurface />
      </div>
    </div>
  );
}
