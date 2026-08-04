import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { readFile, readdir } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const FRONTEND_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const REPO_ROOT = resolve(FRONTEND_ROOT, "..");
export const PHASE_DIR = resolve(REPO_ROOT, ".planning/phases/13-unified-ui-design-system-and-automated-enforcement");
export const EVIDENCE_DIR = resolve(PHASE_DIR, "evidence");

const HEX40 = /^[0-9a-f]{40}$/i;
const HEX64 = /^[0-9a-f]{64}$/i;

export const REQUIREMENT_IDS = [
  "D-01", "D-02", "D-03", "D-04", "D-05", "D-06", "D-07",
  "D-08", "D-09", "D-10", "D-11", "D-12", "D-13", "D-14",
  "UI-SPEC-MIGRATION-ACCEPTANCE",
];

export const BACKSTOP_IDS = [
  "startup-theme-font-before-settle",
  "error-boundary-before-theme-css",
  "specialized-geometry-900-1280",
  "expanded-copy-2x-reflow",
  "text-zoom-200-900x720",
  "provider-daemon-offline-safety",
];

// ---------------------------------------------------------------------------
// Command/hash normalization and derivation
// ---------------------------------------------------------------------------

export function sha256(text) {
  return createHash("sha256").update(text, "utf8").digest("hex");
}

export function decodeXmlEntities(text) {
  return text
    .replaceAll("&lt;", "<")
    .replaceAll("&gt;", ">")
    .replaceAll("&quot;", '"')
    .replaceAll("&apos;", "'")
    .replaceAll("&amp;", "&");
}

export function normalizeCommand(raw) {
  if (typeof raw !== "string") return null;
  const decoded = decodeXmlEntities(raw);
  const lf = decoded.replaceAll("\r\n", "\n").replaceAll("\r", "\n");
  const trimmed = lf.trim();
  return trimmed.length ? trimmed : null;
}

function isIsoTimestamp(value) {
  return typeof value === "string" && value.length > 0 && !Number.isNaN(Date.parse(value));
}

function isContainedRelativePath(path) {
  return (
    typeof path === "string" &&
    path.length > 0 &&
    !path.startsWith("/") &&
    !/^[A-Za-z]:[\\/]/.test(path) &&
    !path.includes("\\") &&
    !path.split("/").includes("..")
  );
}

// ---------------------------------------------------------------------------
// Plan parsing: derive the exact task/command/hash contract from PLAN.md files
// ---------------------------------------------------------------------------

function extractFrontmatterField(source, field) {
  const match = source.match(new RegExp(`^${field}:\\s*"?([^"\\n]+?)"?\\s*$`, "m"));
  return match ? match[1].trim() : null;
}

/**
 * Parse every <task> element (auto AND checkpoint -- checkpoint tasks can
 * carry their own read-only preflight <verify><automated> command, e.g.
 * 13-01-01 and 13-35-01) from one PLAN.md source, in document order. Task
 * identity is the 1-based position among ALL task elements in the plan's
 * <tasks> block, matching this project's durable local-ID convention.
 */
export function parsePlanTasks(planId, source) {
  const wave = extractFrontmatterField(source, "wave");
  const waveNumber = wave ? Number(wave) : null;
  const tasksBlockMatch = source.match(/<tasks>([\s\S]*?)<\/tasks>/);
  if (!tasksBlockMatch) return [];
  const tasksBlock = tasksBlockMatch[1];
  const taskMatches = [...tasksBlock.matchAll(/<task\s+type="([^"]+)"([^>]*)>([\s\S]*?)<\/task>/g)];
  return taskMatches.map((match, index) => {
    const [, type, attrs, body] = match;
    const position = index + 1;
    const taskId = `${planId}-${String(position).padStart(2, "0")}`;
    const nameMatch = body.match(/<name>([\s\S]*?)<\/name>/);
    const name = nameMatch ? decodeXmlEntities(nameMatch[1]).trim() : null;
    const tddMatch = attrs.match(/tdd="([^"]+)"/);
    const tdd = tddMatch ? tddMatch[1] === "true" : false;
    const gateMatch = attrs.match(/gate="([^"]+)"/);
    const gate = gateMatch ? gateMatch[1] : null;
    let command = null;
    let commandSha256 = null;
    const verifyMatch = body.match(/<verify>\s*<automated>([\s\S]*?)<\/automated>\s*<\/verify>/);
    if (verifyMatch) {
      command = normalizeCommand(verifyMatch[1]);
      commandSha256 = command ? sha256(command) : null;
    }
    return { taskId, plan: planId, wave: waveNumber, position, type, tdd, gate, name, command, commandSha256 };
  });
}

/**
 * Discover every 13-NN-PLAN.md in the phase directory and derive the full
 * task/command/hash contract. Any task carrying a <verify><automated>
 * command is included -- missing/duplicate/stale task rows and any
 * non-equal command or hash are rejected by validateResultRow below.
 */
export async function derivePlanCommandContract(phaseDir = PHASE_DIR) {
  const entries = await readdir(phaseDir, { withFileTypes: true });
  const planFiles = entries
    .filter((entry) => entry.isFile() && /^13-\d{2}-PLAN\.md$/.test(entry.name))
    .map((entry) => entry.name)
    .sort();
  const contract = new Map();
  const errors = [];
  for (const fileName of planFiles) {
    const planId = fileName.slice(0, 5); // "13-NN"
    const source = await readFile(resolve(phaseDir, fileName), "utf8");
    const tasks = parsePlanTasks(planId, source);
    if (tasks.length === 0) errors.push(`${planId}: no <tasks> block parsed from ${fileName}`);
    for (const task of tasks) {
      if (!task.command) continue; // no automated verify command declared for this task
      if (contract.has(task.taskId)) errors.push(`${task.taskId}: duplicate task id derived from ${fileName}`);
      contract.set(task.taskId, task);
    }
  }
  return { contract, errors, planFiles };
}

