// PatchPoolsWorkspace wraps FixturePatch.tsx unchanged (shell restructure
// plan Step 9 CSS-retargets its internals; this wrapper only supplies the
// workspace toolbar/canvas chrome around it).
import { InfoTooltip, WorkspaceFrame } from "../../design-system";
import { HOW_IT_WORKS_BY_ID } from "../../shell/navigation";
import FixturePatch from "../../components/FixturePatch/FixturePatch";

export default function PatchPoolsWorkspace() {
  return (
    <WorkspaceFrame
      title="Patch & Pools"
      action={<InfoTooltip label="How Patch & Pools works" text={HOW_IT_WORKS_BY_ID["build-patch-pools"]} />}
    >
      <FixturePatch />
    </WorkspaceFrame>
  );
}
