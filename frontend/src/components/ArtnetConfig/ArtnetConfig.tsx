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
// (UI-SPEC copy + the `offline` status color) whenever FetchArtnetStatus
// reports Reachable=false; an empty state when no targets are configured;
// an error banner rendering a failed call's own stderr diagnostic; and a
// fixed-height scroll panel for the configured-target list (backstop:
// "scrolls within a fixed-height panel rather than growing the window").
// The full list-interfaces -> configure -> enable/disable -> status click-
// through against a real golc-desktop build is queued as a human-check for
// end-of-phase UAT (workflow.human_verify_mode=end-of-phase) rather than an
// interactive mid-execution checkpoint.

import { useCallback, useEffect, useState } from "react";
import { Plus, Power, PowerOff, Network, TriangleAlert } from "lucide-react";

import {
  configureArtnetTarget,
  disableArtnetTarget,
  enableArtnetTarget,
  errorMessage,
  fetchArtnetStatus,
  listArtnetInterfaces,
  selectInterface,
  type ArtnetInterfaceView,
  type ArtnetStatusView,
  type ArtnetTargetView,
} from "../../lib/wailsBridge";
import styles from "./ArtnetConfig.module.css";

export default function ArtnetConfig() {
  const [interfaces, setInterfaces] = useState<ArtnetInterfaceView[]>([]);
  const [status, setStatus] = useState<ArtnetStatusView | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState(false);
  const [selectingIndex, setSelectingIndex] = useState<number | null>(null);

  const [universe, setUniverse] = useState("1");
  const [ip, setIp] = useState("");
  const [port, setPort] = useState("");
  const [enabled, setEnabled] = useState(true);

  const refresh = useCallback(async (): Promise<void> => {
    try {
      const [interfaceList, statusView] = await Promise.all([
        listArtnetInterfaces(),
        fetchArtnetStatus(),
      ]);
      setInterfaces(interfaceList);
      setStatus(statusView);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

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

  const handleAddTarget = async () => {
    const universeNum = Number(universe);
    const portNum = port.trim() === "" ? 0 : Number(port);
    const trimmedIp = ip.trim();
    if (trimmedIp === "") {
      setError("An IP address is required to configure a target.");
      return;
    }
    // Client-side shape guard only (Task 3 backstop: "out-of-range input
    // rejected on screen"): rejects an obviously invalid universe/port
    // before a round trip. The backend route's own artnet.ValidateTarget
    // remains the sole authority for the real validation rule (T-04-07) --
    // this never replaces or duplicates that check, it only avoids a
    // pointless call for input that could not possibly be numeric.
    if (!Number.isInteger(universeNum) || universeNum < 1) {
      setError(
        `GOLC_ARTNET_USAGE: universe ${universe} is not a valid positive integer.`,
      );
      return;
    }
    if (port.trim() !== "" && (!Number.isInteger(portNum) || portNum < 1 || portNum > 65535)) {
      setError(
        `GOLC_ARTNET_USAGE: port ${port} is not a valid integer in the 1-65535 range.`,
      );
      return;
    }
    setActionLoading(true);
    try {
      const result = await configureArtnetTarget(
        universeNum,
        trimmedIp,
        portNum,
        enabled,
      );
      if (result.exitCode !== 0) {
        throw new Error(result.stderr || "Configure failed");
      }
      setIp("");
      setPort("");
      setError(null);
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setActionLoading(false);
    }
  };

  const handleToggleTarget = async (target: ArtnetTargetView) => {
    setActionLoading(true);
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
      setActionLoading(false);
    }
  };

  const targets = status?.targets ?? [];
  const daemonUnreachable = status !== null && !status.reachable;

  return (
    <section
      className={styles.panel}
      aria-label="Art-Net configuration"
      aria-busy={loading}
    >
      <h2 className={styles.sectionHeading}>Art-Net Configuration</h2>

      {loading ? (
        <div className={styles.skeleton}>Loading Art-Net configuration…</div>
      ) : (
        <>
          {error && <p className={styles.errorText}>{error}</p>}

          {daemonUnreachable && (
            <div className={styles.offlinePanel}>
              <span className={styles.offlineChip}>offline</span>
              <p className={styles.offlineText}>
                <TriangleAlert size={14} aria-hidden="true" style={{ verticalAlign: "-2px", marginRight: 4 }} />
                Can&rsquo;t reach the playback engine. GOLC will try to
                reconnect automatically — Blackout and Stop/Release-All
                remain available.
              </p>
            </div>
          )}

          {/* Interfaces */}
          <div className={styles.subsection}>
            <h3 className={styles.subsectionHeading}>Network Interfaces</h3>
            {interfaces.length === 0 ? (
              <div className={styles.emptyState}>
                <p className={styles.emptyHeading}>
                  <Network size={18} aria-hidden="true" />
                  No network interfaces found
                </p>
              </div>
            ) : (
              <ul
                className={styles.interfaceList}
                aria-label="Interface list"
              >
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
                      <span className={styles.pinnedChip}>In use</span>
                    ) : (
                      <button
                        type="button"
                        className={styles.interfaceSelectButton}
                        disabled={selectingIndex !== null}
                        onClick={() => void handleSelectInterface(iface)}
                        title={`Use ${iface.name} for Art-Net output`}
                      >
                        <Network size={12} aria-hidden="true" />
                        {selectingIndex === iface.index ? "Switching…" : "Use"}
                      </button>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </div>

          {/* Configured targets */}
          <div className={styles.subsection}>
            <h3 className={styles.subsectionHeading}>Universe Targets</h3>
            <div className={styles.createRow}>
              <input
                className={styles.createInputNarrow}
                type="number"
                min={1}
                value={universe}
                placeholder="Universe"
                onChange={(event) => setUniverse(event.target.value)}
                aria-label="Universe"
              />
              <input
                className={styles.createInput}
                type="text"
                value={ip}
                placeholder="Target IP address"
                onChange={(event) => setIp(event.target.value)}
                aria-label="Target IP address"
              />
              <input
                className={styles.createInputNarrow}
                type="number"
                min={1}
                max={65535}
                value={port}
                placeholder="Port (optional)"
                onChange={(event) => setPort(event.target.value)}
                aria-label="Target port (optional)"
              />
              <label className={styles.checkboxLabel}>
                <input
                  type="checkbox"
                  checked={enabled}
                  onChange={(event) => setEnabled(event.target.checked)}
                />
                Enabled
              </label>
              <button
                type="button"
                className={styles.primaryButton}
                disabled={actionLoading}
                onClick={() => void handleAddTarget()}
              >
                <Plus size={14} aria-hidden="true" />
                {actionLoading ? "Configuring…" : "Add Target"}
              </button>
            </div>

            {targets.length === 0 ? (
              <div className={styles.emptyState}>
                <p className={styles.emptyHeading}>
                  <Network size={18} aria-hidden="true" />
                  No Art-Net targets configured
                </p>
                <p className={styles.emptyBody}>
                  Configure a universe and unicast IP target above to start
                  sending Art-Net output.
                </p>
              </div>
            ) : (
              <>
                <p className={styles.countSummary}>
                  {targets.length} target{targets.length === 1 ? "" : "s"}
                </p>
                <ul className={styles.rowScroll} aria-label="Target list">
                  {targets.map((target) => (
                    <li
                      key={`${target.universe}-${target.ip}-${target.port}`}
                      className={styles.row}
                    >
                      <div className={styles.rowHeader}>
                        <span className={styles.rowName}>
                          Universe {target.universe}
                        </span>
                        <span className={styles.technical}>
                          {target.ip}:{target.port || 6454}
                        </span>
                        <span
                          className={
                            target.enabled
                              ? styles.enabledChip
                              : styles.disabledChip
                          }
                        >
                          {target.enabled ? "Enabled" : "Disabled"}
                        </span>
                        <button
                          type="button"
                          className={styles.secondaryButton}
                          disabled={actionLoading}
                          onClick={() => void handleToggleTarget(target)}
                        >
                          {target.enabled ? (
                            <PowerOff size={13} aria-hidden="true" />
                          ) : (
                            <Power size={13} aria-hidden="true" />
                          )}
                          {target.enabled ? "Disable" : "Enable"}
                        </button>
                      </div>
                      <span className={styles.technical}>
                        send_ok={target.sendOk} send_err={target.sendErr}{" "}
                        reachable={String(target.reachable)}
                        {target.lastError ? ` last_error=${target.lastError}` : ""}
                      </span>
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
