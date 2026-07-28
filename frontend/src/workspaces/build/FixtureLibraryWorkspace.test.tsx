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
    SearchOFL: vi.fn().mockResolvedValue({ query: "", manufacturers: [], unreachable: false, detail: "" }),
    PreviewOFL: vi.fn().mockResolvedValue({
      inspect: {
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
      },
      previewToken: "",
      destinationExists: false,
      suggestedFileName: "",
    }),
    CommitPreview: vi.fn().mockResolvedValue({ exitCode: 0, stdout: "", stderr: "" }),
    DiscardPreview: vi.fn().mockResolvedValue({ exitCode: 0, stdout: "", stderr: "" }),
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

  describe("Open Fixture Library catalog search (09-05-PLAN.md Task 1 RED / Task 3 GREEN)", () => {
    function switchToCatalog() {
      fireEvent.click(screen.getByRole("button", { name: "Open Fixture Library" }));
    }

    it("the source toggle switches between the local list and the catalog", async () => {
      installMockFixtureLibraryService({
        ListLocal: vi.fn().mockResolvedValue({ directory: "fixtures", rows: [row()] }),
      });

      render(<FixtureLibraryWorkspace />);
      await waitFor(() => expect(within(fixtureList()).getByText("SlimPAR Pro")).toBeInTheDocument());

      expect(screen.getByRole("button", { name: "My Library" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Open Fixture Library" })).toBeInTheDocument();

      switchToCatalog();

      expect(screen.queryByText("SlimPAR Pro")).not.toBeInTheDocument();
      expect(screen.getByText("Search the Open Fixture Library by name or manufacturer.")).toBeInTheDocument();
    });

    it("renders the catalog empty prompt with no query", async () => {
      installMockFixtureLibraryService();
      render(<FixtureLibraryWorkspace />);
      await waitFor(() => expect(screen.getByText("No fixtures yet")).toBeInTheDocument());

      switchToCatalog();

      expect(screen.getByText("Search the Open Fixture Library by name or manufacturer.")).toBeInTheDocument();
    });

    it("renders the no-results copy with the query interpolated", async () => {
      const svc = installMockFixtureLibraryService({
        SearchOFL: vi.fn().mockResolvedValue({ query: "zzznomatch", manufacturers: [], unreachable: false, detail: "" }),
      });
      render(<FixtureLibraryWorkspace />);
      await waitFor(() => expect(screen.getByText("No fixtures yet")).toBeInTheDocument());

      switchToCatalog();
      const search = screen.getByLabelText("Search fixtures");
      fireEvent.change(search, { target: { value: "zzznomatch" } });

      await waitFor(() => expect(svc.SearchOFL).toHaveBeenCalled());
      await waitFor(() =>
        expect(
          screen.getByText('No fixtures matched "zzznomatch". Try a different name or manufacturer.'),
        ).toBeInTheDocument(),
      );
    });

    it("renders the unreachable copy with the offline tone", async () => {
      installMockFixtureLibraryService({
        SearchOFL: vi.fn().mockResolvedValue({
          query: "acme",
          manufacturers: [],
          unreachable: true,
          detail: "GOLC_FIXTURE_OFL_MANUFACTURERS_FETCH_FAILED: boom",
        }),
      });
      render(<FixtureLibraryWorkspace />);
      await waitFor(() => expect(screen.getByText("No fixtures yet")).toBeInTheDocument());

      switchToCatalog();
      const search = screen.getByLabelText("Search fixtures");
      fireEvent.change(search, { target: { value: "acme" } });

      await waitFor(() =>
        expect(
          screen.getByText("Can't reach the Open Fixture Library. Check your network connection and try again."),
        ).toBeInTheDocument(),
      );
    });

    it("renders manufacturer rows for a matching query", async () => {
      const svc = installMockFixtureLibraryService({
        SearchOFL: vi.fn().mockResolvedValue({
          query: "chauvet",
          manufacturers: [{ key: "chauvet-dj", name: "Chauvet DJ", website: "https://chauvetdj.example" }],
          unreachable: false,
          detail: "",
        }),
      });
      render(<FixtureLibraryWorkspace />);
      await waitFor(() => expect(screen.getByText("No fixtures yet")).toBeInTheDocument());

      switchToCatalog();
      const search = screen.getByLabelText("Search fixtures");
      fireEvent.change(search, { target: { value: "chauvet" } });

      await waitFor(() => expect(svc.SearchOFL).toHaveBeenCalled());
      await waitFor(() => expect(screen.getByText("Chauvet DJ")).toBeInTheDocument());
      expect(screen.getByText("chauvet-dj")).toBeInTheDocument();
    });
  });

  describe("Preview-then-commit import (09-06-PLAN.md Task 1 RED / Task 2-3 GREEN)", () => {
    function switchToCatalog() {
      fireEvent.click(screen.getByRole("button", { name: "Open Fixture Library" }));
    }

    function installMockWithOneManufacturer(overrides: Partial<Record<string, ReturnType<typeof vi.fn>>> = {}) {
      return installMockFixtureLibraryService({
        SearchOFL: vi.fn().mockResolvedValue({
          query: "chauvet",
          manufacturers: [{ key: "chauvet-dj", name: "Chauvet DJ", website: "" }],
          unreachable: false,
          detail: "",
        }),
        ...overrides,
      });
    }

    async function searchAndSelectManufacturer(svc: { SearchOFL: ReturnType<typeof vi.fn> }): Promise<void> {
      switchToCatalog();
      const search = screen.getByLabelText("Search fixtures");
      fireEvent.change(search, { target: { value: "chauvet" } });
      await waitFor(() => expect(svc.SearchOFL).toHaveBeenCalled());
      await waitFor(() => expect(screen.getByText("Chauvet DJ")).toBeInTheDocument());
      fireEvent.click(screen.getByText("Chauvet DJ"));
    }

    it("renders the candidate inspect panel before anything is committed", async () => {
      const svc = installMockWithOneManufacturer({
        PreviewOFL: vi.fn().mockResolvedValue({
          inspect: {
            path: "",
            valid: true,
            errors: [],
            schemaVersion: 1,
            stableKey: "chauvet-dj/led-par-64-tri-b",
            contentHash: "abc123content",
            revision: "abc123conten",
            source: "ofl:chauvet-dj/led-par-64-tri-b",
            validationResult: "valid",
            warnings: [],
          },
          previewToken: "/tmp/preview/chauvet-dj_led-par-64-tri-b.json",
          destinationExists: false,
          suggestedFileName: "chauvet-dj_led-par-64-tri-b.json",
        }),
      });

      render(<FixtureLibraryWorkspace />);
      await waitFor(() => expect(screen.getByText("No fixtures yet")).toBeInTheDocument());
      await searchAndSelectManufacturer(svc);

      fireEvent.change(screen.getByLabelText("Fixture key"), { target: { value: "led-par-64-tri-b" } });
      fireEvent.click(screen.getByRole("button", { name: "Preview" }));

      await waitFor(() => expect(svc.PreviewOFL).toHaveBeenCalledWith("chauvet-dj", "led-par-64-tri-b"));
      await waitFor(() => expect(screen.getByText(/chauvet-dj\/led-par-64-tri-b/)).toBeInTheDocument());
      expect(screen.getByText(/abc123content/)).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Add to Library" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Add to Library" })).toBeEnabled();
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("Add to Library is disabled for an invalid candidate", async () => {
      const svc = installMockWithOneManufacturer({
        PreviewOFL: vi.fn().mockResolvedValue({
          inspect: {
            path: "",
            valid: false,
            errors: ["GOLC_FIXTURE_OFL_FETCH_FAILED: unexpected HTTP status 404"],
            schemaVersion: 0,
            stableKey: "",
            contentHash: "",
            revision: "",
            source: "",
            validationResult: "",
            warnings: [],
          },
          previewToken: "",
          destinationExists: false,
          suggestedFileName: "",
        }),
      });

      render(<FixtureLibraryWorkspace />);
      await waitFor(() => expect(screen.getByText("No fixtures yet")).toBeInTheDocument());
      await searchAndSelectManufacturer(svc);

      fireEvent.change(screen.getByLabelText("Fixture key"), { target: { value: "does-not-exist" } });
      fireEvent.click(screen.getByRole("button", { name: "Preview" }));

      await waitFor(() =>
        expect(
          screen.getByText("This fixture definition has 1 error(s) and can't be added. Fix them and try again."),
        ).toBeInTheDocument(),
      );
      expect(screen.getByRole("button", { name: "Add to Library" })).toBeDisabled();
    });

    it("renders the lossy-import warning copy with the count and the per-attribute details, with Add to Library still enabled", async () => {
      const svc = installMockWithOneManufacturer({
        PreviewOFL: vi.fn().mockResolvedValue({
          inspect: {
            path: "",
            valid: true,
            errors: [],
            schemaVersion: 1,
            stableKey: "chauvet-dj/led-par-64-tri-b",
            contentHash: "abc123content",
            revision: "abc123conten",
            source: "ofl:chauvet-dj/led-par-64-tri-b",
            validationResult: "valid",
            warnings: [
              {
                severity: "warning",
                capabilityType: "",
                detail: 'channel "Effect Speed" capability type "EffectSpeed" is not represented in the v1 canonical model',
              },
            ],
          },
          previewToken: "/tmp/preview/chauvet-dj_led-par-64-tri-b.json",
          destinationExists: false,
          suggestedFileName: "chauvet-dj_led-par-64-tri-b.json",
        }),
      });

      render(<FixtureLibraryWorkspace />);
      await waitFor(() => expect(screen.getByText("No fixtures yet")).toBeInTheDocument());
      await searchAndSelectManufacturer(svc);

      fireEvent.change(screen.getByLabelText("Fixture key"), { target: { value: "led-par-64-tri-b" } });
      fireEvent.click(screen.getByRole("button", { name: "Preview" }));

      await waitFor(() =>
        expect(
          screen.getByText("This import has 1 unsupported or approximated attribute(s) — review before adding."),
        ).toBeInTheDocument(),
      );
      expect(screen.getByText(/EffectSpeed/)).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Add to Library" })).toBeEnabled();
    });

    it("an already-present fixture is reported, not silently replaced", async () => {
      const svc = installMockWithOneManufacturer({
        PreviewOFL: vi.fn().mockResolvedValue({
          inspect: {
            path: "",
            valid: true,
            errors: [],
            schemaVersion: 1,
            stableKey: "chauvet-dj/led-par-64-tri-b",
            contentHash: "abc123content",
            revision: "abc123conten",
            source: "ofl:chauvet-dj/led-par-64-tri-b",
            validationResult: "valid",
            warnings: [],
          },
          previewToken: "/tmp/preview/chauvet-dj_led-par-64-tri-b.json",
          destinationExists: true,
          suggestedFileName: "chauvet-dj_led-par-64-tri-b.json",
        }),
        CommitPreview: vi.fn().mockResolvedValue({
          exitCode: 1,
          stdout: "",
          stderr: "GOLC_WAILS_FIXTURE_IMPORT_EXISTS: chauvet-dj_led-par-64-tri-b.json already exists in the library",
        }),
      });

      render(<FixtureLibraryWorkspace />);
      await waitFor(() => expect(screen.getByText("No fixtures yet")).toBeInTheDocument());
      await searchAndSelectManufacturer(svc);

      fireEvent.change(screen.getByLabelText("Fixture key"), { target: { value: "led-par-64-tri-b" } });
      fireEvent.click(screen.getByRole("button", { name: "Preview" }));
      await waitFor(() => expect(screen.getByRole("button", { name: "Add to Library" })).toBeInTheDocument());

      fireEvent.click(screen.getByRole("button", { name: "Add to Library" }));

      await waitFor(() => expect(svc.CommitPreview).toHaveBeenCalled());
      await waitFor(() => expect(screen.getByText("This fixture is already in your library.")).toBeInTheDocument());
      expect(screen.getByRole("button", { name: "Replace" })).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Add to Library" })).not.toBeInTheDocument();
    });
  });
});
