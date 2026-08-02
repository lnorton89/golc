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
import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import {
  ChevronsDownUp,
  ChevronsLeftRight,
  ChevronsRightLeft,
  ChevronsUpDown,
  Minus,
  MoveHorizontal,
  RotateCcw,
  Sun,
  TriangleAlert,
  Zap,
} from "lucide-react";
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
  return capabilityWord(capabilityType);
}

function capabilityWord(capabilityType: string): string {
  return capabilityType
    .split("_")
    .filter(Boolean)
    .map((word) => word[0].toUpperCase() + word.slice(1))
    .join(" ");
}

/** fullColorLabels is shortColorLabels' un-abbreviated counterpart ("Red"
 * rather than "R") -- used only by capabilityDetailLabel below, once a
 * fader column has room to show it. Plain "Red"/"Green"/"Blue"/"White"
 * rather than capabilityWord's own "Color Red" (which the swatch dot next
 * to it already makes redundant -- the dot IS the "this is a color
 * channel" cue). */
const fullColorLabels: Record<string, string> = {
  color_red: "Red",
  color_green: "Green",
  color_blue: "Blue",
  color_white: "White",
};

/** capabilityDetailLabel is the un-abbreviated name shown alongside a
 * fader's swatch/icon once its column is wide enough (Desk's `detailed`
 * threshold, computed from the row's own current --fader-width rather than
 * tied to any specific preset) -- "Red" instead of just a red dot,
 * "Intensity" instead of just a sun glyph. Capability types that already
 * render as plain text (no swatch/icon) have nothing to add here, so this
 * is only ever consulted for the swatch/icon branch. */
