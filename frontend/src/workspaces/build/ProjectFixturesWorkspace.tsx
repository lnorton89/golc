// ProjectFixturesWorkspace wraps ProjectFixtures.tsx unchanged, mirroring
// PatchPoolsWorkspace's identical wrapper-only-supplies-chrome shape.
import { InfoTooltip, WorkspaceFrame } from "../../design-system";
import { HOW_IT_WORKS_BY_ID } from "../../shell/navigation";
import ProjectFixtures from "../../components/ProjectFixtures/ProjectFixtures";

export default function ProjectFixturesWorkspace() {
  return (
    <WorkspaceFrame
      title="Project Fixtures"
      action={<InfoTooltip label="How Project Fixtures works" text={HOW_IT_WORKS_BY_ID["build-project-fixtures"]} />}
    >
      <ProjectFixtures />
    </WorkspaceFrame>
  );
}
