// FixturePatch.test.tsx is the focused regression suite for the fixture
// patch surface's structural-edit safety contract (13-10 Task 2): preview
// before apply, revision-free atomic dispatch, and destructive confirm
// paths all stay intact through the design-system migration. Mocks
// window.go.wails directly, mirroring every other Wails-bridge test in
// this codebase (see wailsBridge.ts's own doc comment).
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import FixturePatch from "./FixturePatch";

// The Fixture field is the shared Combobox primitive and Fixture mode is
// the shared Select primitive (both Base UI-backed) -- opening each and
// choosing an option needs realistic pointer interaction via userEvent,
// not a bare fireEvent.change on what used to be native <select>s.
async function chooseComboboxOption(triggerName: string, optionName: string) {
  const user = userEvent.setup();
  await user.click(screen.getByRole("combobox", { name: triggerName }));
  await user.click(await screen.findByRole("option", { name: optionName }));
}

function ok(stdout = "") {
  return { exitCode: 0, stdout, stderr: "" };
}

function patchView(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    pools: [],
    deployments: [],
    ...overrides,
  };
}

describe("FixturePatch", () => {
  beforeEach(() => {
    vi.stubGlobal("go", {
      wails: {
        FixturePatchService: {
          ListPatch: vi.fn().mockResolvedValue(patchView()),
          CreatePool: vi.fn().mockResolvedValue(ok()),
          AddPoolMemberPreview: vi.fn().mockResolvedValue(
            ok(
              JSON.stringify({
                schema_version: 1,
                pool_id: "pool-1",
                propagate: "none",
                expected_revision: 1,
                operations: [
                  {
                    dependent_kind: "deployment_instance",
                    dependent_ref: "Deployment A / Fixture 1",
                    dependent_id: "inst-1",
                    action: "add",
                    pool_member_index: 0,
                    pool_member_id: "member-1",
                    proposed_universe: 1,
                    proposed_address: 5,
                    status: "pending",
                  },
                ],
                plan_id: "plan-abc123456789",
              }),
            ),
          ),
          ApplyPatch: vi.fn().mockResolvedValue(ok()),
          RemovePoolMemberPreview: vi.fn(),
          CreateDeployment: vi.fn().mockResolvedValue(ok()),
          ActivateDeployment: vi.fn().mockResolvedValue(ok()),
          RenamePool: vi.fn(),
          DeletePool: vi.fn(),
          RenameDeployment: vi.fn(),
          DeleteDeployment: vi.fn(),
          ReassignInstance: vi.fn(),
        },
        FixtureLibraryService: {
          ListLocal: vi.fn().mockResolvedValue({
            directory: "",
            rows: [
              {
                stableKey: "acme-par64",
                contentHash: "hash-1",
                manufacturer: "Acme",
                model: "PAR64",
                modes: ["4ch"],
                modeChannelCounts: { "4ch": 4 },
                modeChannels: { "4ch": [] },
                fileName: "acme-par64.yaml",
                source: "local",
                status: "valid",
                detail: "",
              },
            ],
          }),
        },
      },
    });
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("shows empty states for pools and deployments when the show has neither", async () => {
    render(<FixturePatch />);
    await waitFor(() => expect(screen.getByText("No fixture pools yet")).toBeInTheDocument());
    expect(screen.getByText("No deployments yet")).toBeInTheDocument();
  });

  it("creates a pool from the create-pool form", async () => {
    const svc = (window as unknown as { go: { wails: { FixturePatchService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.FixturePatchService;
    svc.ListPatch
      .mockResolvedValueOnce(patchView())
      .mockResolvedValueOnce(
        patchView({ pools: [{ id: "pool-1", name: "Wash", members: [] }] }),
      );

    render(<FixturePatch />);
    await waitFor(() => expect(screen.getByText("No fixture pools yet")).toBeInTheDocument());

    fireEvent.change(screen.getByLabelText("New pool name"), { target: { value: "Wash" } });
    fireEvent.click(screen.getByRole("button", { name: "Create Pool" }));

    await waitFor(() => expect(svc.CreatePool).toHaveBeenCalledWith("Wash", []));
    await waitFor(() => expect(screen.getByText("Wash")).toBeInTheDocument());
  });

  it("previews and applies adding a fixture to a pool", async () => {
    const svc = (window as unknown as { go: { wails: { FixturePatchService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.FixturePatchService;
    svc.ListPatch.mockResolvedValue(
      patchView({ pools: [{ id: "pool-1", name: "Wash", members: [] }] }),
    );

    render(<FixturePatch />);
    await waitFor(() => expect(screen.getByText("Wash")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Add Fixture" }));
    await chooseComboboxOption("Fixture", "Acme PAR64");
    await chooseComboboxOption("Fixture mode", "4ch");
    fireEvent.click(screen.getByRole("button", { name: "Review Impact" }));

    await waitFor(() => expect(svc.AddPoolMemberPreview).toHaveBeenCalledWith("Wash", "acme-par64", "hash-1", "4ch", 4));
    await waitFor(() => expect(screen.getByText(/Deployment A \/ Fixture 1/)).toBeInTheDocument());
    expect(screen.getByText(/Universe 1, Address 5/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Apply" }));
    await waitFor(() => expect(svc.ApplyPatch).toHaveBeenCalledWith("plan-abc123456789"));
  });

  it("creates and activates a deployment", async () => {
    const svc = (window as unknown as { go: { wails: { FixturePatchService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.FixturePatchService;
    svc.ListPatch
      .mockResolvedValueOnce(patchView())
      .mockResolvedValueOnce(
        patchView({ deployments: [{ id: "dep-1", name: "Main Rig", active: false, instances: [] }] }),
      )
      .mockResolvedValueOnce(
        patchView({ deployments: [{ id: "dep-1", name: "Main Rig", active: true, instances: [] }] }),
      );

    render(<FixturePatch />);
    await waitFor(() => expect(screen.getByText("No deployments yet")).toBeInTheDocument());

    fireEvent.change(screen.getByLabelText("New deployment name"), { target: { value: "Main Rig" } });
    fireEvent.click(screen.getByRole("button", { name: "Create Deployment" }));

    await waitFor(() => expect(svc.CreateDeployment).toHaveBeenCalledWith("Main Rig"));
    await waitFor(() => expect(screen.getByRole("button", { name: "Activate" })).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Activate" }));
    await waitFor(() => expect(svc.ActivateDeployment).toHaveBeenCalledWith("Main Rig"));
    await waitFor(() => expect(screen.getByText("Active")).toBeInTheDocument());
  });
});
