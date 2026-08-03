import { expect } from "@playwright/test";

import { dialogTest as test } from "./fixtures/dialogFeasibility";

test.describe("dialog feasibility", () => {
  test("keeps focus, dismissal, portals, and safety controls usable", async ({ dialogPage: page }) => {
    await expect(page.getByRole("button", { name: "Open allowed dialog" })).toBeFocused();
    await page.getByRole("button", { name: "Open allowed dialog" }).click();

    const dialog = page.getByRole("dialog", { name: "Review fixture patch" });
    await expect(dialog).toBeVisible();
    await expect(page.getByRole("button", { name: "Keep editing" })).toBeFocused();
    await expect(page.getByTestId("nested-portal-content")).toBeVisible();

    await page.getByRole("button", { name: "Apply review" }).focus();
    await page.keyboard.press("Tab");
    await expect(page.getByRole("button", { name: "Keep editing" })).toBeFocused();
    await page.keyboard.press("Shift+Tab");
    await expect(page.getByRole("button", { name: "Apply review" })).toBeFocused();

    const safetyControls = page.getByLabel("Safety cluster").locator("button");
    await expect(safetyControls).toHaveCount(3);
    for (let index = 0; index < 3; index += 1) {
      await expect(safetyControls.nth(index)).toBeVisible();
      await expect(safetyControls.nth(index)).toBeEnabled();
    }

    await page.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Open allowed dialog" })).toBeFocused();

    await page.getByRole("button", { name: "Open allowed dialog" }).click();
    await page.getByTestId("dialog-backdrop").click({ position: { x: 2, y: 2 } });
    await expect(dialog).toHaveCount(0);

    await page.getByRole("button", { name: "Open blocked dialog" }).click();
    const blocked = page.getByRole("dialog", { name: "Discard fixture changes?" });
    await expect(blocked).toBeVisible();
    await page.keyboard.press("Escape");
    await page.getByTestId("dialog-backdrop").click({ position: { x: 2, y: 2 } });
    await expect(blocked).toBeVisible();
    await page.getByRole("button", { name: "Keep fixture" }).click();
    await expect(blocked).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Open blocked dialog" })).toBeFocused();
  });
});
