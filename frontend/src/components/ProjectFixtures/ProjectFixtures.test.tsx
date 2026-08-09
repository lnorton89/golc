// ProjectFixtures.test.tsx is the focused regression suite for the
// project-fixtures surface's structural-edit safety contract (13-10 Task
// 2): preview before apply and destructive/reassign flows stay intact
// through the design-system migration. Mocks window.go.wails directly,
// mirroring every other Wails-bridge test in this codebase (see
// wailsBridge.ts's own doc comment).
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import ProjectFixtures from "./ProjectFixtures";

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

describe("ProjectFixtures", () => {
  beforeEach(() => {
    vi.stubGlobal("go", {
      wails: {
        FixturePatchService: {
          ListPatch: vi.fn().mockResolvedValue(patchView()),
          CreatePool: vi.fn().mockResolvedValue(ok()),
          CreateDeployment: vi.fn().mockResolvedValue(ok()),
          ActivateDeployment: vi.fn().mockResolvedValue(ok()),
          AddPoolMembersPreview: vi.fn().mockResolvedValue(
            ok(
              JSON.stringify({
                schema_version: 1,
                pool_id: "pool-1",
                operations: [
                  {
                    dependent_kind: "deployment_instance",
                    dependent_ref: "Default / Acme PAR64",
                    dependent_id: "inst-1",
                    action: "add",
                    proposed_universe: 1,
                    proposed_address: 1,
                    status: "pending",
                  },
                ],
                plan_id: "plan-xyz987654321",
              }),
            ),
          ),
          ApplyPatch: vi.fn().mockResolvedValue(ok()),
          RemovePoolMemberPreview: vi.fn(),
          RenamePool: vi.fn(),
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

  it("shows an empty state when the project has no patched fixtures", async () => {
    render(<ProjectFixtures />);
    await waitFor(() => expect(screen.getByText("No fixtures added yet")).toBeInTheDocument());
  });

  it("lists an existing patched instance with its metadata tags", async () => {
    const svc = (window as unknown as { go: { wails: { FixturePatchService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.FixturePatchService;
    svc.ListPatch.mockResolvedValue(
      patchView({
        pools: [{ id: "pool-1", name: "Acme PAR64", members: [{ id: "member-1", fixtureStableKey: "acme-par64", fixtureContentHash: "hash-1" }] }],
        deployments: [
          {
            id: "dep-1",
            name: "Default",
            active: true,
            instances: [{ id: "inst-1", poolId: "pool-1", poolMemberId: "member-1", mode: "4ch", universe: 1, address: 1 }],
          },
        ],
      }),
    );

    render(<ProjectFixtures />);
    await waitFor(() => expect(screen.getByText("Acme PAR64")).toBeInTheDocument());
    expect(screen.getByText("1 fixture in this project")).toBeInTheDocument();
    expect(screen.getByText("4ch")).toBeInTheDocument();
    expect(screen.getByText("Default")).toBeInTheDocument();
  });

  it("opens the add-from-library dialog and previews+applies adding a fixture", async () => {
    const svc = (window as unknown as { go: { wails: { FixturePatchService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.FixturePatchService;
    // An already-active deployment short-circuits handleReviewImpact's
    // create/activate-"Default" bootstrap branch, keeping this test
    // focused on the preview/apply flow rather than that separate path.
    // The pool itself does not exist yet, so the second refreshPatch()
    // (after CreatePool) must reflect it existing, matching the real
    // backend's create-then-relist sequencing.
    const activeDeployment = { id: "dep-1", name: "Default", active: true, instances: [] };
    svc.ListPatch
      .mockResolvedValueOnce(patchView({ deployments: [activeDeployment] }))
      .mockResolvedValue(
        patchView({
          deployments: [activeDeployment],
          pools: [{ id: "pool-1", name: "Acme PAR64", members: [] }],
        }),
      );

    render(<ProjectFixtures />);
    await waitFor(() => expect(screen.getByText("No fixtures added yet")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Add from Library" }));
    expect(screen.getByRole("dialog", { name: "Add from Library" })).toBeInTheDocument();

    await chooseComboboxOption("Fixture", "Acme PAR64");
    await chooseComboboxOption("Fixture mode", "4ch");
    fireEvent.click(screen.getByRole("button", { name: "Review Impact" }));

    await waitFor(() => expect(svc.AddPoolMembersPreview).toHaveBeenCalled());
    await waitFor(() => expect(screen.getByText(/Default \/ Acme PAR64/)).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Apply" }));
    await waitFor(() => expect(svc.ApplyPatch).toHaveBeenCalledWith("plan-xyz987654321"));
  });

  it("closes the add-from-library dialog on Escape", async () => {
    render(<ProjectFixtures />);
    await waitFor(() => expect(screen.getByText("No fixtures added yet")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Add from Library" }));
    expect(screen.getByRole("dialog", { name: "Add from Library" })).toBeInTheDocument();

    fireEvent.keyDown(screen.getByRole("dialog", { name: "Add from Library" }), { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("reassigns an existing instance's mode/universe/address", async () => {
    const svc = (window as unknown as { go: { wails: { FixturePatchService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.FixturePatchService;
    svc.ListPatch.mockResolvedValue(
      patchView({
        pools: [{ id: "pool-1", name: "Acme PAR64", members: [{ id: "member-1", fixtureStableKey: "acme-par64", fixtureContentHash: "hash-1" }] }],
        deployments: [
          {
            id: "dep-1",
            name: "Default",
            active: true,
            instances: [{ id: "inst-1", poolId: "pool-1", poolMemberId: "member-1", mode: "4ch", universe: 1, address: 1 }],
          },
        ],
      }),
    );

    render(<ProjectFixtures />);
    await waitFor(() => expect(screen.getByText("Acme PAR64")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Edit Acme PAR64" }));
    fireEvent.change(screen.getByLabelText("Universe"), { target: { value: "2" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(svc.ReassignInstance).toHaveBeenCalledWith("Default", "inst-1", "4ch", 2, 1));
  });
});
