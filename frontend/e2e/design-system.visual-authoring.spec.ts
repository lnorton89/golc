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

import { assertNoRuntimeIssues, chooseOption, expectChosenOption, expectSafetyClusterAvailable, findOverflowingControls, installHealthyBindings, waitForFonts } from "./helpers";
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

// ---------------------------------------------------------------------------
// Task 2: Fixture Library / Patch & Pools
// ---------------------------------------------------------------------------
//
// UI-SPEC's Required reference matrix lists this as one combined row
// ("Populated browse/inspect and impact-review warning/blocker state"), so
// this task captures the single workspace that demonstrates both halves in
// one bounded state: Patch & Pools' own FixturePatch.tsx already composes
// populated pool/deployment browsing with an inline "review impact before
// apply" inspect panel (ImpactReview) -- driving that panel's
// AddPoolMemberPreview to a mocked plan carrying both a warning and a
// blocker demonstrates the exact "impact-review warning/blocker truth" this
// task's <behavior> names, without inventing a second surface or a second
// screenshot pair.

const PATCH_LIBRARY_VIEW: FixtureLibraryView = {
  directory: "fixtures",
  rows: [
    {
      stableKey: "par-rgbw",
      contentHash: "hash-par-rgbw",
      manufacturer: "Chauvet",
      model: "COLORado 1 Quad Zoom",
      modes: ["RGBW"],
      modeChannelCounts: { RGBW: 4 },
      modeChannels: { RGBW: [] },
      fileName: "chauvet-colorado-1-quad-zoom.yaml",
      source: "local",
      status: "valid",
      detail: "",
    },
  ],
};

const PATCH_VIEW: PatchView = {
  pools: [
    {
      id: "pool-wash",
      name: "Wash",
      requiredCapabilities: ["intensity", "color"],
      members: [{ id: "member-wash-1", fixtureStableKey: "par-rgbw", fixtureContentHash: "hash-par-rgbw" }],
    },
  ],
  deployments: [
    {
      id: "deployment-main",
      name: "Main Rig",
      active: true,
      instances: [
        { id: "instance-1", poolId: "pool-wash", poolMemberId: "member-wash-1", mode: "RGBW", universe: 1, address: 1 },
      ],
    },
  ],
};

// PREVIEW_PLAN mirrors internal/pool/impact.go's own snake_case Impact Plan
// shape exactly (see FixturePatch.tsx's own ImpactPlan interface) --
// carrying exactly one warning and one blocker so both non-color-coded
// truths render in the same accepted baseline. plan_id's distinct hex
// suffix inside its first 12 characters (rather than a generic "preview"
// stem) is deliberate: the rendered "Impact Preview (plan …)" summary must
// prove it reflects THIS exact mocked preview, not a stale/cached one.
const PREVIEW_PLAN = {
  schema_version: 1,
  pool_id: "pool-wash",
  add: [{ fixture_stable_key: "par-rgbw", fixture_content_hash: "hash-par-rgbw", mode: "RGBW" }],
  remove: [],
  propagate: "preview",
  expected_revision: 1,
  operations: [
    {
      dependent_kind: "deployment_instance",
      dependent_ref: "Main Rig",
      dependent_id: "instance-preview-1",
      action: "add",
      pool_member_index: 1,
      pool_member_id: "member-preview-1",
      proposed_universe: 1,
      proposed_address: 5,
      status: "pending",
    },
  ],
  warnings: [
    {
      code: "POOL_CAPABILITY_APPROXIMATED",
      message:
        "This fixture approximates the pool's declared color capability using RGB mixing instead of a dedicated color wheel.",
    },
  ],
  errors: [
    {
      code: "POOL_UNIVERSE_CAPACITY_EXCEEDED",
      message: "Adding this fixture would exceed Universe 1's 512-channel capacity for Main Rig.",
    },
  ],
  plan_id: "plan-cafef00d9999",
};

const PREVIEW_PLAN_SUMMARY = `Impact Preview (plan ${PREVIEW_PLAN.plan_id.slice(0, 12)})`;

