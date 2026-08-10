import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, renderHook } from "@testing-library/react";

import { useKeyboardWorkflow } from "./useKeyboardWorkflow";
import { dispatch } from "../lib/playbackDispatch";
import { beginHotkeyCapture, setHotkeyBinding } from "../lib/hotkeys";

vi.mock("../lib/playbackDispatch", async () => {
  const actual = await vi.importActual<typeof import("../lib/playbackDispatch")>("../lib/playbackDispatch");
  return {
    ...actual,
    dispatch: {
      switchScene: vi.fn(),
      setLayerEnabled: vi.fn(),
      setBPM: vi.fn(),
      recordTap: vi.fn(),
      evaluate: vi.fn(),
      getState: vi.fn(),
    },
  };
});

const baseOptions = {
  sceneNames: ["Alpha", "Beta", "Gamma"],
  activeSceneName: "Alpha",
  layerEnabled: { base_look: true, color_theme: false, chase: false, motion: false },
  bpm: 120,
};

describe("useKeyboardWorkflow", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    window.localStorage.clear();
  });

  it("switches to the Nth scene on a digit key", () => {
    renderHook(() => useKeyboardWorkflow(baseOptions));
    fireEvent.keyDown(window, { key: "2" });
    expect(dispatch.switchScene).toHaveBeenCalledWith("Beta");
  });

  it("ignores a digit key with no corresponding scene", () => {
    renderHook(() => useKeyboardWorkflow(baseOptions));
    fireEvent.keyDown(window, { key: "9" });
    expect(dispatch.switchScene).not.toHaveBeenCalled();
  });

  it.each([
    ["q", "base_look", true],
    ["w", "color_theme", false],
    ["e", "chase", false],
    ["r", "motion", false],
  ] as const)("toggles layer %s -> %s on the active scene", (key, kind, currentEnabled) => {
    renderHook(() => useKeyboardWorkflow(baseOptions));
    fireEvent.keyDown(window, { key });
    expect(dispatch.setLayerEnabled).toHaveBeenCalledWith("Alpha", kind, !currentEnabled);
  });

  it("does not toggle a layer when there is no active scene", () => {
    renderHook(() => useKeyboardWorkflow({ ...baseOptions, activeSceneName: null }));
    fireEvent.keyDown(window, { key: "q" });
    expect(dispatch.setLayerEnabled).not.toHaveBeenCalled();
  });

  it("records a tap on Space", () => {
    renderHook(() => useKeyboardWorkflow(baseOptions));
    fireEvent.keyDown(window, { code: "Space" });
    expect(dispatch.recordTap).toHaveBeenCalledTimes(1);
  });

  it("nudges BPM up on ArrowUp and down (floored at 1) on ArrowDown", () => {
    renderHook(() => useKeyboardWorkflow(baseOptions));
    fireEvent.keyDown(window, { key: "ArrowUp" });
    expect(dispatch.setBPM).toHaveBeenCalledWith(121);

    renderHook(() => useKeyboardWorkflow({ ...baseOptions, bpm: 0.5 }));
    fireEvent.keyDown(window, { key: "ArrowDown" });
    expect(dispatch.setBPM).toHaveBeenCalledWith(1);
  });

  it("evaluates at bar 0 on Enter", () => {
    renderHook(() => useKeyboardWorkflow(baseOptions));
    fireEvent.keyDown(window, { key: "Enter" });
    expect(dispatch.evaluate).toHaveBeenCalledWith(0);
  });

  it("matches a rebound key instead of the default, and stops matching the old default", () => {
    setHotkeyBinding("toggleBaseLook", "z");
    renderHook(() => useKeyboardWorkflow(baseOptions));

    fireEvent.keyDown(window, { key: "z" });
    expect(dispatch.setLayerEnabled).toHaveBeenCalledWith("Alpha", "base_look", false);

    fireEvent.keyDown(window, { key: "q" });
    expect(dispatch.setLayerEnabled).toHaveBeenCalledTimes(1);
  });

  it("ignores a bound key when Ctrl, Alt, or Meta is held, so it never collides with a chorded nav shortcut", () => {
    renderHook(() => useKeyboardWorkflow(baseOptions));

    fireEvent.keyDown(window, { key: "ArrowUp", altKey: true });
    fireEvent.keyDown(window, { key: "ArrowDown", ctrlKey: true, altKey: true });
    fireEvent.keyDown(window, { key: "q", ctrlKey: true });
    fireEvent.keyDown(window, { key: "Enter", metaKey: true });

    expect(dispatch.setBPM).not.toHaveBeenCalled();
    expect(dispatch.setLayerEnabled).not.toHaveBeenCalled();
    expect(dispatch.evaluate).not.toHaveBeenCalled();
  });

  it("ignores auto-repeat keydowns so a held key can't machine-gun an action", () => {
    renderHook(() => useKeyboardWorkflow(baseOptions));

    fireEvent.keyDown(window, { code: "Space" });
    for (let i = 0; i < 10; i += 1) {
      fireEvent.keyDown(window, { code: "Space", repeat: true });
    }

    expect(dispatch.recordTap).toHaveBeenCalledTimes(1);
  });

  it("ignores every shortcut while Settings is capturing a rebind", () => {
    renderHook(() => useKeyboardWorkflow(baseOptions));
    const release = beginHotkeyCapture();

    fireEvent.keyDown(window, { key: "w" });
    fireEvent.keyDown(window, { key: "3" });
    expect(dispatch.setLayerEnabled).not.toHaveBeenCalled();
    expect(dispatch.switchScene).not.toHaveBeenCalled();

    release();
    fireEvent.keyDown(window, { key: "w" });
    expect(dispatch.setLayerEnabled).toHaveBeenCalledTimes(1);
  });

  it("toggles a layer against its own pending value, not a snapshot that hasn't caught up yet", async () => {
    vi.mocked(dispatch.setLayerEnabled).mockResolvedValue({ exitCode: 0, stdout: "", stderr: "" });
    // layerEnabled stays `base_look: true` for the whole test: the props
    // are the 1s poll's snapshot, and both presses land inside one
    // interval, so neither sees the other's result.
    renderHook(() => useKeyboardWorkflow(baseOptions));

    fireEvent.keyDown(window, { key: "q" });
    fireEvent.keyDown(window, { key: "q" });

    expect(dispatch.setLayerEnabled).toHaveBeenNthCalledWith(1, "Alpha", "base_look", false);
    expect(dispatch.setLayerEnabled).toHaveBeenNthCalledWith(2, "Alpha", "base_look", true);
  });

  it("drops the pending layer value when the dispatch is rejected, so the next press reads real state", async () => {
    vi.mocked(dispatch.setLayerEnabled).mockResolvedValue({ exitCode: 1, stdout: "", stderr: "denied" });
    renderHook(() => useKeyboardWorkflow(baseOptions));

    fireEvent.keyDown(window, { key: "q" });
    await vi.waitFor(() => expect(dispatch.setLayerEnabled).toHaveBeenCalledTimes(1));

    fireEvent.keyDown(window, { key: "q" });
    // Snapshot still says enabled and the first attempt never landed, so
    // the second press must ask for `false` again -- not flip to `true`.
    expect(dispatch.setLayerEnabled).toHaveBeenNthCalledWith(2, "Alpha", "base_look", false);
  });

  it("clears pending layer values when the snapshot catches up", async () => {
    vi.mocked(dispatch.setLayerEnabled).mockResolvedValue({ exitCode: 0, stdout: "", stderr: "" });
    const { rerender } = renderHook((props: typeof baseOptions) => useKeyboardWorkflow(props), {
      initialProps: baseOptions,
    });

    fireEvent.keyDown(window, { key: "q" });
    expect(dispatch.setLayerEnabled).toHaveBeenNthCalledWith(1, "Alpha", "base_look", false);

    // The poll lands and now agrees with what we asked for.
    rerender({ ...baseOptions, layerEnabled: { ...baseOptions.layerEnabled, base_look: false } });

    fireEvent.keyDown(window, { key: "q" });
    expect(dispatch.setLayerEnabled).toHaveBeenNthCalledWith(2, "Alpha", "base_look", true);
  });

  it("ignores every shortcut while the event target is a text-entry element", () => {
    renderHook(() => useKeyboardWorkflow(baseOptions));
    const input = document.createElement("input");
    document.body.appendChild(input);
    fireEvent.keyDown(input, { key: "q" });
    fireEvent.keyDown(input, { key: "1" });
    fireEvent.keyDown(input, { code: "Space" });
    expect(dispatch.setLayerEnabled).not.toHaveBeenCalled();
    expect(dispatch.switchScene).not.toHaveBeenCalled();
    expect(dispatch.recordTap).not.toHaveBeenCalled();
    document.body.removeChild(input);
  });
});
