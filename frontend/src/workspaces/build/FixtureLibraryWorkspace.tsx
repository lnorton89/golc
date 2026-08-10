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
import { useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient, keepPreviousData } from "@tanstack/react-query";
import { useVirtualizer } from "@tanstack/react-virtual";
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
import { useToast } from "../../components/primitives/Toast/Toast";
import { useDebouncedValue } from "../../hooks/useDebouncedValue";
import { fuzzySearch } from "../../lib/fuzzySearch";
import { queryKeys } from "../../lib/queryKeys";
import { HOW_IT_WORKS_BY_ID } from "../../shell/navigation";
import { Button, Chip, EmptyState, ErrorState, Field, ListRow, LoadingState, Panel, PanelHeader, ScrollRegion, Toolbar, ToggleGroup, type ToggleGroupOption } from "../../design-system";
import styles from "./FixtureLibraryWorkspace.module.css";

// oflSearchDebounceMs is the client-side debounce D-01/T-09-05-03 require
// so a typing burst issues at most one FixtureLibraryService.SearchOFL
// call, not one per keystroke.
const oflSearchDebounceMs = 250;

// rowHaystack is what a local-library row is searched by: manufacturer and
// model together, so either (or a query spanning both, "chauvet slimpar")
// hits. D-03's scope is unchanged -- still client-side over already-fetched
// rows, still no backend round-trip and no faceted filter controls -- but
// the match is now ranked and typo-tolerant (lib/fuzzySearch.ts) rather
// than a plain substring test that returned nothing for one wrong letter.
function rowHaystack(row: FixtureLibraryRowView): string {
  return `${row.manufacturer} ${row.model}`;
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

// summarizeDiagnostics collapses byte-identical diagnostic messages into
// one entry per distinct message with an occurrence count. OFL's
// per-capability-entry warnings frequently repeat many times over on a
// single channel (an "Auto Program" wheel with a dozen-plus near-identical
// "Pattern N" capability entries all map to the same unmapped-construct
// detail string -- see internal/fixture/ofl/normalize.go's
// unmappedCapabilityDetail), and rendering every one of those as its own
// <li> read as a wall of nonsensical duplicate lines rather than N
// distinct issues.
function summarizeDiagnostics(messages: string[]): { text: string; count: number }[] {
  const counts = new Map<string, number>();
  for (const message of messages) {
    counts.set(message, (counts.get(message) ?? 0) + 1);
  }
  return Array.from(counts.entries()).map(([text, count]) => ({ text, count }));
}

// VIRTUALIZE_ABOVE_ROWS mirrors AppLogPanel's identical threshold and
// exists for the same reason: windowing costs absolute positioning, a
// per-row measurement pass, and a resize observer, which is not worth
// paying until there are more rows than any viewport could show.
const VIRTUALIZE_ABOVE_ROWS = 60;

/** CatalogFixtureList renders the OFL fixture-key results.
 *
 * Its own component purely so the virtualizer hook has an unconditional
 * top level to live at -- the list itself renders inside a chain of
 * ternaries that React's rules of hooks forbid calling a hook from.
 *
 * This is the list that most needs windowing anywhere in the app.
 * internal/fixture/ofl's FilterFixtureIndex returns UNCAPPED matches from
 * a ~4000-fixture index, so an ordinary query like "par" mounts hundreds
 * of ListRows at once, each with its own label and meta line. */
function CatalogFixtureList({
  fixtures,
  scrollElement,
  isSelected,
  onSelect,
}: {
  fixtures: OflFixtureView[];
  scrollElement: HTMLDivElement | null;
  isSelected: (fixture: OflFixtureView) => boolean;
  onSelect: (fixture: OflFixtureView) => void;
}) {
  const listRef = useRef<HTMLUListElement>(null);
  const virtualized = fixtures.length > VIRTUALIZE_ABOVE_ROWS;

  const virtualizer = useVirtualizer({
    count: virtualized ? fixtures.length : 0,
    // The scroll element arrives as a VALUE rather than a ref on purpose.
    // It belongs to an ancestor (the shared ScrollRegion), and a
    // virtualizer only reads getScrollElement during render/layout: if it
    // ever saw null there and nothing subsequently re-rendered, it would
    // stay detached from the scroll container -- still drawing a
    // plausible window at the top and reporting a full total size, so it
    // would LOOK correct while scrolling never moved the window.
    //
    // Passing a ref happens to work today, because this component only
    // mounts once search results exist, long after the ScrollRegion's ref
    // has attached. That is a property of the current render order, not a
    // guarantee. Threading the element through state makes its arrival a
    // re-render and removes the dependency on that ordering entirely.
    getScrollElement: () => scrollElement,
    estimateSize: () => 44,
    overscan: 10,
    // The scroll container is shared with the manufacturers list below, so
    // this list does not start at scrollTop 0 once the viewport has any
    // padding. scrollMargin tells the virtualizer where the list actually
    // begins; without it every row sits that many pixels too high.
    scrollMargin: listRef.current?.offsetTop ?? 0,
  });

  const renderRow = (fixture: OflFixtureView) => (
    <ListRow
      label={humanizeFixtureKey(fixture.fixtureKey)}
      meta={`${fixture.manufacturerName} · ${fixture.fixtureKey}`}
      selected={isSelected(fixture)}
      onSelect={() => onSelect(fixture)}
    />
  );

  return (
    <ul
      ref={listRef}
      className={styles.list}
      aria-label="Open Fixture Library fixtures"
      style={virtualized ? { position: "relative", height: `${virtualizer.getTotalSize()}px` } : undefined}
    >
      {virtualized
        ? virtualizer.getVirtualItems().map((virtualRow) => {
            const fixture = fixtures[virtualRow.index];
            return (
              <li
                key={`${fixture.manufacturerKey}/${fixture.fixtureKey}`}
                ref={virtualizer.measureElement}
                data-index={virtualRow.index}
                style={{
                  position: "absolute",
                  top: 0,
                  left: 0,
                  width: "100%",
                  transform: `translateY(${virtualRow.start - virtualizer.options.scrollMargin}px)`,
                }}
              >
                {renderRow(fixture)}
              </li>
            );
          })
        : fixtures.map((fixture) => (
            <li key={`${fixture.manufacturerKey}/${fixture.fixtureKey}`}>{renderRow(fixture)}</li>
          ))}
    </ul>
  );
}

type LibrarySource = "local" | "catalog";

// PreviewRequest is the discriminated input to the one preview mutation --
// an OFL catalog candidate (manufacturer + fixture key) or a hand-authored
// file on disk (D-04). Both stage into the same candidate slot and render
// through the same renderCandidateBody, so they share one mutation.
type PreviewRequest =
  | { kind: "ofl"; manufacturerKey: string; fixtureKey: string }
  | { kind: "file"; path: string };

// SOURCE_OPTIONS backs the "My Library" / "Open Fixture Library" toggle
// (D-01) -- a mutually-exclusive two-option choice, now the shared
// ToggleGroup primitive instead of two hand-rolled Buttons each carrying
// their own aria-pressed/variant bookkeeping.
const SOURCE_OPTIONS: ReadonlyArray<ToggleGroupOption> = [
  { value: "local", label: "My Library" },
  { value: "catalog", label: "Open Fixture Library" },
];

export default function FixtureLibraryWorkspace() {
  const queryClient = useQueryClient();
  const toast = useToast();
  // State rather than a ref: CatalogFixtureList's virtualizer needs this
  // element, and its arrival has to trigger a render for the virtualizer
  // to see it (see that component's own comment).
  const [catalogScrollElement, setCatalogScrollElement] = useState<HTMLDivElement | null>(null);

  const [search, setSearch] = useState("");
  const [selectedFileName, setSelectedFileName] = useState<string | null>(null);

  // source (D-01): "My Library" (the local fixtures directory, unchanged
  // from 09-01-PLAN.md) vs. "Open Fixture Library" (this plan's catalog
  // manufacturer-name search). The same search input serves both sides.
  const [source, setSource] = useState<LibrarySource>("local");

  // Catalog import candidate (09-06-PLAN.md, D-02): selecting a
  // manufacturer + entering a fixture key stages a preview via PreviewOFL;
  // "Add to Library" commits it. alreadyExists distinguishes a refused
  // commit (destination present) from every other failure, so the
  // "Replace" action only ever appears for that specific case.
  //
  // previewView/previewError/alreadyExists stay useState rather than being
  // read off the mutations below, because all three outlive the mutation
  // that produced them: a staged candidate stays on screen until the
  // operator commits or abandons it, and resetCandidate has to be able to
  // clear all three from unrelated events (manufacturer changed, fixture
  // key edited, source toggled) that no mutation lifecycle knows about.
  const [selectedManufacturer, setSelectedManufacturer] = useState<OflManufacturerView | null>(null);
  const [candidateFixtureKey, setCandidateFixtureKey] = useState("");
  const [previewView, setPreviewView] = useState<FixturePreviewView | null>(null);
  const [previewError, setPreviewError] = useState<string | null>(null);
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

  // The local library listing. errorMessage/ErrorState below stay wired to
  // libraryQuery.error rather than being deleted, but note that
  // listLocalFixtures is contractually non-throwing: it catches internally
  // and returns offlineFixtureLibraryView(), so this error branch is as
  // unreachable now as the try/catch it replaces was. The seam is kept so a
  // future throwing bridge surfaces here instead of silently rendering an
  // empty library.
  const libraryQuery = useQuery({
    queryKey: queryKeys.fixtureLibrary.local(),
    queryFn: listLocalFixtures,
  });
  const directory = libraryQuery.data?.directory ?? "";
  const rows = libraryQuery.data?.rows ?? [];

  // Inspect runs as a query keyed by the resolved path rather than as an
  // imperative call inside handleSelectRow: selecting a row now only sets
  // selectedFileName, and the read follows from that one piece of state.
  // Re-selecting a row already inspected serves the cached result instead
  // of re-crossing the bridge.
  const selectedPath =
    selectedFileName === null ? null : directory ? `${directory}/${selectedFileName}` : selectedFileName;
  const inspectQuery = useQuery({
    queryKey: queryKeys.fixtureLibrary.inspect(selectedPath ?? ""),
    queryFn: () => inspectFixtureFile(selectedPath ?? ""),
    enabled: selectedPath !== null,
  });
  const inspecting = inspectQuery.isFetching;
  const inspectView: FixtureInspectView | null = selectedPath === null ? null : (inspectQuery.data ?? null);

  // Debounced catalog search (T-09-05-03): at most one SearchOFL call per
  // ~250ms typing pause, and only while the catalog side is active. An
  // empty/whitespace-only query never calls the bridge at all -- `enabled`
  // keeps the empty-prompt copy rendering immediately with no in-flight
  // state.
  //
  // Keying the query by the settled search text retires the monotonic
  // request-id ref this used to need: two searches for different text are
  // two cache entries, so a slow response for an earlier query can no
  // longer land after a faster later one and overwrite it. keepPreviousData
  // preserves the prior behaviour of leaving the last results on screen
  // while the next search resolves, rather than blanking to "Searching…"
  // on every keystroke pause.
  const debouncedSearch = useDebouncedValue(search.trim(), oflSearchDebounceMs);
  const catalogEnabled = source === "catalog" && debouncedSearch !== "";
  const catalogQuery = useQuery({
    queryKey: queryKeys.fixtureLibrary.oflSearch(debouncedSearch),
    queryFn: () => searchOflManufacturers(debouncedSearch),
    enabled: catalogEnabled,
    placeholderData: keepPreviousData,
  });
  // Gated on catalogEnabled so clearing the search field (or switching back
  // to "My Library") shows the empty prompt immediately -- without this,
  // keepPreviousData would keep serving the last query's results under the
  // now-empty key.
  const catalogView: OflSearchView | null = catalogEnabled ? (catalogQuery.data ?? null) : null;
  const catalogSearching = catalogEnabled && catalogQuery.isFetching;

  // discardPreviewMutation is fire-and-forget cleanup (T-09-06-05): an
  // abandoned staged preview must not accumulate, but no UI ever waits on
  // it.
  const discardPreviewMutation = useMutation({ mutationFn: discardFixturePreview });

  // previewMutation stages an import candidate from either source -- the
  // OFL catalog (PreviewOFL) or a hand-authored file (PreviewFile). One
  // mutation rather than two because both feed the same single candidate
  // slot, so two would let both be in flight at once and race into it.
  const previewMutation = useMutation({
    mutationFn: (input: PreviewRequest) =>
      input.kind === "ofl" ? previewOflFixture(input.manufacturerKey, input.fixtureKey) : previewFixtureFile(input.path),
    onSuccess: (view) => setPreviewView(view),
  });
  const previewing = previewMutation.isPending;

  // commitMutation is the single confirm action. commitFixturePreview never
  // rejects (it returns bridgeUnavailableResult() when the bridge is
  // absent), so every outcome -- success, already-exists, and every other
  // failure -- is dispatched from the result in onSuccess; onError would
  // never fire.
  const commitMutation = useMutation({
    mutationFn: (overwrite: boolean) => commitFixturePreview(previewView?.previewToken ?? "", overwrite),
    onSuccess: (result) => {
      if (result.exitCode === 0) {
        // A successful commit was the one genuinely silent outcome in this
        // workspace: the candidate panel cleared itself and the list
        // refreshed, but nothing ever said the fixture had been added --
        // the operator had to infer it from a row appearing in a list they
        // may not have been looking at. Every FAILURE path below stays
        // inline instead, and deliberately: "already in your library"
        // carries its own Replace action, and a generic failure belongs
        // next to the candidate it refers to, where it persists until the
        // operator acts. A toast is the right shape only for the outcome
        // that needs acknowledging and then forgetting.
        toast.success("Fixture added", previewView?.inspect.stableKey || undefined);
        // Replaces the former `await refresh()`: invalidation refetches the
        // listing for every mounted reader of this key, not just this
        // component's own private copy of the rows.
        void queryClient.invalidateQueries({ queryKey: queryKeys.fixtureLibrary.local() });
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
    },
  });
  const committing = commitMutation.isPending;

  const filteredRows = useMemo(() => fuzzySearch(rows, search, rowHaystack), [rows, search]);

  // The catalog side is matched Go-side (internal/fixture/ofl's
  // FilterFixtureIndex/FilterManufacturers, both strings.Contains) and
  // returns UNCAPPED results -- a query like "par" pulls hundreds of
  // entries out of a ~4000-fixture index, in index order. Ranking them
  // here does not change which entries Go decided match; it only puts the
  // closest ones first, where an operator will actually see them.
  const rankedCatalogFixtures = useMemo(
    () => fuzzySearch(catalogView?.fixtures ?? [], debouncedSearch, (fixture) => `${fixture.manufacturerName} ${fixture.fixtureKey}`),
    [catalogView, debouncedSearch],
  );
  const rankedCatalogManufacturers = useMemo(
    () => fuzzySearch(catalogView?.manufacturers ?? [], debouncedSearch, (manufacturer) => `${manufacturer.name} ${manufacturer.key}`),
    [catalogView, debouncedSearch],
  );

  const countLabel = `${rows.length} fixture${rows.length === 1 ? "" : "s"}`;
  const trimmedQuery = search.trim();

  // handleSelectSource switches the source toggle (D-01), clearing
  // whichever side's own in-flight candidate/custom-fixture state doesn't
  // apply to the newly selected side -- the same per-branch cleanup the
  // former two onClick handlers each did individually.
  const handleSelectSource = (next: string) => {
    resetCandidate();
    if (next === "local") {
      setSelectedManufacturer(null);
      setCandidateFixtureKey("");
    } else {
      setAddingCustomFixture(false);
      setCustomFixturePath("");
    }
    setSource(next as LibrarySource);
  };

  // handleSelectRow calls Inspect against the row's file resolved under
  // the library's own directory (as ListLocal reported it) -- never a
  // second decode/pin/normalize implementation on this side (D-02,
  // T-09-01-03).
  const handleSelectRow = (row: FixtureLibraryRowView) => {
    setSelectedFileName(row.fileName);
  };

  // discardCandidatePreview fires DiscardPreview for token (if any staged
  // preview exists) without blocking the caller -- an abandoned preview
  // never accumulates (T-09-06-05), but the UI never waits on the
  // fire-and-forget cleanup call.
  const discardCandidatePreview = (token: string | null) => {
    if (!token) return;
    discardPreviewMutation.mutate(token);
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
    setPreviewError(null);
    setAlreadyExists(false);
    previewMutation.mutate({ kind: "ofl", manufacturerKey, fixtureKey });
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
  const handleCommit = (overwrite: boolean) => {
    if (!previewView) return;
    commitMutation.mutate(overwrite);
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
    setPreviewError(null);
    setAlreadyExists(false);
    previewMutation.mutate({ kind: "file", path });
  };

  // renderCandidateBody is the SAME candidate-preview rendering both the
  // OFL catalog path and the custom-fixture path render through -- one
  // "Add to Library" confirm action, one "{N} error(s)" presentation, one
  // lossy-warning treatment, one already-in-library/Replace path, never
  // duplicated per source (D-02 extended to D-04 by 09-07-PLAN.md).
  const renderCandidateBody = (view: FixturePreviewView) => (
    <ScrollRegion className={styles.inspectBody}>
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
            {summarizeDiagnostics(view.inspect.errors).map((entry) => (
              <li key={entry.text}>{entry.count > 1 ? `${entry.text} (×${entry.count})` : entry.text}</li>
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
            {summarizeDiagnostics(view.inspect.warnings.map((warning) => warning.detail)).map((entry) => (
              <li key={entry.text}>{entry.count > 1 ? `${entry.text} (×${entry.count})` : entry.text}</li>
            ))}
          </ul>
        </>
      ) : null}

      {alreadyExists ? (
        <div className={styles.alreadyExists}>
          <p className={styles.warningText}>This fixture is already in your library.</p>
          <Button variant="secondary" icon={Repeat} onClick={() => handleCommit(true)} disabled={committing}>
            Replace
          </Button>
        </div>
      ) : (
        <Button
          variant="primary"
          icon={Plus}
          onClick={() => handleCommit(false)}
          disabled={!view.inspect.valid || committing}
        >
          {committing ? "Adding…" : "Add to Library"}
        </Button>
      )}
    </ScrollRegion>
  );

  return (
    <div className={styles.workspace}>
      <Toolbar title="Fixture Library" icon={Lightbulb} info={HOW_IT_WORKS_BY_ID["build-fixture-library"]} />
      <div className={styles.canvas}>
        {libraryQuery.isPending ? (
          <LoadingState label="Loading fixture library" />
        ) : (
          <>
            {libraryQuery.error ? (
              <ErrorState heading="Fixture library unavailable" message={errorMessage(libraryQuery.error)} />
            ) : null}
            <ToggleGroup aria-label="Fixture source" options={SOURCE_OPTIONS} value={source} onValueChange={handleSelectSource} />
            <Field
              label="Search fixtures"
              value={search}
              placeholder="Search fixtures by name or manufacturer…"
              onChange={(event) => setSearch(event.target.value)}
            />
            <div className={styles.layout}>
              {source === "local" ? (
                <Panel>
                  <PanelHeader
                    label={`Fixture Library (${countLabel})`}
                    icon={Lightbulb}
                    info="Lists every fixture definition already saved to your local library, with its validation status."
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
                  <ScrollRegion className={styles.libraryScroll}>
                    {rows.length === 0 ? (
                      <EmptyState icon={Lightbulb}>
                        <strong>No fixtures yet</strong>
                        <span>
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
                  <PanelHeader
                    label="Open Fixture Library"
                    icon={Lightbulb}
                    info="Searches the community Open Fixture Library catalog by manufacturer or fixture name so you can import a definition into your own library."
                  />
                  <ScrollRegion ref={setCatalogScrollElement} className={styles.libraryScroll}>
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
                        {rankedCatalogFixtures.length > 0 ? (
                          <CatalogFixtureList
                            fixtures={rankedCatalogFixtures}
                            scrollElement={catalogScrollElement}
                            isSelected={(fixture) =>
                              selectedManufacturer?.key === fixture.manufacturerKey &&
                              candidateFixtureKey === fixture.fixtureKey
                            }
                            onSelect={handleSelectFixture}
                          />
                        ) : null}
                        {rankedCatalogManufacturers.length > 0 ? (
                          <ul className={styles.list} aria-label="Open Fixture Library manufacturers">
                            {rankedCatalogManufacturers.map((manufacturer) => (
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
                  <PanelHeader
                    label={addingCustomFixture ? "Add Custom Fixture" : "Inspect"}
                    info={
                      addingCustomFixture
                        ? "Validates a hand-authored YAML fixture file from disk before adding it to your library."
                        : "Shows the selected fixture definition's identity, schema, and any validation errors or warnings."
                    }
                  />
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
                    <ScrollRegion className={styles.inspectBody}>
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
                            {summarizeDiagnostics(inspectView.errors).map((entry) => (
                              <li key={entry.text}>{entry.count > 1 ? `${entry.text} (×${entry.count})` : entry.text}</li>
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
                            {summarizeDiagnostics(inspectView.warnings.map((warning) => warning.detail)).map((entry) => (
                              <li key={entry.text}>{entry.count > 1 ? `${entry.text} (×${entry.count})` : entry.text}</li>
                            ))}
                          </ul>
                        </>
                      ) : null}
                    </ScrollRegion>
                  ) : null}
                </Panel>
              ) : (
                <Panel className={styles.inspectPanel}>
                  <PanelHeader
                    label={selectedManufacturer ? "Import Candidate" : "About this search"}
                    info={
                      selectedManufacturer
                        ? "Lets you enter the fixture key for the selected manufacturer and preview it before adding."
                        : "Explains what the Open Fixture Library search matches, and how selecting a fixture or manufacturer differs."
                    }
                  />
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
