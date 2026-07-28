// FixtureLibraryWorkspace replaces the ComingSoon stub that pointed
// operators at "golc fixture" from the command line (09-01-PLAN.md,
// FDUI-01's first user-visible slice, CONTEXT D-01 local half/D-02/D-03):
// a real browse-and-inspect surface backed by FixtureLibraryService
// (internal/wails/svc_fixturelibrary.go). ListLocal projects the single
// extracted internal/fixture.ListDirectory scan into rows -- it decodes
// and pins nothing on this side, so a fixture the CLI would reject can
// never render as usable here (T-09-01-03). This file follows
// SaveRecoveryWorkspace.tsx's load/refresh/error skeleton and
// ScriptsWorkspace.tsx's list + Chip status composition -- the correct
// structural templates per 09-PATTERNS.md, not a bespoke shape.
//
// Task 2 of 09-01-PLAN.md delivered the LIST only. Task 3 of that same
// plan added client-side text search (D-03: basic name/manufacturer
// substring match, no faceted filtering, no backend round-trip) and the
// inline inspect panel (D-02: selecting a row calls Inspect and renders
// its identity/provenance/validation/warnings in a second Panel inside
// this same workspace -- never a dialog, modal, or wizard step).
//
// 09-05-PLAN.md's Task 3 adds the source toggle ("My Library" / "Open
// Fixture Library") and the catalog search side: the same search input
// debounces and calls FixtureLibraryService.SearchOFL, rendering the
// UI-SPEC's exact empty/no-results/unreachable copy. This version
// searches OFL manufacturer names only -- no full fixture-model index is
// reachable from the single SSRF-guarded host the app allows (09-RESEARCH
// Open Question 1) -- so a permanently visible note states that scope
// next to the results; plan 09-06 adds the fixture-key field and the
// import action.
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Lightbulb } from "lucide-react";

import {
  errorMessage,
  inspectFixtureFile,
  listLocalFixtures,
  searchOflManufacturers,
  type FixtureInspectView,
  type FixtureLibraryRowView,
  type OflSearchView,
} from "../../lib/wailsBridge";
import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import Panel from "../../components/primitives/Panel/Panel";
import PanelHeader from "../../components/primitives/PanelHeader/PanelHeader";
import ScrollRegion from "../../components/primitives/ScrollRegion/ScrollRegion";
import EmptyState from "../../components/primitives/EmptyState/EmptyState";
import ListRow from "../../components/primitives/ListRow/ListRow";
import Chip from "../../components/primitives/Chip/Chip";
import styles from "./FixtureLibraryWorkspace.module.css";

// oflSearchDebounceMs is the client-side debounce D-01/T-09-05-03 require
// so a typing burst issues at most one FixtureLibraryService.SearchOFL
// call, not one per keystroke.
const oflSearchDebounceMs = 250;

// matchesSearch performs D-03's basic client-side text search: a
// case-insensitive substring test against manufacturer OR model, over the
// already-fetched rows -- no backend round-trip, no faceted filter
// controls.
function matchesSearch(row: FixtureLibraryRowView, query: string): boolean {
  if (query.trim() === "") return true;
  const needle = query.trim().toLowerCase();
  return row.manufacturer.toLowerCase().includes(needle) || row.model.toLowerCase().includes(needle);
}

type LibrarySource = "local" | "catalog";

