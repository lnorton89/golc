// design-system.tooltip-overflow.spec.ts: real-browser regression coverage
// for InfoTooltip's viewport-edge flip behavior. jsdom (InfoTooltip.test.tsx)
// has no real layout engine -- getBoundingClientRect is always zeroed there,
// so a genuine CSS layout bug in the flip path is invisible to that suite
// no matter how it's written. This is exactly the class of bug that shipped
// here once already: a `position: fixed` tooltip anchored via `left` plus a
// horizontal `transform: translateX(-100%)` collapsed to its narrowest
// unbreakable word (every other word wrapping onto its own line) when
// opening near the right edge, because the browser computes a fixed box's
// shrink-to-fit width from "left to the viewport's right edge" BEFORE any
// transform is applied -- the transform only repositions the already-narrow
// box afterward, it cannot widen it back out. Fixed by anchoring the flipped
// case via a real `right` offset instead (InfoTooltip.tsx/.module.css).
import { expect, test } from "@playwright/test";

import { installHealthyBindings, settle, waitForFonts } from "./helpers";

const NARROW_WIDTH = 900;
const HEIGHT = 720;

test.describe("InfoTooltip viewport-edge flip", () => {
  test("a tooltip opened near the right edge flips leftward without collapsing to a single-word column", async ({ page }) => {
    await installHealthyBindings(page);
    await page.setViewportSize({ width: NARROW_WIDTH, height: HEIGHT });
    await page.goto("/");
    await expect(page.getByRole("heading", { name: "Overview", exact: true })).toBeVisible();
    await waitForFonts(page);
    await settle(page);

    // WorkspaceFrame's own `action` slot sits at the workspace title row's
    // far right edge by design -- the same slot every workspace's own
    // "How <X> works" InfoTooltip renders into, and the one that actually
    // shipped the collapsed-tooltip regression this test guards against.
    const trigger = page.getByRole("button", { name: "How Overview works" });
    await expect(trigger).toBeVisible();
    const triggerBox = await trigger.boundingBox();
    expect(triggerBox, "trigger must have a real bounding box").not.toBeNull();
    if (!triggerBox) return;
    // Confirms this test is actually exercising the near-edge case it
    // claims to -- if WorkspaceFrame's layout ever moves this trigger away
    // from the edge, this assertion (not a silent pass) is what catches it.
    expect(triggerBox.x + triggerBox.width, "trigger must sit near the viewport's right edge for this test to be meaningful").toBeGreaterThan(NARROW_WIDTH - 80);

    await trigger.hover();
    const tooltip = page.getByRole("tooltip");
    await expect(tooltip).toBeVisible();
    await expect(tooltip).toHaveAttribute("data-flip", "true");

    const tooltipBox = await tooltip.boundingBox();
    expect(tooltipBox, "tooltip must have a real bounding box").not.toBeNull();
    if (!tooltipBox) return;

    // The exact shape of the regression: a collapsed tooltip rendered at
    // ~80px wide and ~380px tall (one word per line) for a one-sentence
    // description. A correctly flowing tooltip for real sentence-length
    // text is wider than it is tall, and comfortably clears a genuine
    // multi-word minimum width.
    expect(tooltipBox.width, "tooltip must not collapse to a single-word column width").toBeGreaterThan(200);
    expect(tooltipBox.width, "tooltip must be wider than it is tall for ordinary sentence-length text").toBeGreaterThan(tooltipBox.height);

    // Must not overflow either edge of the viewport once flipped.
    expect(tooltipBox.x, "tooltip must not overflow the viewport's left edge").toBeGreaterThanOrEqual(-2);
    expect(tooltipBox.x + tooltipBox.width, "tooltip must not overflow the viewport's right edge").toBeLessThanOrEqual(NARROW_WIDTH + 2);
  });

  test("a tooltip opened away from any edge does not flip and renders at its natural width", async ({ page }) => {
    await installHealthyBindings(page);
    await page.setViewportSize({ width: 1280, height: HEIGHT });
    await page.goto("/");
    await expect(page.getByRole("heading", { name: "Overview", exact: true })).toBeVisible();
    await waitForFonts(page);
    await settle(page);

    const trigger = page.getByRole("button", { name: "About the Show section" });
    await expect(trigger).toBeVisible();
    await trigger.hover();

    const tooltip = page.getByRole("tooltip");
    await expect(tooltip).toBeVisible();
    await expect(tooltip).not.toHaveAttribute("data-flip", "");

    const tooltipBox = await tooltip.boundingBox();
    expect(tooltipBox).not.toBeNull();
    if (!tooltipBox) return;
    expect(tooltipBox.width, "an unflipped tooltip must not be malformed either").toBeGreaterThan(200);
    expect(tooltipBox.x + tooltipBox.width, "unflipped tooltip must stay inside the viewport").toBeLessThanOrEqual(1280 + 2);
  });

  test("a tooltip opened near the left edge (CommandRail nav item) opens rightward instead of flipping into negative space", async ({ page }) => {
    // The second, distinct instance of the collapse bug: CommandRail's nav
    // destination buttons sit flush against the rail's own left edge (x=8
    // regardless of viewport width), so a flip decision that only checks
    // "would opening rightward overflow the right edge?" without ever
    // comparing how much room the LEFT side genuinely has can flip a trigger
    // that has almost no room on either side -- anchoring the tooltip's
    // right edge AT the trigger's own left edge (x=8) and pushing the box
    // further left, off-screen. The fix compares spaceLeft/spaceRight up
    // front and only flips when the left side is genuinely roomier.
    await installHealthyBindings(page);
    await page.setViewportSize({ width: NARROW_WIDTH, height: HEIGHT });
    await page.goto("/");
    await expect(page.getByRole("heading", { name: "Overview", exact: true })).toBeVisible();
    await waitForFonts(page);
    await settle(page);

    const trigger = page.getByRole("navigation", { name: "Workspace navigation" }).getByRole("button", { name: "Fixture Library" });
    await expect(trigger).toBeVisible();
    const triggerBox = await trigger.boundingBox();
    expect(triggerBox, "trigger must have a real bounding box").not.toBeNull();
    if (!triggerBox) return;
    expect(triggerBox.x, "trigger must sit near the viewport's left edge for this test to be meaningful").toBeLessThan(20);

    await trigger.hover();
    const tooltip = page.getByRole("tooltip");
    await expect(tooltip).toBeVisible();
    await expect(tooltip).not.toHaveAttribute("data-flip", "true");

    const tooltipBox = await tooltip.boundingBox();
    expect(tooltipBox, "tooltip must have a real bounding box").not.toBeNull();
    if (!tooltipBox) return;

    expect(tooltipBox.width, "tooltip must not collapse to a single-word column width").toBeGreaterThan(200);
    expect(tooltipBox.width, "tooltip must be wider than it is tall for ordinary sentence-length text").toBeGreaterThan(tooltipBox.height);
    expect(tooltipBox.x, "tooltip must not overflow the viewport's left edge").toBeGreaterThanOrEqual(-2);
    expect(tooltipBox.x + tooltipBox.width, "tooltip must not overflow the viewport's right edge").toBeLessThanOrEqual(NARROW_WIDTH + 2);
  });
});
