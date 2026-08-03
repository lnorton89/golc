// designSystem.ts: deterministic capture fixtures shared by the Wave 8
// screenshot-tolerance calibration spec (design-system.calibration.spec.ts)
// and, once calibrated, every Wave 9-13 canonical design-system visual spec.
// Everything a capture needs to be repeatable byte-for-byte across three
// runs on the same machine lives here in one place: a single deterministic
// Wails seed (patch/library/desk data, reused by both the "text" and
// "specialized-geometry" bounded states rather than duplicated), the five
// bounded destinations themselves, and the pairwise-diff calibration
// arithmetic. NAV_LABELS/settle/installHealthyBindings/assertNoRuntimeIssues
// all come from ../helpers -- this file never re-derives the authoritative
// destination catalog or re-implements a second mock-bridge surface.
import { expect, type Page, type TestInfo } from "@playwright/test";
import { mkdirSync, rmSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";
import path from "node:path";

import { installHealthyBindings, waitForFonts } from "../helpers";
import type { PatchView, FixtureLibraryView, DeskUniverseValuesView } from "../../src/lib/wailsBridge";

// ---------------------------------------------------------------------------
// Deterministic Wails seed
// ---------------------------------------------------------------------------

// The pool/deployment/instance names are deliberately long (UI-SPEC's
// long-text backstop: "Visual fixtures use at least one 2x-length
// label/message to prove reflow"). The same fixture is patched twice (two
// different addresses) so the "specialized-geometry" state renders more
// than one fader column, not a degenerate single-channel case.
export const CALIBRATION_PATCH_VIEW: PatchView = {
  pools: [
    {
      id: "pool-cal-wash",
      name: "Calibration Wash — a deliberately long pool name so wrap and ellipsis rendering stay part of every capture",
      members: [{ id: "member-cal-1", fixtureStableKey: "calibration-rgbw-par", fixtureContentHash: "hash-cal-1" }],
    },
  ],
  deployments: [
    {
      id: "deployment-cal-main",
      name: "Calibration Rig",
      active: true,
      instances: [
        { id: "instance-cal-1", poolId: "pool-cal-wash", poolMemberId: "member-cal-1", mode: "RGBW", universe: 1, address: 1 },
        { id: "instance-cal-2", poolId: "pool-cal-wash", poolMemberId: "member-cal-1", mode: "RGBW", universe: 1, address: 5 },
      ],
    },
  ],
};

export const CALIBRATION_LIBRARY_VIEW: FixtureLibraryView = {
  directory: "fixtures",
  rows: [
    {
      stableKey: "calibration-rgbw-par",
      contentHash: "hash-cal-1",
      manufacturer: "Calibration Fixtures Manufacturing Cooperative International",
      model: "RGBW PAR Long-Name Reference Unit Mk. IV Special Edition",
      modes: ["RGBW"],
      modeChannelCounts: { RGBW: 4 },
      modeChannels: {
        RGBW: [
          { index: 0, type: "intensity", occurrence: 0 },
          { index: 1, type: "color_red", occurrence: 0 },
          { index: 2, type: "color_green", occurrence: 0 },
          { index: 3, type: "color_blue", occurrence: 0 },
        ],
      },
      fileName: "calibration-rgbw-par.yaml",
      source: "local",
      status: "valid",
      detail: "Deterministic calibration fixture used only by the Wave 8 screenshot-tolerance harness.",
    },
  ],
};

export const CALIBRATION_DESK_UNIVERSE_VALUES: DeskUniverseValuesView[] = [
  { universe: 1, values: [200, 10, 90, 5, 128, 64, 32, 16] },
];

interface CalibrationSeed {
  patch: PatchView;
  library: FixtureLibraryView;
  deskUniverseValues: DeskUniverseValuesView[];
}

// installCalibrationBindings layers the deterministic seed above onto
// installHealthyBindings' own healthy Wails surface. addInitScript calls
// run in registration order before the page's own scripts, so this second
// script can safely reference and extend window.go.wails.* -- it never
// re-declares window.go.wails from scratch.
export async function installCalibrationBindings(page: Page): Promise<void> {
  await installHealthyBindings(page);
  await page.addInitScript((seed: CalibrationSeed) => {
    const browserWindow = window as unknown as {
      go: { wails: Record<string, Record<string, (...args: unknown[]) => unknown>> };
    };
    const ok = (stdout = "") => ({ exitCode: 0, stdout, stderr: "" });
    browserWindow.go.wails.FixturePatchService.ListPatch = async () => seed.patch;
    browserWindow.go.wails.FixtureLibraryService.ListLocal = async () => seed.library;
    browserWindow.go.wails.ShowService.GetImageDataURI = async () => "";
    browserWindow.go.wails.DeskService = {
      SetAttribute: async () => ok(),
      ClearAttribute: async () => ok(),
      ClearInstance: async () => ok(),
      ClearAll: async () => ok(),
      FetchUniverseValues: async () => seed.deskUniverseValues,
    };
  }, { patch: CALIBRATION_PATCH_VIEW, library: CALIBRATION_LIBRARY_VIEW, deskUniverseValues: CALIBRATION_DESK_UNIVERSE_VALUES });
}

// ---------------------------------------------------------------------------
// Bounded capture states
// ---------------------------------------------------------------------------

export interface CalibrationState {
  name: string;
  description: string;
  goto(page: Page): Promise<void>;
}

// Five bounded, deterministic destinations covering UI-SPEC's "Required
// reference matrix" surface categories at calibration scope: persistent
// shell chrome, the design-system's own state gallery, the dialog layer,
// a populated long-text state, and Desk's specialized fader geometry.
// This is intentionally a small, fixed set for tolerance calibration --
// the full nine-surface light/dark matrix is Waves 9/12-13's job
// (13-30 through 13-34), not this plan's.
export const CALIBRATION_STATES: CalibrationState[] = [
  {
    name: "shell",
    description: "Persistent shell chrome (top bar, rail, canvas, safety cluster) on the default Overview landing destination.",
    async goto(page) {
      await page.goto("/");
      await expect(page.getByRole("heading", { name: "Overview", exact: true })).toBeVisible();
    },
  },
  {
    name: "gallery",
    description: "The design-system's own deterministic state-gallery fixture (zero/one/many, busy/error, guided flow, safety variants).",
    async goto(page) {
      await page.goto("/?e2e=design-system-gallery");
      await expect(page.getByRole("heading", { name: "Design system gallery" })).toBeVisible();
    },
  },
  {
    name: "dialog",
    description: "The Keyboard shortcuts help dialog open above the shell (dialog layer, focus-visible state).",
    async goto(page) {
      await page.goto("/");
      await expect(page.getByRole("heading", { name: "Overview", exact: true })).toBeVisible();
      await page.keyboard.press("?");
      await expect(page.getByRole("dialog", { name: "Keyboard shortcuts" })).toBeVisible();
    },
  },
  {
    name: "text",
    description: "Fixture Library populated with a deliberately long manufacturer/model name (long-text reflow backstop).",
    async goto(page) {
      await page.goto("/");
      await page.getByRole("button", { name: "Fixture Library", exact: true }).click();
      await expect(page.getByRole("heading", { name: "Fixture Library", exact: true })).toBeVisible();
    },
  },
  {
    name: "specialized-geometry",
    description: "Desk fader geometry for two patched channel groups from the deterministic calibration fixture.",
    async goto(page) {
      await page.goto("/");
      await page.getByRole("button", { name: "Desk", exact: true }).click();
      await expect(page.getByRole("heading", { name: "Desk", exact: true })).toBeVisible();
    },
  },
];

export const CALIBRATION_VIEWPORT = { width: 1280, height: 720 } as const;

// ---------------------------------------------------------------------------
// Deterministic capture
// ---------------------------------------------------------------------------

const SCREENSHOT_CSS_PATH = path.join(import.meta.dirname, "..", "screenshot.css");

// captureState opens a fresh page (never reused across captures -- a fresh
// page load is what makes each of the three repeated captures an
// independent measurement of real rendering noise, not a memoized DOM),
// seeds it, waits for fonts and any pending navigation animation to
// settle, freezes remaining nondeterminism, and returns the raw PNG bytes.
export async function captureState(page: Page, state: CalibrationState): Promise<Buffer> {
  await installCalibrationBindings(page);
  await page.setViewportSize(CALIBRATION_VIEWPORT);
  await state.goto(page);
  await waitForFonts(page);
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.addStyleTag({ path: SCREENSHOT_CSS_PATH });
  await page.waitForTimeout(250);
  return page.screenshot({ animations: "disabled", caret: "hide" });
}

export function sha256(buffer: Buffer): string {
  return createHash("sha256").update(buffer).digest("hex");
}

// ---------------------------------------------------------------------------
// Calibration arithmetic
// ---------------------------------------------------------------------------

// PIXEL_COLOR_THRESHOLD is Playwright's own documented default per-pixel
// YIQ perceptual-color threshold (0-1 scale) -- left untouched. The only
// value this harness calibrates is maxDiffPixelRatio (the *tolerance*
// UI-SPEC's Visual Verification Contract calls for), so the measurement
// stays comparable to what expect(page).toHaveScreenshot() itself will
// use once Waves 9-13 read screenshot-tolerance.json back out.
export const PIXEL_COLOR_THRESHOLD = 0.2;

// CALIBRATION_CEILING: the upper bound "a configured upper bound rejects
// excessive noise" (13-17-PLAN.md) refers to. 2% of all pixels is a
// conservative, commonly cited visual-regression ceiling wide enough to
// absorb ordinary font-hinting/sub-pixel rendering variance but narrow
// enough that a real layout regression (a misplaced panel, a missing
// token) still fails loudly. If three real captures ever need more than
// this to agree with each other, that is itself evidence of a
// nondeterminism bug in the fixture, not a threshold to raise blindly --
// see the calibration spec's own failure path.
export const CALIBRATION_CEILING = 0.02;

// CANDIDATE_MAX_DIFF_PIXEL_RATIOS: an ascending ladder of maxDiffPixelRatio
// values searched per pairwise comparison. The "smallest stable" value for
// a pair is the first (smallest) candidate at which Playwright's own
// pixelmatch-backed toMatchSnapshot comparator passes -- this is
// deliberately a real comparison against Playwright's own comparator
// (never a hand-rolled diff algorithm) so the calibrated number stays
// portable to the exact matcher Waves 9-13's canonical baselines use.
export const CANDIDATE_MAX_DIFF_PIXEL_RATIOS: number[] = [
  0, 0.0001, 0.0002, 0.0005, 0.001, 0.0015, 0.002, 0.003, 0.005, 0.0075, 0.01, 0.015, CALIBRATION_CEILING,
];

export interface PairwiseDiffResult {
  pair: [string, string];
  smallestPassingRatio: number | null;
  candidatesTried: number[];
}

// snapshotSegments builds the path segments shared by both
// testInfo.snapshotPath() (to learn where to pre-write a baseline PNG) and
// expect(buffer).toMatchSnapshot() (to compare against that same file) --
// every scratch calibration snapshot lives under one clearly ephemeral
// "calibration-scratch" root, deleted at the end of the run, and is never
// a canonical baseline (Task 2 explicitly must not accept one).
export function snapshotSegments(state: string, id: string): string[] {
  return ["calibration-scratch", state, `${id}.png`];
}

// writeBaselineSnapshot pre-writes a capture's raw bytes to the exact path
// toMatchSnapshot will later compare against, sidestepping Playwright's
// own CI-mode-dependent "auto-write on missing snapshot" behavior entirely
// -- the file always already exists by the time a real comparison runs,
// regardless of whether the CI environment variable happens to be set.
export function writeBaselineSnapshot(testInfo: TestInfo, state: string, id: string, buffer: Buffer): void {
  const target = testInfo.snapshotPath(...snapshotSegments(state, id));
  mkdirSync(path.dirname(target), { recursive: true });
  writeFileSync(target, buffer);
}

// smallestPassingRatio finds the smallest CANDIDATE_MAX_DIFF_PIXEL_RATIOS
// entry at which `actual` matches the pre-written baseline snapshot named
// (state, baselineId), using Playwright's own toMatchSnapshot comparator.
// Returns null if even the ceiling candidate fails (excessive noise).
export async function smallestPassingRatio(
  state: string,
  baselineId: string,
  actual: Buffer,
): Promise<{ ratio: number | null; candidatesTried: number[] }> {
  const candidatesTried: number[] = [];
  for (const candidate of CANDIDATE_MAX_DIFF_PIXEL_RATIOS) {
    candidatesTried.push(candidate);
    try {
      await expect(actual).toMatchSnapshot(snapshotSegments(state, baselineId), {
        threshold: PIXEL_COLOR_THRESHOLD,
        maxDiffPixelRatio: candidate,
      });
      return { ratio: candidate, candidatesTried };
    } catch {
      // Try the next, larger candidate.
    }
  }
  return { ratio: null, candidatesTried };
}

export function cleanupScratchSnapshots(testInfo: TestInfo): void {
  // A *single*-segment snapshotPath() call applies the default
  // snapshotPathTemplate's "-{projectName}-{platform}{ext}" leaf suffix to
  // that lone segment itself (matching what happens to the real "baseline-a"/
  // "baseline-b" leaf names), so it does NOT resolve to the literal
  // "calibration-scratch" directory every real write's path is actually
  // nested under -- calling with two segments and taking the parent of the
  // result mirrors the real 3-segment writes (where only the last segment
  // is suffixed) and reliably lands on the true, un-suffixed root directory
  // regardless of what the suffixed sentinel leaf itself resolves to.
  const sentinelPath = testInfo.snapshotPath("calibration-scratch", "sentinel");
  const root = path.dirname(sentinelPath);
  rmSync(root, { recursive: true, force: true });
}