export default function FixtureLibraryWorkspace() {
  const [directory, setDirectory] = useState("");
  const [rows, setRows] = useState<FixtureLibraryRowView[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");

  const [selectedFileName, setSelectedFileName] = useState<string | null>(null);
  const [inspecting, setInspecting] = useState(false);
  const [inspectView, setInspectView] = useState<FixtureInspectView | null>(null);

  // source (D-01): "My Library" (the local fixtures directory, unchanged
  // from 09-01-PLAN.md) vs. "Open Fixture Library" (this plan's catalog
  // manufacturer-name search). The same search input serves both sides.
  const [source, setSource] = useState<LibrarySource>("local");
  const [catalogView, setCatalogView] = useState<OflSearchView | null>(null);
  const [catalogSearching, setCatalogSearching] = useState(false);
  const catalogRequestRef = useRef(0);

  const refresh = useCallback(async (): Promise<void> => {
    try {
      const view = await listLocalFixtures();
      setDirectory(view.directory);
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

  // Debounced catalog search (T-09-05-03): at most one SearchOFL call per
  // ~250ms typing pause, and only while the catalog side is active. An
  // empty/whitespace-only query never calls the bridge at all -- the
  // empty-prompt copy renders immediately, with no in-flight state.
  useEffect(() => {
    if (source !== "catalog") return;
    const query = search.trim();
    if (query === "") {
      setCatalogView(null);
      setCatalogSearching(false);
      return;
    }

    setCatalogSearching(true);
    const requestId = catalogRequestRef.current + 1;
    catalogRequestRef.current = requestId;
    const timer = setTimeout(() => {
      void (async () => {
        const view = await searchOflManufacturers(query);
        if (catalogRequestRef.current !== requestId) return; // a newer query superseded this one
        setCatalogView(view);
        setCatalogSearching(false);
      })();
    }, oflSearchDebounceMs);

    return () => clearTimeout(timer);
  }, [source, search]);

  const filteredRows = useMemo(() => rows.filter((row) => matchesSearch(row, search)), [rows, search]);

  const countLabel = `${rows.length} fixture${rows.length === 1 ? "" : "s"}`;
  const trimmedQuery = search.trim();

  // handleSelectRow calls Inspect against the row's file resolved under
  // the library's own directory (as ListLocal reported it) -- never a
  // second decode/pin/normalize implementation on this side (D-02,
  // T-09-01-03).
  const handleSelectRow = (row: FixtureLibraryRowView) => {
    setSelectedFileName(row.fileName);
    setInspecting(true);
    setInspectView(null);
    const path = directory ? `${directory}/${row.fileName}` : row.fileName;
    void (async () => {
      try {
        setInspectView(await inspectFixtureFile(path));
      } catch (err) {
        setError(errorMessage(err));
      } finally {
        setInspecting(false);
      }
    })();
  };

  return (
    <div className={styles.workspace}>
      <Toolbar title="Fixture Library" icon={Lightbulb} />
      <div className={styles.canvas}>
        {loading ? (
          <p className={styles.loading}>Loading fixture library…</p>
        ) : (
          <>
            {error ? <p className={styles.errorText}>{error}</p> : null}
            <div className={styles.sourceToggle} role="group" aria-label="Fixture source">
              <button
                type="button"
                className={source === "local" ? styles.sourceButtonActive : styles.sourceButton}
                aria-pressed={source === "local"}
                onClick={() => setSource("local")}
              >
                My Library
              </button>
              <button
                type="button"
                className={source === "catalog" ? styles.sourceButtonActive : styles.sourceButton}
                aria-pressed={source === "catalog"}
                onClick={() => setSource("catalog")}
              >
                Open Fixture Library
              </button>
            </div>
            <input
              className={styles.searchInput}
              type="text"
              value={search}
              placeholder="Search fixtures by name or manufacturer…"
              aria-label="Search fixtures"
              onChange={(event) => setSearch(event.target.value)}
            />
            <div className={styles.layout}>
              {source === "local" ? (
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
                        {filteredRows.map((row) => (
                          <li key={row.stableKey}>
                            <ListRow
                              label={row.model || row.fileName}
                              selected={row.fileName === selectedFileName}
                              onSelect={() => handleSelectRow(row)}
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
              ) : (
                <Panel>
                  <PanelHeader label="Open Fixture Library" icon={Lightbulb} />
                  <ScrollRegion>
                    {trimmedQuery === "" ? (
                      <EmptyState icon={Lightbulb}>
                        Search the Open Fixture Library by name or manufacturer.
                      </EmptyState>
                    ) : catalogView?.unreachable ? (
                      <div className={styles.catalogUnreachable}>
                        <Chip tone="offline">Unreachable</Chip>
                        <p className={styles.errorText}>
                          Can&apos;t reach the Open Fixture Library. Check your network connection and try again.
                        </p>
                      </div>
                    ) : catalogView && catalogView.manufacturers.length === 0 ? (
                      <EmptyState icon={Lightbulb}>
                        {`No fixtures matched "${trimmedQuery}". Try a different name or manufacturer.`}
                      </EmptyState>
                    ) : catalogView ? (
                      <ul className={styles.list} aria-label="Open Fixture Library manufacturers">
                        {catalogView.manufacturers.map((manufacturer) => (
                          <li key={manufacturer.key}>
                            <ListRow label={manufacturer.name} meta={manufacturer.key} />
                          </li>
                        ))}
                      </ul>
                    ) : catalogSearching ? (
                      <p className={styles.loading} aria-live="polite">
                        Searching…
                      </p>
                    ) : null}
                  </ScrollRegion>
                </Panel>
              )}

              {source === "local" ? (
                <Panel className={styles.inspectPanel}>
                  <PanelHeader label="Inspect" />
                  {!selectedFileName ? (
                    <EmptyState icon={Lightbulb}>
                      Select a fixture to see its identity and validation status.
                    </EmptyState>
                  ) : inspecting ? (
                    <p className={styles.loading}>Inspecting…</p>
                  ) : inspectView ? (
                    <div className={styles.inspectBody}>
                      <div
                        className={styles.techReadout}
                      >{`${inspectView.stableKey || "—"} · ${inspectView.contentHash || "—"}`}</div>
                      <div className={styles.techReadout}>
                        {`Schema ${inspectView.schemaVersion} · Revision ${inspectView.revision || "—"}`}
                      </div>
                      <Chip tone={inspectView.valid ? "frame-lock" : "revoked"}>
                        {inspectView.valid ? "Valid" : "Invalid"}
                      </Chip>

                      {!inspectView.valid ? (
                        <>
                          <p className={styles.errorText}>
                            {`This fixture definition has ${inspectView.errors.length} error(s) and can't be added. Fix them and try again.`}
                          </p>
                          <ul className={styles.diagnosticList}>
                            {inspectView.errors.map((message, index) => (
                              <li key={index}>{message}</li>
                            ))}
                          </ul>
                        </>
                      ) : null}

                      {inspectView.warnings.length > 0 ? (
                        <>
                          <Chip tone="armed">Warning</Chip>
                          <p className={styles.warningText}>
                            {`This import has ${inspectView.warnings.length} unsupported or approximated attribute(s) — review before adding.`}
                          </p>
                          <ul className={styles.diagnosticList}>
                            {inspectView.warnings.map((warning, index) => (
                              <li key={index}>{warning.detail}</li>
                            ))}
                          </ul>
                        </>
                      ) : null}
                    </div>
                  ) : null}
                </Panel>
              ) : (
                <Panel className={styles.inspectPanel}>
                  <PanelHeader label="About this search" />
                  <p className={styles.catalogScopeNote}>
                    This search matches Open Fixture Library manufacturer names only. After choosing a
                    manufacturer, you enter the fixture&apos;s key yourself to import it.
                  </p>
                </Panel>
              )}
            </div>
          </>
        )}
      </div>
    </div>
  );
}
