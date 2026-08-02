// NotesWorkspace is Show -> Notes: a list of free-form rich-text notes
// scoped to the current show, each editable through NoteEditor's WYSIWYG
// surface. Owns every NotesService call and all note state, following
// ScriptsWorkspace.tsx's exact load/refresh/error and selection-validity-
// repair pattern (08-PATTERNS.md) -- the closest existing "list of
// entities + editor for the selected one" workspace, simplified: no run/
// debug/validate/live-event machinery a note has no use for.
//
// Notes are addressed by Title over the wire ("note show/edit/delete
// <name>"), but a note's Title is itself editable here (unlike a Script's
// Name, which never changes) -- so this file tracks the note's stable id
// (for React's own selection/remount key) separately from
// selectedAddressName, the title CreateNote/GetNote/SaveNote/DeleteNote
// actually address the note by. selectedAddressName always holds the
// note's LAST SAVED title; a save sends the edited title as SaveNote's new
// value and only then advances selectedAddressName to match, mirroring
// "note edit"'s own current-name-to-look-up-by / --title-to-rename-to
// distinction (internal/command/note.go's runNoteEdit).
import { useCallback, useEffect, useRef, useState, type CSSProperties } from "react";
import { NotebookPen, Plus, X, Save, Trash2 } from "lucide-react";

import {
  assertOk,
  createNote,
  deleteNote,
  errorMessage,
  getNote,
  listNotes,
  saveNote,
  type NoteSummaryView,
} from "../../lib/wailsBridge";

import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import { HOW_IT_WORKS_BY_ID } from "../../shell/navigation";
import Panel from "../../components/primitives/Panel/Panel";
import PanelHeader from "../../components/primitives/PanelHeader/PanelHeader";
import ScrollRegion from "../../components/primitives/ScrollRegion/ScrollRegion";
import ListRow from "../../components/primitives/ListRow/ListRow";
import EmptyState from "../../components/primitives/EmptyState/EmptyState";
import Button from "../../components/primitives/Button/Button";
import ConfirmModal from "../../components/primitives/ConfirmModal/ConfirmModal";
import NoteEditor from "../../components/Notes/NoteEditor";
import { useResizablePanel } from "../../hooks/useResizablePanel";
import ResizeHandle from "../../components/primitives/ResizeHandle/ResizeHandle";
import styles from "./NotesWorkspace.module.css";

// HOST_UNREACHABLE_MESSAGE mirrors ScriptsWorkspace.tsx's identical "can't
// reach the host" copy convention, rendered inline whenever NotesService is
// not bound.
const HOST_UNREACHABLE_MESSAGE = "Can't reach the show host. GOLC will try to reconnect automatically.";

