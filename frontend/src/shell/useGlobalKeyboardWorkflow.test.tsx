import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

import { PlaybackSnapshotProvider, usePlaybackSnapshot } from "./PlaybackSnapshotContext";
import { useGlobalKeyboardWorkflow } from "./useGlobalKeyboardWorkflow";
import { useGolcStore } from "../store/store";

function Harness() {
  const { helpOpen } = useGlobalKeyboardWorkflow();
  const { state } = usePlaybackSnapshot();
  return (
    <div>
      <span data-testid="help-state">{helpOpen ? "open" : "closed"}</span>
      <span data-testid="scene-count">{state?.scenes.length ?? 0}</span>
      <input aria-label="typing target" />
    </div>
  );
}

describe("useGlobalKeyboardWorkflow", () => {
  afterEach(() => {
    cleanup();
    useGolcStore.getState().setConnectionStatus("connecting");
    vi.unstubAllGlobals();
  });

  it("toggles help open on '?' and closes on Escape", async () => {
    render(
      <PlaybackSnapshotProvider>
        <Harness />
      </PlaybackSnapshotProvider>,
    );

    expect(screen.getByTestId("help-state")).toHaveTextContent("closed");

    fireEvent.keyDown(window, { key: "?" });
    expect(screen.getByTestId("help-state")).toHaveTextContent("open");

    fireEvent.keyDown(window, { key: "Escape" });
    expect(screen.getByTestId("help-state")).toHaveTextContent("closed");
  });

  it("does not toggle help when the keystroke targets a text input", () => {
    render(
      <PlaybackSnapshotProvider>
        <Harness />
      </PlaybackSnapshotProvider>,
    );

    fireEvent.keyDown(screen.getByLabelText("typing target"), { key: "?" });
    expect(screen.getByTestId("help-state")).toHaveTextContent("closed");
  });

  it("dispatches a scene switch derived from the polled scene list on a digit key", async () => {
    useGolcStore.getState().setConnectionStatus("connected");
    const switchScene = vi.fn().mockResolvedValue({ exitCode: 0, stdout: "", stderr: "" });
    vi.stubGlobal("go", {
      wails: {
        PlaybackService: {
          GetState: vi.fn().mockResolvedValue({
            exitCode: 0,
            stdout: JSON.stringify({
              bpm: 120,
              scenes: [
                { name: "Alpha", active: true, barsPerLoop: 4, layers: [] },
                { name: "Beta", active: false, barsPerLoop: 4, layers: [] },
              ],
            }),
            stderr: "",
          }),
          SwitchScene: switchScene,
        },
      },
    });

    render(
      <PlaybackSnapshotProvider>
        <Harness />
      </PlaybackSnapshotProvider>,
    );

    await waitFor(() => expect(screen.getByTestId("scene-count")).toHaveTextContent("2"));

    // The "scene-count" text and useKeyboardWorkflow's own keydown listener
    // are two separate effects of the same setState (one a DOM commit, the
    // other a passive effect re-running to close over the fresh
    // sceneNames), so the listener is not guaranteed to have been rebuilt
    // the instant "scene-count" reads "2" -- under load (the full suite
    // running many files concurrently) a single fireEvent.keyDown could
    // race a still-stale (sceneNames=[]) listener and be silently dropped.
    // Firing the keydown inside waitFor's own retry loop removes the race
    // entirely: it keeps re-dispatching until the (by-then-current)
    // listener actually captures it, rather than gambling on one fixed
    // moment in time.
    await waitFor(() => {
      fireEvent.keyDown(window, { key: "2" });
      expect(switchScene).toHaveBeenCalledWith("Beta");
    });
  });
});
