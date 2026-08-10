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

  // --- 2026-08-10 review pass regressions ------------------------------

  function patchService() {
    return (window as unknown as { go: { wails: { FixturePatchService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.FixturePatchService;
  }

  /** deferred lets a test choose the order two in-flight reads resolve. */
  function deferred<T>() {
    let resolve!: (value: T) => void;
    const promise = new Promise<T>((r) => {
      resolve = r;
    });
    return { promise, resolve };
  }

  it("discards a reviewed add-preview when the fixture is changed underneath it", async () => {
    const svc = patchService();
    svc.ListPatch.mockResolvedValue(patchView({ pools: [{ id: "pool-1", name: "Wash", members: [] }] }));
    svc.ListLocal = svc.ListLocal ?? vi.fn();
    (window as unknown as { go: { wails: { FixtureLibraryService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.FixtureLibraryService.ListLocal.mockResolvedValue({
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
          {
            stableKey: "acme-mover",
            contentHash: "hash-2",
            manufacturer: "Acme",
            model: "Mover",
            modes: ["32ch"],
            modeChannelCounts: { "32ch": 32 },
            modeChannels: { "32ch": [] },
            fileName: "acme-mover.yaml",
            source: "local",
            status: "valid",
            detail: "",
          },
        ],
      });

    render(<FixturePatch />);
    await waitFor(() => expect(screen.getByText("Wash")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Add Fixture" }));
    await chooseComboboxOption("Fixture", "Acme PAR64");
    await chooseComboboxOption("Fixture mode", "4ch");
    fireEvent.click(screen.getByRole("button", { name: "Review Impact" }));
    await waitFor(() => expect(screen.getByText(/Universe 1, Address 5/)).toBeInTheDocument());

    // Switching to a different fixture must retire the reviewed plan --
    // Apply used to still commit the FIRST fixture's plan_id.
    await chooseComboboxOption("Fixture", "Acme Mover");

    expect(screen.queryByText(/Universe 1, Address 5/)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Apply" })).not.toBeInTheDocument();
  });

  it("renders the newer member's removal impact when the older preview resolves last", async () => {
    const svc = patchService();
    svc.ListPatch.mockResolvedValue(
      patchView({
        pools: [
          {
            id: "pool-1",
            name: "Wash",
            members: [
              { id: "member-a", fixtureStableKey: "acme-par64" },
              { id: "member-b", fixtureStableKey: "acme-mover" },
            ],
          },
        ],
      }),
    );

    const removePlan = (memberId: string, ref: string) =>
      ok(
        JSON.stringify({
          schema_version: 1,
          pool_id: "pool-1",
          propagate: "none",
          expected_revision: 1,
          operations: [
            {
              dependent_kind: "deployment_instance",
              dependent_ref: ref,
              dependent_id: "inst-1",
              action: "remove",
              pool_member_index: 0,
              pool_member_id: memberId,
              status: "pending",
            },
          ],
          plan_id: `plan-${memberId}`,
        }),
      );

    const first = deferred<ReturnType<typeof ok>>();
    const second = deferred<ReturnType<typeof ok>>();
    svc.RemovePoolMemberPreview.mockImplementation((_pool: string, memberId: string) =>
      memberId === "member-a" ? first.promise : second.promise,
    );

    render(<FixturePatch />);
    await waitFor(() => expect(screen.getByText("Wash")).toBeInTheDocument());

    const removeButtons = screen.getAllByRole("button", { name: "Remove" });
    fireEvent.click(removeButtons[0]);
    await waitFor(() => expect(svc.RemovePoolMemberPreview).toHaveBeenCalledWith("Wash", "member-a"));
    fireEvent.click(screen.getAllByRole("button", { name: "Remove" })[1]);
    await waitFor(() => expect(svc.RemovePoolMemberPreview).toHaveBeenCalledWith("Wash", "member-b"));

    // B lands first; the slower A preview arrives afterwards.
    second.resolve(removePlan("member-b", "Deployment A / B instance"));
    await waitFor(() => expect(screen.getByText(/B instance/)).toBeInTheDocument());

    first.resolve(removePlan("member-a", "Deployment A / A instance"));
    await waitFor(() => expect(svc.RemovePoolMemberPreview).toHaveBeenCalledTimes(2));

    // A's plan must neither render under B's row nor be what Apply commits.
    expect(screen.queryByText(/A instance/)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Apply" }));
    await waitFor(() => expect(svc.ApplyPatch).toHaveBeenCalledWith("plan-member-b"));
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
