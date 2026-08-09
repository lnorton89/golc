// ProjectFixturesWorkspace wraps ProjectFixtures.tsx unchanged, mirroring
// PatchPoolsWorkspace's wrapper-only-supplies-chrome shape -- including
// PatchPoolsWorkspace's own missing-canvas-padding fix (260806 screenshot
// review), since WorkspaceFrame itself is deliberately unpadded and this
// was the second of two routes rendering flush against the window edge
// instead of matching every other route's shared .canvas gutter.
import { WorkspaceFrame } from "../../design-system";
import { HOW_IT_WORKS_BY_ID } from "../../shell/navigation";
import ProjectFixtures from "../../components/ProjectFixtures/ProjectFixtures";
import styles from "../workspace.module.css";

export default function ProjectFixturesWorkspace() {
  return (
    <WorkspaceFrame
      title="Project Fixtures"
      info={HOW_IT_WORKS_BY_ID["build-project-fixtures"]}
    >
      <div className={styles.canvas}>
        <ProjectFixtures />
      </div>
    </WorkspaceFrame>
  );
}
