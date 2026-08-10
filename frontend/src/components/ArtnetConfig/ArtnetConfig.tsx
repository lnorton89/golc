// ArtnetConfig.tsx is the on-screen deployment-interface + Art-Net
// universe/target configuration surface closing VERIFICATION.md Gap B[0]
// for PLAY-11 (06-11-PLAN.md): a show author lists the available Windows
// network interfaces, configures a universe -> unicast target (IP,
// optional port, enabled flag), enables/disables a configured target, and
// reads live per-target/interface status -- all driving the exact same
// "artnet interface list"/"artnet configure"/"artnet target enable"/
// "artnet target disable"/"artnet status" CLI routes internal/command/
// artnet.go already implements and tests, via
// internal/wails/svc_artnetconfig.go's ArtnetConfigService (a thin
// two-hop client, never a second Art-Net output path -- T-06-33).
//
// All Go-bound calls go through this file's own wailsBridge.ts helpers
// (listArtnetInterfaces/configureArtnetTarget/enableArtnetTarget/
// disableArtnetTarget/fetchArtnetStatus) -- this component owns every
// ArtnetConfigService call in the tree.
//
// Malformed targets (bad IP, out-of-range universe/port) are rejected by
// the backend route's own artnet.ValidateTarget check before any daemon
// round trip (T-04-07); this component never re-validates client-side --
// it only ever surfaces the returned Result's own stderr diagnostic
// verbatim, so the UI can never drift from the one real validation rule.
//
// State coverage (06-UI-SPEC.md-style backstop): a loading placeholder on
// initial status/interface fetch; an explicit daemon-unreachable panel
// (UI-SPEC copy + the `offline` status tone) whenever FetchArtnetStatus
// reports Reachable=false; an empty state when no targets are configured;
// an error banner rendering a failed call's own stderr diagnostic; and a
// fixed-height scroll panel for the configured-target list (backstop:
// "scrolls within a fixed-height panel rather than growing the window").
// Layout/domain geometry only lives in ArtnetConfig.module.css -- shared
// control/state chrome (Panel, Button, IconButton, Chip, Field,
// EmptyState, ErrorState, LoadingState, InfoTooltip) comes from the
// design-system barrel (13-26 migration; mirrors FixturePatch.tsx's own
// established shape). The full list-interfaces -> configure ->
// enable/disable -> status click-through against a real golc-desktop
// build is queued as a human-check for end-of-phase UAT
// (workflow.human_verify_mode=end-of-phase) rather than an interactive
// mid-execution checkpoint.

import { useCallback, useEffect, useState } from "react";
import { Plus, Power, PowerOff, Network } from "lucide-react";

import {
  configureArtnetTarget,
  disableArtnetTarget,
  enableArtnetTarget,
  errorMessage,
  fetchArtnetStatus,
  listArtnetInterfaces,
  listPatch,
  selectInterface,
  type ArtnetInterfaceView,
  type ArtnetStatusView,
  type ArtnetTargetView,
} from "../../lib/wailsBridge";
import { Button, Chip, EmptyState, ErrorState, Field, InfoTooltip, LoadingState, Panel } from "../../design-system";
import styles from "./ArtnetConfig.module.css";

interface TargetDraft {
  ip: string;
  port: string;
  enabled: boolean;
}

const emptyDraft: TargetDraft = { ip: "", port: "", enabled: true };

/** targetKey identifies one configured target for per-row busy tracking --
 * the same universe/ip/port triple the enable/disable routes take. */
function targetKey(target: ArtnetTargetView): string {
  return `${target.universe}-${target.ip}-${target.port}`;
}

