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
import { useCallback, useEffect, useMemo, useState } from "react";
import { Plus, Eye, Check, PackagePlus, Pencil, X } from "lucide-react";

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
import {
  Button,
  Combobox,
  Dialog,
  EmptyState,
  ErrorState,
  Field,
  FormActions,
  IconButton,
  ImpactReview,
  LoadingState,
  NumberStepper,
  Panel,
  Select,
} from "../../design-system";
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

/** addImpacts/removeImpacts summarize an ImpactPlan for ImpactReview's
 * preview-before-apply contract, mirroring FixturePatch.tsx's identical
 * helpers -- this file's ImpactPlan/ImpactOperation shapes are the same
 * raw pool-impact encoding, duplicated locally per this file's own header
 * comment rather than centralized in wailsBridge.ts. */
function addImpacts(plan: ImpactPlan): string[] {
  const additions = (plan.operations ?? [])
    .filter((op) => op.dependent_kind === "deployment_instance" && op.action === "add")
    .map((op) => `${op.dependent_ref} → Universe ${op.proposed_universe}, Address ${op.proposed_address}`);
  return additions.length > 0 ? additions : ["No deployment currently references this pool -- nothing to instantiate yet."];
}

function removeImpacts(plan: ImpactPlan): string[] {
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

// MetaTag is a small labeled pill for a bare identifier/value that would
// otherwise be ambiguous on its own (a truncated UUID, a deployment name
// that reads like a generic word) -- label is a fixed uppercase tag ("ID",
// "Deployment") so the value is never shown unexplained. It deliberately
// does not reuse Chip: Chip's tone vocabulary is the fixed six-status
// brand meaning (live/armed/revoked/...), and this row-metadata use has
// five unrelated identity kinds with no status meaning to encode. Every
// kind shares one flat tint -- the fixed label plus each kind's stable
// grid column position (mode/universe/address/deployment always land in
// the same track, per .rowScroll's subgrid) already disambiguate kind
// without needing a second, decorative per-kind color.
function MetaTag({
  label,
  value,
  title,
}: {
  label: string;
  value: string;
  title?: string;
}) {
  return (
    <span className={styles.tag} title={title}>
      <span className={styles.tagLabel}>{label}</span>
      <span className={styles.tagValue}>{value}</span>
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
    <Panel aria-label="Project fixtures" aria-busy={listLoading}>
      {listLoading ? (
        <LoadingState label="Project fixtures are loading" variant="panel" />
      ) : (
        <>
          {error && <ErrorState heading="Project fixtures unavailable" message={error} variant="panel" />}

          <div className={styles.subsection}>
            <div className={styles.createRow}>
              <span className={styles.countSummary}>
                {rows.length} fixture{rows.length === 1 ? "" : "s"} in this project
              </span>
              {!showAddForm && (
                <Button variant="primary" leadingIcon={Plus} onClick={handleOpenAddForm}>
                  Add from Library
                </Button>
              )}
            </div>

            {rows.length === 0 ? (
              <EmptyState
                icon={PackagePlus}
                heading="No fixtures added yet"
                body="Add a fixture from the library to patch it into the show with a universe and address."
              />
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
                        <div className={styles.nameField}>
                          <Field
                            label="Name"
                            value={reassignName}
                            onChange={(event) => setReassignName(event.target.value)}
                          />
                        </div>
                        <Select
                          label="Mode"
                          options={[
                            reassignMode,
                            ...modesForFixture(row.fixtureStableKey).filter(
                              (modeOption) => modeOption !== reassignMode,
                            ),
                          ].map((modeOption) => ({ value: modeOption, label: modeOption }))}
                          value={reassignMode}
                          onValueChange={setReassignMode}
                        />
                        <NumberStepper value={reassignUniverse} onChange={setReassignUniverse} label="Universe" />
                        <NumberStepper value={reassignAddress} onChange={setReassignAddress} label="Address" />
                        <IconButton
                          icon={Check}
                          variant="primary"
                          loading={reassignLoading}
                          label={reassignLoading ? "Saving…" : "Save"}
                          onClick={() => void handleSaveReassign(row)}
                        />
                        <IconButton icon={X} label="Cancel" onClick={handleCancelReassign} />
                      </>
                    ) : (
                      <>
                        <span className={styles.rowName} title={row.displayName}>
                          {row.displayName}
                        </span>
                        <MetaTag
                          label="ID"
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
                        <MetaTag label="Mode" value={row.mode} />
                        <MetaTag label="Universe" value={String(row.universe)} />
                        <MetaTag label="Address" value={String(row.address)} />
                        <MetaTag label="Deployment" value={row.deploymentName} />
                        <IconButton icon={Pencil} label={`Edit ${row.displayName}`} onClick={() => handleStartReassign(row)} />
                        <IconButton
                          icon={X}
                          variant="destructive"
                          label={`Remove ${row.displayName}`}
                          onClick={() => void handleStartRemove(row.poolName, row.poolMemberId)}
                        />
                      </>
                    )}

                    {removeTarget?.poolName === row.poolName && removeTarget.poolMemberId === row.poolMemberId && (
                      removePreviewLoading ? (
                        <LoadingState label="Reviewing removal impact" variant="inline" />
                      ) : (
                        pendingRemovePreview && (
                          <ImpactReview
                            summary={`Impact Preview (plan ${pendingRemovePreview.plan_id.slice(0, 12)})`}
                            impacts={removeImpacts(pendingRemovePreview)}
                          >
                            <FormActions>
                              <Button variant="primary" leadingIcon={Check} loading={removeApplyLoading} onClick={() => void handleApplyRemove()}>
                                {removeApplyLoading ? "Applying…" : "Apply"}
                              </Button>
                              <Button variant="secondary" leadingIcon={X} onClick={handleCancelRemove}>Cancel</Button>
                            </FormActions>
                          </ImpactReview>
                        )
                      )
                    )}
                  </li>
                ))}
              </ul>
            )}

            <Dialog
              open={showAddForm}
              title={
                <span className={styles.dialogTitle}>
                  <PackagePlus size={16} aria-hidden="true" />
                  Add from Library
                </span>
              }
              onClose={handleCancelAddForm}
            >
              <div className={styles.addForm}>
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
                  options={(selectedFixture?.modes ?? []).map((modeOption) => ({ value: modeOption, label: modeOption }))}
                  value={mode}
                  onValueChange={setMode}
                  placeholder="Select a mode…"
                  disabled={!selectedFixture}
                />
                <NumberStepper value={quantity} onChange={setQuantity} label="Quantity" />
                <NumberStepper
                  value={startUniverse}
                  onChange={setStartUniverse}
                  label="Starting universe"
                  placeholder="Auto"
                  description="Optional -- leave blank for the next open universe."
                />
                <NumberStepper
                  value={startAddress}
                  onChange={setStartAddress}
                  label="Starting address"
                  placeholder="Auto"
                  description="Optional -- leave blank for the next open address."
                />

                <FormActions>
                  <Button
                    variant="primary"
                    leadingIcon={Eye}
                    loading={previewLoading}
                    disabled={!selectedFixture || !mode}
                    onClick={() => void handleReviewImpact()}
                  >
                    {previewLoading ? "Reviewing…" : "Review Impact"}
                  </Button>
                  <Button variant="secondary" leadingIcon={X} onClick={handleCancelAddForm}>Cancel</Button>
                </FormActions>

                {pendingPreview && (
                  <ImpactReview
                    summary={`Impact Preview (plan ${pendingPreview.plan_id.slice(0, 12)})`}
                    impacts={addImpacts(pendingPreview)}
                  >
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
                          <li key={`error-${index}`} className={styles.previewBlocker}>
                            {planError.code}: {planError.message}
                          </li>
                        ))}
                      </ul>
                    )}
                    <FormActions>
                      <Button
                        variant="primary"
                        leadingIcon={Check}
                        loading={applyLoading}
                        disabled={(pendingPreview.errors ?? []).length > 0}
                        onClick={() => void handleApply()}
                      >
                        {applyLoading ? "Applying…" : "Apply"}
                      </Button>
                      <Button variant="secondary" leadingIcon={X} onClick={() => setPendingPreview(null)}>Cancel</Button>
                    </FormActions>
                  </ImpactReview>
                )}
              </div>
            </Dialog>
          </div>
        </>
      )}
    </Panel>
  );
}