function capabilityDetailLabel(capabilityType: string): string {
  return fullColorLabels[capabilityType] ?? capabilityWord(capabilityType);
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
  /** detailLabel is capabilityDetailLabel's un-abbreviated name -- only
   * ever shown by Fader once its column is wide enough to fit it. */
  detailLabel: string;
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
        channels: channels.map((channel) => {
          const occurrenceSuffix = channel.occurrence > 0 ? ` #${channel.occurrence + 1}` : "";
          return {
            key: `${instance.id}::${channel.type}`,
            capabilityType: channel.type,
            label: capabilityLabel(channel.type) + occurrenceSuffix,
            detailLabel: capabilityDetailLabel(channel.type) + occurrenceSuffix,
            icon: capabilityIcons[channel.type],
            swatch: colorSwatches[channel.type],
            occurrence: channel.occurrence,
            address: instance.address + channel.index,
          };
        }),
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

/** UNIVERSE_HEIGHT_PRESETS are the three quick-set heights the header's
 * icon-only button group offers (Compact/Normal/Large -- label is the
 * button's aria-label/title only, never rendered as visible text). Normal
 * matches this row's pre-resizable-feature natural height almost exactly
 * (a 120px fader track plus its own fixed chrome, see .faderInput's own
 * doc comment in Desk.module.css), so a fresh Desk looks unchanged from
 * before this feature existed until a user actually reaches for a preset
 * or the drag handle. */
const UNIVERSE_HEIGHT_PRESETS: { label: string; value: number; icon: LucideIcon }[] = [
  { label: "Compact", value: 210, icon: ChevronsDownUp },
  { label: "Normal", value: 260, icon: Minus },
  { label: "Large", value: 340, icon: ChevronsUpDown },
];

/** HeightPreset is a Compact/Normal/Large button click passed down from
 * Desk to every UniverseRow -- version increments on every click (even a
 * repeat of the same value) so each row's own apply-effect (below) always
 * fires exactly once per click, distinct from a plain controlled value
 * that could get re-applied on unrelated re-renders. */
interface HeightPreset {
  value: number;
  version: number;
}

/** FIXTURE_WIDTH_PRESETS mirrors UNIVERSE_HEIGHT_PRESETS exactly, but for
 * the horizontal dimension: each value is a per-channel fader COLUMN width
 * in px (--fader-width), not a fixture-card width directly -- a card's
 * total width is just its channel count times this value (plus fixed
 * chrome), so scaling the one shared column width scales every fixture's
 * card proportionally to how many channels it actually has. Normal (34)
 * matches .fader's pre-resizable-feature natural width exactly, same
 * "unchanged until touched" intent as UNIVERSE_HEIGHT_PRESETS' own Normal. */
const FIXTURE_WIDTH_PRESETS: { label: string; value: number; icon: LucideIcon }[] = [
  { label: "Compact", value: 26, icon: ChevronsRightLeft },
  { label: "Normal", value: 34, icon: Minus },
  { label: "Large", value: 60, icon: ChevronsLeftRight },
];

const FADER_WIDTH_MIN = 18;
const FADER_WIDTH_MAX = 96;

/** FADER_ROW_GAP_PX matches .faderRow's own CSS gap (var(--space-xs),
 * Desk.module.css) -- read back here so cardFaderWidth below can account
 * for it. Without this, a card's approximated width omitted the (channel
 * count - 1) gaps faderRow's own layout actually spends, undercounting a
 * 5-channel card by 16px -- small enough to go unnoticed on the sliders
 * themselves (which just gained a few px of harmless slack), but visibly
 * lopsided on .fixtureHeader above them, whose own max-width is capped to
 * this same figure: its content rendered flush left against the card's
 * padding while sitting a whole gap-allowance short of the card's right
 * edge, reading as "the card's left/right padding doesn't match" even
 * though the padding itself was always symmetric. */
const FADER_ROW_GAP_PX = 4;

/** DETAILED_MIN_FADER_WIDTH is the --fader-width threshold (in px) above
 * which a fader column shows its extra "detailed" content (the full
 * capability name next to a swatch/icon, plus the 0/64/128/192/255 value
 * scale+ticks) instead of just the compact swatch/icon/track -- computed
 * live against whatever the column's current width actually is (a manual
 * drag, a Large click, or a Fit click that happened to land above this
 * line all trigger it identically), never tied to a specific preset's
 * name. Set to exactly FIXTURE_WIDTH_PRESETS' own Large (60) -- Compact/
 * Normal stay compact, Large and any wider Fit result add the detail.
 * Kept SCALE_RESERVED_WIDTH (below) or more below Large's own value, so
 * the track never gets squeezed thinner than FADER_WIDTH_MIN once detailed
 * kicks in. */
const DETAILED_MIN_FADER_WIDTH = 60;

/** SCALE_RESERVED_WIDTH is how many extra px of a detailed fader column's
 * own --fader-width go to the value-scale column (its ticks/gap) rather
 * than the slider track itself -- baked into --fader-track-width below so
 * the track shrinks to make room for the scale instead of the scale
 * overflowing the column. Not pixel-exact against .faderScale's own actual
 * rendered width (that would need a DOM measurement pass, like Fit's own),
 * just a close-enough reservation for its "255"-width 3-digit number, its
 * own tick LINE beside it, and the gaps around both. */
const SCALE_RESERVED_WIDTH = 28;

/** WidthPreset is FIXTURE_WIDTH_PRESETS' click-broadcast counterpart to
 * HeightPreset, plus one more mode HeightPreset has no equivalent of: a
 * "fit" click carries no fixed value at all, since each row computes (via
 * computeFitFaderWidth below) its OWN column width against its OWN current
 * fixtures/channel count/container width -- unlike a fixed preset, the
 * same click can legitimately resolve to a different value per row. */
type WidthPreset = { mode: "fixed"; value: number; version: number } | { mode: "fit"; version: number };

/** computeFitFaderWidth measures a universe row's own fixtureScroll element
 * (its current children, at their current --fader-width) and solves for the
 * exact fader column width that makes the row's total content width equal
 * its container's own visible width -- i.e. exactly wide enough to fill the
 * available space with no horizontal scrollbar, regardless of how many
 * fixtures that row has or how many channels each one carries. Reads real
 * rendered pixel values (padding/border/gaps) off the DOM rather than
 * assuming Desk.module.css's token values, so it stays correct if that
 * spacing ever changes. Returns null when the row has no fader columns to
 * scale against (nothing patched, or every instance in it has no resolved
 * channel layout), since there is then nothing a width preset could do. */
function computeFitFaderWidth(fixtureScroll: HTMLElement): number | null {
  const groups = Array.from(fixtureScroll.querySelectorAll<HTMLElement>(`.${styles.fixtureGroup}`));
  if (groups.length === 0) return null;

  const rowGapPx = parseFloat(getComputedStyle(fixtureScroll).columnGap || "0") || 0;
  let currentFaderWidthPx = 0;
  let totalChannels = 0;
  // Chrome starts as just the gaps BETWEEN fixture cards; each group below
  // adds its own non-fader chrome -- border/padding AND its internal
  // between-fader gaps (deliberately left IN this total rather than netted
  // out: both are fixed px that don't scale with fader width, so both need
  // to be re-added for whatever the NEW width ends up being, exactly like
  // the between-card row gap already is) -- or, for a channel-less group,
  // its entire width, since that group has no fader columns to scale.
  let totalChrome = rowGapPx * (groups.length - 1);

  for (const group of groups) {
    const faders = Array.from(group.querySelectorAll<HTMLElement>(`.${styles.fader}`));
    const groupWidth = group.getBoundingClientRect().width;
    if (faders.length === 0) {
      totalChrome += groupWidth;
      continue;
    }
    if (currentFaderWidthPx === 0) {
      currentFaderWidthPx = faders[0].getBoundingClientRect().width;
    }
    totalChannels += faders.length;
    totalChrome += groupWidth - faders.length * currentFaderWidthPx;
  }

  if (totalChannels === 0) return null;

  // 1px safety margin: without it, float rounding on the measured chrome
  // can land the solved width a hair too wide and leave a 1px scrollbar --
  // erring narrow is invisible, erring wide defeats the whole point of Fit.
  const availableWidth = fixtureScroll.clientWidth - totalChrome - 1;
  const rawWidth = availableWidth / totalChannels;
  return Math.max(FADER_WIDTH_MIN, Math.min(FADER_WIDTH_MAX, Math.floor(rawWidth)));
}

/** UniverseRow renders one universe's fixture-group cards at its OWN
 * independently resizable height (own useResizablePanel, keyed per
 * universe number) -- dragging this row's handle only ever resizes this
 * row. `preset`, when it changes, is the one exception: Desk's Compact/
 * Normal/Large buttons apply the same height to every row at once by
 * bumping `preset.version`, after which each row goes back to being
 * independently draggable again until the next preset click. */
function UniverseRow({
  universe,
  universeInstances,
  range,
  overrides,
  touchedKeys,
  universeValues,
  onFaderChange,
  onFaderClear,
  preset,
  widthPreset,
}: {
  universe: number;
  universeInstances: DeskInstance[];
  range: [number, number] | null;
  overrides: Record<string, number>;
  touchedKeys: Set<string>;
  universeValues: DeskUniverseValuesView[];
  onFaderChange: (channel: DeskChannel, instanceId: string, value: number) => void;
  onFaderClear: (channel: DeskChannel, instanceId: string) => void;
  preset: HeightPreset | null;
  widthPreset: WidthPreset | null;
}) {
  const heightPanel = useResizablePanel({
    min: 190,
    max: 500,
    defaultSize: UNIVERSE_HEIGHT_PRESETS[1].value,
    storageKey: `golc.deskUniverseHeight.${universe}`,
    edge: "end",
    axis: "vertical",
  });
  const { setSize } = heightPanel;

  const widthPanel = useResizablePanel({
    min: FADER_WIDTH_MIN,
    max: FADER_WIDTH_MAX,
    defaultSize: FIXTURE_WIDTH_PRESETS[1].value,
    storageKey: `golc.deskFixtureWidth.${universe}`,
    edge: "end",
    axis: "horizontal",
  });
  const { setSize: setWidthSize, isResizing: isWidthResizing, handlePointerDown: handleWidthPointerDown, resetSize: resetWidthSize } = widthPanel;
  const fixtureScrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (preset) setSize(preset.value);
    // Deliberately keyed on preset.version, not preset.value: a second
    // click of the SAME preset (e.g. re-asserting Normal after this row
    // was dragged away from it) must still re-apply, which a value-only
    // dependency would miss since the value wouldn't have changed.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [preset?.version]);

  useEffect(() => {
    if (!widthPreset) return;
    if (widthPreset.mode === "fixed") {
      setWidthSize(widthPreset.value);
      return;
    }
    // "fit" mode: unlike a fixed preset, there is no shared value to apply
    // -- each row measures its OWN fixtureScroll and solves for the column
    // width that fills ITS OWN current width, so two universes with
    // different fixture/channel counts land on two different fader widths
    // from the very same Fit click.
    const container = fixtureScrollRef.current;
    if (!container) return;
    const fit = computeFitFaderWidth(container);
    if (fit !== null) setWidthSize(fit);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [widthPreset?.version]);

  // detailed drives both a fader's extra label text and its value scale
  // (see DETAILED_MIN_FADER_WIDTH's own doc comment) -- derived from the
  // row's own live width.size on every render, so it reacts identically to
  // a manual drag, a Compact/Normal/Large click, or a Fit result, with no
  // separate state of its own to fall out of sync.
  const detailed = widthPanel.size >= DETAILED_MIN_FADER_WIDTH;
  const trackWidth = widthPanel.size - 10 - (detailed ? SCALE_RESERVED_WIDTH : 0);

  return (
    <div
      className={styles.universeRow}
      style={
        {
          "--universe-height": `${heightPanel.size}px`,
          "--fader-width": `${widthPanel.size}px`,
          "--fader-track-width": `${trackWidth}px`,
        } as CSSProperties
      }
    >
      <div className={styles.universeHeadingRow}>
        <h3 className={styles.universeHeading}>Universe {universe}</h3>
        <span className={styles.universeMeta}>
          {universeInstances.length} fixture{universeInstances.length === 1 ? "" : "s"}
          {range ? ` · Ch ${range[0]}–${range[1]}` : ""}
        </span>
      </div>
      <div className={styles.fixtureScroll} ref={fixtureScrollRef}>
        {universeInstances.map((instance) => {
          // cardFaderWidth is this card's own fader-row width: channel
          // count * the row's current fader width, plus the (channel
          // count - 1) gaps between them (FADER_ROW_GAP_PX -- its own doc
          // comment covers why this needs to be exact, not approximate).
          // Passed down as --card-fader-width so .fixtureHeader can cap
          // itself to it below. Without this cap, a name+badges combo
          // that's wider than the card's own sliders would silently
          // stretch the whole card wider to fit them (flex's own auto-
          // sizing considers every child, header included) instead of
          // wrapping the badges onto their own line, defeating Compact/
          // Normal/Fit's whole point of keeping cards no wider than their
          // channel count needs. null (no channels resolved) leaves the
          // CSS var unset, falling back to .fixtureHeader's own default
          // cap.
          const cardFaderWidth =
            instance.channels.length > 0
              ? instance.channels.length * widthPanel.size + (instance.channels.length - 1) * FADER_ROW_GAP_PX
              : null;
          return (
            <div
              key={instance.id}
              className={styles.fixtureGroup}
              style={cardFaderWidth !== null ? ({ "--card-fader-width": `${cardFaderWidth}px` } as CSSProperties) : undefined}
            >
              <div className={styles.fixtureHeader}>
                <span className={styles.fixtureName} title={instance.displayName}>
                  {instance.displayName}
                </span>
                <span className={styles.badgeRow}>
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
                        detailLabel={channel.detailLabel}
                        detailed={detailed}
                        swatch={channel.swatch}
                        icon={channel.icon}
                        sublabel={`Ch ${channel.address}`}
                        value={value}
                        overridden={overridden}
                        touched={touchedKeys.has(channel.key)}
                        onChange={(next) => onFaderChange(channel, instance.id, next)}
                        onClear={() => onFaderClear(channel, instance.id)}
                      />
                    );
                  })}
                </div>
              )}
            </div>
          );
        })}
      </div>
      <ResizeHandle
        axis="vertical"
        edge="end"
        label={`Resize Universe ${universe} panel`}
        isResizing={heightPanel.isResizing}
        onPointerDown={heightPanel.handlePointerDown}
        onDoubleClick={heightPanel.resetSize}
      />
      <ResizeHandle
        axis="horizontal"
        edge="end"
        label={`Resize Universe ${universe} fixture width`}
        isResizing={isWidthResizing}
        onPointerDown={handleWidthPointerDown}
        onDoubleClick={resetWidthSize}
      />
    </div>
  );
}

