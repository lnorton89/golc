#!/usr/bin/env node
// Phase 13 final sign-off: waiver-aware full evidence-bundle validation.
//
// WHY THIS FILE EXISTS (13-39, third/final attempt): the real, shipped validator at
// frontend/scripts/design-system/validate-phase13-evidence.mjs requires every one of the
// 77 plan-derived task rows to exit exactly 0 against its own literal, historically-authored
// command. 31 of those 77 literal commands now genuinely, reproducibly exit non-zero for a
// disclosed, non-functional-regression, structural reason: Plan 13-19's exceptions.json
// consolidation (27 rows + 13-19-01 itself) and 13-18-02's pre-existing missing `-tags mage`
// (1 row) made those specific historical command arguments stale, plus two rows (13-35-03,
// 13-38-01) whose literal commands intentionally validate only their own narrower evidence
// file and were never designed to pass the full-bundle contract alone (full assembly is this
// plan's own job). The application and design system are independently proven correct today
// (see the corrected re-verifications below and 13-VALIDATION.md's own premise re-check).
//
// The user was presented this exact situation via AskUserQuestion and explicitly chose
// "Accept current-state proof as sufficient" over the two alternatives ("fix each historical
// command" / "pause here"). This file is the MECHANICAL encoding of that explicit decision.
//
// It deliberately does NOT modify frontend/scripts/design-system/validate-phase13-evidence.mjs
// itself -- doing so would touch a non-`.planning/**` path, which would invalidate the
// CI-proven implementation-tree identity this session independently re-confirmed holds
// (commit 15af2730 / Windows CI run 30932536266 / 13-VALIDATION.md's Blocker A section).
// Instead, every semantic/category validator below is imported UNMODIFIED from the real
// shipped module and reused byte-for-byte. The ONLY new logic here is a documented
// "structural waiver" concept layered on top of the real validateResultRow(), which can
// excuse a single row's genuinely non-zero exitCode ONLY when:
//   1. every other validateResultRow() check on that row (command, commandSha256, plan,
//      wave, timestamps, repositoryCommitSha, dirty, environment, build, artifacts) already
//      passes -- a waiver NEVER excuses command/hash drift, only the exitCode clause;
//   2. a fully-specified waiver exists citing a non-zero originalExitCode (never fabricated
//      as if the row itself passed) that matches the row's own real, historical exitCode;
//   3. the waiver cites `reason` (prose) and `supersededBy` (a commit/plan reference) for a
//      genuine structural cause a future reader can verify independently;
//   4. the waiver's own `correctedCommand` was actually re-run this session and genuinely,
//      currently exits 0 (`correctedExitCode: 0`), with real timestamps.
//
// A row's own historical command/commandSha256/exitCode fields are never altered by this
// script -- see phase13-signoff-bundle.json's rows[]: every FAIL row still shows its true,
// non-zero exitCode. Only the "must equal 0" requirement is conditionally satisfied by a
// waiver instead of a literal re-run of the (superseded) historical command.

import { readFile } from "node:fs/promises";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { execFileSync } from "node:child_process";
import {
  PHASE_DIR,
  derivePlanCommandContract,
  validateResultRow,
  sha256,
  validateCalibrationEvidence,
  validateMaskAudit,
  validatePackagedWebView2Evidence,
  validateWindowsCiEvidence,
  validateImplementationTreeIdentity,
  validateBackstopStartupTheme,
  validateBackstopErrorBoundary,
  validateBackstopSpecializedGeometry,
  validateBackstopExpandedCopy,
  validateBackstopTextZoom,
  validateBackstopOfflineSafety,
  validateExactCoverage,
  REQUIREMENT_IDS,
  BACKSTOP_IDS,
} from "../../../../frontend/scripts/design-system/validate-phase13-evidence.mjs";

const HERE = dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = resolve(HERE, "../../../..");

function isIsoTimestamp(value) {
  return typeof value === "string" && value.length > 0 && !Number.isNaN(Date.parse(value));
}

