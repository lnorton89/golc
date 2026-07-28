// FixturesStage is the Guided First Show's first stage (09-03-PLAN.md
// Task 3, FDUI-03): on every mount it calls listLocalFixtures() and
// derives its GuideStageStatus purely from the returned rows -- no
// persisted flag, no cached boolean, no state carried over from a
// previous visit (onboarding-readiness-impact.md's own interaction
// contract: "the guide reads actual domain readiness; it does not own
// duplicate state"). Its single primary action hands off to the real
// Fixture Library workspace rather than embedding a second fixture
// browser here.
import { useCallback, useEffect, useState } from "react";

import { errorMessage, listLocalFixtures } from "../../../../lib/wailsBridge";
import type { GuideStageStatus } from "../stages";

interface FixturesStageProps {
  onStatusChange: (status: GuideStageStatus) => void;
}

const PRIMARY_LABEL = "Go to Fixture Library";
const LOADING_STATUS: GuideStageStatus = { items: [], primaryLabel: "Loading…", primaryDisabled: true };

export default function FixturesStage({ onStatusChange }: FixturesStageProps) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async (): Promise<void> => {
    try {
      const view = await listLocalFixtures();
      const validCount = view.rows.filter((row) => row.status === "valid").length;
      const invalidCount = view.rows.length - validCount;

      const items: GuideStageStatus["items"] = [];
      if (validCount === 0) {
        items.push({
          tone: "blocker",
          label: "No fixtures yet",
          detail: "The show has no fixture definitions available to patch yet.",
        });
      } else {
        items.push({
          tone: "evidence",
          label: "Fixture library",
          detail: `${validCount} fixture${validCount === 1 ? "" : "s"} available in your library.`,
        });
      }
      if (invalidCount > 0) {
        items.push({
          tone: "warning",
          label: "Validation issues",
          detail: `${invalidCount} definition${invalidCount === 1 ? "" : "s"} failed validation.`,
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

  // Re-reads the library on every mount (no cached progress flag) -- see
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
      {loading ? <p>Checking your fixture library…</p> : null}
      {error ? <p>{error}</p> : null}
    </div>
  );
}
