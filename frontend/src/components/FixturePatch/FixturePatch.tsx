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
import { Plus, Eye, X, Check, Zap, MoreVertical, Pencil, Trash2 } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";

import { motionTransition } from "../../design-system/motion";
import { useLatestRequest } from "../../hooks/useLatestRequest";
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
import { Button, Chip, Combobox, EmptyState, ErrorState, Field, FormActions, IconButton, ImpactReview, InfoTooltip, LoadingState, Menu, Panel, Select } from "../../design-system";
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

/** addMemberImpacts summarizes the deployment instances an add-member plan
 * will instantiate, for ImpactReview's preview-before-apply contract. */
function addMemberImpacts(plan: ImpactPlan): string[] {
  const additions = (plan.operations ?? [])
    .filter((op) => op.dependent_kind === "deployment_instance" && op.action === "add")
    .map((op) => `${op.dependent_ref} → Universe ${op.proposed_universe}, Address ${op.proposed_address}`);
  return additions.length > 0
    ? additions
    : ["No deployment currently references this pool -- nothing to instantiate yet."];
}

/** removeMemberImpacts summarizes what a remove-member plan will detach, for
 * the same preview-before-apply contract. */
function removeMemberImpacts(plan: ImpactPlan): string[] {
  const removals = (plan.operations ?? [])
    .filter((op) => op.action === "remove")
    .map((op) =>
      op.dependent_kind === "deployment_instance"
        ? `${op.dependent_ref}: deployment instance removed`
        : `${op.dependent_ref}: group member removed`,
    );
  return removals.length > 0
    ? removals
    : ["Nothing else references this member -- only the pool member itself is removed."];
}

