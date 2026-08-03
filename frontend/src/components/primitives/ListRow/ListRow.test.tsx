import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import ListRow from "./ListRow";

describe("ListRow", () => {
  afterEach(() => cleanup());

  it("renders as a plain (non-interactive) row when no onSelect is given", () => {
    render(<ListRow label="Front of House" />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(screen.getByText("Front of House")).toBeInTheDocument();
  });

  it("renders as a button and calls onSelect when clicked", () => {
    const onSelect = vi.fn();
    render(<ListRow label="Front of House" onSelect={onSelect} />);
    fireEvent.click(screen.getByRole("button", { name: "Front of House" }));
    expect(onSelect).toHaveBeenCalledTimes(1);
  });

  it("renders optional meta content", () => {
    render(<ListRow label="Verse" meta="LIVE" onSelect={() => {}} />);
    expect(screen.getByText("LIVE")).toBeInTheDocument();
  });

  it("marks the row selected via aria-pressed", () => {
    render(<ListRow label="Verse" selected onSelect={() => {}} />);
    expect(screen.getByRole("button")).toHaveAttribute("aria-pressed", "true");
  });

  it("never calls onSelect when locked, even if clicked", () => {
    const onSelect = vi.fn();
    render(<ListRow label="Bridge" locked onSelect={onSelect} />);
    const button = screen.getByRole("button", { name: "Bridge" });
    expect(button).toBeDisabled();
    fireEvent.click(button);
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("forwards row attributes and exposes selected, disabled, and compact state", () => {
    render(
      <ListRow
        label="A very long fixture pool name that should retain its full accessible title"
        selected
        disabled
        density="compact"
        aria-label="Fixture pool row"
        onSelect={() => {}}
      />,
    );

    const row = screen.getByLabelText("Fixture pool row");
    expect(row).toBeDisabled();
    expect(row).toHaveAttribute("data-state", "selected");
    expect(row).toHaveAttribute("data-density", "compact");
    expect(row).toHaveAttribute("title", "A very long fixture pool name that should retain its full accessible title");
  });
});
