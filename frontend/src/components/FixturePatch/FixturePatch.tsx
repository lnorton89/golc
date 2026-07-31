// FixturePatch.tsx is the on-screen fixture-patch surface closing
// VERIFICATION.md Gap B[0] for PLAY-10 (06-10-PLAN.md): a show author
// creates logical fixture pools, adds a fixture to a pool at a concrete
// mode -- always reviewing the backend's own non-committing impact
// preview (each affected deployment instance's system-computed
// proposed_universe/proposed_address) before an explicit Apply commit --
// and creates/activates deployments, all driving the exact same
// "pool"/"deployment" CLI routes internal/command/pool.go and
// internal/command/deployment.go already implement and test. This is a
// UI-binding exercise against a stable backend, architecturally identical
// to 06-07's OperatorSurface.tsx/SurfaceService wiring.
//
// All Go-bound calls go through frontend/src/lib/wailsBridge.ts's
// FixturePatchService helpers (createPool/addPoolMemberPreview/
// removePoolMemberPreview/applyPatch/createDeployment/activateDeployment/
// listPatch) -- this file never re-declares `declare global` itself (the
// same Wave-3 post-merge collision ArtnetConfig.tsx/SceneProgramming.tsx's
// own comments document) and never adds a second pool/deployment mutation
// path.
//
// Universe/address are never manually entered anywhere in this component
// (06-10-PLAN.md's flagged assumption): every displayed universe/address is
// the backend's own system-computed value, surfaced in the impact preview
// (proposed_universe/proposed_address) and in the deployment/instance list
// (persisted Instance.Universe/Address) -- never a second, GUI-owned
// addressing calculation. The add-fixture control's own stable key/content
// hash/mode triple is likewise never hand-typed: it is picked from
// FixtureLibraryService.ListLocal's already-pinned rows (loaded fresh each
// time "Add Fixture" opens), with the mode dropdown scoped to the exact
// fixture selected -- no "fixture inspect" copy/paste round trip.
//
// State coverage (Task 3, 06-UI-SPEC.md-style backstop): listLoading
// renders a skeleton placeholder; a failed bridge call's own stderr
// diagnostic (e.g. a stale/unknown-plan-id ApplyPatch rejection,
// GOLC_POOL_PLAN_STALE/GOLC_WAILS_PLAN_UNKNOWN) surfaces verbatim in the
// error banner, never a silent failure; pool/deployment lists render an
// explicit empty state with correct singular/plural counts; and the pool/
// deployment/member/preview lists all scroll within a fixed-height panel
// (FixturePatch.module.css's rowScroll/memberList/previewList) rather than
// growing the window against a representative large show. The full
// create-pool -> preview -> apply -> create/activate-deployment click-
// through against a real golc-desktop build is queued as a human-check
// for end-of-phase UAT (workflow.human_verify_mode=end-of-phase) rather
// than an interactive mid-execution checkpoint.

import { useCallback, useEffect, useState } from "react";
import { Plus, Eye, X, Check, Zap, Package, Boxes, Pencil, Trash2 } from "lucide-react";

import {
  activateDeployment,
  addPoolMemberPreview,
  applyPatch,
  assertOk,
  createDeployment,
  createPool,
  deleteDeployment,
  deletePool,
  errorMessage,
  listLocalFixtures,
  listPatch,
  offlinePatchView,
  reassignInstance,
  removePoolMemberPreview,
  renameDeployment,
  renamePool,
  type FixtureLibraryRowView,
  type PatchView,
} from "../../lib/wailsBridge";
import styles from "./FixturePatch.module.css";

// ---------------------------------------------------------------------------
// Types (ImpactPlan/ImpactOperation mirror internal/pool/impact.go's own
// snake_case json tags exactly, since AddPoolMemberPreview/
// RemovePoolMemberPreview return that plan's raw canonical encoding
// verbatim in Result.stdout -- never re-cased through the camelCase
// convention wailsBridge.ts's PatchView/etc. types use); PatchView and its
// nested view types are declared once, centrally, in wailsBridge.ts (WR-01)
// -- imported above rather than re-declared here.
// ---------------------------------------------------------------------------

interface ImpactOperation {
  dependent_kind: string;
  dependent_ref: string;
  dependent_id: string;
  action: string;
  pool_member_index: number;
  pool_member_id: string;
  proposed_universe?: number;
  proposed_address?: number;
  status: string;
}

