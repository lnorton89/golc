// expandedCopy.ts (Plan 13-31 Task 2, D-02/D-03/D-10/D-12/D-13/D-14,
// UI-CONSIDERATIONS-BACKSTOP-LONG-TEXT): the "long-text | Localized/expanded
// future copy" backstop -- "Visual fixtures use at least one 2x-length
// label/message to prove reflow even though full localization is outside
// this phase." An explicit fixture pairing a plausible canonical (real
// baseline-length) string against a genuinely longer expanded one for each
// of six representative copy categories (shell identity, dialog impact,
// shared error/empty/loading states, Guided First Show evidence, live
// status/log copy, and field/help validation text) -- every pair's
// expansion ratio is measured in Unicode grapheme clusters (not UTF-16 code
// units, which would silently miscount any future multi-byte-character
// content) and mechanically rejected below the plan's required 2.0x floor.
export type CopyCategory = "shell" | "dialog" | "shared-states" | "guided-first-show" | "status" | "field-help";

export interface CopyPair {
  id: string;
  category: CopyCategory;
  description: string;
  canonical: string;
  expanded: string;
}

// graphemeCount counts Unicode grapheme clusters via Intl.Segmenter --
// "2x-length" is measured the way a reader actually perceives length, not
// raw UTF-16 code units (which would over-count a single surrogate-pair
// emoji as 2, or a combining-mark sequence as several).
export function graphemeCount(value: string): number {
  const segmenter = new Intl.Segmenter(undefined, { granularity: "grapheme" });
  return Array.from(segmenter.segment(value)).length;
}

export function expansionRatio(pair: Pick<CopyPair, "canonical" | "expanded">): number {
  return graphemeCount(pair.expanded) / graphemeCount(pair.canonical);
}

export const MINIMUM_EXPANSION_RATIO = 2.0;

// artnetPortUsageMessage mirrors ArtnetConfig.tsx's handleAddTarget guard
// exactly: "GOLC_ARTNET_USAGE: port {draft.port} is not a valid integer in
// the 1-65535 range." -- computed here (not hand-copied as a literal string)
// so the field-help pair's own expansion ratio and the spec's later DOM
// assertion both derive from one shared source of truth, and a change to
// that exact copy template only ever needs updating in one place.
export function artnetPortUsageMessage(port: string): string {
  return `GOLC_ARTNET_USAGE: port ${port} is not a valid integer in the 1-65535 range.`;
}

// FIELD_HELP_CANONICAL_PORT / FIELD_HELP_EXPANDED_PORT: the raw values typed
// into ArtnetConfig's "Universe N target port (optional)" field -- the
// resulting validation message's own length scales directly with whatever
// was typed (a real, user-driven field-help/inline-validation copy path,
// not a mocked string), so the expanded pair genuinely exercises the field
// itself rather than a fabricated standalone string.
export const FIELD_HELP_CANONICAL_PORT = "99999";
export const FIELD_HELP_EXPANDED_PORT = "9".repeat(120);

