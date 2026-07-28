// stages.ts is the Guided First Show's shared stage/evidence contract
// (09-03-PLAN.md, D-08/D-10/D-11): the five locked stage ids/labels
// (onboarding-readiness-impact.md -- Fixtures/Patch/Program/Assign/
// Verify, in this exact order) and the GuideStageStatus shape every stage
// component reports up to the shared rail/footer/evidence aside in
// GuidedFirstShow.tsx. This plan implements Fixtures/Patch
// (stages/FixturesStage.tsx, stages/PatchStage.tsx); a later plan
// (09-04) implements Program/Assign/Verify against this same contract --
// GuideStageStatus never carries a persisted/cached flag, only the
// current render's own live-derived status (the guide reads actual
// domain readiness; it does not own duplicate state).
export type GuideStageId = "fixtures" | "patch" | "program" | "assign" | "verify";

export const GUIDE_STAGES: GuideStageId[] = ["fixtures", "patch", "program", "assign", "verify"];

export const GUIDE_STAGE_LABELS: Record<GuideStageId, string> = {
  fixtures: "Fixtures",
  patch: "Patch",
  program: "Program",
  assign: "Assign",
  verify: "Verify",
};

/** GuideEvidenceTone is the locked three-way readiness status vocabulary
 * (never a percentage/score, onboarding-readiness-impact.md's own "What
 * to Avoid" list): a blocker prevents the transition to Perform (but
 * never editing in another workspace); a warning needs acknowledgment or
 * evidence but is never treated as blocking; evidence is a met/satisfied
 * signal. */
export type GuideEvidenceTone = "blocker" | "warning" | "evidence";

/** GuideEvidenceItem is one separately labelled, separately toned
 * readiness row in a stage's evidence aside -- label is the row's short
 * title, detail is its fuller explanation. The literal status word
 * ("Blocker"/"Warning"/"Evidence") is rendered from `tone` by the shared
 * GuideEvidenceList component, not stored here. */
export interface GuideEvidenceItem {
  tone: GuideEvidenceTone;
  label: string;
  detail: string;
}

/** GuideStageStatus is what a stage component reports up via its
 * onStatusChange callback: its current evidence rows plus its one
 * dominant primary action's label (and whether it's disabled -- e.g.
 * while the stage's own initial live read is still in flight). */
export interface GuideStageStatus {
  items: GuideEvidenceItem[];
  primaryLabel: string;
  primaryDisabled?: boolean;
}
