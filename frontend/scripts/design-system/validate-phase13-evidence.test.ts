import { describe, expect, it } from "vitest";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import {
  EVIDENCE_DIR,
  PHASE_DIR,
  collectAssertionErrors,
  decodeXmlEntities,
  derivePlanCommandContract,
  normalizeCommand,
  parsePlanTasks,
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
