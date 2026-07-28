// FixtureLibraryWorkspace replaces the ComingSoon stub that pointed
// operators at "golc fixture" from the command line (09-01-PLAN.md,
// FDUI-01's first user-visible slice, CONTEXT D-01's local half): a real
// browse-and-inspect surface backed by FixtureLibraryService
// (internal/wails/svc_fixturelibrary.go). ListLocal projects the single
// extracted internal/fixture.ListDirectory scan into rows -- it decodes
// and pins nothing on this side, so a fixture the CLI would reject can
// never render as usable here (T-09-01-03). This file follows
// SaveRecoveryWorkspace.tsx's load/refresh/error skeleton and
// ScriptsWorkspace.tsx's list + Chip status composition -- the correct
// structural templates per 09-PATTERNS.md, not a bespoke shape.
//
// This first cut (Task 2 of 09-01-PLAN.md) delivers the LIST only: every
// local fixture, its manufacturer, and a validation-status chip. Search
// and the inline inspect panel are added by Task 3.
import { useCallback, useEffect, useState } from "react";
import { Lightbulb } from "lucide-react";

import { errorMessage, listLocalFixtures, type FixtureLibraryRowView } from "../../lib/wailsBridge";
import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import Panel from "../../components/primitives/Panel/Panel";
import PanelHeader from "../../components/primitives/PanelHeader/PanelHeader";
import ScrollRegion from "../../components/primitives/ScrollRegion/ScrollRegion";
import EmptyState from "../../components/primitives/EmptyState/EmptyState";
import ListRow from "../../components/primitives/ListRow/ListRow";
import Chip from "../../components/primitives/Chip/Chip";
import styles from "./FixtureLibraryWorkspace.module.css";

export default function FixtureLibraryWorkspace() {
  const [rows, setRows] = useState<FixtureLibraryRowView[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async (): Promise<void> => {
    try {
      const view = await listLocalFixtures();
      setRows(view.rows);
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

  const countLabel = `${rows.length} fixture${rows.length === 1 ? "" : "s"}`;

  return (
    <div className={styles.workspace}>
      <Toolbar title="Fixture Library" icon={Lightbulb} />
      <div className={styles.canvas}>
        {loading ? (
          <p className={styles.loading}>Loading fixture library…</p>
        ) : (
          <>
            {error ? <p className={styles.errorText}>{error}</p> : null}
            <div className={styles.layout}>
              <Panel>
                <PanelHeader label={`Fixture Library (${countLabel})`} icon={Lightbulb} />
                <ScrollRegion>
                  {rows.length === 0 ? (
                    <EmptyState icon={Lightbulb}>
                      <span className={styles.emptyHeading}>No fixtures yet</span>
                      <span className={styles.emptyBody}>
                        Import a fixture from the Open Fixture Library or add your own YAML definition to get
                        started.
                      </span>
                    </EmptyState>
                  ) : (
                    <ul className={styles.list} aria-label="Fixture library">
                      {rows.map((row) => (
                        <li key={row.stableKey}>
                          <ListRow
                            label={row.model || row.fileName}
                            meta={
                              <span className={styles.rowMeta}>
                                {row.manufacturer ? (
                                  <span className={styles.manufacturer}>{row.manufacturer}</span>
                                ) : null}
                                <Chip tone={row.status === "valid" ? "frame-lock" : "revoked"}>
                                  {row.status === "valid" ? "Valid" : "Invalid"}
                                </Chip>
                              </span>
                            }
                          />
                        </li>
                      ))}
                    </ul>
                  )}
                </ScrollRegion>
              </Panel>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
