import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

import TempoControls from "./TempoControls";
import { PlaybackSnapshotProvider } from "../../shell/PlaybackSnapshotContext";
import { useGolcStore } from "../../store/store";
import { dispatch } from "../../lib/playbackDispatch";

vi.mock("../../lib/playbackDispatch", async () => {
  const actual = await vi.importActual<typeof import("../../lib/playbackDispatch")>("../../lib/playbackDispatch");
  return { ...actual, dispatch: { ...actual.dispatch, setBPM: vi.fn(), recordTap: vi.fn(), getState: vi.fn() } };
});

describe("TempoControls", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    useGolcStore.getState().setConnectionStatus("connecting");
  });

  it("shows a single clickable BPM display reflecting the polled value, with no separate input visible", async () => {
    vi.mocked(dispatch.getState).mockResolvedValue({ bpm: 128, scenes: [] });
    useGolcStore.getState().setConnectionStatus("connected");

    render(
      <PlaybackSnapshotProvider>
        <TempoControls />
      </PlaybackSnapshotProvider>,
    );

    await waitFor(() => expect(screen.getByRole("button", { name: "128 BPM" })).toBeInTheDocument());
    expect(screen.queryByLabelText("BPM")).not.toBeInTheDocument();
  });

  it("turns into an editable input on click, seeded with the current BPM", async () => {
    vi.mocked(dispatch.getState).mockResolvedValue({ bpm: 100, scenes: [] });
    useGolcStore.getState().setConnectionStatus("connected");

    render(
      <PlaybackSnapshotProvider>
        <TempoControls />
      </PlaybackSnapshotProvider>,
    );
    await waitFor(() => screen.getByRole("button", { name: "100 BPM" }));

    fireEvent.click(screen.getByRole("button", { name: "100 BPM" }));
    expect(screen.getByLabelText("BPM")).toHaveValue(100);
  });

  it("commits the new BPM and returns to display mode on Enter, with no separate Set button", async () => {
    vi.mocked(dispatch.getState).mockResolvedValue({ bpm: 100, scenes: [] });
    vi.mocked(dispatch.setBPM).mockResolvedValue({ exitCode: 0, stdout: "", stderr: "" });
    useGolcStore.getState().setConnectionStatus("connected");

    render(
      <PlaybackSnapshotProvider>
        <TempoControls />
      </PlaybackSnapshotProvider>,
    );
    await waitFor(() => screen.getByRole("button", { name: "100 BPM" }));
    expect(screen.queryByRole("button", { name: "Set" })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "100 BPM" }));
    fireEvent.change(screen.getByLabelText("BPM"), { target: { value: "140" } });
    fireEvent.keyDown(screen.getByLabelText("BPM"), { key: "Enter" });

    await waitFor(() => expect(dispatch.setBPM).toHaveBeenCalledWith(140));
    await waitFor(() => expect(screen.queryByLabelText("BPM")).not.toBeInTheDocument());
  });

  it("commits on blur as well as Enter", async () => {
    vi.mocked(dispatch.getState).mockResolvedValue({ bpm: 100, scenes: [] });
    vi.mocked(dispatch.setBPM).mockResolvedValue({ exitCode: 0, stdout: "", stderr: "" });
    useGolcStore.getState().setConnectionStatus("connected");

    render(
      <PlaybackSnapshotProvider>
        <TempoControls />
      </PlaybackSnapshotProvider>,
    );
    await waitFor(() => screen.getByRole("button", { name: "100 BPM" }));

    fireEvent.click(screen.getByRole("button", { name: "100 BPM" }));
    fireEvent.change(screen.getByLabelText("BPM"), { target: { value: "90" } });
    fireEvent.blur(screen.getByLabelText("BPM"));

    await waitFor(() => expect(dispatch.setBPM).toHaveBeenCalledWith(90));
  });

  it("discards the edit on Escape without committing", async () => {
    vi.mocked(dispatch.getState).mockResolvedValue({ bpm: 100, scenes: [] });
    useGolcStore.getState().setConnectionStatus("connected");

    render(
      <PlaybackSnapshotProvider>
        <TempoControls />
      </PlaybackSnapshotProvider>,
    );
    await waitFor(() => screen.getByRole("button", { name: "100 BPM" }));

    fireEvent.click(screen.getByRole("button", { name: "100 BPM" }));
    fireEvent.change(screen.getByLabelText("BPM"), { target: { value: "999" } });
    fireEvent.keyDown(screen.getByLabelText("BPM"), { key: "Escape" });

    expect(screen.queryByLabelText("BPM")).not.toBeInTheDocument();
    expect(dispatch.setBPM).not.toHaveBeenCalled();
  });

  it("records a tap and refreshes state on Tap", async () => {
    vi.mocked(dispatch.getState).mockResolvedValue({ bpm: 100, scenes: [] });
    vi.mocked(dispatch.recordTap).mockResolvedValue({ exitCode: 0, stdout: "", stderr: "" });
    useGolcStore.getState().setConnectionStatus("connected");

    render(
      <PlaybackSnapshotProvider>
        <TempoControls />
      </PlaybackSnapshotProvider>,
    );
    await waitFor(() => screen.getByRole("button", { name: "100 BPM" }));

    fireEvent.click(screen.getByRole("button", { name: "Tap" }));
    await waitFor(() => expect(dispatch.recordTap).toHaveBeenCalledTimes(1));
  });
});
