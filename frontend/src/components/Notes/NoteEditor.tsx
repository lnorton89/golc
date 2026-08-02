// NoteEditor.tsx: a real Tiptap (ProseMirror) instance running standard
// rich-text formatting -- bold/italic/underline/strikethrough, headings
// (H1-H3), bullet/numbered/task lists, blockquote, inline code + code
// blocks, links, and undo/redo. Replaces NotesWorkspace.tsx's per-note
// selection with a genuine WYSIWYG editing surface.
//
// Mirrors components/Scripts/ScriptEditor.tsx's own mount discipline
// closely: "@tiptap/core"/"@tiptap/starter-kit"/task-list/task-item are
// imported DYNAMICALLY inside a mount effect (never a static top-level
// import) and wrapped in try/catch, so any environment incompatibility
// (e.g. a test harness's jsdom lacking a full contenteditable/Selection
// implementation) degrades to a caught failure scoped to this component
// alone, never a crash that reaches AppShell.navigation.test.tsx's
// zero-console-error assertion. NoteEditor.test.tsx vi.mock()s
// "@tiptap/core" et al. to exercise the real mount path against a fake;
// every other test that merely renders through this component (e.g. the
// no-note-selected branch of NotesWorkspace, which never mounts this
// component at all) never touches it.
//
// A deliberately uncontrolled editing surface, unlike ScriptEditor's
// controlled `value`/`onChange` contract: Tiptap's own Editor instance is
// imperative and framework-agnostic (it never re-renders React on its
// own), and `content` only ever seeds the editor ONCE, at construction.
// NotesWorkspace.tsx remounts this component with a fresh React `key` (the
// selected note's id) whenever the selection changes, rather than this
// component diffing an incoming `value` prop against its own live HTML --
// simpler and safer than a Monaco-style controlled-value sync effect, at
// the cost of a full editor re-creation per note switch (cheap: the
// dynamic import above is cached by the ES module loader after the first
// mount, so switching notes never re-fetches anything).
import { useEffect, useRef, useState } from "react";
import type { Editor as TiptapEditor } from "@tiptap/core";
import {
  Bold,
  Italic,
  Underline as UnderlineIcon,
  Strikethrough,
  Heading1,
  Heading2,
  Heading3,
  List,
  ListOrdered,
  ListChecks,
  Quote,
  Code,
  SquareCode,
  Link as LinkIcon,
  Undo2,
  Redo2,
  type LucideIcon,
} from "lucide-react";

import styles from "./NoteEditor.module.css";

const READY_PLACEHOLDER = "Loading editor…";
const LOAD_FAILED_MESSAGE = "The note editor failed to load. Reload GOLC to try again.";

interface ToolbarItem {
  key: string;
  label: string;
  icon: LucideIcon;
  isActive: (editor: TiptapEditor) => boolean;
  run: (editor: TiptapEditor) => void;
}