/**
 * Validates the shape of one structural waiver. Every field is mechanically checked; a
 * missing/malformed/still-failing waiver excuses nothing (fail closed, matching the real
 * validator's own discipline).
 */
export function validateStructuralWaiver(waiver, taskId) {
  const errors = [];
  const label = `structuralWaivers[${taskId}]`;
  if (!waiver || typeof waiver !== "object") return [`${label}: waiver must be an object`];
  if (typeof waiver.reason !== "string" || !waiver.reason.trim()) {
    errors.push(`${label}: reason missing (must explain, in prose, why the historical command no longer reflects current reality)`);
  }
  if (typeof waiver.supersededBy !== "string" || !waiver.supersededBy.trim()) {
    errors.push(`${label}: supersededBy citation missing (must cite the commit/plan that changed the underlying reality)`);
  }
  if (typeof waiver.originalExitCode !== "number" || waiver.originalExitCode === 0) {
    errors.push(`${label}: originalExitCode must be recorded and non-zero (never fabricate a passing original result)`);
  }
  if (typeof waiver.correctedCommand !== "string" || !waiver.correctedCommand.trim()) {
    errors.push(`${label}: correctedCommand missing`);
  } else if (waiver.correctedCommandSha256 !== sha256(waiver.correctedCommand)) {
    errors.push(`${label}: correctedCommandSha256 does not match sha256(correctedCommand)`);
  }
  if (waiver.correctedExitCode !== 0) {
    errors.push(`${label}: correctedExitCode must be exactly 0 (a waiver can never excuse a still-failing check)`);
  }
  if (!isIsoTimestamp(waiver.correctedStartedAt) || !isIsoTimestamp(waiver.correctedCompletedAt)) {
    errors.push(`${label}: corrected re-verification timestamps missing/invalid`);
  }
  return errors;
}

function validateRowWithWaiver(row, contract, waiver) {
  const baseErrors = validateResultRow(row, contract); // real, unmodified function
  const exitCodePrefix = row?.taskId ? `${row.taskId}: exitCode must be exactly 0` : "\0no-taskid";
  const hasExitCodeIssue = baseErrors.some((e) => e.startsWith(exitCodePrefix));

  if (!hasExitCodeIssue) {
    if (waiver) {
      return [`structuralWaivers[${row.taskId}]: waiver present but row.exitCode is already 0 -- unused/unnecessary waiver`];
    }
    return baseErrors;
  }

  // There IS a genuine exitCode!==0 problem on this row.
  if (!waiver) return baseErrors; // no waiver supplied -- the hard failure stands, unmodified

  const waiverErrors = validateStructuralWaiver(waiver, row.taskId);
  if (waiver.originalExitCode !== row.exitCode) {
    waiverErrors.push(`${row.taskId}: structural waiver originalExitCode (${JSON.stringify(waiver.originalExitCode)}) does not match this row's own exitCode (${JSON.stringify(row.exitCode)})`);
  }
  if (waiverErrors.length) return [...baseErrors, ...waiverErrors]; // invalid waiver -- keep hard failure AND report why the waiver itself is invalid

  // Valid waiver: excuse exactly the exitCode clause, keep every other real check's errors (e.g. command/hash drift stays fatal, never excused).
  return baseErrors.filter((e) => !e.startsWith(exitCodePrefix));
}

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

const BACKSTOP_VALIDATORS = [
  ["startupTheme", validateBackstopStartupTheme, "startup-theme-font-before-settle"],
  ["errorBoundary", validateBackstopErrorBoundary, "error-boundary-before-theme-css"],
  ["specializedGeometry", validateBackstopSpecializedGeometry, "specialized-geometry-900-1280"],
  ["expandedCopy", validateBackstopExpandedCopy, "expanded-copy-2x-reflow"],
  ["textZoom", validateBackstopTextZoom, "text-zoom-200-900x720"],
  ["offlineSafety", validateBackstopOfflineSafety, "provider-daemon-offline-safety"],
];

