// SceneList is the Scene Stack's primary programming navigator
// (programming-scene-authoring.md): the scene list is the left-column
// nav; selecting a scene changes the layer rows shown alongside it. The
// create-scene form is a toggled inline reveal, not permanently visible
// (the sketch's own "what to avoid": permanently displaying every
// look-creation form).
import { useState } from "react";
import { Plus, X, Check, Layers, Pencil, Trash2 } from "lucide-react";

import type { ProgSceneView } from "../../lib/wailsBridge";
import Button from "../primitives/Button/Button";
import ListRow from "../primitives/ListRow/ListRow";
import ScrollRegion from "../primitives/ScrollRegion/ScrollRegion";
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
        <Button variant="secondary" icon={creating ? X : Plus} onClick={() => setCreating((current) => !current)}>
          {creating ? "Cancel" : "+ New"}
        </Button>
      </div>

      {creating ? (
        <div className={styles.createForm}>
          <input
            className={styles.input}
            type="text"
            value={name}
            placeholder="Scene name"
            aria-label="New scene name"
            onChange={(event) => setName(event.target.value)}
          />
          <input
            className={styles.inputNarrow}
            type="number"
            min={1}
            value={bars}
            aria-label="Bars per loop"
            onChange={(event) => setBars(event.target.value)}
          />
          <Button variant="primary" icon={Check} onClick={handleCreate}>
            Create
          </Button>
        </div>
      ) : null}

      <ScrollRegion>
        {scenes.length === 0 ? (
          <p className={styles.emptyState}>
            <Layers size={14} aria-hidden="true" />
            No scenes yet — create one above.
          </p>
        ) : (
          <ul className={styles.list} aria-label="Scene list">
            {scenes.map((scene) =>
              renamingName === scene.name ? (
                <li key={scene.name} className={styles.renameRow}>
                  <input
                    className={styles.input}
                    type="text"
                    value={renameValue}
                    aria-label="Scene name"
                    autoFocus
                    onChange={(event) => setRenameValue(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter") handleSaveRename(scene.name);
                      if (event.key === "Escape") setRenamingName(null);
                    }}
                  />
                  <Button variant="secondary" icon={Check} onClick={() => handleSaveRename(scene.name)} aria-label="Save" />
                  <Button variant="secondary" icon={X} onClick={() => setRenamingName(null)} aria-label="Cancel" />
                </li>
              ) : (
                <li key={scene.name} className={styles.sceneRow}>
                  <ListRow
                    label={scene.name}
                    icon={Layers}
                    meta={scene.active ? "LIVE" : `${scene.barsPerLoop}bar`}
                    selected={scene.name === selectedName}
                    onSelect={() => onSelect(scene.name)}
                    actions={
                      <span className={styles.rowActions}>
                        <Button
                          variant="secondary"
                          icon={Pencil}
                          onClick={() => handleStartRename(scene.name)}
                          aria-label={`Rename ${scene.name}`}
                        />
                        <Button
                          variant="destructive"
                          icon={Trash2}
                          onClick={() => handleDelete(scene.name)}
                          aria-label={`Delete ${scene.name}`}
                        />
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
