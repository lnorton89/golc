// Desk.tsx is the Perform > Desk workspace's on-screen surface: a
// QLC+-style "Simple Desk" that visualizes every patched fixture's live
// DMX channels, grouped by universe then by fixture instance, and lets an
// operator drag a fader to manually override any channel independent of
// whether a scene is active (internal/artnet/desk.go's deskState/
// applyDeskOverrides, always still subject to Blackout/Stop-All/master
// scaling).
//
// Channel identity/labels come from listPatch() (instance universe/
// address/mode) joined against listLocalFixtures()'s modeChannels
// projection (fixture.Mode.Channels resolved to capability type +
// occurrence, svc_fixturelibrary.go) -- there is no separate "desk layout"
// read route; this reuses the exact same two calls FixturePatch.tsx/
// ProjectFixtures.tsx already make, joined the same way
// resolveMemberDisplayName does in ProjectFixtures.tsx.
//
// Live per-channel values come from fetchDeskUniverseValues(), polled on
// its own cadence (mirrors ArtnetConfig/LiveStatusBar's own independent-
// cadence status polls) -- already inclusive of any active desk override,
// since that projection reads the Art-Net worker's own final per-tick DMX
// buffer. A channel with an active local override always displays that
// override's own value (the frontend's own tracked truth, matching what
// was just written) rather than the polled value, so a fader never
// visually fights the poll it itself caused; releasing the override drops
// back to trusting the poll.
import { useCallback, useEffect, useMemo, useState } from "react";
import { RotateCcw, TriangleAlert } from "lucide-react";

import {
  clearAllDeskOverrides,
  clearDeskAttribute,
  errorMessage,
  fetchDeskUniverseValues,
  listLocalFixtures,
  listPatch,
  setDeskAttribute,
  type ChannelSlotView,
  type DeskUniverseValuesView,
  type FixtureLibraryRowView,
  type PatchPoolView,
  type PatchView,
} from "../../lib/wailsBridge";
import Fader from "./Fader";
import styles from "./Desk.module.css";

// pollIntervalMs mirrors artnetWatchInterval's own independent, human-
// readable-but-live cadence (internal/command/artnet.go) -- fast enough to
// feel live, slow enough not to hammer a fresh IPC dial+round-trip every
// tick (svc_safety.go's own doc comment on why status polling deliberately
// stays well below the 40Hz worker tick).
const pollIntervalMs = 500;

function fixtureDisplayName(row: FixtureLibraryRowView): string {
  const name = `${row.manufacturer} ${row.model}`.trim();
  return name !== "" ? name : row.stableKey;
}

function resolveDisplayName(
  pool: PatchPoolView | undefined,
  memberId: string,
  libraryRows: FixtureLibraryRowView[],
): string {
  if (pool?.name) return pool.name;
  const member = pool?.members.find((candidate) => candidate.id === memberId);
  if (!member) return "Unknown fixture";
  const row =
    libraryRows.find((candidate) => candidate.stableKey === member.fixtureStableKey) ??
    libraryRows.find((candidate) => candidate.contentHash === member.fixtureContentHash);
  return row ? fixtureDisplayName(row) : member.fixtureStableKey || member.fixtureContentHash;
}

function resolveChannels(
  pool: PatchPoolView | undefined,
  memberId: string,
  mode: string,
  libraryRows: FixtureLibraryRowView[],
): ChannelSlotView[] {
  const member = pool?.members.find((candidate) => candidate.id === memberId);
  if (!member) return [];
  const row =
    libraryRows.find((candidate) => candidate.stableKey === member.fixtureStableKey) ??
    libraryRows.find((candidate) => candidate.contentHash === member.fixtureContentHash);
  return row?.modeChannels[mode] ?? [];
}

/** shortColorLabels renders the four RGBW color-mixing capability types
 * (fixture.CapabilityColorRed/Green/Blue/White) as the single-letter labels
 * a lighting desk conventionally uses for them ("R"/"G"/"B"/"W") rather
 * than the verbose "Color Red" a generic word-split would produce --
 * distinct from the discrete wheel/gel "color" capability, which keeps its
 * full "Color" label since it has no such convention. */