// ---------------------------------------------------------------------------
// Result-row / artifact schema (closed, fail-closed)
// ---------------------------------------------------------------------------

export function validateArtifact(artifact, label) {
  const errors = [];
  if (!artifact || typeof artifact !== "object") {
    errors.push(`${label}: artifact must be an object`);
    return errors;
  }
  const remote = typeof artifact.path === "string" && /^https:\/\/[^\s]+$/.test(artifact.path);
  if (!isContainedRelativePath(artifact.path) && !remote) {
    errors.push(`${label}: path must be a contained repository-relative path or an immutable https:// URL`);
  }
  if (typeof artifact.mediaType !== "string" || !artifact.mediaType) errors.push(`${label}: mediaType missing`);
  if (typeof artifact.byteCount !== "number" || !Number.isFinite(artifact.byteCount) || artifact.byteCount < 0) {
    errors.push(`${label}: byteCount missing/invalid`);
  }
  if (!HEX64.test(artifact.sha256 ?? "")) errors.push(`${label}: sha256 missing/invalid (must be a 64-hex digest)`);
  return errors;
}

/**
 * A completed task's result row must exactly match its plan-derived command
 * and hash, exit zero, and carry a closed evidence schema: timestamps,
 * repository commit SHA, dirty declaration, environment/build identity, and
 * typed artifacts. Markdown/prose or existence-only claims cannot satisfy
 * this contract -- every field below is mechanically checked.
 */
export function validateResultRow(row, contract) {
  const errors = [];
  if (!row || typeof row !== "object") return ["row must be an object"];
  const { taskId } = row;
  if (typeof taskId !== "string" || !taskId) {
    errors.push("row.taskId missing");
    return errors;
  }
  const derived = contract.get(taskId);
  if (!derived) {
    errors.push(`${taskId}: unknown task id -- not derived from any PLAN.md (stale or extra row)`);
    return errors;
  }
  if (row.plan !== derived.plan) errors.push(`${taskId}: plan mismatch (row=${row.plan}, derived=${derived.plan})`);
  if (row.wave !== derived.wave) errors.push(`${taskId}: wave mismatch (row=${row.wave}, derived=${derived.wave})`);
  if (row.command !== derived.command) errors.push(`${taskId}: command does not exactly match the PLAN-derived command`);
  if (row.commandSha256 !== derived.commandSha256) errors.push(`${taskId}: commandSha256 does not match the derived SHA-256`);
  if (row.exitCode !== 0) errors.push(`${taskId}: exitCode must be exactly 0 (got ${JSON.stringify(row.exitCode)})`);
  if (!isIsoTimestamp(row.startedAt)) errors.push(`${taskId}: startedAt missing/invalid`);
  if (!isIsoTimestamp(row.completedAt)) errors.push(`${taskId}: completedAt missing/invalid`);
  if (!HEX40.test(row.repositoryCommitSha ?? "")) errors.push(`${taskId}: repositoryCommitSha must be a 40-hex commit SHA`);
  if (typeof row.dirty !== "boolean") errors.push(`${taskId}: dirty-tree declaration missing`);
  const env = row.environment;
  if (!env || typeof env.os !== "string" || !env.os || typeof env.runtime !== "string" || !env.runtime) {
    errors.push(`${taskId}: environment identity (os/runtime) missing`);
  }
  const build = row.build;
  if (!build || typeof build.identity !== "string" || !build.identity) {
    errors.push(`${taskId}: application build identity missing`);
  }
  if (!Array.isArray(row.artifacts)) {
    errors.push(`${taskId}: artifacts must be an array`);
  } else {
    row.artifacts.forEach((artifact, index) => errors.push(...validateArtifact(artifact, `${taskId}.artifacts[${index}]`)));
  }
  return errors;
}

// ---------------------------------------------------------------------------
// Generic assertion-object helper (used by every backstop evidence schema)
// ---------------------------------------------------------------------------

export function collectAssertionErrors(assertions, label) {
  if (!assertions || typeof assertions !== "object") return [`${label}: assertions object missing`];
  const entries = Object.entries(assertions);
  if (entries.length === 0) return [`${label}: assertions object is empty`];
  const errors = [];
  for (const [key, value] of entries) {
    if (value !== true) errors.push(`${label}: assertion "${key}" is not true (${JSON.stringify(value)})`);
  }
  return errors;
}

// ---------------------------------------------------------------------------
// Calibration arithmetic (Plan 13-17): recompute pairwise diffs and the
// selected threshold rather than trusting a hand-written summary.
// ---------------------------------------------------------------------------

