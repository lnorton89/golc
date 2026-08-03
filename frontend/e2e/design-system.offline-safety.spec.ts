// design-system.offline-safety.spec.ts (Plan 13-41 Task 2, D-02/D-03/D-10/
// D-12/D-13/D-14) -- proves the approved UI-SPEC offline-safety acceptance
// clause is executable: for both named `projectedAcceptanceStates.ts`
// states (provider-offline, daemon-offline), Blackout and Revoke
// Automation stay visible, keyboard-focusable, and keyboard-operable
// through their own independent local SafetyService dispatch path,
// invoked exactly once each with zero cross-dispatch to the other safety
// command, and the projected connectivity copy never infers that
// playback/output stopped beyond what the state's own explicit Go-owned
// truth supplies.
import { test, expect, type Page } from "@playwright/test";
import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";

import { installHealthyBindings, waitForFonts } from "./helpers";
import { getBuildSha } from "./fixtures/startupProbe";
import {
  PROJECTED_ACCEPTANCE_STATES,
  installProjectedAcceptanceState,
  installSafetyDispatchSpies,
  readSafetyDispatchSpies,
  type ProjectedAcceptanceState,
} from "./fixtures/projectedAcceptanceStates";

const EVIDENCE_PATH = path.join(
  import.meta.dirname,
  "..",
  "..",
  ".planning",
  "phases",
  "13-unified-ui-design-system-and-automated-enforcement",
  "evidence",
  "offline-safety.json",
);

const VIEWPORT = { width: 1280, height: 900 } as const;
const MAX_TAB_STEPS = 40;

interface ControlResult {
  role: string;
  name: string;
  visible: boolean;
  box: { x: number; y: number; width: number; height: number } | null;
  focusableViaTab: boolean;
  tabIndexReached: number | null;
  activationMethod: string;
  dispatchCountAfter: number;
  dispatchedExactlyOnce: boolean;
  crossDispatchZero: boolean;
}

interface StateEvidence {
  id: string;
  description: string;
  unavailableDependency: string;
  before: unknown;
  after: unknown;
  projectedConnectivityCopyPresent: boolean;
  projectedConnectivityCopyExpected: boolean;
  forbiddenCopyFound: string[];
  playbackOutputTruthPreserved: boolean;
  controls: { blackout: ControlResult; revokeAutomation: ControlResult };
  assertions: Record<string, boolean>;
}

const evidenceCases: StateEvidence[] = [];

// readLiveTruthSnapshot reads exactly what LiveStatusBar itself renders
// (scene name, bar text, layers, and the raw source/output tone text) --
// the observable, operator-facing projection, not the mocked input value
// -- so "preserved truth" is proven against the real DOM, not merely the
// fixture's own input echoed back.
async function readLiveTruthSnapshot(page: Page): Promise<{
  sceneName: string | null;
  barText: string | null;
  layersText: string | null;
  sourceText: string | null;
  outputText: string | null;
}> {
  const bar = page.getByLabel("Live status bar");
  const metricValues = bar.locator('span[class*="metricValue"]');
  const count = await metricValues.count();
  const values: string[] = [];
  for (let i = 0; i < count; i += 1) {
    values.push((await metricValues.nth(i).textContent())?.trim() ?? "");
  }
  const sourceChip = bar.locator("span[title^='Source:']");
  const outputChip = bar.locator("span[title^='Output:']");
  return {
    sceneName: values[0] ?? null,
    barText: values[2] ?? null,
    layersText: (await bar.locator('span[class*="layersValue"]').textContent())?.trim() ?? null,
    sourceText: (await sourceChip.textContent())?.trim() ?? null,
    outputText: (await outputChip.textContent())?.trim() ?? null,
  };
}

async function tagSafetyControls(page: Page): Promise<void> {
  await page.evaluate(() => {
    const cluster = document.querySelector('[aria-label="Safety cluster"]');
    if (!cluster) return;
    const buttons = Array.from(cluster.querySelectorAll('button[data-safety-control="true"]'));
    const tags = ["offline-safety-blackout", "offline-safety-revoke", "offline-safety-stop-release-all"];
    buttons.forEach((button, index) => {
      if (tags[index]) button.setAttribute("data-offline-probe", tags[index]);
    });
  });
}

async function tabTraversal(page: Page): Promise<Map<string, number>> {
  await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
  const found = new Map<string, number>();
  for (let step = 0; step < MAX_TAB_STEPS; step += 1) {
    await page.keyboard.press("Tab");
    const tag = await page.evaluate(
      () => (document.activeElement as HTMLElement | null)?.getAttribute("data-offline-probe") ?? null,
    );
    if (tag && !found.has(tag)) found.set(tag, step);
  }
  return found;
}

