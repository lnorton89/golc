import { lstat, readFile, realpath, readdir } from "node:fs/promises";
import { dirname, extname, isAbsolute, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { checkCSS } from "./css-policy.mjs";
import { checkTSX } from "./tsx-policy.mjs";

const FRONTEND_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const RULES = new Set(["DS001", "DS002", "DS003", "DS004", "DS005", "DS006", "DS007", "DS008", "DS009", "DS010"]);

function normalize(path) { return path.replaceAll("\\", "/"); }
function sortDiagnostics(diagnostics) { return diagnostics.sort((a, b) => [a.rule, a.path, a.line, a.column, a.value, a.message].map(String).join("\0").localeCompare([b.rule, b.path, b.line, b.column, b.value, b.message].map(String).join("\0"))); }
function fail(path, message, value = "") { return { rule: "DS000", path, line: 1, column: 1, message, value }; }

async function contained(root, candidate) {
  if (typeof candidate !== "string" || !candidate || isAbsolute(candidate) || candidate.includes("\\")) throw new Error("unresolvable source path");
  const absolute = resolve(root, candidate);
  if (relative(root, absolute).startsWith("..") || relative(root, absolute) === "") throw new Error("unresolvable source path");
  const stat = await lstat(absolute);
  const real = stat.isSymbolicLink() ? await realpath(absolute) : absolute;
  if (relative(root, real).startsWith("..")) throw new Error("unresolvable source path");
  return { absolute, path: normalize(relative(root, absolute)) };
}

async function filesBelow(root, directory) {
  const absolute = resolve(root, directory);
  const entries = await readdir(absolute, { withFileTypes: true });
  const found = [];
  for (const entry of entries.sort((a, b) => a.name.localeCompare(b.name))) {
    const nested = normalize(relative(root, resolve(absolute, entry.name)));
    if (entry.isDirectory()) found.push(...await filesBelow(root, nested));
    else if (entry.isFile() && /\.(module\.css|tsx)$/.test(entry.name)) found.push(nested);
  }
  return found;
}

async function declaredTokenNames(root) {
  const names = new Set();
  for (const path of ["src/design-system/tokens.generated.css", "design-system/runtime-geometry.json"]) {
    try {
      const source = await readFile(resolve(root, path), "utf8");
      for (const match of source.matchAll(/--ds-[A-Za-z0-9-]+/g)) names.add(match[0]);
    } catch { /* A scoped fixture need not carry the full authority. */ }
  }
  return names;
}

async function exceptionRecords(root) {
  try {
    const source = await readFile(resolve(root, "design-system/exceptions.json"), "utf8");
    const manifest = JSON.parse(source);
    return Array.isArray(manifest.records) ? manifest.records : [{ invalid: true }];
  } catch (error) {
    if (error?.code === "ENOENT") return [];
    return [{ invalid: true }];
  }
}

function manifestDiagnostic(path, source) {
  try { JSON.parse(source); } catch { return path.endsWith("components.json") ? { rule: "DS007", message: "inventory" } : path.endsWith("exceptions.json") ? { rule: "DS008", message: "exception" } : { rule: "DS009", message: "theme contract" }; }
  if (path.endsWith("components.json")) return { rule: "DS007", message: "inventory" };
  if (path.endsWith("exceptions.json")) return { rule: "DS008", message: "exception" };
  if (path.endsWith("tokens.json")) return { rule: "DS009", message: "theme contract" };
  return null;
}

function exceptionMatches(exception, diagnostics, sourceByPath) {
  if (!exception || typeof exception !== "object" || typeof exception.path !== "string" || typeof exception.rule !== "string" || typeof exception.match !== "string" || !exception.rationale?.trim() || !exception.reviewCondition?.trim() || !RULES.has(exception.rule) || /[*?\n\r]/.test(exception.match)) return { error: "invalid exception record" };
  if (exception.rule === "DS001" && /(padding|margin|gap|space)/i.test(exception.match)) return { error: "spacing exception" };
  const matching = diagnostics.filter((diagnostic) => diagnostic.rule === exception.rule && diagnostic.path === exception.path && (diagnostic.value === exception.match || sourceByPath.get(exception.path)?.includes(exception.match)));
  return { matching };
}

export async function checkFiles(root = FRONTEND_ROOT, requestedPaths = []) {
  const realRoot = await realpath(root);
  const paths = [...new Set(requestedPaths)].sort();
  let diagnostics = [];
  const sources = new Map();
  const declaredTokens = await declaredTokenNames(realRoot);
  for (const path of paths) {
    try {
      const target = await contained(realRoot, path);
      const source = (await readFile(target.absolute, "utf8")).replaceAll("\r\n", "\n");
      sources.set(target.path, source);
      if (extname(target.path) === ".css") diagnostics.push(...checkCSS({ path: target.path, source, declaredTokens, isDesignSystemFile: target.path.startsWith("src/design-system/") }));
      else if (target.path.endsWith(".tsx")) diagnostics.push(...checkTSX({ path: target.path, source, isPrimitiveFile: target.path.includes("src/components/primitives/"), isThemeFile: target.path === "src/lib/theme.ts" }));
      else if (target.path.startsWith("design-system/") && target.path.endsWith(".json")) {
        const issue = manifestDiagnostic(target.path, source);
        if (issue) diagnostics.push({ rule: issue.rule, path: target.path, line: 1, column: 1, message: issue.message, value: "" });
      }
    } catch { diagnostics.push(fail(normalize(path), "unresolvable source path")); }
  }
  const exceptions = await exceptionRecords(realRoot);
  for (const exception of exceptions) {
    const result = exceptionMatches(exception, diagnostics, sources);
    if (result.error) diagnostics.push({ rule: "DS008", path: "design-system/exceptions.json", line: 1, column: 1, message: result.error, value: "" });
    else if (result.matching.length !== 1) diagnostics.push({ rule: "DS008", path: exception.path, line: 1, column: 1, message: result.matching.length === 0 ? "stale exception" : "broad exception", value: exception.match });
    else diagnostics = diagnostics.filter((diagnostic) => diagnostic !== result.matching[0]);
  }
  return { diagnostics: sortDiagnostics(diagnostics) };
}

export async function checkDesignSystem(root = FRONTEND_ROOT, { paths = [], wholeSource = false } = {}) {
  const selected = paths.length ? paths : wholeSource ? await filesBelow(root, "src") : [];
  return checkFiles(root, selected);
}

async function main() {
  const scoped = process.argv.slice(2).filter((argument) => argument !== "--all");
  const result = await checkDesignSystem(FRONTEND_ROOT, { paths: scoped, wholeSource: process.argv.includes("--all") || scoped.length === 0 });
  for (const diagnostic of result.diagnostics) console.error(`${diagnostic.rule} ${diagnostic.path}:${diagnostic.line}:${diagnostic.column} ${diagnostic.message}${diagnostic.value ? ` (${diagnostic.value})` : ""}`);
  process.exitCode = result.diagnostics.length ? 1 : 0;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main();
