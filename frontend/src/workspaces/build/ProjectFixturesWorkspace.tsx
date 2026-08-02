// ProjectFixturesWorkspace wraps ProjectFixtures.tsx unchanged, mirroring
// PatchPoolsWorkspace's identical wrapper-only-supplies-chrome shape.
import { ListChecks } from "lucide-react";

import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import { HOW_IT_WORKS_BY_ID } from "../../shell/navigation";
import ProjectFixtures from "../../components/ProjectFixtures/ProjectFixtures";
import styles from "../workspace.module.css";

export default function ProjectFixturesWorkspace() {
  return (
    <div className={styles.workspace}>
      <Toolbar title="Project Fixtures" icon={ListChecks} info={HOW_IT_WORKS_BY_ID["build-project-fixtures"]} />
      <div className={styles.canvas}>
        <ProjectFixtures />
      </div>
    </div>
  );
}
