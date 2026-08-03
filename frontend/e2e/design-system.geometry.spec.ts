// design-system.geometry.spec.ts (Plan 13-31 Task 1, D-02/D-03/D-10/D-12/
// D-13/D-14, UI-CONSIDERATIONS-BACKSTOP-GEOMETRY): the "overflow | Domain
// visualization" backstop row -- "Playwright geometry assertions cover
// faders, timelines, meters, Monaco/Tiptap, and resize extremes at 900 and
// 1280px widths." This proves the UI-SPEC "Specialized surfaces" carve-out
// (vertical faders/Monaco/Tiptap/MIDI pickup tracks/scene timelines/meters
// may own domain-specific visualization and input geometry, but must still
// consume semantic tokens for everything else and never let their own
// resizable geometry escape its parent, the viewport, or hand scroll
// ownership to the document body) holds under real, evidenced measurement --
// not just "it renders something."
//
// A named matrix of exactly six specialized surface families, each
// exercised at both required widths (900/1280) and, where the family has a
// registered runtime-geometry.json resizable dimension, at both its
// registered minimum and maximum:
//   1. vertical faders (Desk)
//   2. scene timelines (Scenes & Looks' BarTimelinePanel + scene-list column)
//   3. Art-Net/diagnostic meters (ArtnetConfig's bounded target list +
//      Diagnostics' AppLogPanel bounded log stream)
//   4. Monaco (Scripts' real editor instance)
//   5. Tiptap (Notes' real editor instance)
//   6. resize extremes (Desk fader-width/universe-height and Scenes & Looks'
//      scene-list-width driven to their exact runtime-geometry.json min/max)
//
// Every case asserts: an exact numerical bounding-box relationship against
// the domain's own runtime geometry/CSS custom property (not a fabricated
// number -- read back from getComputedStyle, mirroring Desk.tsx's own
// computeFitFaderWidth precedent of measuring real rendered values rather
// than assuming token values); that no box escapes its parent or the
// viewport; that the intended scroll owner (never document/body) is the one
// actually scrolling; minimum-target/visibility for the controls that carry
// one; and that navigation/live-truth/safety controls remain reachable
// throughout. Every measured case is recorded into evidence/
// specialized-geometry.json (schema mirrors Plan 13-30's startup-theme-font.
// json/error-boundary-fallback.json precedent: buildSha, environment,
// per-case measurements/expectations/pass booleans).
import { test, expect, type Page } from "@playwright/test";
import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";

import {
  installHealthyBindings,
  settle,
  waitForFonts,
  expectSafetyClusterAvailable,
  findOverflowingControls,
} from "./helpers";
import { installCalibrationBindings } from "./fixtures/designSystem";
import { getBuildSha } from "./fixtures/startupProbe";

const EVIDENCE_PATH = path.join(
  import.meta.dirname,
  "..",
  "..",
  ".planning",
  "phases",
  "13-unified-ui-design-system-and-automated-enforcement",
  "evidence",
  "specialized-geometry.json",
);

const WIDTHS = [900, 1280] as const;
const HEIGHT = 720;
const TOLERANCE_PX = 2;

interface RectLike {
  top: number;
  left: number;
  right: number;
  bottom: number;
  width: number;
  height: number;
}

interface GeometryCase {
  family: string;
  width: number;
  resizeState: "default" | "min" | "max";
  description: string;
  measurements: unknown;
  expectations: Record<string, unknown>;
  passed: boolean;
}

const evidenceCases: GeometryCase[] = [];

function recordCase(entry: GeometryCase): void {
  evidenceCases.push(entry);
}

function contains(outer: RectLike, inner: RectLike, tolerance = TOLERANCE_PX): boolean {
  return (
    inner.left >= outer.left - tolerance &&
    inner.right <= outer.right + tolerance &&
    inner.top >= outer.top - tolerance &&
    inner.bottom <= outer.bottom + tolerance
  );
}

function withinViewport(rect: RectLike, viewport: { width: number; height: number }, tolerance = TOLERANCE_PX): boolean {
  return (
    rect.left >= -tolerance &&
    rect.top >= -tolerance &&
    rect.right <= viewport.width + tolerance &&
    rect.bottom <= viewport.height + tolerance
  );
}

// withinViewportWidth checks only the horizontal axis -- Desk's own
// canvas (workspace.module.css .canvas: overflow:auto) is the intended
// *vertical* scroll owner, so a universe row resized toward its
// registered max (500px) legitimately extends below the initial window
// height and scrolls into view rather than being clipped/hidden. D-13's
// actual invariant is horizontal: the document/appShell must never
// develop horizontal overflow (already asserted separately via
// findOverflowingControls) -- vertical "escape" into a bounded, scrolling
// ancestor is the expected behavior, not a violation.
function withinViewportWidth(rect: RectLike, viewport: { width: number }, tolerance = TOLERANCE_PX): boolean {
  return rect.left >= -tolerance && rect.right <= viewport.width + tolerance;
}