export function validateCalibrationEvidence(evidence) {
  const errors = [];
  if (!evidence || typeof evidence !== "object") return ["calibration evidence must be an object"];
  if (!Array.isArray(evidence.states) || evidence.states.length === 0) {
    return [...errors, "calibration.states must be a non-empty array"];
  }
  let overallMax = 0;
  for (const state of evidence.states) {
    const label = `calibration[${state?.name ?? "?"}]`;
    if (!Array.isArray(state.captures) || state.captures.length < 3) {
      errors.push(`${label}: requires at least 3 independent captures`);
      continue;
    }
    const ids = state.captures.map((capture) => capture.id);
    if (new Set(ids).size !== ids.length) errors.push(`${label}: duplicate capture ids`);
    for (const capture of state.captures) {
      if (!HEX64.test(capture.sha256 ?? "")) errors.push(`${label}: capture ${capture.id} missing/invalid sha256`);
    }
    if (!Array.isArray(state.pairwiseDiffs) || state.pairwiseDiffs.length === 0) {
      errors.push(`${label}: missing pairwiseDiffs (every capture pair must be diffed)`);
      continue;
    }
    const expectedPairs = (ids.length * (ids.length - 1)) / 2;
    if (state.pairwiseDiffs.length !== expectedPairs) {
      errors.push(`${label}: pairwiseDiffs count ${state.pairwiseDiffs.length} does not equal expected ${expectedPairs} for ${ids.length} captures`);
    }
    let recomputedMax = 0;
    for (const diff of state.pairwiseDiffs) {
      if (typeof diff.smallestPassingRatio !== "number" || diff.smallestPassingRatio < 0) {
        errors.push(`${label}: pair ${JSON.stringify(diff.pair)} smallestPassingRatio invalid`);
        continue;
      }
      recomputedMax = Math.max(recomputedMax, diff.smallestPassingRatio);
    }
    if (typeof state.maxRatio !== "number" || Math.abs(state.maxRatio - recomputedMax) > 1e-9) {
      errors.push(`${label}: declared maxRatio ${state.maxRatio} does not equal recomputed max ${recomputedMax}`);
    }
    overallMax = Math.max(overallMax, typeof state.maxRatio === "number" ? state.maxRatio : recomputedMax);
  }
  if (typeof evidence.ceiling !== "number") errors.push("calibration.ceiling missing");
  if (typeof evidence.selectedThreshold !== "number") {
    errors.push("calibration.selectedThreshold missing");
  } else {
    if (Math.abs(evidence.selectedThreshold - overallMax) > 1e-9) {
      errors.push(`calibration.selectedThreshold ${evidence.selectedThreshold} does not equal the recomputed smallest-stable threshold ${overallMax}`);
    }
    if (typeof evidence.ceiling === "number" && evidence.selectedThreshold > evidence.ceiling) {
      errors.push(`calibration.selectedThreshold ${evidence.selectedThreshold} exceeds ceiling ${evidence.ceiling}`);
    }
  }
  return errors;
}

// ---------------------------------------------------------------------------
// Mask audit: every rectangle/reason, and zero protected-locator intersections
// ---------------------------------------------------------------------------

function rectanglesIntersect(a, b) {
  return !(a.x + a.width <= b.x || b.x + b.width <= a.x || a.y + a.height <= b.y || b.y + b.height <= a.y);
}

export function validateMaskAudit(masks) {
  const errors = [];
  if (!Array.isArray(masks)) return ["mask audit must be an array"];
  masks.forEach((mask, index) => {
    const label = `mask[${index}]`;
    if (!mask || typeof mask !== "object") {
      errors.push(`${label}: mask record must be an object`);
      return;
    }
    const rect = mask.rectangle;
    if (!rect || typeof rect.x !== "number" || typeof rect.y !== "number" || typeof rect.width !== "number" || typeof rect.height !== "number") {
      errors.push(`${label}: rectangle missing/invalid`);
    }
    if (typeof mask.reason !== "string" || !mask.reason.trim()) errors.push(`${label}: reason missing`);
    if (typeof mask.screenshot !== "string" || !mask.screenshot.trim()) errors.push(`${label}: screenshot reference missing`);
    const protectedLocators = Array.isArray(mask.protectedLocatorRectangles) ? mask.protectedLocatorRectangles : [];
    if (rect) {
      protectedLocators.forEach((protectedRect, protectedIndex) => {
        if (rectanglesIntersect(rect, protectedRect)) {
          errors.push(
            `${label}: intersects protected locator rectangle[${protectedIndex}]` +
              `${protectedRect?.name ? ` (${protectedRect.name})` : ""} -- masks may never cover ` +
              "Blackout, Revoke Automation, Stop/Release-All, live truth, navigation, or dialog focus",
          );
        }
      });
    }
  });
  return errors;
}

// ---------------------------------------------------------------------------
// Packaged WebView2 (dialog-feasibility) evidence
// ---------------------------------------------------------------------------

export function validateDialogFeasibilityEvidence(evidence) {
  const errors = [];
  if (!evidence || typeof evidence !== "object") return ["packaged WebView2 evidence must be an object"];
  if (evidence.status !== "passed") errors.push(`packaged WebView2 proof status is "${evidence.status}", expected "passed"`);
  if (evidence.error != null) errors.push(`packaged WebView2 proof recorded an error: ${JSON.stringify(evidence.error)}`);
  if (!isIsoTimestamp(evidence.started_at) || !isIsoTimestamp(evidence.completed_at)) {
    errors.push("packaged WebView2 evidence missing valid started_at/completed_at timestamps");
  }
  const build = evidence.build;
  if (!build || typeof build.executable !== "string" || !build.executable) errors.push("packaged WebView2 evidence missing build.executable path");
  if (!HEX64.test(build?.sha256 ?? "")) errors.push("packaged WebView2 evidence missing/invalid build.sha256");
  const runtime = evidence.runtime;
  if (!runtime || typeof runtime.cdp_endpoint !== "string" || !runtime.cdp_endpoint) {
    errors.push("packaged WebView2 evidence missing runtime.cdp_endpoint (proves real CDP endpoint ownership, not an environment-only harness)");
  }
  const test = evidence.test;
  if (!test || test.exit_code !== 0) errors.push("packaged WebView2 evidence test.exit_code must be exactly 0");
  return errors;
}

