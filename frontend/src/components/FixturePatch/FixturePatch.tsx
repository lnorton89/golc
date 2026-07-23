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
// All Go-bound calls go through window.go.wails.FixturePatchService
// (Wails v2's runtime-injected bridge for internal/wails.FixturePatchService);
// this file owns every FixturePatchService call in the component tree, and
// casts through wailsBridge.ts's shared `window.go.wails` ambient
// declaration -- never re-declares `declare global` itself (the same
// Wave-3 post-merge collision OperatorSurface.tsx/MidiPanel.tsx's own
// comments document).
//
// Universe/address are never manually entered anywhere in this component
// (06-10-PLAN.md's flagged assumption): the add-fixture control only
// accepts a fixture's stable key/content hash/mode triple (sourced from
// "fixture inspect" output, since internal/command/fixture.go exposes no
// structured fixture-list read yet); every displayed universe/address is
// the backend's own system-computed value, surfaced in the impact preview
// (proposed_universe/proposed_address) and in the deployment/instance list
// (persisted Instance.Universe/Address) -- never a second, GUI-owned
// addressing calculation.
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

import styles from "./FixturePatch.module.css";

// ---------------------------------------------------------------------------
// Types (mirror internal/wails/svc_fixturepatch.go's JSON shapes field-for-
// field; ImpactPlan/ImpactOperation mirror internal/pool/impact.go's own
// snake_case json tags exactly, since AddPoolMemberPreview/
// RemovePoolMemberPreview return that plan's raw canonical encoding
// verbatim in Result.stdout -- never re-cased through the camelCase
// convention this file's own PatchView types use)
// ---------------------------------------------------------------------------

interface PatchPoolMemberView {
  id: string;
  fixtureStableKey: string;
  fixtureContentHash: string;
}

interface PatchPoolView {
  id: string;
  name: string;
  requiredCapabilities?: string[];
  members: PatchPoolMemberView[];
}

interface PatchInstanceView {
  id: string;
  poolId: string;
  poolMemberId: string;
  mode: string;
  universe: number;
  address: number;
}

interface PatchDeploymentView {
  id: string;
  name: string;
  active: boolean;
  instances: PatchInstanceView[];
}

interface PatchView {
  pools: PatchPoolView[];
  deployments: PatchDeploymentView[];
}

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
  operations: ImpactOperation[];
  warnings?: { code: string; message: string }[];
  errors?: { code: string; message: string }[];
  plan_id: string;
}

interface GoResult {
  exitCode: number;
  stdout: string;
  stderr: string;
}

interface FixturePatchServiceBinding {
  CreatePool(name: string, requires: string[]): Promise<GoResult>;
  AddPoolMemberPreview(
    poolName: string,
    stableKey: string,
    contentHash: string,
    mode: string,
  ): Promise<GoResult>;
  RemovePoolMemberPreview(poolName: string, memberId: string): Promise<GoResult>;
  ApplyPatch(planId: string): Promise<GoResult>;
  CreateDeployment(name: string): Promise<GoResult>;
  ActivateDeployment(name: string): Promise<GoResult>;
  ListPatch(): Promise<PatchView>;
}

// The `Window.go.wails` global shape itself is declared once, centrally, in
// src/lib/wailsBridge.ts -- declaring it here too would collide with that
// file's declaration under TypeScript's declaration-merging rules (see
// OperatorSurface.tsx's identical comment). Cast through that shared shape
// locally instead.
function fixturePatchService(): FixturePatchServiceBinding | undefined {
  return window.go?.wails?.FixturePatchService as unknown as
    | FixturePatchServiceBinding
    | undefined;
}

function assertOk(result: GoResult, action: string): void {
  if (result.exitCode !== 0) {
    throw new Error(result.stderr || `${action} failed (exit ${result.exitCode})`);
  }
}

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
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

