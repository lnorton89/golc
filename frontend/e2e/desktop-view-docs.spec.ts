import { expect, test, type Page } from "@playwright/test";
import { mkdir, readdir } from "node:fs/promises";
import path from "node:path";

import desktopViews from "../src/shell/desktopViews.json" with { type: "json" };

const OUTPUT_ROOT = path.resolve(import.meta.dirname, "../../site/public/desktop-views");
const views = desktopViews.groups.flatMap((group) => group.views);
const DOCUMENTATION_VIEWPORT = { width: 1920, height: 1080 } as const;

async function installHealthyDocumentationBindings(page: Page): Promise<void> {
  await page.addInitScript(() => {
    const ok = (stdout = "") => ({ exitCode: 0, stdout, stderr: "" });
    const browserWindow = window as unknown as {
      go: { wails: Record<string, unknown> };
      runtime: { EventsOn: () => () => void };
    };

    browserWindow.go = {
      wails: {
        SafetyService: {
          Blackout: async () => ok(),
          StopReleaseAll: async () => ok(),
          RevokeAutomation: async () => ok(),
          SetActiveSurface: async () => ok(),
          FetchStatus: async () => ({
            reachable: true,
            active: false,
            bpm: 120,
            barIndex: 0,
            beatFraction: 0,
            enabledLayers: [],
            controllingSource: "live",
            outputState: "live",
          }),
        },
        PlaybackService: {
          SwitchScene: async () => ok(),
          SetLayerEnabled: async () => ok(),
          SetBPM: async () => ok(),
          TapTempo: async () => ok(),
          Evaluate: async () => ok(),
          SetActiveSurface: async () => ok(),
          GetState: async () => ok(JSON.stringify({ bpm: 120, scenes: [] })),
        },
        ShowService: {
          Save: async () => ok(),
          SaveAs: async () => ok(),
          Inspect: async () => ({
            showPath: "C:\\Shows\\Demo.golc",
            schemaVersion: 1,
            revision: 1,
            pools: [
              {
                id: "pool-wash",
                name: "Wash",
                requiredCapabilities: ["intensity", "color"],
                memberCount: 4,
              },
            ],
            deployments: [
              {
                id: "deployment-main",
                name: "Main Rig",
                active: true,
                instanceCount: 4,
              },
            ],
          }),
          Diagnose: async () => ({
            fileLevelIssues: [],
            structuralOk: true,
            migrationRequired: false,
            schemaVersion: 1,
            revision: 1,
          }),
          DetectRecoveryPoints: async () => [],
          AcceptRecoveryPoint: async () => ok(),
          DiscardRecoveryPoints: async () => ok(),
        },
        FixtureLibraryService: {
          ListLocal: async () => ({ directory: "fixtures", rows: [] }),
        },
        FixturePatchService: {
          ListPatch: async () => ({ pools: [], deployments: [] }),
        },
        ProgrammingService: {
          ListProgramming: async () => ({
            scenes: [],
            themes: [],
            presets: [],
            chases: [],
            motions: [],
            blends: [],
            instances: [],
          }),
        },
        ScriptService: {
          ListScripts: async () => [],
          GetSDKTypeDefinitions: async () => "",
        },
        SurfaceService: {
          ListSurfaces: async () => [],
        },
        MidiService: {
          ListMappings: async () => [],
          SetActiveSurface: async () => ok(),
        },
        ArtnetConfigService: {
          ListInterfaces: async () => [
            {
              index: 1,
              name: "Ethernet",
              up: true,
              addrs: ["192.168.1.10/24"],
              pinned: true,
              status: "ready",
              error: "",
            },
          ],
          FetchArtnetStatus: async () => ({
            reachable: true,
            interface: {
              pinnedIndex: 1,
              pinnedName: "Ethernet",
              status: "ready",
              error: "",
            },
            targets: [],
          }),
        },
      },
    };
    browserWindow.runtime = { EventsOn: () => () => {} };
  });
}

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

