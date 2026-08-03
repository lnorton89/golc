// ArtnetConfig.test.tsx is the focused regression suite for the Art-Net
// configuration surface's dispatch/projection contract (13-26 Task 1):
// every ArtnetConfigService/App.SelectInterface call, the client-side
// port shape guard, and the daemon-unreachable/empty-state projections
// all stay intact through the design-system migration. Mocks
// window.go.wails directly, mirroring every other Wails-bridge test in
// this codebase (see wailsBridge.ts's own doc comment; mirrors
// FixturePatch.test.tsx's identical shape).
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

import ArtnetConfig from "./ArtnetConfig";

function ok(stdout = "") {
  return { exitCode: 0, stdout, stderr: "" };
}

function fail(stderr: string) {
  return { exitCode: 1, stdout: "", stderr };
}

function iface(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    index: 0,
    name: "Ethernet",
    up: true,
    addrs: ["10.0.0.5"],
    pinned: false,
    status: "reachable",
    error: "",
    ...overrides,
  };
}

function target(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    universe: 1,
    ip: "10.0.0.9",
    port: 6454,
    enabled: true,
    sendOk: 100,
    sendErr: 0,
    reachable: true,
    lastError: "",
    ...overrides,
  };
}

function status(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    reachable: true,
    interface: { pinnedIndex: 0, pinnedName: "Ethernet", status: "reachable", error: "" },
    targets: [],
    ...overrides,
  };
}

function patchView(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    pools: [],
    deployments: [
      {
        id: "dep-1",
        name: "Main",
        active: true,
        instances: [{ id: "inst-1", poolId: "pool-1", poolMemberId: "member-1", mode: "4ch", universe: 1, address: 1 }],
      },
    ],
    ...overrides,
  };
}

function stubBridge({
  interfaces = [iface()],
  artnetStatus = status(),
  patch = patchView(),
}: {
  interfaces?: ReturnType<typeof iface>[];
  artnetStatus?: ReturnType<typeof status>;
  patch?: ReturnType<typeof patchView>;
} = {}) {
  vi.stubGlobal("go", {
    wails: {
      ArtnetConfigService: {
        ListInterfaces: vi.fn().mockResolvedValue(interfaces),
        Configure: vi.fn().mockResolvedValue(ok()),
        EnableTarget: vi.fn().mockResolvedValue(ok()),
        DisableTarget: vi.fn().mockResolvedValue(ok()),
        FetchArtnetStatus: vi.fn().mockResolvedValue(artnetStatus),
      },
      FixturePatchService: {
        ListPatch: vi.fn().mockResolvedValue(patch),
      },
      App: {
        SelectInterface: vi.fn().mockResolvedValue(ok()),
      },
    },
  });
}

