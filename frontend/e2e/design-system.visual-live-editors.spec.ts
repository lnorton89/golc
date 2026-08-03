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
  installHealthyBindings,
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

// ---------------------------------------------------------------------------
// Task 2: MIDI Mapping
// ---------------------------------------------------------------------------
//
// One control_change mapping ("Warm Wash scene") carries live pickup
// feedback (physical=62%, target=35%, not yet armed) pushed through a real
// EventsOn subscriber override (window.runtime.EventsOn is a no-op stub in
// installHealthyBindings -- overridden here with an actual pub/sub so this
// test can push a real "midi:feedback" event the same way the Go host
// would), demonstrating pickup physical position + gold target + direction
// copy. A second assigned control ("Blackout Fade scene") has Learn
// clicked against a mocked StartLearn conflict response, demonstrating the
// conflict/error state -- both render in the same accepted baseline.

async function installMidiMappingBindings(page: Page): Promise<void> {
  await installHealthyBindings(page);
  // Overrides window.runtime with a real pub/sub (installHealthyBindings'
  // own stub discards every event) and exposes window.__emit so this test
  // can push a "midi:feedback" event the same way onMidiFeedback
  // (wailsBridge.ts) expects the Go host to.
  await page.addInitScript(() => {
    const listeners: Record<string, Array<(...args: unknown[]) => void>> = {};
    const browserWindow = window as unknown as {
      runtime: { EventsOn: (eventName: string, callback: (...args: unknown[]) => void) => () => void };
      __emit: (eventName: string, ...args: unknown[]) => void;
    };
    browserWindow.runtime = {
      EventsOn: (eventName, callback) => {
        (listeners[eventName] ??= []).push(callback);
        return () => {
          listeners[eventName] = (listeners[eventName] ?? []).filter((cb) => cb !== callback);
        };
      },
    };
    browserWindow.__emit = (eventName, ...args) => {
      for (const cb of listeners[eventName] ?? []) cb(...args);
    };
  });
  await page.addInitScript(() => {
    const browserWindow = window as unknown as {
      go: { wails: Record<string, Record<string, (...args: unknown[]) => unknown>> };
    };
    const ok = (stdout = "") => ({ exitCode: 0, stdout, stderr: "" });
    browserWindow.go.wails.SurfaceService.ListSurfaces = async () => [{ id: "surface-booth", name: "Booth" }];
    browserWindow.go.wails.SurfaceService.ShowSurface = async () => ({
      controls: [
        { kind: "scene", scene: "Warm Wash", label: "Warm Wash scene", assigned: true },
        { kind: "scene", scene: "Blackout Fade", label: "Blackout Fade scene", assigned: true },
      ],
    });
    browserWindow.go.wails.MidiService.ListMappings = async () => [
      {
        id: "map-1",
        channel: 1,
        kind: "control_change",
        number: 20,
        target: { kind: "scene", scene: "Warm Wash" },
        label: "Warm Wash scene",
      },
    ];
    browserWindow.go.wails.MidiService.SetActiveSurface = async () => ok();
    browserWindow.go.wails.MidiService.StartLearn = async (_surfaceName: unknown, controlRef: unknown) => {
      const ref = controlRef as { scene?: string };
      if (ref.scene === "Blackout Fade") {
        return { exitCode: 1, stdout: "", stderr: "GOLC_MIDI_MAPPING_CONFLICT: already mapped to Blackout" };
      }
      return ok();
    };
    browserWindow.go.wails.MidiService.CancelLearn = async () => ok();
  });
}

