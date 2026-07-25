import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import SceneList from "./SceneList";
import type { ProgSceneView } from "../../lib/wailsBridge";

const scenes: ProgSceneView[] = [
  { name: "Alpha", active: true, barsPerLoop: 4, layers: [] },
  { name: "Beta", active: false, barsPerLoop: 8, layers: [] },
];

describe("SceneList", () => {
  afterEach(() => cleanup());

  it("shows an empty state when there are no scenes", () => {
    render(<SceneList scenes={[]} selectedName={null} onSelect={() => {}} onCreate={() => {}} />);
    expect(screen.getByText("No scenes yet — create one above.")).toBeInTheDocument();
  });

  it("renders each scene with LIVE or bar-count meta and marks the selected one", () => {
    render(<SceneList scenes={scenes} selectedName="Alpha" onSelect={() => {}} onCreate={() => {}} />);
    expect(screen.getByRole("button", { name: "AlphaLIVE" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "Beta8bar" })).toHaveAttribute("aria-pressed", "false");
  });

  it("calls onSelect with the clicked scene's name", () => {
    const onSelect = vi.fn();
    render(<SceneList scenes={scenes} selectedName="Alpha" onSelect={onSelect} onCreate={() => {}} />);
    fireEvent.click(screen.getByRole("button", { name: "Beta8bar" }));
    expect(onSelect).toHaveBeenCalledWith("Beta");
  });

  it("toggles the create form open and closed via the New/Cancel button", () => {
    render(<SceneList scenes={[]} selectedName={null} onSelect={() => {}} onCreate={() => {}} />);
    expect(screen.queryByLabelText("New scene name")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "+ New" }));
    expect(screen.getByLabelText("New scene name")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByLabelText("New scene name")).not.toBeInTheDocument();
  });

  it("calls onCreate with the trimmed name and parsed bar count, then closes the form", () => {
    const onCreate = vi.fn();
    render(<SceneList scenes={[]} selectedName={null} onSelect={() => {}} onCreate={onCreate} />);

    fireEvent.click(screen.getByRole("button", { name: "+ New" }));
    fireEvent.change(screen.getByLabelText("New scene name"), { target: { value: "  Gamma  " } });
    fireEvent.change(screen.getByLabelText("Bars per loop"), { target: { value: "16" } });
    fireEvent.click(screen.getByRole("button", { name: "Create" }));

    expect(onCreate).toHaveBeenCalledWith("Gamma", 16);
    expect(screen.queryByLabelText("New scene name")).not.toBeInTheDocument();
  });

  it("does not call onCreate when the name is blank", () => {
    const onCreate = vi.fn();
    render(<SceneList scenes={[]} selectedName={null} onSelect={() => {}} onCreate={onCreate} />);

    fireEvent.click(screen.getByRole("button", { name: "+ New" }));
    fireEvent.click(screen.getByRole("button", { name: "Create" }));

    expect(onCreate).not.toHaveBeenCalled();
  });
});
