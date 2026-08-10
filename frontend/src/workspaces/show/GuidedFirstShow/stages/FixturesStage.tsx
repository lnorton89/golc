// FixturesStage is the Guided First Show's first stage (09-03-PLAN.md
// Task 3, FDUI-03): on every mount it calls listLocalFixtures() and
// derives its GuideStageStatus purely via readiness.ts's
// deriveFixturesStatus (extracted in 09-04-PLAN.md Task 2) -- no
// persisted flag, no cached boolean, no state carried over from a
// previous visit (onboarding-readiness-impact.md's own interaction
// contract: "the guide reads actual domain readiness; it does not own
// duplicate state"). Its single primary action hands off to the real
// Fixture Library workspace rather than embedding a second fixture
// browser here.
//
// 13-25-PLAN.md (D-05): loading/error text now render through the shared
// LoadingState/ErrorState primitives instead of bare <p> tags -- the
// stage's own description paragraph is unchanged and inherits its
// typography from GuidedFirstShow.module.css's .stageBody ancestor.
import { useCallback, useEffect, useState } from "react";

import { errorMessage, listLocalFixtures } from "../../../../lib/wailsBridge";
import { ErrorState, LoadingState } from "../../../../design-system";
import { useLatestRequest } from "../../../../hooks/useLatestRequest";
import { deriveFixturesStatus } from "../readiness";
import type { GuideStageStatus } from "../stages";

interface FixturesStageProps {
  onStatusChange: (status: GuideStageStatus) => void;
}

const PRIMARY_LABEL = "Go to Fixture Library";
const LOADING_STATUS: GuideStageStatus = { items: [], primaryLabel: "Loading…", primaryDisabled: true };

export default function FixturesStage({ onStatusChange }: FixturesStageProps) {
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
      const view = await listLocalFixtures();
      if (!isCurrent()) {
        return;
      }
      onStatusChange(deriveFixturesStatus(view));
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

  // Re-reads the library on every mount (no cached readiness flag) -- see
  // this file's own top-of-file doc comment. Deliberately keyed on an
  // empty dependency array: this must run once per mount, not once per
  // render.
  useEffect(() => {
    onStatusChange(LOADING_STATUS);
    void refresh();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div>
      <p>Get at least one fixture into your library before patching it into the show.</p>
      {loading ? <LoadingState label="Checking your fixture library…" /> : null}
      {error ? <ErrorState heading="Fixture library unavailable" message={error} /> : null}
    </div>
  );
}