const shortColorLabels: Record<string, string> = {
  color_red: "R",
  color_green: "G",
  color_blue: "B",
  color_white: "W",
};

/** capabilityLabel renders a fixture.CapabilityType wire value ("pan") as a
 * human label ("Pan") -- the fader's own label, since no per-channel
 * free-text label exists anywhere in the fixture model (fixture.ChannelSlot
 * only ever carries Type + Occurrence). RGBW color-mixing types render as
 * their short single-letter form instead (see shortColorLabels). */
function capabilityLabel(capabilityType: string): string {
  const short = shortColorLabels[capabilityType];
  if (short) return short;
  return capabilityType
    .split("_")
    .filter(Boolean)
    .map((word) => word[0].toUpperCase() + word.slice(1))
    .join(" ");
}

interface DeskChannel {
  key: string;
  capabilityType: string;
  label: string;
  occurrence: number;
  address: number;
}

interface DeskInstance {
  id: string;
  displayName: string;
  mode: string;
  universe: number;
  address: number;
  channels: DeskChannel[];
}

function buildInstances(patch: PatchView, libraryRows: FixtureLibraryRowView[]): DeskInstance[] {
  const instances: DeskInstance[] = [];
  for (const deployment of patch.deployments) {
    if (!deployment.active) continue;
    for (const instance of deployment.instances) {
      const pool = patch.pools.find((candidate) => candidate.id === instance.poolId);
      const channels = resolveChannels(pool, instance.poolMemberId, instance.mode, libraryRows);
      instances.push({
        id: instance.id,
        displayName: resolveDisplayName(pool, instance.poolMemberId, libraryRows),
        mode: instance.mode,
        universe: instance.universe,
        address: instance.address,
        channels: channels.map((channel) => ({
          key: `${instance.id}::${channel.type}`,
          capabilityType: channel.type,
          label: capabilityLabel(channel.type) + (channel.occurrence > 0 ? ` #${channel.occurrence + 1}` : ""),
          occurrence: channel.occurrence,
          address: instance.address + channel.index,
        })),
      });
    }
  }
  return instances;
}

function groupByUniverse(instances: DeskInstance[]): Map<number, DeskInstance[]> {
  const byUniverse = new Map<number, DeskInstance[]>();
  for (const instance of instances) {
    const existing = byUniverse.get(instance.universe) ?? [];
    existing.push(instance);
    byUniverse.set(instance.universe, existing);
  }
  for (const list of byUniverse.values()) {
    list.sort((a, b) => a.address - b.address);
  }
  return new Map([...byUniverse.entries()].sort(([a], [b]) => a - b));
}

function liveByteAt(universeValues: DeskUniverseValuesView[], universe: number, address: number): number {
  const row = universeValues.find((candidate) => candidate.universe === universe);
  if (!row) return 0;
  return row.values[address - 1] ?? 0;
}