describe("ArtnetConfig", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("loads interfaces, status, and patched universes on mount", async () => {
    stubBridge();
    render(<ArtnetConfig />);

    await waitFor(() => expect(screen.getByText("Ethernet")).toBeInTheDocument());
    expect(screen.getByText("Universe 1")).toBeInTheDocument();
  });

  it("shows an empty state when no network interfaces are found", async () => {
    stubBridge({ interfaces: [] });
    render(<ArtnetConfig />);
    await waitFor(() => expect(screen.getByText("No network interfaces found")).toBeInTheDocument());
  });

  it("shows the offline chip and copy when the daemon is unreachable", async () => {
    stubBridge({ artnetStatus: status({ reachable: false }) });
    render(<ArtnetConfig />);
    await waitFor(() => expect(screen.getByText("Offline")).toBeInTheDocument());
    expect(screen.getByText(/Can.t reach the playback engine/)).toBeInTheDocument();
  });

  it("pins a different interface via App.SelectInterface and refreshes", async () => {
    stubBridge();
    render(<ArtnetConfig />);
    await waitFor(() => expect(screen.getByText("Ethernet")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Use" }));

    const bridge = (window as unknown as { go: { wails: { App: { SelectInterface: ReturnType<typeof vi.fn> }; ArtnetConfigService: { ListInterfaces: ReturnType<typeof vi.fn> } } } }).go.wails;
    await waitFor(() => expect(bridge.App.SelectInterface).toHaveBeenCalledWith(0, "Ethernet"));
    await waitFor(() => expect(bridge.ArtnetConfigService.ListInterfaces).toHaveBeenCalledTimes(2));
  });

  it("renders a pinned interface as an 'In use' chip instead of a Use button", async () => {
    stubBridge({ interfaces: [iface({ pinned: true })] });
    render(<ArtnetConfig />);
    await waitFor(() => expect(screen.getByText("In use")).toBeInTheDocument());
    expect(screen.queryByRole("button", { name: "Use" })).not.toBeInTheDocument();
  });

  it("configures a new target for a patched universe via ArtnetConfigService.Configure", async () => {
    stubBridge();
    render(<ArtnetConfig />);
    await waitFor(() => expect(screen.getByText("Universe 1")).toBeInTheDocument());

    fireEvent.change(screen.getByLabelText("Universe 1 target IP address"), { target: { value: "10.0.0.20" } });
    fireEvent.change(screen.getByLabelText("Universe 1 target port (optional)"), { target: { value: "6455" } });
    fireEvent.click(screen.getByRole("button", { name: /Add Target/ }));

    const svc = (window as unknown as { go: { wails: { ArtnetConfigService: { Configure: ReturnType<typeof vi.fn> } } } }).go.wails.ArtnetConfigService;
    await waitFor(() => expect(svc.Configure).toHaveBeenCalledWith(1, "10.0.0.20", 6455, true));
  });

  it("rejects an out-of-range port on screen without calling Configure", async () => {
    stubBridge();
    render(<ArtnetConfig />);
    await waitFor(() => expect(screen.getByText("Universe 1")).toBeInTheDocument());

    fireEvent.change(screen.getByLabelText("Universe 1 target IP address"), { target: { value: "10.0.0.20" } });
    fireEvent.change(screen.getByLabelText("Universe 1 target port (optional)"), { target: { value: "99999" } });
    fireEvent.click(screen.getByRole("button", { name: /Add Target/ }));

    await waitFor(() => expect(screen.getByText(/GOLC_ARTNET_USAGE/)).toBeInTheDocument());
    const svc = (window as unknown as { go: { wails: { ArtnetConfigService: { Configure: ReturnType<typeof vi.fn> } } } }).go.wails.ArtnetConfigService;
    expect(svc.Configure).not.toHaveBeenCalled();
  });

  it("toggles an existing target between enabled and disabled", async () => {
    stubBridge({ artnetStatus: status({ targets: [target({ enabled: true })] }) });
    render(<ArtnetConfig />);
    // getByRole("status", ...) rather than getByText -- the per-universe
    // draft form's own "Enabled" checkbox label renders the identical text.
    await waitFor(() => expect(screen.getByRole("status", { name: "Enabled" })).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /Disable/ }));

    const svc = (window as unknown as { go: { wails: { ArtnetConfigService: { DisableTarget: ReturnType<typeof vi.fn> } } } }).go.wails.ArtnetConfigService;
    await waitFor(() => expect(svc.DisableTarget).toHaveBeenCalledWith(1, "10.0.0.9", 6454));
  });

  it("surfaces a failed Configure call's own stderr diagnostic", async () => {
    stubBridge();
    const svc = (window as unknown as { go: { wails: { ArtnetConfigService: { Configure: ReturnType<typeof vi.fn> } } } }).go.wails.ArtnetConfigService;
    svc.Configure.mockResolvedValue(fail("GOLC_ARTNET_TARGET_INVALID: bad ip"));
    render(<ArtnetConfig />);
    await waitFor(() => expect(screen.getByText("Universe 1")).toBeInTheDocument());

    fireEvent.change(screen.getByLabelText("Universe 1 target IP address"), { target: { value: "bad" } });
    fireEvent.click(screen.getByRole("button", { name: /Add Target/ }));

    await waitFor(() => expect(screen.getByText(/GOLC_ARTNET_TARGET_INVALID/)).toBeInTheDocument());
  });
});