async function installPatchPoolsBindings(page: Page): Promise<void> {
  await installHealthyBindings(page);
  await page.addInitScript(
    (seed: { library: FixtureLibraryView; patch: PatchView; previewPlan: unknown }) => {
      const browserWindow = window as unknown as {
        go: { wails: Record<string, Record<string, (...args: unknown[]) => unknown>> };
      };
      const ok = (stdout = "") => ({ exitCode: 0, stdout, stderr: "" });
      browserWindow.go.wails.FixtureLibraryService.ListLocal = async () => seed.library;
      browserWindow.go.wails.FixturePatchService.ListPatch = async () => seed.patch;
      browserWindow.go.wails.FixturePatchService.AddPoolMemberPreview = async () => ok(JSON.stringify(seed.previewPlan));
    },
    { library: PATCH_LIBRARY_VIEW, patch: PATCH_VIEW, previewPlan: PREVIEW_PLAN },
  );
}

test.describe("Fixture Library / Patch & Pools", () => {
  for (const width of WIDTHS) {
    for (const theme of THEMES) {
      test(`${width}px ${theme}`, async ({ page }) => {
        await withTheme(page, theme);
        await installPatchPoolsBindings(page);
        await page.setViewportSize({ width, height: HEIGHT });

        await page.goto("/");
        await page.getByRole("button", { name: "Patch & Pools", exact: true }).click();
        await expect(page.getByRole("heading", { name: "Patch & Pools", exact: true })).toBeVisible();

        // browse: the populated pool/deployment lists render.
        await expect(page.getByLabel("Pool list")).toContainText("Wash");
        await expect(page.getByLabel("Deployment list")).toContainText("Main Rig");

        // Drive the inline "review impact before apply" inspect flow.
        await page.getByRole("button", { name: "Add Fixture" }).click();
        // Fixture is a Combobox and Fixture mode is a Select (both Base
        // UI-backed since the design-system migration) -- driven through the
        // shared helper rather than selectOption(), which only ever worked on
        // the native <select> these replaced.
        await chooseOption(page, "Fixture", "Chauvet COLORado 1 Quad Zoom");
        await chooseOption(page, "Fixture mode", "RGBW");

        // selection identity: the exact fixture/mode chosen is reflected
        // back by the form before the review is requested.
        await expectChosenOption(page, "Fixture", "Chauvet COLORado 1 Quad Zoom");
        await expectChosenOption(page, "Fixture mode", "RGBW");

        const reviewButton = page.getByRole("button", { name: "Review Impact" });
        await expect(reviewButton).toBeEnabled();
        await reviewButton.click();

        // preview freshness: the rendered summary carries this exact
        // mocked plan's id, never a stale/cached one.
        await expect(page.getByText(PREVIEW_PLAN_SUMMARY)).toBeVisible();
        await expect(page.getByText(/Main Rig.*Universe 1, Address 5/)).toBeVisible();

        // warning/blocker text/non-color semantics: both render as their
        // own literal code+message text, never inferred from color alone.
        await expect(
          page.getByText(
            "POOL_CAPABILITY_APPROXIMATED: This fixture approximates the pool's declared color capability using RGB mixing instead of a dedicated color wheel.",
          ),
        ).toBeVisible();
        await expect(
          page.getByText(
            "POOL_UNIVERSE_CAPACITY_EXCEEDED: Adding this fixture would exceed Universe 1's 512-channel capacity for Main Rig.",
          ),
        ).toBeVisible();

        // The blocker keeps Apply disabled -- a functional consequence of
        // the blocker, not merely a visual one.
        const applyButton = page.getByRole("button", { name: "Apply" });
        await expect(applyButton).toBeDisabled();

        // The impact-review panel (warning/blocker truth, Apply/Cancel) is
        // this task's own required capture content -- the panel plus the
        // pool row above it is taller than the 720px acceptance viewport
        // even scrolled from the very top, so some scroll trade-off is
        // unavoidable. Anchoring on Apply keeps the entire impact-review
        // panel (Review required chip through Apply/Cancel) in view,
        // trading off the pool's own identity row above it, which is the
        // right priority for this task's own warning/blocker emphasis.
        await applyButton.scrollIntoViewIfNeeded();

        // containment.
        const overflow = await findOverflowingControls(page);
        expect(overflow, "Patch & Pools must not overflow its own chrome or the viewport").toEqual([]);

        // safe focus: requesting the review must never silently move focus
        // onto the (blocked) Apply action -- FixturePatch.tsx re-renders
        // the impact-review panel once the mocked preview resolves, which
        // resets focus to <body> (a neutral, safe outcome) rather than
        // preserving it on "Review Impact"; either is acceptable, landing
        // on the disabled destructive action is not.
        const activeElementText = await page.evaluate(() => document.activeElement?.textContent?.trim() ?? null);
        expect(activeElementText, "focus must never land on the blocked Apply action").not.toBe("Apply");

        // safety.
        await assertNoRuntimeIssues(page);
        await expectSafetyClusterAvailable(page);

        await settleForCapture(page);
        await assertNoProtectedMaskIntersections(page, NO_MASKS);

        await expect(page).toHaveScreenshot(`fixtures-patch-${theme}-${width}.png`);
      });
    }
  }
});

