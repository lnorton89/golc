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
// 09-03-PLAN.md Task 2 (this revision): every stage renders the same
// inert "not yet built" placeholder -- Task 3 replaces the Fixtures/Patch
// branches with FixturesStage/PatchStage, reading real domain state.
// Program/Assign/Verify stay placeholders until a later plan (09-04).
import { useEffect, useState } from "react";

import Button from "../../../components/primitives/Button/Button";
import { useGuidedFirstShow } from "./GuidedFirstShowContext";
import GuideEvidenceList from "./GuideEvidenceList";
import { GUIDE_STAGES, GUIDE_STAGE_LABELS, type GuideStageStatus } from "./stages";
import styles from "./GuidedFirstShow.module.css";

const PLACEHOLDER_STATUS: GuideStageStatus = { items: [], primaryLabel: "Continue", primaryDisabled: true };

export default function GuidedFirstShow() {
  const { stage, setStage, exitGuide } = useGuidedFirstShow();
  const [status, setStatus] = useState<GuideStageStatus>(PLACEHOLDER_STATUS);

  // Every stage currently reports the same placeholder status -- Task 3
  // differentiates Fixtures/Patch with real live-derived status.
  useEffect(() => {
    setStatus(PLACEHOLDER_STATUS);
  }, [stage]);

  const currentIndex = GUIDE_STAGES.indexOf(stage);

  const handleBack = () => {
    if (currentIndex > 0) {
      setStage(GUIDE_STAGES[currentIndex - 1]);
    }
  };

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
            <div className={styles.stageBody}>
              <p className={styles.loading}>This stage isn't built yet — a later plan completes it.</p>
            </div>
            <footer className={styles.footer}>
              <Button variant="secondary" onClick={handleBack} disabled={currentIndex === 0}>
                Back
              </Button>
              <Button variant="secondary" onClick={exitGuide}>
                Exit Guide
              </Button>
              <Button variant="primary" disabled={status.primaryDisabled}>
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
