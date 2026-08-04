import { describe, expect, it } from "vitest";

import inventory from "../../design-system/components.json";
import exceptionsManifest from "../../design-system/exceptions.json";
import * as designSystem from "./index";

type InventoryRecord = { name: string; kind: "primitive" | "pattern"; exportPath: string; guideAnchor: string; testPath: string };
type ExceptionRecord = { path: string; source: string };

describe("public design-system inventory", () => {
  it("has one inventory identity for every public export and guide marker", async () => {
    const components = inventory.components as InventoryRecord[];

    expect(components.length).toBeGreaterThan(20);
    expect(new Set(components.map((component) => component.name)).size).toBe(components.length);
    for (const component of components) {
      expect(designSystem).toHaveProperty(component.name);
    }
    expect(Object.keys(designSystem).sort()).toEqual(components.map((component) => component.name).sort());
  });
});

// ConfirmModal was removed in Phase 13 Plan 37: Dialog/ConfirmDialog are now
// the only public confirmation contracts. These assertions enumerate every
// compatibility seam a silent regression could reintroduce -- directory,
// barrel export, inventory record, exception-manifest entry, guide
// reference, and stray source import/alias -- rather than relying on a
// single file-existence check. Uses Vite's import.meta.glob (not Node's
// fs) so this stays valid under src/'s browser-scoped tsconfig, which has
// no @types/node.
describe("ConfirmModal removal", () => {
  it("has no ConfirmModal directory on disk", () => {
    const confirmModalFiles = import.meta.glob("../components/primitives/ConfirmModal/**");
    expect(Object.keys(confirmModalFiles)).toEqual([]);
  });

  it("does not export ConfirmModal from the public design-system barrel", () => {
    expect(designSystem).not.toHaveProperty("ConfirmModal");
  });

  it("has no ConfirmModal record, alias, or duplicate-authority entry in the component inventory", () => {
    const components = inventory.components as InventoryRecord[];
    expect(components.some((component) => component.name === "ConfirmModal")).toBe(false);
    expect(components.some((component) => component.exportPath.includes("ConfirmModal"))).toBe(false);
    // Dialog/ConfirmDialog remain the sole public confirmation contracts.
    expect(components.some((component) => component.name === "ConfirmDialog")).toBe(true);
    expect(components.some((component) => component.name === "Dialog")).toBe(true);
  });

  it("has no ConfirmModal entry in the exception manifest", () => {
    const exceptions = exceptionsManifest.records as ExceptionRecord[];
    for (const record of exceptions) {
      expect(record.path).not.toMatch(/ConfirmModal/);
      expect(record.source).not.toMatch(/ConfirmModal/);
    }
  });

  it("has no ConfirmModal reference in the design-system guide", async () => {
    const guideFiles = import.meta.glob("../../DESIGN_SYSTEM.md", { query: "?raw", import: "default" });
    const loaders = Object.values(guideFiles);
    expect(loaders.length).toBe(1);
    const guide = (await loaders[0]()) as string;
    expect(guide).not.toMatch(/ConfirmModal/);
  });

  it("has no ConfirmModal import, re-export, or alias anywhere in application source", async () => {
    const sourceFiles = import.meta.glob("../**/*.{ts,tsx}", { query: "?raw", import: "default" });
    const offenders: string[] = [];
    for (const [file, loader] of Object.entries(sourceFiles)) {
      if (file.endsWith("design-system.contract.test.ts")) continue; // this file names it deliberately
      const content = (await loader()) as string;
      if (content.includes("ConfirmModal")) {
        offenders.push(file);
      }
    }
    expect(offenders).toEqual([]);
  });
});