export function validatePackagedWebView2Evidence(evidence, { expectedExecutableSha256 } = {}) {
  const errors = validateDialogFeasibilityEvidence(evidence);
  if (expectedExecutableSha256 && evidence?.build?.sha256 && evidence.build.sha256.toLowerCase() !== expectedExecutableSha256.toLowerCase()) {
    errors.push(`packaged WebView2 executable sha256 ${evidence.build.sha256} does not match the expected application build hash ${expectedExecutableSha256}`);
  }
  return errors;
}

// ---------------------------------------------------------------------------
// Windows CI run evidence
// ---------------------------------------------------------------------------

export function validateWindowsCiEvidence(evidence, { approvedSha } = {}) {
  const errors = [];
  if (!evidence || typeof evidence !== "object") return ["Windows CI evidence must be an object"];
  if (!HEX40.test(evidence.headSha ?? "")) {
    errors.push("Windows CI evidence headSha must be a 40-hex commit SHA");
  } else if (approvedSha && evidence.headSha !== approvedSha) {
    errors.push(`Windows CI evidence headSha ${evidence.headSha} does not match the approved trigger SHA ${approvedSha}`);
  }
  if (evidence.status !== "completed" || evidence.conclusion !== "success") {
    errors.push(`Windows CI run must be completed/success (status=${evidence.status}, conclusion=${evidence.conclusion})`);
  }
  if (typeof evidence.runId !== "number" && typeof evidence.runId !== "string") errors.push("Windows CI evidence missing runId");
  if (typeof evidence.url !== "string" || !/^https:\/\/github\.com\//.test(evidence.url)) {
    errors.push("Windows CI evidence url must be an immutable https://github.com/... run URL");
  }
  if (evidence.workflowPath !== ".github/workflows/design-system.yml") {
    errors.push('Windows CI evidence workflowPath must be ".github/workflows/design-system.yml"');
  }
  if (!Array.isArray(evidence.artifacts) || evidence.artifacts.length === 0) {
    errors.push("Windows CI evidence missing downloaded artifacts");
  } else {
    evidence.artifacts.forEach((artifact, index) => errors.push(...validateArtifact(artifact, `windowsCi.artifacts[${index}]`)));
  }
  if (!Array.isArray(evidence.jobs) || !evidence.jobs.some((job) => job?.conclusion === "success")) {
    errors.push("Windows CI evidence missing at least one successful job record");
  }
  return errors;
}

// ---------------------------------------------------------------------------
// CI-proven implementation-tree identity: sorted non-planning manifest/hash,
// descendant ancestry, and the .planning/**-only allowlist for later commits.
// ---------------------------------------------------------------------------

function realGitRunner(gitRoot) {
  return {
    lsTree(sha) {
      const output = execFileSync("git", ["ls-tree", "-r", "--full-tree", sha], { cwd: gitRoot, encoding: "utf8" });
      return output
        .split("\n")
        .filter(Boolean)
        .map((line) => {
          const tabIndex = line.indexOf("\t");
          const meta = line.slice(0, tabIndex).split(" ");
          const path = line.slice(tabIndex + 1);
          return { mode: meta[0], objectId: meta[2], path };
        });
    },
    diffNameOnly(a, b) {
      const output = execFileSync("git", ["diff", "--name-only", a, b], { cwd: gitRoot, encoding: "utf8" });
      return output.split("\n").filter(Boolean);
    },
    isAncestor(a, b) {
      try {
        execFileSync("git", ["merge-base", "--is-ancestor", a, b], { cwd: gitRoot });
        return true;
      } catch {
        return false;
      }
    },
  };
}

export function computeImplementationManifest(sha, gitRunner) {
  const entries = gitRunner
    .lsTree(sha)
    .filter((entry) => !entry.path.startsWith(".planning/"))
    .slice()
    .sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : 0));
  const manifestText = entries.map((entry) => `${entry.mode} blob ${entry.objectId}\t${entry.path}`).join("\n");
  return { hash: sha256(manifestText), entries };
}

/**
 * Descendants are accepted only when: ancestry holds, every changed path
 * between the proven and observed commit is allowlisted under `.planning/**`,
 * and the recomputed non-planning manifest/hash is identical at both commits.
 * Any frontend/runtime/workflow/dependency/build/configuration change after
 * the proven SHA fails closed.
 */
export function validateImplementationTreeIdentity({ provenSha, observedSha, gitRunner, declaredProvenHash, declaredObservedHash }) {
  const errors = [];
  if (!HEX40.test(provenSha ?? "")) errors.push("ciProvenImplementationSha must be a 40-hex commit SHA");
  if (!HEX40.test(observedSha ?? "")) errors.push("observedDescendantSha must be a 40-hex commit SHA");
  if (errors.length || !gitRunner) return errors;

  if (provenSha !== observedSha) {
    if (!gitRunner.isAncestor(provenSha, observedSha)) {
      errors.push("observedDescendantSha does not descend from ciProvenImplementationSha (ancestry check failed)");
      return errors;
    }
    const changed = gitRunner.diffNameOnly(provenSha, observedSha);
    const disallowed = changed.filter((path) => !path.startsWith(".planning/"));
    if (disallowed.length) {
      errors.push(`non-.planning/** paths changed after the proven SHA (frontend/runtime/workflow/dependency/build/configuration change rejected): ${disallowed.join(", ")}`);
    }
  }

  const proven = computeImplementationManifest(provenSha, gitRunner);
  const observed = computeImplementationManifest(observedSha, gitRunner);
  if (declaredProvenHash && declaredProvenHash !== proven.hash) {
    errors.push("declared proven implementation-tree manifest hash does not match the recomputed manifest hash");
  }
  if (declaredObservedHash && declaredObservedHash !== observed.hash) {
    errors.push("declared observed implementation-tree manifest hash does not match the recomputed manifest hash");
  }
  if (proven.hash !== observed.hash) {
    errors.push(`non-planning implementation-tree manifest hash differs between proven (${proven.hash}) and observed (${observed.hash}) commits`);
  }
  return errors;
}