export default function Desk() {
  const [patch, setPatch] = useState<PatchView | null>(null);
  const [library, setLibrary] = useState<FixtureLibraryRowView[]>([]);
  const [universeValues, setUniverseValues] = useState<DeskUniverseValuesView[]>([]);
  const [overrides, setOverrides] = useState<Record<string, number>>({});
  // touchedKeys is every channel currently grabbed-or-overridden -- gains
  // a key the moment a fader is dragged (handleFaderChange), loses it the
  // moment its override is released (handleFaderClear resets it back to
  // "untouched" grey, the same as a channel nobody has ever grabbed).
  // Drives Fader's own dimmed-until-touched thumb.
  const [touchedKeys, setTouchedKeys] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reachable, setReachable] = useState(true);
  // heightPreset is the last Compact/Normal/Large button click, threaded
  // down to every UniverseRow (see HeightPreset's own doc comment on why
  // each row applies it via a version-keyed effect rather than treating it
  // as an ongoing controlled value) -- null until the user's first click,
  // since each row otherwise owns its own independent height.
  const [heightPreset, setHeightPreset] = useState<HeightPreset | null>(null);
  // widthPreset is heightPreset's horizontal counterpart -- see WidthPreset's
  // own doc comment for why its "fit" mode carries no value the way a
  // Compact/Normal/Large click does.
  const [widthPreset, setWidthPreset] = useState<WidthPreset | null>(null);

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
    setTouchedKeys((prev) => (prev.has(channel.key) ? prev : new Set(prev).add(channel.key)));
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
    // Releasing an override resets this channel's thumb back to dimmed
    // grey (touchedKeys' own doc comment), not back to "touched" blue --
    // clearing is the explicit "I'm done with this one" signal, so it
    // should read the same as a channel nobody has touched yet.
    setTouchedKeys((prev) => {
      if (!prev.has(channel.key)) return prev;
      const next = new Set(prev);
      next.delete(channel.key);
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
            <div className={styles.headerActions}>
              {universes.size > 0 && (
                <div className={styles.heightPresetGroup} role="group" aria-label="Universe panel height">
                  {UNIVERSE_HEIGHT_PRESETS.map((preset) => {
                    const Icon = preset.icon;
                    // Reflects the last preset clicked, not a live check
                    // against every row's own current height -- once a
                    // row is independently dragged away from it, this
                    // stays lit until a different preset is chosen (rows
                    // are independent per the feedback above, so there is
                    // no single "current" height to validate against).
                    const active = heightPreset?.value === preset.value;
                    return (
                      <button
                        key={preset.label}
                        type="button"
                        title={preset.label}
                        aria-label={preset.label}
                        aria-pressed={active}
                        className={active ? `${styles.heightPresetButton} ${styles.heightPresetButtonActive}` : styles.heightPresetButton}
                        onClick={() =>
                          setHeightPreset((current) => ({ value: preset.value, version: (current?.version ?? 0) + 1 }))
                        }
                      >
                        <Icon size={14} aria-hidden="true" />
                      </button>
                    );
                  })}
                </div>
              )}
              {universes.size > 0 && (
                <div className={styles.heightPresetGroup} role="group" aria-label="Fixture panel width">
                  {FIXTURE_WIDTH_PRESETS.map((preset) => {
                    const Icon = preset.icon;
                    const active = widthPreset?.mode === "fixed" && widthPreset.value === preset.value;
                    return (
                      <button
                        key={preset.label}
                        type="button"
                        title={preset.label}
                        aria-label={preset.label}
                        aria-pressed={active}
                        className={active ? `${styles.heightPresetButton} ${styles.heightPresetButtonActive}` : styles.heightPresetButton}
                        onClick={() =>
                          setWidthPreset((current) => ({
                            mode: "fixed",
                            value: preset.value,
                            version: (current?.version ?? 0) + 1,
                          }))
                        }
                      >
                        <Icon size={14} aria-hidden="true" />
                      </button>
                    );
                  })}
                  <button
                    type="button"
                    title="Fit (fill width, no scrollbar)"
                    aria-label="Fit (fill width, no scrollbar)"
                    aria-pressed={widthPreset?.mode === "fit"}
                    className={
                      widthPreset?.mode === "fit"
                        ? `${styles.heightPresetButton} ${styles.heightPresetButtonActive}`
                        : styles.heightPresetButton
                    }
                    onClick={() =>
                      setWidthPreset((current) => ({ mode: "fit", version: (current?.version ?? 0) + 1 }))
                    }
                  >
                    <MoveHorizontal size={14} aria-hidden="true" />
                  </button>
                </div>
              )}
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
                  touchedKeys={touchedKeys}
                  universeValues={universeValues}
                  onFaderChange={handleFaderChange}
                  onFaderClear={handleFaderClear}
                  preset={heightPreset}
                  widthPreset={widthPreset}
                />
              ))}
            </div>
          )}
        </>
      )}
    </section>
  );
}
