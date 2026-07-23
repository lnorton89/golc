// SurfaceList.tsx renders GOLC's multiple independently named operator
// surfaces (06-07-PLAN.md Task 2, D-02): a "Create Operator Surface" form
// (Copywriting Contract primary CTA), the 06-UI-SPEC.md empty state ("No
// operator surfaces yet" + body) when there are none, and otherwise a
// zero-one-many-safe row list (each row: name, assigned scene/layer/master
// count, MIDI-mapping count) inside a fixed-height scroll panel (overflow
// backstop) with the active/selected row in the Signal Blue accent and
// ellipsis-truncated, tooltip-bearing names (long-text). This is purely
// presentational -- all SurfaceService calls and state live in
// OperatorSurface.tsx, the component that mounts this one.

import { useState } from "react";

import styles from "./OperatorSurface.module.css";
import type { SurfaceSummary } from "./OperatorSurface";

interface SurfaceListProps {
  surfaces: SurfaceSummary[];
  selectedName: string | null;
  onSelect: (name: string) => void;
  onCreate: (name: string) => void;
  onRemove: (name: string) => void;
}

export default function SurfaceList({
  surfaces,
  selectedName,
  onSelect,
  onCreate,
  onRemove,
}: SurfaceListProps) {
  const [draftName, setDraftName] = useState("");

  const handleCreate = () => {
    const trimmed = draftName.trim();
    if (trimmed === "") {
      return;
    }
    onCreate(trimmed);
    setDraftName("");
  };

  const assignedLabel = (surface: SurfaceSummary) =>
    `${surface.assignedCount} scene/layer/master assignment${
      surface.assignedCount === 1 ? "" : "s"
    }`;
  const midiLabel = (surface: SurfaceSummary) =>
    `${surface.midiMappingCount} MIDI mapping${
      surface.midiMappingCount === 1 ? "" : "s"
    }`;

  return (
    <div>
      <div className={styles.createRow}>
        <input
          className={styles.createInput}
          type="text"
          value={draftName}
          placeholder="New operator surface name"
          onChange={(event) => setDraftName(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              handleCreate();
            }
          }}
          aria-label="New operator surface name"
        />
        <button type="button" className={styles.primaryButton} onClick={handleCreate}>
          Create Operator Surface
        </button>
      </div>

      {surfaces.length === 0 ? (
        <div className={styles.emptyState}>
          <p className={styles.emptyHeading}>No operator surfaces yet</p>
          <p className={styles.emptyBody}>
            Build one by assigning scenes, layers, and masters from the authoring
            view, then hand it to your operator.
          </p>
        </div>
      ) : (
        <>
          <p className={styles.countSummary}>
            {surfaces.length} operator surface{surfaces.length === 1 ? "" : "s"}
          </p>
          <ul className={styles.rowScroll} aria-label="Operator surface list">
            {surfaces.map((surface) => {
              const isSelected = surface.name === selectedName;
              return (
                <li
                  key={surface.id}
                  className={`${styles.row} ${isSelected ? styles.rowSelected : ""}`}
                >
                  <button
                    type="button"
                    className={styles.rowSelectButton}
                    onClick={() => onSelect(surface.name)}
                    title={surface.name}
                    aria-pressed={isSelected}
                  >
                    <span className={styles.rowName}>{surface.name}</span>
                    <span className={styles.rowCounts}>
                      {assignedLabel(surface)} - {midiLabel(surface)}
                    </span>
                  </button>
                  <button
                    type="button"
                    className={styles.removeButton}
                    onClick={() => {
                      const confirmed = window.confirm(
                        `Remove Operator Surface: This deletes ${surface.name} and all its scene/layer/master assignments and MIDI mappings. This can't be undone.`,
                      );
                      if (confirmed) {
                        onRemove(surface.name);
                      }
                    }}
                    aria-label={`Remove ${surface.name}`}
                  >
                    Remove
                  </button>
                </li>
              );
            })}
          </ul>
        </>
      )}
    </div>
  );
}
