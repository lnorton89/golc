// design-system.visual-live-editors.spec.ts (Plan 13-34, D-01/D-02/D-03/
// D-06/D-10/D-12/D-13/D-14, UI-SPEC-VISUAL-MATRIX): accepts the first-ever
// canonical baseline screenshots for the three remaining live/editor
// surfaces of UI-SPEC's "Required reference matrix" -- Desk / Operator
// Surface, MIDI Mapping, and Scripts / Notes -- at both required
// regression viewports (900/1280) and both themes (light/dark), twelve
// baselines total (three surfaces x two themes x two widths). There is no
// pre-existing baseline to diff against; this spec's job is to seed each
// surface's own live/pickup/editor state deterministically, assert the
// semantic/safety/authority contract that must hold true before any pixel
// comparison is trusted, and commit the resulting PNGs as the accepted
// ground truth going forward.
//
// Every capture inherits playwright.config.ts's single calibrated
// tolerance (Plan 13-17: maxDiffPixelRatio 0, screenshot.css,
// animations-disabled, caret-hide, scale-css) by calling
// expect(page).toHaveScreenshot(name) with no per-call options. Mirrors
// design-system.visual-shell.spec.ts's (Plan 13-32) and
// design-system.visual-authoring.spec.ts's (Plan 13-33) withTheme/
// settleForCapture/mask-intersection conventions rather than inventing a
// second pattern.
//
// Desk / Operator Surface and Scripts / Notes are each combined UI-SPEC
// matrix rows naming two features that live on two separate, unrelated
// WorkspaceRouter destinations (OperatorSurfaceWorkspace/DeskWorkspace;
// ScriptsWorkspace/NotesWorkspace) -- no single existing navigable screen
// renders both halves of either row at once. Mirrors Plan 13-33's own
// precedent of capturing a combined row as one bounded state, extended
// with two small, e2e-only composite fixtures (DeskOperatorFixture.tsx,
// ScriptsNotesFixture.tsx, reached via App.tsx's existing ?e2e=... seam)
// that mount the REAL production components side by side -- see each
// fixture's own doc comment for the full rationale. MIDI Mapping has no
// such combined-row problem and is captured via real navigation through
// the persistent shell exactly like every other single-row surface.
import { test, expect, type Page } from "@playwright/test";

import {
  assertNoRuntimeIssues,
  expectSafetyClusterAvailable,
  findOverflowingControls,
  waitForFonts,
} from "./helpers";
import { installCalibrationBindings } from "./fixtures/designSystem";

type Theme = "light" | "dark";
const THEMES: Theme[] = ["light", "dark"];
const WIDTHS = [900, 1280] as const;
const HEIGHT = 720;

// theme.ts's own STORAGE_KEY ("golc-theme") -- seeded before navigation so
// main.tsx's applyTheme(getStoredTheme()) boots directly into the
// requested face. Identical convention to design-system.visual-shell.spec.ts
// / design-system.visual-authoring.spec.ts.
async function withTheme(page: Page, theme: Theme): Promise<void> {
  await page.addInitScript((seededTheme: string) => {
    window.localStorage.setItem("golc-theme", seededTheme);
  }, theme);
}

// settleForCapture is the shared "freeze remaining nondeterminism" step
// every bounded state performs immediately before its screenshot -- fonts
// loaded, reduced motion, and the same fixed 250ms motion-settle wait
// every other design-system visual spec in this repo uses.
async function settleForCapture(page: Page): Promise<void> {
  await waitForFonts(page);
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.waitForTimeout(250);
}

// ---------------------------------------------------------------------------
// Mask-intersection safety net (identical to the two sibling visual specs)
// ---------------------------------------------------------------------------
//
// must_haves.truths: "No mask intersects safety, navigation, live truth, or
// dialog focus." None of the three surfaces below use
// expect(page).toHaveScreenshot's `mask` option -- every state is built
// from a fully deterministic, fixed Wails seed with no live clock, random
// ID, or telemetry counter to mask. PROTECTED_LANDMARK_SELECTORS and
// assertNoProtectedMaskIntersections stay wired regardless: the empty
// NO_MASKS array is itself the documented mask-rectangle set, and any
// future mask addition to this spec is mechanically checked against every
// protected landmark below.
const PROTECTED_LANDMARK_SELECTORS: readonly string[] = [
  '[aria-label="Safety cluster"]', // safety
  '[aria-label="Workspace navigation"]', // navigation
  '[aria-label="Live status bar"]', // live truth
  '[role="dialog"]', // dialog focus
  '[role="alertdialog"]', // dialog focus (destructive variant)
];

interface MaskRegion {
  selector: string;
  reason: string;
}

const NO_MASKS: readonly MaskRegion[] = [];