// ---------------------------------------------------------------------------
// Six separately named UI backstops
// ---------------------------------------------------------------------------

export function validateBackstopStartupTheme(evidence) {
  if (!evidence || !Array.isArray(evidence.themes) || evidence.themes.length === 0) {
    return ["startup-theme-font evidence missing themes[]"];
  }
  const errors = [];
  if (!HEX40.test(evidence.buildSha ?? "")) errors.push("startup-theme-font evidence missing a valid buildSha");
  for (const theme of evidence.themes) {
    const label = `startup-theme-font[${theme?.theme ?? "?"}]`;
    errors.push(...collectAssertionErrors(theme?.assertions, label));
    if (!Array.isArray(theme?.themeSequence) || theme.themeSequence.length === 0) {
      errors.push(`${label}: themeSequence missing (a continuous per-frame sample timeline is required, not a single after-the-fact screenshot)`);
    }
  }
  return errors;
}

export function validateBackstopErrorBoundary(evidence) {
  if (!evidence || !Array.isArray(evidence.viewports) || evidence.viewports.length === 0) {
    return ["error-boundary-fallback evidence missing viewports[]"];
  }
  const errors = [];
  if (!HEX40.test(evidence.buildSha ?? "")) errors.push("error-boundary-fallback evidence missing a valid buildSha");
  for (const viewport of evidence.viewports) {
    const label = `error-boundary-fallback[${viewport?.width}x${viewport?.height}]`;
    errors.push(...collectAssertionErrors(viewport?.assertions, label));
    if (!viewport?.blockedStylesheet) errors.push(`${label}: blockedStylesheet evidence missing (the generated token CSS must be proven genuinely blocked)`);
  }
  return errors;
}

export function validateBackstopSpecializedGeometry(evidence) {
  if (!evidence || !Array.isArray(evidence.cases) || evidence.cases.length === 0) {
    return ["specialized-geometry evidence missing cases[]"];
  }
  const errors = [];
  const recomputed = evidence.cases.every((geometryCase) => geometryCase?.passed === true);
  if (evidence.allCasesPassed !== true) errors.push("specialized-geometry evidence allCasesPassed must be true");
  if (recomputed !== evidence.allCasesPassed) errors.push("specialized-geometry evidence allCasesPassed does not match the recomputed AND of every case.passed");
  for (const geometryCase of evidence.cases) {
    const label = `specialized-geometry[${geometryCase?.family}/${geometryCase?.width}/${geometryCase?.resizeState}]`;
    if (geometryCase?.passed !== true) errors.push(`${label}: case did not pass`);
    if (![900, 1280].includes(geometryCase?.width)) errors.push(`${label}: width must be exactly 900 or 1280`);
  }
  return errors;
}

export function validateBackstopExpandedCopy(evidence) {
  if (!evidence || !Array.isArray(evidence.cases) || evidence.cases.length === 0) {
    return ["expanded-copy evidence missing cases[]"];
  }
  const errors = [];
  const minRatio = evidence.minimumExpansionRatio;
  if (typeof minRatio !== "number" || minRatio < 2) errors.push("expanded-copy evidence minimumExpansionRatio must be a number >= 2.0");
  const recomputed = evidence.cases.every((copyCase) => copyCase?.passed === true);
  if (evidence.allCasesPassed !== true) errors.push("expanded-copy evidence allCasesPassed must be true");
  if (recomputed !== evidence.allCasesPassed) errors.push("expanded-copy evidence allCasesPassed does not match the recomputed AND of every case.passed");
  for (const copyCase of evidence.cases) {
    const label = `expanded-copy[${copyCase?.pairId}]`;
    errors.push(...collectAssertionErrors(copyCase?.assertions, label));
    if (copyCase?.passed !== true) errors.push(`${label}: case did not pass`);
    if (typeof minRatio === "number" && (typeof copyCase?.expansionRatio !== "number" || copyCase.expansionRatio < minRatio)) {
      errors.push(`${label}: expansionRatio ${copyCase?.expansionRatio} is below the minimum ${minRatio}`);
    }
  }
  if (Array.isArray(evidence.pairs)) {
    for (const pair of evidence.pairs) {
      const canonicalLength = Array.from(pair?.canonical ?? "").length;
      const expandedLength = Array.from(pair?.expanded ?? "").length;
      if (canonicalLength === 0) {
        errors.push(`expanded-copy pair ${pair?.id}: canonical text missing`);
        continue;
      }
      const recomputedRatio = expandedLength / canonicalLength;
      if (typeof pair.expansionRatio === "number" && Math.abs(recomputedRatio - pair.expansionRatio) > 1e-6) {
        errors.push(`expanded-copy pair ${pair.id}: recomputed grapheme ratio ${recomputedRatio} does not match declared ${pair.expansionRatio}`);
      }
      if (typeof minRatio === "number" && recomputedRatio < minRatio) {
        errors.push(`expanded-copy pair ${pair.id}: recomputed expansion ratio ${recomputedRatio} is below the minimum ${minRatio}`);
      }
    }
  }
  return errors;
}

