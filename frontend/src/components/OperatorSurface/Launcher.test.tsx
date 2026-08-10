import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "../../test/renderWithQuery";

import Launcher from "./Launcher";
import type { ControlRefView } from "./OperatorSurface";
import { PlaybackSnapshotProvider } from "../../shell/PlaybackSnapshotContext";
import { useGolcStore } from "../../store/store";

function stubPlaybackState(overrides: {
  switchScene?: ReturnType<typeof vi.fn>;
  setLayerEnabled?: ReturnType<typeof vi.fn>;
} = {}) {
  vi.stubGlobal("go", {
    wails: {
      PlaybackService: {
        GetState: vi.fn().mockResolvedValue({
          exitCode: 0,
          stdout: JSON.stringify({
            bpm: 120,
            scenes: [
              {
                name: "Alpha",
                active: true,
                barsPerLoop: 4,
                layers: [
                  { kind: "base_look", enabled: true, ref: "" },
                  { kind: "color_theme", enabled: false, ref: "" },
                  { kind: "chase", enabled: false, ref: "" },
                  { kind: "motion", enabled: false, ref: "" },
                ],
              },
              { name: "Beta", active: false, barsPerLoop: 4, layers: [] },
            ],
          }),
          stderr: "",
        }),
        SwitchScene: overrides.switchScene ?? vi.fn().mockResolvedValue({ exitCode: 0, stdout: "", stderr: "" }),
        SetLayerEnabled:
          overrides.setLayerEnabled ?? vi.fn().mockResolvedValue({ exitCode: 0, stdout: "", stderr: "" }),
      },
    },
  });
}

async function renderLauncher(controls: ControlRefView[]) {
  const utils = render(
    <PlaybackSnapshotProvider>
      <Launcher controls={controls} />
    </PlaybackSnapshotProvider>,
  );
  // Gate on the ACTIVE SCENE NAME, not the bare "Layers" label: the label
  // renders unconditionally (Launcher.tsx appends " — {name}" only once a
  // snapshot has arrived), so waiting on /Layers/ alone returned before the
  // playback state had loaded and left the assertions below racing the
  // fetch. Every test here stubs Alpha as the active scene.
  await waitFor(() => expect(screen.getByText(/Layers — Alpha/)).toBeInTheDocument());
  return utils;
}

describe("Launcher", () => {
  afterEach(() => {
    cleanup();
    useGolcStore.getState().setConnectionStatus("connecting");
    vi.unstubAllGlobals();
  });

  it("shows an empty state when no scenes are assigned to this surface", async () => {
    stubPlaybackState();
    useGolcStore.getState().setConnectionStatus("connected");
    await renderLauncher([]);
    expect(screen.getByText("No scenes assigned to this surface yet.")).toBeInTheDocument();
  });

  it("launches an assigned scene and shows LIVE on the active pad", async () => {
    const switchScene = vi.fn().mockResolvedValue({ exitCode: 0, stdout: "", stderr: "" });
    stubPlaybackState({ switchScene });
    useGolcStore.getState().setConnectionStatus("connected");

    const controls: ControlRefView[] = [
      { kind: "scene", scene: "Alpha", label: "Alpha", assigned: true },
      { kind: "scene", scene: "Beta", label: "Beta", assigned: true },
    ];
    await renderLauncher(controls);

    expect(screen.getByRole("button", { name: "AlphaLIVE" })).toHaveAttribute("aria-current", "true");

    fireEvent.click(screen.getByRole("button", { name: "Beta" }));
    await waitFor(() => expect(switchScene).toHaveBeenCalledWith("Beta"));
  });

  it("renders an unassigned scene as locked and never dispatches on click", async () => {
    stubPlaybackState();
    useGolcStore.getState().setConnectionStatus("connected");

    const controls: ControlRefView[] = [{ kind: "scene", scene: "Beta", label: "Beta", assigned: false }];
    await renderLauncher(controls);

    const pad = screen.getByRole("button", { name: "BetaLocked" });
    expect(pad).toBeDisabled();
  });

  it("toggles a layer on the active scene via the layer strip", async () => {
    const setLayerEnabled = vi.fn().mockResolvedValue({ exitCode: 0, stdout: "", stderr: "" });
    stubPlaybackState({ setLayerEnabled });
    useGolcStore.getState().setConnectionStatus("connected");

    await renderLauncher([]);

    fireEvent.click(screen.getByRole("button", { name: "Base Look" }));
    await waitFor(() => expect(setLayerEnabled).toHaveBeenCalledWith("Alpha", "base_look", false));
  });

  it("shows assigned masters as non-interactive chips", async () => {
    stubPlaybackState();
    useGolcStore.getState().setConnectionStatus("connected");

    const controls: ControlRefView[] = [
      { kind: "master", masterKind: "grand", label: "Grand Master", assigned: true },
    ];
    await renderLauncher(controls);

    expect(screen.getByText("Grand Master")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Grand Master" })).not.toBeInTheDocument();
  });
});
