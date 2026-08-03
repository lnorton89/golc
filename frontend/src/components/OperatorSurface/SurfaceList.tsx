// SurfaceList.tsx renders GOLC's multiple independently named operator
// surfaces (06-07-PLAN.md Task 2, D-02): a "Create Operator Surface" form
// (Copywriting Contract primary CTA), the 06-UI-SPEC.md empty state ("No
// operator surfaces yet" + body) when there are none, and otherwise a
// zero-one-many-safe row list (each row: name, assigned scene/layer/master
// count, MIDI-mapping count) inside a bounded ScrollRegion with the
// active/selected row highlighted and ellipsis-truncated, tooltip-bearing
// names (long-text). This is purely presentational -- all SurfaceService
// calls and state live in OperatorSurface.tsx, the component that mounts
// this one.
//
// Phase 13 (unified design system, 13-14-PLAN.md Task 1) retargets the
// create form onto Field/Button/FormActions, the row list onto ListRow
// (selected/meta/actions), and the empty state onto EmptyState -- following
// the exact template SceneList.tsx (frontend/src/components/SceneProgramming)
// already established for this same "create form + removable row list"
// shape, including its window.confirm-before-destructive-remove pattern.
import { useState } from "react";
import { Plus, SlidersHorizontal, Trash2 } from "lucide-react";

import { Button, EmptyState, Field, FormActions, IconButton, ListRow, ScrollRegion } from "../../design-system";
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

  const handleRemove = (surface: SurfaceSummary) => {
    const confirmed = window.confirm(
      `Remove Operator Surface: This deletes ${surface.name} and all its scene/layer/master assignments and MIDI mappings. This can't be undone.`,
    );
    if (confirmed) {
      onRemove(surface.name);
    }
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
    <div className={styles.column}>
      <div className={styles.createRow}>
        <Field
          label="New operator surface name"
          value={draftName}
          placeholder="New operator surface name"
          onChange={(event) => setDraftName(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              handleCreate();
            }
          }}
        />
        <FormActions>
          <Button variant="primary" leadingIcon={Plus} onClick={handleCreate}>
            Create Operator Surface
          </Button>
        </FormActions>
      </div>

      {surfaces.length === 0 ? (
        <EmptyState
          icon={SlidersHorizontal}
          heading="No operator surfaces yet"
          body="Build one by assigning scenes, layers, and masters from the authoring view, then hand it to your operator."
        />
      ) : (
        <>
          <p className={styles.countSummary}>
            {surfaces.length} operator surface{surfaces.length === 1 ? "" : "s"}
          </p>
          <ScrollRegion aria-label="Operator surface list">
            <ul className={styles.list}>
              {surfaces.map((surface) => {
                const isSelected = surface.name === selectedName;
                return (
                  <li key={surface.id}>
                    <ListRow
                      label={surface.name}
                      meta={`${assignedLabel(surface)} - ${midiLabel(surface)}`}
                      selected={isSelected}
                      onSelect={() => onSelect(surface.name)}
                      actions={
                        <IconButton
                          icon={Trash2}
                          variant="destructive"
                          label={`Remove ${surface.name}`}
                          onClick={() => handleRemove(surface)}
                        />
                      }
                    />
                  </li>
                );
              })}
            </ul>
          </ScrollRegion>
        </>
      )}
    </div>
  );
}