// ---------------------------------------------------------------------------
// Task 3: Guided First Show
// ---------------------------------------------------------------------------
//
// The Verify stage rolls up every prior stage's own derived readiness into
// one flattened blocker/warning/evidence list (readiness.ts's
// aggregateReadiness) -- seeding one blocker (no pools yet), one warning
// (a fixture-library validation failure), and multiple evidence rows (a
// programmed scene, an operator surface, and the always-present optional
// MIDI-hardware note) produces exactly the "mixed blocker/warning/evidence
// state" this task's must_haves.truths bullet names, with Perform (the
// guide's one dominant next action) visibly disabled by the outstanding
// blocker.

const GUIDE_FIXTURE_LIBRARY_VIEW: FixtureLibraryView = {
  directory: "fixtures",
  rows: [
    {
      stableKey: "par-rgbw",
      contentHash: "hash-par-rgbw",
      manufacturer: "Chauvet",
      model: "COLORado 1 Quad Zoom",
      modes: ["RGBW"],
      modeChannelCounts: { RGBW: 4 },
      modeChannels: { RGBW: [] },
      fileName: "chauvet-colorado-1-quad-zoom.yaml",
      source: "local",
      status: "valid",
      detail: "",
    },
    {
      stableKey: "broken-fixture",
      contentHash: "hash-broken",
      manufacturer: "",
      model: "",
      modes: [],
      modeChannelCounts: {},
      modeChannels: {},
      fileName: "broken-fixture.yaml",
      source: "local",
      status: "invalid",
      detail: "Missing required intensity capability.",
    },
  ],
};

// GUIDE_PATCH_VIEW is deliberately empty (no pools yet) -- derivePatchStatus
// reports this as the guide's one blocker.
const GUIDE_PATCH_VIEW: PatchView = { pools: [], deployments: [] };

const GUIDE_PROGRAMMING_VIEW: ProgrammingView = {
  scenes: [{ name: "Opening Look", active: true, barsPerLoop: 4, layers: [] }],
  themes: [],
  presets: [],
  chases: [],
  motions: [],
  blends: [],
  instances: [],
};

const GUIDE_SURFACES: ReadonlyArray<{ name: string }> = [{ name: "Main Surface" }];

async function installGuideBindings(page: Page): Promise<void> {
  await installHealthyBindings(page);
  await page.addInitScript(
    (seed: {
      library: FixtureLibraryView;
      patch: PatchView;
      programming: ProgrammingView;
      surfaces: ReadonlyArray<{ name: string }>;
    }) => {
      const browserWindow = window as unknown as {
        go: { wails: Record<string, Record<string, (...args: unknown[]) => unknown>> };
      };
      browserWindow.go.wails.FixtureLibraryService.ListLocal = async () => seed.library;
      browserWindow.go.wails.FixturePatchService.ListPatch = async () => seed.patch;
      browserWindow.go.wails.ProgrammingService.ListProgramming = async () => seed.programming;
      browserWindow.go.wails.SurfaceService.ListSurfaces = async () => seed.surfaces;
    },
    {
      library: GUIDE_FIXTURE_LIBRARY_VIEW,
      patch: GUIDE_PATCH_VIEW,
      programming: GUIDE_PROGRAMMING_VIEW,
      surfaces: GUIDE_SURFACES,
    },
  );
}

