// AppShell.test.tsx covers 13-24's own migration contract -- title
// behavior, resizing, and persistent safety space surviving a nav
// switch -- distinct from AppShell.navigation.test.tsx's exhaustive
// per-destination click sweep (which already covers navigation itself).
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "../test/renderWithProviders";

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

  // A nav chord used to call setActiveDestination straight through,
  // bypassing GuardedCommandRail's leave-the-guide confirm entirely: the
  // destination changed underneath the still-open guide with zero visible
  // effect, and exitGuide then returned to the *entry* destination, so the
  // keystrokes were silently discarded. Both keyboard entry points now
  // route through the same guard a rail click does.
  describe("navigating out of the Guided First Show", () => {
    // Overview renders its "Start Guide" CTA (D-10's manual entry point)
    // only once its own async refresh has settled, so this waits rather
    // than querying synchronously.
    async function startGuide() {
      render(<AppShell />);
      fireEvent.click(await screen.findByRole("button", { name: /start guide/i }));
      expect(screen.getByRole("navigation", { name: "First show steps" })).toBeInTheDocument();
    }

    it("prompts to leave the guide when a nav chord tries to navigate, instead of navigating underneath it", async () => {
      await startGuide();

      fireEvent.keyDown(window, { key: "ArrowDown", altKey: true });

      // The confirm is the guard: previously the chord went straight to
      // setActiveDestination and nothing at all appeared. (The guide rail
      // itself is inert to getByRole while the modal is open, so the
      // still-in-the-guide assertion lives in the cancel case below.)
      expect(screen.getByRole("alertdialog", { name: /leave the guide\?/i })).toBeInTheDocument();
    });

    it("stays in the guide when the confirm is cancelled", async () => {
      await startGuide();

      fireEvent.keyDown(window, { key: "ArrowDown", altKey: true });
      fireEvent.click(screen.getByRole("button", { name: /stay in guide/i }));

      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
      expect(screen.getByRole("navigation", { name: "First show steps" })).toBeInTheDocument();
    });

    it("leaves the guide for the chord's own destination once confirmed", async () => {
      await startGuide();

      fireEvent.keyDown(window, { key: "ArrowDown", altKey: true });
      fireEvent.click(screen.getByRole("button", { name: /leave guide/i }));

      expect(screen.queryByRole("navigation", { name: "First show steps" })).not.toBeInTheDocument();
      // Overview is the guide's entry destination and the first item in
      // its nav group, so Alt+ArrowDown lands on the next one.
      expect(screen.queryByRole("button", { name: "Overview" })).not.toHaveAttribute("aria-current", "page");
    });
  });
});
