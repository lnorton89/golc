// SceneList is the Scene Stack's primary programming navigator
// (programming-scene-authoring.md): the scene list is the left-column
// nav; selecting a scene changes the layer rows shown alongside it. The
// create-scene form is a toggled inline reveal, not permanently visible
// (the sketch's own "what to avoid": permanently displaying every
// look-creation form).
//
// Drag-to-reorder: dropping a row updates local `order` immediately for
// zero-latency visual feedback, then calls `onReorder` (the caller
// translates the dragged name order into the 0-based index permutation
// "scene reorder" expects and persists it via ProgrammingService.
// ReorderScenes, mirroring every other mutation in this component's
// contract -- SceneList itself never calls wailsBridge directly). The
// optimistic order is rolled back when onReorder reports the server
// rejected it, since the reset effect below can't see that on its own.
// `order`
// lives in this component's own state, independent of the `scenes` prop's
// own array order, purely so the drop feels instant instead of waiting on
// a round trip.
//
// Reset behavior (see the useEffect below): local order is preserved
// across a `scenes` prop change UNLESS the underlying set of scene names
// actually changed (a scene was created/deleted/renamed elsewhere). This
// matters because ScenesLooksWorkspace.tsx calls refresh() -- which
// produces a brand-new `scenes` array reference -- after nearly every
// mutation in the whole workspace (creating a theme, chase, blend,
// motion, preset, programmer set, ...), not just scene mutations. Resetting
// on every reference change would wipe a local reorder the instant the
// operator did anything else in the workspace, which would make the
// feature useless. Resetting only when the name *set* changes still
// guarantees a stale order can never silently reference a scene that no
// longer exists (or omit one that now does).
//
// Row removal is the one genuine gap CSS can't fill on its own: a deleted
// scene's <li> is simply gone from the next render, so there is no DOM
// node left for a CSS transition to animate -- AnimatePresence is what
// lets the row play its own exit (collapse+fade) before React actually
// removes it. Every other row-level styling stays exactly as it was
// (dnd-kit's own drag-transform inline style, still merged in below);
// only removal is Motion's job here.
import { useEffect, useState, type CSSProperties } from "react";
import { Plus, X, Check, Layers, MoreVertical, Pencil, Trash2, GripVertical } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core";
import {
  SortableContext,
  arrayMove,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";

import type { ProgSceneView } from "../../lib/wailsBridge";
import { motionTransition } from "../../design-system/motion";
import { Button, ConfirmDialog, EmptyState, Field, FormActions, IconButton, ListRow, Menu, ScrollRegion } from "../../design-system";
import styles from "./SceneList.module.css";

const rowExitTransition = motionTransition("settle");

interface SceneListProps {
  scenes: ProgSceneView[];
  selectedName: string | null;
  onSelect: (name: string) => void;
  onCreate: (name: string, bars: number) => void;
  onRename: (oldName: string, newName: string) => void;
  onDelete: (name: string) => void;
  /** onReorder commits the dragged order upstream and reports whether the
   * server accepted it. A `false` result rolls the local optimistic order
   * back: the reset effect below deliberately only fires when the scene
   * *name set* changes, which a failed reorder never does, so without this
   * signal a rejected reorder left the list permanently wrong for the rest
   * of the session and the operator only found out on the next launch. */
  onReorder: (orderedNames: string[]) => boolean | Promise<boolean>;
}

/** reorderSceneNames applies a single drag-and-drop move to a name-ordered
 * array using dnd-kit's own arrayMove utility. Extracted as a pure function
 * (no hooks, no DOM) so the reorder logic itself is directly unit-testable
 * without fighting a simulated pointer-drag in jsdom -- see SceneList.test.tsx. */
export function reorderSceneNames(order: string[], activeName: string, overName: string): string[] {
  const oldIndex = order.indexOf(activeName);
  const newIndex = order.indexOf(overName);
  if (oldIndex === -1 || newIndex === -1 || oldIndex === newIndex) {
    return order;
  }
  return arrayMove(order, oldIndex, newIndex);
}

interface SortableSceneRowProps {
  scene: ProgSceneView;
  selectedName: string | null;
  isRenaming: boolean;
  renameValue: string;
  onSelect: (name: string) => void;
  onStartRename: (name: string) => void;
  onSaveRename: (name: string) => void;
  onCancelRename: () => void;
  onRenameValueChange: (value: string) => void;
  onDelete: (name: string) => void;
}

function SortableSceneRow({
  scene,
  selectedName,
  isRenaming,
  renameValue,
  onSelect,
  onStartRename,
  onSaveRename,
  onCancelRename,
  onRenameValueChange,
  onDelete,
}: SortableSceneRowProps) {
  const { attributes, listeners, setNodeRef, setActivatorNodeRef, transform, transition, isDragging } = useSortable({
    id: scene.name,
  });

  const style: CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  if (isRenaming) {
    return (
      <motion.li
        ref={setNodeRef}
        style={{ ...style, overflow: "hidden" }}
        initial={false}
        exit={{ opacity: 0, height: 0 }}
        transition={rowExitTransition}
        className={styles.renameRow}
      >
        <Field
          label="Scene name"
          value={renameValue}
          autoFocus
          onChange={(event) => onRenameValueChange(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter") onSaveRename(scene.name);
            if (event.key === "Escape") onCancelRename();
          }}
        />
        <IconButton icon={Check} label="Save" onClick={() => onSaveRename(scene.name)} />
        <IconButton icon={X} label="Cancel" onClick={onCancelRename} />
      </motion.li>
    );
  }

  return (
    <motion.li
      ref={setNodeRef}
      style={{ ...style, overflow: "hidden" }}
      initial={false}
      exit={{ opacity: 0, height: 0 }}
      transition={rowExitTransition}
      data-dragging={isDragging || undefined}
      className={scene.name === selectedName ? `${styles.sceneRow} ${styles.selected}` : styles.sceneRow}
    >
      <IconButton
        ref={setActivatorNodeRef}
        icon={GripVertical}
        label={`Reorder ${scene.name}`}
        size="compact"
        className={styles.dragHandle}
        {...attributes}
        {...listeners}
      />
      <ListRow
        label={scene.name}
        icon={Layers}
        meta={scene.active ? "LIVE" : `${scene.barsPerLoop}bar`}
        selected={scene.name === selectedName}
        onSelect={() => onSelect(scene.name)}
        actions={
          <Menu
            trigger={<IconButton icon={MoreVertical} label={`${scene.name} actions`} />}
            items={[
              {
                id: "rename",
                label: "Rename",
                icon: Pencil,
                onSelect: () => onStartRename(scene.name),
              },
              {
                id: "delete",
                label: "Delete",
                icon: Trash2,
                destructive: true,
                onSelect: () => onDelete(scene.name),
              },
            ]}
          />
        }
      />
    </motion.li>
  );
}

export default function SceneList({
  scenes,
  selectedName,
  onSelect,
  onCreate,
  onRename,
  onDelete,
  onReorder,
}: SceneListProps) {
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [bars, setBars] = useState("4");
  const [renamingName, setRenamingName] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState("");
  const [order, setOrder] = useState<string[]>(() => scenes.map((scene) => scene.name));

  // See the file-header doc comment: only reset local order when the
  // *identity set* of scene names changed, not on every `scenes` reference
  // change (refresh() fires after nearly every mutation in the parent
  // workspace, most of them unrelated to scenes at all).
  useEffect(() => {
    const incomingNames = scenes.map((scene) => scene.name);
    setOrder((current) => {
      const sameSet =
        current.length === incomingNames.length && incomingNames.every((sceneName) => current.includes(sceneName));
      return sameSet ? current : incomingNames;
    });
  }, [scenes]);

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const sceneByName = new Map(scenes.map((scene) => [scene.name, scene]));
  const orderedScenes = order
    .map((sceneName) => sceneByName.get(sceneName))
    .filter((scene): scene is ProgSceneView => scene !== undefined);

  const handleDragEnd = ({ active, over }: DragEndEvent) => {
    if (!over) return;
    // `next` is derived from `order` OUTSIDE the updater deliberately.
    // main.tsx wraps the app in React.StrictMode, which double-invokes
    // state updaters in development -- calling onReorder from inside one
    // issued two ReorderScenes calls per drag against the Go host, the
    // second computed against already-reordered server state if refresh()
    // landed between them.
    const next = reorderSceneNames(order, String(active.id), String(over.id));
    if (next === order) return;

    const previous = order;
    setOrder(next);
    void Promise.resolve(onReorder(next)).then((accepted) => {
      if (!accepted) {
        setOrder(previous);
      }
    });
  };

  const handleCreate = () => {
    const trimmed = name.trim();
    const parsedBars = Number.parseInt(bars, 10);
    if (trimmed === "" || Number.isNaN(parsedBars)) {
      return;
    }
    onCreate(trimmed, parsedBars);
    setName("");
    setCreating(false);
  };

  const handleStartRename = (sceneName: string) => {
    setRenamingName(sceneName);
    setRenameValue(sceneName);
  };

  const handleSaveRename = (sceneName: string) => {
    const trimmed = renameValue.trim();
    if (trimmed === "" || trimmed === sceneName) {
      setRenamingName(null);
      return;
    }
    onRename(sceneName, trimmed);
    setRenamingName(null);
  };

  // Destructive confirmations go through ConfirmDialog (the design
  // system's public confirmation contract), not window.confirm: in a
  // Wails webview the native dialog blocks the JS thread and renders
  // unstyled chrome outside the app's own focus/return-focus contract.
  const [pendingDelete, setPendingDelete] = useState<string | null>(null);

  const handleDelete = (sceneName: string) => {
    setPendingDelete(sceneName);
  };

  return (
    <div className={styles.column}>
      <div className={styles.header}>
        <span className={styles.label}>Scenes</span>
        <Button variant="secondary" leadingIcon={creating ? X : Plus} onClick={() => setCreating((current) => !current)}>
          {creating ? "Cancel" : "New"}
        </Button>
      </div>

      {creating ? (
        <div className={styles.createForm}>
          <Field label="New scene name" type="text" value={name} onChange={(event) => setName(event.target.value)} />
          <Field label="Bars per loop" type="number" min={1} value={bars} onChange={(event) => setBars(event.target.value)} />
          <FormActions>
            <Button variant="primary" leadingIcon={Check} onClick={handleCreate}>
              Create
            </Button>
          </FormActions>
        </div>
      ) : null}

      <ScrollRegion>
        {scenes.length === 0 ? (
          <EmptyState icon={Layers}>No scenes yet — create one above.</EmptyState>
        ) : (
          <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
            <SortableContext items={orderedScenes.map((scene) => scene.name)} strategy={verticalListSortingStrategy}>
              <ul className={styles.list} aria-label="Scene list">
                <AnimatePresence initial={false}>
                  {orderedScenes.map((scene) => (
                    <SortableSceneRow
                      key={scene.name}
                      scene={scene}
                      selectedName={selectedName}
                      isRenaming={renamingName === scene.name}
                      renameValue={renameValue}
                      onSelect={onSelect}
                      onStartRename={handleStartRename}
                      onSaveRename={handleSaveRename}
                      onCancelRename={() => setRenamingName(null)}
                      onRenameValueChange={setRenameValue}
                      onDelete={handleDelete}
                    />
                  ))}
                </AnimatePresence>
              </ul>
            </SortableContext>
          </DndContext>
        )}
      </ScrollRegion>

      <ConfirmDialog
        open={pendingDelete !== null}
        title="Delete scene?"
        message={`This permanently deletes the scene "${pendingDelete ?? ""}" and its layer assignments.`}
        confirmLabel="Delete Scene"
        destructive
        onConfirm={() => {
          if (pendingDelete) onDelete(pendingDelete);
          setPendingDelete(null);
        }}
        onCancel={() => setPendingDelete(null)}
      />
    </div>
  );
}
