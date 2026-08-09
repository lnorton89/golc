import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import Combobox, { type ComboboxOption } from "./Combobox";

afterEach(() => cleanup());

const FIXTURES: ComboboxOption[] = [
  { value: "par-can", label: "Par Can" },
  { value: "moving-head", label: "Moving Head" },
  { value: "led-bar", label: "LED Bar" },
  { value: "strobe", label: "Strobe", disabled: true },
];

describe("Combobox", () => {
  it("renders with an accessible name from its label", () => {
    render(<Combobox label="Fixture type" options={FIXTURES} />);
    expect(screen.getByRole("combobox", { name: "Fixture type" })).toBeInTheDocument();
  });

  it("uses the label as the accessible name even when the visible label is hidden", () => {
    render(<Combobox label="Fixture type" options={FIXTURES} hideLabel />);
    expect(screen.getByRole("combobox", { name: "Fixture type" })).toBeInTheDocument();
    expect(screen.queryByText("Fixture type")).not.toBeInTheDocument();
  });

  it("filters the option list as the user types", async () => {
    const user = userEvent.setup();
    render(<Combobox label="Fixture type" options={FIXTURES} />);

    const input = screen.getByRole("combobox", { name: "Fixture type" });
    await user.click(input);
    await user.type(input, "mov");

    await waitFor(() => expect(screen.getByRole("option", { name: "Moving Head" })).toBeInTheDocument());
    expect(screen.queryByRole("option", { name: "Par Can" })).not.toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "LED Bar" })).not.toBeInTheDocument();
  });

  it("filtering is a case-insensitive substring match against the label", async () => {
    const user = userEvent.setup();
    render(<Combobox label="Fixture type" options={FIXTURES} />);

    const input = screen.getByRole("combobox", { name: "Fixture type" });
    await user.click(input);
    await user.type(input, "LED");

    await waitFor(() => expect(screen.getByRole("option", { name: "LED Bar" })).toBeInTheDocument());
  });

  it("opens on ArrowDown and selects the highlighted option on Enter", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(<Combobox label="Fixture type" options={FIXTURES} onValueChange={onValueChange} />);

    const input = screen.getByRole("combobox", { name: "Fixture type" });
    await user.click(input);
    await user.keyboard("{ArrowDown}");

    // The opening ArrowDown already highlights the first option -- a
    // second ArrowDown moves the highlight on to the next one, which is
    // what Enter then selects.
    await waitFor(() => expect(screen.getByRole("listbox")).toBeInTheDocument());

    await user.keyboard("{ArrowDown}");
    await user.keyboard("{Enter}");

    await waitFor(() => expect(onValueChange).toHaveBeenCalledWith("moving-head"));
  });

  it("is uncontrolled via defaultValue and updates its own displayed selection", async () => {
    const user = userEvent.setup();
    render(<Combobox label="Fixture type" options={FIXTURES} defaultValue="par-can" />);

    const input = screen.getByRole("combobox", { name: "Fixture type" }) as HTMLInputElement;
    expect(input.value).toBe("Par Can");

    await user.click(input);
    await waitFor(() => expect(screen.getByRole("listbox")).toBeInTheDocument());
    await user.click(screen.getByRole("option", { name: "LED Bar" }));

    await waitFor(() => expect(input.value).toBe("LED Bar"));
  });

  it("is controlled via value and only reflects a selection once the caller updates value", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    const { rerender } = render(
      <Combobox label="Fixture type" options={FIXTURES} value="par-can" onValueChange={onValueChange} />,
    );

    const input = screen.getByRole("combobox", { name: "Fixture type" }) as HTMLInputElement;
    expect(input.value).toBe("Par Can");

    await user.click(input);
    await waitFor(() => expect(screen.getByRole("listbox")).toBeInTheDocument());
    await user.click(screen.getByRole("option", { name: "LED Bar" }));

    await waitFor(() => expect(onValueChange).toHaveBeenCalledWith("led-bar"));
    // Controlled: the displayed value does not change until the caller
    // feeds the new value back in as a prop.
    expect(input.value).toBe("Par Can");

    rerender(<Combobox label="Fixture type" options={FIXTURES} value="led-bar" onValueChange={onValueChange} />);
    await waitFor(() => expect(input.value).toBe("LED Bar"));
  });

  it("skips a disabled option -- it cannot be clicked into selection", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(<Combobox label="Fixture type" options={FIXTURES} onValueChange={onValueChange} />);

    const input = screen.getByRole("combobox", { name: "Fixture type" });
    await user.click(input);
    await waitFor(() => expect(screen.getByRole("listbox")).toBeInTheDocument());

    const disabledOption = screen.getByRole("option", { name: "Strobe" });
    expect(disabledOption).toHaveAttribute("aria-disabled", "true");

    await user.click(disabledOption);
    expect(onValueChange).not.toHaveBeenCalled();
  });

  it("shows the empty message when the typed filter matches nothing", async () => {
    const user = userEvent.setup();
    render(<Combobox label="Fixture type" options={FIXTURES} emptyMessage="No fixtures found" />);

    const input = screen.getByRole("combobox", { name: "Fixture type" });
    await user.click(input);
    await user.type(input, "zzzz");

    await waitFor(() => expect(screen.getByText("No fixtures found")).toBeInTheDocument());
  });
});
