// OperatorSurface.detailRace.test.tsx exercises the actual race the
// 2026-08-10 review pass found, not a happy path: two overlapping
// ShowSurface reads resolving out of order. Selecting surface A then
// surface B inside A's round trip used to leave the detail panel headed
// "B" while listing A's controls when A resolved last -- and the
// follow-on AssignItem then wrote A's control onto B.
//
// Deliberately drives the real component through the bridge rather than
// unit-testing useLatestRequest alone: the guard is only worth anything if
// it is actually threaded through refreshDetail's commit points.
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "../../test/renderWithProviders";

import OperatorSurface from "./OperatorSurface";
import { useGolcStore } from "../../store/store";
import { PlaybackSnapshotProvider } from "../../shell/PlaybackSnapshotContext";

function summary(name: string, id: string) {
  return {
    id,
    name,
    sceneCount: 1,
    layerCount: 0,
    masterCount: 0,
    safetyCount: 0,
    assignedCount: 1,
    midiMappingCount: 0,
  };
}

function detail(name: string, id: string, controlLabel: string) {
  return {
    id,
    name,
    controls: [{ kind: "scene", scene: controlLabel, label: controlLabel, assigned: true }],
    midiMappingCount: 0,
  };
}

/** deferred lets the test decide the order two in-flight reads resolve in. */
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((r) => {
    resolve = r;
  });
  return { promise, resolve };
}

describe("OperatorSurface detail fetch ordering", () => {
  afterEach(() => {
    cleanup();
    delete (window as unknown as { go?: unknown }).go;
    useGolcStore.getState().setConnectionStatus("connecting");
    vi.restoreAllMocks();
  });

  it("keeps the newer surface's controls when the older read resolves last", async () => {
    useGolcStore.getState().setConnectionStatus("connected");

    const alphaDetail = deferred<ReturnType<typeof detail>>();
    const betaDetail = deferred<ReturnType<typeof detail>>();

    const ShowSurface = vi.fn((name: string) =>
      name === "Alpha" ? alphaDetail.promise : betaDetail.promise,
    );

    (window as unknown as { go: unknown }).go = {
      wails: {
        SurfaceService: {
          ListSurfaces: vi.fn().mockResolvedValue([summary("Alpha", "1"), summary("Beta", "2")]),
          ShowSurface,
          CreateSurface: vi.fn(),
          AssignItem: vi.fn(),
          UnassignItem: vi.fn(),
          RemoveSurface: vi.fn(),
          AuthorizeControl: vi.fn(),
        },
        SafetyService: { SetActiveSurface: vi.fn().mockResolvedValue({ exitCode: 0, stdout: "", stderr: "" }) },
        PlaybackService: {
          SetActiveSurface: vi.fn().mockResolvedValue({ exitCode: 0, stdout: "", stderr: "" }),
          GetState: vi.fn().mockResolvedValue(undefined),
        },
      },
    };

    render(
      <PlaybackSnapshotProvider>
        <OperatorSurface />
      </PlaybackSnapshotProvider>,
    );

    // Select Alpha, then Beta before Alpha's round trip finishes.
    fireEvent.click(await screen.findByRole("button", { name: /^Alpha/ }));
    await waitFor(() => expect(ShowSurface).toHaveBeenCalledWith("Alpha"));
    fireEvent.click(await screen.findByRole("button", { name: /^Beta/ }));
    await waitFor(() => expect(ShowSurface).toHaveBeenCalledWith("Beta"));

    // Beta lands first, then the slower Alpha response arrives.
    betaDetail.resolve(detail("Beta", "2", "BetaControl"));
    await screen.findByText("BetaControl");

    alphaDetail.resolve(detail("Alpha", "1", "AlphaControl"));
    await waitFor(() => expect(ShowSurface).toHaveBeenCalledTimes(2));

    // The stale Alpha response must not repopulate the panel.
    expect(screen.getByText("BetaControl")).toBeInTheDocument();
    expect(screen.queryByText("AlphaControl")).not.toBeInTheDocument();
  });
});
