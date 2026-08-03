import { lstat, readFile, realpath } from "node:fs/promises";
import { dirname, isAbsolute, relative, resolve, sep } from "node:path";

const MANIFEST_ROOT = "design-system";
const FILES = ["tokens", "components", "runtime-geometry", "exceptions"];
const THEME_NAMES = ["default", "gruvbox", "tokyo", "dracula", "nord", "catppuccin", "solarized", "one-dark", "rose-pine", "everforest", "rainbow", "acid"];
const MODES = ["light", "dark"];

function fail(code, detail) {
  throw new Error(`${code}: ${detail}`);
}

// JSON.parse intentionally accepts duplicate object keys. This small scanner walks
// the JSON grammar first so every manifest remains a one-owner authority.
function rejectDuplicateKeys(source, label) {
  let cursor = 0;
  const whitespace = () => { while (/\s/.test(source[cursor] ?? "")) cursor += 1; };
  const string = () => {
    if (source[cursor] !== '"') fail("DSMANIFEST_JSON", `${label} has an invalid string`);
    const start = cursor++;
    while (cursor < source.length) {
      if (source[cursor] === "\\") { cursor += 2; continue; }
      if (source[cursor++] === '"') return JSON.parse(source.slice(start, cursor));
    }
    fail("DSMANIFEST_JSON", `${label} has an unterminated string`);
  };
  const scalar = () => { while (cursor < source.length && !/[,}\]\s]/.test(source[cursor])) cursor += 1; };
  const value = () => {
    whitespace();
    if (source[cursor] === '"') { string(); return; }
    if (source[cursor] === "{") {
      cursor += 1; const keys = new Set(); whitespace();
      if (source[cursor] === "}") { cursor += 1; return; }
      while (true) {
        whitespace(); const key = string();
        if (keys.has(key)) fail("DSMANIFEST_DUPLICATE_KEY", `${label} duplicates ${key}`);
        keys.add(key); whitespace();
        if (source[cursor++] !== ":") fail("DSMANIFEST_JSON", `${label} has an invalid object`);
        value(); whitespace();
        if (source[cursor] === "}") { cursor += 1; return; }
        if (source[cursor++] !== ",") fail("DSMANIFEST_JSON", `${label} has an invalid object`);
      }
    }
    if (source[cursor] === "[") {
      cursor += 1; whitespace(); if (source[cursor] === "]") { cursor += 1; return; }
      while (true) { value(); whitespace(); if (source[cursor] === "]") { cursor += 1; return; } if (source[cursor++] !== ",") fail("DSMANIFEST_JSON", `${label} has an invalid array`); }
    }
    scalar();
  };
  value(); whitespace();
  if (cursor !== source.length) fail("DSMANIFEST_JSON", `${label} has trailing content`);
}

async function readJSON(root, name) {
  const path = resolve(root, MANIFEST_ROOT, `${name}.json`);
  const source = await readFile(path, "utf8");
  rejectDuplicateKeys(source, `${MANIFEST_ROOT}/${name}.json`);
  try { return JSON.parse(source); } catch { fail("DSMANIFEST_JSON", `${MANIFEST_ROOT}/${name}.json is invalid JSON`); }
}

function exactKeys(value, keys, label) {
  if (!value || typeof value !== "object" || Array.isArray(value)) fail("DSMANIFEST_SCHEMA", `${label} must be an object`);
  const got = Object.keys(value).sort(); const want = [...keys].sort();
  if (got.length !== want.length || got.some((key, index) => key !== want[index])) fail("DSMANIFEST_SCHEMA", `${label} has unknown or missing properties`);
}

function assertVersion(value, label, keys) {
  exactKeys(value, keys, label);
  if (value.schemaVersion !== 1) fail("DSMANIFEST_SCHEMA", `${label}.schemaVersion must equal 1`);
}

async function assertContainedPath(root, candidate, label) {
  if (typeof candidate !== "string" || candidate.length === 0 || isAbsolute(candidate) || candidate.includes("\\")) fail("DSMANIFEST_PATH", `${label} must be a repository-relative slash path`);
  const target = resolve(root, candidate);
  if (relative(root, target).startsWith("..") || relative(root, target) === "") fail("DSMANIFEST_PATH", `${label} escapes repository root`);
  try {
    const stat = await lstat(target);
    const resolved = stat.isSymbolicLink() ? await realpath(target) : target;
    if (relative(root, resolved).startsWith("..")) fail("DSMANIFEST_PATH_ESCAPE", `${label} resolves outside repository root`);
  } catch (error) {
    if (error?.message?.startsWith("DSMANIFEST_")) throw error;
    fail("DSMANIFEST_PATH", `${label} does not resolve`);
  }
}