test.describe("Guided First Show", () => {
  for (const width of WIDTHS) {
    for (const theme of THEMES) {
      test(`${width}px ${theme}`, async ({ page }) => {
        await withTheme(page, theme);
        await installGuideBindings(page);
        await page.setViewportSize({ width, height: HEIGHT });

        await page.goto("/");
        await expect(page.getByRole("heading", { name: "Overview", exact: true })).toBeVisible();

        await page.getByRole("button", { name: "Start Guide" }).click();
        await expect(page.getByRole("heading", { name: "Fixtures", exact: true })).toBeVisible();

        const verifyRailItem = page.getByRole("button", { name: "Verify", exact: true });
        await verifyRailItem.click();
        await expect(page.getByRole("heading", { name: "Verify", exact: true })).toBeVisible();

        // readiness semantics: exactly one blocker, one warning, and the
        // seeded evidence rows -- never a combined score or percentage.
        const readinessSummary = page.getByRole("list", { name: "Readiness summary" });
        await expect(readinessSummary).toContainText("1 blocker");
        await expect(readinessSummary).toContainText("1 warning");
        await expect(readinessSummary).toContainText("4 evidence items");

        // The mixed evidence aside renders every contributing row's own
        // tone word -- text/icon signal, never color alone. GuidedFirstShow
        // .module.css's own documented "safety valve" (.stageSection's
        // 460px floor competing with .evidenceAside's minmax(0, 260px))
        // squeezes this column to a narrow sliver at 900px -- the DOM
        // assertion below still holds true at every width regardless (the
        // rows exist with correct counts), and the always-visible
        // "Readiness summary" list above already satisfies this baseline's
        // "mixed blocker/warning/evidence state" requirement at 900px;
        // 1280px additionally shows the fuller tone-chip cards legibly.
        const evidenceAside = page.getByLabel("Stage evidence");
        await expect(evidenceAside.getByText("Blocker", { exact: true })).toHaveCount(1);
        await expect(evidenceAside.getByText("Warning", { exact: true })).toHaveCount(1);
        await expect(evidenceAside.getByText("Evidence", { exact: true })).toHaveCount(4);

        // next-action hierarchy: Perform is the guide's one dominant next
        // action, visibly disabled while the blocker remains; Back and
        // Exit Guide stay reachable regardless. Scoped to the "Verify"
        // stage section (aria-labelledby="current-step-title") -- "Perform"
        // bare would also match the persistent nav's own "Perform" group
        // toggle and its "About the Perform section" info tooltip.
        const stageSection = page.getByLabel("Verify", { exact: true });
        await expect(stageSection.getByRole("button", { name: "Perform" })).toBeDisabled();
        await expect(stageSection.getByRole("button", { name: "Back" })).toBeEnabled();
        await expect(stageSection.getByRole("button", { name: "Exit Guide" })).toBeEnabled();

        // navigation/exit: the rail marks Verify as the current step, and
        // exiting the guide is always available and never disabled by an
        // outstanding blocker (only Perform is ever gated).
        await expect(verifyRailItem).toHaveAttribute("aria-current", "step");

        // 8px grid (D-03: Phase 13 supersedes the inherited 7px gap): both
        // the rail/content-area grid and the stage footer's button row use
        // the standard 8px spacing token.
        const railParentGap = await page.evaluate(() => {
          const nav = document.querySelector('nav[aria-label="First show steps"]');
          const parent = nav?.parentElement;
          return parent ? window.getComputedStyle(parent).gap : null;
        });
        expect(railParentGap, "the rail/content-area grid must use the 8px D-03 gap").toBe("8px");

        const footerGap = await page
          .getByRole("button", { name: "Exit Guide" })
          .locator("xpath=..")
          .evaluate((el) => window.getComputedStyle(el).gap);
        expect(footerGap, "the stage footer's button row must use the 8px spacing token").toBe("8px");

        // containment.
        const overflow = await findOverflowingControls(page);
        expect(overflow, "Guided First Show must not overflow its own chrome or the viewport").toEqual([]);

        // safety: the guide overlay replaces only the workspace canvas --
        // the persistent safety cluster and runtime-health guarantees stay
        // intact underneath it.
        await assertNoRuntimeIssues(page);
        await expectSafetyClusterAvailable(page);

        await settleForCapture(page);
        await assertNoProtectedMaskIntersections(page, NO_MASKS);

        await expect(page).toHaveScreenshot(`guided-first-show-${theme}-${width}.png`);
      });
    }
  }
});
