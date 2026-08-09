import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

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

  it("uses arrow keys, Home, and End to focus and select enabled tabs", async () => {
    const user = userEvent.setup();
    render(<Tabs aria-label="Show views" tabs={tabs} />);

    const fixtures = screen.getByRole("tab", { name: "Fixtures" });
    const scenes = screen.getByRole("tab", { name: "Scenes" });
    const output = screen.getByRole("tab", { name: "Output" });

    fixtures.focus();
    await waitFor(() => expect(fixtures).toHaveFocus());
    // Base UI's Tabs handles arrow/Home/End navigation through its own
    // composite-list keyboard handling, which needs a real keypress
    // (user-event) to exercise -- a synthetic fireEvent.keyDown on a single
    // button doesn't drive it the same way.
    await user.keyboard("{ArrowRight}");
    await waitFor(() => expect(scenes).toHaveFocus());
    await waitFor(() => expect(scenes).toHaveAttribute("aria-selected", "true"));

    await user.keyboard("{End}");
    await waitFor(() => expect(output).toHaveFocus());
    await waitFor(() => expect(output).toHaveAttribute("aria-selected", "true"));

    await user.keyboard("{Home}");
    await waitFor(() => expect(fixtures).toHaveFocus());
    await waitFor(() => expect(fixtures).toHaveAttribute("aria-selected", "true"));
  });

  it("wraps arrow navigation and skips disabled tabs", async () => {
    const user = userEvent.setup();
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
    await waitFor(() => expect(fixtures).toHaveFocus());
    await user.keyboard("{ArrowLeft}");
    await waitFor(() => expect(output).toHaveFocus());

    await user.keyboard("{ArrowRight}");
    await waitFor(() => expect(fixtures).toHaveFocus());
    // Base UI marks a disabled Tab with aria-disabled/data-disabled rather
    // than the native `disabled` attribute -- keeps it perceivable/roving-
    // tabindex-aware in the composite list rather than removing it from the
    // accessibility tree outright, the recommended pattern for tablists.
    expect(disabled).toHaveAttribute("aria-disabled", "true");
    expect(disabled).toHaveAttribute("aria-selected", "false");
  });

  it("activates a focused tab with Enter or Space and reports the selected id", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(<Tabs aria-label="Show views" tabs={tabs} onValueChange={onValueChange} />);

    const output = screen.getByRole("tab", { name: "Output" });
    output.focus();
    await waitFor(() => expect(output).toHaveFocus());
    await user.keyboard("{Enter}");
    await waitFor(() => expect(onValueChange).toHaveBeenLastCalledWith("output"));
    await waitFor(() => expect(output).toHaveAttribute("aria-selected", "true"));

    await user.keyboard(" ");
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
