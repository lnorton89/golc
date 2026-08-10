// VerifyStage is the Guided First Show's fifth and final stage
// (09-04-PLAN.md Task 3, FDUI-03): on every mount it performs all four
// live reads in parallel (listLocalFixtures, listPatch, listProgramming,
// and the operator-surface count via wailsBridge.ts's shared
// readSurfaceCount(), the same helper AssignStage.tsx
// uses), re-derives all four upstream stage statuses
// through readiness.ts's pure functions, and rolls them up with
// aggregateReadiness -- it stores no result across mounts and reads no
// persisted readiness record anywhere (T-09-04-01: a green verdict must
// always reflect the show's actual current state, never a cached one).
//
// Its single primary action is the Perform transition
// (navigateTo("operate-operator-surface"), wired centrally by
// GuidedFirstShow.tsx's STAGE_DESTINATION map): disabled exactly when the
// blocker count is greater than zero. This disabled state is the ONLY
// thing a blocker gates anywhere in the guide (T-09-04-02) -- the stage
// rail stays fully selectable and every other workspace stays fully
// editable regardless of outstanding blockers.
//
// Blocker/warning/evidence stay three independently counted categories,
// each rendered with the locked status word plus its own
// singular/plural-agreeing count ("1 blocker", "0 warnings") -- never a
// combined score, ratio, or bar. This <ul aria-label="Readiness summary">
// structure and its exact text are asserted directly by
// GuidedFirstShow.test.tsx and are NOT converted to a shared primitive
// (13-25-PLAN.md): no inventory primitive renders this three-category,
// singular/plural-agreeing count shape, and the locked design's own "no
// combined score/progressbar" contract is easiest to keep verifiably true
// in this stage's own plain markup.
//
// 13-25-PLAN.md (D-05): only the stage's own top-level loading/error text
// now render through the shared LoadingState/ErrorState primitives.
import { useCallback, useEffect, useState } from "react";

import { errorMessage, listLocalFixtures, listPatch, listProgramming, readSurfaceCount } from "../../../../lib/wailsBridge";
import { ErrorState, LoadingState } from "../../../../design-system";
import { useLatestRequest } from "../../../../hooks/useLatestRequest";
import {
  aggregateReadiness,
  deriveAssignStatus,
  deriveFixturesStatus,
  deriveProgramStatus,
  derivePatchStatus,
  type ReadinessRollup,
} from "../readiness";
import type { GuideStageStatus } from "../stages";

interface VerifyStageProps {
  onStatusChange: (status: GuideStageStatus) => void;
}

const PRIMARY_LABEL = "Perform";
const LOADING_STATUS: GuideStageStatus = { items: [], primaryLabel: "Loading…", primaryDisabled: true };

function pluralize(count: number, noun: string): string {
  return `${count} ${noun}${count === 1 ? "" : "s"}`;
}

export default function VerifyStage({ onStatusChange }: VerifyStageProps) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [rollup, setRollup] = useState<ReadinessRollup | null>(null);

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
      const [fixturesView, patchView, progView, surfaceCount] = await Promise.all([
        listLocalFixtures(),
        listPatch(),
        listProgramming(),
        readSurfaceCount(),
      ]);
      if (!isCurrent()) {
        return;
      }

      const statuses: GuideStageStatus[] = [
        deriveFixturesStatus(fixturesView),
        derivePatchStatus(patchView),
        deriveProgramStatus(progView),
        deriveAssignStatus(surfaceCount, progView),
      ];
      const nextRollup = aggregateReadiness(statuses);
      setRollup(nextRollup);

      onStatusChange({
        items: nextRollup.items,
        primaryLabel: PRIMARY_LABEL,
        primaryDisabled: nextRollup.blockers > 0,
      });
      setError(null);
    } catch (err) {
      if (!isCurrent()) {
        return;
      }
      setError(errorMessage(err));
      setRollup(null);
      onStatusChange({ items: [], primaryLabel: PRIMARY_LABEL, primaryDisabled: true });
    } finally {
      if (isCurrent()) {
        setLoading(false);
      }
    }
  }, [onStatusChange, beginLatest]);

  // Re-derives readiness from four live reads on every mount -- no result
  // cached across mounts (T-09-04-01), deliberately keyed on an empty
  // dependency array.
  useEffect(() => {
    onStatusChange(LOADING_STATUS);
    void refresh();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div>
      <p>
        Review the readiness evidence gathered from every stage before handing this show off to a
        player.
      </p>
      {loading ? <LoadingState label="Checking readiness…" /> : null}
      {error ? <ErrorState heading="Readiness check unavailable" message={error} /> : null}
      {rollup ? (
        <ul aria-label="Readiness summary">
          <li>
            <span>Blocker</span>
            <span>{pluralize(rollup.blockers, "blocker")}</span>
          </li>
          <li>
            <span>Warning</span>
            <span>{pluralize(rollup.warnings, "warning")}</span>
          </li>
          <li>
            <span>Evidence</span>
            <span>{pluralize(rollup.evidence, "evidence item")}</span>
          </li>
        </ul>
      ) : null}
      {rollup && rollup.blockers > 0 ? (
        <p>
          Perform stays disabled until every blocker above is resolved -- this never restricts editing
          in any other workspace.
        </p>
      ) : null}
    </div>
  );
}
