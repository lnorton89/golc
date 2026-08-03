import { createRef } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import Button from "./Button";

describe("Button", () => {
  afterEach(() => cleanup());

  it("defaults to type=\"button\" (never accidentally submits a form)", () => {
    render(<Button>Click me</Button>);
    expect(screen.getByRole("button")).toHaveAttribute("type", "button");
  });

  it("calls onClick when clicked", () => {
    const onClick = vi.fn();
    render(<Button onClick={onClick}>Click me</Button>);
    fireEvent.click(screen.getByRole("button"));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("does not call onClick when disabled", () => {
    const onClick = vi.fn();
    render(
      <Button onClick={onClick} disabled>
        Click me
      </Button>,
    );
    fireEvent.click(screen.getByRole("button"));
    expect(onClick).not.toHaveBeenCalled();
  });

  it.each(["primary", "secondary", "destructive"] as const)("renders the %s variant without throwing", (variant) => {
    expect(() => render(<Button variant={variant}>{variant}</Button>)).not.toThrow();
    cleanup();
  });

  it("forwards an explicit type override (e.g. submit) when given", () => {
    render(<Button type="submit">Save</Button>);
    expect(screen.getByRole("button")).toHaveAttribute("type", "submit");
  });

  it("forwards native props and the button ref", () => {
    const ref = createRef<HTMLButtonElement>();
    render(
      <Button ref={ref} name="save-show" value="save">
        Save
      </Button>,
    );

    expect(ref.current).toBe(screen.getByRole("button", { name: "Save" }));
    expect(ref.current).toHaveAttribute("name", "save-show");
    expect(ref.current).toHaveAttribute("value", "save");
  });

  it("prevents duplicate dispatch while loading without losing its accessible name", () => {
    const onClick = vi.fn();
    render(
      <Button loading onClick={onClick}>
        Save show
      </Button>,
    );

    const button = screen.getByRole("button", { name: "Save show" });
    fireEvent.click(button);
    expect(button).toBeDisabled();
    expect(button).toHaveAttribute("aria-busy", "true");
    expect(onClick).not.toHaveBeenCalled();
  });
});
