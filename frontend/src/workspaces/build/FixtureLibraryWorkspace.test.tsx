// FixtureLibraryWorkspace.test.tsx exercises the real browse + inline-
// inspect workspace (09-01-PLAN.md Task 1 RED / Task 2-3 GREEN) against a
// mocked window.go.wails.FixtureLibraryService -- the same direct-
// window-object mock convention OperatorSurface.activeSurface.test.tsx
// already uses (assign in beforeEach, delete in afterEach), following
// this plan's own explicit instruction. These assertions describe the
// real workspace; the previous "Coming Soon" stub assertions this file
// used to carry are gone.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";

import FixtureLibraryWorkspace from "./FixtureLibraryWorkspace";

interface MockRow {
  stableKey: string;
  manufacturer: string;
  model: string;
  fileName: string;
  source: string;
  status: string;
  detail: string;
}

function row(overrides: Partial<MockRow> = {}): MockRow {
  return {
    stableKey: "Chauvet/SlimPAR Pro",
    manufacturer: "Chauvet",
    model: "SlimPAR Pro",
    fileName: "slimpar.yaml",
    source: "local",
    status: "valid",
    detail: "",
    ...overrides,
  };
}

const secondRow = row({
  stableKey: "American DJ/Mega Par",
  manufacturer: "American DJ",
  model: "Mega Par",
  fileName: "megapar.yaml",
});

function installMockFixtureLibraryService(overrides: Partial<Record<string, ReturnType<typeof vi.fn>>> = {}) {
  const svc = {
    ListLocal: vi.fn().mockResolvedValue({ directory: "fixtures", rows: [] }),
    Inspect: vi.fn().mockResolvedValue({
      path: "",
      valid: true,
      errors: [],
      schemaVersion: 1,
      stableKey: "",
      contentHash: "",
      revision: "",
      source: "",
      validationResult: "valid",
      warnings: [],
    }),
    ...overrides,
  };
  (window as unknown as { go: unknown }).go = { wails: { FixtureLibraryService: svc } };
  return svc;
}

function fixtureList(): HTMLElement {
  return screen.getByRole("list", { name: "Fixture library" });
}

