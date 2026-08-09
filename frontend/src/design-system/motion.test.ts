import type { Easing } from "motion/react";
import { describe, expect, it } from "vitest";

import { motionTransition } from "./motion";
import { motionTokens } from "./tokens.generated";

describe("motionTransition", () => {
  it("converts a token's ms duration to seconds", () => {
    expect(motionTransition("settle").duration).toBeCloseTo(motionTokens.settle.ms / 1000);
    expect(motionTransition("tap").duration).toBeCloseTo(motionTokens.tap.ms / 1000);
  });

  it("translates every generated token's CSS easing keyword into a Motion-valid ease name", () => {
    const validMotionEases = new Set<Easing>(["linear", "easeIn", "easeOut", "easeInOut"]);
    for (const tokenName of Object.keys(motionTokens) as (keyof typeof motionTokens)[]) {
      expect(validMotionEases.has(motionTransition(tokenName).ease)).toBe(true);
    }
  });
});