// TOOLBAR_ITEMS is the agreed "standard rich text" formatting set: bold/
// italic/underline/strikethrough, headings H1-H3, bullet/numbered/task
// lists, blockquote, inline code + code blocks, and undo/redo. Link is
// handled separately below (it needs a URL prompt, not a plain toggle).
const TOOLBAR_ITEMS: ToolbarItem[] = [
  {
    key: "bold",
    label: "Bold",
    icon: Bold,
    isActive: (editor) => editor.isActive("bold"),
    run: (editor) => editor.chain().focus().toggleBold().run(),
  },
  {
    key: "italic",
    label: "Italic",
    icon: Italic,
    isActive: (editor) => editor.isActive("italic"),
    run: (editor) => editor.chain().focus().toggleItalic().run(),
  },
  {
    key: "underline",
    label: "Underline",
    icon: UnderlineIcon,
    isActive: (editor) => editor.isActive("underline"),
    run: (editor) => editor.chain().focus().toggleUnderline().run(),
  },
  {
    key: "strike",
    label: "Strikethrough",
    icon: Strikethrough,
    isActive: (editor) => editor.isActive("strike"),
    run: (editor) => editor.chain().focus().toggleStrike().run(),
  },
  {
    key: "h1",
    label: "Heading 1",
    icon: Heading1,
    isActive: (editor) => editor.isActive("heading", { level: 1 }),
    run: (editor) => editor.chain().focus().toggleHeading({ level: 1 }).run(),
  },
  {
    key: "h2",
    label: "Heading 2",
    icon: Heading2,
    isActive: (editor) => editor.isActive("heading", { level: 2 }),
    run: (editor) => editor.chain().focus().toggleHeading({ level: 2 }).run(),
  },
  {
    key: "h3",
    label: "Heading 3",
    icon: Heading3,
    isActive: (editor) => editor.isActive("heading", { level: 3 }),
    run: (editor) => editor.chain().focus().toggleHeading({ level: 3 }).run(),
  },
  {
    key: "bulletList",
    label: "Bullet list",
    icon: List,
    isActive: (editor) => editor.isActive("bulletList"),
    run: (editor) => editor.chain().focus().toggleBulletList().run(),
  },
  {
    key: "orderedList",
    label: "Numbered list",
    icon: ListOrdered,
    isActive: (editor) => editor.isActive("orderedList"),
    run: (editor) => editor.chain().focus().toggleOrderedList().run(),
  },
  {
    key: "taskList",
    label: "Task list",
    icon: ListChecks,
    isActive: (editor) => editor.isActive("taskList"),
    run: (editor) => editor.chain().focus().toggleTaskList().run(),
  },
  {
    key: "blockquote",
    label: "Blockquote",
    icon: Quote,
    isActive: (editor) => editor.isActive("blockquote"),
    run: (editor) => editor.chain().focus().toggleBlockquote().run(),
  },
  {
    key: "code",
    label: "Inline code",
    icon: Code,
    isActive: (editor) => editor.isActive("code"),
    run: (editor) => editor.chain().focus().toggleCode().run(),
  },
  {
    key: "codeBlock",
    label: "Code block",
    icon: SquareCode,
    isActive: (editor) => editor.isActive("codeBlock"),
    run: (editor) => editor.chain().focus().toggleCodeBlock().run(),
  },
];

interface NoteEditorProps {
  value: string;
  onChange: (html: string) => void;
  readOnly?: boolean;
  ariaLabel?: string;
}