describe("FixtureLibraryWorkspace", () => {
  beforeEach(() => {
    // Default: no bridge present, so a test that doesn't need one still
    // exercises the "no fixtures yet" path cleanly rather than an
    // undefined window.go.
  });

  afterEach(() => {
    cleanup();
    delete (window as unknown as { go?: unknown }).go;
  });

  it("renders local library rows with manufacturer and validation chip", async () => {
    installMockFixtureLibraryService({
      ListLocal: vi.fn().mockResolvedValue({ directory: "fixtures", rows: [row(), secondRow] }),
    });

    render(<FixtureLibraryWorkspace />);

    await waitFor(() => expect(within(fixtureList()).getByText("SlimPAR Pro")).toBeInTheDocument());
    expect(within(fixtureList()).getByText("Mega Par")).toBeInTheDocument();
    expect(within(fixtureList()).getByText("Chauvet")).toBeInTheDocument();
    expect(within(fixtureList()).getByText("American DJ")).toBeInTheDocument();
  });

  it("renders the no-fixtures empty state", async () => {
    installMockFixtureLibraryService();

    render(<FixtureLibraryWorkspace />);

    await waitFor(() => expect(screen.getByText("No fixtures yet")).toBeInTheDocument());
    expect(
      screen.getByText(
        "Import a fixture from the Open Fixture Library or add your own YAML definition to get started.",
      ),
    ).toBeInTheDocument();
  });

  it("filters rows by search text, case-insensitively, matching manufacturer or model", async () => {
    installMockFixtureLibraryService({
      ListLocal: vi.fn().mockResolvedValue({ directory: "fixtures", rows: [row(), secondRow] }),
    });

    render(<FixtureLibraryWorkspace />);
    await waitFor(() => expect(within(fixtureList()).getByText("SlimPAR Pro")).toBeInTheDocument());

    const search = screen.getByLabelText("Search fixtures");
    fireEvent.change(search, { target: { value: "chauvet" } });

    expect(within(fixtureList()).getByText("SlimPAR Pro")).toBeInTheDocument();
    expect(within(fixtureList()).queryByText("Mega Par")).not.toBeInTheDocument();

    fireEvent.change(search, { target: { value: "mega" } });
    expect(within(fixtureList()).getByText("Mega Par")).toBeInTheDocument();
    expect(within(fixtureList()).queryByText("SlimPAR Pro")).not.toBeInTheDocument();
  });

  it("renders inline inspect detail for the selected row, not a dialog", async () => {
    const svc = installMockFixtureLibraryService({
      ListLocal: vi.fn().mockResolvedValue({ directory: "fixtures", rows: [row()] }),
      Inspect: vi.fn().mockResolvedValue({
        path: "slimpar.yaml",
        valid: true,
        errors: [],
        schemaVersion: 1,
        stableKey: "Chauvet/SlimPAR Pro",
        contentHash: "abc123content",
        revision: "abc123conten",
        source: "fixtures/slimpar.yaml",
        validationResult: "valid",
        warnings: [],
      }),
    });

    render(<FixtureLibraryWorkspace />);
    await waitFor(() => expect(within(fixtureList()).getByText("SlimPAR Pro")).toBeInTheDocument());

    fireEvent.click(within(fixtureList()).getByText("SlimPAR Pro"));

    await waitFor(() => expect(svc.Inspect).toHaveBeenCalled());
    await waitFor(() => expect(screen.getByText(/Chauvet\/SlimPAR Pro/)).toBeInTheDocument());
    expect(screen.getByText(/abc123content/)).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("renders the validation-failure copy when the selected row's inspect result is invalid", async () => {
    installMockFixtureLibraryService({
      ListLocal: vi.fn().mockResolvedValue({ directory: "fixtures", rows: [row({ status: "invalid" })] }),
      Inspect: vi.fn().mockResolvedValue({
        path: "slimpar.yaml",
        valid: false,
        errors: ["GOLC_FIXTURE_MODEL_EMPTY: fixture model must not be empty", "GOLC_FIXTURE_EMPTY: fixture document is empty"],
        schemaVersion: 0,
        stableKey: "",
        contentHash: "",
        revision: "",
        source: "",
        validationResult: "",
        warnings: [],
      }),
    });

    render(<FixtureLibraryWorkspace />);
    await waitFor(() => expect(within(fixtureList()).getByText("SlimPAR Pro")).toBeInTheDocument());
    fireEvent.click(within(fixtureList()).getByText("SlimPAR Pro"));

    await waitFor(() =>
      expect(
        screen.getByText("This fixture definition has 2 error(s) and can't be added. Fix them and try again."),
      ).toBeInTheDocument(),
    );
  });

  it("renders the lossy-import warning copy distinctly from the invalid state", async () => {
    installMockFixtureLibraryService({
      ListLocal: vi.fn().mockResolvedValue({ directory: "fixtures", rows: [row()] }),
      Inspect: vi.fn().mockResolvedValue({
        path: "slimpar.yaml",
        valid: true,
        errors: [],
        schemaVersion: 1,
        stableKey: "Chauvet/SlimPAR Pro",
        contentHash: "abc123content",
        revision: "abc123conten",
        source: "fixtures/slimpar.yaml",
        validationResult: "valid",
        warnings: [{ severity: "warning", capabilityType: "gobo", detail: "Gobo wheel approximated as a static color" }],
      }),
    });

    render(<FixtureLibraryWorkspace />);
    await waitFor(() => expect(within(fixtureList()).getByText("SlimPAR Pro")).toBeInTheDocument());
    fireEvent.click(within(fixtureList()).getByText("SlimPAR Pro"));

    await waitFor(() =>
      expect(
        screen.getByText("This import has 1 unsupported or approximated attribute(s) — review before adding."),
      ).toBeInTheDocument(),
    );
    expect(
      screen.queryByText(/error\(s\) and can't be added/),
    ).not.toBeInTheDocument();
  });

  it("stays visually quiet with no row selected", async () => {
    installMockFixtureLibraryService({
      ListLocal: vi.fn().mockResolvedValue({ directory: "fixtures", rows: [row()] }),
    });

    render(<FixtureLibraryWorkspace />);
    await waitFor(() => expect(within(fixtureList()).getByText("SlimPAR Pro")).toBeInTheDocument());

    expect(screen.queryByText(/error\(s\) and can't be added/)).not.toBeInTheDocument();
    expect(screen.queryByText(/unsupported or approximated attribute/)).not.toBeInTheDocument();
  });

  it("gives a long fixture/manufacturer name a title attribute carrying the full value", async () => {
    const longModel = "M".repeat(120);
    installMockFixtureLibraryService({
      ListLocal: vi.fn().mockResolvedValue({
        directory: "fixtures",
        rows: [row({ model: longModel, stableKey: `Chauvet/${longModel}` })],
      }),
    });

    render(<FixtureLibraryWorkspace />);
    await waitFor(() => expect(within(fixtureList()).getByText(longModel)).toBeInTheDocument());

    const rowElement = within(fixtureList()).getByText(longModel).closest("[title]");
    expect(rowElement).not.toBeNull();
    expect(rowElement).toHaveAttribute("title", longModel);
  });
});
