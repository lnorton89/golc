// design-system.visual-shell.spec.ts (Plan 13-32, D-01/D-02/D-06/D-10/D-12/
// D-13, UI-SPEC-VISUAL-MATRIX): accepts the first-ever canonical baseline
// screenshots for three of UI-SPEC's "Required reference matrix" surfaces --
// the persistent shell, the destructive dialog layer, and the shared
// state gallery -- at both required regression viewports (900/1280) and
// both themes (light/dark), twelve baselines total. There is no
// pre-existing baseline to diff against; this spec's job is to render each
// state deterministically and commit the resulting PNGs as the accepted
// ground truth going forward.
//
// Every capture inherits playwright.config.ts's single calibrated
// tolerance (Plan 13-17: maxDiffPixelRatio 0, screenshot.css,
// animations-disabled, caret-hide, scale-css) by calling
// expect(page).toHaveScreenshot(name) with no per-call options -- this spec
// only seeds deterministic state and asserts the semantic/safety contract
// that must hold true BEFORE any pixel comparison is trusted.
import { test, expect, type Page } from "@playwright/test";

import {
  assertNoRuntimeIssues,
  expectSafetyClusterAvailable,
  findOverflowingControls,
  installHealthyBindings,
  waitForFonts,
} from "./helpers";

type Theme = "light" | "dark";
const THEMES: Theme[] = ["light", "dark"];
const WIDTHS = [900, 1280] as const;
const HEIGHT = 720;

// theme.ts's own STORAGE_KEY ("golc-theme") -- seeded before navigation so
// main.tsx's applyTheme(getStoredTheme()) boots directly into the requested
// face. Mirrors design-system.expanded-copy.spec.ts's (Plan 13-31) identical
// convention rather than inventing a second theme-seeding mechanism.
async function withTheme(page: Page, theme: Theme): Promise<void> {
  await page.addInitScript((seededTheme: string) => {
    window.localStorage.setItem("golc-theme", seededTheme);
  }, theme);
}

// settleForCapture is the shared "freeze remaining nondeterminism" step
// every bounded state performs immediately before its screenshot: web fonts
// loaded, reduced motion, and the same fixed 250ms motion-settle wait
// e2e/fixtures/designSystem.ts's captureState() and helpers.ts's settle()
// both use for the identical reason -- there is no better signal to poll
// for than the geometry itself still animating.
async function settleForCapture(page: Page): Promise<void> {
  await waitForFonts(page);
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.waitForTimeout(250);
}

// ---------------------------------------------------------------------------
// Mask-intersection safety net
// ---------------------------------------------------------------------------
//
// must_haves.truths: "No mask intersects safety, navigation, live truth, or
// dialog focus." None of the three states below actually use
// expect(page).toHaveScreenshot's `mask` option: every state is built from
// the same fully deterministic, fixed Wails seed pattern Plan 13-17's
// calibration measured byte-identical across three independent repeated
// captures on this machine -- there is no known nondeterministic pixel
// region (no live clock, no random ID, no telemetry counter) in any of
// these three states to mask. PROTECTED_LANDMARK_SELECTORS and
// assertNoProtectedMaskIntersections stay wired regardless: the empty
// NO_MASKS array is itself the documented mask-rectangle set (not an
// omission), and if a future edit to this spec ever adds a mask entry, it
// is mechanically checked against every protected landmark below rather
// than merely documented as "should not overlap."
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
// Task 1: persistent shell
// ---------------------------------------------------------------------------

