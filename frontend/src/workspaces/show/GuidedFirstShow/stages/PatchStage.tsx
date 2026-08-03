// PatchStage is the Guided First Show's second stage (09-03-PLAN.md
// Task 3, FDUI-03): on every mount it calls listPatch() and derives its
// GuideStageStatus purely via readiness.ts's derivePatchStatus
// (extracted in 09-04-PLAN.md Task 2) -- same no-cached-state discipline
// as FixturesStage. Its single primary action ("Review Patch & Continue",
// the locked footer label) hands off to the real Patch & Pools workspace;
// this stage calls no mutating FixturePatchService method (T-09-03-01) --
// structural patch changes stay preview-only until the operator
// explicitly reviews and applies the deterministic impact plan there
// (onboarding-readiness-impact.md's own interaction contract: "Patch
// changes remain preview-only until the user reviews and applies the
// deterministic impact plan").
//
// 13-25-PLAN.md (D-05): loading/error text now render through the shared
// LoadingState/ErrorState primitives instead of bare <p> tags.
import { useCallback, useEffect, useState } from "react";

import { errorMessage, listPatch } from "../../../../lib/wailsBridge";
import { ErrorState, LoadingState } from "../../../../design-system";
import { derivePatchStatus } from "../readiness";
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
      onStatusChange(derivePatchStatus(view));
      setError(null);
    } catch (err) {
      setError(errorMessage(err));
      onStatusChange({ items: [], primaryLabel: PRIMARY_LABEL, primaryDisabled: false });
    } finally {
      setLoading(false);
    }
  }, [onStatusChange]);

  // Re-reads the patch state on every mount (no cached readiness flag) --
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
      {loading ? <LoadingState label="Checking your patch…" /> : null}
      {error ? <ErrorState heading="Patch unavailable" message={error} /> : null}
    </div>
  );
}
