// safety-action-hold.spec.ts is the real-browser regression guard for
// Plan 13-15's hold-to-confirm safety controls (D-13/D-14) -- the
// SafetyCluster.test.tsx unit suite already covers the state machine
// deterministically under fake timers (including a simulated
// pointercancel), but only a real browser exercises genuine OS-level
// pointer capture/release and window/element focus semantics: a mouse
// button actually held down for N real milliseconds, a real pointerleave
// fired by moving the cursor off the control while still pressed, and a
// real keyboard focus/blur/Escape sequence. Chromium only (real pointer
// timing, not a cross-browser sweep).
import { test, expect, type Page } from "@playwright/test";

import { installHealthyBindings } from "./helpers";

// Mirrors SafetyCluster.tsx's own HOLD_DURATION_MS -- kept as a literal
// here rather than importing app source into a Playwright spec (this repo
// convention: e2e specs stay decoupled from src internals, driving only
// through rendered DOM/aria surfaces).
const HOLD_DURATION_MS = 750;

// installHealthyBindings itself registers an addInitScript that
// constructs the entire `window.go.wails` surface from scratch --
// Playwright runs addInitScript callbacks in registration order at each
// new document, so any per-test override of a single SafetyService method
// must be registered *after* installHealthyBindings, never before (an
// earlier override would run against a `window.go` that doesn't exist yet
// and be silently clobbered once installHealthyBindings's own script
// constructs the whole object moments later).
async function gotoWithHealthyBindings(page: Page): Promise<void> {
  await installHealthyBindings(page);
  await page.goto("/");
}

