import { createRef } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

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

  it("nudges the value up and down without stealing focus from the input", () => {
    const onChange = vi.fn();
    render(<NumberStepper label="Quantity" value="4" onChange={onChange} />);

    fireEvent.click(screen.getByRole("button", { name: "Increase quantity" }));
    expect(onChange).toHaveBeenCalledWith("5");

    fireEvent.click(screen.getByRole("button", { name: "Decrease quantity" }));
    expect(onChange).toHaveBeenCalledWith("3");

    for (const button of screen.getAllByRole("button")) {
      expect(button).toHaveAttribute("tabindex", "-1");
    }
  });

  it("never nudges below the configured minimum", () => {
    const onChange = vi.fn();
    render(<NumberStepper label="Quantity" value="1" onChange={onChange} min={1} />);

    fireEvent.click(screen.getByRole("button", { name: "Decrease quantity" }));
    expect(onChange).toHaveBeenCalledWith("1");
  });

  it("disables the input and both nudge buttons together", () => {
    render(<NumberStepper label="Quantity" value="1" onChange={() => {}} disabled />);

    expect(screen.getByLabelText("Quantity")).toBeDisabled();
    for (const button of screen.getAllByRole("button")) {
      expect(button).toBeDisabled();
    }
  });

  it("nudges by a fractional step when one is provided, without floating-point drift", () => {
    const onChange = vi.fn();
    render(<NumberStepper label="BPM" value="120" onChange={onChange} min={1} step={0.1} />);

    fireEvent.click(screen.getByRole("button", { name: "Increase bpm" }));
    expect(onChange).toHaveBeenCalledWith("120.1");

    fireEvent.click(screen.getByRole("button", { name: "Decrease bpm" }));
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
});
