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

  it("renders a bounded heading, explanation, and optional named action", () => {
    render(
      <EmptyState
        heading="No fixture pools yet"
        body="Create a fixture pool before patching fixtures into this show."
        action={<button type="button">Create fixture pool</button>}
      />,
    );

    expect(screen.getByRole("heading", { name: "No fixture pools yet" })).toBeInTheDocument();
    expect(screen.getByText("Create a fixture pool before patching fixtures into this show.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create fixture pool" })).toBeInTheDocument();
  });
});
