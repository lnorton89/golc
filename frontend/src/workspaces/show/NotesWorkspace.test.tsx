// NotesWorkspace.test.tsx exercises the workspace end-to-end against a
// mocked window.go.wails.NotesService -- the same direct-window-object
// convention ShowsWorkspace.test.tsx/SaveRecoveryWorkspace.test.tsx use
// (see wailsBridge.ts's own doc comment). Tiptap is mocked exactly like
// NoteEditor.test.tsx (jsdom cannot reliably instantiate real ProseMirror)
// so selecting a note and mounting the editor stays cheap and deterministic
// here too.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";

vi.mock("@tiptap/core", () => {
  class FakeEditor {
    private html: string;
    commands = { setContent: vi.fn() };
    setEditable = vi.fn();
    destroy = vi.fn();
    constructor(options: Record<string, unknown>) {
      this.html = (options.content as string) ?? "";
    }
    getHTML(): string {
      return this.html;
    }
    isActive(): boolean {
      return false;
    }
    can() {
      return { undo: () => true, redo: () => true };
    }
    chain() {
      const chainObj: Record<string, (...args: unknown[]) => unknown> = {};
      ["focus", "toggleBold", "undo", "redo"].forEach((method) => {
        chainObj[method] = () => chainObj;
      });
      chainObj.run = () => chainObj;
      return chainObj;
    }
  }
  return { Editor: FakeEditor };
});
vi.mock("@tiptap/starter-kit", () => ({ default: { configure: vi.fn(() => "starter-kit") } }));
vi.mock("@tiptap/extension-task-list", () => ({ default: "task-list" }));
vi.mock("@tiptap/extension-task-item", () => ({ default: { configure: vi.fn(() => "task-item") } }));

import NotesWorkspace from "./NotesWorkspace";

function ok(stdout = "") {
  return { exitCode: 0, stdout, stderr: "" };
}

interface NoteSummary {
  id: string;
  title: string;
  updatedAt?: string;
}

function summary(id: string, title: string): NoteSummary {
  return { id, title, updatedAt: "2026-01-01T00:00:00Z" };
}

type NotesServiceMock = {
  ListNotes: ReturnType<typeof vi.fn>;
  GetNote: ReturnType<typeof vi.fn>;
  CreateNote: ReturnType<typeof vi.fn>;
  SaveNote: ReturnType<typeof vi.fn>;
  DeleteNote: ReturnType<typeof vi.fn>;
};

function notesService(): NotesServiceMock {
  return (window as unknown as { go: { wails: { NotesService: NotesServiceMock } } }).go.wails.NotesService;
}

