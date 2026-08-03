// SceneList is the Scene Stack's primary programming navigator
// (programming-scene-authoring.md): the scene list is the left-column
// nav; selecting a scene changes the layer rows shown alongside it. The
// create-scene form is a toggled inline reveal, not permanently visible
// (the sketch's own "what to avoid": permanently displaying every
// look-creation form).
import { useState } from "react";
import { Plus, X, Check, Layers, Pencil, Trash2 } from "lucide-react";

import type { ProgSceneView } from "../../lib/wailsBridge";
import { Button, EmptyState, Field, FormActions, IconButton, ListRow, ScrollRegion } from "../../design-system";
import styles from "./SceneList.module.css";

interface SceneListProps {
  scenes: ProgSceneView[];
  selectedName: string | null;
  onSelect: (name: string) => void;
  onCreate: (name: string, bars: number) => void;
  onRename: (oldName: string, newName: string) => void;
  onDelete: (name: string) => void;
}

export default function SceneList({
  scenes,
  selectedName,
  onSelect,
  onCreate,
  onRename,
  onDelete,
}: SceneListProps) {
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [bars, setBars] = useState("4");
  const [renamingName, setRenamingName] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState("");

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
          <ul className={styles.list} aria-label="Scene list">
            {scenes.map((scene) =>
              renamingName === scene.name ? (
                <li key={scene.name} className={styles.renameRow}>
                  <Field
                    label="Scene name"
                    value={renameValue}
                    autoFocus
                    onChange={(event) => setRenameValue(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter") handleSaveRename(scene.name);
                      if (event.key === "Escape") setRenamingName(null);
                    }}
                  />
                  <IconButton icon={Check} label="Save" onClick={() => handleSaveRename(scene.name)} />
                  <IconButton icon={X} label="Cancel" onClick={() => setRenamingName(null)} />
                </li>
              ) : (
                <li
                  key={scene.name}
                  className={
                    scene.name === selectedName ? `${styles.sceneRow} ${styles.selected}` : styles.sceneRow
                  }
                >
                  <ListRow
                    label={scene.name}
                    icon={Layers}
                    meta={scene.active ? "LIVE" : `${scene.barsPerLoop}bar`}
                    selected={scene.name === selectedName}
                    onSelect={() => onSelect(scene.name)}
                    actions={
                      <span className={styles.rowActions}>
                        <IconButton icon={Pencil} label={`Rename ${scene.name}`} onClick={() => handleStartRename(scene.name)} />
                        <IconButton icon={Trash2} variant="destructive" label={`Delete ${scene.name}`} onClick={() => handleDelete(scene.name)} />
                      </span>
                    }
                  />
                </li>
              ),
            )}
          </ul>
        )}
      </ScrollRegion>
    </div>
  );
}
