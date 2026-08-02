// deskLabels.ts is Desk.tsx's own fixture/capability label-resolution
// logic, extracted so MidiPanel.tsx's "Desk mappings" section (which has no
// other reason to duplicate patch/fixture-library lookups) can resolve the
// exact same human-readable labels for a deskmidi.Mapping's bare
// instanceId/capability pair -- there is only one label-resolution
// implementation for a Desk channel in this codebase, not one per
// consumer.
import type { FixtureLibraryRowView, PatchPoolView, PatchView } from "../../lib/wailsBridge";

export function fixtureDisplayName(row: FixtureLibraryRowView): string {
  const name = `${row.manufacturer} ${row.model}`.trim();
  return name !== "" ? name : row.stableKey;
}

export function resolveDisplayName(
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

function capabilityWord(capabilityType: string): string {
  return capabilityType
    .split("_")
    .filter(Boolean)
    .map((word) => word[0].toUpperCase() + word.slice(1))
    .join(" ");
}

/** capabilityLabel renders a fixture.CapabilityType wire value ("pan") as a
 * human label ("Pan") -- the fader's own label, since no per-channel
 * free-text label exists anywhere in the fixture model (fixture.ChannelSlot
 * only ever carries Type + Occurrence). RGBW color-mixing types render as
 * their short single-letter form instead (see shortColorLabels). */
export function capabilityLabel(capabilityType: string): string {
  const short = shortColorLabels[capabilityType];
  if (short) return short;
  return capabilityWord(capabilityType);
}

/** fullColorLabels is shortColorLabels' un-abbreviated counterpart ("Red"
 * rather than "R") -- used only by capabilityDetailLabel below, once a
 * fader column has room to show it. */
const fullColorLabels: Record<string, string> = {
  color_red: "Red",
  color_green: "Green",
  color_blue: "Blue",
  color_white: "White",
};

/** capabilityDetailLabel is the un-abbreviated name shown alongside a
 * fader's swatch/icon once its column is wide enough (Desk's `detailed`
 * threshold) -- "Red" instead of just a red dot, "Intensity" instead of
 * just a sun glyph. */
export function capabilityDetailLabel(capabilityType: string): string {
  return fullColorLabels[capabilityType] ?? capabilityWord(capabilityType);
}

/** resolveDeskChannelLabel resolves a deskmidi.Mapping's bare
 * (instanceId, capability) pair into the exact same "<fixture name> · <capability>"
 * label a Desk fader itself would show, searching every deployment (not
 * just the active one -- a mapping survives switching which deployment is
 * active, so MidiPanel's own list must still resolve a label for a mapping
 * on a currently-inactive deployment). Returns a fallback string embedding
 * the raw IDs when the instance can no longer be found (e.g. between a
 * deployment edit and the next ScrubDangling-backed Save) rather than
 * throwing -- MidiPanel's own list is a read-only projection, same
 * tolerance convention as svc_midi.go's sceneNameByID. */
export function resolveDeskChannelLabel(
  patch: PatchView | null,
  library: FixtureLibraryRowView[],
  instanceId: string,
  capability: string,
): string {
  for (const deployment of patch?.deployments ?? []) {
    const instance = deployment.instances.find((candidate) => candidate.id === instanceId);
    if (!instance) continue;
    const pool = patch?.pools.find((candidate) => candidate.id === instance.poolId);
    const fixtureName = resolveDisplayName(pool, instance.poolMemberId, library);
    return `${fixtureName} · ${capabilityLabel(capability)}`;
  }
  return `Unknown fixture · ${capabilityLabel(capability)}`;
}
