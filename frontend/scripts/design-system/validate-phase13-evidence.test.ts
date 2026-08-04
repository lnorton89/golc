import { describe, expect, it } from "vitest";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import {
  BACKSTOP_IDS,
  EVIDENCE_DIR,
  PHASE_DIR,
  REQUIREMENT_IDS,
  collectAssertionErrors,
  computeImplementationManifest,
  decodeXmlEntities,
  derivePlanCommandContract,
  normalizeCommand,
  parseCommandLineArgs,
  parsePlanTasks,
  parseStrictJson,
  sha256,
  validateArtifact,
  validateBackstopErrorBoundary,
  validateBackstopExpandedCopy,
  validateBackstopOfflineSafety,
  validateBackstopSpecializedGeometry,
  validateBackstopStartupTheme,
  validateBackstopTextZoom,
  validateCalibrationEvidence,
  validateDialogFeasibilityEvidence,
  validateEvidenceBundle,
  validateExactCoverage,
  validateImplementationTreeIdentity,
  validateMaskAudit,
  validatePackagedWebView2Evidence,
  validateResultRow,
  validateWindowsCiEvidence,
} from "./validate-phase13-evidence.mjs";

async function loadEvidence(name: string) {
  return JSON.parse(await readFile(resolve(EVIDENCE_DIR, name), "utf8"));
}

function deepClone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value));
}

// ---------------------------------------------------------------------------
// Command normalization, plan-derived task/command/hash contract
// ---------------------------------------------------------------------------

describe("command normalization and hashing", () => {
  it("decodes XML entities exactly", () => {
    expect(decodeXmlEntities("cd frontend &amp;&amp; npm test &lt;file&gt; &quot;x&quot; &apos;y&apos;")).toBe(
      'cd frontend && npm test <file> "x" \'y\'',
    );
  });

  it("normalizes CRLF/CR to LF and trims surrounding whitespace once, without collapsing internal whitespace", () => {
    expect(normalizeCommand("  cd frontend &amp;&amp; npm test\r\n  ")).toBe("cd frontend && npm test");
    expect(normalizeCommand("cd frontend &amp;&amp;   npm test")).toBe("cd frontend &&   npm test");
  });

  it("hashes identical normalized commands to the same SHA-256 and rejects whitespace/entity boundary drift", () => {
    const a = sha256(normalizeCommand("cd frontend &amp;&amp; npm test")!);
    const b = sha256(normalizeCommand("cd frontend &amp;&amp; npm test\r\n")!);
    const c = sha256(normalizeCommand("cd frontend && npm test ")!);
    expect(a).toBe(b);
    expect(a).toBe(c);
    const d = sha256(normalizeCommand("cd frontend  &amp;&amp; npm test")!); // extra internal space
    expect(d).not.toBe(a);
  });

  it("returns null for non-string or empty-after-trim input", () => {
    expect(normalizeCommand(undefined as unknown as string)).toBeNull();
    expect(normalizeCommand("   \r\n  ")).toBeNull();
  });
});

describe("plan task parsing", () => {
  const fixturePlan = `---
plan: 99
wave: 3
---
<tasks>
<task type="checkpoint:human-verify" gate="blocking-human">
  <name>Task 1: Approve something</name>
  <verify><automated>echo one &amp;&amp; echo two</automated></verify>
</task>
<task type="auto" tdd="true">
  <name>Task 2: Do the thing</name>
  <verify><automated>cd frontend &amp;&amp; npx vitest run x.test.ts</automated></verify>
</task>
<task type="checkpoint:human-verify" gate="blocking-human">
  <name>Task 3: No automated verify, human-only</name>
</task>
</tasks>`;

  it("derives 1-based task position among ALL task elements, including checkpoints with their own verify command", () => {
    const tasks = parsePlanTasks("13-99", fixturePlan);
    expect(tasks).toHaveLength(3);
    expect(tasks[0]).toMatchObject({ taskId: "13-99-01", position: 1, type: "checkpoint:human-verify", command: "echo one && echo two" });
    expect(tasks[1]).toMatchObject({ taskId: "13-99-02", position: 2, type: "auto", tdd: true, command: "cd frontend && npx vitest run x.test.ts" });
    expect(tasks[2]).toMatchObject({ taskId: "13-99-03", position: 3, command: null, commandSha256: null });
    expect(tasks[0].wave).toBe(3);
  });

  it("returns an empty list when no <tasks> block is present", () => {
    expect(parsePlanTasks("13-99", "no tasks block here")).toEqual([]);
  });
});

