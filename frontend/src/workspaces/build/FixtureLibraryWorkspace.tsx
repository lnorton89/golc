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
// UI-SPEC's exact empty/no-results/unreachable copy.
//
// 09-06-PLAN.md adds the fixture-key field and the "Add to Library"
// confirm action (D-02): selecting a manufacturer reveals a Field for the
// fixture key; previewing calls FixtureLibraryService.PreviewOFL and
// renders the candidate's identity/warnings inline in this same panel --
// never a modal, dialog, or wizard step. "Add to Library" stays disabled
// until the candidate passes validation, mirroring ScriptsWorkspace's own
// validation-gates-action discipline. A commit that finds the destination
// already present renders a distinct message with a separate "Replace"
// action rather than silently overwriting anything. An abandoned candidate
// is discarded (DiscardPreview) when the manufacturer selection changes,
// the fixture key is edited, or the operator switches back to "My
// Library," so a staged preview never lingers.
//
// A later revision implements 09-RESEARCH.md Open Question 1's flagged
// follow-up: SearchOFL now also matches fixture keys (not just
// manufacturer names), backed by a second catalog index fetched from a
// second, discretely reviewed SSRF-allowed host
// (internal/fixture/ofl/fixtureindex.go's githubAPIHost). Fixture search
// results render above the manufacturer list; selecting one (
// handleSelectFixture) fills in both the manufacturer and fixture key and
// calls PreviewOFL immediately -- unlike a manufacturer row, a fixture row
// already identifies one specific candidate, so there is nothing left for
// the operator to type.
//
// 09-07-PLAN.md adds the hand-authored YAML path on the "My Library" side
// (D-04, FIXT-04): an "Add Custom Fixture…" action reveals a "Fixture file
// path" Field with a "Browse…" button that opens the native picker
// (pickFixtureFile) -- or the operator can type a path directly. Either
// way, validating calls previewFixtureFile and renders the result through
// the exact same candidate-preview rendering (renderCandidateBody) the OFL
// catalog path already uses -- one confirm action ("Add to Library"), one
// error presentation, no in-app YAML editor. A staged custom-fixture
// preview is discarded when the path is edited, the affordance is
// dismissed, or the operator switches to the catalog side.
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Lightbulb, Repeat, Plus, X, FolderOpen, ShieldCheck, Eye } from "lucide-react";

import {
  commitFixturePreview,
  discardFixturePreview,
  errorMessage,
  inspectFixtureFile,
  listLocalFixtures,
  pickFixtureFile,
  previewFixtureFile,
  previewOflFixture,
  searchOflManufacturers,
  type FixtureInspectView,
  type FixtureLibraryRowView,
  type FixturePreviewView,
  type OflFixtureView,
  type OflManufacturerView,
  type OflSearchView,
} from "../../lib/wailsBridge";
import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import Panel from "../../components/primitives/Panel/Panel";
import PanelHeader from "../../components/primitives/PanelHeader/PanelHeader";
import ScrollRegion from "../../components/primitives/ScrollRegion/ScrollRegion";
import EmptyState from "../../components/primitives/EmptyState/EmptyState";
import ListRow from "../../components/primitives/ListRow/ListRow";
import Chip from "../../components/primitives/Chip/Chip";
import Field from "../../components/primitives/Field/Field";
import Button from "../../components/primitives/Button/Button";
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

