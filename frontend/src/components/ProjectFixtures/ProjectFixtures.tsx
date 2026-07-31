// ProjectFixtures.tsx is the "fixtures already in the project, plus add
// more from the library" surface: it lists every patched deployment
// instance (flattened, one row per instance, resolved against the fixture
// library for a friendly manufacturer/model name) and offers an "Add from
// Library" panel that picks a fixture + mode + quantity, optionally an
// explicit starting universe/address, and reviews the backend's own
// non-committing impact preview before an explicit Apply commit -- exactly
// FixturePatch.tsx's review-before-apply discipline (POOL-04/D-15), reused
// rather than reinvented.
//
// This page never introduces a second fixture-patch data model: it reads
// and writes the exact same Pool/Deployment/Instance show-state data
// FixturePatch.tsx (Patch & Pools) does, via the same wailsBridge.ts
// FixturePatchService helpers. What's new here is that the user is never
// required to understand pools/deployments as separate concepts: adding a
// fixture transparently reuses an existing pool by a deterministic
// manufacturer/model name (or creates one), and reuses the active
// deployment (or creates/activates a "Default" one) as the force-attach
// target for AddPoolMembersPreview's AttachDeployments -- closing the
// "adopt a never-before-used pool" gap described in internal/pool/impact.go.
//
// All Go-bound calls go through wailsBridge.ts's FixturePatchService/
// FixtureLibraryService helpers -- this file never re-declares
// `declare global` itself and never adds a second pool/deployment mutation
// path (mirrors FixturePatch.tsx's own header comment).
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Plus, Eye, X, Check, PackagePlus, Pencil, ChevronUp, ChevronDown } from "lucide-react";

import {
  activateDeployment,
  addPoolMembersPreview,
  applyPatch,
  assertOk,
  createDeployment,
  createPool,
  errorMessage,
  listLocalFixtures,
  listPatch,
  offlinePatchView,
  reassignInstance,
  removePoolMemberPreview,
  renamePool,
  type FixtureLibraryRowView,
  type PatchPoolMemberView,
  type PatchView,
} from "../../lib/wailsBridge";
import styles from "./ProjectFixtures.module.css";

// ImpactOperation/ImpactPlan mirror internal/pool/impact.go's own
// snake_case JSON tags exactly -- duplicated locally rather than
// centralized in wailsBridge.ts, matching FixturePatch.tsx's own stated
// reasoning (these mirror the impact plan's raw encoding, never the
// camelCase view types wailsBridge.ts otherwise exposes).
interface ImpactOperation {
  dependent_kind: string;
  dependent_ref: string;
  dependent_id: string;
  action: string;
  proposed_universe?: number;
  proposed_address?: number;
  status: string;
}

interface ImpactPlan {
  schema_version: number;
  pool_id: string;
  operations: ImpactOperation[] | null;
  warnings?: { code: string; message: string }[];
  errors?: { code: string; message: string }[];
  plan_id: string;
}

interface FixtureRow {
  key: string;
  displayName: string;
  mode: string;
  universe: number;
  address: number;
  deploymentName: string;
  poolId: string;
  poolName: string;
  poolMemberId: string;
  fixtureStableKey: string;
}

function fixtureDisplayName(row: FixtureLibraryRowView): string {
  const name = `${row.manufacturer} ${row.model}`.trim();
  return name !== "" ? name : row.stableKey;
}

function resolveMemberDisplayName(
  pool: { name: string } | undefined,
  member: PatchPoolMemberView | undefined,
  libraryRows: FixtureLibraryRowView[],
): string {
  // pool.name is the one part of this row that renamePool can actually
  // change (RenamePool, wailsBridge.ts) -- it starts out equal to the
  // library's own "manufacturer model" name (handleReviewImpact below
  // mints a pool named via fixtureDisplayName), so preferring it here
  // means a rename is immediately visible on every row sharing that pool,
  // while an orphaned/pool-less instance still falls back to the
  // library-derived name rather than showing blank.
  if (pool?.name) {
    return pool.name;
  }
  if (!member) {
    return "Unknown fixture";
  }
  const row =
    libraryRows.find((candidate) => candidate.stableKey === member.fixtureStableKey) ??
    libraryRows.find((candidate) => candidate.contentHash === member.fixtureContentHash);
  if (row) {
    return fixtureDisplayName(row);
  }
  return member.fixtureStableKey || member.fixtureContentHash;
}

