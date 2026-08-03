// design-system.expanded-copy.spec.ts (Plan 13-31 Task 2, D-02/D-03/D-10/
// D-12/D-13/D-14, UI-CONSIDERATIONS-BACKSTOP-LONG-TEXT): the "long-text |
// Localized/expanded future copy" backstop -- "Visual fixtures use at least
// one 2x-length label/message to prove reflow even though full localization
// is outside this phase." Exercises six representative copy categories
// (shell identity, dialog impact, shared error states, Guided First Show
// evidence, live status/log copy, and field/help validation text) with the
// explicit expandedCopy.ts fixture's genuinely >=2.0x-expanded strings, at
// both required widths (900/1280) and both themes (light/dark).
//
// Two distinct long-text policies are proven, matching UI-SPEC's own split:
//   - shell: a bounded IDENTITY slot (TitleBar's projectName) truncates
//     with ellipsis and never grows its own fixed chrome, but the complete
//     name remains accessible via its title attribute.
//   - dialog/shared-states/guided-first-show/status/field-help: a MESSAGE
//     surface wraps across multiple lines, growing its own bounded
//     container without truncation, clipping, overlap, or lost safety
//     controls.
// Every case is recorded into evidence/expanded-copy.json (ratios, boxes,
// line counts, overflow results, environment, build SHA, assertions),
// mirroring Plan 13-30's startup-theme-font.json/error-boundary-fallback.
// json precedent.
import { test, expect, type Page } from "@playwright/test";
import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";

import { installHealthyBindings, settle, waitForFonts, expectSafetyClusterAvailable, findOverflowingControls } from "./helpers";
import { getBuildSha } from "./fixtures/startupProbe";
import {
  COPY_PAIRS,
  MINIMUM_EXPANSION_RATIO,
  assertPairsMeetMinimumExpansion,
  expansionRatio,
  FIELD_HELP_EXPANDED_PORT,
  type CopyPair,
} from "./fixtures/expandedCopy";

// Fail loudly, before any browser ever opens, if a registered pair doesn't
// meet the plan's required 2.0x expansion floor -- the plan's own <action>:
// "reject any pair below 2.0x."
assertPairsMeetMinimumExpansion();

const EVIDENCE_PATH = path.join(
  import.meta.dirname,
  "..",
  "..",
  ".planning",
  "phases",
  "13-unified-ui-design-system-and-automated-enforcement",
  "evidence",
  "expanded-copy.json",
);

const WIDTHS = [900, 1280] as const;
const HEIGHT = 720;
type Theme = "light" | "dark";
const THEMES: Theme[] = ["light", "dark"];

interface EvidenceCase {
  pairId: string;
  category: string;
  width: number;
  theme: Theme;
  expansionRatio: number;
  measurement: unknown;
  assertions: Record<string, boolean>;
  passed: boolean;
}

const evidenceCases: EvidenceCase[] = [];
function recordCase(entry: EvidenceCase): void {
  evidenceCases.push(entry);
}

function pairFor(id: string): CopyPair {
  const pair = COPY_PAIRS.find((candidate) => candidate.id === id);
  if (!pair) throw new Error(`expandedCopy fixture is missing pair "${id}"`);
  return pair;
}

// theme.ts's own STORAGE_KEY ("golc-theme") -- seeded before navigation so
// main.tsx's applyTheme(getStoredTheme()) boots directly into the
// requested face, mirroring startupProbe.ts's identical convention.
async function withTheme(page: Page, theme: Theme): Promise<void> {
  await page.addInitScript((seededTheme: string) => {
    window.localStorage.setItem("golc-theme", seededTheme);
  }, theme);
}

interface WrapMeasurement {
  lineCount: number;
  textContent: string;
  clientHeight: number;
  scrollHeight: number;
  clientWidth: number;
  scrollWidth: number;
  rect: { top: number; left: number; right: number; bottom: number; width: number; height: number };
}

