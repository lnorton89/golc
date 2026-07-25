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
});
