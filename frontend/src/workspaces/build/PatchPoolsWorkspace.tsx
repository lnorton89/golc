// PatchPoolsWorkspace wraps FixturePatch.tsx unchanged (shell restructure
// plan Step 9 CSS-retargets its internals; this wrapper only supplies the
// workspace toolbar/canvas chrome around it).
import { Cable } from "lucide-react";

import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import FixturePatch from "../../components/FixturePatch/FixturePatch";
import styles from "../workspace.module.css";

export default function PatchPoolsWorkspace() {
  return (
    <div className={styles.workspace}>
      <Toolbar title="Patch & Pools" icon={Cable} />
      <div className={styles.canvas}>
        <FixturePatch />
      </div>
    </div>
  );
}
