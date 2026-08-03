// NoteEditor.test.tsx: Tiptap/ProseMirror cannot reliably instantiate
// under jsdom (no real Selection/contenteditable layer), so this file
// vi.mock()s "@tiptap/core"/"@tiptap/starter-kit"/"@tiptap/extension-task-
// list"/"@tiptap/extension-task-item" with a fake Editor exposing chain/
// isActive/getHTML/commands/can/setEditable/destroy, then asserts
// NoteEditor.tsx's own wiring against the fake's recorded calls -- mirrors
// ScriptEditor.test.tsx's identical "mock the heavy editor library, test
// this component's own wiring" convention exactly.
//
// NoteEditor.tsx dynamically imports these modules inside its mount effect
// (rather than a static top-level import), so mounting resolves on a
// microtask -- every test below awaits the fake Editor having been
// constructed before asserting against it.
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

const TOGGLE_TO_MARK: Record<string, string> = {
  toggleBold: "bold",
  toggleItalic: "italic",
  toggleUnderline: "underline",
  toggleStrike: "strike",
  toggleBulletList: "bulletList",
  toggleOrderedList: "orderedList",
  toggleTaskList: "taskList",
  toggleBlockquote: "blockquote",
  toggleCode: "code",
  toggleCodeBlock: "codeBlock",
};

vi.mock("@tiptap/core", () => {
  class FakeEditor {
    private html: string;
    private activeMarks = new Set<string>();
    private options: Record<string, unknown>;
    commands = { setContent: vi.fn((html: string) => { this.html = html; }) };
    setEditable = vi.fn();
    destroy = vi.fn();

    constructor(options: Record<string, unknown>) {
      this.options = options;
      this.html = (options.content as string) ?? "";
    }

    getHTML(): string {
      return this.html;
    }

    isActive(name: string): boolean {
      return this.activeMarks.has(name);
    }

    can() {
      return { undo: () => true, redo: () => true };
    }

    chain() {
      const pending: string[] = [];
      const chainObj: Record<string, (...args: unknown[]) => unknown> = {};
      const methods = [
        "focus",
        ...Object.keys(TOGGLE_TO_MARK),
        "toggleHeading",
        "setLink",
        "unsetLink",
        "extendMarkRange",
        "undo",
        "redo",
      ];
      methods.forEach((method) => {
        chainObj[method] = (...args: unknown[]) => {
          pending.push(method);
          if (method === "setLink" && args[0]) {
            this.html += `<a href="${(args[0] as { href: string }).href}">link</a>`;
          }
          return chainObj;
        };
      });
      chainObj.run = () => {
        pending.forEach((method) => {
          const mark = TOGGLE_TO_MARK[method];
          if (mark) {
            if (this.activeMarks.has(mark)) {
              this.activeMarks.delete(mark);
            } else {
              this.activeMarks.add(mark);
              this.html += `<strong data-mark="${mark}"></strong>`;
            }
          }
          if (method === "unsetLink") {
            this.activeMarks.delete("link");
          }
        });
        const onUpdate = this.options.onUpdate as ((args: { editor: FakeEditor }) => void) | undefined;
        onUpdate?.({ editor: this });
        const onTransaction = this.options.onTransaction as (() => void) | undefined;
        onTransaction?.();
        return this;
      };
      return chainObj;
    }
  }

  return { Editor: FakeEditor };
});

vi.mock("@tiptap/starter-kit", () => ({
  default: { configure: vi.fn(() => "starter-kit-configured") },
}));
vi.mock("@tiptap/extension-task-list", () => ({ default: "task-list" }));
vi.mock("@tiptap/extension-task-item", () => ({
  default: { configure: vi.fn(() => "task-item-configured") },
}));

vi.mock("../../design-system", () => ({
  IconButton: ({ label, ...props }: { label: string }) => <button data-design-system-icon-button="true" aria-label={label} {...props} />,
  Button: ({ children, ...props }: { children: React.ReactNode }) => <button data-design-system-button="true" {...props}>{children}</button>,
  Field: ({ label, ...props }: { label: string }) => <label>{label}<input data-design-system-field="true" {...props} /></label>,
}));

import NoteEditor from "./NoteEditor";

describe("NoteEditor", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("mounts with initial content and renders the formatting toolbar", async () => {
    render(<NoteEditor value="<p>Hello</p>" onChange={() => {}} ariaLabel="Test note body" />);

    expect(await screen.findByRole("button", { name: "Bold" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Italic" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Link" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Undo" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Redo" })).toBeInTheDocument();
  });

  it("uses public design-system controls for editor-adjacent chrome", async () => {
    render(<NoteEditor value="<p>Hello</p>" onChange={() => {}} />);

    await screen.findByRole("button", { name: "Bold" });
    expect(document.querySelectorAll("[data-design-system-icon-button='true']")).toHaveLength(16);

    fireEvent.click(screen.getByRole("button", { name: "Link" }));
    expect(screen.getByLabelText("Link URL")).toHaveAttribute("data-design-system-field", "true");
    expect(screen.getByRole("button", { name: "Apply" })).toHaveAttribute("data-design-system-button", "true");
  });

  it("toggling Bold marks the toolbar button active and fires onChange with updated HTML", async () => {
    const handleChange = vi.fn();
    render(<NoteEditor value="<p>Hello</p>" onChange={handleChange} />);

    const boldButton = await screen.findByRole("button", { name: "Bold" });
    expect(boldButton).toHaveAttribute("aria-pressed", "false");

    fireEvent.click(boldButton);

    await waitFor(() => expect(boldButton).toHaveAttribute("aria-pressed", "true"));
    expect(handleChange).toHaveBeenCalledWith(expect.stringContaining('data-mark="bold"'));
  });

  it("toggling Bold again turns it back off", async () => {
    render(<NoteEditor value="<p>Hello</p>" onChange={() => {}} />);

    const boldButton = await screen.findByRole("button", { name: "Bold" });
    fireEvent.click(boldButton);
    await waitFor(() => expect(boldButton).toHaveAttribute("aria-pressed", "true"));

    fireEvent.click(boldButton);
    await waitFor(() => expect(boldButton).toHaveAttribute("aria-pressed", "false"));
  });

  it("applies a link via the inline URL prompt", async () => {
    const handleChange = vi.fn();
    render(<NoteEditor value="<p>Hello</p>" onChange={handleChange} />);

    const linkButton = await screen.findByRole("button", { name: "Link" });
    fireEvent.click(linkButton);

    const input = screen.getByLabelText("Link URL");
    fireEvent.change(input, { target: { value: "https://example.com" } });
    fireEvent.click(screen.getByText("Apply"));

    await waitFor(() => expect(handleChange).toHaveBeenCalledWith(expect.stringContaining("https://example.com")));
  });
});