describe("plan-derived command contract (real PLAN.md corpus)", () => {
  it("derives exactly 77 tasks across the phase's 41 PLAN.md files, matching 13-VALIDATION.md's declared task_count", async () => {
    const { contract, errors } = await derivePlanCommandContract(PHASE_DIR);
    expect(errors).toEqual([]);
    expect(contract.size).toBe(77);
  });

  it("derives the exact 13-20-01 and 13-20-02 commands and hashes matching this plan's own <verify> blocks", async () => {
    const { contract } = await derivePlanCommandContract(PHASE_DIR);
    const task1 = contract.get("13-20-01")!;
    const task2 = contract.get("13-20-02")!;
    expect(task1.command).toBe("cd frontend && npx vitest run scripts/design-system/validate-phase13-evidence.test.ts");
    expect(task1.commandSha256).toBe(sha256(task1.command!));
    expect(task2.command).toContain('--testNamePattern="mutation|false sign-off|semantic|implementation tree|zoom|offline safety"');
    expect(task2.wave).toBe(11);
  });

  it("derives a checkpoint task's own automated preflight command (13-01-01, 13-35-01)", async () => {
    const { contract } = await derivePlanCommandContract(PHASE_DIR);
    const preflight = contract.get("13-01-01")!;
    expect(preflight.type).toBe("checkpoint:human-verify");
    expect(preflight.command).toContain("npm view postcss@8.5.22");
    const windowsPreflight = contract.get("13-35-01")!;
    expect(windowsPreflight.type).toBe("checkpoint:decision");
    expect(windowsPreflight.command).toContain("gh workflow view design-system.yml");
  });
});

// ---------------------------------------------------------------------------
// Closed result-row / artifact schema
// ---------------------------------------------------------------------------

describe("closed result-row schema", () => {
  const derivedTask = { taskId: "13-20-01", plan: "13-20", wave: 11, command: "cd frontend && npx vitest run x.test.ts", commandSha256: sha256("cd frontend && npx vitest run x.test.ts") };
  const contract = new Map([[derivedTask.taskId, derivedTask]]);

  function validRow() {
    return {
      taskId: "13-20-01",
      plan: "13-20",
      wave: 11,
      command: derivedTask.command,
      commandSha256: derivedTask.commandSha256,
      exitCode: 0,
      startedAt: "2026-08-03T10:00:00.000Z",
      completedAt: "2026-08-03T10:00:05.000Z",
      repositoryCommitSha: "a".repeat(40),
      dirty: false,
      environment: { os: "win32", runtime: "node-22" },
      build: { identity: "golc-desktop.exe@abc123" },
      artifacts: [{ path: "evidence/x.json", mediaType: "application/json", byteCount: 12, sha256: "b".repeat(64) }],
    };
  }

  it("accepts a fully closed row with zero errors", () => {
    expect(validateResultRow(validRow(), contract)).toEqual([]);
  });

  it("rejects an unknown/stale/extra task id row", () => {
    const row = { ...validRow(), taskId: "13-99-99" };
    expect(validateResultRow(row, contract).some((e) => e.includes("unknown task id"))).toBe(true);
  });

  it("rejects every missing required field independently", () => {
    for (const field of ["exitCode", "startedAt", "completedAt", "repositoryCommitSha", "dirty", "environment", "build", "artifacts"]) {
      const row: any = validRow();
      delete row[field];
      const errors = validateResultRow(row, contract);
      expect(errors.length, `missing ${field} should fail closed`).toBeGreaterThan(0);
    }
  });

  it("rejects artifacts with unresolvable/absolute/traversal paths and malformed hashes", () => {
    expect(validateArtifact({ path: "/etc/passwd", mediaType: "text/plain", byteCount: 1, sha256: "b".repeat(64) }, "x").length).toBeGreaterThan(0);
    expect(validateArtifact({ path: "../outside.json", mediaType: "text/plain", byteCount: 1, sha256: "b".repeat(64) }, "x").length).toBeGreaterThan(0);
    expect(validateArtifact({ path: "C:\\Windows\\x.json", mediaType: "text/plain", byteCount: 1, sha256: "b".repeat(64) }, "x").length).toBeGreaterThan(0);
    expect(validateArtifact({ path: "evidence/x.json", mediaType: "text/plain", byteCount: 1, sha256: "not-a-hash" }, "x").length).toBeGreaterThan(0);
    expect(validateArtifact({ path: "https://example.com/x.png", mediaType: "image/png", byteCount: 1, sha256: "b".repeat(64) }, "x")).toEqual([]);
  });
});

// ---------------------------------------------------------------------------
// Semantic evidence: calibration, mask audit, packaged WebView2
// ---------------------------------------------------------------------------

describe("semantic calibration arithmetic", () => {
  it("accepts the real committed calibration evidence and recomputes its selected threshold", async () => {
    const evidence = await loadEvidence("screenshot-calibration.json");
    expect(validateCalibrationEvidence(evidence)).toEqual([]);
  });

  it("rejects tampered pairwise arithmetic (declared maxRatio no longer matches recomputed max)", async () => {
    const evidence = deepClone(await loadEvidence("screenshot-calibration.json"));
    evidence.states[0].maxRatio = 0.5; // forged, does not match recomputed pairwiseDiffs max
    expect(validateCalibrationEvidence(evidence).some((e) => e.includes("does not equal recomputed max"))).toBe(true);
  });

  it("rejects a selectedThreshold above the declared ceiling", async () => {
    const evidence = deepClone(await loadEvidence("screenshot-calibration.json"));
    evidence.selectedThreshold = evidence.ceiling + 0.5;
    for (const state of evidence.states) state.maxRatio = evidence.selectedThreshold;
    for (const state of evidence.states) for (const diff of state.pairwiseDiffs) diff.smallestPassingRatio = evidence.selectedThreshold;
    expect(validateCalibrationEvidence(evidence).some((e) => e.includes("exceeds ceiling"))).toBe(true);
  });

  it("rejects fewer than three independent captures per state", async () => {
    const evidence = deepClone(await loadEvidence("screenshot-calibration.json"));
    evidence.states[0].captures = evidence.states[0].captures.slice(0, 2);
    expect(validateCalibrationEvidence(evidence).some((e) => e.includes("at least 3"))).toBe(true);
  });
});