/**
 * Waiver-aware equivalent of the real validateEvidenceBundle(). Every category besides the
 * per-row exitCode clause is validated by the real, unmodified functions -- see the header
 * comment for exactly what the waiver layer does and does not excuse.
 */
export async function validateEvidenceBundleWithWaivers(bundle, options = {}) {
  const errors = [];
  if (!bundle || typeof bundle !== "object") return { ok: false, errors: ["evidence bundle must be an object"] };

  const { contract } = options.contract ? { contract: options.contract } : await derivePlanCommandContract(options.phaseDir ?? PHASE_DIR);
  const structuralWaivers = bundle.structuralWaivers && typeof bundle.structuralWaivers === "object" ? bundle.structuralWaivers : {};

  if (!Array.isArray(bundle.rows)) {
    errors.push("evidence bundle missing rows[]");
  } else {
    const seen = new Set();
    for (const row of bundle.rows) {
      if (row?.taskId && seen.has(row.taskId)) errors.push(`duplicate row for task ${row.taskId}`);
      if (row?.taskId) seen.add(row.taskId);
      errors.push(...validateRowWithWaiver(row, contract, row?.taskId ? structuralWaivers[row.taskId] : undefined));
    }
    const missingTasks = [...contract.keys()].filter((taskId) => !seen.has(taskId));
    if (missingTasks.length) errors.push(`evidence bundle missing rows for tasks: ${missingTasks.join(", ")}`);
    const unusedWaivers = Object.keys(structuralWaivers).filter((taskId) => !seen.has(taskId));
    if (unusedWaivers.length) errors.push(`structuralWaivers reference taskIds with no matching row: ${unusedWaivers.join(", ")}`);
  }

  if (bundle.calibration) errors.push(...validateCalibrationEvidence(bundle.calibration).map((e) => `calibration: ${e}`));
  else errors.push("evidence bundle missing calibration evidence");

  if (Array.isArray(bundle.masks)) errors.push(...validateMaskAudit(bundle.masks).map((e) => `mask audit: ${e}`));
  else errors.push("evidence bundle missing mask audit");

  if (bundle.packagedWebView2) {
    errors.push(...validatePackagedWebView2Evidence(bundle.packagedWebView2, { expectedExecutableSha256: bundle.expectedExecutableSha256 }).map((e) => `packaged WebView2: ${e}`));
  } else {
    errors.push("evidence bundle missing packaged WebView2 evidence");
  }

  if (bundle.windowsCi) errors.push(...validateWindowsCiEvidence(bundle.windowsCi, options).map((e) => `Windows CI: ${e}`));
  else errors.push("evidence bundle missing Windows CI evidence");

  if (bundle.implementationTree) {
    const gitRunner = options.gitRunner ?? realGitRunner(options.gitRoot ?? REPO_ROOT);
    errors.push(...validateImplementationTreeIdentity({ ...bundle.implementationTree, gitRunner }).map((e) => `implementation tree: ${e}`));
  } else {
    errors.push("evidence bundle missing CI implementation-tree identity");
  }

  for (const [key, validator, label] of BACKSTOP_VALIDATORS) {
    const evidence = bundle.backstops?.[key];
    if (evidence) errors.push(...validator(evidence).map((e) => `${label}: ${e}`));
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

async function main() {
  const bundlePath = resolve(HERE, "phase13-signoff-bundle.json");
  const bundle = JSON.parse(await readFile(bundlePath, "utf8"));
  const result = await validateEvidenceBundleWithWaivers(bundle);
  for (const error of result.errors) console.error(`FAIL ${error}`);
  console.log(result.ok ? "PASS phase-13 full evidence bundle validated WITH documented structural waivers (see reason/supersededBy per waiver)" : "FAIL (see above)");
  process.exitCode = result.ok ? 0 : 1;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main();
