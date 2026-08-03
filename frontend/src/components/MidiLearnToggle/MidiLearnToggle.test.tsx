// MidiLearnToggle.test.tsx is the focused regression suite for Plan
// 13-28's design-system migration: soft-disabled state on an unsupported
// destination (native disabled attribute must never be set, so its own
// title tooltip keeps working), toggling midiLearnMode on and off on a
// supported destination, and auto-exiting learn mode on navigating away
// from a supported destination all stay intact through the conversion.
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import MidiLearnToggle from "./MidiLearnToggle";
import { useGolcStore } from "../../store/store";

describe("MidiLearnToggle", () => {
  afterEach(() => {
    cleanup();
    useGolcStore.setState({ midiLearnMode: false });
  });

  it("renders soft-disabled (aria-disabled, not the native attribute) on an unsupported destination", () => {
    render(<MidiLearnToggle activeDestination="show-overview" />);
    const toggle = screen.getByRole("button", { name: "Enter MIDI Learn mode" });

    expect(toggle).toHaveAttribute("aria-disabled", "true");
    expect(toggle).not.toBeDisabled();
    expect(toggle).toHaveAttribute("title", expect.stringContaining("switch to Perform > Desk"));
  });

  it("does not toggle midiLearnMode when clicked on an unsupported destination", () => {
    render(<MidiLearnToggle activeDestination="show-overview" />);
    fireEvent.click(screen.getByRole("button", { name: "Enter MIDI Learn mode" }));

    expect(useGolcStore.getState().midiLearnMode).toBe(false);
  });

  it("turns midiLearnMode on and off when clicked on a supported destination", () => {
    render(<MidiLearnToggle activeDestination="perform-desk" />);
    const toggle = screen.getByRole("button", { name: "Enter MIDI Learn mode" });

    fireEvent.click(toggle);
    expect(useGolcStore.getState().midiLearnMode).toBe(true);

    const activeToggle = screen.getByRole("button", { name: "Exit MIDI Learn mode" });
    expect(activeToggle).toHaveAttribute("aria-pressed", "true");
    expect(activeToggle).toHaveAttribute("data-active", "true");

    fireEvent.click(activeToggle);
    expect(useGolcStore.getState().midiLearnMode).toBe(false);
  });

  it("turns learn mode back off automatically when the active destination stops being supported", () => {
    useGolcStore.setState({ midiLearnMode: true });
    const { rerender } = render(<MidiLearnToggle activeDestination="perform-desk" />);
    expect(useGolcStore.getState().midiLearnMode).toBe(true);

    rerender(<MidiLearnToggle activeDestination="show-overview" />);

    expect(useGolcStore.getState().midiLearnMode).toBe(false);
  });
});