function validateTokens(tokens) {
  assertVersion(tokens, "tokens", ["schemaVersion", "semanticRoles", "foundation", "palettes", "themes"]);
  if (!Array.isArray(tokens.semanticRoles) || tokens.semanticRoles.length === 0 || new Set(tokens.semanticRoles).size !== tokens.semanticRoles.length || !tokens.semanticRoles.every((role) => typeof role === "string" && /^[a-z]+(?:[.-][a-z]+)*$/.test(role))) fail("DSMANIFEST_SCHEMA", "tokens.semanticRoles must be unique semantic names");
  const faces = new Set();
  if (!Array.isArray(tokens.themes) || tokens.themes.length !== THEME_NAMES.length * MODES.length) fail("DSMANIFEST_THEME", "tokens.themes must declare every approved theme face");
  for (const face of tokens.themes) {
    exactKeys(face, ["name", "mode", "palette"], "tokens.themes[]");
    if (!THEME_NAMES.includes(face.name) || !MODES.includes(face.mode) || typeof face.palette !== "string") fail("DSMANIFEST_THEME", "tokens.themes contains an unsupported face");
    const id = `${face.name}/${face.mode}`; if (faces.has(id)) fail("DSMANIFEST_THEME", `tokens.themes duplicates ${id}`); faces.add(id);
    const palette = tokens.palettes[face.palette]; exactKeys(palette, tokens.semanticRoles, `palette ${face.palette}`);
    if (!Object.values(palette).every((value) => typeof value === "string" && value.length > 0)) fail("DSMANIFEST_THEME", `palette ${face.palette} has a non-string semantic value`);
  }
  for (const name of THEME_NAMES) for (const mode of MODES) if (!faces.has(`${name}/${mode}`)) fail("DSMANIFEST_THEME", `tokens.themes is missing ${name}/${mode}`);
  for (const value of Object.values(tokens.foundation.spacing ?? {})) if (!/^\d+px$/.test(value) || Number.parseInt(value, 10) % 4 !== 0) fail("DSMANIFEST_GRID", "foundation spacing must use the 4px grid");
  if (tokens.foundation.spacing?.guidedFirstShowGap !== "8px" || tokens.foundation.sizing?.guidedFirstShowStageRail !== "210px") fail("DSMANIFEST_GEOMETRY", "Guided First Show geometry must be 8px gap and 210px sizing rail");
}

async function validateExceptions(root, exceptions) {
  assertVersion(exceptions, "exceptions", ["schemaVersion", "records"]);
  if (!Array.isArray(exceptions.records)) fail("DSMANIFEST_SCHEMA", "exceptions.records must be an array");
  for (const [index, record] of exceptions.records.entries()) {
    exactKeys(record, ["path", "rule", "match", "rationale", "source", "owner", "reviewCondition"], `exceptions.records[${index}]`);
    if (!Object.values(record).every((value) => typeof value === "string" && value.trim())) fail("DSMANIFEST_SCHEMA", `exceptions.records[${index}] must use non-empty strings`);
    await assertContainedPath(root, record.path, `exceptions.records[${index}].path`);
  }
}

export async function loadDesignSystem(root = resolve(dirname(new URL(import.meta.url).pathname), "../..")) {
  const repositoryRoot = await realpath(root);
  const [tokens, components, runtimeGeometry, exceptions] = await Promise.all(FILES.map((name) => readJSON(repositoryRoot, name)));
  validateTokens(tokens);
  assertVersion(components, "components", ["schemaVersion", "components"]);
  assertVersion(runtimeGeometry, "runtime-geometry", ["schemaVersion", "properties"]);
  await validateExceptions(repositoryRoot, exceptions);
  return { tokens: { ...tokens, themes: tokens.themes.map((face) => ({ ...face, tokens: tokens.palettes[face.palette] })) }, components, runtimeGeometry, exceptions };
}
