import { expect, test, type Page } from "@playwright/test";
import { mkdir, readdir } from "node:fs/promises";
import path from "node:path";

import desktopViews from "../src/shell/desktopViews.json" with { type: "json" };
import { installHealthyBindings, assertNoRuntimeIssues, expectTopBarTextToBeReadable } from "./helpers";

const OUTPUT_ROOT = path.resolve(import.meta.dirname, "../../site/public/desktop-views");
const regularViews = desktopViews.groups.flatMap((group) => group.views);
// The Guided First Show overlay is deliberately never a nav destination
// (D-10), so its stages live in desktopViews.json's separate top-level
// "onboarding" section rather than as a fifth group -- captured here in
// its own pass while the guide is open, not through the sidebar-click
// loop below.
const guideViews = desktopViews.onboarding?.views ?? [];
const views = [...regularViews, ...guideViews];
const DOCUMENTATION_VIEWPORT = { width: 1920, height: 1080 } as const;

function outputPath(screenshot: string): string {
  if (!screenshot.startsWith("/desktop-views/") || path.posix.normalize(screenshot) !== screenshot) {
    throw new Error(`unsafe desktop-view screenshot path: ${screenshot}`);
  }
  const resolved = path.resolve(OUTPUT_ROOT, path.basename(screenshot));
  if (path.dirname(resolved) !== OUTPUT_ROOT) {
    throw new Error(`desktop-view screenshot escaped output root: ${screenshot}`);
  }
  return resolved;
}

async function settle(page: Page): Promise<void> {
  await page.addStyleTag({
    content: `
      *, *::before, *::after {
        animation: none !important;
        caret-color: transparent !important;
        transition: none !important;
      }
    `,
  });
  await page.waitForTimeout(250);
}

test("captures every catalog desktop destination", async ({ page }) => {
  test.setTimeout(90_000);
  await page.setViewportSize(DOCUMENTATION_VIEWPORT);
  await installHealthyBindings(page);
  await mkdir(OUTPUT_ROOT, { recursive: true });

  const expectedNames = views.map((view) => path.basename(view.screenshot)).sort();
  expect(new Set(expectedNames).size, "catalog screenshot paths must be unique").toBe(expectedNames.length);

  const existingNames = (await readdir(OUTPUT_ROOT)).filter((name) => name.endsWith(".png")).sort();
  const extraNames = existingNames.filter((name) => !expectedNames.includes(name));
  expect(extraNames, "remove stale desktop-view screenshots before capture").toEqual([]);

  await page.goto("/");
  const exitGuide = page.getByRole("button", { name: "Exit Guide" });

  // The mocked bindings above report a show with a pool and a deployment
  // already present, so it is not "genuinely empty" (D-08) and the guide
  // does not auto-launch; Overview's own manual "Start Guide" action
  // (D-10's deliberate entry point) is used instead. Overview is the
  // default landing destination, so this is reachable straight off "/".
  if (guideViews.length > 0) {
    await page.getByRole("button", { name: "Start Guide" }).click();
    await expect(exitGuide).toBeVisible();

    for (const view of guideViews) {
      await page.getByRole("button", { name: view.navLabel, exact: true }).click();
      await expect(page.getByRole("heading", { name: view.navLabel, exact: true })).toBeVisible();
      await settle(page);
      await assertNoRuntimeIssues(page);
      await expectTopBarTextToBeReadable(page);
      await page.screenshot({
        path: outputPath(view.screenshot),
        animations: "disabled",
      });
    }

    await exitGuide.click();
  } else if (await exitGuide.isVisible()) {
    await exitGuide.click();
  }

  for (const view of regularViews) {
    await page.getByRole("button", { name: view.navLabel, exact: true }).click();
    await expect(page.getByRole("heading", { name: view.navLabel, exact: true })).toBeVisible();
    await settle(page);
    await assertNoRuntimeIssues(page);
    await expectTopBarTextToBeReadable(page);
    await page.screenshot({
      path: outputPath(view.screenshot),
      animations: "disabled",
    });
  }

  const generatedNames = (await readdir(OUTPUT_ROOT)).filter((name) => name.endsWith(".png")).sort();
  expect(generatedNames).toEqual(expectedNames);
});