export default function ArtnetConfig() {
  const [interfaces, setInterfaces] = useState<ArtnetInterfaceView[]>([]);
  const [status, setStatus] = useState<ArtnetStatusView | null>(null);
  const [patchedUniverses, setPatchedUniverses] = useState<number[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  // Per-row busy flags, following selectingIndex's own established
  // pattern. A single shared `actionLoading` boolean froze every universe
  // row at once: clicking Add Target on universe 1 put "Configuring…" on
  // all three rows and disabled every existing target's Enable/Disable
  // button, even though `drafts` is deliberately keyed per universe so
  // each row can be filled in and submitted on its own.
  const [addingUniverse, setAddingUniverse] = useState<number | null>(null);
  const [togglingTargetKey, setTogglingTargetKey] = useState<string | null>(null);
  const [selectingIndex, setSelectingIndex] = useState<number | null>(null);

  // One independent add-target draft per patched universe (keyed by
  // universe number) so every row in the Universe Targets list can be
  // filled in and submitted on its own, rather than funneling every
  // universe through one shared form.
  const [drafts, setDrafts] = useState<Record<number, TargetDraft>>({});

  const refresh = useCallback(async (): Promise<void> => {
    try {
      const [interfaceList, statusView, patchView] = await Promise.all([
        listArtnetInterfaces(),
        fetchArtnetStatus(),
        listPatch(),
      ]);
      setInterfaces(interfaceList);
      setStatus(statusView);
      // Only the active deployment's instances are actually driving live
      // output, so the target-universe picker offers only those universes
      // rather than every universe any (possibly inactive) deployment has
      // ever used.
      const universes = [
        ...new Set(
          patchView.deployments
            .filter((deployment) => deployment.active)
            .flatMap((deployment) =>
              deployment.instances.map((instance) => instance.universe),
            ),
        ),
      ].sort((a, b) => a - b);
      setPatchedUniverses(universes);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  // Seeds a blank draft for every newly-patched universe and drops drafts
  // for universes that no longer have fixtures patched, without clobbering
  // in-progress edits on rows that are still patched.
  useEffect(() => {
    setDrafts((prev) => {
      const next: Record<number, TargetDraft> = {};
      for (const u of patchedUniverses) {
        next[u] = prev[u] ?? emptyDraft;
      }
      return next;
    });
  }, [patchedUniverses]);

  // handleSelectInterface pins Art-Net output to a different network
  // interface (ARTN-01): the pinned interface is fixed for the daemon's
  // whole lifetime, so "choosing" one restarts the supervised daemon bound
  // to it (App.SelectInterface) rather than sending a live reconfigure
  // command. This only restarts the daemon, never this app itself, so the
  // call always resolves -- on success, refresh() re-reads the interface
  // list/status so the newly pinned interface renders immediately; a
  // failure (daemon never came up on the requested interface, or a switch
  // already in flight) surfaces exactly like any other action error.
  const handleSelectInterface = async (iface: ArtnetInterfaceView) => {
    setSelectingIndex(iface.index);
    try {
      const result = await selectInterface(iface.index, iface.name);
      if (result.exitCode !== 0) {
        throw new Error(result.stderr || "Interface switch failed");
      }
      setError(null);
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSelectingIndex(null);
    }
  };

  const updateDraft = (universeNum: number, patch: Partial<TargetDraft>) => {
    setDrafts((prev) => ({
      ...prev,
      [universeNum]: { ...(prev[universeNum] ?? emptyDraft), ...patch },
    }));
  };

  const handleAddTarget = async (universeNum: number) => {
    const draft = drafts[universeNum] ?? emptyDraft;
    const portNum = draft.port.trim() === "" ? 0 : Number(draft.port);
    const trimmedIp = draft.ip.trim();
    if (trimmedIp === "") {
      setError("An IP address is required to configure a target.");
      return;
    }
    // Client-side shape guard only (Task 3 backstop: "out-of-range input
    // rejected on screen"): rejects an obviously invalid port before a
    // round trip. The backend route's own artnet.ValidateTarget remains
    // the sole authority for the real validation rule (T-04-07) -- this
    // never replaces or duplicates that check, it only avoids a pointless
    // call for input that could not possibly be numeric. universeNum
    // itself always comes from the patched-universe row it belongs to, so
    // it needs no separate validation here.
    if (
      draft.port.trim() !== "" &&
      (!Number.isInteger(portNum) || portNum < 1 || portNum > 65535)
    ) {
      setError(
        `GOLC_ARTNET_USAGE: port ${draft.port} is not a valid integer in the 1-65535 range.`,
      );
      return;
    }
    setAddingUniverse(universeNum);
    try {
      const result = await configureArtnetTarget(
        universeNum,
        trimmedIp,
        portNum,
        draft.enabled,
      );
      if (result.exitCode !== 0) {
        throw new Error(result.stderr || "Configure failed");
      }
      updateDraft(universeNum, { ip: "", port: "" });
      setError(null);
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setAddingUniverse(null);
    }
  };

  const handleToggleTarget = async (target: ArtnetTargetView) => {
    setTogglingTargetKey(targetKey(target));
    try {
      const result = target.enabled
        ? await disableArtnetTarget(target.universe, target.ip, target.port)
        : await enableArtnetTarget(target.universe, target.ip, target.port);
      if (result.exitCode !== 0) {
        throw new Error(result.stderr || "Toggle failed");
      }
      setError(null);
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setTogglingTargetKey(null);
    }
  };

  const targets = status?.targets ?? [];
  const daemonUnreachable = status !== null && !status.reachable;

  const targetsByUniverse = new Map<number, ArtnetTargetView[]>();
  for (const target of targets) {
    const list = targetsByUniverse.get(target.universe) ?? [];
    list.push(target);
    targetsByUniverse.set(target.universe, list);
  }

  // The list iterates every universe that has a configured target as well
  // as every patched one. Rendering only patchedUniverses (which is
  // filtered to the ACTIVE deployment) dropped any target whose universe
  // left that set -- activating a different deployment, or readdressing
  // the universe's fixtures -- from the DOM entirely while the daemon kept
  // unicasting to it, with no in-app way to disable it. ArtnetConfig is
  // the frontend's only consumer of fetchArtnetStatus/enable/disable, so
  // no other surface could recover from it either.
  const patchedUniverseSet = new Set(patchedUniverses);
  const listedUniverses = [...new Set([...patchedUniverses, ...targetsByUniverse.keys()])].sort((a, b) => a - b);

  return (
    <Panel aria-label="Art-Net configuration" aria-busy={loading}>
      <div className={styles.content}>
        {loading ? (
          <LoadingState label="Art-Net configuration is loading" variant="panel" />
        ) : (
          <>
            {error ? <ErrorState heading="Art-Net configuration issue" message={error} variant="inline" /> : null}

            {daemonUnreachable ? (
              <div className={styles.offlinePanel}>
                <Chip tone="offline">Offline</Chip>
                <p className={styles.offlineText}>
                  Can&rsquo;t reach the playback engine. GOLC will try to
                  reconnect automatically — Blackout and Stop/Release-All
                  remain available.
                </p>
              </div>
            ) : null}

            {/* Interfaces */}
            <div className={styles.subsection}>
              <div className={styles.subsectionHeadingRow}>
                <h3 className={styles.subsectionHeading}>Network Interfaces</h3>
                <InfoTooltip
                  label="About Network Interfaces"
                  text="Lists the network interfaces the Art-Net output can bind to, and lets you pin which one it uses."
                />
              </div>
              {interfaces.length === 0 ? (
                <EmptyState icon={Network} heading="No network interfaces found" />
              ) : (
                <ul className={styles.interfaceList} aria-label="Interface list">
                  {interfaces.map((iface) => (
                    <li key={iface.index} className={styles.interfaceRow}>
                      <span
                        className={
                          iface.up
                            ? styles.interfaceStatusUp
                            : styles.interfaceStatusDown
                        }
                        aria-hidden="true"
                        title={iface.up ? "up" : "down"}
                      />
                      <span
                        className={styles.interfaceName}
                        title={iface.name}
                      >
                        {iface.name}
                      </span>
                      <span
                        className={styles.interfaceAddrs}
                        title={iface.addrs.join(", ") || "no addresses"}
                      >
                        {iface.addrs.join(", ") || "no addresses"}
                      </span>
                      {iface.pinned ? (
                        <Chip tone="live">In use</Chip>
                      ) : (
                        <Button
                          variant="secondary"
                          size="compact"
                          leadingIcon={Network}
                          disabled={selectingIndex !== null}
                          loading={selectingIndex === iface.index}
                          onClick={() => void handleSelectInterface(iface)}
                        >
                          {selectingIndex === iface.index ? "Switching…" : "Use"}
                        </Button>
                      )}
                    </li>
                  ))}
                </ul>
              )}
            </div>

            {/* Universe targets: one row per universe that currently has
                fixtures patched into the active deployment (refresh()) --
                blank when none do -- each independently editable so every
                universe can be configured without stepping through a shared
                form one at a time. */}
            <div className={styles.subsection}>
              <div className={styles.subsectionHeadingRow}>
                <h3 className={styles.subsectionHeading}>Universe Targets</h3>
                <InfoTooltip
                  label="About Universe Targets"
                  text="Lists the DMX universes currently configured to send over Art-Net and their live output state."
                />
              </div>

              {patchedUniverses.length > 0 ? (
                <p className={styles.countSummary}>
                  {patchedUniverses.length} universe
                  {patchedUniverses.length === 1 ? "" : "s"} patched
                </p>
              ) : null}

              <ul className={styles.rowScroll} aria-label="Universe target list">
                {listedUniverses.map((u) => {
                  const existing = targetsByUniverse.get(u) ?? [];
                  const draft = drafts[u] ?? emptyDraft;
                  const orphaned = !patchedUniverseSet.has(u);
                  return (
                    <li key={u} className={styles.row}>
                      <div className={styles.rowHeader}>
                        <span className={styles.rowName}>Universe {u}</span>
                        {orphaned ? <Chip tone="armed">Not in the active deployment</Chip> : null}
                      </div>

                      {existing.map((target) => (
                        <div
                          key={`${target.ip}-${target.port}`}
                          className={styles.rowHeader}
                        >
                          <span className={styles.technical}>
                            {target.ip}:{target.port || 6454}
                          </span>
                          <Chip tone={target.enabled ? "frame-lock" : "offline"}>
                            {target.enabled ? "Enabled" : "Disabled"}
                          </Chip>
                          <Button
                            variant="secondary"
                            size="compact"
                            leadingIcon={target.enabled ? PowerOff : Power}
                            disabled={togglingTargetKey === targetKey(target)}
                            onClick={() => void handleToggleTarget(target)}
                          >
                            {target.enabled ? "Disable" : "Enable"}
                          </Button>
                          <span className={styles.technical}>
                            send_ok={target.sendOk} send_err={target.sendErr}{" "}
                            reachable={String(target.reachable)}
                            {target.lastError
                              ? ` last_error=${target.lastError}`
                              : ""}
                          </span>
                        </div>
                      ))}

                      {orphaned ? (
                        <p className={styles.offlineText}>
                          No fixtures in the active deployment use this universe, but the daemon is still
                          unicasting to the targets above. Disable them here, or re-patch the universe.
                        </p>
                      ) : (
                      <div className={styles.createRow}>
                        <Field
                          className={styles.createInput}
                          label={`Universe ${u} target IP address`}
                          type="text"
                          value={draft.ip}
                          placeholder="Target IP address"
                          onChange={(event) =>
                            updateDraft(u, { ip: event.target.value })
                          }
                        />
                        <Field
                          className={styles.createInputNarrow}
                          label={`Universe ${u} target port (optional)`}
                          type="number"
                          min={1}
                          max={65535}
                          value={draft.port}
                          placeholder="Port (optional)"
                          onChange={(event) =>
                            updateDraft(u, { port: event.target.value })
                          }
                        />
                        <label className={styles.checkboxLabel}>
                          <input
                            type="checkbox"
                            checked={draft.enabled}
                            onChange={(event) =>
                              updateDraft(u, { enabled: event.target.checked })
                            }
                          />
                          Enabled
                        </label>
                        <Button
                          variant="primary"
                          leadingIcon={Plus}
                          loading={addingUniverse === u}
                          onClick={() => void handleAddTarget(u)}
                        >
                          {addingUniverse === u ? "Configuring…" : "Add Target"}
                        </Button>
                      </div>
                      )}
                    </li>
                  );
                })}
              </ul>
            </div>
          </>
        )}
      </div>
    </Panel>
  );
}
