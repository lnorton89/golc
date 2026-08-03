// design-system.calibration.spec.ts is Plan 13-17's screenshot-tolerance
// calibration harness: it must enumerate deterministically (Task 1) before
// any three-capture pairwise-diff calibration runs against it (Task 2).
//
// Task 1 asserts that every bounded calibration state in
// ./fixtures/designSystem.ts is independently, deterministically reachable
// -- no baseline is captured or accepted here. Task 2 (below) is the actual
// calibration: it captures each state three times, computes every pairwise
// diff via Playwright's own toMatchSnapshot comparator, and writes both the
// machine-readable evidence and the single selected global tolerance.
import { test, expect } from "@playwright/test";
import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";

import { assertNoRuntimeIssues, expectTopBarTextToBeReadable } from "./helpers";
import {
  CALIBRATION_CEILING,
  CALIBRATION_STATES,
  CALIBRATION_VIEWPORT,
  PIXEL_COLOR_THRESHOLD,
  captureState,
  cleanupScratchSnapshots,
  installCalibrationBindings,
  sha256,
  smallestPassingRatio,
  writeBaselineSnapshot,
  type PairwiseDiffResult,
} from "./fixtures/designSystem";

test.describe("bounded calibration states enumerate deterministically", () => {
  for (const state of CALIBRATION_STATES) {
    test(`${state.name} reaches its bounded destination deterministically`, async ({ page }) => {
      await installCalibrationBindings(page);
      await page.setViewportSize(CALIBRATION_VIEWPORT);
      await state.goto(page);

      // "shell", "dialog", and "text" states render the persistent top bar
      // in view; "gallery" mounts DesignSystemGallery standalone (no top
      // bar at all, by design -- a proof seam, not a workspace), and
      // "specialized-geometry" (Desk) already gets its own runtime-issue
      // assertion below. Only assert top-bar readability where a top bar
      // actually exists.
      if (state.name !== "gallery") {
        await expectTopBarTextToBeReadable(page);
        await assertNoRuntimeIssues(page);
      }
    });
  }

  test("every bounded state name is unique", () => {
    const names = CALIBRATION_STATES.map((state) => state.name);
    expect(new Set(names).size, "calibration state names must be unique").toBe(names.length);
  });
});

// ---------------------------------------------------------------------------
// Task 2: three-capture calibration
// ---------------------------------------------------------------------------

const EVIDENCE_PATH = path.join(
  import.meta.dirname,
  "..",
  "..",
  ".planning",
  "phases",
  "13-unified-ui-design-system-and-automated-enforcement",
  "evidence",
  "screenshot-calibration.json",
);
const TOLERANCE_PATH = path.join(import.meta.dirname, "..", "design-system", "screenshot-tolerance.json");

interface StateEvidence {
  name: string;
  description: string;
  captures: Array<{ id: string; sha256: string; width: number; height: number }>;
  pairwiseDiffs: PairwiseDiffResult[];
  maxRatio: number | null;
}

