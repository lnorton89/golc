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
//
// Autosave, not an explicit Save button: every title/body edit reschedules
// a debounced save (AUTOSAVE_DEBOUNCE_MS after the last edit), mirroring
// FixtureLibraryWorkspace.tsx's own inline setTimeout-based debounce for
// its OFL search -- there is no shared debounce hook anywhere in this
// codebase to reuse instead. The pending save is tracked in a ref (not
// state, since it never needs to re-render anything itself) and is always
// flushed immediately -- never merely dropped -- the moment it would
// otherwise be lost: switching to a different note, deleting the note a
// save is pending for, and unmounting the workspace entirely all flush
// first. selectNote() is this file's single choke point for changing
// selectedId, specifically so no call site can forget the flush.
import { useCallback, useEffect, useRef, useState, type CSSProperties } from "react";
import { NotebookPen, Plus, X, Trash2 } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";

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

import { Button, ConfirmDialog, EmptyState, Field, ListRow, Panel, ResizeHandle, ScrollRegion, WorkspaceFrame } from "../../design-system";
import { motionTransition } from "../../design-system/motion";
import NoteEditor from "../../components/Notes/NoteEditor";
import { useResizablePanel } from "../../hooks/useResizablePanel";
import styles from "./NotesWorkspace.module.css";

const rowExitTransition = motionTransition("settle");

// HOST_UNREACHABLE_MESSAGE mirrors ScriptsWorkspace.tsx's identical "can't
// reach the host" copy convention, rendered inline whenever NotesService is
// not bound.
const HOST_UNREACHABLE_MESSAGE = "Can't reach the show host. GOLC will try to reconnect automatically.";

// AUTOSAVE_DEBOUNCE_MS is how long a title/body field must sit idle before
// the accumulated edit actually saves -- long enough that a normal typing
// burst issues at most one SaveNote call, not one per keystroke (mirrors
// FixtureLibraryWorkspace.tsx's oflSearchDebounceMs rationale, just tuned
// for prose-length typing pauses rather than a search box).
const AUTOSAVE_DEBOUNCE_MS = 800;

// SAVED_STATUS_LINGER_MS is how long the "Saved" status stays visible
// after a successful autosave before fading back to no status at all.
const SAVED_STATUS_LINGER_MS = 2000;

type SaveStatus = "idle" | "pending" | "saving" | "saved" | "error";

function saveStatusLabel(status: SaveStatus): string | null {
  switch (status) {
    case "pending":
      return "Unsaved changes";
    case "saving":
      return "Saving…";
    case "saved":
      return "Saved";
    case "error":
      return "Save failed";
    default:
      return null;
  }
}

