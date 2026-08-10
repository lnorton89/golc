// DeskMappingsSection.tsx is MidiPanel.tsx's own "Desk mappings" section:
// every direct Desk-fader<->MIDI mapping (internal/deskmidi), independent
// of Operator Surfaces entirely -- unlike the surface mapping list above it
// in MidiPanel.tsx, this section is always visible, never gated behind
// picking a surface from the selector, since a desk mapping has no surface
// to select in the first place. Learning/remapping itself happens from the
// Desk workspace's own click-to-map overlay (Desk.tsx/Fader.tsx); this
// section is read-plus-delete only, mirroring MidiPanel.tsx's own
// "Remove" affordance and destructive-confirmation convention exactly.
//
// Phase 13 Plan 28 migrated this section onto shared design-system
// primitives (PanelHeader/ListRow/EmptyState/ErrorState/LoadingState/
// IconButton); every Wails call and dispatch path above is unchanged.
import { useCallback, useEffect, useState } from "react";
import { Music2, Trash2 } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";

import {
  errorMessage,
  getMidiService,
  listLocalFixtures,
  listPatch,
  type DeskMidiMappingView,
  type FixtureLibraryRowView,
  type PatchView,
} from "../../lib/wailsBridge";
import { resolveDeskChannelLabel } from "../Desk/deskLabels";
import { motionTransition } from "../../design-system/motion";
import { EmptyState, ErrorState, IconButton, ListRow, LoadingState, PanelHeader } from "../../design-system";
import styles from "./MidiPanel.module.css";

const rowExitTransition = motionTransition("settle");

// The MidiService binding and DeskMidiMappingView are declared once,
// centrally, in src/lib/wailsBridge.ts, which owns every window.go read.
// This section uses only the two desk-mapping methods it needs
// (ListDeskMappings/RemoveDeskMapping); Desk.tsx uses the learn methods
// off the same single declaration.
const deskMidiService = getMidiService;

function mappingTechnical(mapping: DeskMidiMappingView): string {
  const kindLabel = mapping.kind === "note" ? "Note" : "CC";
  return `${kindLabel} ${mapping.number} · ch ${mapping.channel}`;
}

export default function DeskMappingsSection() {
  const [mappings, setMappings] = useState<DeskMidiMappingView[]>([]);
  const [patch, setPatch] = useState<PatchView | null>(null);
  const [library, setLibrary] = useState<FixtureLibraryRowView[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async (): Promise<void> => {
    const svc = deskMidiService();
    if (!svc) {
      setLoading(false);
      return;
    }
    try {
      const [mappingRows, patchView, libraryView] = await Promise.all([
        svc.ListDeskMappings(),
        listPatch(),
        listLocalFixtures(),
      ]);
      setMappings(mappingRows);
      setPatch(patchView);
      setLibrary(libraryView.rows);
      setError(null);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const handleRemove = async (mapping: DeskMidiMappingView) => {
    const svc = deskMidiService();
    if (!svc) return;
    const label = resolveDeskChannelLabel(patch, library, mapping.instanceId, mapping.capability);
    // Mirrors MidiPanel.tsx's own 06-UI-SPEC.md destructive-confirmation
    // convention for Remove Mapping.
    const confirmed = window.confirm(`Remove Mapping: This unassigns ${mappingTechnical(mapping)} from ${label}.`);
    if (!confirmed) return;
    try {
      const result = await svc.RemoveDeskMapping(mapping.id);
      if (result.exitCode !== 0) {
        throw new Error(result.stderr || "RemoveDeskMapping failed");
      }
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  return (
    <div className={styles.mappingSection}>
      <PanelHeader
        label="Desk mappings"
        info="Every Desk fader mapped directly to a MIDI control via the global MIDI Learn toggle, independent of Operator Surfaces."
      />
      {error && <ErrorState heading="Desk mappings unavailable" message={error} variant="inline" />}
      {loading ? (
        <LoadingState label="Loading Desk mappings" variant="inline" />
      ) : mappings.length === 0 ? (
        <EmptyState
          icon={Music2}
          heading="No Desk mappings yet"
          body="Turn on MIDI Learn (top of the window) and click a fader in the Desk workspace, then move or press the matching hardware control."
        />
      ) : (
        <ul className={styles.mappingList} aria-label="Desk MIDI mappings">
          <AnimatePresence initial={false}>
            {mappings.map((mapping) => {
              const label = resolveDeskChannelLabel(patch, library, mapping.instanceId, mapping.capability);
              return (
                <motion.li
                  key={mapping.id}
                  style={{ overflow: "hidden" }}
                  initial={false}
                  exit={{ opacity: 0, height: 0 }}
                  transition={rowExitTransition}
                >
                  <ListRow
                    label={label}
                    meta={mappingTechnical(mapping)}
                    actions={
                      <IconButton
                        icon={Trash2}
                        label={`Remove mapping from ${label}`}
                        variant="destructive"
                        size="compact"
                        onClick={() => handleRemove(mapping)}
                      />
                    }
                  />
                </motion.li>
              );
            })}
          </AnimatePresence>
        </ul>
      )}
    </div>
  );
}
