// GuidedFirstShow is Sketch 004 Variant B's locked overlay (D-08/D-10/
// D-11, .planning/sketches/references/onboarding-readiness-impact.md): a
// canvas-replacing flow (never a nav destination, D-10) with a fixed
// five-stage rail (Fixtures/Patch/Program/Assign/Verify), the active
// stage's own content and single primary action, and a live evidence
// aside that derives every blocker/warning/evidence row from the show's
// actual domain state on render -- never a persisted progress record,
// never a completion percentage (locked design's own "What to Avoid"
// list).
//
// 09-03-PLAN.md Task 3 wired Fixtures/Patch into the stage-content switch
// below, passing each stage's own reported status up to this shared
// rail/footer/evidence-aside contract. 09-04-PLAN.md Task 2 adds
// Program/Assign against the identical GuideStageStatus contract; Task 3
// finishes with the real Verify stage and its evidence-based Perform
// gate.
import { useEffect, useState, type ReactNode } from "react";

import Button from "../../../components/primitives/Button/Button";
import type { DestinationId } from "../../../shell/navigation";
import { useGuidedFirstShow } from "./GuidedFirstShowContext";
import GuideEvidenceList from "./GuideEvidenceList";
import { GUIDE_STAGES, GUIDE_STAGE_LABELS, type GuideStageId, type GuideStageStatus } from "./stages";
import FixturesStage from "./stages/FixturesStage";
import PatchStage from "./stages/PatchStage";
import ProgramStage from "./stages/ProgramStage";
import AssignStage from "./stages/AssignStage";
import styles from "./GuidedFirstShow.module.css";

// STAGE_DESTINATION names the real workspace each stage's primary action
// hands off to -- the guide itself never mutates anything (T-09-03-01/
// T-09-04-04). Verify falls through to the inert placeholder below until
// 09-04-PLAN.md Task 3 replaces it with the real evidence-based stage,
// whose primary action is the Perform transition rather than a plain
// hand-off.
const STAGE_DESTINATION: Partial<Record<GuideStageId, DestinationId>> = {
  fixtures: "build-fixture-library",
  patch: "build-patch-pools",
  program: "build-scenes-looks",
  assign: "operate-operator-surface",
};

const PLACEHOLDER_STATUS: GuideStageStatus = { items: [], primaryLabel: "Continue", primaryDisabled: true };

export default function GuidedFirstShow() {
  const { stage, setStage, exitGuide, navigateTo } = useGuidedFirstShow();
  const [status, setStatus] = useState<GuideStageStatus>(PLACEHOLDER_STATUS);

  // Resets to the placeholder status whenever the active stage changes --
  // Fixtures/PatchStage's own mount effect (below, via onStatusChange)
  // then reports the real derived status once its own live read resolves.
  useEffect(() => {
    setStatus(PLACEHOLDER_STATUS);
  }, [stage]);

  const currentIndex = GUIDE_STAGES.indexOf(stage);

  const handleBack = () => {
    if (currentIndex > 0) {
      setStage(GUIDE_STAGES[currentIndex - 1]);
    }
  };

  const handlePrimary = () => {
    const destination = STAGE_DESTINATION[stage];
    if (destination) {
      navigateTo(destination);
    }
  };

  let stageContent: ReactNode;
  if (stage === "fixtures") {
    stageContent = <FixturesStage onStatusChange={setStatus} />;
  } else if (stage === "patch") {
    stageContent = <PatchStage onStatusChange={setStatus} />;
  } else if (stage === "program") {
    stageContent = <ProgramStage onStatusChange={setStatus} />;
  } else if (stage === "assign") {
    stageContent = <AssignStage onStatusChange={setStatus} />;
  } else {
    stageContent = <p className={styles.loading}>This stage isn't built yet — a later plan completes it.</p>;
  }

  return (
    <div className={styles.overlay}>
      <div className={styles["guided-flow"]}>
        <nav aria-label="First show steps" className={styles.rail}>
          {GUIDE_STAGES.map((id) => (
            <button
              key={id}
              type="button"
              className={styles["guide-step"]}
              aria-current={id === stage ? "step" : undefined}
              onClick={() => setStage(id)}
            >
              {GUIDE_STAGE_LABELS[id]}
            </button>
          ))}
        </nav>

        <div className={styles.contentArea}>
          <section aria-labelledby="current-step-title" className={styles.stageSection}>
            <header className={styles.stageHeader}>
              <h2 id="current-step-title">{GUIDE_STAGE_LABELS[stage]}</h2>
            </header>
            <div className={styles.stageBody}>{stageContent}</div>
            <footer className={styles.footer}>
              <Button variant="secondary" onClick={handleBack} disabled={currentIndex === 0}>
                Back
              </Button>
              <Button variant="secondary" onClick={exitGuide}>
                Exit Guide
              </Button>
              <Button variant="primary" onClick={handlePrimary} disabled={status.primaryDisabled}>
                {status.primaryLabel}
              </Button>
            </footer>
          </section>

          <aside aria-label="Live preview and evidence" className={styles.evidenceAside}>
            <GuideEvidenceList items={status.items} />
          </aside>
        </div>
      </div>
    </div>
  );
}
