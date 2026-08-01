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
import { useCallback, useEffect, useMemo, useState, type CSSProperties } from "react";
import { RotateCcw, Sun, TriangleAlert, Zap } from "lucide-react";
import type { LucideIcon } from "lucide-react";

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
import { useResizablePanel } from "../../hooks/useResizablePanel";
import ResizeHandle from "../primitives/ResizeHandle/ResizeHandle";
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

/** capabilityIcons swaps a text label for a compact glyph on the two
 * capability types whose full word ("Intensity", "Strobe") is the widest
 * offender against Desk's per-fader column width -- DeskKey below renders
 * the icon -> label legend so an icon is never shown unexplained,
 * mirroring shortColorLabels' identical "short label, list of what it
 * means" pairing for the RGBW letters. */
const capabilityIcons: Record<string, LucideIcon> = {
  intensity: Sun,
  strobe: Zap,
};

/** colorSwatches renders the four RGBW color-mixing capability types as an
 * actual colored dot rather than a letter -- "the red channel" reads more
 * directly from a red circle than from the letter "R" -- self-explanatory
 * without needing a DeskKey entry, unlike capabilityIcons' abstract glyphs.
 * Plain saturated red/green/blue (not the brand's --accent blue, which
 * already carries an unrelated interactive-color meaning elsewhere in this
 * app) so each swatch reads unambiguously as its literal color; white gets
 * a visible border since a near-white fill would otherwise vanish against
 * the light theme's own page background. */
const colorSwatches: Record<string, string> = {
  color_red: "#dc2626",
  color_green: "#16a34a",
  color_blue: "#2563eb",
  color_white: "#f5f4f0",
};

