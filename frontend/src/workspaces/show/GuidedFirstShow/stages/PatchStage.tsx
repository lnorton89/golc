// PatchStage is the Guided First Show's second stage (09-03-PLAN.md
// Task 3, FDUI-03): on every mount it calls listPatch() and derives its
// GuideStageStatus purely from the returned pools/deployments -- same
// no-cached-state discipline as FixturesStage. Its single primary action
// ("Review Patch & Continue", the locked footer label) hands off to the
// real Patch & Pools workspace; this stage calls no mutating
// FixturePatchService method (T-09-03-01) -- structural patch changes
// stay preview-only until the operator explicitly reviews and applies
// the deterministic impact plan there
// (onboarding-readiness-impact.md's own interaction contract: "Patch
// changes remain preview-only until the user reviews and applies the
// deterministic impact plan").
import { useCallback, useEffect, useState } from "react";

import { errorMessage, listPatch } from "../../../../lib/wailsBridge";
import type { GuideStageStatus } from "../stages";

interface PatchStageProps {
  onStatusChange: (status: GuideStageStatus) => void;
}

const PRIMARY_LABEL = "Review Patch & Continue";
const LOADING_STATUS: GuideStageStatus = { items: [], primaryLabel: "Loading…", primaryDisabled: true };

export default function PatchStage({ onStatusChange }: PatchStageProps) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async (): Promise<void> => {
    try {
      const view = await listPatch();
      const hasActiveDeployment = view.deployments.some((deployment) => deployment.active);

      const items: GuideStageStatus["items"] = [];
      if (view.pools.length === 0) {
        items.push({
          tone: "blocker",
          label: "No pools yet",
          detail: "Create a fixture pool before patching it into a deployment.",
        });
      } else if (!hasActiveDeployment) {
        items.push({
          tone: "warning",
          label: "No active deployment",
          detail: "You have at least one pool, but no deployment is marked active yet.",
        });
      } else {
        items.push({
          tone: "evidence",
          label: "Patch ready",
          detail: "An active deployment is patched and ready to play.",
        });
      }

      onStatusChange({ items, primaryLabel: PRIMARY_LABEL, primaryDisabled: false });
      setError(null);
    } catch (err) {
      setError(errorMessage(err));
      onStatusChange({ items: [], primaryLabel: PRIMARY_LABEL, primaryDisabled: false });
    } finally {
      setLoading(false);
    }
  }, [onStatusChange]);

  // Re-reads the patch state on every mount (no cached progress flag) --
  // same discipline as FixturesStage, deliberately keyed on an empty
  // dependency array.
  useEffect(() => {
    onStatusChange(LOADING_STATUS);
    void refresh();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div>
      <p>Patch at least one fixture pool into an active deployment before programming a scene.</p>
      {loading ? <p>Checking your patch…</p> : null}
      {error ? <p>{error}</p> : null}
    </div>
  );
}
