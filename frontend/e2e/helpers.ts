// Shared Playwright e2e harness for the responsive, resize, and desktop-view
// docs-capture suites: catalog-derived destinations, the fixed motion-settle
// wait, the healthy Wails mock surface, runtime-issue/top-bar guards, and the
// geometry/safety-cluster assertions every suite needs. Extracting this once
// (260801-qrz) replaces responsive.spec.ts's own hardcoded twelve-destination
// list -- stale against desktopViews.json's fourteen catalog entries, missing
// Project Fixtures and Desk -- with the catalog itself, so a future catalog
// change fails the sweep loudly instead of silently skipping a workspace.
import { expect, type Page } from "@playwright/test";

import desktopViews from "../src/shell/desktopViews.json" with { type: "json" };

// NAV_LABELS: every regular (non-guide) destination's nav label, in catalog
// order. The Guided First Show overlay is deliberately never a nav
// destination (D-10) and lives in desktopViews.json's separate "onboarding"
// section, so it is not part of this sweep.
export const NAV_LABELS: string[] = desktopViews.groups.flatMap((group) => group.views.map((view) => view.navLabel));

// settle waits out AppShell.module.css's own `--motion-settle` transition
// (the inspector column's width, which several workspaces trigger by
// portaling content into it) before measuring layout -- a fixed short wait,
// not the usual Playwright anti-pattern, because there's no better signal to
// poll for: the geometry itself is what's still animating.
export async function settle(page: Page): Promise<void> {
  await page.waitForTimeout(250);
}

// waitForFonts resolves once the browser's own FontFaceSet has finished
// loading every @font-face declared by index.css (Archivo, JetBrains Mono).
// Required before any deterministic screenshot capture (260803, Plan
// 13-17): a capture taken before web fonts settle silently falls back to a
// system font for one or two frames, which is real nondeterminism no
// animation/caret freeze touches -- Chromium's own font-loading timing
// varies run to run independent of anything this app controls.
export async function waitForFonts(page: Page): Promise<void> {
  await page.evaluate(async () => {
    await document.fonts.ready;
  });
}

