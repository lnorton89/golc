import { expect, test, type Page } from "@playwright/test";
import { mkdir, readdir } from "node:fs/promises";
import path from "node:path";

import desktopViews from "../src/shell/desktopViews.json" with { type: "json" };

const OUTPUT_ROOT = path.resolve(import.meta.dirname, "../../site/public/desktop-views");
const views = desktopViews.groups.flatMap((group) => group.views);

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
  await page.setViewportSize({ width: 1440, height: 900 });
  await mkdir(OUTPUT_ROOT, { recursive: true });

  const expectedNames = views.map((view) => path.basename(view.screenshot)).sort();
  expect(new Set(expectedNames).size, "catalog screenshot paths must be unique").toBe(expectedNames.length);

  const existingNames = (await readdir(OUTPUT_ROOT)).filter((name) => name.endsWith(".png")).sort();
  const extraNames = existingNames.filter((name) => !expectedNames.includes(name));
  expect(extraNames, "remove stale desktop-view screenshots before capture").toEqual([]);

  await page.goto("/");
  const exitGuide = page.getByRole("button", { name: "Exit Guide" });
  if (await exitGuide.isVisible()) {
    await exitGuide.click();
  }

  for (const view of views) {
    await page.getByRole("button", { name: view.navLabel, exact: true }).click();
    await expect(page.getByRole("heading", { name: view.navLabel, exact: true })).toBeVisible();
    await settle(page);
    await page.screenshot({
      path: outputPath(view.screenshot),
      animations: "disabled",
    });
  }

  const generatedNames = (await readdir(OUTPUT_ROOT)).filter((name) => name.endsWith(".png")).sort();
  expect(generatedNames).toEqual(expectedNames);
});