// measureTextWrap uses a Range over the element's own text content (not
// getBoundingClientRect on the element itself, which always reports one box
// for a block element regardless of how many lines it actually wrapped
// across) -- Range.getClientRects() returns one rect per rendered line box,
// the real way to prove multi-line growth actually happened rather than
// merely asserting a taller bounding box could mean anything.
async function measureTextWrap(page: Page, selector: string): Promise<WrapMeasurement | null> {
  return page.evaluate((sel) => {
    const el = document.querySelector(sel);
    if (!el) return null;
    const range = document.createRange();
    range.selectNodeContents(el);
    const rects = Array.from(range.getClientRects());
    const rect = el.getBoundingClientRect();
    return {
      lineCount: rects.length,
      textContent: el.textContent ?? "",
      clientHeight: el.clientHeight,
      scrollHeight: el.scrollHeight,
      clientWidth: el.clientWidth,
      scrollWidth: el.scrollWidth,
      rect: { top: rect.top, left: rect.left, right: rect.right, bottom: rect.bottom, width: rect.width, height: rect.height },
    };
  }, selector);
}

// measureTextWrapByContent is measureTextWrap's counterpart for a repeated
// element shape with no distinguishing class per instance (GuideEvidenceList
// renders every row's own detail as the identical `.evidenceDetail`
// class) -- finds the one instance whose own text content contains
// `needle` rather than requiring a unique CSS selector.
async function measureTextWrapByContent(
  page: Page,
  itemSelector: string,
  needle: string,
): Promise<WrapMeasurement | null> {
  return page.evaluate(
    ({ sel, text }) => {
      const candidates = Array.from(document.querySelectorAll(sel));
      const el = candidates.find((candidate) => (candidate.textContent ?? "").includes(text));
      if (!el) return null;
      const range = document.createRange();
      range.selectNodeContents(el);
      const rects = Array.from(range.getClientRects());
      const rect = el.getBoundingClientRect();
      return {
        lineCount: rects.length,
        textContent: el.textContent ?? "",
        clientHeight: el.clientHeight,
        scrollHeight: el.scrollHeight,
        clientWidth: el.clientWidth,
        scrollWidth: el.scrollWidth,
        rect: { top: rect.top, left: rect.left, right: rect.right, bottom: rect.bottom, width: rect.width, height: rect.height },
      };
    },
    { sel: itemSelector, text: needle },
  );
}

// assertNoTextOverlap mirrors helpers.ts's own expectTopBarTextToBeReadable
// algorithm, generalized to any container: every visible text-bearing
// descendant's own box must never intersect a sibling's.
async function findTextOverlaps(page: Page, containerSelector: string): Promise<string[]> {
  return page.evaluate((sel) => {
    const container = document.querySelector(sel);
    if (!container) return [`container "${sel}" not found`];
    const textElements = [container, ...container.querySelectorAll<HTMLElement>("*")].filter((element) => {
      const hasDirectText = [...element.childNodes].some(
        (node) => node.nodeType === Node.TEXT_NODE && node.textContent?.trim(),
      );
      if (!hasDirectText) return false;
      const style = window.getComputedStyle(element);
      const rect = element.getBoundingClientRect();
      return style.visibility !== "hidden" && style.display !== "none" && rect.width > 0 && rect.height > 0;
    });

    const failures: string[] = [];
    for (let i = 0; i < textElements.length; i += 1) {
      const a = textElements[i];
      const aRect = a.getBoundingClientRect();
      for (let j = i + 1; j < textElements.length; j += 1) {
        const b = textElements[j];
        if (a.contains(b) || b.contains(a)) continue;
        const bRect = b.getBoundingClientRect();
        const iw = Math.min(aRect.right, bRect.right) - Math.max(aRect.left, bRect.left);
        const ih = Math.min(aRect.bottom, bRect.bottom) - Math.max(aRect.top, bRect.top);
        if (iw > 1 && ih > 1) failures.push("two text elements visually overlap");
      }
    }
    return [...new Set(failures)];
  }, containerSelector);
}