export function validateBackstopTextZoom(evidence) {
  if (!evidence || typeof evidence !== "object") return ["text-zoom-200 evidence must be an object"];
  const errors = [];
  if (evidence.viewport?.width !== 900 || evidence.viewport?.height !== 720) {
    errors.push("text-zoom-200 evidence viewport must be exactly 900x720");
  }
  if (Number(evidence.requestedZoom) !== 2 || Number(evidence.computedZoom) !== 2) {
    errors.push("text-zoom-200 evidence must record requestedZoom and computedZoom exactly 2 (a real 200% browser text zoom)");
  }
  const overflow = evidence.overflow;
  if (!overflow || overflow.rootOverflows !== false || overflow.bodyOverflows !== false) {
    errors.push("text-zoom-200 evidence must prove root/body do not overflow at 200% text zoom");
  }
  errors.push(...collectAssertionErrors(evidence.assertions, "text-zoom-200"));
  const requiredLocators = ["navigation", "liveTruth", "activeTask", "blackout", "revokeAutomation", "stopReleaseAll"];
  for (const key of requiredLocators) {
    const locator = evidence.locators?.[key];
    if (!locator || locator.present !== true || locator.passed !== true) {
      errors.push(`text-zoom-200 evidence locator "${key}" missing or not proven reachable`);
    }
  }
  if (!Array.isArray(evidence.focusTraversal) || evidence.focusTraversal.length === 0) {
    errors.push("text-zoom-200 evidence missing focusTraversal (ordered keyboard focus traversal)");
  }
  return errors;
}

export function validateBackstopOfflineSafety(evidence) {
  if (!evidence || !Array.isArray(evidence.states) || evidence.states.length === 0) {
    return ["offline-safety evidence missing states[]"];
  }
  const errors = [];
  if (evidence.allStatesPassed !== true) errors.push("offline-safety evidence allStatesPassed must be true");
  const truthFields = ["sceneName", "barText", "layersText", "sourceText", "outputText"];
  for (const state of evidence.states) {
    const label = `offline-safety[${state?.id}]`;
    errors.push(...collectAssertionErrors(state?.assertions, label));
    if (!Array.isArray(state?.forbiddenCopyFound) || state.forbiddenCopyFound.length > 0) {
      errors.push(`${label}: forbiddenCopyFound must be an empty array (connectivity loss must never synthesize a stopped/idle claim)`);
    }
    if (state?.playbackOutputTruthPreserved !== true) errors.push(`${label}: playbackOutputTruthPreserved must be true`);
    const before = state?.before ?? {};
    const after = state?.after ?? {};
    for (const field of truthFields) {
      if (before[field] !== after[field]) {
        errors.push(`${label}: connectivity loss changed ${field} from "${before[field]}" to "${after[field]}" -- Go-owned playback/output truth must remain authoritative and unchanged`);
      }
    }
    for (const controlName of ["blackout", "revokeAutomation"]) {
      const control = state?.controls?.[controlName];
      if (!control || control.visible !== true || control.focusableViaTab !== true || control.dispatchedExactlyOnce !== true || control.crossDispatchZero !== true) {
        errors.push(`${label}: control "${controlName}" not proven independently keyboard-operable via its own local command path with exactly-once dispatch`);
      }
    }
  }
  return errors;
}

// ---------------------------------------------------------------------------
// D-01..D-14/UI-SPEC and six-backstop coverage: exact set equality
// ---------------------------------------------------------------------------

export function validateExactCoverage(declared, canonical, label) {
  if (!Array.isArray(declared)) return [`${label}: coverage list missing`];
  const errors = [];
  const declaredSet = new Set(declared);
  const canonicalSet = new Set(canonical);
  const missing = canonical.filter((id) => !declaredSet.has(id));
  const extra = declared.filter((id) => !canonicalSet.has(id));
  if (missing.length) errors.push(`${label}: missing coverage for ${missing.join(", ")}`);
  if (extra.length) errors.push(`${label}: unexpected/unknown coverage entries: ${extra.join(", ")}`);
  if (declaredSet.size !== declared.length) errors.push(`${label}: duplicate coverage entries`);
  return errors;
}

// ---------------------------------------------------------------------------
// Whole-bundle sign-off gate
// ---------------------------------------------------------------------------

const BACKSTOP_VALIDATORS = [
  ["startupTheme", validateBackstopStartupTheme, "startup-theme-font-before-settle"],
  ["errorBoundary", validateBackstopErrorBoundary, "error-boundary-before-theme-css"],
  ["specializedGeometry", validateBackstopSpecializedGeometry, "specialized-geometry-900-1280"],
  ["expandedCopy", validateBackstopExpandedCopy, "expanded-copy-2x-reflow"],
  ["textZoom", validateBackstopTextZoom, "text-zoom-200-900x720"],
  ["offlineSafety", validateBackstopOfflineSafety, "provider-daemon-offline-safety"],
];

/**
 * Validate a complete phase-13 sign-off evidence bundle: every plan-derived
 * task/command/hash, the closed result-row schema, every semantic evidence
 * category, D-01..D-14/UI-SPEC and six-backstop coverage, and only then the
 * sign-off flags themselves. Markdown existence alone can never satisfy this;
 * every check below is mechanical and fails closed on missing/malformed data.
 */