test.describe("MIDI Mapping", () => {
  for (const width of WIDTHS) {
    for (const theme of THEMES) {
      test(`${width}px ${theme}`, async ({ page }) => {
        await withTheme(page, theme);
        await installMidiMappingBindings(page);
        await page.setViewportSize({ width, height: HEIGHT });

        await page.goto("/");
        await page.getByRole("button", { name: "MIDI Mapping", exact: true }).click();
        await expect(page.getByRole("heading", { name: "MIDI Mapping", exact: true })).toBeVisible();

        const surfaceSelect = page.getByLabel("Select operator surface for MIDI mappings");
        await surfaceSelect.selectOption("Booth");
        await expect(page.getByText("Warm Wash scene").first()).toBeVisible();

        // conflict/error state: Learn a second assigned control against a
        // mocked conflict response.
        const conflictLearnButton = page.getByLabel("Learn MIDI mapping for Blackout Fade scene");
        await conflictLearnButton.click();
        await expect(page.getByText("already mapped to Blackout")).toBeVisible();

        // focus: a conflict resolving must never leave focus trapped on a
        // removed/hidden element (mirrors Plan 13-33's identical reasoning
        // for FixturePatch.tsx's own impact-review re-render).
        const activeElementVisible = await page.evaluate(() => {
          const el = document.activeElement;
          if (!el || el === document.body) return true;
          const rect = el.getBoundingClientRect();
          return rect.width > 0 && rect.height > 0;
        });
        expect(activeElementVisible, "focus must never land on a removed/hidden element after a conflict resolves").toBe(
          true,
        );

        // Push live pickup feedback for the one control_change mapping:
        // physical=62%, target=35%, not yet armed (pickup state).
        await page.evaluate(() => {
          (window as unknown as { __emit: (eventName: string, ...args: unknown[]) => void }).__emit(
            "midi:feedback",
            {
              scope: "surface",
              surfaceName: "Booth",
              mappingId: "map-1",
              kind: "control_change",
              armed: false,
              appValue: 0.35,
              physical: 0.62,
            },
          );
        });

        // pickup arithmetic: MidiPickup's own live-region text reflects the
        // exact rounded physical/target percentages just pushed.
        await expect(page.getByText("MIDI waiting: control 62, target 35.")).toBeVisible();

        // direction copy + target geometry: the ghost/target marker's own
        // title names the exact crossing direction copy, and its inline
        // position/the fill's inline width match the pushed values exactly.
        const ghostMarker = page.locator('[title="Target: the app\'s current value the fader must cross"]');
        await expect(ghostMarker).toHaveCount(1);
        await expect(ghostMarker).toHaveAttribute("style", /left: 35%/);
        const fillEl = ghostMarker.locator("xpath=preceding-sibling::div[1]");
        await expect(fillEl).toHaveAttribute("style", /width: 62%/);

        const track = ghostMarker.locator("..");
        const trackBox = await track.boundingBox();
        const ghostBox = await ghostMarker.boundingBox();
        expect(trackBox, "the soft-takeover track must render").not.toBeNull();
        expect(ghostBox, "the ghost/target marker must render").not.toBeNull();
        if (trackBox && ghostBox) {
          expect(ghostBox.x, "the target marker must stay within its own track's left edge").toBeGreaterThanOrEqual(
            trackBox.x - 1,
          );
          expect(
            ghostBox.x + ghostBox.width,
            "the target marker must stay within its own track's right edge",
          ).toBeLessThanOrEqual(trackBox.x + trackBox.width + 1);
        }

        // no support claim: generic MIDI Note/CC learn never claims named
        // hardware compatibility or certification (MIDI-HW-02 remains an
        // independent per-device evidence requirement, not a UI claim).
        await expect(page.getByText(/certified|officially supported|guaranteed compatib/i)).toHaveCount(0);

        // safety.
        await assertNoRuntimeIssues(page);
        await expectSafetyClusterAvailable(page);

        // containment.
        const overflow = await findOverflowingControls(page);
        expect(overflow, "MIDI Mapping must not overflow its own chrome or the viewport").toEqual([]);

        await settleForCapture(page);
        await assertNoProtectedMaskIntersections(page, NO_MASKS);

        await expect(page).toHaveScreenshot(`midi-mapping-${theme}-${width}.png`);
      });
    }
  }
});
