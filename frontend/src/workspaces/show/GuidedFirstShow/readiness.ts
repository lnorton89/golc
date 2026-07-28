// readiness.ts holds the Guided First Show's pure, render-free readiness
// derivations (09-04-PLAN.md Task 2/3, FDUI-03): each derive*Status
// function takes an already-fetched domain view and returns a
// GuideStageStatus with the tone-labelled evidence rows and the fixed
// destination label the stage hands off to on its primary action. Every
// derivation and the aggregate rollup below is a pure function of its
// input -- no persisted or cached readiness flag anywhere
// (onboarding-readiness-impact.md's own interaction contract: "the guide
// reads actual domain readiness; it does not own duplicate state"). Never
// a combined score or ratio (locked design's "What to Avoid" list) --
// blockers/warnings/evidence stay three independently counted categories
// all the way up through aggregateReadiness.
import type { FixtureLibraryView, PatchView, ProgrammingView } from "../../../lib/wailsBridge";
import type { GuideEvidenceItem, GuideStageStatus } from "./stages";

// ReadinessRollup is aggregateReadiness's return shape: three independent
// counts plus every contributing item, flattened -- never a combined
// score.
export interface ReadinessRollup {
  blockers: number;
  warnings: number;
  evidence: number;
  items: GuideEvidenceItem[];
}

const FIXTURES_PRIMARY_LABEL = "Go to Fixture Library";
const PATCH_PRIMARY_LABEL = "Review Patch & Continue";
const PROGRAM_PRIMARY_LABEL = "Go to Scenes & Looks";
const ASSIGN_PRIMARY_LABEL = "Go to Operator Surface";

// deriveFixturesStatus derives the Fixtures stage's readiness -- moved
// verbatim in behaviour from FixturesStage.tsx's own former inline
// derivation (09-03-PLAN.md Task 3).
export function deriveFixturesStatus(view: FixtureLibraryView): GuideStageStatus {
  const validCount = view.rows.filter((row) => row.status === "valid").length;
  const invalidCount = view.rows.length - validCount;

  const items: GuideEvidenceItem[] = [];
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

  return { items, primaryLabel: FIXTURES_PRIMARY_LABEL, primaryDisabled: false };
}

// derivePatchStatus derives the Patch stage's readiness -- moved verbatim
// in behaviour from PatchStage.tsx's own former inline derivation
// (09-03-PLAN.md Task 3).
export function derivePatchStatus(view: PatchView): GuideStageStatus {
  const hasActiveDeployment = view.deployments.some((deployment) => deployment.active);

  const items: GuideEvidenceItem[] = [];
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

  return { items, primaryLabel: PATCH_PRIMARY_LABEL, primaryDisabled: false };
}

// deriveProgramStatus derives the Program stage's readiness: zero scenes
// is a blocker stating the show has no scene to perform yet; one or more
// is evidence naming the count with singular/plural agreement.
export function deriveProgramStatus(view: ProgrammingView): GuideStageStatus {
  const count = view.scenes.length;

  const items: GuideEvidenceItem[] = [];
  if (count === 0) {
    items.push({
      tone: "blocker",
      label: "No scene yet",
      detail: "The show has no scene to perform yet.",
    });
  } else {
    items.push({
      tone: "evidence",
      label: "Scene programmed",
      detail: `${count} scene${count === 1 ? "" : "s"} programmed and ready to perform.`,
    });
  }

  return { items, primaryLabel: PROGRAM_PRIMARY_LABEL, primaryDisabled: false };
}

// deriveAssignStatus derives the Assign stage's readiness: zero operator
// surfaces is a blocker; at least one is evidence naming the count. In
// both cases it appends exactly one optional MIDI-hardware evidence row
// -- physical MIDI hardware evidence is optional for on-screen and
// keyboard operation and is required only for a named hardware
// compatibility claim (onboarding-readiness-impact.md's own interaction
// contract) -- this row is never a blocker or a warning, regardless of
// surface count. `programming` is accepted for parity with the other
// live-derived stages; today's derivation does not need its fields.
export function deriveAssignStatus(
  surfaceCount: number,
  programming: ProgrammingView,
): GuideStageStatus {
  void programming;

  const items: GuideEvidenceItem[] = [];
  if (surfaceCount === 0) {
    items.push({
      tone: "blocker",
      label: "No operator surface yet",
      detail: "Create an operator surface before handing this show off to a player.",
    });
  } else {
    items.push({
      tone: "evidence",
      label: "Operator surface",
      detail: `${surfaceCount} operator surface${surfaceCount === 1 ? "" : "s"} ready for assignment.`,
    });
  }

  items.push({
    tone: "evidence",
    label: "MIDI hardware (optional)",
    detail:
      "Physical MIDI hardware evidence is optional for on-screen and keyboard operation -- it's required only for a named hardware compatibility claim.",
  });

  return { items, primaryLabel: ASSIGN_PRIMARY_LABEL, primaryDisabled: false };
}

// aggregateReadiness rolls up every stage's own derived status into three
// independently counted categories plus the flattened item list -- no
// combined score or ratio anywhere in this module (locked design's own
// "What to Avoid" list).
export function aggregateReadiness(statuses: GuideStageStatus[]): ReadinessRollup {
  const items = statuses.flatMap((status) => status.items);
  let blockers = 0;
  let warnings = 0;
  let evidence = 0;
  for (const item of items) {
    if (item.tone === "blocker") blockers += 1;
    else if (item.tone === "warning") warnings += 1;
    else evidence += 1;
  }
  return { blockers, warnings, evidence, items };
}