// ---------------------------------------------------------------------------
// Category 1: shell (TitleBar project-name identity slot -- expected to
// truncate with ellipsis, never grow its own fixed chrome, while the full
// value remains accessible via its title attribute).
// ---------------------------------------------------------------------------

async function gotoShell(page: Page, theme: Theme, showName: string): Promise<void> {
  await installHealthyBindings(page);
  await withTheme(page, theme);
  await page.addInitScript((name: string) => {
    const bw = window as unknown as { go: { wails: { ShowService: Record<string, unknown> } } };
    bw.go.wails.ShowService.Inspect = async () => ({
      showPath: `C:\\Shows\\${name}.golc`,
      schemaVersion: 1,
      revision: 1,
      // Non-empty pools/deployments: OverviewWorkspace.tsx's own
      // requestAutoLaunch condition ("a genuinely empty show" -- non-empty
      // showPath AND zero pools AND zero deployments AND zero scenes)
      // would otherwise auto-open the Guided First Show overlay instead of
      // rendering Overview, which is not what this shell-identity category
      // is exercising.
      pools: [{ id: "pool-1", name: "Shell Test Pool", requiredCapabilities: [], memberCount: 1 }],
      deployments: [{ id: "deployment-1", name: "Shell Test Rig", active: true, instanceCount: 1 }],
    });
  }, showName);
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Overview", exact: true })).toBeVisible();
  await settle(page);
}

async function assertShellCategory(page: Page, pair: CopyPair, width: number, theme: Theme): Promise<void> {
  const selector = 'div[class*="titleBar"] span[class*="projectName"]';
  const info = await measureTextWrap(page, selector);
  expect(info, "TitleBar's project-name element must render").not.toBeNull();
  const titleAttr = await page.locator(selector).getAttribute("title");

  const fullTextPresent = info!.textContent.trim() === pair.expanded;
  const titleMatchesExpanded = titleAttr === pair.expanded;
  const visuallyTruncated = info!.scrollWidth > info!.clientWidth + 1;

  expect(fullTextPresent, "the full expanded show name must be present in the element's own text content").toBe(true);
  expect(
    titleMatchesExpanded,
    "the complete un-truncated name must remain accessible via the title attribute (UI-SPEC Accessibility Contract)",
  ).toBe(true);
  expect(
    visuallyTruncated,
    "an oversized name must actually be visually truncated (ellipsis) -- proving the fixed titlebar chrome does not grow to fit it",
  ).toBe(true);

  const offenders = await findOverflowingControls(page);
  expect(offenders, `shell at ${width}px/${theme} must not overflow`).toEqual([]);
  await expectSafetyClusterAvailable(page);

  recordCase({
    pairId: pair.id,
    category: pair.category,
    width,
    theme,
    expansionRatio: expansionRatio(pair),
    measurement: info,
    assertions: { fullTextPresent, titleMatchesExpanded, visuallyTruncated, noOverflow: offenders.length === 0 },
    passed: fullTextPresent && titleMatchesExpanded && visuallyTruncated && offenders.length === 0,
  });
}

// ---------------------------------------------------------------------------
// Category 2: dialog (Notes' destructive ConfirmModal impact message --
// expected to wrap fully, never truncate).
// ---------------------------------------------------------------------------

async function gotoDialog(page: Page, theme: Theme, noteTitle: string): Promise<void> {
  await installHealthyBindings(page);
  await withTheme(page, theme);
  await page.addInitScript((title: string) => {
    const bw = window as unknown as { go: { wails: { NotesService: Record<string, unknown> } } };
    bw.go.wails.NotesService.ListNotes = async () => [{ id: "note-1", title }];
    bw.go.wails.NotesService.GetNote = async () => ({ id: "note-1", title, body: "" });
  }, noteTitle);
  await page.goto("/");
  await page.getByRole("button", { name: "Notes", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Notes", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Delete Note", exact: true }).click();
  await expect(page.getByRole("alertdialog", { name: "Delete Note" })).toBeVisible();
}

