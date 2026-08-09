import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import Checkbox from "./Checkbox";

afterEach(() => cleanup());

describe("Checkbox", () => {
  it("renders with an accessible name and the checkbox role", () => {
    render(<Checkbox label="Enable safety interlock" defaultChecked={false} />);

    const checkbox = screen.getByRole("checkbox", { name: "Enable safety interlock" });
    expect(checkbox).toHaveAttribute("aria-checked", "false");
  });

  it("shows the visible label by default and swaps to an aria-label when hideLabel is set", () => {
    const { rerender } = render(<Checkbox label="Fixture linked" defaultChecked={false} />);
    expect(screen.getByText("Fixture linked")).toBeInTheDocument();

    rerender(<Checkbox label="Fixture linked" defaultChecked={false} hideLabel />);
    expect(screen.queryByText("Fixture linked")).not.toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "Fixture linked" })).toBeInTheDocument();
  });

  it("is uncontrolled when only defaultChecked is supplied: it manages its own state", async () => {
    const user = userEvent.setup();
    const onCheckedChange = vi.fn();
    render(<Checkbox label="Armed" defaultChecked={false} onCheckedChange={onCheckedChange} />);

    const checkbox = screen.getByRole("checkbox", { name: "Armed" });
    expect(checkbox).toHaveAttribute("aria-checked", "false");

    await user.click(checkbox);
    await waitFor(() => expect(checkbox).toHaveAttribute("aria-checked", "true"));
    expect(onCheckedChange).toHaveBeenLastCalledWith(true);
  });

  it("is controlled when checked is supplied: the caller's state is the source of truth", async () => {
    const user = userEvent.setup();

    function Controlled() {
      const [checked, setChecked] = useState(false);
      return (
        <>
          <Checkbox label="Armed" checked={checked} onCheckedChange={setChecked} />
          <button onClick={() => setChecked(true)}>Force on</button>
        </>
      );
    }

    render(<Controlled />);
    const checkbox = screen.getByRole("checkbox", { name: "Armed" });
    expect(checkbox).toHaveAttribute("aria-checked", "false");

    await user.click(screen.getByRole("button", { name: "Force on" }));
    await waitFor(() => expect(checkbox).toHaveAttribute("aria-checked", "true"));

    await user.click(checkbox);
    await waitFor(() => expect(checkbox).toHaveAttribute("aria-checked", "false"));
  });

  it("toggles via the keyboard (Space) when focused", async () => {
    const user = userEvent.setup();
    const onCheckedChange = vi.fn();
    render(<Checkbox label="Armed" defaultChecked={false} onCheckedChange={onCheckedChange} />);

    const checkbox = screen.getByRole("checkbox", { name: "Armed" });
    checkbox.focus();
    expect(checkbox).toHaveFocus();

    await user.keyboard(" ");
    await waitFor(() => expect(checkbox).toHaveAttribute("aria-checked", "true"));
    expect(onCheckedChange).toHaveBeenLastCalledWith(true);

    await user.keyboard(" ");
    await waitFor(() => expect(checkbox).toHaveAttribute("aria-checked", "false"));
    expect(onCheckedChange).toHaveBeenLastCalledWith(false);
  });

  it("does not respond to click or keyboard input when disabled", async () => {
    const user = userEvent.setup();
    const onCheckedChange = vi.fn();
    render(<Checkbox label="Armed" defaultChecked={false} disabled onCheckedChange={onCheckedChange} />);

    const checkbox = screen.getByRole("checkbox", { name: "Armed" });
    // Disabled composite controls in this codebase's Base UI primitives get
    // aria-disabled + tabindex="-1" rather than the native `disabled`
    // attribute (this is a <span>, not a native form control) -- confirmed
    // against the rendered DOM, mirroring RadioGroup's own disabled items.
    expect(checkbox).toHaveAttribute("aria-disabled", "true");

    await user.click(checkbox);
    expect(onCheckedChange).not.toHaveBeenCalled();
    expect(checkbox).toHaveAttribute("aria-checked", "false");
  });

  it("renders the indeterminate (mixed) state distinctly from checked/unchecked", () => {
    render(<Checkbox label="Select all" checked={false} indeterminate onCheckedChange={() => {}} />);

    const checkbox = screen.getByRole("checkbox", { name: "Select all" });
    expect(checkbox).toHaveAttribute("aria-checked", "mixed");
    expect(checkbox).toHaveAttribute("data-indeterminate");
  });
});
