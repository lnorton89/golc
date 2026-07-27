import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { Package } from "lucide-react";

import EmptyState from "./EmptyState";

describe("EmptyState", () => {
  afterEach(() => cleanup());

  it("renders the message", () => {
    render(<EmptyState>No fixture pools yet.</EmptyState>);
    expect(screen.getByText("No fixture pools yet.")).toBeInTheDocument();
  });

  it("accepts a custom icon without changing the message", () => {
    render(<EmptyState icon={Package}>No fixture pools yet.</EmptyState>);
    expect(screen.getByText("No fixture pools yet.")).toBeInTheDocument();
  });
});