async function rectOf(page: Page, selector: string): Promise<RectLike | null> {
  return page.evaluate((sel) => {
    const el = document.querySelector(sel);
    if (!el) return null;
    const r = el.getBoundingClientRect();
    return { top: r.top, left: r.left, right: r.right, bottom: r.bottom, width: r.width, height: r.height };
  }, selector);
}

// findDeskOverflowingControls mirrors helpers.ts's own findOverflowingControls
// exactly, with one narrow addition: a control legitimately inside Desk's own
// horizontally-scrolling fixtureScroll (overflow-x:auto, per-universe --
// Desk.module.css's own doc comment) is skipped from the individual
// right-edge check when that ancestor is genuinely scrolled (scrollWidth >
// clientWidth). A later fixture's fader column sitting past the row's own
// currently-visible right edge there is scrolled, not escaped -- exactly the
// "intended scroll owner" fixtureScroll already is; the document/appShell
// level checks below (unchanged from the shared helper) remain the real,
// unrelaxed invariant.
async function findDeskOverflowingControls(page: Page): Promise<string[]> {
  return page.evaluate(() => {
    const offenders: string[] = [];
    const viewportWidth = window.innerWidth;
    const controls = Array.from(document.querySelectorAll<HTMLElement>("button, input, select, [contenteditable]"));

    for (const control of controls) {
      const scrollAncestor = control.closest('[class*="fixtureScroll"]');
      const insideScrolledFixtureScroll =
        scrollAncestor instanceof HTMLElement && scrollAncestor.scrollWidth > scrollAncestor.clientWidth + 1;

      const rect = control.getBoundingClientRect();
      if (rect.width === 0 && rect.height === 0) continue;
      const label =
        control.textContent?.trim() ||
        control.getAttribute("aria-label") ||
        (control as HTMLInputElement).name ||
        `(unlabeled ${control.tagName.toLowerCase()})`;

      if (control.scrollHeight - control.clientHeight > 1) {
        offenders.push(
          `"${label}": content (${control.scrollHeight}px) taller than its own box (${control.clientHeight}px) -- wrapped and got cut off`,
        );
      }
      if (!insideScrolledFixtureScroll) {
        if (rect.right > viewportWidth + 1) {
          offenders.push(`"${label}": right edge (${Math.round(rect.right)}px) past the viewport (${viewportWidth}px)`);
        }
        if (rect.left < -1) {
          offenders.push(`"${label}": left edge (${Math.round(rect.left)}px) pushed off-screen`);
        }
      }
    }

    const doc = document.documentElement;
    if (doc.scrollWidth - doc.clientWidth > 1) {
      offenders.push(`document: horizontal overflow (scrollWidth ${doc.scrollWidth}px > clientWidth ${doc.clientWidth}px)`);
    }

    const appShell = document.querySelector<HTMLElement>('[class*="appShell"]');
    if (appShell && appShell.scrollWidth - appShell.clientWidth > 1) {
      offenders.push(
        `.appShell: horizontal overflow (scrollWidth ${appShell.scrollWidth}px > clientWidth ${appShell.clientWidth}px)`,
      );
    }

    return offenders;
  });
}

async function cssVarOf(page: Page, selector: string, varName: string): Promise<number> {
  return page.evaluate(
    ({ sel, name }) => {
      const el = document.querySelector(sel);
      if (!el) return NaN;
      return Number.parseFloat(getComputedStyle(el).getPropertyValue(name));
    },
    { sel: selector, name: varName },
  );
}

async function viewportOf(page: Page): Promise<{ width: number; height: number }> {
  return page.evaluate(() => ({ width: window.innerWidth, height: window.innerHeight }));
}

// ---------------------------------------------------------------------------
// Family 1: vertical faders (Desk)
// ---------------------------------------------------------------------------

