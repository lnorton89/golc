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
import { test, expect, type Page } from "@playwright/test";

const DESTINATIONS = [
  "Overview",
  "Shows",
  "Save & Recovery",
  "Settings",
  "Fixture Library",
  "Patch & Pools",
  "Scenes & Looks",
  "Scripts",
  "Operator Surface",
  "MIDI Mapping",
  "Art-Net",
  "Diagnostics",
];

// NARROW is below GuidedFirstShow's stageSection min-width (460px) plus
// its rail/gaps/evidence-aside, and below ScriptsWorkspace's 7-button
// toolbar row's natural width -- both this suite's two named regression
// cases actually need a viewport this tight to reproduce. WIDE is a
// normal desktop width, included so the sweep also catches a control
// that only breaks once there's *extra* room (unlikely here, cheap to
// check).
const WIDTHS = [900, 1280] as const;

// settle waits out AppShell.module.css's own `--motion-settle` transition
// (the inspector column's width, which several workspaces below trigger
// by portaling content into it) before measuring layout -- a fixed short
// wait, not the usual Playwright anti-pattern, because there's no better
// signal to poll for: the geometry itself is what's still animating.
async function settle(page: Page): Promise<void> {
  await page.waitForTimeout(250);
}

// findOverflowingButtons returns one description string per <button> that
// exhibits either failure mode above; an empty array means everything on
// screen fits its own box and the viewport. Elements with zero size are
// skipped (display:none/detached, e.g. a hidden dialog's own buttons).
async function findOverflowingButtons(page: Page): Promise<string[]> {
  return page.evaluate(() => {
    const offenders: string[] = [];
    const viewportWidth = window.innerWidth;
    for (const button of Array.from(document.querySelectorAll("button"))) {
      const rect = button.getBoundingClientRect();
      if (rect.width === 0 && rect.height === 0) continue;
      const label = button.textContent?.trim() || button.getAttribute("aria-label") || "(unlabeled button)";
      if (button.scrollHeight - button.clientHeight > 1) {
        offenders.push(`"${label}": content (${button.scrollHeight}px) taller than its own box (${button.clientHeight}px) -- label wrapped and got cut off`);
      }
      if (rect.right > viewportWidth + 1) {
        offenders.push(`"${label}": right edge (${Math.round(rect.right)}px) past the viewport (${viewportWidth}px)`);
      }
      if (rect.left < -1) {
        offenders.push(`"${label}": left edge (${Math.round(rect.left)}px) pushed off-screen`);
      }
    }
    return offenders;
  });
}

test.describe("responsive layout: no button overflows its box or the window", () => {
  for (const width of WIDTHS) {
    test(`every workspace at ${width}px width`, async ({ page }) => {
      await page.setViewportSize({ width, height: 720 });
      await page.goto("/");

      for (const label of DESTINATIONS) {
        await page.getByRole("button", { name: label, exact: true }).click();
        await expect(page.getByRole("heading", { name: label, exact: true })).toBeVisible();
        await settle(page);

        const offenders = await findOverflowingButtons(page);
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
    const offenders = await findOverflowingButtons(page);
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
    const offenders = await findOverflowingButtons(page);
    expect(offenders).toEqual([]);
  });
});