function buildFixtureRows(patch: PatchView, libraryRows: FixtureLibraryRowView[]): FixtureRow[] {
  const rows: FixtureRow[] = [];
  for (const deployment of patch.deployments) {
    for (const instance of deployment.instances) {
      const pool = patch.pools.find((candidate) => candidate.id === instance.poolId);
      const member = pool?.members.find((candidate) => candidate.id === instance.poolMemberId);
      rows.push({
        key: instance.id,
        displayName: resolveMemberDisplayName(pool, member, libraryRows),
        mode: instance.mode,
        universe: instance.universe,
        address: instance.address,
        deploymentName: deployment.name,
        poolId: instance.poolId,
        poolName: pool?.name ?? "",
        poolMemberId: instance.poolMemberId,
        fixtureStableKey: member?.fixtureStableKey ?? "",
      });
    }
  }
  return rows;
}

function parsePositiveInt(raw: string): number {
  const trimmed = raw.trim();
  if (trimmed === "") {
    return 0;
  }
  const value = Number(trimmed);
  if (!Number.isFinite(value) || value < 1) {
    return 0;
  }
  return Math.floor(value);
}

// NumberStepper mirrors TempoControls.tsx's BPM spinner (the app's one
// existing up/down-styled numeric control): a plain number input plus a
// pair of chevron buttons that nudge the uncommitted value by 1, each
// with tabIndex={-1} + onMouseDown preventDefault so a spinner click
// never steals focus from the input.
function NumberStepper({
  value,
  onChange,
  label,
  min = 1,
  placeholder,
}: {
  value: string;
  onChange: (value: string) => void;
  label: string;
  min?: number;
  placeholder?: string;
}) {
  const step = (delta: number) => {
    const parsed = Number(value) || 0;
    onChange(String(Math.max(min, Math.round(parsed + delta))));
  };

  return (
    <span className={styles.stepperWrap}>
      <input
        className={styles.stepperInput}
        type="number"
        min={min}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        aria-label={label}
      />
      <span className={styles.stepperSpinner}>
        <button
          type="button"
          className={styles.stepperSpinnerButton}
          tabIndex={-1}
          aria-label={`Increase ${label.toLowerCase()}`}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => step(1)}
        >
          <ChevronUp size={10} aria-hidden="true" />
        </button>
        <button
          type="button"
          className={styles.stepperSpinnerButton}
          tabIndex={-1}
          aria-label={`Decrease ${label.toLowerCase()}`}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => step(-1)}
        >
          <ChevronDown size={10} aria-hidden="true" />
        </button>
      </span>
    </span>
  );
}

type MetaBadgeKind = "id" | "mode" | "universe" | "address" | "deployment";

const META_BADGE_KIND_CLASS: Record<MetaBadgeKind, string> = {
  id: styles.badgeId,
  mode: styles.badgeMode,
  universe: styles.badgeUniverse,
  address: styles.badgeAddress,
  deployment: styles.badgeDeployment,
};

// MetaBadge is a small labeled pill for a bare identifier/value that
// would otherwise be ambiguous on its own (a truncated UUID, a
// deployment name that reads like a generic word) -- label is a fixed
// uppercase tag ("ID", "Deployment") so the value is never shown
// unexplained, mirroring the primitives/Chip pill shape (border/pill
// radius/monospace value) without adopting Chip's own tone/status
// vocabulary, which this row-metadata use has no need for. kind selects
// a background tint so the five badge kinds read apart at a glance
// (ProjectFixtures.module.css's .badge* modifiers).
function MetaBadge({
  label,
  value,
  kind,
  title,
}: {
  label: string;
  value: string;
  kind: MetaBadgeKind;
  title?: string;
}) {
  return (
    <span className={`${styles.badge} ${META_BADGE_KIND_CLASS[kind]}`} title={title}>
      <span className={styles.badgeLabel}>{label}</span>
      <span className={styles.badgeValue}>{value}</span>
    </span>
  );
}

