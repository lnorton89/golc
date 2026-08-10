// ProgramStage is the Guided First Show's third stage (09-04-PLAN.md
// Task 2, FDUI-03): on every mount it calls listProgramming() and derives
// its GuideStageStatus purely via readiness.ts's deriveProgramStatus --
// same no-cached-state discipline as FixturesStage/PatchStage. Its single
// primary action hands off to the real Scenes & Looks workspace
// (navigateTo("build-scenes-looks"), wired centrally by
// GuidedFirstShow.tsx's STAGE_DESTINATION map); this stage performs no
// programming mutation of its own (T-09-04-04).
//
// 13-25-PLAN.md (D-05): loading/error text now render through the shared
// LoadingState/ErrorState primitives instead of bare <p> tags.
import { useCallback, useEffect, useState } from "react";

import { errorMessage, listProgramming } from "../../../../lib/wailsBridge";
import { ErrorState, LoadingState } from "../../../../design-system";
import { useLatestRequest } from "../../../../hooks/useLatestRequest";
import { deriveProgramStatus } from "../readiness";
import type { GuideStageStatus } from "../stages";

interface ProgramStageProps {
  onStatusChange: (status: GuideStageStatus) => void;
}

const PRIMARY_LABEL = "Go to Scenes & Looks";
const LOADING_STATUS: GuideStageStatus = { items: [], primaryLabel: "Loading…", primaryDisabled: true };

export default function ProgramStage({ onStatusChange }: ProgramStageProps) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Every stage reports upward through onStatusChange from an async read,
  // and switching stages unmounts the previous one -- an in-flight read
  // landing afterwards used to overwrite the *new* stage's status, so the
  // footer's primary button showed the old stage's label and disabled
  // state over the new stage's body. useLatestRequest answers both "did a
  // newer read start" and "are we still mounted" with one predicate.
  const beginLatest = useLatestRequest();

  const refresh = useCallback(async (): Promise<void> => {
    const isCurrent = beginLatest();
    try {
      const view = await listProgramming();
      if (!isCurrent()) {
        return;
      }
      onStatusChange(deriveProgramStatus(view));
      setError(null);
    } catch (err) {
      if (!isCurrent()) {
        return;
      }
      setError(errorMessage(err));
      onStatusChange({ items: [], primaryLabel: PRIMARY_LABEL, primaryDisabled: false });
    } finally {
      if (isCurrent()) {
        setLoading(false);
      }
    }
  }, [onStatusChange, beginLatest]);

  // Re-reads the show's programming on every mount (no cached readiness
  // flag) -- same discipline as FixturesStage/PatchStage, deliberately
  // keyed on an empty dependency array.
  useEffect(() => {
    onStatusChange(LOADING_STATUS);
    void refresh();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div>
      <p>Program at least one scene before assigning it to an operator surface.</p>
      {loading ? <LoadingState label="Checking your show's programming…" /> : null}
      {error ? <ErrorState heading="Programming unavailable" message={error} /> : null}
    </div>
  );
}
