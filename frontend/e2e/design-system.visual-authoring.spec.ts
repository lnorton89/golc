// design-system.visual-authoring.spec.ts (Plan 13-33, D-01/D-02/D-03/D-06/
// D-10/D-12/D-14, UI-SPEC-VISUAL-MATRIX): accepts the first-ever canonical
// baseline screenshots for three more of UI-SPEC's "Required reference
// matrix" surfaces -- Scenes & Looks, the combined Fixture Library / Patch
// & Pools surface, and Guided First Show -- at both required regression
// viewports (900/1280) and both themes (light/dark), twelve baselines
// total (three surfaces x two themes x two widths). There is no
// pre-existing baseline to diff against; this spec's job is to seed each
// surface's own populated/warning/blocker authoring state deterministically,
// assert the semantic/safety contract that must hold true before any pixel
// comparison is trusted, and commit the resulting PNGs as the accepted
// ground truth going forward.
//
// Every capture inherits playwright.config.ts's single calibrated
// tolerance (Plan 13-17: maxDiffPixelRatio 0, screenshot.css,
// animations-disabled, caret-hide, scale-css) by calling
// expect(page).toHaveScreenshot(name) with no per-call options. Mirrors
// design-system.visual-shell.spec.ts's (Plan 13-32) withTheme/
// settleForCapture/mask-intersection conventions rather than inventing a
// second pattern.
import { test, expect, type Page } from "@playwright/test";

import {
  assertNoRuntimeIssues,
  expectSafetyClusterAvailable,
  findOverflowingControls,
  installHealthyBindings,
  waitForFonts,
} from "./helpers";
import type {
  FixtureLibraryView,
  PatchView,
  ProgrammingView,
} from "../src/lib/wailsBridge";

type Theme = "light" | "dark";
const THEMES: Theme[] = ["light", "dark"];
const WIDTHS = [900, 1280] as const;
const HEIGHT = 720;

// theme.ts's own STORAGE_KEY ("golc-theme") -- seeded before navigation so
// main.tsx's applyTheme(getStoredTheme()) boots directly into the requested
// face. Identical convention to design-system.visual-shell.spec.ts.
async function withTheme(page: Page, theme: Theme): Promise<void> {
  await page.addInitScript((seededTheme: string) => {
    window.localStorage.setItem("golc-theme", seededTheme);
  }, theme);
}

// settleForCapture is the shared "freeze remaining nondeterminism" step
// every bounded state performs immediately before its screenshot -- fonts
// loaded, reduced motion, and the same fixed 250ms motion-settle wait every
// other design-system visual spec in this repo uses.
async function settleForCapture(page: Page): Promise<void> {
  await waitForFonts(page);
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.waitForTimeout(250);
}

// ---------------------------------------------------------------------------
// Mask-intersection safety net (identical to design-system.visual-shell.spec.ts)
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
// Task 1: Scenes & Looks
// ---------------------------------------------------------------------------
//
// A single active scene carries all four fixed layers (base_look/
// color_theme/chase/motion) enabled and pointed at a real look, and its own
// name is deliberately long (UI-SPEC's long-text reflow backstop) -- this
// lets one selected scene simultaneously demonstrate "populated Scene
// Stack," "exactly four layers," "inspector" (LookBrowser's populated
// Looks/Blend Presets panels), "timeline" (BarTimelinePanel), and "long
// scene name" from this task's own must_haves.truths bullet.

const LONG_SCENE_NAME =
  "Main Set, a deliberately long descriptive scene name proving wrap and ellipsis rendering stay stable throughout the interface";