describe("mask audit: protected-region intersection", () => {
  const blackout = { name: "Blackout", x: 700, y: 30, width: 120, height: 50 };

  it("accepts masks with zero protected intersections", () => {
    const masks = [{ rectangle: { x: 0, y: 0, width: 50, height: 50 }, reason: "clock", screenshot: "shell.png", protectedLocatorRectangles: [blackout] }];
    expect(validateMaskAudit(masks)).toEqual([]);
  });

  it("rejects a mask rectangle that intersects a protected locator (Blackout)", () => {
    const masks = [{ rectangle: { x: 710, y: 35, width: 40, height: 20 }, reason: "flaky timestamp", screenshot: "shell.png", protectedLocatorRectangles: [blackout] }];
    const errors = validateMaskAudit(masks);
    expect(errors.some((e) => e.includes("intersects protected locator"))).toBe(true);
  });

  it("rejects masks missing a reason or screenshot reference", () => {
    const masks = [{ rectangle: { x: 0, y: 0, width: 10, height: 10 }, reason: "", screenshot: "" }];
    const errors = validateMaskAudit(masks);
    expect(errors.some((e) => e.includes("reason missing"))).toBe(true);
    expect(errors.some((e) => e.includes("screenshot reference missing"))).toBe(true);
  });
});

