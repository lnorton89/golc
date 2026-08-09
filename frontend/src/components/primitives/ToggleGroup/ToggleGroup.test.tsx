import { useState } from "react";
import { Sun } from "lucide-react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import ToggleGroup from "./ToggleGroup";

afterEach(() => cleanup());

const OPTIONS = [
  { value: "local", label: "My Library" },
  { value: "catalog", label: "Open Fixture Library" },
  { value: "disabled-option", label: "Disabled Option", disabled: true },
];

describe("ToggleGroup", () => {
  it("renders a labeled group with each option exposed as a button", () => {
    render(<ToggleGroup label="Fixture source" defaultValue="local" options={OPTIONS} />);

    const group = screen.getByRole("group", { name: "Fixture source" });
    expect(group).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "My Library" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "Open Fixture Library" })).toHaveAttribute("aria-pressed", "false");
  });

  it("uses aria-label directly when there is no visible label", () => {
    render(<ToggleGroup aria-label="Fixture source" defaultValue="local" options={OPTIONS} />);

    expect(screen.getByRole("group", { name: "Fixture source" })).toBeInTheDocument();
    expect(screen.queryByText("Fixture source")).not.toBeInTheDocument();
  });

  it("uses aria-label to override the accessible name while still rendering the visible label text", () => {
    render(<ToggleGroup label="Fixture source" aria-label="Custom name" defaultValue="local" options={OPTIONS} />);

    expect(screen.getByRole("group", { name: "Custom name" })).toBeInTheDocument();
    expect(screen.getByText("Fixture source")).toBeInTheDocument();
  });

  it("is uncontrolled when only defaultValue is supplied: it manages its own state", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(<ToggleGroup label="Fixture source" defaultValue="local" options={OPTIONS} onValueChange={onValueChange} />);

    const catalog = screen.getByRole("button", { name: "Open Fixture Library" });
    await user.click(catalog);

    await waitFor(() => expect(catalog).toHaveAttribute("aria-pressed", "true"));
    expect(onValueChange).toHaveBeenLastCalledWith("catalog");
  });

  it("is controlled when value is supplied: the caller's state is the source of truth", async () => {
    const user = userEvent.setup();

    function Controlled() {
      const [value, setValue] = useState("local");
      return (
        <>
          <ToggleGroup label="Fixture source" value={value} onValueChange={setValue} options={OPTIONS} />
          <button onClick={() => setValue("catalog")}>Force catalog</button>
        </>
      );
    }

    render(<Controlled />);
    expect(screen.getByRole("button", { name: "My Library" })).toHaveAttribute("aria-pressed", "true");

    await user.click(screen.getByRole("button", { name: "Force catalog" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Open Fixture Library" })).toHaveAttribute("aria-pressed", "true"));
    expect(screen.getByRole("button", { name: "My Library" })).toHaveAttribute("aria-pressed", "false");
  });

  it("moves selection between options with the arrow keys", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(<ToggleGroup label="Fixture source" defaultValue="local" options={OPTIONS} onValueChange={onValueChange} />);

    const local = screen.getByRole("button", { name: "My Library" });
    const catalog = screen.getByRole("button", { name: "Open Fixture Library" });
    local.focus();

    await user.keyboard("{ArrowRight}");
    await waitFor(() => expect(catalog).toHaveFocus());
    expect(onValueChange).not.toHaveBeenCalled(); // arrow movement alone doesn't press an option -- matches roving-tabindex, not the radio auto-select-on-move behavior
  });

  it("skips a disabled option and does not respond to a click on it", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(<ToggleGroup label="Fixture source" defaultValue="local" options={OPTIONS} onValueChange={onValueChange} />);

    const disabledOption = screen.getByRole("button", { name: "Disabled Option" });
    expect(disabledOption).toBeDisabled();

    await user.click(disabledOption);
    expect(onValueChange).not.toHaveBeenCalled();
    expect(disabledOption).toHaveAttribute("aria-pressed", "false");
  });

  it("does not respond to any interaction when the whole group is disabled", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(<ToggleGroup label="Fixture source" defaultValue="local" options={OPTIONS} disabled onValueChange={onValueChange} />);

    const catalog = screen.getByRole("button", { name: "Open Fixture Library" });
    await user.click(catalog);
    expect(onValueChange).not.toHaveBeenCalled();
    expect(catalog).toHaveAttribute("aria-pressed", "false");
  });

  it("renders an option's optional leading icon", () => {
    const withIcon = [
      { value: "light", label: "Light", icon: Sun },
      { value: "dark", label: "Dark" },
    ];
    render(<ToggleGroup label="Mode" defaultValue="light" options={withIcon} />);

    const lightButton = screen.getByRole("button", { name: "Light" });
    expect(lightButton.querySelector("svg")).toBeInTheDocument();
  });

  it("ignores an attempt to deselect the only pressed option, keeping exactly one selected", async () => {
    // Base UI's ToggleGroup models its value as string[] even in
    // single-select mode, so clicking the already-pressed option would
    // deselect it to [] left to Base UI's own default behavior -- this
    // primitive's contract (like RadioGroup's) promises exactly one
    // selected option at all times, so that deselect must be a no-op.
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(<ToggleGroup label="Fixture source" defaultValue="local" options={OPTIONS} onValueChange={onValueChange} />);

    const local = screen.getByRole("button", { name: "My Library" });
    expect(local).toHaveAttribute("aria-pressed", "true");

    await user.click(local);
    expect(onValueChange).not.toHaveBeenCalled();
    expect(local).toHaveAttribute("aria-pressed", "true");
  });
});
