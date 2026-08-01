import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import HotkeySettings from "./HotkeySettings";
import { getStoredHotkeys, getStoredNavHotkeys } from "../../lib/hotkeys";

describe("HotkeySettings", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    cleanup();
    window.localStorage.clear();
  });

  it("renders every rebindable playback action with its default key", () => {
    render(<HotkeySettings />);
    expect(screen.getByRole("button", { name: "Q" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Space" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "↑" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Enter" })).toBeInTheDocument();
  });

  it("renders every rebindable navigation action with its default chord, and the fixed scene-switch entry separately", () => {
    render(<HotkeySettings />);
    expect(screen.getByRole("button", { name: "Alt + ↑" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Alt + ↓" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Ctrl + Alt + ↑" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Ctrl + Alt + ↓" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Ctrl + K" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Ctrl + ," })).toBeInTheDocument();

    expect(screen.getByText("Switch to the Nth scene in the current show")).toBeInTheDocument();
    expect(screen.getByText("1 – 9")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "1 – 9" })).not.toBeInTheDocument();
  });

  it("rebinds a playback action to a newly pressed plain key", () => {
    render(<HotkeySettings />);

    fireEvent.click(screen.getByRole("button", { name: "Q" }));
    expect(screen.getByRole("button", { name: "Press a key…" })).toBeInTheDocument();

    fireEvent.keyDown(window, { key: "z" });

    expect(screen.getByRole("button", { name: "Z" })).toBeInTheDocument();
    expect(getStoredHotkeys().toggleBaseLook).toBe("z");
  });

  it("cancels playback recording on Escape without changing the binding", () => {
    render(<HotkeySettings />);

    fireEvent.click(screen.getByRole("button", { name: "Q" }));
    fireEvent.keyDown(window, { key: "Escape" });

    expect(screen.getByRole("button", { name: "Q" })).toBeInTheDocument();
    expect(getStoredHotkeys().toggleBaseLook).toBe("q");
  });

  it("rejects a plain key already bound to another playback action, and stays in recording mode", () => {
    render(<HotkeySettings />);

    fireEvent.click(screen.getByRole("button", { name: "Q" }));
    fireEvent.keyDown(window, { key: "w" });

    expect(screen.getByRole("alert")).toHaveTextContent(/already used by/i);
    expect(screen.getByRole("button", { name: "Press a key…" })).toBeInTheDocument();
    expect(getStoredHotkeys().toggleBaseLook).toBe("q");
  });

  it("rejects a digit reserved for scene switching", () => {
    render(<HotkeySettings />);

    fireEvent.click(screen.getByRole("button", { name: "Enter" }));
    fireEvent.keyDown(window, { key: "5" });

    expect(screen.getByRole("alert")).toHaveTextContent(/reserved for scene switching/i);
    expect(getStoredHotkeys().evaluate).toBe("Enter");
  });

  it("rejects a chorded key press while recording a playback action", () => {
    render(<HotkeySettings />);

    fireEvent.click(screen.getByRole("button", { name: "Q" }));
    fireEvent.keyDown(window, { key: "z", ctrlKey: true });

    expect(screen.getByRole("alert")).toHaveTextContent(/can't use ctrl\/alt\/cmd/i);
    expect(getStoredHotkeys().toggleBaseLook).toBe("q");
  });

  it("shows a reset button only for customized playback actions, and resets on click", () => {
    render(<HotkeySettings />);

    expect(screen.queryByRole("button", { name: /reset toggle base look/i })).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Q" }));
    fireEvent.keyDown(window, { key: "z" });

    const resetButton = screen.getByRole("button", { name: /reset toggle base look/i });
    expect(resetButton).toBeEnabled();

    fireEvent.click(resetButton);
    expect(screen.getByRole("button", { name: "Q" })).toBeInTheDocument();
    expect(getStoredHotkeys().toggleBaseLook).toBe("q");
  });

  it("rebinds a navigation action to a newly pressed chord", () => {
    render(<HotkeySettings />);

    fireEvent.click(screen.getByRole("button", { name: "Ctrl + K" }));
    expect(screen.getByRole("button", { name: "Press a combo…" })).toBeInTheDocument();

    fireEvent.keyDown(window, { key: "j", ctrlKey: true, shiftKey: true });

    expect(screen.getByRole("button", { name: "Ctrl + Shift + J" })).toBeInTheDocument();
    expect(getStoredNavHotkeys().openQuickSwitcher).toBe("Ctrl|Shift|j");
  });

  it("keeps waiting through a lone modifier keydown while recording a navigation chord", () => {
    render(<HotkeySettings />);

    fireEvent.click(screen.getByRole("button", { name: "Ctrl + K" }));
    fireEvent.keyDown(window, { key: "Control", ctrlKey: true });
    expect(screen.getByRole("button", { name: "Press a combo…" })).toBeInTheDocument();

    fireEvent.keyDown(window, { key: "j", ctrlKey: true });
    expect(screen.getByRole("button", { name: "Ctrl + J" })).toBeInTheDocument();
  });

  it("rejects a bare key with no modifier while recording a navigation chord", () => {
    render(<HotkeySettings />);

    fireEvent.click(screen.getByRole("button", { name: "Ctrl + K" }));
    fireEvent.keyDown(window, { key: "j" });

    expect(screen.getByRole("alert")).toHaveTextContent(/need ctrl or alt/i);
    expect(getStoredNavHotkeys().openQuickSwitcher).toBe("Ctrl|k");
  });

  it("rejects a chord already bound to another navigation action, and stays in recording mode", () => {
    render(<HotkeySettings />);

    fireEvent.click(screen.getByRole("button", { name: "Ctrl + K" }));
    fireEvent.keyDown(window, { key: ",", ctrlKey: true });

    expect(screen.getByRole("alert")).toHaveTextContent(/already used by/i);
    expect(screen.getByRole("button", { name: "Press a combo…" })).toBeInTheDocument();
    expect(getStoredNavHotkeys().openQuickSwitcher).toBe("Ctrl|k");
  });

  it("cancels navigation recording on Escape without changing the binding", () => {
    render(<HotkeySettings />);

    fireEvent.click(screen.getByRole("button", { name: "Ctrl + K" }));
    fireEvent.keyDown(window, { key: "Escape" });

    expect(screen.getByRole("button", { name: "Ctrl + K" })).toBeInTheDocument();
    expect(getStoredNavHotkeys().openQuickSwitcher).toBe("Ctrl|k");
  });

  it("shows a reset button only for customized navigation actions, and resets on click", () => {
    render(<HotkeySettings />);

    expect(screen.queryByRole("button", { name: /reset open the quick switcher/i })).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Ctrl + K" }));
    fireEvent.keyDown(window, { key: "j", ctrlKey: true });

    const resetButton = screen.getByRole("button", { name: /reset open the quick switcher/i });
    expect(resetButton).toBeEnabled();

    fireEvent.click(resetButton);
    expect(screen.getByRole("button", { name: "Ctrl + K" })).toBeInTheDocument();
    expect(getStoredNavHotkeys().openQuickSwitcher).toBe("Ctrl|k");
  });

  it("'Reset all to defaults' is disabled until any binding changes, then restores both playback and navigation", () => {
    render(<HotkeySettings />);

    const resetAll = screen.getByRole("button", { name: "Reset all to defaults" });
    expect(resetAll).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Q" }));
    fireEvent.keyDown(window, { key: "z" });
    expect(resetAll).toBeEnabled();

    fireEvent.click(screen.getByRole("button", { name: "Ctrl + K" }));
    fireEvent.keyDown(window, { key: "j", ctrlKey: true });
    expect(resetAll).toBeEnabled();

    fireEvent.click(resetAll);
    expect(screen.getByRole("button", { name: "Q" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Ctrl + K" })).toBeInTheDocument();
    expect(resetAll).toBeDisabled();
  });
});
