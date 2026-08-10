import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "../test/renderWithProviders";

import { PlaybackSnapshotProvider, usePlaybackSnapshot } from "./PlaybackSnapshotContext";
import { useGolcStore } from "../store/store";

function Consumer() {
  const { state } = usePlaybackSnapshot();
  return <span data-testid="bpm">{state ? state.bpm : "none"}</span>;
}

describe("usePlaybackSnapshot", () => {
  afterEach(() => {
    cleanup();
    useGolcStore.getState().setConnectionStatus("connecting");
    vi.unstubAllGlobals();
  });

  it("throws when used without a PlaybackSnapshotProvider", () => {
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => render(<Consumer />)).toThrow(
      "usePlaybackSnapshot must be used within a PlaybackSnapshotProvider",
    );
    consoleError.mockRestore();
  });

  it("supplies the polled state to consumers once connected", async () => {
    useGolcStore.getState().setConnectionStatus("connected");
    vi.stubGlobal("go", {
      wails: {
        PlaybackService: {
          GetState: vi.fn().mockResolvedValue({
            exitCode: 0,
            stdout: JSON.stringify({ bpm: 128, scenes: [] }),
            stderr: "",
          }),
        },
      },
    });

    render(
      <PlaybackSnapshotProvider>
        <Consumer />
      </PlaybackSnapshotProvider>,
    );

    await waitFor(() => expect(screen.getByTestId("bpm")).toHaveTextContent("128"));
  });
});
