import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, renderHook, waitFor } from "../test/renderWithProviders";

import { usePlaybackStateSnapshot } from "./usePlaybackStateSnapshot";
import { useGolcStore } from "../store/store";
import { dispatch } from "../lib/playbackDispatch";

vi.mock("../lib/playbackDispatch", async () => {
  const actual = await vi.importActual<typeof import("../lib/playbackDispatch")>("../lib/playbackDispatch");
  return { ...actual, dispatch: { ...actual.dispatch, getState: vi.fn() } };
});

describe("usePlaybackStateSnapshot", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    vi.useRealTimers();
    useGolcStore.getState().setConnectionStatus("connecting");
  });

  it("does not poll while the daemon is not connected", async () => {
    useGolcStore.getState().setConnectionStatus("connecting");
    renderHook(() => usePlaybackStateSnapshot());
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(dispatch.getState).not.toHaveBeenCalled();
  });

  it("polls immediately and on each interval tick once connected", async () => {
    vi.mocked(dispatch.getState).mockResolvedValue({ bpm: 100, scenes: [] });
    useGolcStore.getState().setConnectionStatus("connected");

    const { result } = renderHook(() => usePlaybackStateSnapshot());
    await waitFor(() => expect(result.current.state).toEqual({ bpm: 100, scenes: [] }));
    expect(dispatch.getState).toHaveBeenCalledTimes(1);
  });

  it("refreshState re-fetches and updates state on demand", async () => {
    vi.mocked(dispatch.getState)
      .mockResolvedValueOnce({ bpm: 100, scenes: [] })
      .mockResolvedValueOnce({ bpm: 140, scenes: [] });
    useGolcStore.getState().setConnectionStatus("connected");

    const { result } = renderHook(() => usePlaybackStateSnapshot());
    await waitFor(() => expect(result.current.state?.bpm).toBe(100));

    await act(async () => {
      await result.current.refreshState();
    });
    // waitFor rather than a bare expect: refreshState resolves as soon as
    // the refetch itself has, and Query then notifies subscribers on a
    // microtask, so the re-render carrying the new value lands just after
    // act() unwinds. The assertion is unchanged in substance -- exactly two
    // getState calls have happened by here (mount + this refresh), and the
    // second one's value must reach the hook's returned state.
    await waitFor(() => expect(result.current.state?.bpm).toBe(140));
    expect(dispatch.getState).toHaveBeenCalledTimes(2);
  });

  it("stops polling once connection is lost, clearing the interval", async () => {
    vi.mocked(dispatch.getState).mockResolvedValue({ bpm: 100, scenes: [] });
    useGolcStore.getState().setConnectionStatus("connected");

    const { rerender } = renderHook(() => usePlaybackStateSnapshot());
    await waitFor(() => expect(dispatch.getState).toHaveBeenCalledTimes(1));

    act(() => useGolcStore.getState().setConnectionStatus("unreachable"));
    rerender();

    const callsAtDisconnect = vi.mocked(dispatch.getState).mock.calls.length;
    await new Promise((resolve) => setTimeout(resolve, 1200));
    expect(dispatch.getState).toHaveBeenCalledTimes(callsAtDisconnect);
  });
});