describe("packaged WebView2 evidence (semantic, not existence-only)", () => {
  it("accepts the real committed dialog-feasibility evidence", async () => {
    const evidence = await loadEvidence("dialog-feasibility.json");
    expect(validateDialogFeasibilityEvidence(evidence)).toEqual([]);
  });

  it("rejects an environment-only harness masquerading as passed (status not passed, or an error recorded)", async () => {
    const evidence = deepClone(await loadEvidence("dialog-feasibility.json"));
    evidence.status = "failed";
    expect(validateDialogFeasibilityEvidence(evidence).some((e) => e.includes('expected "passed"'))).toBe(true);
    const withError = deepClone(await loadEvidence("dialog-feasibility.json"));
    withError.error = "CDP endpoint refused";
    expect(validateDialogFeasibilityEvidence(withError).some((e) => e.includes("recorded an error"))).toBe(true);
  });

  it("rejects a packaged executable hash mismatch against the expected application build hash", async () => {
    const evidence = await loadEvidence("dialog-feasibility.json");
    const errors = validatePackagedWebView2Evidence(evidence, { expectedExecutableSha256: "0".repeat(64) });
    expect(errors.some((e) => e.includes("does not match the expected application build hash"))).toBe(true);
  });

  it("rejects missing runtime.cdp_endpoint (no proof of real CDP endpoint ownership)", async () => {
    const evidence = deepClone(await loadEvidence("dialog-feasibility.json"));
    delete evidence.runtime.cdp_endpoint;
    expect(validateDialogFeasibilityEvidence(evidence).some((e) => e.includes("cdp_endpoint"))).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// Six separately named UI backstops -- semantic, not existence-only
// ---------------------------------------------------------------------------

describe("semantic backstop: startup theme/font", () => {
  it("accepts the real committed evidence", async () => {
    expect(validateBackstopStartupTheme(await loadEvidence("startup-theme-font.json"))).toEqual([]);
  });

  it("rejects a single flipped assertion (semantic false-positive rejection)", async () => {
    const evidence = deepClone(await loadEvidence("startup-theme-font.json"));
    evidence.themes[0].assertions.neverTransparentBackground = false;
    expect(validateBackstopStartupTheme(evidence).some((e) => e.includes("neverTransparentBackground"))).toBe(true);
  });

  it("rejects a missing per-frame sample timeline", async () => {
    const evidence = deepClone(await loadEvidence("startup-theme-font.json"));
    evidence.themes[0].themeSequence = [];
    expect(validateBackstopStartupTheme(evidence).some((e) => e.includes("themeSequence missing"))).toBe(true);
  });
});

describe("semantic backstop: error boundary before theme CSS", () => {
  it("accepts the real committed evidence", async () => {
    expect(validateBackstopErrorBoundary(await loadEvidence("error-boundary-fallback.json"))).toEqual([]);
  });

  it("rejects a viewport missing proof the token stylesheet was genuinely blocked", async () => {
    const evidence = deepClone(await loadEvidence("error-boundary-fallback.json"));
    evidence.viewports[0].blockedStylesheet = null;
    expect(validateBackstopErrorBoundary(evidence).some((e) => e.includes("blockedStylesheet"))).toBe(true);
  });
});

describe("semantic backstop: specialized geometry at 900/1280", () => {
  it("accepts the real committed evidence", async () => {
    expect(validateBackstopSpecializedGeometry(await loadEvidence("specialized-geometry.json"))).toEqual([]);
  });

  it("rejects a case marked failed while allCasesPassed still claims true", async () => {
    const evidence = deepClone(await loadEvidence("specialized-geometry.json"));
    evidence.cases[0].passed = false;
    expect(validateBackstopSpecializedGeometry(evidence).some((e) => e.includes("did not pass"))).toBe(true);
    expect(validateBackstopSpecializedGeometry(evidence).some((e) => e.includes("does not match the recomputed AND"))).toBe(true);
  });

  it("rejects an out-of-contract width", async () => {
    const evidence = deepClone(await loadEvidence("specialized-geometry.json"));
    evidence.cases[0].width = 1024;
    expect(validateBackstopSpecializedGeometry(evidence).some((e) => e.includes("must be exactly 900 or 1280"))).toBe(true);
  });
});

describe("semantic backstop: expanded copy 2x reflow", () => {
  it("accepts the real committed evidence", async () => {
    expect(validateBackstopExpandedCopy(await loadEvidence("expanded-copy.json"))).toEqual([]);
  });

  it("rejects a pair whose expansion ratio drops below the 2.0 minimum", async () => {
    const evidence = deepClone(await loadEvidence("expanded-copy.json"));
    evidence.cases[0].expansionRatio = 1.5;
    expect(validateBackstopExpandedCopy(evidence).some((e) => e.includes("is below the minimum"))).toBe(true);
  });

  it("rejects forged canonical/expanded arithmetic (declared ratio does not match recomputed grapheme ratio)", async () => {
    const evidence = deepClone(await loadEvidence("expanded-copy.json"));
    evidence.pairs[0].expansionRatio = 999;
    expect(validateBackstopExpandedCopy(evidence).some((e) => e.includes("does not match declared"))).toBe(true);
  });
});

describe("semantic backstop: 200% text zoom at 900x720", () => {
  it("accepts the real committed evidence", async () => {
    expect(validateBackstopTextZoom(await loadEvidence("text-zoom-200.json"))).toEqual([]);
  });

  it("rejects a positive body overflow at 200% zoom", async () => {
    const evidence = deepClone(await loadEvidence("text-zoom-200.json"));
    evidence.overflow.bodyOverflows = true;
    expect(validateBackstopTextZoom(evidence).some((e) => e.includes("do not overflow"))).toBe(true);
  });

  it("rejects the wrong viewport (900x720 is the exact required contract)", async () => {
    const evidence = deepClone(await loadEvidence("text-zoom-200.json"));
    evidence.viewport = { width: 1280, height: 720 };
    expect(validateBackstopTextZoom(evidence).some((e) => e.includes("900x720"))).toBe(true);
  });
});

describe("semantic backstop: provider/daemon offline safety", () => {
  it("accepts the real committed evidence", async () => {
    expect(validateBackstopOfflineSafety(await loadEvidence("offline-safety.json"))).toEqual([]);
  });

  it("rejects offline status that falsely infers stopped playback/output from connectivity loss", async () => {
    const evidence = deepClone(await loadEvidence("offline-safety.json"));
    evidence.states[0].after.outputText = "stopped";
    const errors = validateBackstopOfflineSafety(evidence);
    expect(errors.some((e) => e.includes("outputText") && e.includes("Go-owned playback/output truth"))).toBe(true);
  });

  it("rejects forbidden copy findings (must be an empty array)", async () => {
    const evidence = deepClone(await loadEvidence("offline-safety.json"));
    evidence.states[0].forbiddenCopyFound = ["Output stopped"];
    expect(validateBackstopOfflineSafety(evidence).some((e) => e.includes("forbiddenCopyFound"))).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// Windows CI evidence
// ---------------------------------------------------------------------------

function validWindowsCiEvidence(headSha = "c".repeat(40)) {
  return {
    runId: 123456,
    url: "https://github.com/lnorton89/golc/actions/runs/123456",
    workflowName: "design-system.yml",
    workflowPath: ".github/workflows/design-system.yml",
    event: "pull_request",
    headSha,
    headBranch: "worktree-agent-x",
    status: "completed",
    conclusion: "success",
    createdAt: "2026-08-03T10:00:00.000Z",
    updatedAt: "2026-08-03T10:05:00.000Z",
    jobs: [{ name: "design-system-windows", conclusion: "success" }],
    artifacts: [{ path: "https://example.com/artifact.zip", mediaType: "application/zip", byteCount: 1024, sha256: "d".repeat(64) }],
  };
}

describe("Windows CI run evidence", () => {
  it("accepts a well-formed run matching the approved SHA", () => {
    const sha = "c".repeat(40);
    expect(validateWindowsCiEvidence(validWindowsCiEvidence(sha), { approvedSha: sha })).toEqual([]);
  });

  it("rejects a workflow head SHA mismatch against the approved trigger SHA", () => {
    const errors = validateWindowsCiEvidence(validWindowsCiEvidence("c".repeat(40)), { approvedSha: "e".repeat(40) });
    expect(errors.some((e) => e.includes("does not match the approved trigger SHA"))).toBe(true);
  });

  it("rejects a non-completed or non-success run", () => {
    const evidence = { ...validWindowsCiEvidence(), status: "in_progress", conclusion: null };
    expect(validateWindowsCiEvidence(evidence).some((e) => e.includes("completed/success"))).toBe(true);
  });

  it("rejects a workflow path that is not the required design-system.yml", () => {
    const evidence = { ...validWindowsCiEvidence(), workflowPath: ".github/workflows/other.yml" };
    expect(validateWindowsCiEvidence(evidence).some((e) => e.includes("workflowPath"))).toBe(true);
  });

  it("rejects artifacts with a missing/invalid hash (schema violation)", () => {
    const evidence = { ...validWindowsCiEvidence(), artifacts: [{ path: "https://example.com/x.zip", mediaType: "application/zip", byteCount: 1 }] };
    expect(validateWindowsCiEvidence(evidence).some((e) => e.includes("sha256"))).toBe(true);
  });

  it("rejects a run with no successful job record", () => {
    const evidence = { ...validWindowsCiEvidence(), jobs: [{ name: "x", conclusion: "failure" }] };
    expect(validateWindowsCiEvidence(evidence).some((e) => e.includes("successful job"))).toBe(true);
  });
});

describe("collectAssertionErrors", () => {
  it("passes when every assertion is exactly true", () => {
    expect(collectAssertionErrors({ a: true, b: true }, "x")).toEqual([]);
  });
  it("fails when an assertion is any non-true value, including truthy strings", () => {
    expect(collectAssertionErrors({ a: true, b: "yes" as unknown as boolean }, "x").length).toBe(1);
    expect(collectAssertionErrors({}, "x").length).toBe(1);
    expect(collectAssertionErrors(null, "x").length).toBe(1);
  });
});

// ---------------------------------------------------------------------------
// Task 2: prove every false-sign-off mutation is rejected
// ---------------------------------------------------------------------------

describe("mutation: command/hash and exit-status boundaries", () => {
  const derivedTask = { taskId: "13-20-01", plan: "13-20", wave: 11, command: "cd frontend && npx vitest run x.test.ts", commandSha256: sha256("cd frontend && npx vitest run x.test.ts") };
  const contract = new Map([[derivedTask.taskId, derivedTask]]);
  function validRow() {
    return {
      taskId: "13-20-01",
      plan: "13-20",
      wave: 11,
      command: derivedTask.command,
      commandSha256: derivedTask.commandSha256,
      exitCode: 0,
      startedAt: "2026-08-03T10:00:00.000Z",
      completedAt: "2026-08-03T10:00:05.000Z",
      repositoryCommitSha: "a".repeat(40),
      dirty: false,
      environment: { os: "win32", runtime: "node-22" },
      build: { identity: "golc-desktop.exe@abc123" },
      artifacts: [],
    };
  }

  it("rejects a wrong-but-successful command substitution (single character drift)", () => {
    const row = { ...validRow(), command: derivedTask.command.replace("run x", "run  x") };
    expect(validateResultRow(row, contract).some((e) => e.includes("command"))).toBe(true);
  });

  it("rejects a command that matches but carries a forged/stale hash", () => {
    const row = { ...validRow(), commandSha256: "f".repeat(64) };
    expect(validateResultRow(row, contract).some((e) => e.includes("commandSha256"))).toBe(true);
  });

  it("rejects a non-zero exit status even when everything else is correct", () => {
    const row = { ...validRow(), exitCode: 1 };
    expect(validateResultRow(row, contract).some((e) => e.includes("exitCode"))).toBe(true);
  });

  it("rejects a stale repository commit SHA identity (non-hex or wrong length)", () => {
    expect(validateResultRow({ ...validRow(), repositoryCommitSha: "not-a-sha" }, contract).some((e) => e.includes("repositoryCommitSha"))).toBe(true);
  });

  it("rejects plan/wave mapping mismatches (task mapping mutation)", () => {
    expect(validateResultRow({ ...validRow(), plan: "13-99" }, contract).some((e) => e.includes("plan mismatch"))).toBe(true);
    expect(validateResultRow({ ...validRow(), wave: 999 }, contract).some((e) => e.includes("wave mismatch"))).toBe(true);
  });
});

describe("mutation: semantic backstop and calibration arithmetic forgery", () => {
  it("rejects a calibration selectedThreshold below the recomputed smallest-stable threshold", async () => {
    const evidence = deepClone(await loadEvidence("screenshot-calibration.json"));
    evidence.states[0].pairwiseDiffs[0].smallestPassingRatio = 0.9;
    evidence.states[0].maxRatio = 0.9;
    // selectedThreshold left at its original (lower) value -- now inconsistent with the new max.
    expect(validateCalibrationEvidence(evidence).some((e) => e.includes("selectedThreshold"))).toBe(true);
  });

  it("rejects duplicate capture identities within one calibration state", async () => {
    const evidence = deepClone(await loadEvidence("screenshot-calibration.json"));
    evidence.states[0].captures[1].id = evidence.states[0].captures[0].id;
    expect(validateCalibrationEvidence(evidence).some((e) => e.includes("duplicate capture ids"))).toBe(true);
  });

  it("rejects 100% zoom mislabeled as 200% zoom", async () => {
    const evidence = deepClone(await loadEvidence("text-zoom-200.json"));
    evidence.requestedZoom = "1";
    evidence.computedZoom = "1";
    expect(validateBackstopTextZoom(evidence).some((e) => e.includes("real 200% browser text zoom"))).toBe(true);
  });

  it("rejects an unreachable required safety/navigation locator at 200% zoom", async () => {
    const evidence = deepClone(await loadEvidence("text-zoom-200.json"));
    evidence.locators.blackout.passed = false;
    expect(validateBackstopTextZoom(evidence).some((e) => e.includes('locator "blackout"'))).toBe(true);
  });

  it("rejects a missing keyboard focus traversal record at 200% zoom", async () => {
    const evidence = deepClone(await loadEvidence("text-zoom-200.json"));
    evidence.focusTraversal = [];
    expect(validateBackstopTextZoom(evidence).some((e) => e.includes("focusTraversal"))).toBe(true);
  });

  it("rejects offline controls that are not independently keyboard-operable via their own local path", async () => {
    const evidence = deepClone(await loadEvidence("offline-safety.json"));
    evidence.states[0].controls.blackout.dispatchedExactlyOnce = false;
    expect(validateBackstopOfflineSafety(evidence).some((e) => e.includes('control "blackout"'))).toBe(true);
  });

  it("rejects a cross-dispatch leak (blackout and revoke must never share a dispatch path)", async () => {
    const evidence = deepClone(await loadEvidence("offline-safety.json"));
    evidence.states[0].controls.revokeAutomation.crossDispatchZero = false;
    expect(validateBackstopOfflineSafety(evidence).some((e) => e.includes('control "revokeAutomation"'))).toBe(true);
  });

  it("rejects a specialized-geometry/expanded-copy case with a flipped semantic assertion", async () => {
    const evidence = deepClone(await loadEvidence("expanded-copy.json"));
    const firstKey = Object.keys(evidence.cases[0].assertions)[0];
    evidence.cases[0].assertions[firstKey] = false;
    expect(validateBackstopExpandedCopy(evidence).length).toBeGreaterThan(0);
  });
});

function fakeGitRunner(commits: Record<string, { path: string; mode: string; objectId: string }[]>, ancestry: Record<string, string[]>) {
  return {
    lsTree(sha: string) {
      const entries = commits[sha];
      if (!entries) throw new Error(`unknown fixture commit ${sha}`);
      return entries;
    },
    diffNameOnly(a: string, b: string) {
      return ancestry[`${a}..${b}`] ?? [];
    },
    isAncestor(a: string, b: string) {
      return Boolean(ancestry[`${a}..${b}`]);
    },
  };
}

describe("implementation tree ancestry and non-planning manifest identity", () => {
  const provenSha = "1".repeat(40);
  const observedSha = "2".repeat(40);
  const nonAncestorSha = "3".repeat(40);

  const baseEntries = [
    { mode: "100644", objectId: "aaa", path: "internal/command/foo.go" },
    { mode: "100644", objectId: "bbb", path: "frontend/src/App.tsx" },
  ];

  it("accepts an identical commit (no drift) trivially", () => {
    const gitRunner = fakeGitRunner({ [provenSha]: baseEntries }, {});
    expect(validateImplementationTreeIdentity({ provenSha, observedSha: provenSha, gitRunner })).toEqual([]);
  });

  it("accepts a descendant whose only changes are under .planning/** and whose non-planning manifest hash is identical", () => {
    const commits = {
      [provenSha]: baseEntries,
      [observedSha]: [...baseEntries, { mode: "100644", objectId: "ccc", path: ".planning/phases/13-x/13-20-SUMMARY.md" }],
    };
    const ancestry = { [`${provenSha}..${observedSha}`]: [".planning/phases/13-x/13-20-SUMMARY.md"] };
    const gitRunner = fakeGitRunner(commits, ancestry);
    const proven = computeImplementationManifest(provenSha, gitRunner);
    const observed = computeImplementationManifest(observedSha, gitRunner);
    expect(proven.hash).toBe(observed.hash); // the .planning/** addition must not change the non-planning manifest
    expect(validateImplementationTreeIdentity({ provenSha, observedSha, gitRunner, declaredProvenHash: proven.hash, declaredObservedHash: observed.hash })).toEqual([]);
  });

  it("rejects a non-ancestor evidence commit", () => {
    const gitRunner = fakeGitRunner({ [provenSha]: baseEntries, [nonAncestorSha]: baseEntries }, {});
    const errors = validateImplementationTreeIdentity({ provenSha, observedSha: nonAncestorSha, gitRunner });
    expect(errors.some((e) => e.includes("does not descend"))).toBe(true);
  });

  it("rejects a changed frontend/runtime path outside the .planning/** allowlist after the proven SHA", () => {
    const commits = {
      [provenSha]: baseEntries,
      [observedSha]: [{ mode: "100644", objectId: "zzz", path: "internal/command/foo.go" }, { mode: "100644", objectId: "bbb", path: "frontend/src/App.tsx" }],
    };
    const ancestry = { [`${provenSha}..${observedSha}`]: ["internal/command/foo.go"] }; // changed OUTSIDE .planning/**
    const gitRunner = fakeGitRunner(commits, ancestry);
    const errors = validateImplementationTreeIdentity({ provenSha, observedSha, gitRunner });
    expect(errors.some((e) => e.includes("non-.planning/** paths changed"))).toBe(true);
  });

  it("rejects a forged declared implementation-tree manifest hash", () => {
    const gitRunner = fakeGitRunner({ [provenSha]: baseEntries }, {});
    const errors = validateImplementationTreeIdentity({ provenSha, observedSha: provenSha, gitRunner, declaredProvenHash: "forgedhash" });
    expect(errors.some((e) => e.includes("does not match the recomputed manifest hash"))).toBe(true);
  });

  it("rejects mismatched non-planning manifests between proven and observed commits even when ancestry/allowlist hold", () => {
    const commits = {
      [provenSha]: baseEntries,
      [observedSha]: [{ mode: "100644", objectId: "different-blob", path: "internal/command/foo.go" }, { mode: "100644", objectId: "bbb", path: "frontend/src/App.tsx" }],
    };
    // Simulate the diff tool only reporting a .planning/** change while the ls-tree blob for a
    // non-planning path was tampered with -- the recomputed manifest hash must still catch it.
    const ancestry = { [`${provenSha}..${observedSha}`]: [".planning/phases/13-x/note.md"] };
    const gitRunner = fakeGitRunner(commits, ancestry);
    const errors = validateImplementationTreeIdentity({ provenSha, observedSha, gitRunner });
    expect(errors.some((e) => e.includes("manifest hash differs"))).toBe(true);
  });

  it("computeImplementationManifest excludes every .planning/** path from the hashed manifest", () => {
    const gitRunner = fakeGitRunner({
      [provenSha]: [...baseEntries, { mode: "100644", objectId: "ddd", path: ".planning/STATE.md" }],
    }, {});
    const manifest = computeImplementationManifest(provenSha, gitRunner);
    expect(manifest.entries.some((entry) => entry.path.startsWith(".planning/"))).toBe(false);
    expect(manifest.entries).toHaveLength(2);
  });

  it("rejects a malformed proven/observed SHA (not 40-hex)", () => {
    expect(validateImplementationTreeIdentity({ provenSha: "short", observedSha, gitRunner: fakeGitRunner({}, {}) }).length).toBeGreaterThan(0);
  });
});

describe("requirement and backstop coverage exactness", () => {
  it("accepts the exact canonical D-01..D-14/UI-SPEC list", () => {
    expect(validateExactCoverage(REQUIREMENT_IDS, REQUIREMENT_IDS, "requirements")).toEqual([]);
  });

  it("rejects missing coverage for any single requirement", () => {
    const incomplete = REQUIREMENT_IDS.filter((id) => id !== "D-13");
    expect(validateExactCoverage(incomplete, REQUIREMENT_IDS, "requirements").some((e) => e.includes("D-13"))).toBe(true);
  });

  it("rejects unknown/extra coverage entries", () => {
    const withExtra = [...REQUIREMENT_IDS, "D-99"];
    expect(validateExactCoverage(withExtra, REQUIREMENT_IDS, "requirements").some((e) => e.includes("D-99"))).toBe(true);
  });

  it("rejects duplicate coverage entries", () => {
    const withDuplicate = [...REQUIREMENT_IDS, "D-01"];
    expect(validateExactCoverage(withDuplicate, REQUIREMENT_IDS, "requirements").some((e) => e.includes("duplicate"))).toBe(true);
  });

  it("accepts and rejects the six separately named backstops the same way", () => {
    expect(validateExactCoverage(BACKSTOP_IDS, BACKSTOP_IDS, "backstops")).toEqual([]);
    expect(validateExactCoverage(BACKSTOP_IDS.slice(0, 5), BACKSTOP_IDS, "backstops").length).toBeGreaterThan(0);
  });
});

describe("strict JSON parsing (malformed/duplicate-key fail-closed)", () => {
  it("parses well-formed JSON identically to JSON.parse", () => {
    const source = '{"a":1,"b":[1,2,"x"],"c":{"d":true,"e":null}}';
    expect(parseStrictJson(source, "fixture")).toEqual(JSON.parse(source));
  });

  it("rejects a duplicate object key with a stable diagnostic", () => {
    expect(() => parseStrictJson('{"a":1,"a":2}', "fixture")).toThrow(/duplicate key "a"/);
  });

  it("rejects a duplicate key nested inside an array of objects", () => {
    expect(() => parseStrictJson('{"rows":[{"taskId":"x","taskId":"y"}]}', "fixture")).toThrow(/duplicate key "taskId"/);
  });

  it("rejects malformed/truncated JSON with a positional diagnostic", () => {
    expect(() => parseStrictJson('{"a":1,', "fixture")).toThrow(/PHASE13_EVIDENCE_MALFORMED_JSON/);
    expect(() => parseStrictJson("not json at all", "fixture")).toThrow(/PHASE13_EVIDENCE_MALFORMED_JSON/);
    expect(() => parseStrictJson('{"a":1}trailing', "fixture")).toThrow(/trailing content/);
  });
});

describe("CLI argument parsing", () => {
  it("accepts no arguments (light validation mode)", () => {
    expect(parseCommandLineArgs([])).toEqual({ evidencePath: null });
  });
  it("accepts --evidence <path>", () => {
    expect(parseCommandLineArgs(["--evidence", "evidence/phase-acceptance.json"])).toEqual({ evidencePath: "evidence/phase-acceptance.json" });
  });
  it("rejects --evidence without a value", () => {
    expect(() => parseCommandLineArgs(["--evidence"])).toThrow("VALIDATE_ARGS");
  });
  it("rejects unrecognized arguments", () => {
    expect(() => parseCommandLineArgs(["--bogus"])).toThrow("VALIDATE_ARGS");
  });
});

describe("whole evidence bundle: false sign-off is rejected", () => {
  async function fullyValidBundle() {
    const { contract } = await derivePlanCommandContract(PHASE_DIR);
    const rows = [...contract.values()].map((task) => ({
      taskId: task.taskId,
      plan: task.plan,
      wave: task.wave,
      command: task.command,
      commandSha256: task.commandSha256,
      exitCode: 0,
      startedAt: "2026-08-03T10:00:00.000Z",
      completedAt: "2026-08-03T10:00:05.000Z",
      repositoryCommitSha: "a".repeat(40),
      dirty: false,
      environment: { os: "win32", runtime: "node-22" },
      build: { identity: "golc-desktop.exe@abc123" },
      artifacts: [],
    }));
    const proven = "1".repeat(40);
    const gitRunner = fakeGitRunner({ [proven]: [{ mode: "100644", objectId: "aaa", path: "internal/command/foo.go" }] }, {});
    const manifest = computeImplementationManifest(proven, gitRunner);
    return {
      bundle: {
        rows,
        calibration: await loadEvidence("screenshot-calibration.json"),
        masks: [] as unknown[],
        packagedWebView2: await loadEvidence("dialog-feasibility.json"),
        windowsCi: validWindowsCiEvidence(proven),
        implementationTree: { provenSha: proven, observedSha: proven, declaredProvenHash: manifest.hash, declaredObservedHash: manifest.hash },
        backstops: {
          startupTheme: await loadEvidence("startup-theme-font.json"),
          errorBoundary: await loadEvidence("error-boundary-fallback.json"),
          specializedGeometry: await loadEvidence("specialized-geometry.json"),
          expandedCopy: await loadEvidence("expanded-copy.json"),
          textZoom: await loadEvidence("text-zoom-200.json"),
          offlineSafety: await loadEvidence("offline-safety.json"),
        },
        requirementsCovered: REQUIREMENT_IDS,
        backstopsCovered: BACKSTOP_IDS,
        signOff: { wave_0_complete: true, nyquist_compliant: true, approved: true },
      },
      contract,
      gitRunner,
    };
  }

  it("accepts a fully closed, semantically valid bundle and only then allows sign-off", async () => {
    const { bundle, contract, gitRunner } = await fullyValidBundle();
    const result = await validateEvidenceBundle(bundle, { contract, gitRunner, approvedSha: bundle.implementationTree.provenSha });
    expect(result.errors).toEqual([]);
    expect(result.ok).toBe(true);
  });

  it("rejects sign-off flags claimed true while evidence errors remain (premature approval)", async () => {
    const { bundle, contract, gitRunner } = await fullyValidBundle();
    bundle.rows[0].exitCode = 1; // introduce a genuine failure
    const result = await validateEvidenceBundle(bundle, { contract, gitRunner });
    expect(result.ok).toBe(false);
    expect(result.errors.some((e) => e.includes("premature wave_0_complete/nyquist_compliant/approval"))).toBe(true);
  });

  it("rejects a bundle missing rows for one derived task (missing task row)", async () => {
    const { bundle, contract, gitRunner } = await fullyValidBundle();
    bundle.rows = bundle.rows.filter((row) => row.taskId !== "13-01-01");
    const result = await validateEvidenceBundle(bundle, { contract, gitRunner });
    expect(result.errors.some((e) => e.includes("missing rows for tasks") && e.includes("13-01-01"))).toBe(true);
  });

  it("rejects a duplicate row for the same task id", async () => {
    const { bundle, contract, gitRunner } = await fullyValidBundle();
    bundle.rows.push({ ...bundle.rows[0] });
    const result = await validateEvidenceBundle(bundle, { contract, gitRunner });
    expect(result.errors.some((e) => e.includes("duplicate row"))).toBe(true);
  });

  it("rejects a bundle with markdown-only/existence-only evidence (missing calibration/masks/webview2/windowsCi/implementationTree entirely)", async () => {
    const { bundle, contract, gitRunner } = await fullyValidBundle();
    delete (bundle as any).calibration;
    delete (bundle as any).masks;
    delete (bundle as any).packagedWebView2;
    delete (bundle as any).windowsCi;
    delete (bundle as any).implementationTree;
    const result = await validateEvidenceBundle(bundle, { contract, gitRunner });
    expect(result.errors.some((e) => e.includes("missing calibration evidence"))).toBe(true);
    expect(result.errors.some((e) => e.includes("missing mask audit"))).toBe(true);
    expect(result.errors.some((e) => e.includes("missing packaged WebView2 evidence"))).toBe(true);
    expect(result.errors.some((e) => e.includes("missing Windows CI evidence"))).toBe(true);
    expect(result.errors.some((e) => e.includes("missing CI implementation-tree identity"))).toBe(true);
  });

  it("rejects a bundle where the backstop coverage list omits one of the six named backstops", async () => {
    const { bundle, contract, gitRunner } = await fullyValidBundle();
    bundle.backstopsCovered = BACKSTOP_IDS.filter((id) => id !== "provider-daemon-offline-safety");
    const result = await validateEvidenceBundle(bundle, { contract, gitRunner });
    expect(result.errors.some((e) => e.includes("provider-daemon-offline-safety"))).toBe(true);
  });

  it("rejects any single semantic evidence category regression (offline safety false-stop) even if every row/coverage entry is otherwise correct", async () => {
    const { bundle, contract, gitRunner } = await fullyValidBundle();
    bundle.backstops.offlineSafety.states[0].after.outputText = "stopped";
    const result = await validateEvidenceBundle(bundle, { contract, gitRunner });
    expect(result.ok).toBe(false);
    expect(result.errors.some((e) => e.includes("provider-daemon-offline-safety"))).toBe(true);
  });
});