interface DeskChannel {
  key: string;
  capabilityType: string;
  label: string;
  icon?: LucideIcon;
  swatch?: string;
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
          icon: capabilityIcons[channel.type],
          swatch: colorSwatches[channel.type],
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

/** universeAddressRange returns the lowest and highest DMX address any
 * instance in universeInstances actually occupies -- the lowest instance
 * start address, and the highest resolved channel address (instance.
 * address + its last channel's index) across every instance, falling back
 * to an instance's own address when it has no resolved channel layout
 * (resolveChannels' own "no channel layout available" edge). Returns null
 * for an empty universe (never reached today, since groupByUniverse only
 * ever creates a universe entry from at least one instance, but this keeps
 * the helper defensively total). */
function universeAddressRange(universeInstances: DeskInstance[]): [number, number] | null {
  if (universeInstances.length === 0) return null;
  let min = Infinity;
  let max = -Infinity;
  for (const instance of universeInstances) {
    min = Math.min(min, instance.address);
    const highest =
      instance.channels.length > 0
        ? instance.channels[instance.channels.length - 1].address
        : instance.address;
    max = Math.max(max, highest);
  }
  return [min, max];
}

function liveByteAt(universeValues: DeskUniverseValuesView[], universe: number, address: number): number {
  const row = universeValues.find((candidate) => candidate.universe === universe);
  if (!row) return 0;
  return row.values[address - 1] ?? 0;
}

type MetaBadgeKind = "mode" | "address";

const META_BADGE_KIND_CLASS: Record<MetaBadgeKind, string> = {
  mode: styles.badgeMode,
  address: styles.badgeAddress,
};

/** MetaBadge mirrors ProjectFixtures.tsx's own MetaBadge pill exactly (same
 * badge/badgeLabel/badgeValue classes and per-kind tint convention,
 * duplicated locally here rather than imported since ProjectFixtures.tsx
 * has no exported shared primitive to import and this component owns its
 * own display logic, mirroring ArtnetConfig/FixturePatch's established
 * "small helper duplicated per feature" precedent) -- a fixed uppercase
 * label plus a monospace value, so a bare number (a mode's channel count,
 * a DMX start address) is never shown unexplained. */
function MetaBadge({ label, value, kind }: { label: string; value: string; kind: MetaBadgeKind }) {
  return (
    <span className={`${styles.badge} ${META_BADGE_KIND_CLASS[kind]}`}>
      <span className={styles.badgeLabel}>{label}</span>
      <span className={styles.badgeValue}>{value}</span>
    </span>
  );
}

/** UniverseRow renders one universe's fixture-group cards and owns that
 * universe's own vertically-resizable height (useResizablePanel, keyed per
 * universe number so Universe 1 and Universe 2 each remember their own
 * dragged height independently). Pulled out as its own component -- not
 * inlined in Desk's own universes.entries().map -- because a hook can't be
 * called a variable number of times inside a single component's render
 * (the universe count can change between renders); a subcomponent
 * instantiated once per map entry is the only way each row gets its own
 * independent hook instance. */
function UniverseRow({
  universe,
  universeInstances,
  range,
  overrides,
  universeValues,
  onFaderChange,
  onFaderClear,
}: {
  universe: number;
  universeInstances: DeskInstance[];
  range: [number, number] | null;
  overrides: Record<string, number>;
  universeValues: DeskUniverseValuesView[];
  onFaderChange: (channel: DeskChannel, instanceId: string, value: number) => void;
  onFaderClear: (channel: DeskChannel, instanceId: string) => void;
}) {
  const heightPanel = useResizablePanel({
    min: 140,
    max: 640,
    defaultSize: 240,
    storageKey: `golc.deskUniverseHeight.${universe}`,
    edge: "end",
    axis: "vertical",
  });

  return (
    <div className={styles.universeRow} style={{ "--universe-height": `${heightPanel.size}px` } as CSSProperties}>
      <div className={styles.universeHeadingRow}>
        <h3 className={styles.universeHeading}>Universe {universe}</h3>
        <span className={styles.universeMeta}>
          {universeInstances.length} fixture{universeInstances.length === 1 ? "" : "s"}
          {range ? ` · Ch ${range[0]}–${range[1]}` : ""}
        </span>
      </div>
      <div className={styles.fixtureScroll}>
        {universeInstances.map((instance) => (
          <div key={instance.id} className={styles.fixtureGroup}>
            <div className={styles.fixtureHeader}>
              <span className={styles.fixtureName} title={instance.displayName}>
                {instance.displayName}
              </span>
              <span className={styles.badgeRow}>
                <MetaBadge label="Mode" kind="mode" value={instance.mode} />
                <MetaBadge label="Address" kind="address" value={String(instance.address)} />
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
                      swatch={channel.swatch}
                      icon={channel.icon}
                      sublabel={`Ch ${channel.address}`}
                      value={value}
                      overridden={overridden}
                      onChange={(next) => onFaderChange(channel, instance.id, next)}
                      onClear={() => onFaderClear(channel, instance.id)}
                    />
                  );
                })}
              </div>
            )}
          </div>
        ))}
      </div>
      <ResizeHandle
        axis="vertical"
        edge="end"
        label={`Resize Universe ${universe} panel`}
        isResizing={heightPanel.isResizing}
        onPointerDown={heightPanel.handlePointerDown}
        onDoubleClick={heightPanel.resetSize}
      />
    </div>
  );
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

  // presentIconTypes drives DeskKey's legend: only the icon-labeled
  // capability types actually in view render an entry, so the key never
  // explains a glyph nothing on screen currently uses.
  const presentIconTypes = useMemo(() => {
    const types = new Set<string>();
    for (const instance of instances) {
      for (const channel of instance.channels) {
        if (channel.icon) types.add(channel.capabilityType);
      }
    }
    return types;
  }, [instances]);

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

          {presentIconTypes.size > 0 && (
            <div className={styles.keyRow} aria-label="Fader icon key">
              {[...presentIconTypes].map((capabilityType) => {
                const Icon = capabilityIcons[capabilityType];
                return (
                  <span key={capabilityType} className={styles.keyEntry}>
                    <Icon size={12} aria-hidden="true" />
                    {capabilityLabel(capabilityType)}
                  </span>
                );
              })}
            </div>
          )}

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
                <UniverseRow
                  key={universe}
                  universe={universe}
                  universeInstances={universeInstances}
                  range={universeAddressRange(universeInstances)}
                  overrides={overrides}
                  universeValues={universeValues}
                  onFaderChange={handleFaderChange}
                  onFaderClear={handleFaderClear}
                />
              ))}
            </div>
          )}
        </>
      )}
    </section>
  );
}