describe("NotesWorkspace", () => {
  beforeEach(() => {
    vi.stubGlobal("go", {
      wails: {
        NotesService: {
          ListNotes: vi.fn().mockResolvedValue([]),
          GetNote: vi.fn(),
          CreateNote: vi.fn().mockResolvedValue(ok()),
          SaveNote: vi.fn().mockResolvedValue(ok()),
          DeleteNote: vi.fn().mockResolvedValue(ok()),
        },
      },
    });
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("renders the empty state when there are no notes", async () => {
    render(<NotesWorkspace />);
    await waitFor(() => expect(screen.getByText("No notes yet")).toBeInTheDocument());
    expect(screen.getAllByRole("button", { name: "New Note" }).length).toBeGreaterThan(0);
  });

  it("creates a note and selects it", async () => {
    const svc = notesService();
    svc.ListNotes
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([summary("id-1", "Load-In Checklist")]);
    svc.GetNote.mockResolvedValue({ id: "id-1", title: "Load-In Checklist", body: "" });

    render(<NotesWorkspace />);
    await waitFor(() => expect(screen.getByText("No notes yet")).toBeInTheDocument());

    fireEvent.click(screen.getAllByRole("button", { name: "New Note" })[0]);
    fireEvent.change(screen.getByLabelText("New note title"), { target: { value: "Load-In Checklist" } });
    fireEvent.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => expect(svc.CreateNote).toHaveBeenCalledWith("Load-In Checklist"));
    await waitFor(() => expect(svc.GetNote).toHaveBeenCalledWith("Load-In Checklist"));
    await waitFor(() => expect(screen.getByLabelText("Note title")).toHaveValue("Load-In Checklist"));
  });

  it("selects an existing note and loads its body into the editor", async () => {
    const svc = notesService();
    svc.ListNotes.mockResolvedValue([summary("id-1", "Load-In Checklist")]);
    svc.GetNote.mockResolvedValue({ id: "id-1", title: "Load-In Checklist", body: "<p>Bring gaff tape.</p>" });

    render(<NotesWorkspace />);

    await waitFor(() => expect(screen.getByRole("button", { name: "Load-In Checklist" })).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "Load-In Checklist" }));

    await waitFor(() => expect(svc.GetNote).toHaveBeenCalledWith("Load-In Checklist"));
    await waitFor(() => expect(screen.getByLabelText("Note title")).toHaveValue("Load-In Checklist"));
  });

  it("autosaves an edited title after a debounce, with no Save button anywhere", async () => {
    const svc = notesService();
    svc.ListNotes.mockResolvedValue([summary("id-1", "Load-In Checklist")]);
    svc.GetNote.mockResolvedValue({ id: "id-1", title: "Load-In Checklist", body: "<p>Bring gaff tape.</p>" });

    render(<NotesWorkspace />);
    await waitFor(() => expect(screen.getByLabelText("Note title")).toHaveValue("Load-In Checklist"));

    expect(screen.queryByRole("button", { name: "Save" })).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Note title"), { target: { value: "Load-In & Strike Checklist" } });

    await waitFor(() => expect(screen.getByText("Unsaved changes")).toBeInTheDocument());
    await waitFor(
      () =>
        expect(svc.SaveNote).toHaveBeenCalledWith(
          "Load-In Checklist",
          "Load-In & Strike Checklist",
          "<p>Bring gaff tape.</p>",
        ),
      { timeout: 2000 },
    );
    await waitFor(() => expect(screen.getByText("Saved")).toBeInTheDocument());
  });

  it("flushes a pending autosave immediately when switching to a different note", async () => {
    const svc = notesService();
    svc.ListNotes.mockResolvedValue([summary("id-1", "Load-In Checklist"), summary("id-2", "Strike Notes")]);
    svc.GetNote.mockImplementation((name: string) =>
      Promise.resolve(
        name === "Load-In Checklist"
          ? { id: "id-1", title: "Load-In Checklist", body: "<p>Bring gaff tape.</p>" }
          : { id: "id-2", title: "Strike Notes", body: "" },
      ),
    );

    render(<NotesWorkspace />);
    await waitFor(() => expect(screen.getByLabelText("Note title")).toHaveValue("Load-In Checklist"));

    fireEvent.change(screen.getByLabelText("Note title"), { target: { value: "Load-In & Strike Checklist" } });
    fireEvent.click(screen.getByRole("button", { name: "Strike Notes" }));

    await waitFor(() =>
      expect(svc.SaveNote).toHaveBeenCalledWith(
        "Load-In Checklist",
        "Load-In & Strike Checklist",
        "<p>Bring gaff tape.</p>",
      ),
    );
    await waitFor(() => expect(screen.getByLabelText("Note title")).toHaveValue("Strike Notes"));
  });

  it("deletes a note through the confirmation dialog", async () => {
    const svc = notesService();
    svc.ListNotes
      .mockResolvedValueOnce([summary("id-1", "Load-In Checklist")])
      .mockResolvedValueOnce([]);
    svc.GetNote.mockResolvedValue({ id: "id-1", title: "Load-In Checklist", body: "" });

    render(<NotesWorkspace />);
    await waitFor(() => expect(screen.getByLabelText("Note title")).toHaveValue("Load-In Checklist"));

    fireEvent.click(screen.getByRole("button", { name: "Delete Note" }));
    const dialog = screen.getByRole("alertdialog");

    // Base UI (Dialog's underlying primitive) marks the rest of the page
    // inert while the dialog is open, so the workspace's own toolbar
    // "Delete Note" trigger drops out of the accessibility tree -- only the
    // dialog's own confirm button is queryable now, scoped via `within`.
    fireEvent.click(within(dialog).getByRole("button", { name: "Delete Note" }));

    await waitFor(() => expect(svc.DeleteNote).toHaveBeenCalledWith("Load-In Checklist"));
    await waitFor(() => expect(screen.getByText("No notes yet")).toBeInTheDocument());
  });
});