export default function NoteEditor({ value, onChange, readOnly, ariaLabel }: NoteEditorProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const editorRef = useRef<TiptapEditor | null>(null);
  const onChangeRef = useRef(onChange);
  const valueRef = useRef(value);
  const readOnlyRef = useRef(readOnly);
  const [ready, setReady] = useState(false);
  const [loadFailed, setLoadFailed] = useState(false);
  const [linkPromptOpen, setLinkPromptOpen] = useState(false);
  const [linkUrl, setLinkUrl] = useState("");
  // tick forces a re-render on every Tiptap transaction so toolbar
  // active-state (editor.isActive(...)) stays in sync with the live
  // selection -- the Editor instance is imperative and never re-renders
  // React on its own.
  const [, setTick] = useState(0);

  onChangeRef.current = onChange;
  valueRef.current = value;
  readOnlyRef.current = readOnly;

  useEffect(() => {
    let cancelled = false;

    void (async () => {
      let core: typeof import("@tiptap/core");
      let starterKitModule: typeof import("@tiptap/starter-kit");
      let taskListModule: typeof import("@tiptap/extension-task-list");
      let taskItemModule: typeof import("@tiptap/extension-task-item");
      try {
        [core, starterKitModule, taskListModule, taskItemModule] = await Promise.all([
          import("@tiptap/core"),
          import("@tiptap/starter-kit"),
          import("@tiptap/extension-task-list"),
          import("@tiptap/extension-task-item"),
        ]);
      } catch {
        if (!cancelled) setLoadFailed(true);
        return;
      }
      if (cancelled || !containerRef.current) return;

      const StarterKit = starterKitModule.default;
      const TaskList = taskListModule.default;
      const TaskItem = taskItemModule.default;

      const editor = new core.Editor({
        element: containerRef.current,
        extensions: [
          StarterKit.configure({
            heading: { levels: [1, 2, 3] },
            link: { openOnClick: false },
          }),
          TaskList,
          TaskItem.configure({ nested: true }),
        ],
        content: valueRef.current,
        editable: !readOnlyRef.current,
        editorProps: {
          attributes: {
            "aria-label": ariaLabel ?? "Note body",
            class: styles.prose,
          },
        },
        onUpdate: ({ editor: updated }) => onChangeRef.current(updated.getHTML()),
        onTransaction: () => setTick((current) => current + 1),
      });
      editorRef.current = editor;
      setReady(true);
    })();

    return () => {
      cancelled = true;
      editorRef.current?.destroy();
      editorRef.current = null;
    };
    // Mounted exactly once per NoteEditor instance -- NotesWorkspace.tsx
    // keys this component by the selected note's id, so a note switch
    // remounts a fresh instance rather than this effect ever re-running in
    // place.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    editorRef.current?.setEditable(!readOnly);
  }, [readOnly]);

  const editor = editorRef.current;

  const handleLinkClick = () => {
    if (!editor) return;
    if (editor.isActive("link")) {
      editor.chain().focus().unsetLink().run();
      return;
    }
    setLinkUrl("");
    setLinkPromptOpen(true);
  };

  const applyLink = () => {
    const trimmed = linkUrl.trim();
    setLinkPromptOpen(false);
    if (!editor || trimmed === "") return;
    editor.chain().focus().extendMarkRange("link").setLink({ href: trimmed }).run();
  };

  return (
    <div className={styles.editor}>
      <div className={styles.toolbar} role="toolbar" aria-label="Formatting">
        {TOOLBAR_ITEMS.map(({ key, label, icon: Icon, isActive, run }) => (
          <button
            key={key}
            type="button"
            className={editor && isActive(editor) ? `${styles.toolbarButton} ${styles.active}` : styles.toolbarButton}
            aria-label={label}
            aria-pressed={editor ? isActive(editor) : false}
            disabled={!editor}
            onClick={() => editor && run(editor)}
          >
            <Icon size={14} aria-hidden="true" />
          </button>
        ))}
        <button
          type="button"
          className={editor?.isActive("link") ? `${styles.toolbarButton} ${styles.active}` : styles.toolbarButton}
          aria-label="Link"
          aria-pressed={editor ? editor.isActive("link") : false}
          disabled={!editor}
          onClick={handleLinkClick}
        >
          <LinkIcon size={14} aria-hidden="true" />
        </button>
        <span className={styles.toolbarDivider} aria-hidden="true" />
        <button
          type="button"
          className={styles.toolbarButton}
          aria-label="Undo"
          disabled={!editor || !editor.can().undo()}
          onClick={() => editor?.chain().focus().undo().run()}
        >
          <Undo2 size={14} aria-hidden="true" />
        </button>
        <button
          type="button"
          className={styles.toolbarButton}
          aria-label="Redo"
          disabled={!editor || !editor.can().redo()}
          onClick={() => editor?.chain().focus().redo().run()}
        >
          <Redo2 size={14} aria-hidden="true" />
        </button>
      </div>

      {linkPromptOpen ? (
        <div className={styles.linkPrompt}>
          <input
            className={styles.linkInput}
            type="text"
            value={linkUrl}
            placeholder="https://…"
            aria-label="Link URL"
            autoFocus
            onChange={(event) => setLinkUrl(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") applyLink();
              if (event.key === "Escape") setLinkPromptOpen(false);
            }}
          />
          <button type="button" className={styles.linkApply} onClick={applyLink}>
            Apply
          </button>
        </div>
      ) : null}

      <div ref={containerRef} className={styles.surface} />

      {!ready && !loadFailed ? <p className={styles.statusText}>{READY_PLACEHOLDER}</p> : null}
      {loadFailed ? <p className={styles.statusText}>{LOAD_FAILED_MESSAGE}</p> : null}
    </div>
  );
}
