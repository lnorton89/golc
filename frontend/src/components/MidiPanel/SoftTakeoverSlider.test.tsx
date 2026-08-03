// SoftTakeoverSlider.test.tsx is the focused regression suite for Plan
// 13-28's design-system migration: the pickup-vs-armed visual state, the
// ghost/target marker's presence rule, and the MidiPickup status
// projection's value/target/pickedUp fields all stay intact through the
// conversion.
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import SoftTakeoverSlider from "./SoftTakeoverSlider";
import type { MidiFeedback } from "../../lib/wailsBridge";

function feedback(overrides: Partial<MidiFeedback> = {}): MidiFeedback {
  return {
    scope: "surface",
    surfaceName: "Booth",
    mappingId: "map-1",
    kind: "control_change",
    armed: false,
    appValue: 0,
    physical: 0,
    ...overrides,
  };
}

describe("SoftTakeoverSlider", () => {
  afterEach(() => cleanup());

  it("renders the not-armed pickup status with a ghost marker when no feedback has arrived yet", () => {
    render(<SoftTakeoverSlider />);
    expect(screen.getByText("MIDI waiting: control 0, target 0.")).toBeInTheDocument();
  });

  it("reflects the live physical position and app-value ghost target while not armed", () => {
    render(<SoftTakeoverSlider feedback={feedback({ physical: 0.5, appValue: 0.75 })} />);
    expect(screen.getByText("MIDI waiting: control 50, target 75.")).toBeInTheDocument();
  });

  it("switches to the armed status once feedback.armed is true", () => {
    render(<SoftTakeoverSlider feedback={feedback({ armed: true, physical: 0.75, appValue: 0.75 })} />);
    expect(screen.getByText("MIDI picked up: control 75, target 75.")).toBeInTheDocument();
  });

  it("clamps out-of-range and NaN feedback values", () => {
    render(<SoftTakeoverSlider feedback={feedback({ physical: 1.4, appValue: Number.NaN })} />);
    expect(screen.getByText("MIDI waiting: control 100, target 0.")).toBeInTheDocument();
  });
});
