import { test as base, type Page } from "@playwright/test";

type DialogFeasibilityFixtures = {
  dialogPage: Page;
};

function fixtureURL(page: Page): string {
  const current = page.url();
  if (!current || current === "about:blank") return "/?e2e=dialog-feasibility";

  const url = new URL(current);
  url.pathname = "/";
  url.search = "?e2e=dialog-feasibility";
  url.hash = "";
  return url.toString();
}

/**
 * Runs the same acceptance assertions against Vite Chromium by default and
 * against an already-running packaged WebView2 process when the harness sets
 * GOLC_WEBVIEW2_CDP_ENDPOINT.
 */
export const dialogTest = base.extend<DialogFeasibilityFixtures>({
  dialogPage: async ({ page, playwright }, use) => {
    const endpoint = process.env.GOLC_WEBVIEW2_CDP_ENDPOINT;
    if (!endpoint) {
      await page.goto("/?e2e=dialog-feasibility");
      await use(page);
      return;
    }

    const browser = await playwright.chromium.connectOverCDP(endpoint);
    const context = browser.contexts()[0];
    const packagedPage = context.pages().find((candidate) => !candidate.url().startsWith("devtools://"));
    if (!packagedPage) throw new Error(`GOLC_WEBVIEW2_CDP_NO_PAGE: no application page attached at ${endpoint}`);

    await packagedPage.goto(fixtureURL(packagedPage));
    await use(packagedPage);
    await browser.close();
  },
});