test(
  "captures the bounded calibration set three times and selects one global tolerance",
  async ({ browser }, testInfo) => {
    test.setTimeout(300_000);

    const context = await browser.newContext();
    const stateEvidence: StateEvidence[] = [];

    try {
      for (const state of CALIBRATION_STATES) {
        const buffers: Buffer[] = [];
        for (let index = 0; index < 3; index += 1) {
          const page = await context.newPage();
          buffers.push(await captureState(page, state));
          await page.close();
        }

        const [captureA, captureB, captureC] = buffers;
        // Baselines are written directly to disk -- never through
        // toMatchSnapshot's own CI-mode-dependent auto-write-on-missing
        // behavior (writeBaselineSnapshot's own doc comment) -- so every
        // real comparison below runs identically regardless of whether
        // the CI environment variable happens to be set.
        writeBaselineSnapshot(testInfo, state.name, "baseline-a", captureA);
        writeBaselineSnapshot(testInfo, state.name, "baseline-b", captureB);

        const pairAB = await smallestPassingRatio(state.name, "baseline-a", captureB);
        const pairAC = await smallestPassingRatio(state.name, "baseline-a", captureC);
        const pairBC = await smallestPassingRatio(state.name, "baseline-b", captureC);

        const pairwiseDiffs: PairwiseDiffResult[] = [
          { pair: ["capture-1", "capture-2"], smallestPassingRatio: pairAB.ratio, candidatesTried: pairAB.candidatesTried },
          { pair: ["capture-1", "capture-3"], smallestPassingRatio: pairAC.ratio, candidatesTried: pairAC.candidatesTried },
          { pair: ["capture-2", "capture-3"], smallestPassingRatio: pairBC.ratio, candidatesTried: pairBC.candidatesTried },
        ];

        const ratios = pairwiseDiffs.map((diff) => diff.smallestPassingRatio);
        const maxRatio = ratios.some((ratio) => ratio === null) ? null : Math.max(...(ratios as number[]));

        stateEvidence.push({
          name: state.name,
          description: state.description,
          captures: buffers.map((buffer, index) => ({
            id: `capture-${index + 1}`,
            sha256: sha256(buffer),
            width: CALIBRATION_VIEWPORT.width,
            height: CALIBRATION_VIEWPORT.height,
          })),
          pairwiseDiffs,
          maxRatio,
        });
      }
    } finally {
      cleanupScratchSnapshots(testInfo);
      await context.close();
    }

    const failedStates = stateEvidence.filter((state) => state.maxRatio === null);
    const measuredRatios = stateEvidence
      .map((state) => state.maxRatio)
      .filter((ratio): ratio is number => ratio !== null);
    // "computed" is the literal max across every pairwise measurement in
    // every state, with no invented safety margin -- must_haves.truths
    // calls for "the smallest stable value measured", not a padded guess.
    const computedSmallestStableThreshold = measuredRatios.length > 0 ? Math.max(...measuredRatios) : null;
    const selectedThreshold =
      computedSmallestStableThreshold !== null && computedSmallestStableThreshold <= CALIBRATION_CEILING
        ? computedSmallestStableThreshold
        : null;

    const evidence = {
      schemaVersion: 1,
      capturedAt: new Date().toISOString(),
      environment: {
        platform: process.platform,
        browserName: browser.browserType().name(),
        browserVersion: browser.version(),
        viewport: CALIBRATION_VIEWPORT,
      },
      ceiling: CALIBRATION_CEILING,
      pixelColorThreshold: PIXEL_COLOR_THRESHOLD,
      candidateComparator: "playwright-toMatchSnapshot",
      states: stateEvidence,
      recomputation: {
        formula:
          "computedSmallestStableThreshold = max(state.maxRatio for state in states); " +
          "state.maxRatio = max(pair.smallestPassingRatio for pair in state.pairwiseDiffs), or null if any pair never passed by the ceiling candidate; " +
          "selectedThreshold = computedSmallestStableThreshold if it is <= ceiling, else null (excessive noise, calibration fails).",
        measuredRatios,
        computedSmallestStableThreshold,
        ceiling: CALIBRATION_CEILING,
        selectedThreshold,
        exceedsCeiling: computedSmallestStableThreshold !== null && computedSmallestStableThreshold > CALIBRATION_CEILING,
      },
      selectedThreshold,
    };

    mkdirSync(path.dirname(EVIDENCE_PATH), { recursive: true });
    writeFileSync(EVIDENCE_PATH, `${JSON.stringify(evidence, null, 2)}\n`);

    expect(
      failedStates.map((state) => state.name),
      "states whose three captures never agreed even at the configured ceiling (excessive, unexplained noise -- see screenshot-calibration.json)",
    ).toEqual([]);
    expect(selectedThreshold, "a selected threshold must be computed once every state is within the ceiling").not.toBeNull();

    // Task 2 explicitly must not accept or update canonical baselines --
    // only the single reviewed tolerance value is written here.
    const tolerance = {
      schemaVersion: 1,
      maxDiffPixelRatio: selectedThreshold,
      pixelThreshold: PIXEL_COLOR_THRESHOLD,
      calibratedFrom: path.relative(path.join(import.meta.dirname, "..", ".."), EVIDENCE_PATH).replaceAll("\\", "/"),
      calibratedAt: evidence.capturedAt,
    };
    mkdirSync(path.dirname(TOLERANCE_PATH), { recursive: true });
    writeFileSync(TOLERANCE_PATH, `${JSON.stringify(tolerance, null, 2)}\n`);
  },
);
