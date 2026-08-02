// responsive.spec.ts is a real-browser regression guard for a class of
// bug jsdom cannot detect at all (see playwright.config.ts's own doc
// comment): a control that overflows its own box or the window at a
// narrower-than-designed width. It reproduces two concrete incidents
// caught by hand during 260729-style UI polish work --
//   1. Button.module.css previously let a label wrap across multiple
//      lines inside a button's fixed height, spilling text out past the
//      button's own box (GuidedFirstShow's footer at ~900px).
//   2. Toolbar.module.css's action slot, when too crowded to fit one
//      row (ScriptsWorkspace's 7-button row), first tried horizontal
//      scroll -- which visually cut buttons off mid-label with a
//      scrollbar underneath them instead of just showing every button.
// -- plus a general sweep across every real workspace so a *new*
// instance of either failure mode (not just a regression of these two)
// gets caught too, without needing to know in advance which component
// broke.
//
// Destinations are the catalog's own NAV_LABELS (e2e/helpers.ts), not a
// second hand-maintained list -- the previous hardcoded twelve-label list
// had drifted stale against desktopViews.json's fourteen (missing Project
// Fixtures and Desk), silently skipping both from this sweep.
import { test, expect } from "@playwright/test";

import { NAV_LABELS, settle, findOverflowingControls } from "./helpers";

// NARROW is below GuidedFirstShow's stageSection min-width (460px) plus
// its rail/gaps/evidence-aside, and below ScriptsWorkspace's 7-button
// toolbar row's natural width -- both this suite's two named regression
// cases actually need a viewport this tight to reproduce. WIDE is a
// normal desktop width, included so the sweep also catches a control
// that only breaks once there's *extra* room (unlikely here, cheap to
// check).
const WIDTHS = [900, 1280] as const;

test.describe("responsive layout: no control overflows its box or the window", () => {
  for (const width of WIDTHS) {
    test(`every workspace at ${width}px width`, async ({ page }) => {
      await page.setViewportSize({ width, height: 720 });
      await page.goto("/");

      for (const label of NAV_LABELS) {
        await page.getByRole("button", { name: label, exact: true }).click();
        await expect(page.getByRole("heading", { name: label, exact: true })).toBeVisible();
        await settle(page);

        const offenders = await findOverflowingControls(page);
        expect(offenders, `${label} at ${width}px`).toEqual([]);
      }
    });
  }

  test("Guided First Show footer stays on-screen at 900px (regression: stageSection min-width)", async ({ page }) => {
    await page.setViewportSize({ width: 900, height: 720 });
    await page.goto("/");
    await page.getByRole("button", { name: "Start Guide" }).click();

    await expect(page.getByRole("navigation", { name: "First show steps" })).toBeVisible();
    await settle(page);
    const offenders = await findOverflowingControls(page);
    expect(offenders).toEqual([]);
  });

  test("Scripts toolbar wraps its 7-button row instead of scrolling or clipping at 900px (regression: Toolbar action slot)", async ({ page }) => {
    await page.setViewportSize({ width: 900, height: 720 });
    await page.goto("/");
    await page.getByRole("button", { name: "Scripts", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Scripts", exact: true })).toBeVisible();
    await settle(page);

    // Scoped to the toolbar's own action group (not getByRole by name --
    // ScriptsWorkspace's empty-state canvas separately renders its own
    // "New Script" button, a pre-existing duplicate label unrelated to
    // this test's point) so this asserts the 7 real toolbar buttons
    // specifically, not just "a button with this text exists somewhere".
    const toolbarButtons = page.locator('[class*="toolbarActions"] button');
    await expect(toolbarButtons).toHaveCount(7);
    for (const label of ["New Script", "Save", "Delete Script", "Validate", "Run", "Debug", "Stop Script"]) {
      await expect(toolbarButtons.filter({ hasText: label })).toBeVisible();
    }
    const offenders = await findOverflowingControls(page);
    expect(offenders).toEqual([]);
  });
});
