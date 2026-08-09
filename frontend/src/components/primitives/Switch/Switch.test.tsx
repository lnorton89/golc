import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import Switch from "./Switch";

afterEach(() => cleanup());

describe("Switch", () => {
  it("renders with an accessible name and the switch role", () => {
    render(<Switch label="MIDI learn" defaultChecked={false} />);

    const toggle = screen.getByRole("switch", { name: "MIDI learn" });
    expect(toggle).toHaveAttribute("aria-checked", "false");
  });

  it("shows the visible label by default and swaps to an aria-label when hideLabel is set", () => {
    const { rerender } = render(<Switch label="Blackout confirm" defaultChecked={false} />);
    expect(screen.getByText("Blackout confirm")).toBeInTheDocument();

    rerender(<Switch label="Blackout confirm" defaultChecked={false} hideLabel />);
    expect(screen.queryByText("Blackout confirm")).not.toBeInTheDocument();
    expect(screen.getByRole("switch", { name: "Blackout confirm" })).toBeInTheDocument();
  });

  it("is uncontrolled when only defaultChecked is supplied: it manages its own state", async () => {
    const user = userEvent.setup();
    const onCheckedChange = vi.fn();
    render(<Switch label="Live mode" defaultChecked={false} onCheckedChange={onCheckedChange} />);

    const toggle = screen.getByRole("switch", { name: "Live mode" });
    expect(toggle).toHaveAttribute("aria-checked", "false");

    await user.click(toggle);
    await waitFor(() => expect(toggle).toHaveAttribute("aria-checked", "true"));
    expect(onCheckedChange).toHaveBeenLastCalledWith(true);
  });

  it("is controlled when checked is supplied: the caller's state is the source of truth", async () => {
    const user = userEvent.setup();

    function Controlled() {
      const [checked, setChecked] = useState(false);
      return (
        <>
          <Switch label="Live mode" checked={checked} onCheckedChange={setChecked} />
          <button onClick={() => setChecked(true)}>Force on</button>
        </>
      );
    }

    render(<Controlled />);
    const toggle = screen.getByRole("switch", { name: "Live mode" });
    expect(toggle).toHaveAttribute("aria-checked", "false");

    await user.click(screen.getByRole("button", { name: "Force on" }));
    await waitFor(() => expect(toggle).toHaveAttribute("aria-checked", "true"));

    await user.click(toggle);
    await waitFor(() => expect(toggle).toHaveAttribute("aria-checked", "false"));
  });

  it("toggles via the keyboard (Space) when focused", async () => {
    const user = userEvent.setup();
    const onCheckedChange = vi.fn();
    render(<Switch label="Live mode" defaultChecked={false} onCheckedChange={onCheckedChange} />);

    const toggle = screen.getByRole("switch", { name: "Live mode" });
    toggle.focus();
    expect(toggle).toHaveFocus();

    await user.keyboard(" ");
    await waitFor(() => expect(toggle).toHaveAttribute("aria-checked", "true"));
    expect(onCheckedChange).toHaveBeenLastCalledWith(true);

    await user.keyboard(" ");
    await waitFor(() => expect(toggle).toHaveAttribute("aria-checked", "false"));
    expect(onCheckedChange).toHaveBeenLastCalledWith(false);
  });

  it("does not respond to click or keyboard input when disabled", async () => {
    const user = userEvent.setup();
    const onCheckedChange = vi.fn();
    render(<Switch label="Live mode" defaultChecked={false} disabled onCheckedChange={onCheckedChange} />);

    const toggle = screen.getByRole("switch", { name: "Live mode" });
    // Disabled here is a <span>, not a native form control, so Base UI
    // marks it aria-disabled + tabindex="-1" rather than setting the
    // native `disabled` attribute -- confirmed against the rendered DOM,
    // same as Checkbox's own disabled state.
    expect(toggle).toHaveAttribute("aria-disabled", "true");

    await user.click(toggle);
    expect(onCheckedChange).not.toHaveBeenCalled();
    expect(toggle).toHaveAttribute("aria-checked", "false");
  });
});