const SCENES_LOOKS_PROGRAMMING_VIEW: ProgrammingView = {
  scenes: [
    {
      name: LONG_SCENE_NAME,
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
  themes: [{ id: "theme-warm-wash", name: "Warm Wash" }],
  presets: [{ id: "preset-full-wash", name: "Full Wash", kind: "intensity" }],
  chases: [{ id: "chase-sweep", name: "Sweep", stepUnit: "bar", stepDuration: 1 }],
  motions: [{ id: "motion-drift", name: "Drift" }],
  blends: [],
  instances: [],
};

// INSPECTOR_WIDTH_PX seeds useResizablePanel's own "golc.inspectorWidth"
// localStorage key (AppShell.tsx: min 220 / default 258 / max 480) to a
// slightly wider-than-default, still entirely user-reachable value, mostly
// as headroom for the "Looks" panel's count-summary line
// ("1 theme, 1 chase, 1 motion preset, 1 base-look preset") to read
// comfortably on one line. The actual root cause of LookBrowser rows
// bleeding past the inspector column at the 258px default -- .lookRow and
// its unstyled name span both lacking min-width: 0 -- is fixed directly in
// LookBrowser.module.css/.tsx (13-33 Task 1), not papered over here.
const INSPECTOR_WIDTH_PX = 360;

async function installScenesLooksBindings(page: Page): Promise<void> {
  await installHealthyBindings(page);
  await page.addInitScript((seed: ProgrammingView) => {
    const browserWindow = window as unknown as {
      go: { wails: Record<string, Record<string, (...args: unknown[]) => unknown>> };
    };
    browserWindow.go.wails.ProgrammingService.ListProgramming = async () => seed;
  }, SCENES_LOOKS_PROGRAMMING_VIEW);
  await page.addInitScript((widthPx: number) => {
    window.localStorage.setItem("golc.inspectorWidth", String(widthPx));
  }, INSPECTOR_WIDTH_PX);
}

test.describe("Scenes & Looks", () => {
  for (const width of WIDTHS) {
    for (const theme of THEMES) {
      test(`${width}px ${theme}`, async ({ page }) => {
        await withTheme(page, theme);
        await installScenesLooksBindings(page);
        await page.setViewportSize({ width, height: HEIGHT });

        await page.goto("/");
        const navButton = page.getByRole("button", { name: "Scenes & Looks", exact: true });
        await navButton.click();
        await expect(page.getByRole("heading", { name: "Scenes & Looks", exact: true })).toBeVisible();

        // AppShell.module.css's own .appShell transitions grid-template-
        // columns over --ds-motion-settle (200ms) whenever the contextual
        // inspector's hasContent flag flips true (ContextualInspector.tsx's
        // MutationObserver, itself async) -- this transition has no
        // prefers-reduced-motion override, so emulateMedia alone does not
        // skip it. Waiting for the aside to actually reach the seeded
        // INSPECTOR_WIDTH_PX (rather than a fixed timeout guess) avoids
        // capturing mid-transition, which otherwise intermittently leaves
        // LookBrowser's rows a real but transient few tens of pixels
        // narrower than their settled width. Only applicable above the
        // compact-width breakpoint (max-width: 1100px), where .inspector
        // is display: none by design and never reaches any width.
        if (width > 1100) {
          await page.waitForFunction(
            (expectedWidth) => {
              const aside = document.querySelector('aside[aria-label="Details"]');
              return aside !== null && aside.getBoundingClientRect().width >= expectedWidth - 5;
            },
            INSPECTOR_WIDTH_PX,
          );
        }

        // roles: the Scene Stack readout and the selectable scene list both
        // render with their documented accessible names.
        await expect(page.getByLabel("Scene stack")).toBeVisible();
        await expect(page.getByRole("list", { name: "Scene list" })).toBeVisible();

        // layer count: the selected (active, long-named) scene renders
        // exactly its four fixed layers.
        const layerList = page.getByLabel(`${LONG_SCENE_NAME} layers`);
        await expect(layerList).toBeVisible();
        await expect(layerList.locator("li")).toHaveCount(4);

        // selection: the long-named scene is both the active (LIVE) row and
        // the currently selected row -- signaled by aria-pressed/data-state,
        // never color alone.
        const selectedRow = page.getByRole("button", { name: `${LONG_SCENE_NAME} LIVE` });
        await expect(selectedRow).toHaveAttribute("aria-pressed", "true");
        await expect(selectedRow).toHaveAttribute("data-state", "selected");

        // inspector: LookBrowser's populated Looks panel (published into
        // the shell's contextual inspector) reflects the exact seeded
        // theme/chase/motion/preset counts. AppShell.module.css's own
        // documented compact-width contract (max-width: 1100px) collapses
        // the inspector column entirely below that breakpoint -- "a full
        // drawer-overlay treatment is deferred" is the shell's own stated
        // design, not a bug this task's baselines should paper over -- so
        // this assertion only applies at the 1280px regression width.
        if (width > 1100) {
          await expect(page.getByText("1 theme, 1 chase, 1 motion preset, 1 base-look preset")).toBeVisible();
        }

        // timing projection: the bottom evaluation panel names the exact
        // selected scene.
        await expect(page.getByLabel("Bar timeline preview")).toBeVisible();
        await expect(page.getByText(`Evaluate — ${LONG_SCENE_NAME}`)).toBeVisible();

        // containment.
        const overflow = await findOverflowingControls(page);
        expect(overflow, "Scenes & Looks must not overflow its own chrome or the viewport").toEqual([]);

        // focus: navigating here leaves focus on the nav item just
        // activated, never an unexpected element.
        await expect(navButton).toBeFocused();

        // safety.
        await assertNoRuntimeIssues(page);
        await expectSafetyClusterAvailable(page);

        await settleForCapture(page);
        await assertNoProtectedMaskIntersections(page, NO_MASKS);

        await expect(page).toHaveScreenshot(`scenes-looks-${theme}-${width}.png`);
      });
    }
  }
});