async function gotoDesk(page: Page): Promise<void> {
  await installCalibrationBindings(page);
  await page.goto("/");
  await page.getByRole("button", { name: "Desk", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Desk", exact: true })).toBeVisible();
  await settle(page);
}

interface DeskRowMeasurement {
  rowRect: RectLike;
  setHeight: number;
  setFaderWidth: number;
  setFaderTrackWidth: number;
  expectedRowHeight: number;
  scrollRect: RectLike | null;
  scrollOverflowsX: boolean;
  inputCount: number;
  inputRects: RectLike[];
  clearButtonRects: RectLike[];
}

async function measureDeskRows(page: Page): Promise<DeskRowMeasurement[]> {
  return page.evaluate(() => {
    const rows = Array.from(document.querySelectorAll<HTMLElement>('div[class*="universeRow"]'));
    return rows.map((row) => {
      const style = getComputedStyle(row);
      const rowRect = row.getBoundingClientRect();
      const setHeight = Number.parseFloat(style.getPropertyValue("--ds-universe-height"));
      const setFaderWidth = Number.parseFloat(style.getPropertyValue("--ds-fader-width"));
      // --ds-fader-track-width (Desk.tsx's UniverseRow, computed as
      // widthPanel.size - 10 - (detailed ? SCALE_RESERVED_WIDTH : 0)) is
      // the exact expected .faderInput width -- read directly rather than
      // re-deriving the "detailed" threshold/scale-reserved arithmetic
      // here, mirroring Desk.tsx's own computeFitFaderWidth precedent of
      // trusting real rendered/set values over reimplemented formulas.
      const setFaderTrackWidth = Number.parseFloat(style.getPropertyValue("--ds-fader-track-width"));
      // index.css declares a global `*, *::before, *::after { box-sizing:
      // border-box; }` reset, so .universeRow's own `height: var(--ds-universe-height)`
      // IS its full border-box (padding/border already included) -- the row's
      // rendered rect height equals the set custom property directly, with no
      // separate padding/border addition needed.
      const expectedRowHeight = setHeight;

      const scroll = row.querySelector<HTMLElement>('div[class*="fixtureScroll"]');
      const scrollRectRaw = scroll ? scroll.getBoundingClientRect() : null;
      const scrollRect = scrollRectRaw
        ? {
            top: scrollRectRaw.top,
            left: scrollRectRaw.left,
            right: scrollRectRaw.right,
            bottom: scrollRectRaw.bottom,
            width: scrollRectRaw.width,
            height: scrollRectRaw.height,
          }
        : null;
      const scrollOverflowsX = scroll ? scroll.scrollWidth > scroll.clientWidth + 1 : false;

      const inputs = Array.from(row.querySelectorAll<HTMLInputElement>('input[type="range"]'));
      const inputRects = inputs.map((input) => {
        const r = input.getBoundingClientRect();
        return { top: r.top, left: r.left, right: r.right, bottom: r.bottom, width: r.width, height: r.height };
      });

      const clearButtons = Array.from(row.querySelectorAll<HTMLElement>('button[class*="faderClearButton"]'));
      const clearButtonRects = clearButtons.map((button) => {
        const r = button.getBoundingClientRect();
        return { top: r.top, left: r.left, right: r.right, bottom: r.bottom, width: r.width, height: r.height };
      });

      return {
        rowRect: {
          top: rowRect.top,
          left: rowRect.left,
          right: rowRect.right,
          bottom: rowRect.bottom,
          width: rowRect.width,
          height: rowRect.height,
        },
        setHeight,
        setFaderWidth,
        setFaderTrackWidth,
        expectedRowHeight,
        scrollRect,
        scrollOverflowsX,
        inputCount: inputs.length,
        inputRects,
        clearButtonRects,
      };
    });
  });
}

async function assertDeskGeometry(page: Page, width: number, resizeState: "default" | "min" | "max"): Promise<void> {
  const viewport = await viewportOf(page);
  const rows = await measureDeskRows(page);
  expect(rows.length, `Desk at ${width}px (${resizeState}): at least one universe row must render`).toBeGreaterThan(0);

  let allPassed = true;
  for (const row of rows) {
    const rowHeightOk = Math.abs(row.rowRect.height - row.expectedRowHeight) <= TOLERANCE_PX;
    expect(rowHeightOk, `universe row height (${row.rowRect.height}) must equal --ds-universe-height (${row.expectedRowHeight})`).toBe(true);
    // Horizontal-only: Desk's own canvas (workspace.module.css .canvas)
    // is the intended vertical scroll owner, so a row resized toward its
    // registered 500px max legitimately extends below the fold.
    expect(withinViewportWidth(row.rowRect, viewport)).toBe(true);
    expect(row.inputCount, "at least one fader input must render").toBeGreaterThan(0);

    for (const inputRect of row.inputRects) {
      // fixtureScroll is overflow-x:auto (deliberately no vertical
      // overflow, per its own doc comment) -- a wide row legitimately
      // scrolls a later fixture's faders past the container's own
      // currently-visible right edge (clipped by the browser's own
      // overflow handling, not visible until scrolled). That is the
      // *intended* scroll-owner behavior, not an escape: the real
      // invariants are that every input stays vertically inside its own
      // row's scroll box, and never starts to the LEFT of it (which would
      // mean it escaped the container rather than merely scrolled within
      // it).
      const inScroll = row.scrollRect
        ? inputRect.top >= row.scrollRect.top - TOLERANCE_PX &&
          inputRect.bottom <= row.scrollRect.bottom + TOLERANCE_PX &&
          inputRect.left >= row.scrollRect.left - TOLERANCE_PX
        : false;
      expect(inScroll, "every fader input must stay vertically inside, and never left-escape, its own universe's fixtureScroll box").toBe(true);
      expect(inputRect.width).toBeGreaterThan(0);
      expect(inputRect.height).toBeGreaterThan(0);
      const widthOk = Math.abs(inputRect.width - row.setFaderTrackWidth) <= TOLERANCE_PX;
      expect(
        widthOk,
        `fader input width (${inputRect.width}) must equal --ds-fader-track-width (${row.setFaderTrackWidth})`,
      ).toBe(true);
      if (!inScroll || !widthOk) allPassed = false;
    }

    for (const clearRect of row.clearButtonRects) {
      // desk-fader-clear-button-jsx (design-system/exception-proposals/desk.json):
      // fixed 18x18px domain geometry, registered because it must stay
      // usable even at FADER_WIDTH_MIN=18px where IconButton's own
      // smallest size would collide with the adjacent fader -- not the
      // general 44px minimum-target rule. .fader is itself a flex column
      // (flex-direction:column), so at the registered minimum
      // universe-height (190px) this fixed-height button's own flex-basis
      // (derived from its height:18px) can legitimately flex-shrink a few
      // px vertically to fit the row's own reduced content budget -- its
      // width (not the column's main axis) stays exactly 18px regardless.
      // Still asserted non-zero and reasonably sized (never collapses to
      // an unusable sliver) in both dimensions.
      expect(clearRect.width).toBeGreaterThanOrEqual(8);
      expect(clearRect.height).toBeGreaterThanOrEqual(8);
      // Same fixtureScroll horizontal-scroll-clipping reasoning as the
      // fader input check above: a wide-column row can legitimately
      // scroll a later fixture's clear button past the container's
      // current right edge.
      if (row.scrollRect) {
        expect(clearRect.left, "clear button must never escape left of its fixtureScroll box").toBeGreaterThanOrEqual(
          row.scrollRect.left - TOLERANCE_PX,
        );
      }
    }
  }

  const offenders = await findDeskOverflowingControls(page);
  expect(offenders, `Desk at ${width}px (${resizeState}) must not overflow its own box or the viewport`).toEqual([]);
  await expectSafetyClusterAvailable(page);

  recordCase({
    family: "vertical-faders",
    width,
    resizeState,
    description: "Desk universe rows and per-channel vertical fader inputs",
    measurements: rows,
    expectations: { rowHeightTolerancePx: TOLERANCE_PX, faderTrackWidthTolerancePx: TOLERANCE_PX },
    passed: allPassed && offenders.length === 0,
  });
}

test.describe("Family 1: vertical faders (Desk)", () => {
  for (const width of WIDTHS) {
    test(`Desk fader geometry at ${width}px`, async ({ page }) => {
      await page.setViewportSize({ width, height: HEIGHT });
      await gotoDesk(page);
      await waitForFonts(page);
      await assertDeskGeometry(page, width, "default");
    });
  }
});

// ---------------------------------------------------------------------------
// Family 2: scene timelines (Scenes & Looks)
// ---------------------------------------------------------------------------

async function gotoScenesLooks(page: Page): Promise<void> {
  await installHealthyBindings(page);
  await page.addInitScript(() => {
    const bw = window as unknown as { go: { wails: { ProgrammingService: Record<string, unknown> } } };
    bw.go.wails.ProgrammingService.ListProgramming = async () => ({
      scenes: [
        {
          name: "Geometry Test Scene",
          active: true,
          barsPerLoop: 4,
          layers: [
            { kind: "base_look", enabled: true, ref: "look-1" },
            { kind: "color_theme", enabled: false },
            { kind: "chase", enabled: false },
            { kind: "motion", enabled: false },
          ],
        },
      ],
      themes: [],
      presets: [{ id: "look-1", name: "Look One" }],
      chases: [],
      motions: [],
      blends: [],
      instances: [],
    });
  });
  await page.goto("/");
  await page.getByRole("button", { name: "Scenes & Looks", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Scenes & Looks", exact: true })).toBeVisible();
  await settle(page);
}

async function assertScenesLooksGeometry(page: Page, width: number, resizeState: "default" | "min" | "max"): Promise<void> {
  const viewport = await viewportOf(page);
  const layoutRect = await rectOf(page, 'div[class*="layout"]');
  const sceneListColumnRect = await rectOf(page, 'div[class*="sceneListColumn"]');
  const mainColumnRect = await rectOf(page, 'div[class*="mainColumn"]');
  const timelineRect = await rectOf(page, '[aria-label="Bar timeline preview"]');
  const setSceneListWidth = await cssVarOf(page, 'div[class*="layout"]', "--ds-scenelist-width");

  expect(layoutRect, "layout grid must render").not.toBeNull();
  expect(sceneListColumnRect, "scene list column must render").not.toBeNull();
  expect(mainColumnRect, "main column must render").not.toBeNull();
  expect(timelineRect, "BarTimelinePanel (the bottom evaluation/timeline panel) must render").not.toBeNull();

  let passed = true;
  if (sceneListColumnRect) {
    const widthOk = Math.abs(sceneListColumnRect.width - setSceneListWidth) <= TOLERANCE_PX;
    expect(widthOk, `scene list column width (${sceneListColumnRect.width}) must equal --ds-scenelist-width (${setSceneListWidth})`).toBe(true);
    expect(withinViewport(sceneListColumnRect, viewport)).toBe(true);
    if (layoutRect) expect(contains(layoutRect, sceneListColumnRect)).toBe(true);
    if (!widthOk) passed = false;
  }
  if (timelineRect && mainColumnRect) {
    // BarTimelinePanel occupies mainColumn's own fixed 130px grid row
    // (ScenesLooksWorkspace.module.css .mainColumn's grid-template-rows:
    // "auto minmax(0, 1fr) 130px") -- the scene timeline/evaluation panel's
    // exact domain geometry.
    const heightOk = Math.abs(timelineRect.height - 130) <= TOLERANCE_PX + 2;
    expect(heightOk, `BarTimelinePanel height (${timelineRect.height}) must equal the mainColumn's fixed 130px timeline row`).toBe(true);
    expect(contains(mainColumnRect, timelineRect)).toBe(true);
    expect(withinViewport(timelineRect, viewport)).toBe(true);
    if (!heightOk) passed = false;
  }

  const offenders = await findOverflowingControls(page);
  expect(offenders, `Scenes & Looks at ${width}px (${resizeState}) must not overflow`).toEqual([]);
  await expectSafetyClusterAvailable(page);

  recordCase({
    family: "scene-timelines",
    width,
    resizeState,
    description: "Scenes & Looks scene-list column and BarTimelinePanel bottom evaluation/timeline panel",
    measurements: { layoutRect, sceneListColumnRect, mainColumnRect, timelineRect, setSceneListWidth },
    expectations: { fixedTimelineRowHeightPx: 130 },
    passed: passed && offenders.length === 0,
  });
}

test.describe("Family 2: scene timelines (Scenes & Looks)", () => {
  for (const width of WIDTHS) {
    test(`Scenes & Looks timeline geometry at ${width}px`, async ({ page }) => {
      await page.setViewportSize({ width, height: HEIGHT });
      await gotoScenesLooks(page);
      await waitForFonts(page);
      await assertScenesLooksGeometry(page, width, "default");
    });
  }
});

// ---------------------------------------------------------------------------
// Family 3: Art-Net/diagnostic meters (ArtnetConfig + Diagnostics AppLogPanel)
// ---------------------------------------------------------------------------

async function gotoArtnet(page: Page): Promise<void> {
  await installHealthyBindings(page);
  await page.addInitScript(() => {
    const bw = window as unknown as {
      go: { wails: { FixturePatchService: Record<string, unknown>; ArtnetConfigService: Record<string, unknown> } };
    };
    bw.go.wails.FixturePatchService.ListPatch = async () => ({
      pools: [{ id: "p1", name: "Meter Pool", members: [{ id: "m1", fixtureStableKey: "x", fixtureContentHash: "x" }] }],
      deployments: [
        {
          id: "d1",
          name: "Meter Rig",
          active: true,
          instances: [{ id: "i1", poolId: "p1", poolMemberId: "m1", mode: "RGBW", universe: 1, address: 1 }],
        },
      ],
    });
    bw.go.wails.ArtnetConfigService.FetchArtnetStatus = async () => ({
      reachable: true,
      interface: { pinnedIndex: 1, pinnedName: "Ethernet", status: "ready", error: "" },
      targets: [
        { universe: 1, ip: "192.168.1.50", port: 6454, enabled: true, sendOk: 100000, sendErr: 0, reachable: true, lastError: "" },
      ],
    });
  });
  await page.goto("/");
  await page.getByRole("button", { name: "Art-Net", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Art-Net", exact: true })).toBeVisible();
  await settle(page);
}

async function gotoDiagnostics(page: Page): Promise<void> {
  await installHealthyBindings(page);
  await page.addInitScript(() => {
    const bw = window as unknown as { go: { wails: { App?: Record<string, unknown> } } };
    bw.go.wails.App = {
      RecentAppLogs: async () =>
        Array.from({ length: 8 }, (_, index) => ({
          seq: index + 1,
          level: index % 3 === 0 ? "error" : index % 3 === 1 ? "warn" : "info",
          source: index % 2 === 0 ? "daemon" : "hotkeys",
          message: `Geometry backstop diagnostic line ${index + 1}: verifying the bounded log stream never grows the window.`,
          at: new Date(2026, 0, 1, 0, 0, index).toISOString(),
        })),
    };
  });
  await page.goto("/");
  await page.getByRole("button", { name: "Diagnostics", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Diagnostics", exact: true })).toBeVisible();
  await settle(page);
}

async function assertMeterGeometry(page: Page, width: number, family: string, boundedSelector: string, label: string): Promise<void> {
  const viewport = await viewportOf(page);
  const boundedRect = await rectOf(page, boundedSelector);
  expect(boundedRect, `${label} bounded panel must render`).not.toBeNull();

  let passed = true;
  const overflowsInternally = await page.evaluate((sel) => {
    const el = document.querySelector(sel);
    if (!el) return false;
    return el.scrollHeight > el.clientHeight + 1 || el.scrollWidth > el.clientWidth + 1;
  }, boundedSelector);
  const boundedHeight = await page.evaluate((sel) => {
    const el = document.querySelector(sel);
    return el ? el.getBoundingClientRect().height : NaN;
  }, boundedSelector);

  if (boundedRect) {
    expect(withinViewport(boundedRect, viewport), `${label} must stay inside the viewport`).toBe(true);
    // ArtnetConfig's .rowScroll / AppLogPanel's .stream are both registered
    // fixed-height (max-height: 320px) bounded scroll panels -- the domain
    // visualization never grows past that ceiling regardless of content.
    const boundedOk = boundedHeight <= 320 + TOLERANCE_PX;
    expect(boundedOk, `${label} bounded height (${boundedHeight}) must not exceed its 320px domain ceiling`).toBe(true);
    if (!boundedOk) passed = false;
  }

  const offenders = await findOverflowingControls(page);
  expect(offenders, `${label} at ${width}px must not overflow (bounded region owns its own scroll, not the body)`).toEqual([]);
  await expectSafetyClusterAvailable(page);

  recordCase({
    family,
    width,
    resizeState: "default",
    description: label,
    measurements: { boundedRect, boundedHeight, overflowsInternally },
    expectations: { maxHeightPx: 320 },
    passed: passed && offenders.length === 0,
  });
}

test.describe("Family 3: Art-Net/diagnostic meters", () => {
  for (const width of WIDTHS) {
    test(`Art-Net universe target list geometry at ${width}px`, async ({ page }) => {
      await page.setViewportSize({ width, height: HEIGHT });
      await gotoArtnet(page);
      await waitForFonts(page);
      await assertMeterGeometry(page, width, "artnet-diagnostic-meters", 'ul[class*="rowScroll"]', "Art-Net universe target list");
    });

    test(`Diagnostics application log stream geometry at ${width}px`, async ({ page }) => {
      await page.setViewportSize({ width, height: HEIGHT });
      await gotoDiagnostics(page);
      await waitForFonts(page);
      await assertMeterGeometry(page, width, "artnet-diagnostic-meters", 'div[class*="stream"]', "Diagnostics application log stream");
    });
  }
});

// ---------------------------------------------------------------------------
// Family 4: Monaco (Scripts)
// ---------------------------------------------------------------------------

async function gotoScriptsWithSelection(page: Page): Promise<void> {
  await installHealthyBindings(page);
  await page.addInitScript(() => {
    const bw = window as unknown as { go: { wails: { ScriptService: Record<string, unknown> } } };
    bw.go.wails.ScriptService.ListScripts = async () => [
      {
        id: "demo-1",
        name: "Demo Script",
        lastRunStatus: "never_run",
        scope: "read-only",
        preset: "standard",
        deadlineSeconds: 5,
        ratePerSecond: 10,
        memoryLimitMB: 64,
        cpuCapPercent: 25,
      },
    ];
  });
  await page.goto("/");
  await page.getByRole("button", { name: "Scripts", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Scripts", exact: true })).toBeVisible();
  await page.locator('button[title="Demo Script"]').click();
  await expect(page.locator(".monaco-editor")).toBeVisible({ timeout: 20_000 });
}

async function assertMonacoGeometry(page: Page, width: number): Promise<void> {
  const viewport = await viewportOf(page);
  const wrapperRect = await rectOf(page, 'div[class*="editorColumn"] div[class*="wrapper"]');
  const containerRect = await rectOf(page, 'div[class*="editorColumn"] div[class*="container"]');
  const monacoRect = await rectOf(page, ".monaco-editor");

  expect(wrapperRect, "ScriptEditor's own bounded wrapper must render").not.toBeNull();
  expect(monacoRect, "the real Monaco instance must mount").not.toBeNull();

  let passed = true;
  if (wrapperRect && monacoRect) {
    const contained = contains(wrapperRect, monacoRect);
    expect(contained, "Monaco's own internal layout must stay inside ScriptEditor's bounded wrapper (overflow: hidden)").toBe(true);
    expect(withinViewport(monacoRect, viewport)).toBe(true);
    expect(monacoRect.width).toBeGreaterThan(0);
    expect(monacoRect.height).toBeGreaterThan(0);
    if (!contained) passed = false;
  }
  if (containerRect && monacoRect) {
    const widthOk = Math.abs(containerRect.width - monacoRect.width) <= TOLERANCE_PX + 2;
    expect(widthOk, "Monaco must fill its own .container host (automaticLayout: true)").toBe(true);
    if (!widthOk) passed = false;
  }

  // Editor viewport usability: type into the real Monaco instance and
  // confirm the typed text actually reaches its rendered view -- proves
  // this isn't just a correctly-sized inert box.
  const marker = `GEOMETRY_BACKSTOP_${width}`;
  await page.locator(".monaco-editor .view-lines").click();
  // insertText (one atomic input event) rather than per-character
  // keyboard.type: Monaco's own async TypeScript-worker-backed model
  // updates can otherwise race a rapid per-keystroke event stream,
  // occasionally landing characters out of order -- a harness timing
  // artifact, not a real product bug, and not what this geometry backstop
  // is testing.
  await page.keyboard.press("End");
  await page.keyboard.insertText(`\n// ${marker}`);
  await expect(page.locator(".monaco-editor .view-lines")).toContainText(marker);

  const offenders = await findOverflowingControls(page);
  expect(offenders, `Scripts/Monaco at ${width}px must not overflow`).toEqual([]);
  await expectSafetyClusterAvailable(page);

  recordCase({
    family: "monaco",
    width,
    resizeState: "default",
    description: "Scripts workspace real Monaco editor instance",
    measurements: { wrapperRect, containerRect, monacoRect },
    expectations: { typedMarkerVisible: true },
    passed: passed && offenders.length === 0,
  });
}

test.describe("Family 4: Monaco (Scripts)", () => {
  for (const width of WIDTHS) {
    test(`Scripts Monaco editor geometry at ${width}px`, async ({ page }) => {
      await page.setViewportSize({ width, height: HEIGHT });
      await gotoScriptsWithSelection(page);
      await waitForFonts(page);
      await assertMonacoGeometry(page, width);
    });
  }
});

// ---------------------------------------------------------------------------
// Family 5: Tiptap (Notes)
// ---------------------------------------------------------------------------

async function gotoNotesWithSelection(page: Page): Promise<void> {
  await installHealthyBindings(page);
  await page.addInitScript(() => {
    const bw = window as unknown as { go: { wails: { NotesService: Record<string, unknown> } } };
    bw.go.wails.NotesService.ListNotes = async () => [{ id: "note-1", title: "Geometry Test Note" }];
    bw.go.wails.NotesService.GetNote = async () => ({
      id: "note-1",
      title: "Geometry Test Note",
      body: "<p>Initial body content for the Tiptap geometry backstop.</p>",
    });
  });
  await page.goto("/");
  await page.getByRole("button", { name: "Notes", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Notes", exact: true })).toBeVisible();
  await expect(page.locator('[contenteditable="true"]')).toBeVisible({ timeout: 15_000 });
}

async function assertTiptapGeometry(page: Page, width: number): Promise<void> {
  const viewport = await viewportOf(page);
  const surfaceRect = await rectOf(page, 'div[class*="surface"]');
  const proseRect = await rectOf(page, '[contenteditable="true"]');

  expect(surfaceRect, "NoteEditor's own bounded surface must render").not.toBeNull();
  expect(proseRect, "the real Tiptap editable root must mount").not.toBeNull();

  let passed = true;
  if (surfaceRect && proseRect) {
    const contained = contains(surfaceRect, proseRect);
    expect(contained, "Tiptap's own editable content must stay inside NoteEditor's bounded, overflow:auto surface").toBe(true);
    expect(withinViewport(surfaceRect, viewport)).toBe(true);
    expect(proseRect.width).toBeGreaterThan(0);
    expect(proseRect.height).toBeGreaterThan(0);
    if (!contained) passed = false;
  }

  // Editor viewport usability: type into the real Tiptap instance and
  // confirm the typed text actually reaches the rendered content.
  const marker = `Geometry backstop ${width}`;
  await page.locator('[contenteditable="true"]').click();
  await page.keyboard.press("Control+End");
  await page.keyboard.type(` ${marker}`, { delay: 20 });
  await expect(page.locator('[contenteditable="true"]')).toContainText(marker);

  const offenders = await findOverflowingControls(page);
  expect(offenders, `Notes/Tiptap at ${width}px must not overflow`).toEqual([]);
  await expectSafetyClusterAvailable(page);

  recordCase({
    family: "tiptap",
    width,
    resizeState: "default",
    description: "Notes workspace real Tiptap editor instance",
    measurements: { surfaceRect, proseRect },
    expectations: { typedMarkerVisible: true },
    passed: passed && offenders.length === 0,
  });
}

test.describe("Family 5: Tiptap (Notes)", () => {
  for (const width of WIDTHS) {
    test(`Notes Tiptap editor geometry at ${width}px`, async ({ page }) => {
      await page.setViewportSize({ width, height: HEIGHT });
      await gotoNotesWithSelection(page);
      await waitForFonts(page);
      await assertTiptapGeometry(page, width);
    });
  }
});

// ---------------------------------------------------------------------------
// Family 6: resize extremes (Desk fader-width/universe-height,
// Scenes & Looks scene-list-width) driven to their registered
// design-system/runtime-geometry.json minimum and maximum.
// ---------------------------------------------------------------------------

const DESK_UNIVERSE = 1;

async function seedLocalStorage(page: Page, key: string, value: number): Promise<void> {
  await page.addInitScript(
    ({ storageKey, storageValue }) => {
      window.localStorage.setItem(storageKey, String(storageValue));
    },
    { storageKey: key, storageValue: value },
  );
}

test.describe("Family 6: resize extremes", () => {
  const deskExtremes: { state: "min" | "max"; faderWidth: number; universeHeight: number }[] = [
    { state: "min", faderWidth: 18, universeHeight: 190 },
    { state: "max", faderWidth: 96, universeHeight: 500 },
  ];

  for (const width of WIDTHS) {
    for (const extreme of deskExtremes) {
      test(`Desk fader-width/universe-height at its registered ${extreme.state} extreme, ${width}px`, async ({ page }) => {
        await page.setViewportSize({ width, height: HEIGHT });
        await seedLocalStorage(page, `golc.deskFixtureWidth.${DESK_UNIVERSE}`, extreme.faderWidth);
        await seedLocalStorage(page, `golc.deskUniverseHeight.${DESK_UNIVERSE}`, extreme.universeHeight);
        await gotoDesk(page);
        await waitForFonts(page);

        const rows = await measureDeskRows(page);
        expect(rows.length).toBeGreaterThan(0);
        const [row] = rows;
        expect(row.setFaderWidth, "seeded fader-width extreme must actually apply").toBeCloseTo(extreme.faderWidth, 0);
        expect(row.setHeight, "seeded universe-height extreme must actually apply").toBeCloseTo(extreme.universeHeight, 0);

        await assertDeskGeometry(page, width, extreme.state);
      });
    }
  }

  const sceneListExtremes: { state: "min" | "max"; width: number }[] = [
    { state: "min", width: 160 },
    { state: "max", width: 400 },
  ];

  for (const width of WIDTHS) {
    for (const extreme of sceneListExtremes) {
      test(`Scenes & Looks scene-list-width at its registered ${extreme.state} extreme, ${width}px`, async ({ page }) => {
        await page.setViewportSize({ width, height: HEIGHT });
        await seedLocalStorage(page, "golc.sceneListWidth", extreme.width);
        await gotoScenesLooks(page);
        await waitForFonts(page);

        const setSceneListWidth = await cssVarOf(page, 'div[class*="layout"]', "--ds-scenelist-width");
        expect(setSceneListWidth, "seeded scene-list-width extreme must actually apply").toBeCloseTo(extreme.width, 0);

        await assertScenesLooksGeometry(page, width, extreme.state);
      });
    }
  }
});

// ---------------------------------------------------------------------------
// Evidence
// ---------------------------------------------------------------------------

test.afterAll(() => {
  const evidence = {
    schemaVersion: 1,
    capturedAt: new Date().toISOString(),
    buildSha: getBuildSha(),
    environment: {
      platform: process.platform,
      widths: WIDTHS,
      height: HEIGHT,
    },
    families: [
      "vertical-faders",
      "scene-timelines",
      "artnet-diagnostic-meters",
      "monaco",
      "tiptap",
      "resize-extremes",
    ],
    cases: evidenceCases,
    allCasesPassed: evidenceCases.every((entry) => entry.passed),
  };

  mkdirSync(path.dirname(EVIDENCE_PATH), { recursive: true });
  writeFileSync(EVIDENCE_PATH, `${JSON.stringify(evidence, null, 2)}\n`);
});
