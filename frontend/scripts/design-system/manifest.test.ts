import { cp, mkdtemp, mkdir, readFile, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { loadDesignSystem } from "./manifest.mjs";
import { generateDesignSystem } from "./generate.mjs";

const fixtureRoots: string[] = [];

async function fixture(): Promise<string> {
  const root = await mkdtemp(join(tmpdir(), "golc-design-system-"));
  fixtureRoots.push(root);
  await mkdir(join(root, "design-system", "schema"), { recursive: true });
  await cp(join(process.cwd(), "design-system"), join(root, "design-system"), { recursive: true });
  return root;
}

// Only generateDesignSystem's own round-trip test needs this: it loads the
// REAL, unmodified design-system/exceptions.json (Plan 13-19 merged Wave
// 7-10's evidence-backed domain exceptions there), whose record.path
// values are real repo-relative src/... files that assertContainedPath
// must lstat successfully. The other fixture()-based tests below always
// overwrite exceptions.json with their own dummy records before calling
// loadDesignSystem, so they never need a src/ copy.
async function fixtureWithSource(): Promise<string> {
  const root = await fixture();
  await cp(join(process.cwd(), "src"), join(root, "src"), { recursive: true });
  return root;
}

afterEach(async () => {
  await Promise.all(fixtureRoots.splice(0).map(async (root) => {
    const { rm } = await import("node:fs/promises");
    await rm(root, { recursive: true, force: true });
  }));
});

describe("loadDesignSystem", () => {
  it("loads the closed, complete manifest authority", async () => {
    const authority = await loadDesignSystem(process.cwd());
    // Wave 7-10 migration plans' evidence-backed domain exceptions are
    // merged here (Plan 13-19) -- exceptions.json is no longer empty, but
    // every record still validates against validateExceptions' strict
    // 7-key schema (asserted structurally above) and check.mjs --all
    // confirms each resolves to exactly one live diagnostic.
    expect(Array.isArray(authority.exceptions.records)).toBe(true);
    expect(authority.exceptions.records.length).toBeGreaterThan(0);
    expect(authority.tokens.themes).toHaveLength(24);
    expect(authority.tokens.themes.every((face: { tokens: Record<string, string> }) =>
      Object.keys(face.tokens).length === authority.tokens.semanticRoles.length,
    )).toBe(true);
    expect(authority.tokens.foundation.spacing.guidedFirstShowGap).toBe("8px");
    expect(authority.tokens.foundation.sizing.guidedFirstShowStageRail).toBe("210px");
  });

  it.each([
    ["unknown property", '{"schemaVersion":1,"records":[],"unknown":true}'],
    ["duplicate property", '{"schemaVersion":1,"records":[],"records":[]}'],
    ["absolute path", '{"schemaVersion":1,"records":[{"path":"C:/escape.css","rule":"DS001","match":"x","rationale":"x","source":"x","owner":"x","reviewCondition":"review"}]}'],
    ["parent traversal", '{"schemaVersion":1,"records":[{"path":"../escape.css","rule":"DS001","match":"x","rationale":"x","source":"x","owner":"x","reviewCondition":"review"}]}'],
  ])("fails closed for %s", async (_name, source) => {
    const root = await fixture();
    await writeFile(join(root, "design-system", "exceptions.json"), source);
    await expect(loadDesignSystem(root)).rejects.toThrow(/DSMANIFEST_/);
  });

  it("rejects a symlink that escapes the manifest root", async () => {
    const root = await fixture();
    const outside = await mkdtemp(join(tmpdir(), "golc-outside-"));
    fixtureRoots.push(outside);
    await writeFile(join(outside, "outside.css"), ".outside {}");
    await symlink(join(outside, "outside.css"), join(root, "design-system", "escape.css"));
    await writeFile(join(root, "design-system", "exceptions.json"), JSON.stringify({
      schemaVersion: 1,
      records: [{ path: "design-system/escape.css", rule: "DS001", match: ".outside", rationale: "fixture", source: "test", owner: "test", reviewCondition: "next review" }],
    }));
    await expect(loadDesignSystem(root)).rejects.toThrow(/DSMANIFEST_PATH_ESCAPE/);
  });
});

describe("generateDesignSystem", () => {
  it("is byte-stable and leaves checked output untouched in check mode", async () => {
    const root = await fixtureWithSource();
    await generateDesignSystem(root);
    const cssPath = join(root, "src", "design-system", "tokens.generated.css");
    const typesPath = join(root, "src", "design-system", "tokens.generated.ts");
    const first = await Promise.all([readFile(cssPath, "utf8"), readFile(typesPath, "utf8")]);
    await generateDesignSystem(root);
    expect(await Promise.all([readFile(cssPath, "utf8"), readFile(typesPath, "utf8")])).toEqual(first);
    await expect(generateDesignSystem(root, { check: true })).resolves.toEqual([]);
    await writeFile(cssPath, `${first[0]}/* drift */\n`);
    await expect(generateDesignSystem(root, { check: true })).rejects.toThrow(/DSMANIFEST_DRIFT/);
    expect(await readFile(cssPath, "utf8")).toContain("/* drift */");
  }, 30_000);
});
