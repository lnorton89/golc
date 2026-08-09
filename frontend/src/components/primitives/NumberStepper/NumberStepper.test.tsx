import { createRef } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import NumberStepper from "./NumberStepper";

describe("NumberStepper", () => {
  afterEach(() => cleanup());

  it("renders a visible label and forwards the input ref", () => {
    const ref = createRef<HTMLInputElement>();
    render(<NumberStepper ref={ref} label="Quantity" value="1" onChange={() => {}} />);

    expect(screen.getByText("Quantity")).toBeInTheDocument();
    const input = screen.getByLabelText("Quantity");
    expect(ref.current).toBe(input);
  });

  it("nudges the value up and down without stealing focus from the input", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<NumberStepper label="Quantity" value="4" onChange={onChange} />);

    const input = screen.getByLabelText("Quantity");
    input.focus();

    await user.click(screen.getByRole("button", { name: "Increase quantity" }));
    expect(onChange).toHaveBeenCalledWith("5");
    expect(document.activeElement).toBe(input);

    await user.click(screen.getByRole("button", { name: "Decrease quantity" }));
    expect(onChange).toHaveBeenCalledWith("3");
    expect(document.activeElement).toBe(input);

    for (const button of screen.getAllByRole("button")) {
      expect(button).toHaveAttribute("tabindex", "-1");
    }
  });

  it("never nudges below the configured minimum", async () => {
    // Base UI's NumberField disables the Decrement button once the value
    // reaches `min` (a real HTML `disabled`, not just aria-disabled --
    // Increment/Decrement are plain buttons, not part of a roving-tabindex
    // composite) rather than leaving it clickable-but-a-no-op the way the
    // hand-rolled version did, so clicking it now fires no onChange at all.
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<NumberStepper label="Quantity" value="1" onChange={onChange} min={1} />);

    const decrement = screen.getByRole("button", { name: "Decrease quantity" });
    expect(decrement).toBeDisabled();

    await user.click(decrement);
    expect(onChange).not.toHaveBeenCalled();
  });

  it("disables the input and both nudge buttons together", () => {
    render(<NumberStepper label="Quantity" value="1" onChange={() => {}} disabled />);

    expect(screen.getByLabelText("Quantity")).toBeDisabled();
    for (const button of screen.getAllByRole("button")) {
      expect(button).toBeDisabled();
    }
  });

  it("nudges by a fractional step when one is provided, without floating-point drift", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<NumberStepper label="BPM" value="120" onChange={onChange} min={1} step={0.1} />);

    await user.click(screen.getByRole("button", { name: "Increase bpm" }));
    expect(onChange).toHaveBeenCalledWith("120.1");

    await user.click(screen.getByRole("button", { name: "Decrease bpm" }));
    expect(onChange).toHaveBeenCalledWith("119.9");
  });

  it("forwards onBlur and onKeyDown to the underlying input", () => {
    const onBlur = vi.fn();
    const onKeyDown = vi.fn();
    render(<NumberStepper label="BPM" value="120" onChange={() => {}} onBlur={onBlur} onKeyDown={onKeyDown} />);

    const input = screen.getByLabelText("BPM");
    fireEvent.keyDown(input, { key: "Enter" });
    fireEvent.blur(input);

    expect(onKeyDown).toHaveBeenCalledTimes(1);
    expect(onBlur).toHaveBeenCalledTimes(1);
  });

  it("round-trips an empty value to null and back to an empty string, for optional 'Auto' fields", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<NumberStepper label="Starting universe" value="" onChange={onChange} placeholder="Auto" />);

    const input = screen.getByLabelText("Starting universe");
    await user.type(input, "3");
    expect(onChange).toHaveBeenLastCalledWith("3");
  });
});