async function expectTopBarTextToBeReadable(page: Page): Promise<void> {
  const issues = await page.locator("header").evaluate((frame) => {
    const frameRect = frame.getBoundingClientRect();
    const textElements = [frame, ...frame.querySelectorAll<HTMLElement>("*")].filter((element) => {
      const hasDirectText = [...element.childNodes].some(
        (node) => node.nodeType === Node.TEXT_NODE && node.textContent?.trim(),
      );
      if (!hasDirectText) return false;

      const style = window.getComputedStyle(element);
      const rect = element.getBoundingClientRect();
      return style.visibility !== "hidden" && style.display !== "none" && rect.width > 0 && rect.height > 0;
    });
    const failures: string[] = [];
    const describe = (element: HTMLElement) =>
      (element.textContent ?? "").replace(/\s+/g, " ").trim().slice(0, 80);

    for (const element of textElements) {
      const rect = element.getBoundingClientRect();
      if (
        rect.left < frameRect.left - 1 ||
        rect.right > frameRect.right + 1 ||
        rect.top < frameRect.top - 1 ||
        rect.bottom > frameRect.bottom + 1
      ) {
        failures.push(`"${describe(element)}" extends outside the top bar`);
      }

      const style = window.getComputedStyle(element);
      const clipsHorizontally = ["auto", "clip", "hidden", "scroll"].includes(style.overflowX);
      const clipsVertically = ["auto", "clip", "hidden", "scroll"].includes(style.overflowY);
      if (
        (clipsHorizontally && element.scrollWidth > element.clientWidth + 1) ||
        (clipsVertically && element.scrollHeight > element.clientHeight + 1)
      ) {
        failures.push(`"${describe(element)}" is clipped inside its own box`);
      }
    }

    for (let leftIndex = 0; leftIndex < textElements.length; leftIndex += 1) {
      const left = textElements[leftIndex];
      const leftRect = left.getBoundingClientRect();
      for (let rightIndex = leftIndex + 1; rightIndex < textElements.length; rightIndex += 1) {
        const right = textElements[rightIndex];
        if (left.contains(right) || right.contains(left)) continue;

        const rightRect = right.getBoundingClientRect();
        const intersectionWidth = Math.min(leftRect.right, rightRect.right) - Math.max(leftRect.left, rightRect.left);
        const intersectionHeight =
          Math.min(leftRect.bottom, rightRect.bottom) - Math.max(leftRect.top, rightRect.top);
        if (intersectionWidth > 1 && intersectionHeight > 1) {
          failures.push(`"${describe(left)}" overlaps "${describe(right)}"`);
        }
      }
    }

    return [...new Set(failures)];
  });

  expect(issues, "top-bar text must not overlap, jumble, or clip before capture").toEqual([]);
}

test("captures every catalog desktop destination", async ({ page }) => {
  test.setTimeout(90_000);
  await page.setViewportSize(DOCUMENTATION_VIEWPORT);
  await installHealthyDocumentationBindings(page);
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
    await expect(page.getByText(/Can.t reach the playback engine/i)).toHaveCount(0);
    await expect(page.getByText(/Can.t reach the script host/i)).toHaveCount(0);
    await expect(page.getByText(/GOLC_WAILS_(?:BINDING|BRIDGE)_UNAVAILABLE/i)).toHaveCount(0);
    await expect(page.getByText("Issues found", { exact: true })).toHaveCount(0);
    await expect(page.getByText("offline", { exact: true })).toHaveCount(0);
    await expectTopBarTextToBeReadable(page);
    await page.screenshot({
      path: outputPath(view.screenshot),
      animations: "disabled",
    });
  }

  const generatedNames = (await readdir(OUTPUT_ROOT)).filter((name) => name.endsWith(".png")).sort();
  expect(generatedNames).toEqual(expectedNames);
});