// humanizeFixtureKey renders an OFL fixture key ("colorband-pix") as a
// readable label ("Colorband Pix") for the catalog fixture-result rows --
// an inferred display label, not the fixture's official branded name (OFL
// does not expose that in the tree-listing index this search reads), so
// the row's meta text always shows the raw key/manufacturer alongside it.
function humanizeFixtureKey(key: string): string {
  return key
    .split("-")
    .filter((segment) => segment !== "")
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(" ");
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

  // Catalog import candidate (09-06-PLAN.md, D-02): selecting a
  // manufacturer + entering a fixture key stages a preview via PreviewOFL;
  // "Add to Library" commits it. alreadyExists distinguishes a refused
  // commit (destination present) from every other failure, so the
  // "Replace" action only ever appears for that specific case.
  const [selectedManufacturer, setSelectedManufacturer] = useState<OflManufacturerView | null>(null);
  const [candidateFixtureKey, setCandidateFixtureKey] = useState("");
  const [previewing, setPreviewing] = useState(false);
  const [previewView, setPreviewView] = useState<FixturePreviewView | null>(null);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [committing, setCommitting] = useState(false);
  const [alreadyExists, setAlreadyExists] = useState(false);

  // Custom-fixture add (09-07-PLAN.md, D-04): "Add Custom Fixture…" reveals
  // a "Fixture file path" field (typed or picked via the native dialog).
  // Validating it shares the exact same previewView/previewing/
  // previewError/committing/alreadyExists state the catalog candidate
  // above uses -- the two flows are mutually exclusive on screen, so one
  // shared candidate slot serves the same confirm action and error
  // presentation for both (D-02's "one shared panel" rule extended to the
  // custom-fixture path).
  const [addingCustomFixture, setAddingCustomFixture] = useState(false);
  const [customFixturePath, setCustomFixturePath] = useState("");

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

  // discardCandidatePreview fires DiscardPreview for token (if any staged
  // preview exists) without blocking the caller -- an abandoned preview
  // never accumulates (T-09-06-05), but the UI never waits on the
  // fire-and-forget cleanup call.
  const discardCandidatePreview = (token: string | null) => {
    if (!token) return;
    void discardFixturePreview(token);
  };

  // resetCandidate clears every piece of catalog-candidate state and
  // discards any staged preview -- called whenever the manufacturer
  // selection changes, the fixture key is edited, or the operator switches
  // back to "My Library" (D-02).
  const resetCandidate = () => {
    discardCandidatePreview(previewView?.previewToken ?? null);
    setPreviewView(null);
    setPreviewError(null);
    setAlreadyExists(false);
  };

  const handleSelectManufacturer = (manufacturer: OflManufacturerView) => {
    resetCandidate();
    setSelectedManufacturer(manufacturer);
    setCandidateFixtureKey("");
  };

  const handleFixtureKeyChange = (value: string) => {
    resetCandidate();
    setCandidateFixtureKey(value);
  };

  // beginPreview calls PreviewOFL against manufacturerKey/fixtureKey --
  // never a second fetch/normalize/pin implementation on this side (D-02,
  // T-09-06-04). Shared by handlePreview (the operator-entered fixture-key
  // field) and handleSelectFixture (a fixture search result row, which
  // already carries both keys and so previews immediately on selection).
  const beginPreview = (manufacturerKey: string, fixtureKey: string) => {
    setPreviewing(true);
    setPreviewError(null);
    setAlreadyExists(false);
    void (async () => {
      const view = await previewOflFixture(manufacturerKey, fixtureKey);
      setPreviewView(view);
      setPreviewing(false);
    })();
  };

  // handlePreview calls PreviewOFL against the selected manufacturer's key
  // and the operator-entered fixture key.
  const handlePreview = () => {
    if (!selectedManufacturer) return;
    const key = candidateFixtureKey.trim();
    if (key === "") return;
    beginPreview(selectedManufacturer.key, key);
  };

  // handleSelectFixture (D-01's fixture-name search result) fills in both
  // the manufacturer and fixture-key fields from the selected row and
  // previews the candidate immediately -- unlike selecting a manufacturer
  // row, a fixture row already identifies one specific fixture, so there is
  // nothing left for the operator to type.
  const handleSelectFixture = (fixture: OflFixtureView) => {
    resetCandidate();
    setSelectedManufacturer({ key: fixture.manufacturerKey, name: fixture.manufacturerName, website: "" });
    setCandidateFixtureKey(fixture.fixtureKey);
    beginPreview(fixture.manufacturerKey, fixture.fixtureKey);
  };

  // handleCommit calls CommitPreview against the currently staged preview.
  // A refused-existing-destination result (GOLC_WAILS_FIXTURE_IMPORT_EXISTS)
  // renders the distinct already-in-library message with a separate
  // Replace action -- never an automatic overwrite (T-09-06-02). On
  // success the local library list is refreshed so the newly added fixture
  // appears immediately, and the candidate is cleared.
  const handleCommit = async (overwrite: boolean): Promise<void> => {
    if (!previewView) return;
    setCommitting(true);
    try {
      const result = await commitFixturePreview(previewView.previewToken, overwrite);
      if (result.exitCode === 0) {
        await refresh();
        setPreviewView(null);
        setPreviewError(null);
        setAlreadyExists(false);
        setSelectedManufacturer(null);
        setCandidateFixtureKey("");
        return;
      }
      if (result.stderr.includes("GOLC_WAILS_FIXTURE_IMPORT_EXISTS")) {
        setAlreadyExists(true);
        return;
      }
      setPreviewError(result.stderr.trim() || "Add to Library failed");
    } finally {
      setCommitting(false);
    }
  };

  // handleToggleAddCustomFixture reveals/dismisses the custom-fixture
  // affordance, discarding any staged candidate either way (D-04).
  const handleToggleAddCustomFixture = () => {
    resetCandidate();
    setCustomFixturePath("");
    setAddingCustomFixture((current) => !current);
  };

  // handleCustomFixturePathChange discards any staged preview whenever the
  // operator edits the path -- a stale candidate must never linger against
  // an edited path (D-04).
  const handleCustomFixturePathChange = (value: string) => {
    resetCandidate();
    setCustomFixturePath(value);
  };

  // handleBrowseCustomFixture calls the native picker (pickFixtureFile); an
  // empty return (cancellation, or an absent bridge) leaves the field
  // untouched with no error, mirroring ShowsWorkspace's identical
  // cancel-is-a-no-op contract.
  const handleBrowseCustomFixture = () => {
    void (async () => {
      const path = await pickFixtureFile();
      if (path === "") return;
      resetCandidate();
      setCustomFixturePath(path);
    })();
  };

  // handleValidateCustomFixture calls PreviewFile against the typed/picked
  // path -- the sole validation authority is the canonical "fixture
  // inspect" route PreviewFile forwards to (T-09-07-01); this side never
  // decodes, validates, or pins anything itself.
  const handleValidateCustomFixture = () => {
    const path = customFixturePath.trim();
    if (path === "") return;
    setPreviewing(true);
    setPreviewError(null);
    setAlreadyExists(false);
    void (async () => {
      const view = await previewFixtureFile(path);
      setPreviewView(view);
      setPreviewing(false);
    })();
  };

  // renderCandidateBody is the SAME candidate-preview rendering both the
  // OFL catalog path and the custom-fixture path render through -- one
  // "Add to Library" confirm action, one "{N} error(s)" presentation, one
  // lossy-warning treatment, one already-in-library/Replace path, never
  // duplicated per source (D-02 extended to D-04 by 09-07-PLAN.md).
  const renderCandidateBody = (view: FixturePreviewView) => (
    <div className={styles.inspectBody}>
      <div className={styles.techReadout}>
        {`${view.inspect.stableKey || "—"} · ${view.inspect.contentHash || "—"}`}
      </div>
      <div className={styles.techReadout}>
        {`Schema ${view.inspect.schemaVersion} · Revision ${view.inspect.revision || "—"}`}
      </div>
      <Chip tone={view.inspect.valid ? "frame-lock" : "revoked"}>{view.inspect.valid ? "Valid" : "Invalid"}</Chip>

      {!view.inspect.valid ? (
        <>
          <p className={styles.errorText}>
            {`This fixture definition has ${view.inspect.errors.length} error(s) and can't be added. Fix them and try again.`}
          </p>
          <ul className={styles.diagnosticList}>
            {view.inspect.errors.map((message, index) => (
              <li key={index}>{message}</li>
            ))}
          </ul>
        </>
      ) : null}

      {view.inspect.warnings.length > 0 ? (
        <>
          <Chip tone="armed">Warning</Chip>
          <p className={styles.warningText}>
            {`This import has ${view.inspect.warnings.length} unsupported or approximated attribute(s) — review before adding.`}
          </p>
          <ul className={styles.diagnosticList}>
            {view.inspect.warnings.map((warning, index) => (
              <li key={index}>{warning.detail}</li>
            ))}
          </ul>
        </>
      ) : null}

      {alreadyExists ? (
        <div className={styles.alreadyExists}>
          <p className={styles.warningText}>This fixture is already in your library.</p>
          <Button variant="secondary" icon={Repeat} onClick={() => void handleCommit(true)} disabled={committing}>
            Replace
          </Button>
        </div>
      ) : (
        <Button
          variant="primary"
          icon={Plus}
          onClick={() => void handleCommit(false)}
          disabled={!view.inspect.valid || committing}
        >
          {committing ? "Adding…" : "Add to Library"}
        </Button>
      )}
    </div>
  );

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
                onClick={() => {
                  resetCandidate();
                  setSelectedManufacturer(null);
                  setCandidateFixtureKey("");
                  setSource("local");
                }}
              >
                My Library
              </button>
              <button
                type="button"
                className={source === "catalog" ? styles.sourceButtonActive : styles.sourceButton}
                aria-pressed={source === "catalog"}
                onClick={() => {
                  resetCandidate();
                  setAddingCustomFixture(false);
                  setCustomFixturePath("");
                  setSource("catalog");
                }}
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
                  <PanelHeader
                    label={`Fixture Library (${countLabel})`}
                    icon={Lightbulb}
                    action={
                      <Button
                        variant="secondary"
                        icon={addingCustomFixture ? X : Plus}
                        onClick={handleToggleAddCustomFixture}
                      >
                        {addingCustomFixture ? "Cancel" : "Add Custom Fixture…"}
                      </Button>
                    }
                  />
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
                        Search the Open Fixture Library by fixture or manufacturer name.
                      </EmptyState>
                    ) : catalogView?.unreachable ? (
                      <div className={styles.catalogUnreachable}>
                        <Chip tone="offline">Unreachable</Chip>
                        <p className={styles.errorText}>
                          Can&apos;t reach the Open Fixture Library. Check your network connection and try again.
                        </p>
                      </div>
                    ) : catalogView && catalogView.fixtures.length === 0 && catalogView.manufacturers.length === 0 ? (
                      <EmptyState icon={Lightbulb}>
                        {`No fixtures matched "${trimmedQuery}". Try a different name or manufacturer.`}
                      </EmptyState>
                    ) : catalogView ? (
                      <>
                        {catalogView.fixtures.length > 0 ? (
                          <ul className={styles.list} aria-label="Open Fixture Library fixtures">
                            {catalogView.fixtures.map((fixture) => (
                              <li key={`${fixture.manufacturerKey}/${fixture.fixtureKey}`}>
                                <ListRow
                                  label={humanizeFixtureKey(fixture.fixtureKey)}
                                  meta={`${fixture.manufacturerName} · ${fixture.fixtureKey}`}
                                  selected={
                                    selectedManufacturer?.key === fixture.manufacturerKey &&
                                    candidateFixtureKey === fixture.fixtureKey
                                  }
                                  onSelect={() => handleSelectFixture(fixture)}
                                />
                              </li>
                            ))}
                          </ul>
                        ) : null}
                        {catalogView.manufacturers.length > 0 ? (
                          <ul className={styles.list} aria-label="Open Fixture Library manufacturers">
                            {catalogView.manufacturers.map((manufacturer) => (
                              <li key={manufacturer.key}>
                                <ListRow
                                  label={manufacturer.name}
                                  meta={manufacturer.key}
                                  selected={
                                    selectedManufacturer?.key === manufacturer.key && candidateFixtureKey === ""
                                  }
                                  onSelect={() => handleSelectManufacturer(manufacturer)}
                                />
                              </li>
                            ))}
                          </ul>
                        ) : null}
                      </>
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
                  <PanelHeader label={addingCustomFixture ? "Add Custom Fixture" : "Inspect"} />
                  {addingCustomFixture ? (
                    <div className={styles.candidateBody}>
                      <Field
                        label="Fixture file path"
                        value={customFixturePath}
                        placeholder="e.g. C:\fixtures\my-fixture.yaml"
                        onChange={(event) => handleCustomFixturePathChange(event.target.value)}
                        disabled={previewing}
                      />
                      <Button variant="secondary" icon={FolderOpen} onClick={handleBrowseCustomFixture} disabled={previewing}>
                        Browse…
                      </Button>
                      <Button
                        variant="secondary"
                        icon={ShieldCheck}
                        onClick={handleValidateCustomFixture}
                        disabled={customFixturePath.trim() === "" || previewing}
                      >
                        {previewing ? "Validating…" : "Validate"}
                      </Button>

                      {previewError ? <p className={styles.errorText}>{previewError}</p> : null}

                      {previewView ? renderCandidateBody(previewView) : null}
                    </div>
                  ) : !selectedFileName ? (
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
                  <PanelHeader label={selectedManufacturer ? "Import Candidate" : "About this search"} />
                  {!selectedManufacturer ? (
                    <p className={styles.catalogScopeNote}>
                      This search matches Open Fixture Library manufacturer names and fixture keys. Selecting a
                      fixture previews it directly; selecting a manufacturer instead lets you enter a fixture key
                      yourself.
                    </p>
                  ) : (
                    <div className={styles.candidateBody}>
                      <p className={styles.candidateManufacturer}>{selectedManufacturer.name}</p>
                      <Field
                        label="Fixture key"
                        value={candidateFixtureKey}
                        placeholder="e.g. led-par-64-tri-b"
                        onChange={(event) => handleFixtureKeyChange(event.target.value)}
                      />
                      <Button
                        variant="secondary"
                        icon={Eye}
                        onClick={handlePreview}
                        disabled={candidateFixtureKey.trim() === "" || previewing}
                      >
                        {previewing ? "Previewing…" : "Preview"}
                      </Button>

                      {previewError ? <p className={styles.errorText}>{previewError}</p> : null}

                      {previewView ? renderCandidateBody(previewView) : null}
                    </div>
                  )}
                </Panel>
              )}
            </div>
          </>
        )}
      </div>
    </div>
  );
}
