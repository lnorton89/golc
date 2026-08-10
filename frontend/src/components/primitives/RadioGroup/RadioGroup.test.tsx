import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import RadioGroup from "./RadioGroup";

afterEach(() => cleanup());

const OPTIONS = [
  { value: "immediate", label: "Immediate" },
  { value: "fade", label: "Fade to black" },
  { value: "hold", label: "Hold last state", disabled: true },
];

describe("RadioGroup", () => {
  it("renders a labeled group with each option exposed as a radio", () => {
    render(<RadioGroup label="Blackout behavior" defaultValue="immediate" options={OPTIONS} />);

    const group = screen.getByRole("radiogroup", { name: "Blackout behavior" });
    expect(group).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "Immediate" })).toHaveAttribute("aria-checked", "true");
    expect(screen.getByRole("radio", { name: "Fade to black" })).toHaveAttribute("aria-checked", "false");
  });

  it("uses aria-label to override the accessible name while still rendering the visible label text", () => {
    render(<RadioGroup label="Blackout behavior" aria-label="Custom name" defaultValue="immediate" options={OPTIONS} />);

    expect(screen.getByRole("radiogroup", { name: "Custom name" })).toBeInTheDocument();
    expect(screen.getByText("Blackout behavior")).toBeInTheDocument();
  });

  it("is uncontrolled when only defaultValue is supplied: it manages its own state", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(<RadioGroup label="Blackout behavior" defaultValue="immediate" options={OPTIONS} onValueChange={onValueChange} />);

    const fade = screen.getByRole("radio", { name: "Fade to black" });
    await user.click(fade);

    await waitFor(() => expect(fade).toHaveAttribute("aria-checked", "true"));
    expect(onValueChange).toHaveBeenLastCalledWith("fade");
  });

  it("is controlled when value is supplied: the caller's state is the source of truth", async () => {
    const user = userEvent.setup();

    function Controlled() {
      const [value, setValue] = useState("immediate");
      return (
        <>
          <RadioGroup label="Blackout behavior" value={value} onValueChange={setValue} options={OPTIONS} />
          <button onClick={() => setValue("fade")}>Force fade</button>
        </>
      );
    }

    render(<Controlled />);
    expect(screen.getByRole("radio", { name: "Immediate" })).toHaveAttribute("aria-checked", "true");

    await user.click(screen.getByRole("button", { name: "Force fade" }));
    await waitFor(() => expect(screen.getByRole("radio", { name: "Fade to black" })).toHaveAttribute("aria-checked", "true"));
    expect(screen.getByRole("radio", { name: "Immediate" })).toHaveAttribute("aria-checked", "false");
  });

  it("moves selection between options with the arrow keys", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(<RadioGroup label="Blackout behavior" defaultValue="immediate" options={OPTIONS} onValueChange={onValueChange} />);

    const immediate = screen.getByRole("radio", { name: "Immediate" });
    const fade = screen.getByRole("radio", { name: "Fade to black" });
    immediate.focus();

    await user.keyboard("{ArrowDown}");
    await waitFor(() => expect(fade).toHaveAttribute("aria-checked", "true"));
    expect(fade).toHaveFocus();
    expect(immediate).toHaveAttribute("aria-checked", "false");
    expect(onValueChange).toHaveBeenLastCalledWith("fade");
  });

  it("skips a disabled option and does not respond to a click on it", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(<RadioGroup label="Blackout behavior" defaultValue="immediate" options={OPTIONS} onValueChange={onValueChange} />);

    const hold = screen.getByRole("radio", { name: "Hold last state" });
    expect(hold).toHaveAttribute("aria-disabled", "true");

    await user.click(hold);
    expect(onValueChange).not.toHaveBeenCalled();
    expect(hold).toHaveAttribute("aria-checked", "false");
  });

  it("does not respond to any interaction when the whole group is disabled", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(<RadioGroup label="Blackout behavior" defaultValue="immediate" options={OPTIONS} disabled onValueChange={onValueChange} />);

    const group = screen.getByRole("radiogroup", { name: "Blackout behavior" });
    expect(group).toHaveAttribute("aria-disabled", "true");

    const fade = screen.getByRole("radio", { name: "Fade to black" });
    await user.click(fade);
    expect(onValueChange).not.toHaveBeenCalled();
    expect(fade).toHaveAttribute("aria-checked", "false");
  });

  describe('layout="wrap"', () => {
    const SWATCH_OPTIONS = [
      { value: "default", label: "Default", swatch: "#1b44d9" },
      { value: "gruvbox", label: "Gruvbox", swatch: "#fe8019" },
    ];

    it("still exposes each option as a radio, and renders its swatch as a decorative element", () => {
      render(<RadioGroup label="Theme" layout="wrap" defaultValue="default" options={SWATCH_OPTIONS} />);

      const group = screen.getByRole("radiogroup", { name: "Theme" });
      expect(group).toHaveAttribute("data-layout", "wrap");
      const gruvbox = screen.getByRole("radio", { name: "Gruvbox" });
      expect(gruvbox).toHaveAttribute("aria-checked", "false");
      expect(screen.getByRole("radio", { name: "Default" })).toHaveAttribute("aria-checked", "true");
    });

    it("selects an option on click, same as the stacked layout", async () => {
      const user = userEvent.setup();
      const onValueChange = vi.fn();
      render(
        <RadioGroup label="Theme" layout="wrap" defaultValue="default" options={SWATCH_OPTIONS} onValueChange={onValueChange} />,
      );

      await user.click(screen.getByRole("radio", { name: "Gruvbox" }));
      await waitFor(() => expect(screen.getByRole("radio", { name: "Gruvbox" })).toHaveAttribute("aria-checked", "true"));
      expect(onValueChange).toHaveBeenLastCalledWith("gruvbox");
    });
  });
});
