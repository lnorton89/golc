// DeskMappingsSection.tsx is MidiPanel.tsx's own "Desk mappings" section:
// every direct Desk-fader<->MIDI mapping (internal/deskmidi), independent
// of Operator Surfaces entirely -- unlike the surface mapping list above it
// in MidiPanel.tsx, this section is always visible, never gated behind
// picking a surface from the selector, since a desk mapping has no surface
// to select in the first place. Learning/remapping itself happens from the
// Desk workspace's own click-to-map overlay (Desk.tsx/Fader.tsx); this
// section is read-plus-delete only, mirroring MidiPanel.tsx's own
// "Remove" affordance and destructive-confirmation convention exactly.
import { useCallback, useEffect, useState } from "react";
import { Music2, Trash2 } from "lucide-react";

import { errorMessage, listLocalFixtures, listPatch, type FixtureLibraryRowView, type PatchView } from "../../lib/wailsBridge";
import { resolveDeskChannelLabel } from "../Desk/deskLabels";
import InfoTooltip from "../primitives/InfoTooltip/InfoTooltip";
import styles from "./MidiPanel.module.css";

export interface DeskMidiMappingView {
  id: string;
  channel: number;
  kind: "note" | "control_change";
  number: number;
  instanceId: string;
  capability: string;
}

interface DeskGoResult {
  exitCode: number;
  stdout: string;
  stderr: string;
}

interface DeskMidiServiceBinding {
  RemoveDeskMapping(mappingId: string): Promise<DeskGoResult>;
  ListDeskMappings(): Promise<DeskMidiMappingView[]>;
}

function deskMidiService(): DeskMidiServiceBinding | undefined {
  return window.go?.wails?.MidiService as unknown as DeskMidiServiceBinding | undefined;
}

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
      <div className={styles.sectionHeadingRow}>
        <h3 className={styles.sectionHeading}>Desk mappings</h3>
        <InfoTooltip
          label="About Desk mappings"
          text="Every Desk fader mapped directly to a MIDI control via the global MIDI Learn toggle, independent of Operator Surfaces."
        />
      </div>
      {error && <p className={styles.errorText}>{error}</p>}
      {loading ? (
        <div className={styles.skeleton}>Loading Desk mappings…</div>
      ) : mappings.length === 0 ? (
        <div className={styles.emptyState}>
          <p className={styles.emptyHeading}>
            <Music2 size={18} aria-hidden="true" />
            No Desk mappings yet
          </p>
          <p className={styles.emptyBody}>
            Turn on MIDI Learn (top of the window) and click a fader in the Desk workspace, then
            move or press the matching hardware control.
          </p>
        </div>
      ) : (
        <ul className={styles.mappingList} aria-label="Desk MIDI mappings">
          {mappings.map((mapping) => {
            const label = resolveDeskChannelLabel(patch, library, mapping.instanceId, mapping.capability);
            return (
              <li key={mapping.id} className={styles.mappingRow}>
                <div className={styles.mappingInfo}>
                  <span className={styles.mappingLabel} title={label}>
                    {label}
                  </span>
                  <span className={styles.mappingTechnical}>{mappingTechnical(mapping)}</span>
                </div>
                <button
                  type="button"
                  className={styles.removeButton}
                  onClick={() => handleRemove(mapping)}
                  aria-label={`Remove mapping from ${label}`}
                >
                  <Trash2 size={13} aria-hidden="true" />
                  Remove
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