export default function FixturePatch() {
  const [patch, setPatch] = useState<PatchView | null>(null);
  const [listLoading, setListLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [newPoolName, setNewPoolName] = useState("");
  const [newPoolRequires, setNewPoolRequires] = useState("");

  const [addPoolTarget, setAddPoolTarget] = useState<string | null>(null);
  const [addStableKey, setAddStableKey] = useState("");
  const [addContentHash, setAddContentHash] = useState("");
  const [addMode, setAddMode] = useState("");
  const [previewLoading, setPreviewLoading] = useState(false);
  const [pendingPreview, setPendingPreview] = useState<PendingPreview | null>(null);
  const [applyLoading, setApplyLoading] = useState(false);

  const [newDeploymentName, setNewDeploymentName] = useState("");

  const refreshPatch = useCallback(async (): Promise<void> => {
    const svc = fixturePatchService();
    if (!svc) {
      setError(
        "GOLC_WAILS_BRIDGE_UNAVAILABLE: not running inside the GOLC desktop shell",
      );
      setListLoading(false);
      return;
    }
    try {
      const view = await svc.ListPatch();
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

  const handleCreatePool = async () => {
    const trimmed = newPoolName.trim();
    if (trimmed === "") {
      return;
    }
    const svc = fixturePatchService();
    if (!svc) {
      setError(
        "GOLC_WAILS_BRIDGE_UNAVAILABLE: not running inside the GOLC desktop shell",
      );
      return;
    }
    try {
      const result = await svc.CreatePool(trimmed, parseRequires(newPoolRequires));
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
    setAddStableKey("");
    setAddContentHash("");
    setAddMode("");
    setPendingPreview(null);
  };

  const handlePreviewAddMember = async () => {
    if (!addPoolTarget) {
      return;
    }
    const svc = fixturePatchService();
    if (!svc) {
      setError(
        "GOLC_WAILS_BRIDGE_UNAVAILABLE: not running inside the GOLC desktop shell",
      );
      return;
    }
    setPreviewLoading(true);
    try {
      const result = await svc.AddPoolMemberPreview(
        addPoolTarget,
        addStableKey.trim(),
        addContentHash.trim(),
        addMode.trim(),
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
    const svc = fixturePatchService();
    if (!svc) {
      setError(
        "GOLC_WAILS_BRIDGE_UNAVAILABLE: not running inside the GOLC desktop shell",
      );
      return;
    }
    setApplyLoading(true);
    try {
      const result = await svc.ApplyPatch(pendingPreview.plan.plan_id);
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
    const svc = fixturePatchService();
    if (!svc) {
      setError(
        "GOLC_WAILS_BRIDGE_UNAVAILABLE: not running inside the GOLC desktop shell",
      );
      return;
    }
    try {
      const result = await svc.CreateDeployment(trimmed);
      assertOk(result, "CreateDeployment");
      setNewDeploymentName("");
      await refreshPatch();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const handleActivateDeployment = async (name: string) => {
    const svc = fixturePatchService();
    if (!svc) {
      setError(
        "GOLC_WAILS_BRIDGE_UNAVAILABLE: not running inside the GOLC desktop shell",
      );
      return;
    }
    try {
      const result = await svc.ActivateDeployment(name);
      assertOk(result, "ActivateDeployment");
      await refreshPatch();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const pools = patch?.pools ?? [];
  const deployments = patch?.deployments ?? [];

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
                Create Pool
              </button>
            </div>

            {pools.length === 0 ? (
              <div className={styles.emptyState}>
                <p className={styles.emptyHeading}>No fixture pools yet</p>
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
                          onClick={() => handleStartAddMember(p.name)}
                        >
                          Add Fixture
                        </button>
                      </div>
                      {p.members.length > 0 && (
                        <ul className={styles.memberList}>
                          {p.members.map((m) => (
                            <li key={m.id} className={styles.memberRow}>
                              <span className={styles.technical}>
                                {m.fixtureStableKey}
                              </span>
                            </li>
                          ))}
                        </ul>
                      )}

                      {addPoolTarget === p.name && (
                        <div className={styles.addMemberForm}>
                          <input
                            className={styles.createInput}
                            type="text"
                            value={addStableKey}
                            placeholder="Fixture stable key (fixture inspect)"
                            onChange={(event) => setAddStableKey(event.target.value)}
                            aria-label="Fixture stable key"
                          />
                          <input
                            className={styles.createInput}
                            type="text"
                            value={addContentHash}
                            placeholder="Fixture content hash (fixture inspect)"
                            onChange={(event) =>
                              setAddContentHash(event.target.value)
                            }
                            aria-label="Fixture content hash"
                          />
                          <input
                            className={styles.createInput}
                            type="text"
                            value={addMode}
                            placeholder="Mode"
                            onChange={(event) => setAddMode(event.target.value)}
                            aria-label="Fixture mode"
                          />
                          <div className={styles.formActions}>
                            <button
                              type="button"
                              className={styles.primaryButton}
                              disabled={previewLoading}
                              onClick={() => void handlePreviewAddMember()}
                            >
                              {previewLoading ? "Reviewing…" : "Review Impact"}
                            </button>
                            <button
                              type="button"
                              className={styles.secondaryButton}
                              onClick={() => setAddPoolTarget(null)}
                            >
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
                                {pendingPreview.plan.operations
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
                                {pendingPreview.plan.operations.filter(
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
                                  {applyLoading ? "Applying…" : "Apply"}
                                </button>
                                <button
                                  type="button"
                                  className={styles.secondaryButton}
                                  onClick={handleCancelPreview}
                                >
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
                Create Deployment
              </button>
            </div>

            {deployments.length === 0 ? (
              <div className={styles.emptyState}>
                <p className={styles.emptyHeading}>No deployments yet</p>
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
                            Activate
                          </button>
                        )}
                      </div>
                      {d.instances.length > 0 && (
                        <ul className={styles.memberList}>
                          {d.instances.map((instance) => (
                            <li key={instance.id} className={styles.memberRow}>
                              <span>{instance.mode}</span>
                              <span className={styles.technical}>
                                Universe {instance.universe}, Address{" "}
                                {instance.address}
                              </span>
                            </li>
                          ))}
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
