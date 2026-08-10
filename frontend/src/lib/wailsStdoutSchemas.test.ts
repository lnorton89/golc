// wailsStdoutSchemas.test.ts pins the boundary behaviour the unvalidated
// `JSON.parse(result.stdout) as <Shape>` casts never had: a malformed
// payload must fail at the boundary with a named diagnostic, not travel
// onward as a plausible-looking object.
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  impactPlanSchema,
  parseStdoutJson,
  parseStdoutJsonOrUndefined,
  playbackStateSummarySchema,
} from "./wailsStdoutSchemas";

function validPlan(overrides: Record<string, unknown> = {}) {
  return {
    schema_version: 1,
    pool_id: "pool-1",
    propagate: "none",
    expected_revision: 1,
    operations: [
      {
        dependent_kind: "deployment_instance",
        dependent_ref: "Main / Fixture 1",
        dependent_id: "inst-1",
        action: "add",
        pool_member_index: 0,
        pool_member_id: "member-1",
        proposed_universe: 1,
        proposed_address: 5,
        status: "pending",
      },
    ],
    plan_id: "plan-abc123456789",
    ...overrides,
  };
}

describe("impactPlanSchema", () => {
  it("accepts a well-formed plan", () => {
    const plan = parseStdoutJson(impactPlanSchema, "AddPoolMemberPreview", JSON.stringify(validPlan()));
    expect(plan.plan_id).toBe("plan-abc123456789");
    expect(plan.operations?.[0].proposed_universe).toBe(1);
  });

  it("accepts a null operations list (Go marshals the nil slice as JSON null)", () => {
    const plan = parseStdoutJson(impactPlanSchema, "AddPoolMemberPreview", JSON.stringify(validPlan({ operations: null })));
    expect(plan.operations).toBeNull();
  });

  it("rejects a plan with no plan_id, instead of handing undefined to applyPatch", () => {
    const { plan_id: _dropped, ...withoutPlanId } = validPlan();
    expect(() =>
      parseStdoutJson(impactPlanSchema, "AddPoolMemberPreview", JSON.stringify(withoutPlanId)),
    ).toThrow(/GOLC_WAILS_STDOUT_INVALID/);
  });

  it("rejects an empty plan_id", () => {
    expect(() =>
      parseStdoutJson(impactPlanSchema, "AddPoolMemberPreview", JSON.stringify(validPlan({ plan_id: "" }))),
    ).toThrow(/GOLC_WAILS_STDOUT_INVALID/);
  });

  it("rejects a wrongly-typed operation field rather than rendering it as 'undefined'", () => {
    const plan = validPlan({
      operations: [
        {
          dependent_kind: "deployment_instance",
          dependent_ref: "Main / Fixture 1",
          dependent_id: "inst-1",
          action: "add",
          proposed_universe: "one",
          status: "pending",
        },
      ],
    });
    expect(() => parseStdoutJson(impactPlanSchema, "AddPoolMemberPreview", JSON.stringify(plan))).toThrow(
      /GOLC_WAILS_STDOUT_INVALID/,
    );
  });

  it("strips unknown keys so a new Go field never breaks a running frontend", () => {
    const plan = parseStdoutJson(
      impactPlanSchema,
      "AddPoolMemberPreview",
      JSON.stringify(validPlan({ some_future_field: "added later" })),
    );
    expect(plan).not.toHaveProperty("some_future_field");
    expect(plan.plan_id).toBe("plan-abc123456789");
  });

  it("names the route in the diagnostic and reports invalid JSON separately", () => {
    expect(() => parseStdoutJson(impactPlanSchema, "RemovePoolMemberPreview", "not json")).toThrow(
      /RemovePoolMemberPreview did not return valid JSON/,
    );
  });
});

describe("playbackStateSummarySchema", () => {
  const validState = {
    bpm: 120,
    scenes: [
      {
        name: "Alpha",
        active: true,
        barsPerLoop: 4,
        layers: [{ kind: "base_look", enabled: true, ref: "" }],
      },
    ],
  };

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("accepts a well-formed snapshot", () => {
    const state = parseStdoutJsonOrUndefined(
      playbackStateSummarySchema,
      "PlaybackService.GetState",
      JSON.stringify(validState),
    );
    expect(state?.scenes[0].layers[0].kind).toBe("base_look");
  });

  it("answers undefined (never throws) on a bad shape, keeping getState's documented contract", () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    const state = parseStdoutJsonOrUndefined(
      playbackStateSummarySchema,
      "PlaybackService.GetState",
      JSON.stringify({ bpm: "fast", scenes: [] }),
    );

    expect(state).toBeUndefined();
    // The drop is visible rather than silent.
    expect(errorSpy).toHaveBeenCalledWith(expect.stringContaining("GOLC_WAILS_STDOUT_INVALID"));
  });

  it("rejects an unknown layer kind", () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    const state = parseStdoutJsonOrUndefined(
      playbackStateSummarySchema,
      "PlaybackService.GetState",
      JSON.stringify({ bpm: 120, scenes: [{ name: "A", active: true, barsPerLoop: 4, layers: [{ kind: "strobe", enabled: true }] }] }),
    );
    expect(state).toBeUndefined();
  });
});