async function assertDialogCategory(page: Page, pair: CopyPair, width: number, theme: Theme): Promise<void> {
  const dialog = page.getByRole("alertdialog", { name: "Delete Note" });
  const selector = 'div[role="alertdialog"] p[class*="message"]';
  const info = await measureTextWrap(page, selector);
  expect(info, "the confirm dialog's own message paragraph must render").not.toBeNull();

  const fullTextPresent = info!.textContent.includes(pair.expanded);
  const noClipping = info!.scrollHeight <= info!.clientHeight + 2 && info!.scrollWidth <= info!.clientWidth + 2;
  const grewMultiline = info!.lineCount > 1;
  const dialogBox = await dialog.boundingBox();
  const viewport = page.viewportSize();
  const containedInViewport =
    dialogBox !== null &&
    viewport !== null &&
    dialogBox.x >= -2 &&
    dialogBox.y >= -2 &&
    dialogBox.x + dialogBox.width <= viewport.width + 2 &&
    dialogBox.y + dialogBox.height <= viewport.height + 2;

  expect(fullTextPresent, "the complete expanded impact message must be present, never truncated").toBe(true);
  expect(noClipping, "the message must not be clipped by its own paragraph box").toBe(true);
  expect(grewMultiline, "a genuinely 2x-expanded message must wrap across more than one line").toBe(true);
  expect(containedInViewport, "the dialog itself must stay fully inside the viewport at every width/theme").toBe(true);

  const overlaps = await findTextOverlaps(page, 'div[role="alertdialog"]');
  expect(overlaps, "no text inside the dialog may visually overlap another element").toEqual([]);

  // Focus reachability: Cancel autofocuses on open (ConfirmModal.tsx); both
  // actions must stay reachable and operable regardless of message length.
  const cancelButton = dialog.getByRole("button", { name: "Cancel", exact: true });
  const confirmButton = dialog.getByRole("button", { name: "Delete Note", exact: true });
  await expect(cancelButton).toBeFocused();
  await expect(confirmButton).toBeVisible();
  await expect(confirmButton).toBeEnabled();

  await expectSafetyClusterAvailable(page);
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);

  recordCase({
    pairId: pair.id,
    category: pair.category,
    width,
    theme,
    expansionRatio: expansionRatio(pair),
    measurement: { ...info, dialogBox },
    assertions: { fullTextPresent, noClipping, grewMultiline, containedInViewport, noOverlaps: overlaps.length === 0 },
    passed: fullTextPresent && noClipping && grewMultiline && containedInViewport && overlaps.length === 0,
  });
}

// ---------------------------------------------------------------------------
// Category 3: shared-states (Diagnostics' Integrity Check structuralError --
// a shared error-state surface, expected to wrap and preserve the complete
// actionable message).
// ---------------------------------------------------------------------------