export default function Desk() {
  const [patch, setPatch] = useState<PatchView | null>(null);
  const [library, setLibrary] = useState<FixtureLibraryRowView[]>([]);
  const [universeValues, setUniverseValues] = useState<DeskUniverseValuesView[]>([]);
  const [overrides, setOverrides] = useState<Record<string, number>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reachable, setReachable] = useState(true);

  const loadLayout = useCallback(async (): Promise<void> => {
    try {
      const [patchView, libraryView] = await Promise.all([listPatch(), listLocalFixtures()]);
      setPatch(patchView);
      setLibrary(libraryView.rows);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadLayout();
  }, [loadLayout]);

  useEffect(() => {
    let cancelled = false;
    const poll = async (): Promise<void> => {
      try {
        const values = await fetchDeskUniverseValues();
        if (!cancelled) {
          setUniverseValues(values);
          setReachable(true);
        }
      } catch {
        if (!cancelled) setReachable(false);
      }
    };
    void poll();
    const id = window.setInterval(() => void poll(), pollIntervalMs);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, []);

  const instances = useMemo(() => (patch ? buildInstances(patch, library) : []), [patch, library]);
  const universes = useMemo(() => groupByUniverse(instances), [instances]);

  const handleFaderChange = (channel: DeskChannel, instanceId: string, value: number) => {
    setOverrides((prev) => ({ ...prev, [channel.key]: value }));
    void setDeskAttribute(instanceId, channel.capabilityType, value / 255).then((result) => {
      if (result.exitCode !== 0) setError(result.stderr || "Failed to set channel");
    });
  };

  const handleFaderClear = (channel: DeskChannel, instanceId: string) => {
    setOverrides((prev) => {
      const next = { ...prev };
      delete next[channel.key];
      return next;
    });
    void clearDeskAttribute(instanceId, channel.capabilityType).then((result) => {
      if (result.exitCode !== 0) setError(result.stderr || "Failed to clear channel");
    });
  };

  const handleClearAll = () => {
    setOverrides({});
    void clearAllDeskOverrides().then((result) => {
      if (result.exitCode !== 0) setError(result.stderr || "Failed to release overrides");
    });
  };

  const overrideCount = Object.keys(overrides).length;

  return (
    <section className={styles.panel} aria-label="Desk" aria-busy={loading}>
      {loading ? (
        <div className={styles.skeleton}>Loading Desk…</div>
      ) : (
        <>
          {error && <p className={styles.errorText}>{error}</p>}

          {!reachable && (
            <div className={styles.offlinePanel}>
              <span className={styles.offlineChip}>offline</span>
              <p className={styles.offlineText}>
                <TriangleAlert size={14} aria-hidden="true" style={{ verticalAlign: "-2px", marginRight: 4 }} />
                Can&rsquo;t reach the playback engine. Live values will resume updating once it&rsquo;s reachable
                again.
              </p>
            </div>
          )}

          <div className={styles.headerRow}>
            <p className={styles.countSummary}>
              {instances.length} fixture{instances.length === 1 ? "" : "s"} across {universes.size} universe
              {universes.size === 1 ? "" : "s"}
              {overrideCount > 0 ? ` · ${overrideCount} channel${overrideCount === 1 ? "" : "s"} overridden` : ""}
            </p>
            <button
              type="button"
              className={styles.secondaryButton}
              onClick={handleClearAll}
              disabled={overrideCount === 0}
            >
              <RotateCcw size={13} aria-hidden="true" />
              Release All
            </button>
          </div>

          {universes.size === 0 ? (
            <div className={styles.emptyState}>
              <p className={styles.emptyHeading}>No patched fixtures in the active deployment</p>
              <p className={styles.emptyBody}>
                Patch fixtures into an active deployment (Build &gt; Patch &amp; Pools) to control them here.
              </p>
            </div>
          ) : (
            <div className={styles.universeList}>
              {[...universes.entries()].map(([universe, universeInstances]) => (
                <div key={universe} className={styles.universeRow}>
                  <h3 className={styles.universeHeading}>Universe {universe}</h3>
                  <div className={styles.fixtureScroll}>
                    {universeInstances.map((instance) => (
                      <div key={instance.id} className={styles.fixtureGroup}>
                        <div className={styles.fixtureHeader}>
                          <span className={styles.fixtureName} title={instance.displayName}>
                            {instance.displayName}
                          </span>
                          <span className={styles.technical}>
                            {instance.mode} · A{instance.address}
                          </span>
                        </div>
                        {instance.channels.length === 0 ? (
                          <p className={styles.noChannels}>No channel layout available</p>
                        ) : (
                          <div className={styles.faderRow}>
                            {instance.channels.map((channel) => {
                              const overridden = channel.key in overrides;
                              const value = overridden
                                ? overrides[channel.key]
                                : liveByteAt(universeValues, instance.universe, channel.address);
                              return (
                                <Fader
                                  key={channel.key}
                                  label={channel.label}
                                  sublabel={`Ch ${channel.address}`}
                                  value={value}
                                  overridden={overridden}
                                  onChange={(next) => handleFaderChange(channel, instance.id, next)}
                                  onClear={() => handleFaderClear(channel, instance.id)}
                                />
                              );
                            })}
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          )}
        </>
      )}
    </section>
  );
}