async function operateViaKeyboard(page: Page, probeTag: string): Promise<boolean> {
  const locator = page.locator(`[data-offline-probe="${probeTag}"]`);
  await locator.focus();
  const isFocused = await locator.evaluate((el) => el === document.activeElement);
  if (!isFocused) return false;
  await page.keyboard.down("Enter");
  await page.waitForTimeout(900);
  await page.keyboard.up("Enter");
  await page.waitForTimeout(100);
  return true;
}

async function measureControl(
  page: Page,
  probeTag: string,
  role: string,
  name: string,
  tabResults: Map<string, number>,
): Promise<ControlResult> {
  const locator = page.locator(`[data-offline-probe="${probeTag}"]`);
  const visible = await locator.isVisible();
  const box = await locator.boundingBox();
  const tabIndexReached = tabResults.get(probeTag) ?? null;
  return {
    role,
    name,
    visible,
    box,
    focusableViaTab: tabIndexReached !== null,
    tabIndexReached,
    activationMethod: "keyboard-hold-enter",
    dispatchCountAfter: 0,
    dispatchedExactlyOnce: false,
    crossDispatchZero: false,
  };
}

async function runState(page: Page, state: ProjectedAcceptanceState): Promise<void> {
  await page.setViewportSize(VIEWPORT);
  await installHealthyBindings(page);
  await installProjectedAcceptanceState(page, state);
  await installSafetyDispatchSpies(page);

  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Overview", exact: true })).toBeVisible();
  await waitForFonts(page);
  await page.waitForTimeout(300);

  await tagSafetyControls(page);

  // ---------------------------------------------------------------------
  // Connectivity copy + preserved playback/output truth (before any
  // interaction).
  // ---------------------------------------------------------------------
  const before = await readLiveTruthSnapshot(page);
  const bodyText = (await page.locator("body").textContent()) ?? "";

  const projectedConnectivityCopyPresent = state.expectedConnectivityCopyPattern
    ? state.expectedConnectivityCopyPattern.test(bodyText)
    : false;
  const projectedConnectivityCopyExpected = state.expectedConnectivityCopyPattern !== null;
  const forbiddenCopyFound = state.forbiddenCopyPatterns
    .filter((pattern) => pattern.test(bodyText))
    .map((pattern) => pattern.toString());

  if (state.expectedConnectivityCopyPattern) {
    await expect(
      page.getByText(state.expectedConnectivityCopyPattern),
      `${state.id}: the connectivity copy naming "${state.unavailableDependency}" must appear`,
    ).toBeVisible();
  } else {
    // reachable === true for this state: no daemon-unreachable banner may
    // appear at all.
    await expect(page.getByText(/Can.t reach the playback engine/i)).toHaveCount(0);
  }
  expect(forbiddenCopyFound, `${state.id}: forbidden copy must never appear`).toEqual([]);

  const scenePreservedBefore = before.sceneName === state.goOwnedStatus.sceneName;
  expect(
    scenePreservedBefore,
    `${state.id}: LiveStatusBar must show the explicit Go-owned scene name ("${state.goOwnedStatus.sceneName}"), never an inferred idle placeholder`,
  ).toBe(true);

  // ---------------------------------------------------------------------
  // Keyboard Tab traversal + activation for Blackout and Revoke
  // Automation.
  // ---------------------------------------------------------------------
  const tabResults = await tabTraversal(page);
  const blackout = await measureControl(page, "offline-safety-blackout", "button", "Blackout", tabResults);
  const revoke = await measureControl(page, "offline-safety-revoke", "button", "Automation", tabResults);

  expect(blackout.visible, `${state.id}: Blackout must be visible`).toBe(true);
  expect(blackout.focusableViaTab, `${state.id}: Blackout must be reachable via Tab`).toBe(true);
  expect(revoke.visible, `${state.id}: Revoke Automation must be visible`).toBe(true);
  expect(revoke.focusableViaTab, `${state.id}: Revoke Automation must be reachable via Tab`).toBe(true);

  const blackoutOperated = await operateViaKeyboard(page, "offline-safety-blackout");
  expect(blackoutOperated, `${state.id}: Blackout must accept keyboard focus`).toBe(true);
  const spiesAfterBlackout = await readSafetyDispatchSpies(page);
  blackout.dispatchCountAfter = spiesAfterBlackout.Blackout.count;
  blackout.dispatchedExactlyOnce = spiesAfterBlackout.Blackout.count === 1;
  blackout.crossDispatchZero = spiesAfterBlackout.RevokeAutomation.count === 0 && spiesAfterBlackout.StopReleaseAll.count === 0;
  expect(blackout.dispatchedExactlyOnce, `${state.id}: Blackout must dispatch its own local path exactly once`).toBe(true);
  expect(blackout.crossDispatchZero, `${state.id}: Blackout must not trigger any other safety command`).toBe(true);

  const revokeOperated = await operateViaKeyboard(page, "offline-safety-revoke");
  expect(revokeOperated, `${state.id}: Revoke Automation must accept keyboard focus`).toBe(true);
  const spiesAfterRevoke = await readSafetyDispatchSpies(page);
  revoke.dispatchCountAfter = spiesAfterRevoke.RevokeAutomation.count;
  revoke.dispatchedExactlyOnce = spiesAfterRevoke.RevokeAutomation.count === 1;
  // Blackout's own count must stay at exactly 1 (unchanged since the
  // prior activation) -- Revoke Automation's dispatch must never also
  // re-trigger Blackout or Stop/Release-All.
  revoke.crossDispatchZero = spiesAfterRevoke.Blackout.count === 1 && spiesAfterRevoke.StopReleaseAll.count === 0;
  expect(revoke.dispatchedExactlyOnce, `${state.id}: Revoke Automation must dispatch its own local path exactly once`).toBe(true);
  expect(revoke.crossDispatchZero, `${state.id}: Revoke Automation must not trigger any other safety command`).toBe(true);

  // ---------------------------------------------------------------------
  // Playback/output truth must remain the same explicit values after both
  // dispatches -- no optimistic/inferred change in the absence of a new
  // status:update push (UI-SPEC: "UI actions dispatch shared commands and
  // wait for projected Go state before displaying authoritative live/
  // output/safety success").
  // ---------------------------------------------------------------------
  const after = await readLiveTruthSnapshot(page);
  const bodyTextAfter = (await page.locator("body").textContent()) ?? "";
  const forbiddenCopyFoundAfter = state.forbiddenCopyPatterns
    .filter((pattern) => pattern.test(bodyTextAfter))
    .map((pattern) => pattern.toString());
  expect(forbiddenCopyFoundAfter, `${state.id}: forbidden copy must never appear after dispatch either`).toEqual([]);

  const playbackOutputTruthPreserved =
    after.sceneName === before.sceneName && after.barText === before.barText && after.outputText === before.outputText;
  expect(
    playbackOutputTruthPreserved,
    `${state.id}: playback/output truth must be unchanged by a local dispatch with no new status push`,
  ).toBe(true);

  evidenceCases.push({
    id: state.id,
    description: state.description,
    unavailableDependency: state.unavailableDependency,
    before,
    after,
    projectedConnectivityCopyPresent,
    projectedConnectivityCopyExpected,
    forbiddenCopyFound,
    playbackOutputTruthPreserved,
    controls: { blackout, revokeAutomation: revoke },
    assertions: {
      connectivityCopyMatchesExpectation: projectedConnectivityCopyPresent === projectedConnectivityCopyExpected,
      noForbiddenCopyBefore: forbiddenCopyFound.length === 0,
      noForbiddenCopyAfter: forbiddenCopyFoundAfter.length === 0,
      scenePreservedBefore,
      playbackOutputTruthPreserved,
      blackoutVisible: blackout.visible,
      blackoutKeyboardReachable: blackout.focusableViaTab,
      blackoutDispatchedExactlyOnce: blackout.dispatchedExactlyOnce,
      blackoutCrossDispatchZero: blackout.crossDispatchZero,
      revokeVisible: revoke.visible,
      revokeKeyboardReachable: revoke.focusableViaTab,
      revokeDispatchedExactlyOnce: revoke.dispatchedExactlyOnce,
      revokeCrossDispatchZero: revoke.crossDispatchZero,
    },
  });
}

for (const state of PROJECTED_ACCEPTANCE_STATES) {
  test(`${state.id}: Blackout and Revoke Automation remain reachable and independently operable`, async ({ page }) => {
    await runState(page, state);
  });
}

test.afterAll(() => {
  const evidence = {
    schemaVersion: 1,
    capturedAt: new Date().toISOString(),
    buildSha: getBuildSha(),
    environment: { platform: process.platform, browser: "chromium", viewport: VIEWPORT },
    states: evidenceCases,
    allStatesPassed: evidenceCases.every((entry) =>
      Object.values(entry.assertions).every((value) => value === true),
    ),
  };

  mkdirSync(path.dirname(EVIDENCE_PATH), { recursive: true });
  writeFileSync(EVIDENCE_PATH, `${JSON.stringify(evidence, null, 2)}\n`);
});
