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
// contract -- SceneList itself never calls wailsBridge directly). `order`
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
import { useEffect, useState, type CSSProperties } from "react";
import { Plus, X, Check, Layers, MoreVertical, Pencil, Trash2, GripVertical } from "lucide-react";
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
import { Button, EmptyState, Field, FormActions, IconButton, ListRow, Menu, ScrollRegion } from "../../design-system";
import styles from "./SceneList.module.css";

interface SceneListProps {
  scenes: ProgSceneView[];
  selectedName: string | null;
  onSelect: (name: string) => void;
  onCreate: (name: string, bars: number) => void;
  onRename: (oldName: string, newName: string) => void;
  onDelete: (name: string) => void;
  onReorder: (orderedNames: string[]) => void;
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
      <li ref={setNodeRef} style={style} className={styles.renameRow}>
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
      </li>
    );
  }

  return (
    <li
      ref={setNodeRef}
      style={style}
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
    </li>
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
    setOrder((current) => {
      const next = reorderSceneNames(current, String(active.id), String(over.id));
      if (next !== current) {
        onReorder(next);
      }
      return next;
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

  const handleDelete = (sceneName: string) => {
    if (window.confirm(`Delete scene "${sceneName}"?`)) {
      onDelete(sceneName);
    }
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
              </ul>
            </SortableContext>
          </DndContext>
        )}
      </ScrollRegion>
    </div>
  );
}