interface ImpactPlan {
  schema_version: number;
  pool_id: string;
  add?: { fixture_stable_key: string; fixture_content_hash: string; mode: string }[];
  remove?: string[];
  propagate: string;
  expected_revision: number;
  // internal/pool/impact.go's own Operations field carries no `omitempty`
  // and is left as a nil slice (never explicitly initialized to []) when
  // no deployment references the pool yet -- encoding/json marshals that
  // as JSON null, not []. Every read of this field must go through
  // `?? []`, mirroring warnings/errors below (T-FIXPATCH-NULL-OPS).
  operations: ImpactOperation[] | null;
  warnings?: { code: string; message: string }[];
  errors?: { code: string; message: string }[];
  plan_id: string;
}

function parseRequires(raw: string): string[] {
  return raw
    .split(",")
    .map((entry) => entry.trim())
    .filter((entry) => entry.length > 0);
}

interface PendingPreview {
  poolName: string;
  plan: ImpactPlan;
}

interface PendingRemovePreview {
  poolName: string;
  memberId: string;
  plan: ImpactPlan;
}

export default function FixturePatch() {
  const [patch, setPatch] = useState<PatchView>(offlinePatchView());
  const [listLoading, setListLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [newPoolName, setNewPoolName] = useState("");
  const [newPoolRequires, setNewPoolRequires] = useState("");

  const [addPoolTarget, setAddPoolTarget] = useState<string | null>(null);
  const [libraryRows, setLibraryRows] = useState<FixtureLibraryRowView[]>([]);
  const [selectedFixture, setSelectedFixture] = useState<FixtureLibraryRowView | null>(null);
  const [addMode, setAddMode] = useState("");
  const [previewLoading, setPreviewLoading] = useState(false);
  const [pendingPreview, setPendingPreview] = useState<PendingPreview | null>(null);
  const [applyLoading, setApplyLoading] = useState(false);

  const [newDeploymentName, setNewDeploymentName] = useState("");

  const [renamingPoolId, setRenamingPoolId] = useState<string | null>(null);
  const [renamePoolValue, setRenamePoolValue] = useState("");
  const [renamingDeploymentId, setRenamingDeploymentId] = useState<string | null>(null);
  const [renameDeploymentValue, setRenameDeploymentValue] = useState("");

  const [removeTarget, setRemoveTarget] = useState<{ poolName: string; memberId: string } | null>(null);
  const [removePreviewLoading, setRemovePreviewLoading] = useState(false);
  const [pendingRemovePreview, setPendingRemovePreview] = useState<PendingRemovePreview | null>(null);
  const [removeApplyLoading, setRemoveApplyLoading] = useState(false);

  const [reassigningInstanceId, setReassigningInstanceId] = useState<string | null>(null);
  const [reassignMode, setReassignMode] = useState("");
  const [reassignUniverse, setReassignUniverse] = useState("");
  const [reassignAddress, setReassignAddress] = useState("");
  const [reassignLoading, setReassignLoading] = useState(false);

  const refreshPatch = useCallback(async (): Promise<void> => {
    try {
      const view = await listPatch();
      setPatch(view);
      setError(null);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setListLoading(false);
    }
  }, []);

  useEffect(() => {
    void refreshPatch();
  }, [refreshPatch]);

  useEffect(() => {
    void listLocalFixtures().then((view) => setLibraryRows(view.rows));
  }, []);

  const handleCreatePool = async () => {
    const trimmed = newPoolName.trim();
    if (trimmed === "") {
      return;
    }
    try {
      const result = await createPool(trimmed, parseRequires(newPoolRequires));
      assertOk(result, "CreatePool");
      setNewPoolName("");
      setNewPoolRequires("");
      await refreshPatch();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleStartAddMember = (poolName: string) => {
    setAddPoolTarget(poolName);
    setSelectedFixture(null);
    setAddMode("");
    setPendingPreview(null);
  };

  const handleSelectFixture = (stableKey: string) => {
    const row = libraryRows.find((candidate) => candidate.stableKey === stableKey) ?? null;
    setSelectedFixture(row);
    setAddMode(row?.modes[0] ?? "");
  };

  const handlePreviewAddMember = async () => {
    if (!addPoolTarget || !selectedFixture || !addMode) {
      return;
    }
    setPreviewLoading(true);
    try {
      const result = await addPoolMemberPreview(
        addPoolTarget,
        selectedFixture.stableKey,
        selectedFixture.contentHash,
        addMode,
      );
      assertOk(result, "AddPoolMemberPreview");
      const plan = JSON.parse(result.stdout) as ImpactPlan;
      setPendingPreview({ poolName: addPoolTarget, plan });
      setError(null);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setPreviewLoading(false);
    }
  };

  const handleApplyPreview = async () => {
    if (!pendingPreview) {
      return;
    }
    setApplyLoading(true);
    try {
      const result = await applyPatch(pendingPreview.plan.plan_id);
      assertOk(result, "ApplyPatch");
      setPendingPreview(null);
      setAddPoolTarget(null);
      await refreshPatch();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setApplyLoading(false);
    }
  };

  const handleCancelPreview = () => {
    setPendingPreview(null);
  };

  const handleCreateDeployment = async () => {
    const trimmed = newDeploymentName.trim();
    if (trimmed === "") {
      return;
    }
    try {
      const result = await createDeployment(trimmed);
      assertOk(result, "CreateDeployment");
      setNewDeploymentName("");
      await refreshPatch();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleActivateDeployment = async (name: string) => {
    try {
      const result = await activateDeployment(name);
      assertOk(result, "ActivateDeployment");
      await refreshPatch();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleStartRenamePool = (id: string, currentName: string) => {
    setRenamingPoolId(id);
    setRenamePoolValue(currentName);
  };

  const handleSaveRenamePool = async (currentName: string) => {
    const trimmed = renamePoolValue.trim();
    if (trimmed === "" || trimmed === currentName) {
      setRenamingPoolId(null);
      return;
    }
    try {
      const result = await renamePool(currentName, trimmed);
      assertOk(result, "RenamePool");
      setRenamingPoolId(null);
      await refreshPatch();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleDeletePool = async (name: string, memberCount: number, instanceCount: number) => {
    const confirmed = window.confirm(
      `Delete pool "${name}"? This removes ${memberCount} member${memberCount === 1 ? "" : "s"} and unpatches ${instanceCount} instance${instanceCount === 1 ? "" : "s"}.`,
    );
    if (!confirmed) {
      return;
    }
    try {
      const result = await deletePool(name);
      assertOk(result, "DeletePool");
      await refreshPatch();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleStartRenameDeployment = (id: string, currentName: string) => {
    setRenamingDeploymentId(id);
    setRenameDeploymentValue(currentName);
  };

  const handleSaveRenameDeployment = async (currentName: string) => {
    const trimmed = renameDeploymentValue.trim();
    if (trimmed === "" || trimmed === currentName) {
      setRenamingDeploymentId(null);
      return;
    }
    try {
      const result = await renameDeployment(currentName, trimmed);
      assertOk(result, "RenameDeployment");
      setRenamingDeploymentId(null);
      await refreshPatch();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleDeleteDeployment = async (name: string, instanceCount: number) => {
    const confirmed = window.confirm(
      `Delete deployment "${name}"? This removes ${instanceCount} instance${instanceCount === 1 ? "" : "s"}.`,
    );
    if (!confirmed) {
      return;
    }
    try {
      const result = await deleteDeployment(name);
      assertOk(result, "DeleteDeployment");
      await refreshPatch();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleStartRemoveMember = async (poolName: string, memberId: string) => {
    setRemoveTarget({ poolName, memberId });
    setPendingRemovePreview(null);
    setRemovePreviewLoading(true);
    try {
      const result = await removePoolMemberPreview(poolName, memberId);
      assertOk(result, "RemovePoolMemberPreview");
      const plan = JSON.parse(result.stdout) as ImpactPlan;
      setPendingRemovePreview({ poolName, memberId, plan });
      setError(null);
    } catch (err) {
      setError(errorMessage(err));
      setRemoveTarget(null);
    } finally {
      setRemovePreviewLoading(false);
    }
  };

  const handleApplyRemoveMember = async () => {
    if (!pendingRemovePreview) {
      return;
    }
    setRemoveApplyLoading(true);
    try {
      const result = await applyPatch(pendingRemovePreview.plan.plan_id);
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

  const handleCancelRemoveMember = () => {
    setRemoveTarget(null);
    setPendingRemovePreview(null);
  };

  const handleStartReassign = (instanceId: string, mode: string, universe: number, address: number) => {
    setReassigningInstanceId(instanceId);
    setReassignMode(mode);
    setReassignUniverse(String(universe));
    setReassignAddress(String(address));
  };

  const handleSaveReassign = async (deploymentName: string, instanceId: string) => {
    const universe = Number(reassignUniverse);
    const address = Number(reassignAddress);
    if (!reassignMode || !Number.isFinite(universe) || !Number.isFinite(address) || universe < 1 || address < 1) {
      return;
    }
    setReassignLoading(true);
    try {
      const result = await reassignInstance(deploymentName, instanceId, reassignMode, universe, address);
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

  const modesForMember = (fixtureStableKey: string): string[] => {
    const row = libraryRows.find((candidate) => candidate.stableKey === fixtureStableKey);
    return row?.modes ?? [];
  };

  const instanceCountForPool = (poolId: string): number =>
    patch.deployments.reduce(
      (count, d) => count + d.instances.filter((instance) => instance.poolId === poolId).length,
      0,
    );

  const pools = patch.pools;
  const deployments = patch.deployments;

  return (
    <section
      className={styles.panel}
      aria-label="Fixture patch"
      aria-busy={listLoading}
    >
      <h2 className={styles.sectionHeading}>Fixture Patch</h2>

      {listLoading ? (
        <div className={styles.skeleton}>Loading fixture patch…</div>
      ) : (
        <>
          {error && <p className={styles.errorText}>{error}</p>}

          {/* Pools */}
          <div className={styles.subsection}>
            <h3 className={styles.subsectionHeading}>Pools</h3>
            <div className={styles.createRow}>
              <input
                className={styles.createInput}
                type="text"
                value={newPoolName}
                placeholder="New pool name"
                onChange={(event) => setNewPoolName(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    void handleCreatePool();
                  }
                }}
                aria-label="New pool name"
              />
              <input
                className={styles.createInput}
                type="text"
                value={newPoolRequires}
                placeholder="Required capabilities (comma-separated, optional)"
                onChange={(event) => setNewPoolRequires(event.target.value)}
                aria-label="Required capabilities"
              />
              <button
                type="button"
                className={styles.primaryButton}
                onClick={() => void handleCreatePool()}
              >
                <Plus size={14} aria-hidden="true" />
                Create Pool
              </button>
            </div>

            {pools.length === 0 ? (
              <div className={styles.emptyState}>
                <p className={styles.emptyHeading}>
                  <Package size={18} aria-hidden="true" />
                  No fixture pools yet
                </p>
                <p className={styles.emptyBody}>
                  Create a pool, then add a fixture at a mode to patch it into a
                  deployment.
                </p>
              </div>
            ) : (
              <>
                <p className={styles.countSummary}>
                  {pools.length} pool{pools.length === 1 ? "" : "s"}
                </p>
                <ul className={styles.rowScroll} aria-label="Pool list">
                  {pools.map((p) => (
                    <li key={p.id} className={styles.row}>
                      <div className={styles.rowHeader}>
                        {renamingPoolId === p.id ? (
                          <>
                            <input
                              className={styles.createInput}
                              type="text"
                              value={renamePoolValue}
                              onChange={(event) => setRenamePoolValue(event.target.value)}
                              onKeyDown={(event) => {
                                if (event.key === "Enter") {
                                  void handleSaveRenamePool(p.name);
                                }
                                if (event.key === "Escape") {
                                  setRenamingPoolId(null);
                                }
                              }}
                              aria-label="Pool name"
                              autoFocus
                            />
                            <button
                              type="button"
                              className={styles.secondaryButton}
                              onClick={() => void handleSaveRenamePool(p.name)}
                            >
                              <Check size={13} aria-hidden="true" />
                            </button>
                            <button
                              type="button"
                              className={styles.secondaryButton}
                              onClick={() => setRenamingPoolId(null)}
                            >
                              <X size={13} aria-hidden="true" />
                            </button>
                          </>
                        ) : (
                          <>
                            <span className={styles.rowName} title={p.name}>
                              {p.name}
                            </span>
                            <span className={styles.rowCounts}>
                              {p.members.length} member
                              {p.members.length === 1 ? "" : "s"}
                            </span>
                            <button
                              type="button"
                              className={styles.secondaryButton}
                              onClick={() => handleStartRenamePool(p.id, p.name)}
                              aria-label={`Rename ${p.name}`}
                            >
                              <Pencil size={13} aria-hidden="true" />
                            </button>
                            <button
                              type="button"
                              className={styles.secondaryButton}
                              onClick={() => void handleDeletePool(p.name, p.members.length, instanceCountForPool(p.id))}
                              aria-label={`Delete ${p.name}`}
                            >
                              <Trash2 size={13} aria-hidden="true" />
                            </button>
                            <button
                              type="button"
                              className={styles.secondaryButton}
                              onClick={() => handleStartAddMember(p.name)}
                            >
                              <Plus size={13} aria-hidden="true" />
                              Add Fixture
                            </button>
                          </>
                        )}
                      </div>
                      {p.members.length > 0 && (
                        <ul className={styles.memberList}>
                          {p.members.map((m) => (
                            <li key={m.id} className={styles.memberRow}>
                              <span className={styles.technical}>
                                {m.fixtureStableKey}
                              </span>
                              <button
                                type="button"
                                className={styles.secondaryButton}
                                onClick={() => void handleStartRemoveMember(p.name, m.id)}
                              >
                                <X size={13} aria-hidden="true" />
                                Remove
                              </button>
                              {removeTarget?.poolName === p.name && removeTarget.memberId === m.id && (
                                <div className={styles.previewPanel}>
                                  {removePreviewLoading ? (
                                    <p className={styles.previewRow}>Reviewing…</p>
                                  ) : (
                                    pendingRemovePreview && (
                                      <>
                                        <p className={styles.previewHeading}>
                                          Impact Preview (plan{" "}
                                          <span className={styles.technical}>
                                            {pendingRemovePreview.plan.plan_id.slice(0, 12)}
                                          </span>
                                          )
                                        </p>
                                        <ul className={styles.previewList}>
                                          {(pendingRemovePreview.plan.operations ?? [])
                                            .filter((op) => op.action === "remove")
                                            .map((op, index) => (
                                              <li key={`${op.dependent_id}-${index}`} className={styles.previewRow}>
                                                {op.dependent_kind === "deployment_instance"
                                                  ? `${op.dependent_ref}: deployment instance removed`
                                                  : `${op.dependent_ref}: group member removed`}
                                              </li>
                                            ))}
                                          {(pendingRemovePreview.plan.operations ?? []).filter(
                                            (op) => op.action === "remove",
                                          ).length === 0 && (
                                            <li className={styles.previewRow}>
                                              Nothing else references this member -- only the pool member itself
                                              is removed.
                                            </li>
                                          )}
                                        </ul>
                                        <div className={styles.formActions}>
                                          <button
                                            type="button"
                                            className={styles.primaryButton}
                                            disabled={removeApplyLoading}
                                            onClick={() => void handleApplyRemoveMember()}
                                          >
                                            <Check size={14} aria-hidden="true" />
                                            {removeApplyLoading ? "Applying…" : "Apply"}
                                          </button>
                                          <button
                                            type="button"
                                            className={styles.secondaryButton}
                                            onClick={handleCancelRemoveMember}
                                          >
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

                      {addPoolTarget === p.name && (
                        <div className={styles.addMemberForm}>
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
                            value={addMode}
                            onChange={(event) => setAddMode(event.target.value)}
                            aria-label="Fixture mode"
                            disabled={!selectedFixture}
                          >
                            <option value="" disabled>
                              Select a mode…
                            </option>
                            {(selectedFixture?.modes ?? []).map((mode) => (
                              <option key={mode} value={mode}>
                                {mode}
                              </option>
                            ))}
                          </select>
                          <div className={styles.formActions}>
                            <button
                              type="button"
                              className={styles.primaryButton}
                              disabled={previewLoading || !selectedFixture || !addMode}
                              onClick={() => void handlePreviewAddMember()}
                            >
                              <Eye size={14} aria-hidden="true" />
                              {previewLoading ? "Reviewing…" : "Review Impact"}
                            </button>
                            <button
                              type="button"
                              className={styles.secondaryButton}
                              onClick={() => setAddPoolTarget(null)}
                            >
                              <X size={13} aria-hidden="true" />
                              Cancel
                            </button>
                          </div>

                          {pendingPreview && pendingPreview.poolName === p.name && (
                            <div className={styles.previewPanel}>
                              <p className={styles.previewHeading}>
                                Impact Preview (plan{" "}
                                <span className={styles.technical}>
                                  {pendingPreview.plan.plan_id.slice(0, 12)}
                                </span>
                                )
                              </p>
                              <ul className={styles.previewList}>
                                {(pendingPreview.plan.operations ?? [])
                                  .filter(
                                    (op) =>
                                      op.dependent_kind === "deployment_instance" &&
                                      op.action === "add",
                                  )
                                  .map((op, index) => (
                                    <li
                                      key={`${op.dependent_id}-${index}`}
                                      className={styles.previewRow}
                                    >
                                      {op.dependent_ref} → Universe{" "}
                                      <span className={styles.technical}>
                                        {op.proposed_universe}
                                      </span>
                                      , Address{" "}
                                      <span className={styles.technical}>
                                        {op.proposed_address}
                                      </span>
                                    </li>
                                  ))}
                                {(pendingPreview.plan.operations ?? []).filter(
                                  (op) =>
                                    op.dependent_kind === "deployment_instance" &&
                                    op.action === "add",
                                ).length === 0 && (
                                  <li className={styles.previewRow}>
                                    No deployment currently references this pool --
                                    nothing to instantiate yet.
                                  </li>
                                )}
                              </ul>
                              {(pendingPreview.plan.warnings ?? []).length > 0 && (
                                <ul className={styles.previewList}>
                                  {pendingPreview.plan.warnings?.map((warning, index) => (
                                    <li
                                      key={`warning-${index}`}
                                      className={styles.previewWarning}
                                    >
                                      {warning.code}: {warning.message}
                                    </li>
                                  ))}
                                </ul>
                              )}
                              {(pendingPreview.plan.errors ?? []).length > 0 && (
                                <ul className={styles.previewList}>
                                  {pendingPreview.plan.errors?.map((planError, index) => (
                                    <li
                                      key={`error-${index}`}
                                      className={styles.previewError}
                                    >
                                      {planError.code}: {planError.message}
                                    </li>
                                  ))}
                                </ul>
                              )}
                              <div className={styles.formActions}>
                                <button
                                  type="button"
                                  className={styles.primaryButton}
                                  disabled={
                                    applyLoading ||
                                    (pendingPreview.plan.errors ?? []).length > 0
                                  }
                                  onClick={() => void handleApplyPreview()}
                                >
                                  <Check size={14} aria-hidden="true" />
                                  {applyLoading ? "Applying…" : "Apply"}
                                </button>
                                <button
                                  type="button"
                                  className={styles.secondaryButton}
                                  onClick={handleCancelPreview}
                                >
                                  <X size={13} aria-hidden="true" />
                                  Cancel
                                </button>
                              </div>
                            </div>
                          )}
                        </div>
                      )}
                    </li>
                  ))}
                </ul>
              </>
            )}
          </div>

          {/* Deployments */}
          <div className={styles.subsection}>
            <h3 className={styles.subsectionHeading}>Deployments</h3>
            <div className={styles.createRow}>
              <input
                className={styles.createInput}
                type="text"
                value={newDeploymentName}
                placeholder="New deployment name"
                onChange={(event) => setNewDeploymentName(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    void handleCreateDeployment();
                  }
                }}
                aria-label="New deployment name"
              />
              <button
                type="button"
                className={styles.primaryButton}
                onClick={() => void handleCreateDeployment()}
              >
                <Plus size={14} aria-hidden="true" />
                Create Deployment
              </button>
            </div>

            {deployments.length === 0 ? (
              <div className={styles.emptyState}>
                <p className={styles.emptyHeading}>
                  <Boxes size={18} aria-hidden="true" />
                  No deployments yet
                </p>
                <p className={styles.emptyBody}>
                  Create a deployment, then activate it to patch pool fixtures
                  into concrete instances.
                </p>
              </div>
            ) : (
              <>
                <p className={styles.countSummary}>
                  {deployments.length} deployment
                  {deployments.length === 1 ? "" : "s"}
                </p>
                <ul className={styles.rowScroll} aria-label="Deployment list">
                  {deployments.map((d) => (
                    <li key={d.id} className={styles.row}>
                      <div className={styles.rowHeader}>
                        {renamingDeploymentId === d.id ? (
                          <>
                            <input
                              className={styles.createInput}
                              type="text"
                              value={renameDeploymentValue}
                              onChange={(event) => setRenameDeploymentValue(event.target.value)}
                              onKeyDown={(event) => {
                                if (event.key === "Enter") {
                                  void handleSaveRenameDeployment(d.name);
                                }
                                if (event.key === "Escape") {
                                  setRenamingDeploymentId(null);
                                }
                              }}
                              aria-label="Deployment name"
                              autoFocus
                            />
                            <button
                              type="button"
                              className={styles.secondaryButton}
                              onClick={() => void handleSaveRenameDeployment(d.name)}
                            >
                              <Check size={13} aria-hidden="true" />
                            </button>
                            <button
                              type="button"
                              className={styles.secondaryButton}
                              onClick={() => setRenamingDeploymentId(null)}
                            >
                              <X size={13} aria-hidden="true" />
                            </button>
                          </>
                        ) : (
                          <>
                            <span className={styles.rowName} title={d.name}>
                              {d.name}
                            </span>
                            {d.active ? (
                              <span className={styles.activeChip}>Active</span>
                            ) : (
                              <button
                                type="button"
                                className={styles.secondaryButton}
                                onClick={() => void handleActivateDeployment(d.name)}
                              >
                                <Zap size={13} aria-hidden="true" />
                                Activate
                              </button>
                            )}
                            <button
                              type="button"
                              className={styles.secondaryButton}
                              onClick={() => handleStartRenameDeployment(d.id, d.name)}
                              aria-label={`Rename ${d.name}`}
                            >
                              <Pencil size={13} aria-hidden="true" />
                            </button>
                            <button
                              type="button"
                              className={styles.secondaryButton}
                              onClick={() => void handleDeleteDeployment(d.name, d.instances.length)}
                              aria-label={`Delete ${d.name}`}
                            >
                              <Trash2 size={13} aria-hidden="true" />
                            </button>
                          </>
                        )}
                      </div>
                      {d.instances.length > 0 && (
                        <ul className={styles.memberList}>
                          {d.instances.map((instance) => {
                            const member = patch.pools
                              .find((p) => p.id === instance.poolId)
                              ?.members.find((m) => m.id === instance.poolMemberId);
                            return (
                              <li key={instance.id} className={styles.memberRow}>
                                {reassigningInstanceId === instance.id ? (
                                  <>
                                    <select
                                      className={styles.createInput}
                                      value={reassignMode}
                                      onChange={(event) => setReassignMode(event.target.value)}
                                      aria-label="Mode"
                                    >
                                      <option value={reassignMode}>{reassignMode}</option>
                                      {modesForMember(member?.fixtureStableKey ?? "")
                                        .filter((mode) => mode !== reassignMode)
                                        .map((mode) => (
                                          <option key={mode} value={mode}>
                                            {mode}
                                          </option>
                                        ))}
                                    </select>
                                    <input
                                      className={styles.createInput}
                                      type="number"
                                      min={1}
                                      value={reassignUniverse}
                                      onChange={(event) => setReassignUniverse(event.target.value)}
                                      aria-label="Universe"
                                    />
                                    <input
                                      className={styles.createInput}
                                      type="number"
                                      min={1}
                                      value={reassignAddress}
                                      onChange={(event) => setReassignAddress(event.target.value)}
                                      aria-label="Address"
                                    />
                                    <button
                                      type="button"
                                      className={styles.primaryButton}
                                      disabled={reassignLoading}
                                      onClick={() => void handleSaveReassign(d.name, instance.id)}
                                    >
                                      <Check size={13} aria-hidden="true" />
                                      {reassignLoading ? "Saving…" : "Save"}
                                    </button>
                                    <button
                                      type="button"
                                      className={styles.secondaryButton}
                                      onClick={handleCancelReassign}
                                    >
                                      <X size={13} aria-hidden="true" />
                                      Cancel
                                    </button>
                                  </>
                                ) : (
                                  <>
                                    <span>{instance.mode}</span>
                                    <span className={styles.technical}>
                                      Universe {instance.universe}, Address{" "}
                                      {instance.address}
                                    </span>
                                    <button
                                      type="button"
                                      className={styles.secondaryButton}
                                      onClick={() =>
                                        handleStartReassign(instance.id, instance.mode, instance.universe, instance.address)
                                      }
                                      aria-label="Edit instance"
                                    >
                                      <Pencil size={13} aria-hidden="true" />
                                    </button>
                                  </>
                                )}
                              </li>
                            );
                          })}
                        </ul>
                      )}
                    </li>
                  ))}
                </ul>
              </>
            )}
          </div>
        </>
      )}
    </section>
  );
}
