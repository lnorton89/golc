import { describe, expect, it } from "vitest";

import inventory from "../../design-system/components.json";
import * as designSystem from "./index";

type InventoryRecord = { name: string; kind: "primitive" | "pattern"; exportPath: string; guideAnchor: string; testPath: string };

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