// installHealthyBindings installs the complete healthy Wails mock surface
// (moved verbatim from desktop-view-docs.spec.ts) so every workspace renders
// its content-rich state instead of a daemon-unreachable fallback.
export async function installHealthyBindings(page: Page): Promise<void> {
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
          ListDeskMappings: async () => [],
          RemoveDeskMapping: async () => ok(),
        },
        NotesService: {
          ListNotes: async () => [],
          GetNote: async () => ({ id: "", title: "", body: "" }),
          CreateNote: async () => ok(),
          SaveNote: async () => ok(),
          DeleteNote: async () => ok(),
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

// expectTopBarTextToBeReadable (moved verbatim from desktop-view-docs.spec.ts):
// asserts every piece of text in the persistent top bar stays inside its own
// box, isn't clipped, and never overlaps another text element -- the
// screenshot-viewport-text-overlap regression this repo already hit once.
export async function expectTopBarTextToBeReadable(page: Page): Promise<void> {
  // .first(): GlobalFrame's persistent top-bar <header> is always the first
  // <header> in DOM order (AppShell.tsx mounts it ahead of <main>/
  // ShellCanvas); GuidedFirstShow.tsx's own per-stage <header> nests a
  // second one only while the guide overlay is open.
  const issues = await page.locator("header").first().evaluate((frame) => {
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
    const describe = (element: Element) =>
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

// assertNoRuntimeIssues (moved verbatim from desktop-view-docs.spec.ts):
// fails loudly if a workspace ever renders a daemon/script-host-unreachable
// fallback instead of its real content-rich state.
export async function assertNoRuntimeIssues(page: Page): Promise<void> {
  await expect(page.getByText(/Can.t reach the playback engine/i)).toHaveCount(0);
  await expect(page.getByText(/Can.t reach the script host/i)).toHaveCount(0);
  await expect(page.getByText(/Can.t reach the show host/i)).toHaveCount(0);
  await expect(page.getByText(/GOLC_WAILS_(?:BINDING|BRIDGE)_UNAVAILABLE/i)).toHaveCount(0);
  await expect(page.getByText("Issues found", { exact: true })).toHaveCount(0);
  await expect(page.getByText("offline", { exact: true })).toHaveCount(0);
}

// findOverflowingControls extends responsive.spec.ts's original
// findOverflowingButtons (still only checking <button>) to also check
// input/select/[contenteditable], plus a document- and .appShell-level
// horizontal-overflow check -- the window overflowing instead of the
// workspace canvas scrolling internally is exactly the failure mode D-13's
// shell contract forbids. Elements with zero size are skipped (display:none/
// detached, e.g. a hidden dialog's own controls). An empty array means
// everything on screen fits its own box and the viewport.
export async function findOverflowingControls(page: Page): Promise<string[]> {
  return page.evaluate(() => {
    const offenders: string[] = [];
    const viewportWidth = window.innerWidth;
    const controls = Array.from(document.querySelectorAll<HTMLElement>("button, input, select, [contenteditable]"));

    for (const control of controls) {
      const rect = control.getBoundingClientRect();
      if (rect.width === 0 && rect.height === 0) continue;
      const label =
        control.textContent?.trim() ||
        control.getAttribute("aria-label") ||
        (control as HTMLInputElement).name ||
        `(unlabeled ${control.tagName.toLowerCase()})`;

      if (control.scrollHeight - control.clientHeight > 1) {
        offenders.push(`"${label}": content (${control.scrollHeight}px) taller than its own box (${control.clientHeight}px) -- wrapped and got cut off`);
      }
      if (rect.right > viewportWidth + 1) {
        offenders.push(`"${label}": right edge (${Math.round(rect.right)}px) past the viewport (${viewportWidth}px)`);
      }
      if (rect.left < -1) {
        offenders.push(`"${label}": left edge (${Math.round(rect.left)}px) pushed off-screen`);
      }
    }

    const doc = document.documentElement;
    if (doc.scrollWidth - doc.clientWidth > 1) {
      offenders.push(`document: horizontal overflow (scrollWidth ${doc.scrollWidth}px > clientWidth ${doc.clientWidth}px)`);
    }

    const appShell = document.querySelector<HTMLElement>('[class*="appShell"]');
    if (appShell && appShell.scrollWidth - appShell.clientWidth > 1) {
      offenders.push(`.appShell: horizontal overflow (scrollWidth ${appShell.scrollWidth}px > clientWidth ${appShell.clientWidth}px)`);
    }

    return offenders;
  });
}

// expectSafetyClusterAvailable asserts D-13's guarantee: Blackout, Automation,
// and Stop/Release All stay visible and interactive on every workspace and at
// every size, independent of daemon reachability. Located by the cluster's
// own aria-label rather than by each button's label text, because Blackout
// and Stop/Release All each toggle their own label ("Blackout" <->
// "Release Blackout", etc.) depending on live output state.
export async function expectSafetyClusterAvailable(page: Page): Promise<void> {
  const cluster = page.getByLabel("Safety cluster");
  await expect(cluster).toBeVisible();
  const controls = cluster.locator("button");
  await expect(controls).toHaveCount(3);
  for (let index = 0; index < 3; index += 1) {
    await expect(controls.nth(index)).toBeVisible();
    await expect(controls.nth(index)).toBeEnabled();
  }
}

// chooseOption drives the design system's Select and Combobox primitives,
// both of which are Base UI-backed and expose `role="combobox"` on their
// trigger -- a <button> for Select, an <input> for Combobox -- with their
// options portalled out as `role="option"` elements.
//
// This exists because Playwright's `selectOption()` only works on a native
// <select>, which is what these controls used to be before the design-system
// migration. Specs that still called selectOption() failed two different
// ways depending on which primitive they hit: "Element is not a <select>
// element" for the Combobox, and a 30s timeout for the Select (whose
// accessible name also changed with the migration). Driving the real
// controls the way an operator does -- open, then pick -- keeps every future
// spec off that rake, and matches the equivalent helper the Vitest suites
// already share (see FixturePatch.test.tsx's chooseComboboxOption).
//
// `optionLabel` is the option's VISIBLE text, not the underlying value: the
// old native-<select> calls passed values ("par-rgbw"), while the rendered
// option reads "Acme PAR RGBW".
export async function chooseOption(page: Page, triggerName: string, optionLabel: string): Promise<void> {
  await page.getByRole("combobox", { name: triggerName, exact: true }).click();
  await page.getByRole("option", { name: optionLabel, exact: true }).click();
}

// expectChosenOption asserts a Select/Combobox is currently showing
// `optionLabel`. Deliberately not toHaveValue(): a Base UI Select's trigger
// is a <button>, which has no value property at all, so the old
// toHaveValue() assertions silently only ever made sense for the native
// <select> these replaced. The Combobox's <input> does carry a value, so
// both shapes are handled here rather than at each call site.
export async function expectChosenOption(page: Page, triggerName: string, optionLabel: string): Promise<void> {
  const trigger = page.getByRole("combobox", { name: triggerName, exact: true });
  const tag = await trigger.evaluate((node) => node.tagName);
  if (tag === "INPUT") {
    await expect(trigger).toHaveValue(optionLabel);
  } else {
    await expect(trigger).toHaveText(optionLabel);
  }
}