async function assertNoProtectedMaskIntersections(page: Page, masks: readonly MaskRegion[]): Promise<void> {
  for (const mask of masks) {
    const offenders = await page.evaluate(
      ({ maskSelector, protectedSelectors }) => {
        const maskEl = document.querySelector(maskSelector);
        if (!maskEl) return [];
        const maskRect = maskEl.getBoundingClientRect();
        const found: string[] = [];
        for (const protectedSelector of protectedSelectors) {
          for (const protectedEl of Array.from(document.querySelectorAll(protectedSelector))) {
            const protectedRect = protectedEl.getBoundingClientRect();
            const intersects =
              maskRect.left < protectedRect.right &&
              maskRect.right > protectedRect.left &&
              maskRect.top < protectedRect.bottom &&
              maskRect.bottom > protectedRect.top;
            if (intersects) found.push(protectedSelector);
          }
        }
        return found;
      },
      { maskSelector: mask.selector, protectedSelectors: PROTECTED_LANDMARK_SELECTORS },
    );
    expect(offenders, `mask "${mask.selector}" (${mask.reason}) must never intersect a protected landmark`).toEqual(
      [],
    );
  }
}

// ---------------------------------------------------------------------------
// Task 1: Desk / Operator Surface
// ---------------------------------------------------------------------------
//
// DeskOperatorFixture.tsx (?e2e=desk-operator) mounts the real Launcher
// (operate-mode pads/masters) directly adjacent to the real Desk
// (projected fader grid), with the persistent SafetyCluster visible below
// both -- see that fixture's own doc comment for why no single existing
// navigable destination demonstrates this combined UI-SPEC row. One
// assigned, currently-live scene ("Opening Look") gives the required LIVE
// pad; one unassigned scene ("Interlude") gives the required locked pad;
// one assigned Grand Master gives the required masters section; Desk's
// own calibrated two-instance/four-channel-each seed (reused verbatim from
// e2e/fixtures/designSystem.ts's calibration fixture, Plan 13-17) gives
// the required projected faders.

async function installDeskOperatorBindings(page: Page): Promise<void> {
  await installCalibrationBindings(page);
  await page.addInitScript(() => {
    const browserWindow = window as unknown as {
      go: { wails: Record<string, Record<string, (...args: unknown[]) => unknown>> };
    };
    const ok = (stdout = "") => ({ exitCode: 0, stdout, stderr: "" });
    browserWindow.go.wails.PlaybackService.GetState = async () =>
      ok(
        JSON.stringify({
          bpm: 120,
          scenes: [
            {
              name: "Opening Look",
              active: true,
              barsPerLoop: 8,
              layers: [
                { kind: "base_look", enabled: true, ref: "preset-full-wash" },
                { kind: "color_theme", enabled: true, ref: "theme-warm-wash" },
                { kind: "chase", enabled: true, ref: "chase-sweep" },
                { kind: "motion", enabled: true, ref: "motion-drift" },
              ],
            },
            { name: "Interlude", active: false, barsPerLoop: 4, layers: [] },
          ],
        }),
      );
  });
}

test.describe("Desk / Operator Surface", () => {
  for (const width of WIDTHS) {
    for (const theme of THEMES) {
      test(`${width}px ${theme}`, async ({ page }) => {
        await withTheme(page, theme);
        await installDeskOperatorBindings(page);
        await page.setViewportSize({ width, height: HEIGHT });

        await page.goto("/?e2e=desk-operator");
        await expect(page.getByRole("heading", { name: "Desk / Operator Surface", exact: true })).toBeVisible();

        // focus: initial fixture load must never leave an unexpected
        // element holding focus.
        const activeElementTag = await page.evaluate(() => document.activeElement?.tagName ?? null);
        expect(activeElementTag, "initial load must not leave an unexpected element focused").toBe("BODY");

        // Active LIVE pad: aria-current="true" plus the non-color-coded
        // "LIVE" text tag (D-06 -- never color alone).
        const livePad = page.getByRole("button", { name: "Opening Look LIVE" });
        await expect(livePad).toHaveAttribute("aria-current", "true");
        await expect(livePad).toBeEnabled();

        // Locked pad: aria-disabled/disabled plus the non-color-coded
        // "Locked" text tag -- visible but locked (D-04), never hidden.
        const lockedPad = page.getByRole("button", { name: "Interlude Locked" });
        await expect(lockedPad).toHaveAttribute("aria-disabled", "true");
        await expect(lockedPad).toBeDisabled();

        // Masters: the assigned Grand Master renders through the shared
        // LauncherMasters pattern.
        await expect(page.getByLabel("Launcher masters")).toContainText("Grand Master");

        // Projected faders: Desk's calibrated two-instance/four-channel
        // seed renders real range-input faders.
        const faders = page.locator('input[type="range"]');
        await expect(faders.first()).toBeVisible();
        expect(await faders.count(), "Desk's calibrated seed must render at least one projected fader").toBeGreaterThan(0);

        // safety.
        await assertNoRuntimeIssues(page);
        await expectSafetyClusterAvailable(page);

        // containment.
        const overflow = await findOverflowingControls(page);
        expect(overflow, "Desk / Operator Surface must not overflow its own chrome or the viewport").toEqual([]);

        await settleForCapture(page);
        await assertNoProtectedMaskIntersections(page, NO_MASKS);

        await expect(page).toHaveScreenshot(`desk-operator-${theme}-${width}.png`);
      });
    }
  }
});