interface PendingSave {
  addressName: string;
  title: string;
  body: string;
}

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
  const [saveStatus, setSaveStatus] = useState<SaveStatus>("idle");

  const notesRef = useRef(notes);
  notesRef.current = notes;
  const selectedAddressNameRef = useRef(selectedAddressName);
  selectedAddressNameRef.current = selectedAddressName;
  // pendingSaveRef/saveTimerRef are refs, not state: the pending edit and
  // its debounce timer never need to trigger a re-render themselves --
  // only saveStatus (set alongside them) does.
  const pendingSaveRef = useRef<PendingSave | null>(null);
  const saveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const savedLingerTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

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

  // performSave is the one place that actually calls SaveNote -- shared by
  // the debounced autosave timer and every flush call site below, so there
  // is exactly one save implementation regardless of what triggered it.
  const performSave = useCallback(
    (pending: PendingSave) => {
      setSaveStatus("saving");
      void (async () => {
        try {
          const result = await saveNote(pending.addressName, pending.title, pending.body);
          assertOk(result, "SaveNote");
          // A rename mid-edit means later saves must address the note by
          // its now-current title -- but only apply this if the selection
          // hasn't moved on to a different note entirely in the meantime.
          if (selectedAddressNameRef.current === pending.addressName) {
            setSelectedAddressName(pending.title);
          }
          await refresh();
          setSaveStatus("saved");
          if (savedLingerTimerRef.current) clearTimeout(savedLingerTimerRef.current);
          savedLingerTimerRef.current = setTimeout(() => {
            setSaveStatus((current) => (current === "saved" ? "idle" : current));
          }, SAVED_STATUS_LINGER_MS);
        } catch (err) {
          setError(errorMessage(err));
          setSaveStatus("error");
        }
      })();
    },
    [refresh],
  );

  // flushPendingSave cancels any pending debounce timer and saves
  // immediately -- called at every point a pending autosave would
  // otherwise be silently lost: switching to a different note, and
  // unmounting the whole workspace (WorkspaceRouter unmounts the previous
  // workspace on every navigate-away).
  const flushPendingSave = useCallback(() => {
    if (saveTimerRef.current) {
      clearTimeout(saveTimerRef.current);
      saveTimerRef.current = null;
    }
    const pending = pendingSaveRef.current;
    pendingSaveRef.current = null;
    if (pending) {
      performSave(pending);
    }
  }, [performSave]);

  // scheduleAutosave reschedules the debounced save on every title/body
  // edit -- only the edit still pending AUTOSAVE_DEBOUNCE_MS after the
  // last keystroke actually reaches performSave.
  const scheduleAutosave = useCallback(
    (nextTitle: string, nextBody: string) => {
      const addressName = selectedAddressNameRef.current;
      if (!addressName) return;
      pendingSaveRef.current = { addressName, title: nextTitle, body: nextBody };
      setSaveStatus("pending");
      if (saveTimerRef.current) clearTimeout(saveTimerRef.current);
      saveTimerRef.current = setTimeout(() => {
        saveTimerRef.current = null;
        const pending = pendingSaveRef.current;
        pendingSaveRef.current = null;
        if (pending) performSave(pending);
      }, AUTOSAVE_DEBOUNCE_MS);
    },
    [performSave],
  );

  const handleTitleChange = (next: string) => {
    setTitle(next);
    scheduleAutosave(next, body);
  };

  const handleBodyChange = (next: string) => {
    setBody(next);
    scheduleAutosave(title, next);
  };

  // selectNote is the single choke point for changing selectedId (every
  // call site below routes through it instead of the raw setSelectedId
  // setter) specifically so a pending autosave for the note being switched
  // away from always flushes first, never silently dropped.
  const selectNote = useCallback(
    (id: string | null) => {
      flushPendingSave();
      setSelectedId(id);
    },
    [flushPendingSave],
  );

  // Flushes once more on unmount -- catches the case where the debounce
  // timer was still pending when the user navigated to a different
  // destination entirely (never reachable through selectNote).
  useEffect(() => {
    return () => {
      flushPendingSave();
      if (savedLingerTimerRef.current) clearTimeout(savedLingerTimerRef.current);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Selection-validity-repair effect (ScenesLooksWorkspace.tsx/
  // ScriptsWorkspace.tsx's identical discipline): drop a selection that no
  // longer exists (e.g. a CLI-side delete outside this session) and
  // default to the first note once data loads.
  useEffect(() => {
    if (selectedId && notes.some((note) => note.id === selectedId)) {
      return;
    }
    selectNote(notes[0]?.id ?? null);
  }, [notes, selectedId, selectNote]);

  // Loads the selected note's full detail (including body) whenever
  // selection changes. Reads notesRef (not this effect's own closured
  // `notes`) so the addressing title used for GetNote is always the
  // freshest known one for this id, even though this effect only re-runs
  // on selectedId changing.
  useEffect(() => {
    setSaveStatus("idle");
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
        selectNote(created?.id ?? null);
      } catch (err) {
        setError(errorMessage(err));
      }
    })();
  };

  const handleDelete = () => {
    if (!selectedAddressName) {
      return;
    }
    const addressName = selectedAddressName;
    // The note is about to be deleted -- discard any pending autosave for
    // it outright rather than flushing (a write immediately followed by a
    // delete would be pure waste, and could itself race the delete).
    if (saveTimerRef.current) {
      clearTimeout(saveTimerRef.current);
      saveTimerRef.current = null;
    }
    pendingSaveRef.current = null;
    void (async () => {
      try {
        const result = await deleteNote(addressName);
        assertOk(result, "DeleteNote");
        setConfirmingDelete(false);
        selectNote(null);
        await refresh();
      } catch (err) {
        setError(errorMessage(err));
      }
    })();
  };

  const selectedNote = notes.find((note) => note.id === selectedId) ?? null;

  const newNoteForm = creating ? (
    <div className={styles.createForm}>
      <Field
        label="New note title"
        type="text"
        value={newTitle}
        placeholder="Note title"
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
      <Button variant="destructive" icon={Trash2} onClick={() => setConfirmingDelete(true)} disabled={!selectedAddressName}>
        Delete Note
      </Button>
    </div>
  );

  return (
    <WorkspaceFrame title="Notes" action={toolbarActions}>
      <div className={styles.canvas}>
        {error ? <p className={styles.feedback}>{error}</p> : null}

        {!loading && notes.length === 0 ? (
          <Panel>
            <EmptyState
              icon={NotebookPen}
              heading="No notes yet"
              body="Create a note to keep free-form, formatted text alongside this show."
              action={creating ? newNoteForm : <Button variant="primary" icon={Plus} onClick={() => setCreating(true)}>New Note</Button>}
            />
          </Panel>
        ) : (
          <div className={styles.layout} style={{ gridTemplateColumns: `${libraryPanel.size}px minmax(0, 1fr)` } as CSSProperties}>
            <div className={styles.library}>
              {newNoteForm}
              <ResizeHandle
                edge="end"
                label="Resize note list"
                isResizing={libraryPanel.isResizing}
                onPointerDown={libraryPanel.handlePointerDown}
                onDoubleClick={libraryPanel.resetSize}
              />
              <ScrollRegion>
                {loading ? (
                  <ul className={styles.list} aria-label="Note list">
                    <li className={styles.noteListItem}>Loading notes…</li>
                  </ul>
                ) : (
                  <ul className={styles.list} aria-label="Note list">
                    <AnimatePresence initial={false}>
                      {notes.map((note) => (
                        <motion.li
                          key={note.id}
                          style={{ overflow: "hidden" }}
                          initial={false}
                          exit={{ opacity: 0, height: 0 }}
                          transition={rowExitTransition}
                        >
                          <ListRow
                            label={note.title}
                            icon={NotebookPen}
                            selected={note.id === selectedId}
                            onSelect={() => selectNote(note.id)}
                          />
                        </motion.li>
                      ))}
                    </AnimatePresence>
                  </ul>
                )}
              </ScrollRegion>
            </div>

            <div className={styles.editorColumn}>
              {selectedNote ? (
                <>
                  <div className={styles.editorHeaderRow}>
                    <Field
                      label="Note title"
                      type="text"
                      value={title}
                      onChange={(event) => handleTitleChange(event.target.value)}
                    />
                    {saveStatusLabel(saveStatus) ? (
                      <span className={styles.saveStatus}>{saveStatusLabel(saveStatus)}</span>
                    ) : null}
                  </div>
                  {detailLoading ? (
                    <p className={styles.statusText}>Loading note…</p>
                  ) : (
                    <NoteEditor
                      key={selectedNote.id}
                      value={body}
                      onChange={handleBodyChange}
                      ariaLabel={`${selectedNote.title} body`}
                    />
                  )}
                </>
              ) : (
                <p className={styles.statusText}>Select a note to view and edit it.</p>
              )}
            </div>
          </div>
        )}
      </div>

      <ConfirmDialog
        open={confirmingDelete && selectedNote !== null}
        destructive
        title="Delete Note"
        message={
          selectedNote ? `This permanently removes "${selectedNote.title}" from this show. This can't be undone.` : ""
        }
        confirmLabel="Delete Note"
        onConfirm={handleDelete}
        onCancel={() => setConfirmingDelete(false)}
      />
    </WorkspaceFrame>
  );
}