test.describe("persistent shell", () => {
  for (const width of WIDTHS) {
    for (const theme of THEMES) {
      test(`${width}px ${theme}`, async ({ page }) => {
        await withTheme(page, theme);
        await installHealthyBindings(page);
        await page.setViewportSize({ width, height: HEIGHT });

        await page.goto("/");
        await expect(page.getByRole("heading", { name: "Overview", exact: true })).toBeVisible();

        await settleForCapture(page);

        // focus: initial shell load must never leave an unexpected element
        // holding focus (no accidental autofocus trap on first paint).
        const activeElementTag = await page.evaluate(() => document.activeElement?.tagName ?? null);
        expect(activeElementTag, "initial shell load must not leave an unexpected element focused").toBe("BODY");

        // ARIA + semantic geometry: the shell's own persistent landmarks
        // render with the exact accessible names the rest of this suite (and
        // helpers.ts) depend on. This deliberately stops at UI-SPEC's own
        // "no control/text/overlay escapes its own box or viewport" bullet
        // (findOverflowingControls below) rather than also requiring
        // zero cross-element text overlap (helpers.ts's stricter
        // expectTopBarTextToBeReadable): that stricter check currently fails
        // at 900px on unmodified `master` too (pre-existing, reproduced via
        // `npx playwright test e2e/resize.spec.ts --grep "900x720"`,
        // unrelated to this plan's own files) -- see this phase's
        // deferred-items.md. Re-adding it here is this plan's own
        // must_haves.truths follow-up once that pre-existing gap closes.
        await expect(page.locator('[aria-label="Workspace navigation"]')).toBeVisible();
        await expect(page.locator('[aria-label="Live status bar"]')).toBeVisible();

        // reduced motion: emulateMedia above must actually be reflected by
        // the real browser before this capture is trusted.
        const prefersReducedMotion = await page.evaluate(
          () => window.matchMedia("(prefers-reduced-motion: reduce)").matches,
        );
        expect(prefersReducedMotion, "reduced motion must be active before capture").toBe(true);

        // body/workspace overflow.
        const overflow = await findOverflowingControls(page);
        expect(overflow, "the persistent shell must not overflow its own chrome or the viewport").toEqual([]);

        // safety.
        await assertNoRuntimeIssues(page);
        await expectSafetyClusterAvailable(page);

        await assertNoProtectedMaskIntersections(page, NO_MASKS);

        await expect(page).toHaveScreenshot(`persistent-shell-${theme}-${width}.png`);
      });
    }
  }
});

// ---------------------------------------------------------------------------
// Task 2: destructive dialog matrix
// ---------------------------------------------------------------------------
//
// DialogFeasibility.tsx's "blocked dialog" (title "Discard fixture
// changes?") is the only ConfirmDialog actually mounted anywhere reachable
// in the app outside its own component/unit tests (reached via the existing
// ?e2e=dialog-feasibility route, Plan 13-13's packaged-proof seam) -- this
// spec reuses that exact route/fixture rather than inventing a second one.
// dialog-feasibility.spec.ts already proves this specific ConfirmDialog
// instance is deliberately Escape/backdrop-immune (closeOnEscape={false},
// closeOnBackdrop={false} -- "This fixture keeps its dismissal policy
// explicit."): for a destructive confirmation, requiring an explicit button
// click rather than a stray Escape keypress is itself the safety contract
// this matrix captures, not a gap in it.

test.describe("dialog layer", () => {
  for (const width of WIDTHS) {
    for (const theme of THEMES) {
      test(`${width}px ${theme}`, async ({ page }) => {
        await withTheme(page, theme);
        await installHealthyBindings(page);
        await page.setViewportSize({ width, height: HEIGHT });

        await page.goto("/?e2e=dialog-feasibility");
        const trigger = page.getByRole("button", { name: "Open blocked dialog" });
        await expect(page.getByRole("button", { name: "Open allowed dialog" })).toBeFocused();
        await trigger.click();

        const dialog = page.getByRole("dialog", { name: "Discard fixture changes?" });
        await expect(dialog).toBeVisible();
        await expect(page.getByText("This fixture keeps its dismissal policy explicit.")).toBeVisible();

        // direct focus: the safe (cancel) action holds initial focus, never
        // the destructive one.
        const cancelAction = page.getByRole("button", { name: "Keep fixture" });
        await expect(cancelAction).toBeFocused();

        // Escape: this destructive confirmation's explicit-dismissal-only
        // policy means a stray Escape must never silently dismiss it --
        // containment holds even against an accidental keypress.
        await page.keyboard.press("Escape");
        await expect(dialog).toBeVisible();
        await expect(cancelAction).toBeFocused();

        // containment: the dialog itself stays fully inside the viewport at
        // every required width.
        const dialogBox = await dialog.boundingBox();
        const viewportSize = page.viewportSize();
        const containedInViewport =
          dialogBox !== null &&
          viewportSize !== null &&
          dialogBox.x >= -2 &&
          dialogBox.y >= -2 &&
          dialogBox.x + dialogBox.width <= viewportSize.width + 2 &&
          dialogBox.y + dialogBox.height <= viewportSize.height + 2;
        expect(containedInViewport, "the dialog must stay fully inside the viewport at every width/theme").toBe(true);

        await expectSafetyClusterAvailable(page);
        await assertNoRuntimeIssues(page);
        const overflow = await findOverflowingControls(page);
        expect(overflow, "the dialog layer must not overflow the viewport").toEqual([]);

        await settleForCapture(page);
        await assertNoProtectedMaskIntersections(page, NO_MASKS);

        await expect(page).toHaveScreenshot(`dialog-layer-${theme}-${width}.png`);

        // return-focus: only an explicit action closes the dialog and
        // returns focus to its own trigger -- proven after the capture above
        // so it never perturbs the accepted baseline pixel state.
        await cancelAction.click();
        await expect(dialog).toHaveCount(0);
        await expect(trigger).toBeFocused();
      });
    }
  }
});