export default function ProjectFixtures() {
  const [patch, setPatch] = useState<PatchView>(offlinePatchView());
  const [listLoading, setListLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [showAddForm, setShowAddForm] = useState(false);
  const [libraryRows, setLibraryRows] = useState<FixtureLibraryRowView[]>([]);
  const [selectedFixture, setSelectedFixture] = useState<FixtureLibraryRowView | null>(null);
  const [mode, setMode] = useState("");
  const [quantity, setQuantity] = useState("1");
  const [startUniverse, setStartUniverse] = useState("");
  const [startAddress, setStartAddress] = useState("");
  const [previewLoading, setPreviewLoading] = useState(false);
  const [pendingPreview, setPendingPreview] = useState<ImpactPlan | null>(null);
  const [applyLoading, setApplyLoading] = useState(false);

  const [removeTarget, setRemoveTarget] = useState<{ poolName: string; poolMemberId: string } | null>(null);
  const [removePreviewLoading, setRemovePreviewLoading] = useState(false);
  const [pendingRemovePreview, setPendingRemovePreview] = useState<ImpactPlan | null>(null);
  const [removeApplyLoading, setRemoveApplyLoading] = useState(false);

  const [reassigningInstanceId, setReassigningInstanceId] = useState<string | null>(null);
  const [reassignName, setReassignName] = useState("");
  const [reassignMode, setReassignMode] = useState("");
  const [reassignUniverse, setReassignUniverse] = useState("");
  const [reassignAddress, setReassignAddress] = useState("");
  const [reassignLoading, setReassignLoading] = useState(false);

  const refreshPatch = useCallback(async (): Promise<PatchView> => {
    const view = await listPatch();
    setPatch(view);
    return view;
  }, []);

  useEffect(() => {
    void (async () => {
      try {
        await refreshPatch();
        setError(null);
      } catch (err) {
        setError(errorMessage(err));
      } finally {
        setListLoading(false);
      }
    })();
  }, [refreshPatch]);

  useEffect(() => {
    void listLocalFixtures().then((view) => setLibraryRows(view.rows));
  }, []);

  const rows = useMemo(() => buildFixtureRows(patch, libraryRows), [patch, libraryRows]);

  const handleOpenAddForm = () => {
    setShowAddForm(true);
    setSelectedFixture(null);
    setMode("");
    setQuantity("1");
    setStartUniverse("");
    setStartAddress("");
    setPendingPreview(null);
  };

  const handleCancelAddForm = () => {
    setShowAddForm(false);
    setPendingPreview(null);
  };

  // addFormDialogRef/the two effects below mirror ScriptRunDialog.tsx's
  // established backdrop+dialog pattern exactly (no Radix, plain custom
  // dialog): focus moves onto the dialog surface itself on open, and
  // Escape closes it, both only while it's actually open.
  const addFormDialogRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (showAddForm) {
      addFormDialogRef.current?.focus();
    }
  }, [showAddForm]);

  useEffect(() => {
    if (!showAddForm) {
      return;
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        handleCancelAddForm();
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [showAddForm]);

  const handleSelectFixture = (stableKey: string) => {
    const row = libraryRows.find((candidate) => candidate.stableKey === stableKey) ?? null;
    setSelectedFixture(row);
    setMode(row?.modes[0] ?? "");
    setPendingPreview(null);
  };

  const handleReviewImpact = async () => {
    if (!selectedFixture || !mode) {
      return;
    }
    const qty = parsePositiveInt(quantity);
    if (qty < 1) {
      setError("Quantity must be at least 1.");
      return;
    }

    setPreviewLoading(true);
    setError(null);
    try {
      let current = await refreshPatch();

      let activeDeployment = current.deployments.find((d) => d.active);
      if (!activeDeployment && current.deployments.length > 0) {
        assertOk(await activateDeployment(current.deployments[0].name), "ActivateDeployment");
        current = await refreshPatch();
        activeDeployment = current.deployments.find((d) => d.active);
      }
      if (!activeDeployment) {
        assertOk(await createDeployment("Default"), "CreateDeployment");
        assertOk(await activateDeployment("Default"), "ActivateDeployment");
        current = await refreshPatch();
        activeDeployment = current.deployments.find((d) => d.active);
      }
      if (!activeDeployment) {
        throw new Error("no active deployment available after bootstrap");
      }

      const poolName = fixtureDisplayName(selectedFixture);
      let targetPool = current.pools.find((p) => p.name === poolName);
      if (!targetPool) {
        assertOk(await createPool(poolName, []), "CreatePool");
        current = await refreshPatch();
        targetPool = current.pools.find((p) => p.name === poolName);
      }
      if (!targetPool) {
        throw new Error(`pool ${poolName} was not found after creation`);
      }

      const result = await addPoolMembersPreview(
        targetPool.name,
        selectedFixture.stableKey,
        selectedFixture.contentHash,
        mode,
        qty,
        activeDeployment.id,
        parsePositiveInt(startUniverse),
        parsePositiveInt(startAddress),
        selectedFixture.modeChannelCounts[mode] ?? 0,
      );
      assertOk(result, "AddPoolMembersPreview");
      const plan = JSON.parse(result.stdout) as ImpactPlan;
      setPendingPreview(plan);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setPreviewLoading(false);
    }
  };

  const handleApply = async () => {
    if (!pendingPreview) {
      return;
    }
    setApplyLoading(true);
    try {
      const result = await applyPatch(pendingPreview.plan_id);
      assertOk(result, "ApplyPatch");
      setPendingPreview(null);
      setShowAddForm(false);
      await refreshPatch();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setApplyLoading(false);
    }
  };

  const handleStartRemove = async (poolName: string, poolMemberId: string) => {
    setRemoveTarget({ poolName, poolMemberId });
    setPendingRemovePreview(null);
    setRemovePreviewLoading(true);
    try {
      const result = await removePoolMemberPreview(poolName, poolMemberId);
      assertOk(result, "RemovePoolMemberPreview");
      const plan = JSON.parse(result.stdout) as ImpactPlan;
      setPendingRemovePreview(plan);
      setError(null);
    } catch (err) {
      setError(errorMessage(err));
      setRemoveTarget(null);
    } finally {
      setRemovePreviewLoading(false);
    }
  };

  const handleApplyRemove = async () => {
    if (!pendingRemovePreview) {
      return;
    }
    setRemoveApplyLoading(true);
    try {
      const result = await applyPatch(pendingRemovePreview.plan_id);
      assertOk(result, "ApplyPatch");
      setPendingRemovePreview(null);
      setRemoveTarget(null);
      await refreshPatch();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setRemoveApplyLoading(false);
    }
  };

  const handleCancelRemove = () => {
    setRemoveTarget(null);
    setPendingRemovePreview(null);
  };

  const handleStartReassign = (row: FixtureRow) => {
    setReassigningInstanceId(row.key);
    setReassignName(row.displayName);
    setReassignMode(row.mode);
    setReassignUniverse(String(row.universe));
    setReassignAddress(String(row.address));
  };

  // handleSaveReassign commits both edits the reassign row exposes: the
  // displayed name (renamePool against the row's own poolName, only when
  // it actually changed) and mode/universe/address (reassignInstance) --
  // one Save button, two backend calls, since a rename and a re-patch are
  // independent mutations with no shared preview/apply step.
  const handleSaveReassign = async (row: FixtureRow) => {
    const universe = Number(reassignUniverse);
    const address = Number(reassignAddress);
    const trimmedName = reassignName.trim();
    if (
      !reassignMode ||
      trimmedName === "" ||
      !Number.isFinite(universe) ||
      !Number.isFinite(address) ||
      universe < 1 ||
      address < 1
    ) {
      return;
    }
    setReassignLoading(true);
    try {
      if (trimmedName !== row.poolName) {
        assertOk(await renamePool(row.poolName, trimmedName), "RenamePool");
      }
      const result = await reassignInstance(row.deploymentName, row.key, reassignMode, universe, address);
      assertOk(result, "ReassignInstance");
      setReassigningInstanceId(null);
      await refreshPatch();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setReassignLoading(false);
    }
  };

  const handleCancelReassign = () => {
    setReassigningInstanceId(null);
  };

  const modesForFixture = (fixtureStableKey: string): string[] => {
    const row = libraryRows.find((candidate) => candidate.stableKey === fixtureStableKey);
    return row?.modes ?? [];
  };

  return (
    <section className={styles.panel} aria-label="Project fixtures" aria-busy={listLoading}>
      {listLoading ? (
        <div className={styles.skeleton}>Loading project fixtures…</div>
      ) : (
        <>
          {error && <p className={styles.errorText}>{error}</p>}

          <div className={styles.subsection}>
            <div className={styles.createRow}>
              <span className={styles.countSummary}>
                {rows.length} fixture{rows.length === 1 ? "" : "s"} in this project
              </span>
              {!showAddForm && (
                <button type="button" className={styles.primaryButton} onClick={handleOpenAddForm}>
                  <Plus size={14} aria-hidden="true" />
                  Add from Library
                </button>
              )}
            </div>

            {rows.length === 0 ? (
              <div className={styles.emptyState}>
                <p className={styles.emptyHeading}>
                  <PackagePlus size={18} aria-hidden="true" />
                  No fixtures added yet
                </p>
                <p className={styles.emptyBody}>
                  Add a fixture from the library to patch it into the show with a universe and address.
                </p>
              </div>
            ) : (
              <ul className={styles.rowScroll} aria-label="Project fixture list">
                {rows.map((row) => (
                  <li
                    key={row.key}
                    className={
                      reassigningInstanceId === row.key ? `${styles.row} ${styles.rowEditing}` : styles.row
                    }
                  >
                    {reassigningInstanceId === row.key ? (
                      <>
                        <label className={`${styles.fieldLabel} ${styles.nameField}`}>
                          Name
                          <input
                            className={styles.createInput}
                            value={reassignName}
                            onChange={(event) => setReassignName(event.target.value)}
                            aria-label="Fixture name"
                          />
                        </label>
                        <label className={styles.fieldLabel}>
                          Mode
                          <select
                            className={styles.createInput}
                            value={reassignMode}
                            onChange={(event) => setReassignMode(event.target.value)}
                          >
                            <option value={reassignMode}>{reassignMode}</option>
                            {modesForFixture(row.fixtureStableKey)
                              .filter((modeOption) => modeOption !== reassignMode)
                              .map((modeOption) => (
                                <option key={modeOption} value={modeOption}>
                                  {modeOption}
                                </option>
                              ))}
                          </select>
                        </label>
                        <label className={styles.fieldLabel}>
                          Universe
                          <NumberStepper
                            value={reassignUniverse}
                            onChange={setReassignUniverse}
                            label="Universe"
                          />
                        </label>
                        <label className={styles.fieldLabel}>
                          Address
                          <NumberStepper value={reassignAddress} onChange={setReassignAddress} label="Address" />
                        </label>
                        <button
                          type="button"
                          className={`${styles.primaryButton} ${styles.rowFormButton}`}
                          disabled={reassignLoading}
                          onClick={() => void handleSaveReassign(row)}
                          aria-label={reassignLoading ? "Saving…" : "Save"}
                        >
                          <Check size={13} aria-hidden="true" />
                        </button>
                        <button
                          type="button"
                          className={`${styles.secondaryButton} ${styles.rowFormButton}`}
                          onClick={handleCancelReassign}
                          aria-label="Cancel"
                        >
                          <X size={13} aria-hidden="true" />
                        </button>
                      </>
                    ) : (
                      <>
                        <span className={styles.rowName} title={row.displayName}>
                          {row.displayName}
                        </span>
                        <MetaBadge
                          label="ID"
                          kind="id"
                          // UUIDv7's leading bits encode a millisecond
                          // timestamp, so members minted in the same
                          // batch add (like these 5 rows) share an
                          // identical prefix -- slice(0, 8) collided
                          // across every row here. The trailing
                          // characters are the actually-random portion,
                          // so slice(-8) is the part that's unique per
                          // instance.
                          value={row.poolMemberId ? row.poolMemberId.slice(-8) : "—"}
                          title={`Fixture unit ${row.poolMemberId}`}
                        />
                        <span className={styles.rowSpacer} aria-hidden="true" />
                        <MetaBadge label="Mode" kind="mode" value={row.mode} />
                        <MetaBadge label="Universe" kind="universe" value={String(row.universe)} />
                        <MetaBadge label="Address" kind="address" value={String(row.address)} />
                        <MetaBadge label="Deployment" kind="deployment" value={row.deploymentName} />
                        <button
                          type="button"
                          className={`${styles.secondaryButton} ${styles.rowFormButton}`}
                          onClick={() => handleStartReassign(row)}
                          aria-label={`Edit ${row.displayName}`}
                        >
                          <Pencil size={13} aria-hidden="true" />
                        </button>
                        <button
                          type="button"
                          className={`${styles.secondaryButton} ${styles.rowFormButton}`}
                          onClick={() => void handleStartRemove(row.poolName, row.poolMemberId)}
                          aria-label={`Remove ${row.displayName}`}
                        >
                          <X size={13} aria-hidden="true" />
                        </button>
                      </>
                    )}

                    {removeTarget?.poolName === row.poolName && removeTarget.poolMemberId === row.poolMemberId && (
                      <div className={styles.previewPanel}>
                        {removePreviewLoading ? (
                          <p className={styles.previewRow}>Reviewing…</p>
                        ) : (
                          pendingRemovePreview && (
                            <>
                              <p className={styles.previewHeading}>
                                Impact Preview (plan{" "}
                                <span className={styles.technical}>
                                  {pendingRemovePreview.plan_id.slice(0, 12)}
                                </span>
                                )
                              </p>
                              <ul className={styles.previewList}>
                                {(pendingRemovePreview.operations ?? [])
                                  .filter((op) => op.action === "remove")
                                  .map((op, index) => (
                                    <li key={`${op.dependent_id}-${index}`} className={styles.previewRow}>
                                      {op.dependent_kind === "deployment_instance"
                                        ? `${op.dependent_ref}: deployment instance removed`
                                        : `${op.dependent_ref}: group member removed`}
                                    </li>
                                  ))}
                              </ul>
                              <div className={styles.formActions}>
                                <button
                                  type="button"
                                  className={styles.primaryButton}
                                  disabled={removeApplyLoading}
                                  onClick={() => void handleApplyRemove()}
                                >
                                  <Check size={14} aria-hidden="true" />
                                  {removeApplyLoading ? "Applying…" : "Apply"}
                                </button>
                                <button type="button" className={styles.secondaryButton} onClick={handleCancelRemove}>
                                  <X size={13} aria-hidden="true" />
                                  Cancel
                                </button>
                              </div>
                            </>
                          )
                        )}
                      </div>
                    )}
                  </li>
                ))}
              </ul>
            )}

            {showAddForm && (
              <div className={styles.backdrop} onClick={handleCancelAddForm}>
                <div
                  ref={addFormDialogRef}
                  className={styles.dialog}
                  role="dialog"
                  aria-modal="true"
                  aria-label="Add fixture from library"
                  tabIndex={-1}
                  onClick={(event) => event.stopPropagation()}
                >
                  <div className={styles.dialogHeader}>
                    <span className={styles.dialogTitle}>
                      <PackagePlus size={16} aria-hidden="true" />
                      Add from Library
                    </span>
                    <button
                      type="button"
                      className={styles.secondaryButton}
                      onClick={handleCancelAddForm}
                      aria-label="Close"
                    >
                      <X size={13} aria-hidden="true" />
                    </button>
                  </div>
                  <div className={styles.dialogBody}>
                    <div className={styles.addForm}>
                      <select
                        className={styles.createInput}
                        value={selectedFixture?.stableKey ?? ""}
                        onChange={(event) => handleSelectFixture(event.target.value)}
                        aria-label="Fixture"
                      >
                        <option value="" disabled>
                          {libraryRows.filter((row) => row.status === "valid").length === 0
                            ? "No fixtures in library -- import one first"
                            : "Select a fixture…"}
                        </option>
                        {libraryRows
                          .filter((row) => row.status === "valid")
                          .map((row) => (
                            <option key={row.stableKey} value={row.stableKey}>
                              {row.manufacturer} {row.model}
                            </option>
                          ))}
                      </select>
                      <select
                        className={styles.createInput}
                        value={mode}
                        onChange={(event) => setMode(event.target.value)}
                        aria-label="Fixture mode"
                        disabled={!selectedFixture}
                      >
                        <option value="" disabled>
                          Select a mode…
                        </option>
                        {(selectedFixture?.modes ?? []).map((modeOption) => (
                          <option key={modeOption} value={modeOption}>
                            {modeOption}
                          </option>
                        ))}
                      </select>
                      <label className={styles.fieldLabel}>
                        Quantity
                        <NumberStepper value={quantity} onChange={setQuantity} label="Quantity" />
                      </label>
                      <label className={styles.fieldLabel}>
                        Starting universe (optional)
                        <NumberStepper
                          value={startUniverse}
                          onChange={setStartUniverse}
                          label="Starting universe"
                          placeholder="Auto"
                        />
                      </label>
                      <label className={styles.fieldLabel}>
                        Starting address (optional)
                        <NumberStepper
                          value={startAddress}
                          onChange={setStartAddress}
                          label="Starting address"
                          placeholder="Auto"
                        />
                      </label>

                      <div className={styles.formActions}>
                        <button
                          type="button"
                          className={styles.primaryButton}
                          disabled={previewLoading || !selectedFixture || !mode}
                          onClick={() => void handleReviewImpact()}
                        >
                          <Eye size={14} aria-hidden="true" />
                          {previewLoading ? "Reviewing…" : "Review Impact"}
                        </button>
                        <button type="button" className={styles.secondaryButton} onClick={handleCancelAddForm}>
                          <X size={13} aria-hidden="true" />
                          Cancel
                        </button>
                      </div>

                      {pendingPreview && (
                        <div className={styles.previewPanel}>
                          <p className={styles.previewHeading}>
                            Impact Preview (plan{" "}
                            <span className={styles.technical}>{pendingPreview.plan_id.slice(0, 12)}</span>)
                          </p>
                          <ul className={styles.previewList}>
                            {(pendingPreview.operations ?? [])
                              .filter((op) => op.dependent_kind === "deployment_instance" && op.action === "add")
                              .map((op, index) => (
                                <li key={`${op.dependent_id}-${index}`} className={styles.previewRow}>
                                  {op.dependent_ref} → Universe{" "}
                                  <span className={styles.technical}>{op.proposed_universe}</span>, Address{" "}
                                  <span className={styles.technical}>{op.proposed_address}</span>
                                </li>
                              ))}
                          </ul>
                          {(pendingPreview.warnings ?? []).length > 0 && (
                            <ul className={styles.previewList}>
                              {pendingPreview.warnings?.map((warning, index) => (
                                <li key={`warning-${index}`} className={styles.previewWarning}>
                                  {warning.code}: {warning.message}
                                </li>
                              ))}
                            </ul>
                          )}
                          {(pendingPreview.errors ?? []).length > 0 && (
                            <ul className={styles.previewList}>
                              {pendingPreview.errors?.map((planError, index) => (
                                <li key={`error-${index}`} className={styles.previewError}>
                                  {planError.code}: {planError.message}
                                </li>
                              ))}
                            </ul>
                          )}
                          <div className={styles.formActions}>
                            <button
                              type="button"
                              className={styles.primaryButton}
                              disabled={applyLoading || (pendingPreview.errors ?? []).length > 0}
                              onClick={() => void handleApply()}
                            >
                              <Check size={14} aria-hidden="true" />
                              {applyLoading ? "Applying…" : "Apply"}
                            </button>
                            <button
                              type="button"
                              className={styles.secondaryButton}
                              onClick={() => setPendingPreview(null)}
                            >
                              <X size={13} aria-hidden="true" />
                              Cancel
                            </button>
                          </div>
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              </div>
            )}
          </div>
        </>
      )}
    </section>
  );
}