export async function validateEvidenceBundle(bundle, options = {}) {
  const errors = [];
  if (!bundle || typeof bundle !== "object") return { ok: false, errors: ["evidence bundle must be an object"] };

  const { contract } = options.contract
    ? { contract: options.contract }
    : await derivePlanCommandContract(options.phaseDir ?? PHASE_DIR);

  if (!Array.isArray(bundle.rows)) {
    errors.push("evidence bundle missing rows[]");
  } else {
    const seen = new Set();
    for (const row of bundle.rows) {
      if (row?.taskId && seen.has(row.taskId)) errors.push(`duplicate row for task ${row.taskId}`);
      if (row?.taskId) seen.add(row.taskId);
      errors.push(...validateResultRow(row, contract));
    }
    const missingTasks = [...contract.keys()].filter((taskId) => !seen.has(taskId));
    if (missingTasks.length) errors.push(`evidence bundle missing rows for tasks: ${missingTasks.join(", ")}`);
  }

  if (bundle.calibration) errors.push(...validateCalibrationEvidence(bundle.calibration).map((error) => `calibration: ${error}`));
  else errors.push("evidence bundle missing calibration evidence");

  if (Array.isArray(bundle.masks)) errors.push(...validateMaskAudit(bundle.masks).map((error) => `mask audit: ${error}`));
  else errors.push("evidence bundle missing mask audit");

  if (bundle.packagedWebView2) {
    errors.push(
      ...validatePackagedWebView2Evidence(bundle.packagedWebView2, { expectedExecutableSha256: bundle.expectedExecutableSha256 }).map(
        (error) => `packaged WebView2: ${error}`,
      ),
    );
  } else {
    errors.push("evidence bundle missing packaged WebView2 evidence");
  }

  if (bundle.windowsCi) errors.push(...validateWindowsCiEvidence(bundle.windowsCi, options).map((error) => `Windows CI: ${error}`));
  else errors.push("evidence bundle missing Windows CI evidence");

  if (bundle.implementationTree) {
    const gitRunner = options.gitRunner ?? realGitRunner(options.gitRoot ?? REPO_ROOT);
    errors.push(
      ...validateImplementationTreeIdentity({ ...bundle.implementationTree, gitRunner }).map((error) => `implementation tree: ${error}`),
    );
  } else {
    errors.push("evidence bundle missing CI implementation-tree identity");
  }

  for (const [key, validator, label] of BACKSTOP_VALIDATORS) {
    const evidence = bundle.backstops?.[key];
    if (evidence) errors.push(...validator(evidence).map((error) => `${label}: ${error}`));
    else errors.push(`evidence bundle missing backstop evidence for ${label}`);
  }

  errors.push(...validateExactCoverage(bundle.requirementsCovered, REQUIREMENT_IDS, "requirements coverage"));
  errors.push(...validateExactCoverage(bundle.backstopsCovered, BACKSTOP_IDS, "backstop coverage"));

  const signOff = bundle.signOff ?? {};
  const signOffClaimed = signOff.wave_0_complete === true || signOff.nyquist_compliant === true || signOff.approved === true;
  if (errors.length === 0) {
    if (signOff.wave_0_complete !== true || signOff.nyquist_compliant !== true || signOff.approved !== true) {
      errors.push("all evidence passed but sign-off flags (wave_0_complete/nyquist_compliant/approved) are not all true");
    }
  } else if (signOffClaimed) {
    errors.push("sign-off flags cannot be true while evidence validation errors remain (premature wave_0_complete/nyquist_compliant/approval)");
  }

  return { ok: errors.length === 0, errors };
}

// ---------------------------------------------------------------------------
// Strict JSON parsing: duplicate object keys fail closed with a stable,
// positional diagnostic instead of silently keeping the last value (which is
// how a bare JSON.parse behaves on ambiguous/duplicate-key input).
// ---------------------------------------------------------------------------

