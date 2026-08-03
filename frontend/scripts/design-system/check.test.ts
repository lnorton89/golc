import { afterEach, describe, expect, it } from "vitest";
import { mkdtemp, mkdir, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { checkFiles } from "./check.mjs";

const roots: string[] = [];

async function fixture(files: Record<string, string>): Promise<string> {
  const root = await mkdtemp(join(tmpdir(), "golc-ds-policy-"));
  roots.push(root);
  for (const [path, source] of Object.entries(files)) {
    const target = join(root, path);
    await mkdir(join(target, ".."), { recursive: true });
    await writeFile(target, source, "utf8");
  }
  return root;
}

afterEach(async () => { await Promise.all(roots.splice(0).map((root) => rm(root, { recursive: true, force: true }))); });

describe("DS001-DS010 policy boundaries", () => {
  it.each([
    ["DS001", "src/Feature.module.css", ".panel { color: #fff; }", "raw visual literal"],
    ["DS002", "src/Feature.module.css", ".panel { --feature-color: red; }", "custom property declaration"],
    ["DS003", "src/Feature.module.css", ".panel { color: var(--missing); }", "unknown custom property"],
    ["DS004", "src/Feature.module.css", "[data-theme='dark'] .panel {}", "theme selector"],
    ["DS005", "src/Feature.tsx", "export const Feature = () => <button className='button'>Go</button>;", "styled native control"],
    ["DS006", "src/Feature.module.css", ".primaryButton { padding: 8px; }", "shared visual class"],
    ["DS007", "design-system/components.json", '{"schemaVersion":1,"components":[{"name":"Broken"}', "inventory"],
    ["DS008", "design-system/exceptions.json", '{"schemaVersion":1,"records":[{"path":"src/Feature.module.css"}]}', "exception"],
    ["DS009", "design-system/tokens.json", '{"schemaVersion":1}', "theme contract"],
    ["DS010", "src/Feature.module.css", ".button { outline: none; }", "focus"],
  ])("%s rejects its forbidden fixture", async (rule, path, source, expected) => {
    const root = await fixture({ [path]: source });
    const result = await checkFiles(root, [path]);
    expect(result.diagnostics.some((diagnostic) => diagnostic.rule === rule && diagnostic.message.includes(expected))).toBe(true);
  });

  it("accepts semantic CSS and native semantics inside a public primitive", async () => {
    const root = await fixture({
      "src/Feature.module.css": ".panel { color: var(--ds-text-primary); padding: var(--ds-space-2); }",
      "src/components/primitives/Button/Button.tsx": "export const Button = () => <button className='button'>Go</button>;",
    });
    expect((await checkFiles(root, ["src/Feature.module.css", "src/components/primitives/Button/Button.tsx"])).diagnostics).toEqual([]);
  });

  it("fails closed with stable diagnostics for malformed source and unresolved paths", async () => {
    const root = await fixture({ "src/Feature.module.css": ".panel { color: red" });
    const first = await checkFiles(root, ["src/Feature.module.css", "src/missing.tsx"]);
    const second = await checkFiles(root, ["src/missing.tsx", "src/Feature.module.css"]);
    expect(first.diagnostics).toEqual(second.diagnostics);
    expect(first.diagnostics.every((diagnostic) => diagnostic.rule === "DS000" || diagnostic.path.startsWith("src/"))).toBe(true);
  });
});