const rowExitTransition = motionTransition("settle");

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

  // Both impact-preview round trips are latest-wins (see useLatestRequest).
  const beginLatestPreview = useLatestRequest();
  const beginLatestRemovePreview = useLatestRequest();

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

  // Changing either half of the proposed member invalidates any preview
  // already on screen: the render gate below only checks the pool name, so
  // a preview reviewed for a 4-channel fixture stayed visible after
  // switching to a 32-channel one and Apply committed the FIRST fixture's
  // plan_id at the first footprint. ProjectFixtures.tsx's own
  // handleSelectFixture already did this; this one was the outlier.
  const handleSelectFixture = (stableKey: string) => {
    const row = libraryRows.find((candidate) => candidate.stableKey === stableKey) ?? null;
    setSelectedFixture(row);
    setAddMode(row?.modes[0] ?? "");
    setPendingPreview(null);
  };

  const handleSelectAddMode = (mode: string) => {
    setAddMode(mode);
    setPendingPreview(null);
  };

  const handlePreviewAddMember = async () => {
    if (!addPoolTarget || !selectedFixture || !addMode) {
      return;
    }
    const isCurrent = beginLatestPreview();
    setPreviewLoading(true);
    try {
      const result = await addPoolMemberPreview(
        addPoolTarget,
        selectedFixture.stableKey,
        selectedFixture.contentHash,
        addMode,
        selectedFixture.modeChannelCounts[addMode] ?? 0,
      );
      if (!isCurrent()) {
        return;
      }
      assertOk(result, "AddPoolMemberPreview");
      const plan = JSON.parse(result.stdout) as ImpactPlan;
      setPendingPreview({ poolName: addPoolTarget, plan });
      setError(null);
    } catch (err) {
      if (!isCurrent()) {
        return;
      }
      setError(errorMessage(err));
    } finally {
      if (isCurrent()) {
        setPreviewLoading(false);
      }
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

  // Generation-guarded: clicking Remove on member A and then on member B
  // inside A's round trip used to render A's impact list under B's row and
  // commit A's plan_id on Apply, deleting the wrong member. The render
  // gate below now also compares pendingRemovePreview's own
  // poolName/memberId (which it has always carried) against removeTarget.
  const handleStartRemoveMember = async (poolName: string, memberId: string) => {
    const isCurrent = beginLatestRemovePreview();
    setRemoveTarget({ poolName, memberId });
    setPendingRemovePreview(null);
    setRemovePreviewLoading(true);
    try {
      const result = await removePoolMemberPreview(poolName, memberId);
      if (!isCurrent()) {
        return;
      }
      assertOk(result, "RemovePoolMemberPreview");
      const plan = JSON.parse(result.stdout) as ImpactPlan;
      setPendingRemovePreview({ poolName, memberId, plan });
      setError(null);
    } catch (err) {
      if (!isCurrent()) {
        return;
      }
      setError(errorMessage(err));
      setRemoveTarget(null);
    } finally {
      if (isCurrent()) {
        setRemovePreviewLoading(false);
      }
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
    <Panel aria-label="Fixture patch" aria-busy={listLoading}>
      <div className={styles.content}>
      {listLoading ? (
        <LoadingState label="Fixture patch is loading" variant="panel" />
      ) : (
        <>
          {error && <ErrorState heading="Fixture patch unavailable" message={error} variant="panel" />}

          {/* Pools */}
          <div className={styles.subsection}>
            <div className={styles.subsectionHeadingRow}>
              <h3 className={styles.subsectionHeading}>Pools</h3>
              <InfoTooltip
                label="About Pools"
                text="Groups patched fixture instances into named pools by required capability, so Scenes & Looks can target a pool instead of individual fixtures."
              />
            </div>
            <div className={styles.createRow}>
              <Field
                className={styles.createInput}
                label="New pool name"
                type="text"
                value={newPoolName}
                onChange={(event) => setNewPoolName(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    void handleCreatePool();
                  }
                }}
              />
              <Field
                className={styles.createInput}
                label="Required capabilities"
                type="text"
                value={newPoolRequires}
                placeholder="comma-separated, optional"
                onChange={(event) => setNewPoolRequires(event.target.value)}
              />
              <Button
                variant="primary"
                leadingIcon={Plus}
                onClick={() => void handleCreatePool()}
              >
                Create Pool
              </Button>
            </div>

            {pools.length === 0 ? (
              <EmptyState heading="No fixture pools yet" body="Create a pool, then add a fixture at a mode to patch it into a deployment." />
            ) : (
              <>
                <p className={styles.countSummary}>
                  {pools.length} pool{pools.length === 1 ? "" : "s"}
                </p>
                <ul className={styles.rowScroll} aria-label="Pool list">
                  <AnimatePresence initial={false}>
                  {pools.map((p) => (
                    <motion.li
                      key={p.id}
                      style={{ overflow: "hidden" }}
                      initial={false}
                      exit={{ opacity: 0, height: 0 }}
                      transition={rowExitTransition}
                      className={styles.row}
                    >
                      <div className={styles.rowHeader}>
                        {renamingPoolId === p.id ? (
                          <>
                            <Field
                              className={styles.createInput}
                              label="Pool name"
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
                              autoFocus
                            />
                            <IconButton
                              icon={Check}
                              label={`Save ${p.name}`}
                              onClick={() => void handleSaveRenamePool(p.name)}
                            />
                            <IconButton
                              icon={X}
                              label={`Cancel renaming ${p.name}`}
                              onClick={() => setRenamingPoolId(null)}
                            />
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
                            <Menu
                              trigger={<IconButton icon={MoreVertical} label={`${p.name} actions`} />}
                              items={[
                                {
                                  id: "rename",
                                  label: "Rename",
                                  icon: Pencil,
                                  onSelect: () => handleStartRenamePool(p.id, p.name),
                                },
                                {
                                  id: "delete",
                                  label: "Delete",
                                  icon: Trash2,
                                  destructive: true,
                                  onSelect: () => void handleDeletePool(p.name, p.members.length, instanceCountForPool(p.id)),
                                },
                              ]}
                            />
                            <Button
                              variant="secondary"
                              leadingIcon={Plus}
                              onClick={() => handleStartAddMember(p.name)}
                            >
                              Add Fixture
                            </Button>
                          </>
                        )}
                      </div>
                      {p.members.length > 0 && (
                        <ul className={styles.memberList}>
                          <AnimatePresence initial={false}>
                          {p.members.map((m) => (
                            <motion.li
                              key={m.id}
                              style={{ overflow: "hidden" }}
                              initial={false}
                              exit={{ opacity: 0, height: 0 }}
                              transition={rowExitTransition}
                              className={styles.memberRow}
                            >
                              <span className={styles.technical}>
                                {m.fixtureStableKey}
                              </span>
                              <Button
                                variant="destructive"
                                leadingIcon={X}
                                onClick={() => void handleStartRemoveMember(p.name, m.id)}
                              >
                                Remove
                              </Button>
                              {removeTarget?.poolName === p.name && removeTarget.memberId === m.id && (
                                removePreviewLoading ? (
                                  <LoadingState label="Reviewing removal impact" variant="inline" />
                                ) : (
                                  pendingRemovePreview &&
                                  pendingRemovePreview.poolName === p.name &&
                                  pendingRemovePreview.memberId === m.id && (
                                    <ImpactReview
                                      summary={`Impact Preview (plan ${pendingRemovePreview.plan.plan_id.slice(0, 12)})`}
                                      impacts={removeMemberImpacts(pendingRemovePreview.plan)}
                                    >
                                      <FormActions>
                                        <Button variant="primary" leadingIcon={Check} loading={removeApplyLoading} onClick={() => void handleApplyRemoveMember()}>
                                          {removeApplyLoading ? "Applying…" : "Apply"}
                                        </Button>
                                        <Button variant="secondary" leadingIcon={X} onClick={handleCancelRemoveMember}>Cancel</Button>
                                      </FormActions>
                                    </ImpactReview>
                                  )
                                )
                              )}
                            </motion.li>
                          ))}
                          </AnimatePresence>
                        </ul>
                      )}

                      {addPoolTarget === p.name && (
                        <div className={styles.addMemberForm}>
                          <Combobox
                            label="Fixture"
                            options={libraryRows
                              .filter((row) => row.status === "valid")
                              .map((row) => ({ value: row.stableKey, label: `${row.manufacturer} ${row.model}` }))}
                            value={selectedFixture?.stableKey ?? ""}
                            onValueChange={handleSelectFixture}
                            placeholder={
                              libraryRows.filter((row) => row.status === "valid").length === 0
                                ? "No fixtures in library -- import one first"
                                : "Select a fixture…"
                            }
                          />
                          <Select
                            label="Fixture mode"
                            options={(selectedFixture?.modes ?? []).map((mode) => ({ value: mode, label: mode }))}
                            value={addMode}
                            onValueChange={handleSelectAddMode}
                            placeholder="Select a mode…"
                            disabled={!selectedFixture}
                          />
                          <FormActions>
                            <Button variant="primary" leadingIcon={Eye} loading={previewLoading} disabled={!selectedFixture || !addMode} onClick={() => void handlePreviewAddMember()}>
                              {previewLoading ? "Reviewing…" : "Review Impact"}
                            </Button>
                            <Button variant="secondary" leadingIcon={X} onClick={() => setAddPoolTarget(null)}>Cancel</Button>
                          </FormActions>

                          {pendingPreview && pendingPreview.poolName === p.name && (
                            <ImpactReview
                              summary={`Impact Preview (plan ${pendingPreview.plan.plan_id.slice(0, 12)})`}
                              impacts={addMemberImpacts(pendingPreview.plan)}
                            >
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
                                      className={styles.previewBlocker}
                                    >
                                      {planError.code}: {planError.message}
                                    </li>
                                  ))}
                                </ul>
                              )}
                              <FormActions>
                                <Button variant="primary" leadingIcon={Check} loading={applyLoading} disabled={(pendingPreview.plan.errors ?? []).length > 0} onClick={() => void handleApplyPreview()}>
                                  {applyLoading ? "Applying…" : "Apply"}
                                </Button>
                                <Button variant="secondary" leadingIcon={X} onClick={handleCancelPreview}>Cancel</Button>
                              </FormActions>
                            </ImpactReview>
                          )}
                        </div>
                      )}
                    </motion.li>
                  ))}
                  </AnimatePresence>
                </ul>
              </>
            )}
          </div>

          {/* Deployments */}
          <div className={styles.subsection}>
            <div className={styles.subsectionHeadingRow}>
              <h3 className={styles.subsectionHeading}>Deployments</h3>
              <InfoTooltip
                label="About Deployments"
                text="Groups pools into a deployment — the active set of patched instances actually driven on stage — and lets you activate a different one."
              />
            </div>
            <div className={styles.createRow}>
              <Field
                className={styles.createInput}
                label="New deployment name"
                type="text"
                value={newDeploymentName}
                onChange={(event) => setNewDeploymentName(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    void handleCreateDeployment();
                  }
                }}
              />
              <Button
                variant="primary"
                leadingIcon={Plus}
                onClick={() => void handleCreateDeployment()}
              >
                Create Deployment
              </Button>
            </div>

            {deployments.length === 0 ? (
              <EmptyState heading="No deployments yet" body="Create a deployment, then activate it to patch pool fixtures into concrete instances." />
            ) : (
              <>
                <p className={styles.countSummary}>
                  {deployments.length} deployment
                  {deployments.length === 1 ? "" : "s"}
                </p>
                <ul className={styles.rowScroll} aria-label="Deployment list">
                  <AnimatePresence initial={false}>
                  {deployments.map((d) => (
                    <motion.li
                      key={d.id}
                      style={{ overflow: "hidden" }}
                      initial={false}
                      exit={{ opacity: 0, height: 0 }}
                      transition={rowExitTransition}
                      className={styles.row}
                    >
                      <div className={styles.rowHeader}>
                        {renamingDeploymentId === d.id ? (
                          <>
                            <Field
                              className={styles.createInput}
                              label="Deployment name"
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
                              autoFocus
                            />
                            <IconButton
                              icon={Check}
                              label={`Save ${d.name}`}
                              onClick={() => void handleSaveRenameDeployment(d.name)}
                            />
                            <IconButton
                              icon={X}
                              label={`Cancel renaming ${d.name}`}
                              onClick={() => setRenamingDeploymentId(null)}
                            />
                          </>
                        ) : (
                          <>
                            <span className={styles.rowName} title={d.name}>
                              {d.name}
                            </span>
                            {d.active ? (
                              <Chip tone="live">Active</Chip>
                            ) : (
                              <Button
                                variant="secondary"
                                leadingIcon={Zap}
                                onClick={() => void handleActivateDeployment(d.name)}
                              >
                                Activate
                              </Button>
                            )}
                            <Menu
                              trigger={<IconButton icon={MoreVertical} label={`${d.name} actions`} />}
                              items={[
                                {
                                  id: "rename",
                                  label: "Rename",
                                  icon: Pencil,
                                  onSelect: () => handleStartRenameDeployment(d.id, d.name),
                                },
                                {
                                  id: "delete",
                                  label: "Delete",
                                  icon: Trash2,
                                  destructive: true,
                                  onSelect: () => void handleDeleteDeployment(d.name, d.instances.length),
                                },
                              ]}
                            />
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
                                    <Select
                                      label="Mode"
                                      options={[
                                        reassignMode,
                                        ...modesForMember(member?.fixtureStableKey ?? "").filter(
                                          (mode) => mode !== reassignMode,
                                        ),
                                      ].map((mode) => ({ value: mode, label: mode }))}
                                      value={reassignMode}
                                      onValueChange={setReassignMode}
                                    />
                                    <Field
                                      className={styles.createInput}
                                      label="Universe"
                                      type="number"
                                      min={1}
                                      value={reassignUniverse}
                                      onChange={(event) => setReassignUniverse(event.target.value)}
                                    />
                                    <Field
                                      className={styles.createInput}
                                      label="Address"
                                      type="number"
                                      min={1}
                                      value={reassignAddress}
                                      onChange={(event) => setReassignAddress(event.target.value)}
                                    />
                                    <Button variant="primary" leadingIcon={Check} loading={reassignLoading} onClick={() => void handleSaveReassign(d.name, instance.id)}>
                                      {reassignLoading ? "Saving…" : "Save"}
                                    </Button>
                                    <Button variant="secondary" leadingIcon={X} onClick={handleCancelReassign}>Cancel</Button>
                                  </>
                                ) : (
                                  <>
                                    <span>{instance.mode}</span>
                                    <span className={styles.technical}>
                                      Universe {instance.universe}, Address{" "}
                                      {instance.address}
                                    </span>
                                    <IconButton
                                      icon={Pencil}
                                      label="Edit instance"
                                      onClick={() =>
                                        handleStartReassign(instance.id, instance.mode, instance.universe, instance.address)
                                      }
                                    />
                                  </>
                                )}
                              </li>
                            );
                          })}
                        </ul>
                      )}
                    </motion.li>
                  ))}
                  </AnimatePresence>
                </ul>
              </>
            )}
          </div>
        </>
      )}
    </div>
    </Panel>
  );
}
