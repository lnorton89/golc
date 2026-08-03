// ScenesLooksWorkspace.test.tsx exercises the orchestrator end-to-end
// against a mocked window.go.wails.ProgrammingService, the same
// direct-window-object convention every other Wails-bridge test in this
// codebase uses (see wailsBridge.ts's own doc comment).
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

import ScenesLooksWorkspace from "./ScenesLooksWorkspace";

function view(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    scenes: [
      {
        name: "Alpha",
        active: true,
        barsPerLoop: 4,
        layers: [{ kind: "base_look", enabled: true, ref: "preset-1" }],
      },
      { name: "Beta", active: false, barsPerLoop: 8, layers: [] },
    ],
    themes: [],
    presets: [{ id: "preset-1", name: "Full Wash", kind: "intensity" }],
    chases: [],
    motions: [],
    blends: [],
    instances: [],
    ...overrides,
  };
}

function ok(stdout = "") {
  return { exitCode: 0, stdout, stderr: "" };
}

describe("ScenesLooksWorkspace", () => {
  beforeEach(() => {
    vi.stubGlobal("go", {
      wails: {
        ProgrammingService: {
          ListProgramming: vi.fn().mockResolvedValue(view()),
          CreateScene: vi.fn().mockResolvedValue(ok()),
          ActivateScene: vi.fn().mockResolvedValue(ok()),
          SetSceneLayer: vi.fn().mockResolvedValue(ok()),
          CreateTheme: vi.fn().mockResolvedValue(ok()),
          CreateMotion: vi.fn().mockResolvedValue(ok()),
          CreateChase: vi.fn().mockResolvedValue(ok()),
          CreateBlend: vi.fn().mockResolvedValue(ok()),
          ProgrammerSet: vi.fn().mockResolvedValue(ok()),
          RecordPreset: vi.fn().mockResolvedValue(ok()),
        },
      },
    });
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("loads and displays scenes, defaulting the selection to the active scene", async () => {
    render(<ScenesLooksWorkspace />);
    await waitFor(() => expect(screen.getByLabelText("Alpha layers")).toBeInTheDocument());
    expect(screen.getByLabelText("Scene stack")).toBeInTheDocument();
    expect(screen.getAllByText("LIVE")).toHaveLength(2);
  });

  it("switches the displayed layers when a different scene is selected", async () => {
    render(<ScenesLooksWorkspace />);
    await waitFor(() => expect(screen.getByLabelText("Alpha layers")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Beta8bar" }));
    await waitFor(() => expect(screen.getByLabelText("Beta layers")).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Activate" })).toBeInTheDocument();
  });

  it("activates a non-live scene via the Activate button", async () => {
    const svc = (window as unknown as { go: { wails: { ProgrammingService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.ProgrammingService;
    render(<ScenesLooksWorkspace />);
    await waitFor(() => expect(screen.getByLabelText("Alpha layers")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Beta8bar" }));
    await waitFor(() => screen.getByRole("button", { name: "Activate" }));
    fireEvent.click(screen.getByRole("button", { name: "Activate" }));

    await waitFor(() => expect(svc.ActivateScene).toHaveBeenCalledWith("Beta"));
  });

  it("toggles a layer's enabled state on the selected scene", async () => {
    const svc = (window as unknown as { go: { wails: { ProgrammingService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.ProgrammingService;
    render(<ScenesLooksWorkspace />);
    await waitFor(() => expect(screen.getByLabelText("Alpha layers")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Base Look" }));
    await waitFor(() => expect(svc.SetSceneLayer).toHaveBeenCalledWith("Alpha", "base_look", "", false));
  });

  it("creates a new scene from the SceneList form and selects it", async () => {
    const svc = (window as unknown as { go: { wails: { ProgrammingService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.ProgrammingService;
    svc.ListProgramming
      .mockResolvedValueOnce(view())
      .mockResolvedValueOnce(
        view({
          scenes: [
            ...view().scenes,
            { name: "Gamma", active: false, barsPerLoop: 4, layers: [] },
          ],
        }),
      );

    render(<ScenesLooksWorkspace />);
    await waitFor(() => expect(screen.getByLabelText("Alpha layers")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "New" }));
    fireEvent.change(screen.getByLabelText("New scene name"), { target: { value: "Gamma" } });
    fireEvent.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => expect(svc.CreateScene).toHaveBeenCalledWith("Gamma", 4));
    await waitFor(() => expect(screen.getByLabelText("Gamma layers")).toBeInTheDocument());
  });

  it("shows an empty state when there are no scenes yet", async () => {
    const svc = (window as unknown as { go: { wails: { ProgrammingService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.ProgrammingService;
    svc.ListProgramming.mockResolvedValue(view({ scenes: [] }));

    render(<ScenesLooksWorkspace />);
    await waitFor(() =>
      expect(
        screen.getByText("Create a scene to start pointing its layers at reusable looks."),
      ).toBeInTheDocument(),
    );
  });
});
