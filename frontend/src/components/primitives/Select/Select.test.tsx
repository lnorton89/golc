import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import Select, { type SelectOption } from "./Select";

afterEach(() => cleanup());

const OPTIONS: readonly SelectOption[] = [
  { value: "wash", label: "Wash" },
  { value: "spot", label: "Spot" },
  { value: "beam", label: "Beam", disabled: true },
  { value: "hybrid", label: "Hybrid" },
];

describe("Select", () => {
  it("renders with an accessible name from its visible label", () => {
    render(<Select label="Fixture type" options={OPTIONS} />);
    expect(screen.getByRole("combobox", { name: "Fixture type" })).toBeInTheDocument();
    expect(screen.getByText("Fixture type")).toBeInTheDocument();
  });

  it("hides the visible label but keeps the accessible name when hideLabel is set", () => {
    render(<Select label="Fixture type" options={OPTIONS} hideLabel />);
    expect(screen.getByRole("combobox", { name: "Fixture type" })).toBeInTheDocument();
    expect(screen.queryByText("Fixture type")).not.toBeInTheDocument();
  });

  it("opens with the keyboard and selects an option with ArrowDown/Enter", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(<Select label="Fixture type" options={OPTIONS} onValueChange={onValueChange} />);

    const trigger = screen.getByRole("combobox", { name: "Fixture type" });
    trigger.focus();
    await user.keyboard("[ArrowDown]");
    await waitFor(() => expect(screen.getByRole("option", { name: "Wash" })).toBeInTheDocument());

    await user.keyboard("[ArrowDown]");
    await user.keyboard("[Enter]");

    await waitFor(() => expect(onValueChange).toHaveBeenCalledWith("spot"));
  });

  it("supports an uncontrolled defaultValue", () => {
    render(<Select label="Fixture type" options={OPTIONS} defaultValue="hybrid" />);
    expect(screen.getByRole("combobox", { name: "Fixture type" })).toHaveTextContent("Hybrid");
  });

  it("reflects a controlled value and calls onValueChange without mutating on its own", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    const { rerender } = render(<Select label="Fixture type" options={OPTIONS} value="wash" onValueChange={onValueChange} />);
    expect(screen.getByRole("combobox", { name: "Fixture type" })).toHaveTextContent("Wash");

    const trigger = screen.getByRole("combobox", { name: "Fixture type" });
    await user.click(trigger);
    const option = await screen.findByRole("option", { name: "Spot" });
    await user.click(option);

    await waitFor(() => expect(onValueChange).toHaveBeenCalledWith("spot"));
    // Controlled: the displayed value does not change until the caller
    // feeds the new value back in as a prop.
    expect(screen.getByRole("combobox", { name: "Fixture type" })).toHaveTextContent("Wash");

    rerender(<Select label="Fixture type" options={OPTIONS} value="spot" onValueChange={onValueChange} />);
    expect(screen.getByRole("combobox", { name: "Fixture type" })).toHaveTextContent("Spot");
  });

  it("skips a disabled option -- it cannot be selected via keyboard or pointer", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(<Select label="Fixture type" options={OPTIONS} onValueChange={onValueChange} />);

    const trigger = screen.getByRole("combobox", { name: "Fixture type" });
    await user.click(trigger);
    const disabledOption = await screen.findByRole("option", { name: "Beam" });
    expect(disabledOption).toHaveAttribute("aria-disabled", "true");

    await user.click(disabledOption);
    expect(onValueChange).not.toHaveBeenCalled();
  });

  it("renders a disabled Select whose trigger cannot be opened", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(<Select label="Fixture type" options={OPTIONS} disabled onValueChange={onValueChange} />);

    const trigger = screen.getByRole("combobox", { name: "Fixture type" });
    expect(trigger).toBeDisabled();

    await user.click(trigger);
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    expect(onValueChange).not.toHaveBeenCalled();
  });
});
