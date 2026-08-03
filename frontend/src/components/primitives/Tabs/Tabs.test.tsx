import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import Tabs from "./Tabs";

const tabs = [
  { id: "fixtures", label: "Fixtures", panel: <p>Fixture patch</p> },
  { id: "scenes", label: "Scenes", panel: <p>Scene list</p> },
  { id: "output", label: "Output", panel: <p>Output status</p> },
] as const;

describe("Tabs", () => {
  afterEach(() => cleanup());

  it("connects a named tab list, selected tab, and its labelled panel", () => {
    render(<Tabs aria-label="Show views" tabs={tabs} />);

    const tabList = screen.getByRole("tablist", { name: "Show views" });
    const fixtures = screen.getByRole("tab", { name: "Fixtures" });
    const panel = screen.getByRole("tabpanel");

    expect(tabList).toBeInTheDocument();
    expect(fixtures).toHaveAttribute("aria-selected", "true");
    expect(fixtures).toHaveAttribute("tabindex", "0");
    expect(panel).toHaveAttribute("aria-labelledby", fixtures.id);
    expect(screen.getByText("Fixture patch")).toBeInTheDocument();
  });

  it("uses arrow keys, Home, and End to focus and select enabled tabs", () => {
    render(<Tabs aria-label="Show views" tabs={tabs} />);

    const fixtures = screen.getByRole("tab", { name: "Fixtures" });
    const scenes = screen.getByRole("tab", { name: "Scenes" });
    const output = screen.getByRole("tab", { name: "Output" });

    fixtures.focus();
    fireEvent.keyDown(fixtures, { key: "ArrowRight" });
    expect(scenes).toHaveFocus();
    expect(scenes).toHaveAttribute("aria-selected", "true");

    fireEvent.keyDown(scenes, { key: "End" });
    expect(output).toHaveFocus();
    expect(output).toHaveAttribute("aria-selected", "true");

    fireEvent.keyDown(output, { key: "Home" });
    expect(fixtures).toHaveFocus();
    expect(fixtures).toHaveAttribute("aria-selected", "true");
  });

  it("wraps arrow navigation and skips disabled tabs", () => {
    render(
      <Tabs
        aria-label="Show views"
        tabs={[
          tabs[0],
          { id: "disabled", label: "Disabled", panel: <p>Unavailable</p>, disabled: true },
          tabs[2],
        ]}
      />,
    );

    const fixtures = screen.getByRole("tab", { name: "Fixtures" });
    const output = screen.getByRole("tab", { name: "Output" });
    const disabled = screen.getByRole("tab", { name: "Disabled" });

    fixtures.focus();
    fireEvent.keyDown(fixtures, { key: "ArrowLeft" });
    expect(output).toHaveFocus();

    fireEvent.keyDown(output, { key: "ArrowRight" });
    expect(fixtures).toHaveFocus();
    expect(disabled).toBeDisabled();
    expect(disabled).toHaveAttribute("aria-selected", "false");
  });

  it("activates a focused tab with Enter or Space and reports the selected id", () => {
    const onValueChange = vi.fn();
    render(<Tabs aria-label="Show views" tabs={tabs} onValueChange={onValueChange} />);

    const output = screen.getByRole("tab", { name: "Output" });
    output.focus();
    fireEvent.keyDown(output, { key: "Enter" });
    expect(onValueChange).toHaveBeenLastCalledWith("output");
    expect(output).toHaveAttribute("aria-selected", "true");

    fireEvent.keyDown(output, { key: " " });
    expect(onValueChange).toHaveBeenLastCalledWith("output");
  });

  it("honours a controlled selected value without selecting disabled tabs", () => {
    const onValueChange = vi.fn();
    render(
      <Tabs
        aria-label="Show views"
        value="scenes"
        onValueChange={onValueChange}
        tabs={[
          ...tabs,
          { id: "disabled", label: "Disabled", panel: <p>Unavailable</p>, disabled: true },
        ]}
      />,
    );

    const scenes = screen.getByRole("tab", { name: "Scenes" });
    const disabled = screen.getByRole("tab", { name: "Disabled" });
    expect(scenes).toHaveAttribute("aria-selected", "true");

    fireEvent.click(disabled);
    expect(onValueChange).not.toHaveBeenCalled();
  });
});
