import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import Slider from "./Slider";

afterEach(() => cleanup());

describe("Slider", () => {
  it("renders with an accessible name and the native ARIA slider attributes", () => {
    render(<Slider label="Intensity" defaultValue={40} min={0} max={100} step={1} />);

    const slider = screen.getByRole("slider", { name: "Intensity" });
    // Base UI's Thumb renders the min/max bounds as the native input's own
    // min/max HTML attributes (which the browser's accessibility tree maps
    // to aria-valuemin/aria-valuemax implicitly for a real <input
    // type="range">) rather than setting aria-valuemin/aria-valuemax as
    // explicit attributes -- confirmed empirically, not assumed from docs.
    // aria-valuenow IS set explicitly, so that one is asserted directly.
    expect(slider).toHaveAttribute("min", "0");
    expect(slider).toHaveAttribute("max", "100");
    expect(slider).toHaveAttribute("aria-valuenow", "40");
  });

  it("shows the visible label by default and swaps to an aria-label when hideLabel is set", () => {
    const { rerender } = render(<Slider label="Pan" defaultValue={0} />);
    expect(screen.getByText("Pan")).toBeInTheDocument();

    rerender(<Slider label="Pan" defaultValue={0} hideLabel />);
    expect(screen.queryByText("Pan")).not.toBeInTheDocument();
    expect(screen.getByRole("slider", { name: "Pan" })).toBeInTheDocument();
  });

  it("nudges the value by step on ArrowRight/ArrowLeft and reports the change", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(<Slider label="Intensity" defaultValue={50} step={5} onValueChange={onValueChange} />);

    const slider = screen.getByRole("slider", { name: "Intensity" });
    slider.focus();

    await user.keyboard("{ArrowRight}");
    await waitFor(() => expect(slider).toHaveAttribute("aria-valuenow", "55"));
    expect(onValueChange).toHaveBeenLastCalledWith(55);

    await user.keyboard("{ArrowLeft}");
    await waitFor(() => expect(slider).toHaveAttribute("aria-valuenow", "50"));
    expect(onValueChange).toHaveBeenLastCalledWith(50);
  });

  it("jumps to min/max on Home/End", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(<Slider label="Intensity" defaultValue={50} min={0} max={100} onValueChange={onValueChange} />);

    const slider = screen.getByRole("slider", { name: "Intensity" });
    slider.focus();

    await user.keyboard("{End}");
    await waitFor(() => expect(slider).toHaveAttribute("aria-valuenow", "100"));
    expect(onValueChange).toHaveBeenLastCalledWith(100);

    await user.keyboard("{Home}");
    await waitFor(() => expect(slider).toHaveAttribute("aria-valuenow", "0"));
    expect(onValueChange).toHaveBeenLastCalledWith(0);
  });

  it("clamps keyboard nudges at the min/max bounds instead of overshooting", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(<Slider label="Intensity" value={100} min={0} max={100} step={10} onValueChange={onValueChange} />);

    const slider = screen.getByRole("slider", { name: "Intensity" });
    slider.focus();

    await user.keyboard("{ArrowRight}");
    // Already at max -- a value-committing nudge past the bound must not
    // report anything beyond max, whether that means no call at all or a
    // call still pinned to max.
    if (onValueChange.mock.calls.length > 0) {
      expect(onValueChange).toHaveBeenLastCalledWith(100);
    }
    expect(slider).toHaveAttribute("aria-valuenow", "100");
  });

  it("is controlled when value is supplied: the caller's state is the source of truth", async () => {
    const user = userEvent.setup();

    function Controlled() {
      const [value, setValue] = useState(20);
      return (
        <>
          <Slider label="Intensity" value={value} onValueChange={setValue} min={0} max={100} step={10} />
          <button onClick={() => setValue(90)}>Jump to 90</button>
        </>
      );
    }

    render(<Controlled />);
    const slider = screen.getByRole("slider", { name: "Intensity" });
    expect(slider).toHaveAttribute("aria-valuenow", "20");

    await user.click(screen.getByRole("button", { name: "Jump to 90" }));
    await waitFor(() => expect(slider).toHaveAttribute("aria-valuenow", "90"));

    slider.focus();
    await user.keyboard("{ArrowRight}");
    await waitFor(() => expect(slider).toHaveAttribute("aria-valuenow", "100"));
  });

  it("is uncontrolled when only defaultValue is supplied: it manages its own state", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(<Slider label="Intensity" defaultValue={20} step={10} onValueChange={onValueChange} />);

    const slider = screen.getByRole("slider", { name: "Intensity" });
    slider.focus();
    await user.keyboard("{ArrowRight}");

    await waitFor(() => expect(slider).toHaveAttribute("aria-valuenow", "30"));
    expect(onValueChange).toHaveBeenLastCalledWith(30);
  });

  it("does not respond to keyboard or pointer input when disabled", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(<Slider label="Intensity" defaultValue={50} disabled onValueChange={onValueChange} />);

    const slider = screen.getByRole("slider", { name: "Intensity" });
    expect(slider).toBeDisabled();

    slider.focus();
    expect(slider).not.toHaveFocus();

    await user.keyboard("{ArrowRight}");
    expect(onValueChange).not.toHaveBeenCalled();
    expect(slider).toHaveAttribute("aria-valuenow", "50");
  });
});
