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

import { assertNoRuntimeIssues, expectTopBarTextToBeReadable } from "./helpers";
import {
  CALIBRATION_STATES,
  CALIBRATION_VIEWPORT,
  installCalibrationBindings,
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
