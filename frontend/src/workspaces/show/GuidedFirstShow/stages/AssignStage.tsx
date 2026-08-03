// AssignStage is the Guided First Show's fourth stage (09-04-PLAN.md
// Task 2, FDUI-03): on every mount it reads the operator surface count
// via window.go?.wails?.SurfaceService?.ListSurfaces() -- wailsBridge.ts's
// own documented "for a service without a helper yet, cast through
// window.go?.wails?.<Service>" escape hatch -- wrapped in a try/catch
// that falls back to an empty count, plus listProgramming() for the scene
// context, and derives its GuideStageStatus purely via readiness.ts's
// deriveAssignStatus. Its single primary action hands off to the real
// Operator Surface workspace (navigateTo("operate-operator-surface"),
// wired centrally by GuidedFirstShow.tsx's STAGE_DESTINATION map); this
// stage performs no assignment mutation of its own (T-09-04-04) --
// assigning is the Operator Surface workspace's job.
//
// 13-25-PLAN.md (D-05): loading/error text now render through the shared
// LoadingState/ErrorState primitives instead of bare <p> tags.
import { useCallback, useEffect, useState } from "react";

import { errorMessage, listProgramming } from "../../../../lib/wailsBridge";
import { ErrorState, LoadingState } from "../../../../design-system";
import { deriveAssignStatus } from "../readiness";
import type { GuideStageStatus } from "../stages";

interface AssignStageProps {
  onStatusChange: (status: GuideStageStatus) => void;
}

const PRIMARY_LABEL = "Go to Operator Surface";
const LOADING_STATUS: GuideStageStatus = { items: [], primaryLabel: "Loading…", primaryDisabled: true };

async function readSurfaceCount(): Promise<number> {
  try {
    const surfaces = await window.go?.wails?.SurfaceService?.ListSurfaces();
    return surfaces?.length ?? 0;
  } catch {
    return 0;
  }
}

export default function AssignStage({ onStatusChange }: AssignStageProps) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async (): Promise<void> => {
    try {
      const [surfaceCount, programming] = await Promise.all([readSurfaceCount(), listProgramming()]);
      onStatusChange(deriveAssignStatus(surfaceCount, programming));
      setError(null);
    } catch (err) {
      setError(errorMessage(err));
      onStatusChange({ items: [], primaryLabel: PRIMARY_LABEL, primaryDisabled: false });
    } finally {
      setLoading(false);
    }
  }, [onStatusChange]);

  // Re-reads operator surfaces and scene context on every mount (no
  // cached readiness flag) -- same discipline as the earlier stages,
  // deliberately keyed on an empty dependency array.
  useEffect(() => {
    onStatusChange(LOADING_STATUS);
    void refresh();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div>
      <p>Create an operator surface so someone else can play this show back.</p>
      {loading ? <LoadingState label="Checking operator surfaces…" /> : null}
      {error ? <ErrorState heading="Operator surfaces unavailable" message={error} /> : null}
    </div>
  );
}
