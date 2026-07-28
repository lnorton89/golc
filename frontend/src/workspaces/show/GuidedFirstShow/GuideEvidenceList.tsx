// GuideEvidenceList renders a Guided First Show stage's evidence aside
// content (09-03-PLAN.md, locked design): each blocker/warning/evidence
// item as its own separately labelled, separately toned row (never
// merged into a score or percentage, per onboarding-readiness-impact.md's
// own "What to Avoid" list), or the locked "nothing to preview yet" copy
// when a stage has no evidence at all (UI-SPEC empty state). Extracted
// into its own component so this shared rendering contract stays
// independently testable from any one stage's own live-data derivation.
import Chip, { type ChipTone } from "../../../components/primitives/Chip/Chip";
import type { GuideEvidenceItem, GuideEvidenceTone } from "./stages";
import styles from "./GuidedFirstShow.module.css";

// TONE_CHIP maps the locked three-way readiness vocabulary onto the
// existing six-colour status system (09-UI-SPEC.md's Status Vocabulary
// table): blocker -> revoked, warning -> armed, evidence -> frame-lock.
// No new status colours are introduced.
const TONE_CHIP: Record<GuideEvidenceTone, ChipTone> = {
  blocker: "revoked",
  warning: "armed",
  evidence: "frame-lock",
};

// TONE_WORD is the literal status word every row's Chip carries (locked
// copy, never a percentage/score).
const TONE_WORD: Record<GuideEvidenceTone, string> = {
  blocker: "Blocker",
  warning: "Warning",
  evidence: "Evidence",
};

interface GuideEvidenceListProps {
  items: GuideEvidenceItem[];
}

export default function GuideEvidenceList({ items }: GuideEvidenceListProps) {
  if (items.length === 0) {
    return (
      <p className={styles.emptyPreview}>
        Nothing to preview yet — complete this stage's action to see it here.
      </p>
    );
  }

  return (
    <ul className={styles.evidenceList} aria-label="Stage evidence">
      {items.map((item, index) => (
        <li
          key={index}
          className={
            item.tone === "warning"
              ? `${styles.evidenceRow} ${styles["impact-preview"]}`
              : styles.evidenceRow
          }
        >
          <Chip tone={TONE_CHIP[item.tone]}>{TONE_WORD[item.tone]}</Chip>
          <span className={styles.evidenceLabel}>{item.label}</span>
          <p className={styles.evidenceDetail}>{item.detail}</p>
        </li>
      ))}
    </ul>
  );
}
