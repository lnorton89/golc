// OperatorSurface.activeSurface.test.tsx guards the CR-01 fix's
// correctness under the shell restructure's unmount-on-navigate contract
// (shell/WorkspaceRouter.tsx): entering "operate" mode scopes real
// Safety/Playback dispatch to the selected surface via SetActiveSurface;
// unmounting (what WorkspaceRouter does on every navigate-away) must clear
// that scoping back to "" via the same effect's cleanup function. Without
// this test, a future "keep workspaces mounted-but-hidden instead of
// unmounting" optimization would silently reintroduce stuck active-surface
// scoping with nothing else catching it.
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "../../test/renderWithQuery";

import OperatorSurface from "./OperatorSurface";
import { useGolcStore } from "../../store/store";
import { PlaybackSnapshotProvider } from "../../shell/PlaybackSnapshotContext";

const SURFACE_NAME = "Front of House";

function installMockBridge() {
  const setSafetyActiveSurface = vi.fn().mockResolvedValue({ exitCode: 0, stdout: "", stderr: "" });
  const setPlaybackActiveSurface = vi.fn().mockResolvedValue({ exitCode: 0, stdout: "", stderr: "" });

  const surfaceService = {
    ListSurfaces: vi.fn().mockResolvedValue([
      {
        id: "1",
        name: SURFACE_NAME,
        sceneCount: 1,
        layerCount: 0,
        masterCount: 0,
        safetyCount: 0,
        assignedCount: 1,
        midiMappingCount: 0,
      },
    ]),
    ShowSurface: vi.fn().mockResolvedValue({
      id: "1",
      name: SURFACE_NAME,
      controls: [{ kind: "scene", scene: "Verse", label: "Verse", assigned: true }],
      midiMappingCount: 0,
    }),
    CreateSurface: vi.fn(),
    AssignItem: vi.fn(),
    UnassignItem: vi.fn(),
    RemoveSurface: vi.fn(),
    AuthorizeControl: vi.fn(),
  };

  (window as unknown as { go: unknown }).go = {
    wails: {
      SurfaceService: surfaceService,
      SafetyService: { SetActiveSurface: setSafetyActiveSurface },
      PlaybackService: { SetActiveSurface: setPlaybackActiveSurface, GetState: vi.fn().mockResolvedValue(undefined) },
    },
  };

  return { setSafetyActiveSurface, setPlaybackActiveSurface };
}

describe("OperatorSurface active-surface scoping (CR-01)", () => {
  afterEach(() => {
    cleanup();
    delete (window as unknown as { go?: unknown }).go;
    vi.restoreAllMocks();
    // Zustand's useGolcStore is a module-level singleton -- reset it so
    // this test's connectionStatus override doesn't leak into other test
    // files sharing the same Vitest worker.
    useGolcStore.setState({ connectionStatus: "connecting" });
  });

  it("clears the active surface on both services when unmounted while in operate mode", async () => {
    const { setSafetyActiveSurface, setPlaybackActiveSurface } = installMockBridge();

    // OperatorSurface.tsx gates its own loading state on the shared store's
    // connectionStatus (normally flipped by LiveStatusBar's own fetch
    // effect, not mounted in this focused test) -- set it directly so the
    // component's skeleton resolves.
    useGolcStore.getState().setConnectionStatus("connected");

    // OperatorSurface's "operate" mode renders Launcher, which reads
    // usePlaybackSnapshot() -- a real dependency on being mounted inside
    // shell/PlaybackSnapshotContext's provider (normally supplied by
    // AppShell). Wrap it here too, matching production composition.
    const { unmount } = render(
      <PlaybackSnapshotProvider>
        <OperatorSurface />
      </PlaybackSnapshotProvider>,
    );

    // Select the surface, then enter "operate" mode -- mirrors the exact
    // user action that triggers the CR-01 effect.
    const surfaceRow = await screen.findByText(SURFACE_NAME);
    fireEvent.click(surfaceRow);

    const previewButton = await screen.findByRole("button", { name: "Preview as Operator" });
    fireEvent.click(previewButton);

    await waitFor(() => {
      expect(setSafetyActiveSurface).toHaveBeenCalledWith(SURFACE_NAME);
      expect(setPlaybackActiveSurface).toHaveBeenCalledWith(SURFACE_NAME);
    });

    // This is the behavior under test: WorkspaceRouter's switch unmounts
    // the previous workspace on every navigate-away. Simulate that here.
    unmount();

    await waitFor(() => {
      expect(setSafetyActiveSurface).toHaveBeenLastCalledWith("");
      expect(setPlaybackActiveSurface).toHaveBeenLastCalledWith("");
    });
  });
});