export const COPY_PAIRS: CopyPair[] = [
  {
    id: "shell-titlebar-project-name",
    category: "shell",
    description:
      "TitleBar's centered show-identity label (projectName, derived from ShowService.Inspect's showPath) -- a bounded identity slot expected to truncate with ellipsis while remaining fully available via its own title attribute, per UI-SPEC's 'User-authored names in chrome/rows/pads' contract.",
    canonical: "Fall Tour",
    // Deliberately long enough to guarantee ellipsis truncation at BOTH
    // required widths (900/1280) regardless of exact font metrics -- a
    // borderline-length string would only truncate at the narrower width,
    // leaving the wider one an accidental false negative for "the fixed
    // titlebar chrome does not grow."
    expanded:
      "Fall Tour — Winter Festival 2026 Main Stage Full Production Backup Copy (Archive Revision 3, Confidential Technical Rider, Do Not Distribute Outside The Touring Party, Front Of House Reference Only, Monitor World Copy, Lighting Department Master File, Final Approved Version)",
  },
  {
    id: "dialog-notes-delete-confirm",
    category: "dialog",
    description:
      "NotesWorkspace's destructive ConfirmModal impact message (Copywriting Contract's 'This {specific impact}. This can't be undone.' pattern, interpolating the selected note's own title) -- expected to wrap and remain fully readable, never truncated.",
    canonical: "Show Notes",
    expanded:
      "Show Notes — Full Production Run-of-Show Cue Sheet and Technical Rider Annotations (Draft Revision 7, Confidential)",
  },
  {
    id: "shared-states-diagnostics-structural-error",
    category: "shared-states",
    description:
      "DiagnosticsWorkspace's Integrity Check structuralError paragraph (ShowService.Diagnose's own real diagnostic text) -- a shared error-state surface expected to wrap and preserve the complete actionable message.",
    canonical: "Integrity check failed.",
    expanded:
      "Integrity check failed: 3 orphaned scene references, 2 duplicate fixture instance IDs, and a schema-version mismatch were detected while validating the show file; a verified backup was created automatically before this check ran.",
  },
  {
    // Both strings here are the Guided First Show's own REAL, already-
    // shipped evidence copy (readiness.ts's deriveAssignStatus), not a
    // mocked/injected string: listLocalFixtures/wailsBridge.ts's own
    // "never throw" contract (every list* wrapper catches and falls back
    // to an explicit empty projection) makes it structurally impossible to
    // reach an ErrorState on this stage via a rejected mock, so this pair
    // instead proves the exact same evidence-list UI already renders both
    // its short blocker row and its notably longer, always-appended MIDI-
    // hardware evidence row correctly side by side -- a genuine >=2x
    // real-copy pair the Assign stage renders unconditionally whenever
    // zero operator surfaces exist (the default installHealthyBindings
    // SurfaceService.ListSurfaces() => [] state, no extra mocking needed).
    id: "guided-first-show-assign-stage-evidence",
    category: "guided-first-show",
    description:
      "Guided First Show's Assign stage evidence list (readiness.ts's deriveAssignStatus): the short 'No operator surface yet' blocker detail next to the notably longer, always-appended 'MIDI hardware (optional)' evidence detail -- both real, already-shipped copy rendered in the same evidence list, proving it wraps/reflows a genuinely longer row without truncation, clipping, or overlap.",
    canonical: "Create an operator surface before handing this show off to a player.",
    expanded:
      "Physical MIDI hardware evidence is optional for on-screen and keyboard operation -- it's required only for a named hardware compatibility claim.",
  },
  {
    id: "status-diagnostics-app-log-row",
    category: "status",
    description:
      "Diagnostics workspace's live Application Log stream (AppLogPanel, App.RecentAppLogs-backed) -- one row's own message text, expected to wrap inside its bounded, scrolling log stream rather than clip or force the stream wider.",
    canonical: "Art-Net daemon reconnected.",
    expanded:
      "Art-Net daemon reconnected after a brief network interface flap on Ethernet; output resumed automatically and no frames were dropped during the outage window.",
  },
  {
    id: "field-help-artnet-port-usage",
    category: "field-help",
    description:
      "ArtnetConfig's inline field-validation message for an out-of-range universe target port (GOLC_ARTNET_USAGE) -- a real, user-typed-length-driven field/help copy path (not a mocked string): the message's own length scales directly with the digit string typed into the port Field.",
    canonical: artnetPortUsageMessage(FIELD_HELP_CANONICAL_PORT),
    expanded: artnetPortUsageMessage(FIELD_HELP_EXPANDED_PORT),
  },
];

// assertPairsMeetMinimumExpansion fails loudly (thrown, not a soft warning)
// the instant any registered pair's expansion ratio falls below the plan's
// required 2.0x floor -- called at module scope from the spec file itself,
// before any browser ever opens, so a badly-authored pair can never
// silently pass by having its ratio check skipped or swallowed.
export function assertPairsMeetMinimumExpansion(pairs: CopyPair[] = COPY_PAIRS): void {
  for (const pair of pairs) {
    const ratio = expansionRatio(pair);
    if (ratio < MINIMUM_EXPANSION_RATIO) {
      throw new Error(
        `expandedCopy fixture pair "${pair.id}" only expands ${ratio.toFixed(2)}x (need >= ${MINIMUM_EXPANSION_RATIO}x): canonical="${pair.canonical}" expanded="${pair.expanded}"`,
      );
    }
  }
}