test.describe("SafetyCluster hold-to-confirm (real browser)", () => {
  test("a real held-down mouse press past the threshold dispatches exactly once", async ({ page }) => {
    let blackoutCalls = 0;
    await installHealthyBindings(page);
    await page.addInitScript(() => {
      const browserWindow = window as unknown as { go: { wails: { SafetyService: Record<string, unknown> } } };
      browserWindow.go.wails.SafetyService.Blackout = async (on: boolean) => {
        (window as unknown as { __blackoutCalls: unknown[] }).__blackoutCalls ??= [];
        (window as unknown as { __blackoutCalls: unknown[] }).__blackoutCalls.push(on);
        return { exitCode: 0, stdout: "", stderr: "" };
      };
    });
    await page.goto("/");

    const button = page.getByRole("button", { name: "Blackout" });
    await expect(button).toBeVisible();

    const box = await button.boundingBox();
    if (!box) throw new Error("Blackout button has no bounding box");
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
    await page.mouse.down();
    await page.waitForTimeout(HOLD_DURATION_MS + 150);
    await page.mouse.up();

    blackoutCalls = await page.evaluate(
      () => ((window as unknown as { __blackoutCalls?: unknown[] }).__blackoutCalls ?? []).length,
    );
    // The label toggling to "Release Blackout" depends on the daemon's own
    // status echo (fetchSafetyStatus/"status:update"), not on this call's
    // own resolution -- out of scope for this mock, which only proves the
    // real-browser hold timing dispatched the command exactly once.
    expect(blackoutCalls).toBe(1);
  });

  test("releasing the real mouse press before the threshold cancels without dispatching", async ({ page }) => {
    await installHealthyBindings(page);
    await page.addInitScript(() => {
      const browserWindow = window as unknown as { go: { wails: { SafetyService: Record<string, unknown> } } };
      browserWindow.go.wails.SafetyService.RevokeAutomation = async () => {
        (window as unknown as { __revokeCalls: number }).__revokeCalls =
          ((window as unknown as { __revokeCalls?: number }).__revokeCalls ?? 0) + 1;
        return { exitCode: 0, stdout: "", stderr: "" };
      };
    });
    await page.goto("/");

    const button = page.getByRole("button", { name: "Automation" });
    const box = await button.boundingBox();
    if (!box) throw new Error("Automation button has no bounding box");
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
    await page.mouse.down();
    await page.waitForTimeout(HOLD_DURATION_MS - 300);
    await page.mouse.up();
    await page.waitForTimeout(HOLD_DURATION_MS);

    const revokeCalls = await page.evaluate(
      () => (window as unknown as { __revokeCalls?: number }).__revokeCalls ?? 0,
    );
    expect(revokeCalls).toBe(0);
    // The control must remain fully reachable for an immediate, successful
    // retry -- not stuck in a half-held state.
    await expect(button).toBeEnabled();
  });

  test("moving the real cursor off the control while still pressed (pointerleave) cancels the hold", async ({ page }) => {
    await installHealthyBindings(page);
    await page.addInitScript(() => {
      const browserWindow = window as unknown as { go: { wails: { SafetyService: Record<string, unknown> } } };
      browserWindow.go.wails.SafetyService.StopReleaseAll = async () => {
        (window as unknown as { __stopCalls: number }).__stopCalls =
          ((window as unknown as { __stopCalls?: number }).__stopCalls ?? 0) + 1;
        return { exitCode: 0, stdout: "", stderr: "" };
      };
    });
    await page.goto("/");

    const button = page.getByRole("button", { name: "Stop / Release All" });
    const box = await button.boundingBox();
    if (!box) throw new Error("Stop / Release All button has no bounding box");
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
    await page.mouse.down();
    await page.waitForTimeout(200);
    // Move far outside the control's own box while the button is still
    // held -- a real pointerleave, the actual browser signal this
    // component treats as a cancellation trigger (no setPointerCapture is
    // used, matching a plain, undecorated hold-to-confirm control).
    await page.mouse.move(box.x + box.width + 400, box.y + box.height + 400);
    await page.waitForTimeout(HOLD_DURATION_MS);
    await page.mouse.up();

    const stopCalls = await page.evaluate(
      () => (window as unknown as { __stopCalls?: number }).__stopCalls ?? 0,
    );
    expect(stopCalls).toBe(0);
  });

  test("keyboard focus/blur: tabbing away mid-hold cancels without dispatching", async ({ page }) => {
    await installHealthyBindings(page);
    await page.addInitScript(() => {
      const browserWindow = window as unknown as { go: { wails: { SafetyService: Record<string, unknown> } } };
      browserWindow.go.wails.SafetyService.Blackout = async () => {
        (window as unknown as { __blackoutCalls: number }).__blackoutCalls =
          ((window as unknown as { __blackoutCalls?: number }).__blackoutCalls ?? 0) + 1;
        return { exitCode: 0, stdout: "", stderr: "" };
      };
    });
    await page.goto("/");

    const button = page.getByRole("button", { name: "Blackout" });
    await button.focus();
    await page.keyboard.down("Space");
    await page.waitForTimeout(200);
    // A real Tab moves focus away (element blur) well before the
    // threshold -- must cancel without ever dispatching.
    await page.keyboard.press("Tab");
    await page.keyboard.up("Space");
    await page.waitForTimeout(HOLD_DURATION_MS);

    const blackoutCalls = await page.evaluate(
      () => (window as unknown as { __blackoutCalls?: number }).__blackoutCalls ?? 0,
    );
    expect(blackoutCalls).toBe(0);
  });

  test("a real Escape keypress mid-hold cancels without dispatching, and a fresh hold then completes normally", async ({ page }) => {
    await installHealthyBindings(page);
    await page.addInitScript(() => {
      const browserWindow = window as unknown as { go: { wails: { SafetyService: Record<string, unknown> } } };
      browserWindow.go.wails.SafetyService.Blackout = async () => {
        (window as unknown as { __blackoutCalls: number }).__blackoutCalls =
          ((window as unknown as { __blackoutCalls?: number }).__blackoutCalls ?? 0) + 1;
        return { exitCode: 0, stdout: "", stderr: "" };
      };
    });
    await page.goto("/");

    const button = page.getByRole("button", { name: "Blackout" });
    await button.focus();
    await page.keyboard.down("Space");
    await page.waitForTimeout(200);
    await page.keyboard.press("Escape");
    await page.keyboard.up("Space");
    await page.waitForTimeout(HOLD_DURATION_MS);

    let blackoutCalls = await page.evaluate(
      () => (window as unknown as { __blackoutCalls?: number }).__blackoutCalls ?? 0,
    );
    expect(blackoutCalls).toBe(0);

    // A fresh hold after the cancelled one must still complete normally --
    // the machine is not left in a broken state by the earlier Escape.
    await button.focus();
    await page.keyboard.down("Space");
    await page.waitForTimeout(HOLD_DURATION_MS + 150);
    await page.keyboard.up("Space");

    blackoutCalls = await page.evaluate(
      () => (window as unknown as { __blackoutCalls?: number }).__blackoutCalls ?? 0,
    );
    expect(blackoutCalls).toBe(1);
  });

  test("all three safety controls stay visible, distinct, and independently reachable throughout", async ({ page }) => {
    await gotoWithHealthyBindings(page);

    const cluster = page.getByLabel("Safety cluster");
    await expect(cluster).toBeVisible();
    const controls = cluster.locator("button");
    await expect(controls).toHaveCount(3);
    for (let index = 0; index < 3; index += 1) {
      await expect(controls.nth(index)).toBeVisible();
      await expect(controls.nth(index)).toBeEnabled();
    }
    await expect(page.getByRole("button", { name: "Blackout" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Automation" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Stop / Release All" })).toBeVisible();
  });
});