async function gotoSharedStates(page: Page, theme: Theme, structuralError: string): Promise<void> {
  await installHealthyBindings(page);
  await withTheme(page, theme);
  await page.addInitScript((message: string) => {
    const bw = window as unknown as { go: { wails: { ShowService: Record<string, unknown> } } };
    bw.go.wails.ShowService.Diagnose = async () => ({
      fileLevelIssues: [],
      structuralOk: false,
      structuralError: message,
      migrationRequired: false,
      schemaVersion: 1,
      revision: 1,
    });
  }, structuralError);
  await page.goto("/");
  await page.getByRole("button", { name: "Diagnostics", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Diagnostics", exact: true })).toBeVisible();
  await settle(page);
}

async function assertWrappingParagraphCategory(
  page: Page,
  pair: CopyPair,
  width: number,
  theme: Theme,
  selector: string,
  containerSelector: string,
): Promise<void> {
  const info = await measureTextWrap(page, selector);
  expect(info, `"${selector}" must render`).not.toBeNull();

  const fullTextPresent = info!.textContent.includes(pair.expanded);
  const noClipping = info!.scrollHeight <= info!.clientHeight + 2 && info!.scrollWidth <= info!.clientWidth + 2;
  const grewMultiline = info!.lineCount > 1;

  expect(fullTextPresent, "the complete expanded message must be present, never truncated").toBe(true);
  expect(noClipping, "the message must not be clipped by its own box").toBe(true);
  expect(grewMultiline, "a genuinely 2x-expanded message must wrap across more than one line").toBe(true);

  const overlaps = await findTextOverlaps(page, containerSelector);
  expect(overlaps, "no text inside the container may visually overlap another element").toEqual([]);

  const offenders = await findOverflowingControls(page);
  expect(offenders, `${pair.category} at ${width}px/${theme} must not overflow (body/appShell never become the scroll owner)`).toEqual(
    [],
  );
  await expectSafetyClusterAvailable(page);

  recordCase({
    pairId: pair.id,
    category: pair.category,
    width,
    theme,
    expansionRatio: expansionRatio(pair),
    measurement: info,
    assertions: {
      fullTextPresent,
      noClipping,
      grewMultiline,
      noOverlaps: overlaps.length === 0,
      noOverflow: offenders.length === 0,
    },
    passed: fullTextPresent && noClipping && grewMultiline && overlaps.length === 0 && offenders.length === 0,
  });
}

// ---------------------------------------------------------------------------
// Category 4: guided-first-show (Assign stage evidence list -- the guide's
// own always-mounted rail/inspector/safety chrome plus a real, notably
// longer evidence row rendered next to a shorter one, both already-shipped
// copy -- see expandedCopy.ts's own doc comment on this pair for why no
// mocked error string is used).
// ---------------------------------------------------------------------------

async function gotoGuidedFirstShow(page: Page, theme: Theme): Promise<void> {
  await installHealthyBindings(page);
  await withTheme(page, theme);
  await page.goto("/");
  await page.getByRole("button", { name: "Start Guide" }).click();
  await expect(page.getByRole("navigation", { name: "First show steps" })).toBeVisible();
  // The rail's own ListRow items are directly selectable regardless of
  // stage-completion order (GuidedFirstShow.tsx: onSelect={() =>
  // setStage(id)}), so this jumps straight to Assign rather than stepping
  // through Fixtures/Patch/Program first.
  await page.getByRole("navigation", { name: "First show steps" }).getByText("Assign", { exact: true }).click();
  await expect(page.getByRole("heading", { name: "Assign", exact: true, level: 2 })).toBeVisible();
  await settle(page);
}

async function assertGuidedFirstShowCategory(page: Page, pair: CopyPair, width: number, theme: Theme): Promise<void> {
  const itemSelector = 'p[class*="evidenceDetail"]';
  const info = await measureTextWrapByContent(page, itemSelector, pair.expanded);
  expect(info, "the Assign stage's MIDI-hardware evidence detail row must render").not.toBeNull();

  const canonicalInfo = await measureTextWrapByContent(page, itemSelector, pair.canonical);
  expect(canonicalInfo, "the Assign stage's shorter blocker detail row must also render alongside it").not.toBeNull();

  // GuidedFirstShow.module.css's own documented "safety valve": below
  // stageSection's 460px floor, .contentArea deliberately collapses
  // evidenceAside toward 0 width FIRST (a pre-existing, reviewed tradeoff,
  // not something introduced by this expanded-copy fixture -- the SAME
  // collapse reproduces identically for the short canonical row, proving
  // it is width-driven, not content-length-driven) and only once that is
  // exhausted does .contentArea itself scroll horizontally. When the aside
  // has genuinely collapsed to that documented floor, the guaranteed
  // invariant is "nothing is silently lost, and contentArea (not the
  // document/body) is the one that scrolls" -- not that the paragraph
  // renders wrapped at a comfortable width, which the aside's own box
  // cannot offer in that state. Above the floor, the ordinary "wraps
  // without clipping" invariant applies normally.
  const asideCollapsed = await page.evaluate(() => {
    const aside = document.querySelector<HTMLElement>('[class*="evidenceAside"]');
    return aside ? aside.clientWidth < 100 : false;
  });

  const fullTextPresent = info!.textContent.includes(pair.expanded);
  expect(fullTextPresent, "the complete, notably longer evidence detail must be present, never truncated").toBe(true);

  let noClipping = true;
  let grewMultiline = true;
  let contentAreaScrolls = true;
  if (asideCollapsed) {
    contentAreaScrolls = await page.evaluate(() => {
      const contentArea = document.querySelector<HTMLElement>('[class*="contentArea"]');
      return contentArea ? contentArea.scrollWidth > contentArea.clientWidth + 1 : false;
    });
    expect(
      contentAreaScrolls,
      "when evidenceAside collapses toward its documented narrow-width floor, contentArea's own overflow-x:auto must be the compensating scroll owner",
    ).toBe(true);
  } else {
    noClipping = info!.scrollHeight <= info!.clientHeight + 2 && info!.scrollWidth <= info!.clientWidth + 2;
    grewMultiline = info!.lineCount > 1;
    expect(noClipping, "the evidence detail must not be clipped by its own box").toBe(true);
    expect(grewMultiline, "a genuinely 2x-longer evidence row must wrap across more than one line").toBe(true);
  }

  const overlaps = asideCollapsed ? [] : await findTextOverlaps(page, 'ul[class*="evidenceList"]');
  expect(overlaps, "no text inside the evidence list may visually overlap another element").toEqual([]);

  const offenders = await findOverflowingControls(page);
  expect(
    offenders,
    `guided-first-show at ${width}px/${theme} must not overflow (persistent rail/inspector/safety chrome stays mounted)`,
  ).toEqual([]);
  await expectSafetyClusterAvailable(page);
  await expect(page.getByRole("navigation", { name: "First show steps" })).toBeVisible();

  recordCase({
    pairId: pair.id,
    category: pair.category,
    width,
    theme,
    expansionRatio: expansionRatio(pair),
    measurement: { expandedRow: info, canonicalRow: canonicalInfo, asideCollapsed },
    assertions: {
      fullTextPresent,
      noClipping,
      grewMultiline,
      contentAreaScrolls,
      noOverlaps: overlaps.length === 0,
      noOverflow: offenders.length === 0,
    },
    passed: fullTextPresent && noClipping && grewMultiline && contentAreaScrolls && overlaps.length === 0 && offenders.length === 0,
  });
}

// ---------------------------------------------------------------------------
// Category 5: status (Diagnostics' live Application Log stream row).
// ---------------------------------------------------------------------------

async function gotoStatus(page: Page, theme: Theme, logMessage: string): Promise<void> {
  await installHealthyBindings(page);
  await withTheme(page, theme);
  await page.addInitScript((message: string) => {
    const bw = window as unknown as { go: { wails: { App?: Record<string, unknown> } } };
    bw.go.wails.App = {
      RecentAppLogs: async () => [
        { seq: 1, level: "info", source: "daemon", message, at: new Date(2026, 0, 1).toISOString() },
      ],
    };
  }, logMessage);
  await page.goto("/");
  await page.getByRole("button", { name: "Diagnostics", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Diagnostics", exact: true })).toBeVisible();
  await settle(page);
}

// ---------------------------------------------------------------------------
// Category 6: field-help (ArtnetConfig's real, user-typed-length-driven
// GOLC_ARTNET_USAGE inline validation message).
// ---------------------------------------------------------------------------

async function gotoFieldHelp(page: Page, theme: Theme): Promise<void> {
  await installHealthyBindings(page);
  await withTheme(page, theme);
  await page.addInitScript(() => {
    const bw = window as unknown as {
      go: { wails: { FixturePatchService: Record<string, unknown>; ArtnetConfigService: Record<string, unknown> } };
    };
    bw.go.wails.FixturePatchService.ListPatch = async () => ({
      pools: [{ id: "p1", name: "Field Help Pool", members: [{ id: "m1", fixtureStableKey: "x", fixtureContentHash: "x" }] }],
      deployments: [
        {
          id: "d1",
          name: "Field Help Rig",
          active: true,
          instances: [{ id: "i1", poolId: "p1", poolMemberId: "m1", mode: "RGBW", universe: 1, address: 1 }],
        },
      ],
    });
    bw.go.wails.ArtnetConfigService.FetchArtnetStatus = async () => ({
      reachable: true,
      interface: { pinnedIndex: 1, pinnedName: "Ethernet", status: "ready", error: "" },
      targets: [],
    });
  });
  await page.goto("/");
  await page.getByRole("button", { name: "Art-Net", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Art-Net", exact: true })).toBeVisible();
  await settle(page);
}

async function assertFieldHelpCategory(page: Page, pair: CopyPair, width: number, theme: Theme): Promise<void> {
  // ArtnetConfig.tsx's handleAddTarget checks the IP field first (a
  // required non-empty value) before it ever reaches the port-range
  // validation this category exercises -- a real IP must be present or
  // the earlier "An IP address is required" guard fires instead.
  await page.getByLabel("Universe 1 target IP address").fill("192.168.1.50");
  const portField = page.getByLabel("Universe 1 target port (optional)");
  await portField.fill(FIELD_HELP_EXPANDED_PORT);
  await page.getByRole("button", { name: "Add Target", exact: true }).click();

  const selector = 'section[class*="errorState"] p[class*="message"]';
  await expect(page.locator(selector)).toBeVisible();
  const info = await measureTextWrap(page, selector);
  expect(info, "the field-help inline validation message must render").not.toBeNull();

  const fullTextPresent = info!.textContent.includes(pair.expanded);
  const noClipping = info!.scrollHeight <= info!.clientHeight + 2 && info!.scrollWidth <= info!.clientWidth + 2;
  const grewMultiline = info!.lineCount > 1;

  expect(fullTextPresent, "the complete GOLC_ARTNET_USAGE message (scaled by the typed port length) must be present").toBe(true);
  expect(noClipping, "the validation message must not be clipped by its own box").toBe(true);
  expect(grewMultiline, "a genuinely 2x-expanded validation message must wrap across more than one line").toBe(true);

  const overlaps = await findTextOverlaps(page, 'section[class*="errorState"]');
  expect(overlaps, "no text inside the error state may visually overlap another element").toEqual([]);

  // The port Field itself must remain focusable/visible after the failed
  // submission (field-level focus reachability, not just the page-level
  // safety cluster).
  await expect(portField).toBeVisible();
  await portField.focus();
  await expect(portField).toBeFocused();

  const offenders = await findOverflowingControls(page);
  expect(offenders, `field-help at ${width}px/${theme} must not overflow`).toEqual([]);
  await expectSafetyClusterAvailable(page);

  recordCase({
    pairId: pair.id,
    category: pair.category,
    width,
    theme,
    expansionRatio: expansionRatio(pair),
    measurement: info,
    assertions: {
      fullTextPresent,
      noClipping,
      grewMultiline,
      noOverlaps: overlaps.length === 0,
      noOverflow: offenders.length === 0,
    },
    passed: fullTextPresent && noClipping && grewMultiline && overlaps.length === 0 && offenders.length === 0,
  });
}

// ---------------------------------------------------------------------------
// Test registration
// ---------------------------------------------------------------------------

test.describe("Category 1: shell (TitleBar identity)", () => {
  const pair = pairFor("shell-titlebar-project-name");
  for (const width of WIDTHS) {
    for (const theme of THEMES) {
      test(`shell identity at ${width}px, ${theme}`, async ({ page }) => {
        await page.setViewportSize({ width, height: HEIGHT });
        await gotoShell(page, theme, pair.expanded);
        await waitForFonts(page);
        await assertShellCategory(page, pair, width, theme);
      });
    }
  }
});

test.describe("Category 2: dialog (Notes destructive confirm)", () => {
  const pair = pairFor("dialog-notes-delete-confirm");
  for (const width of WIDTHS) {
    for (const theme of THEMES) {
      test(`dialog impact message at ${width}px, ${theme}`, async ({ page }) => {
        await page.setViewportSize({ width, height: HEIGHT });
        await gotoDialog(page, theme, pair.expanded);
        await waitForFonts(page);
        await assertDialogCategory(page, pair, width, theme);
      });
    }
  }
});

test.describe("Category 3: shared-states (Diagnostics structural error)", () => {
  const pair = pairFor("shared-states-diagnostics-structural-error");
  for (const width of WIDTHS) {
    for (const theme of THEMES) {
      test(`shared error state at ${width}px, ${theme}`, async ({ page }) => {
        await page.setViewportSize({ width, height: HEIGHT });
        await gotoSharedStates(page, theme, pair.expanded);
        await waitForFonts(page);
        await assertWrappingParagraphCategory(
          page,
          pair,
          width,
          theme,
          'p[class*="structuralDetail"]',
          'div[class*="workspace"]',
        );
      });
    }
  }
});

test.describe("Category 4: guided-first-show (Assign stage evidence list)", () => {
  const pair = pairFor("guided-first-show-assign-stage-evidence");
  for (const width of WIDTHS) {
    for (const theme of THEMES) {
      test(`guided evidence at ${width}px, ${theme}`, async ({ page }) => {
        await page.setViewportSize({ width, height: HEIGHT });
        await gotoGuidedFirstShow(page, theme);
        await waitForFonts(page);
        await assertGuidedFirstShowCategory(page, pair, width, theme);
      });
    }
  }
});

test.describe("Category 5: status (Diagnostics application log row)", () => {
  const pair = pairFor("status-diagnostics-app-log-row");
  for (const width of WIDTHS) {
    for (const theme of THEMES) {
      test(`status log copy at ${width}px, ${theme}`, async ({ page }) => {
        await page.setViewportSize({ width, height: HEIGHT });
        await gotoStatus(page, theme, pair.expanded);
        await waitForFonts(page);
        await assertWrappingParagraphCategory(
          page,
          pair,
          width,
          theme,
          'li[class*="logRow"] span[class*="rowText"]',
          'ul[class*="streamList"]',
        );
      });
    }
  }
});

test.describe("Category 6: field-help (ArtnetConfig port validation)", () => {
  const pair = pairFor("field-help-artnet-port-usage");
  for (const width of WIDTHS) {
    for (const theme of THEMES) {
      test(`field-help validation message at ${width}px, ${theme}`, async ({ page }) => {
        await page.setViewportSize({ width, height: HEIGHT });
        await gotoFieldHelp(page, theme);
        await waitForFonts(page);
        await assertFieldHelpCategory(page, pair, width, theme);
      });
    }
  }
});

// ---------------------------------------------------------------------------
// Evidence
// ---------------------------------------------------------------------------

test.afterAll(() => {
  const evidence = {
    schemaVersion: 1,
    capturedAt: new Date().toISOString(),
    buildSha: getBuildSha(),
    environment: {
      platform: process.platform,
      widths: WIDTHS,
      themes: THEMES,
      height: HEIGHT,
    },
    minimumExpansionRatio: MINIMUM_EXPANSION_RATIO,
    pairs: COPY_PAIRS.map((pair) => ({
      id: pair.id,
      category: pair.category,
      canonical: pair.canonical,
      expanded: pair.expanded,
      expansionRatio: expansionRatio(pair),
    })),
    cases: evidenceCases,
    allCasesPassed: evidenceCases.every((entry) => entry.passed),
  };

  mkdirSync(path.dirname(EVIDENCE_PATH), { recursive: true });
  writeFileSync(EVIDENCE_PATH, `${JSON.stringify(evidence, null, 2)}\n`);
});
