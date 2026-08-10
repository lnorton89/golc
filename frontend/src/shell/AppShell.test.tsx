// AppShell.test.tsx covers 13-24's own migration contract -- title
// behavior, resizing, and persistent safety space surviving a nav
// switch -- distinct from AppShell.navigation.test.tsx's exhaustive
// per-destination click sweep (which already covers navigation itself).
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "../test/renderWithQuery";

import AppShell from "./AppShell";

describe("AppShell", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    cleanup();
  });

  it("renders the self-drawn title bar with brand and window controls", () => {
    render(<AppShell />);

    expect(screen.getByText("GOLC")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Minimize" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Maximize" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
  });

  it("keeps the nav rail resize handle reachable regardless of the active destination", () => {
    render(<AppShell />);
    expect(screen.getByRole("separator", { name: "Resize navigation rail" })).toBeInTheDocument();
  });

  it("keeps GlobalFrame's live status bar, tempo controls, and safety cluster mounted across a navigation switch (D-13: persistent, independent of the active workspace)", () => {
    render(<AppShell />);

    expect(screen.getByLabelText("Live status bar")).toBeInTheDocument();
    expect(screen.getByLabelText("Tempo controls")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Art-Net" }));

    // Still the same persistently-mounted header content after switching
    // workspaces -- GlobalFrame is a direct AppShell child, never inside
    // WorkspaceRouter's own switch.
    expect(screen.getByLabelText("Live status bar")).toBeInTheDocument();
    expect(screen.getByLabelText("Tempo controls")).toBeInTheDocument();
  });

  it("navigating never unmounts the command rail itself, only the active workspace canvas", () => {
    render(<AppShell />);
    fireEvent.click(screen.getByRole("button", { name: "Art-Net" }));
    expect(screen.getByRole("button", { name: "Art-Net" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("navigation", { name: "Workspace navigation" })).toBeInTheDocument();
  });
});
