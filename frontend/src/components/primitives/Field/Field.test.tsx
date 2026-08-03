import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import Field from "./Field";

describe("Field", () => {
  afterEach(() => cleanup());

  it("renders a labeled text input by default", () => {
    render(<Field label="Scene name" value="" onChange={() => {}} />);
    expect(screen.getByText("Scene name")).toBeInTheDocument();
    expect(screen.getByRole("textbox")).toBeInTheDocument();
  });

  it("forwards onChange events from the rendered input", () => {
    const onChange = vi.fn();
    render(<Field label="Scene name" value="" onChange={onChange} />);
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "Verse" } });
    expect(onChange).toHaveBeenCalledTimes(1);
  });

  it("renders custom children instead of the default input when given (e.g. a <select>)", () => {
    render(
      <Field label="Chase unit">
        <select aria-label="unit-select">
          <option value="bar">bar</option>
        </select>
      </Field>,
    );
    expect(screen.getByRole("combobox", { name: "unit-select" })).toBeInTheDocument();
    // The default input must not also render alongside custom children.
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  });

  it("connects the label, description, and error to its default input", () => {
    render(
      <Field label="Scene name" description="Shown on the operator surface." error="A scene name is required." value="" onChange={() => {}} />,
    );

    const input = screen.getByRole("textbox", { name: "Scene name" });
    expect(input).toHaveAccessibleDescription("Shown on the operator surface. A scene name is required.");
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByText("A scene name is required.")).toHaveAttribute("role", "alert");
  });

  it("forwards required and disabled semantics to the field control", () => {
    render(<Field label="Universe" required disabled value="1" onChange={() => {}} />);
    const input = screen.getByRole("textbox", { name: /universe/i });
    expect(input).toBeRequired();
    expect(input).toBeDisabled();
  });
});