export default function NotesWorkspace() {
  const [notes, setNotes] = useState<NoteSummaryView[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [selectedAddressName, setSelectedAddressName] = useState<string | null>(null);
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [detailLoading, setDetailLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [newTitle, setNewTitle] = useState("");
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [saving, setSaving] = useState(false);

  const notesRef = useRef(notes);
  notesRef.current = notes;

  const libraryPanel = useResizablePanel({
    min: 180,
    max: 440,
    defaultSize: 240,
    storageKey: "golc.notesLibraryWidth",
    edge: "end",
  });

  const refresh = useCallback(async (): Promise<NoteSummaryView[]> => {
    try {
      const next = await listNotes();
      setNotes(next);
      const bridgeMissing = typeof window === "undefined" || !window.go?.wails?.NotesService;
      setError(bridgeMissing ? HOST_UNREACHABLE_MESSAGE : null);
      return next;
    } catch (err) {
      setError(errorMessage(err));
      return notesRef.current;
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  // Selection-validity-repair effect (ScenesLooksWorkspace.tsx/
  // ScriptsWorkspace.tsx's identical discipline): drop a selection that no
  // longer exists (e.g. a CLI-side delete outside this session) and
  // default to the first note once data loads.
  useEffect(() => {
    if (selectedId && notes.some((note) => note.id === selectedId)) {
      return;
    }
    setSelectedId(notes[0]?.id ?? null);
  }, [notes, selectedId]);

  // Loads the selected note's full detail (including body) whenever
  // selection changes. Reads notesRef (not this effect's own closured
  // `notes`) so the addressing title used for GetNote is always the
  // freshest known one for this id, even though this effect only re-runs
  // on selectedId changing.
  useEffect(() => {
    if (!selectedId) {
      setSelectedAddressName(null);
      setTitle("");
      setBody("");
      return;
    }
    const addressName = notesRef.current.find((note) => note.id === selectedId)?.title;
    if (!addressName) {
      return;
    }
    let cancelled = false;
    setDetailLoading(true);
    void (async () => {
      try {
        const detail = await getNote(addressName);
        if (!cancelled) {
          setSelectedAddressName(detail.title);
          setTitle(detail.title);
          setBody(detail.body);
        }
      } catch (err) {
        if (!cancelled) {
          setError(errorMessage(err));
        }
      } finally {
        if (!cancelled) {
          setDetailLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [selectedId]);

  const handleCreate = () => {
    const trimmed = newTitle.trim();
    if (trimmed === "") {
      return;
    }
    void (async () => {
      try {
        const result = await createNote(trimmed);
        assertOk(result, "CreateNote");
        setNewTitle("");
        setCreating(false);
        const list = await refresh();
        const created = list.find((note) => note.title === trimmed);
        setSelectedId(created?.id ?? null);
      } catch (err) {
        setError(errorMessage(err));
      }
    })();
  };

  const handleSave = () => {
    if (!selectedAddressName) {
      return;
    }
    const addressName = selectedAddressName;
    setSaving(true);
    void (async () => {
      try {
        const result = await saveNote(addressName, title, body);
        assertOk(result, "SaveNote");
        setSelectedAddressName(title);
        await refresh();
      } catch (err) {
        setError(errorMessage(err));
      } finally {
        setSaving(false);
      }
    })();
  };

  const handleDelete = () => {
    if (!selectedAddressName) {
      return;
    }
    void (async () => {
      try {
        const result = await deleteNote(selectedAddressName);
        assertOk(result, "DeleteNote");
        setConfirmingDelete(false);
        setSelectedId(null);
        await refresh();
      } catch (err) {
        setError(errorMessage(err));
      }
    })();
  };

  const selectedNote = notes.find((note) => note.id === selectedId) ?? null;

  const newNoteForm = creating ? (
    <div className={styles.createForm}>
      <input
        className={styles.createInput}
        type="text"
        value={newTitle}
        placeholder="Note title"
        aria-label="New note title"
        onChange={(event) => setNewTitle(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === "Enter") {
            handleCreate();
          }
        }}
      />
      <Button variant="primary" onClick={handleCreate}>
        Create
      </Button>
    </div>
  ) : null;

  const toolbarActions = (
    <div className={styles.toolbarActions}>
      <Button variant="secondary" icon={creating ? X : Plus} onClick={() => setCreating((current) => !current)}>
        {creating ? "Cancel" : "New Note"}
      </Button>
      <Button variant="primary" icon={Save} onClick={handleSave} disabled={!selectedAddressName || saving}>
        Save
      </Button>
      <Button variant="destructive" icon={Trash2} onClick={() => setConfirmingDelete(true)} disabled={!selectedAddressName}>
        Delete Note
      </Button>
    </div>
  );

  return (
    <div className={styles.workspace}>
      <Toolbar title="Notes" icon={NotebookPen} info={HOW_IT_WORKS_BY_ID["show-notes"]} action={toolbarActions} />
      <div className={styles.canvas}>
        {error ? <p className={styles.errorText}>{error}</p> : null}

        {!loading && notes.length === 0 ? (
          <Panel className={styles.emptyState}>
            <EmptyState icon={NotebookPen}>
              <span className={styles.emptyHeading}>No notes yet</span>
              <span className={styles.emptyBody}>Create a note to keep free-form, formatted text alongside this show.</span>
            </EmptyState>
            {creating ? null : (
              <Button variant="primary" icon={Plus} onClick={() => setCreating(true)}>
                New Note
              </Button>
            )}
            {newNoteForm}
          </Panel>
        ) : (
          <div className={styles.layout} style={{ "--library-width": `${libraryPanel.size}px` } as CSSProperties}>
            <div className={styles.library}>
              {newNoteForm}
              <ResizeHandle
                edge="end"
                label="Resize note list"
                isResizing={libraryPanel.isResizing}
                onPointerDown={libraryPanel.handlePointerDown}
                onDoubleClick={libraryPanel.resetSize}
              />
              <PanelHeader label="Notes" icon={NotebookPen} />
              <ScrollRegion>
                {loading ? (
                  <ul className={styles.list} aria-label="Note list">
                    <li className={styles.loadingRow}>Loading notes…</li>
                  </ul>
                ) : (
                  <ul className={styles.list} aria-label="Note list">
                    {notes.map((note) => (
                      <li key={note.id}>
                        <ListRow
                          label={note.title}
                          icon={NotebookPen}
                          selected={note.id === selectedId}
                          onSelect={() => setSelectedId(note.id)}
                        />
                      </li>
                    ))}
                  </ul>
                )}
              </ScrollRegion>
            </div>

            <div className={styles.editorColumn}>
              {selectedNote ? (
                <>
                  <input
                    className={styles.titleInput}
                    type="text"
                    value={title}
                    aria-label="Note title"
                    onChange={(event) => setTitle(event.target.value)}
                  />
                  {detailLoading ? (
                    <p className={styles.statusText}>Loading note…</p>
                  ) : (
                    <NoteEditor
                      key={selectedNote.id}
                      value={body}
                      onChange={setBody}
                      ariaLabel={`${selectedNote.title} body`}
                    />
                  )}
                </>
              ) : (
                <p className={styles.emptySelection}>Select a note to view and edit it.</p>
              )}
            </div>
          </div>
        )}
      </div>

      {confirmingDelete && selectedNote ? (
        <ConfirmModal
          title="Delete Note"
          message={`This permanently removes "${selectedNote.title}" from this show. This can't be undone.`}
          confirmLabel="Delete Note"
          onConfirm={handleDelete}
          onCancel={() => setConfirmingDelete(false)}
        />
      ) : null}
    </div>
  );
}
