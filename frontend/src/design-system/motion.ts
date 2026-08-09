// Motion (the JS animation library) has its own easing vocabulary --
// CSS's "ease"/"ease-out"/... keywords aren't valid Motion `ease` values
// (only "linear"/"easeIn"/"easeOut"/"easeInOut"/... names, a cubic-bezier
// array, or a spring config are). tokens.generated.ts's motionTokens are
// generated straight from each --ds-motion-* CSS custom property's own
// transition-timing-function, so every consumer reaching for a real
// motion.* transition (as opposed to a CSS transition, which takes the
// token's raw shorthand value directly) needs this same translation --
// centralized here once instead of duplicated per component.
import type { Easing } from "motion/react";

import { motionTokens, type MotionTokenName } from "./tokens.generated";

const CSS_EASING_TO_MOTION_EASE: Record<string, Easing> = {
  ease: "easeInOut",
  linear: "linear",
  "ease-in": "easeIn",
  "ease-out": "easeOut",
  "ease-in-out": "easeInOut",
};

/** motionTransition converts a design-system motion token into a Motion
 * `transition` object ({ duration, ease }), so a motion.* component's
 * animation timing stays derived from the same --ds-motion-* source of
 * truth as this app's CSS transitions, rather than a hand-picked duration. */
export function motionTransition(tokenName: MotionTokenName): { duration: number; ease: Easing } {
  const token = motionTokens[tokenName];
  return {
    duration: token.ms / 1000,
    ease: CSS_EASING_TO_MOTION_EASE[token.easing] ?? "easeInOut",
  };
}