// ---------------------------------------------------------------------------
// Task 3: shared state gallery matrix
// ---------------------------------------------------------------------------
//
// DesignSystemGallery.tsx (reached via the existing ?e2e=design-system-
// gallery route, Plan 13-17) already composes all six required bounded
// states in one deterministic fixture: an empty DataList, a busy DataList, an
// error DataList, a disabled list row, a selected list row, and a
// deliberately long fixture name. Every assertion below targets a role,
// accessible name, or DOM state attribute -- never a color -- per this
// task's own <action>.

test.describe("shared gallery", () => {
  for (const width of WIDTHS) {
    for (const theme of THEMES) {
      test(`${width}px ${theme}`, async ({ page }) => {
        await withTheme(page, theme);
        await installHealthyBindings(page);
        await page.setViewportSize({ width, height: HEIGHT });

        await page.goto("/?e2e=design-system-gallery");
        await expect(page.getByRole("heading", { name: "Design system gallery" })).toBeVisible();

        // empty: the one truly empty (non-busy, non-error) DataList falls
        // back to its default EmptyState heading.
        await expect(page.getByRole("heading", { name: "Nothing here yet" })).toBeVisible();

        // loading: role=status, aria-busy, named via its own accessible
        // label -- never inferred from a spinner's color.
        await expect(page.getByRole("status", { name: "Busy scenes is loading" })).toBeVisible();

        // error: role=alert with its own heading/message text.
        await expect(page.getByRole("alert")).toBeVisible();
        await expect(page.getByRole("heading", { name: "Error scenes unavailable" })).toBeVisible();
        await expect(
          page.getByText("The daemon did not answer. Your current show remains unchanged."),
        ).toBeVisible();

        // disabled: aria-disabled on the row itself, not a lower-contrast
        // color treatment.
        const disabledRow = page.locator('[aria-label="Many fixtures"] [aria-disabled="true"]');
        await expect(disabledRow).toBeVisible();
        await expect(disabledRow).toContainText("Back wash");

        // selected: a data-state="selected" attribute (this DataList has no
        // onSelect wired, so ListRow renders a plain div rather than an
        // aria-pressed button).
        const selectedRow = page.locator('[aria-label="One fixture"] [data-state="selected"]');
        await expect(selectedRow).toBeVisible();
        await expect(selectedRow).toContainText("Key light");

        // long-text: the deliberately long fixture name stays fully present
        // and reachable (UI-SPEC's long-text reflow backstop).
        await expect(
          page.getByText("Long fixture name that remains readable in a compact operating row"),
        ).toBeVisible();

        await assertNoRuntimeIssues(page);
        const overflow = await findOverflowingControls(page);
        expect(overflow, "the shared gallery must not overflow its own chrome or the viewport").toEqual([]);

        await settleForCapture(page);
        await assertNoProtectedMaskIntersections(page, NO_MASKS);

        await expect(page).toHaveScreenshot(`shared-gallery-${theme}-${width}.png`);
      });
    }
  }
});