export function parseStrictJson(source, label = "input") {
  let index = 0;
  const length = source.length;

  function fail(message) {
    throw new Error(`PHASE13_EVIDENCE_MALFORMED_JSON: ${label}: ${message} (position ${index})`);
  }
  function skipWhitespace() {
    while (index < length && /\s/.test(source[index])) index += 1;
  }
  function parseKeyword() {
    for (const [word, value] of [["true", true], ["false", false], ["null", null]]) {
      if (source.startsWith(word, index)) {
        index += word.length;
        return value;
      }
    }
    fail("invalid literal");
    return undefined;
  }
  function parseNumber() {
    const start = index;
    if (source[index] === "-") index += 1;
    while (index < length && /[0-9]/.test(source[index])) index += 1;
    if (source[index] === ".") {
      index += 1;
      while (index < length && /[0-9]/.test(source[index])) index += 1;
    }
    if (source[index] === "e" || source[index] === "E") {
      index += 1;
      if (source[index] === "+" || source[index] === "-") index += 1;
      while (index < length && /[0-9]/.test(source[index])) index += 1;
    }
    const text = source.slice(start, index);
    if (!text || text === "-") fail("invalid number");
    return Number(text);
  }
  function parseString() {
    index += 1; // opening quote
    let result = "";
    for (;;) {
      if (index >= length) fail("unterminated string");
      const ch = source[index];
      if (ch === '"') {
        index += 1;
        return result;
      }
      if (ch === "\\") {
        index += 1;
        const esc = source[index];
        const map = { '"': '"', "\\": "\\", "/": "/", b: "\b", f: "\f", n: "\n", r: "\r", t: "\t" };
        if (esc === "u") {
          const hex = source.slice(index + 1, index + 5);
          if (!/^[0-9a-fA-F]{4}$/.test(hex)) fail("invalid unicode escape");
          result += String.fromCharCode(parseInt(hex, 16));
          index += 5;
        } else if (esc in map) {
          result += map[esc];
          index += 1;
        } else {
          fail("invalid escape sequence");
        }
      } else {
        result += ch;
        index += 1;
      }
    }
  }
  function parseObject() {
    index += 1; // {
    const result = {};
    const seenKeys = new Set();
    skipWhitespace();
    if (source[index] === "}") {
      index += 1;
      return result;
    }
    for (;;) {
      skipWhitespace();
      if (source[index] !== '"') fail("expected string key");
      const key = parseString();
      if (seenKeys.has(key)) fail(`duplicate key "${key}"`);
      seenKeys.add(key);
      skipWhitespace();
      if (source[index] !== ":") fail('expected ":"');
      index += 1;
      result[key] = parseValue();
      skipWhitespace();
      if (source[index] === ",") {
        index += 1;
        continue;
      }
      if (source[index] === "}") {
        index += 1;
        return result;
      }
      fail('expected "," or "}"');
    }
  }
  function parseArray() {
    index += 1; // [
    const result = [];
    skipWhitespace();
    if (source[index] === "]") {
      index += 1;
      return result;
    }
    for (;;) {
      result.push(parseValue());
      skipWhitespace();
      if (source[index] === ",") {
        index += 1;
        continue;
      }
      if (source[index] === "]") {
        index += 1;
        return result;
      }
      fail('expected "," or "]"');
    }
  }
  function parseValue() {
    skipWhitespace();
    const ch = source[index];
    if (ch === "{") return parseObject();
    if (ch === "[") return parseArray();
    if (ch === '"') return parseString();
    if (ch === "t" || ch === "f" || ch === "n") return parseKeyword();
    if (ch === "-" || (ch >= "0" && ch <= "9")) return parseNumber();
    fail(`unexpected character "${ch ?? "<eof>"}"`);
    return undefined;
  }

  const value = parseValue();
  skipWhitespace();
  if (index !== length) fail("unexpected trailing content after top-level value");
  return value;
}

// ---------------------------------------------------------------------------
// CLI
// ---------------------------------------------------------------------------

export function parseCommandLineArgs(argv) {
  const evidenceIndex = argv.indexOf("--evidence");
  if (evidenceIndex !== -1) {
    const value = argv[evidenceIndex + 1];
    if (!value) throw new Error("VALIDATE_ARGS: --evidence requires a path");
    return { evidencePath: value };
  }
  if (argv.length) throw new Error("VALIDATE_ARGS: use --evidence <path> or no arguments");
  return { evidencePath: null };
}

const DEFAULT_MODE_CATEGORY_FILES = {
  "dialog-feasibility.json": validateDialogFeasibilityEvidence,
  "screenshot-calibration.json": validateCalibrationEvidence,
  "startup-theme-font.json": validateBackstopStartupTheme,
  "error-boundary-fallback.json": validateBackstopErrorBoundary,
  "specialized-geometry.json": validateBackstopSpecializedGeometry,
  "expanded-copy.json": validateBackstopExpandedCopy,
  "text-zoom-200.json": validateBackstopTextZoom,
  "offline-safety.json": validateBackstopOfflineSafety,
};

async function main() {
  const { evidencePath } = parseCommandLineArgs(process.argv.slice(2));

  if (evidencePath) {
    const absolute = resolve(process.cwd(), evidencePath);
    let bundle;
    try {
      bundle = parseStrictJson(await readFile(absolute, "utf8"), evidencePath);
    } catch (error) {
      console.error(`FAIL ${error instanceof Error ? error.message : "unreadable/malformed evidence bundle"}`);
      process.exitCode = 1;
      return;
    }
    const approvedSha = process.env.GOLC_PHASE13_APPROVED_SHA || undefined;
    const result = await validateEvidenceBundle(bundle, { approvedSha });
    for (const error of result.errors) console.error(`FAIL ${error}`);
    if (result.ok) console.log(`PASS phase-13 evidence bundle validated (${evidencePath})`);
    process.exitCode = result.ok ? 0 : 1;
    return;
  }

  // No --evidence bundle supplied: validate the plan-derived command contract
  // itself plus whatever individual typed evidence/*.json files already
  // exist in the phase evidence directory. This is a lighter, always-runnable
  // sanity check; the full sign-off gate requires --evidence.
  const { contract, errors: contractErrors } = await derivePlanCommandContract();
  const errors = [...contractErrors];
  for (const [fileName, validator] of Object.entries(DEFAULT_MODE_CATEGORY_FILES)) {
    try {
      const source = await readFile(resolve(EVIDENCE_DIR, fileName), "utf8");
      const evidence = JSON.parse(source);
      errors.push(...validator(evidence).map((error) => `${fileName}: ${error}`));
    } catch (error) {
      if (error?.code === "ENOENT") continue; // not produced yet by a later plan
      errors.push(`${fileName}: ${error instanceof Error ? error.message : "unreadable/malformed evidence"}`);
    }
  }
  console.log(`Derived ${contract.size} plan-verified task commands from ${PHASE_DIR}`);
  for (const error of errors) console.error(`FAIL ${error}`);
  if (errors.length === 0) console.log("PASS available phase-13 evidence validated (no --evidence bundle supplied; full sign-off gate requires one)");
  process.exitCode = errors.length ? 1 : 0;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main();
