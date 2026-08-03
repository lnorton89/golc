import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import KeyboardShortcuts from "./KeyboardShortcuts";
import { setHotkeyBinding, setNavHotkeyBinding } from "../../lib/hotkeys";

describe("KeyboardShortcuts", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    cleanup();
    window.localStorage.clear();
  });

  it("renders the reference panel heading inside a named region", () => {
    render(<KeyboardShortcuts />);
    expect(screen.getByRole("heading", { name: "Keyboard Shortcuts" })).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Keyboard shortcuts reference" })).toBeInTheDocument();
  });

  it("groups every default shortcut by category, including the fixed scene-switch entry", () => {
    render(<KeyboardShortcuts />);

    expect(screen.getByRole("heading", { name: "Scenes" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Layers" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Tempo" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Transport" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Navigation" })).toBeInTheDocument();

    expect(screen.getByText("1 – 9")).toBeInTheDocument();
    expect(screen.getByText("Switch to the Nth scene in the current show")).toBeInTheDocument();
    expect(screen.getByText("Q")).toBeInTheDocument();
    expect(screen.getByText("Ctrl + K")).toBeInTheDocument();
  });

  it("reflects a custom playback binding immediately (never shows a stale key)", () => {
    setHotkeyBinding("toggleBaseLook", "z");
    render(<KeyboardShortcuts />);

    expect(screen.getByText("Z")).toBeInTheDocument();
    expect(screen.queryByText("Q")).not.toBeInTheDocument();
  });

  it("reflects a custom navigation chord immediately (never shows a stale chord)", () => {
    setNavHotkeyBinding("openQuickSwitcher", "Ctrl|Shift|j");
    render(<KeyboardShortcuts />);

    expect(screen.getByText("Ctrl + Shift + J")).toBeInTheDocument();
    expect(screen.queryByText("Ctrl + K")).not.toBeInTheDocument();
  });
});
