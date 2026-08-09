// PatchPoolsWorkspace wraps FixturePatch.tsx unchanged (shell restructure
// plan Step 9 CSS-retargets its internals; this wrapper only supplies the
// workspace toolbar/canvas chrome around it) -- WorkspaceFrame itself is
// deliberately unpadded (patterns.module.css's own .workspaceFrame), so
// this needs the same shared .canvas padding wrapper every sibling
// workspace already applies (OverviewWorkspace.module.css's own .canvas,
// MidiMappingWorkspace's shared workspace.module.css, etc.) -- without it,
// this was the one route rendering flush against the toolbar/window edge
// (260806 screenshot review: inconsistent gutters vs. every other route).
import { WorkspaceFrame } from "../../design-system";
import { HOW_IT_WORKS_BY_ID } from "../../shell/navigation";
import FixturePatch from "../../components/FixturePatch/FixturePatch";
import styles from "../workspace.module.css";

export default function PatchPoolsWorkspace() {
  return (
    <WorkspaceFrame
      title="Patch & Pools"
      info={HOW_IT_WORKS_BY_ID["build-patch-pools"]}
    >
      <div className={styles.canvas}>
        <FixturePatch />
      </div>
    </WorkspaceFrame>
  );
}
