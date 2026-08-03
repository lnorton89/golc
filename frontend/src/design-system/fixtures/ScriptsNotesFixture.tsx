// ScriptsNotesFixture.tsx (Plan 13-34 Task 3, UI-SPEC-VISUAL-MATRIX): a
// deterministic, browser-reachable e2e-only fixture mounting the real
// ScriptsWorkspace (a real Monaco instance plus its own Save/Delete/
// Validate/Run/Debug/Stop toolbar) directly adjacent to the real
// NotesWorkspace (a real Tiptap instance plus its own autosave status) --
// UI-SPEC's Required reference matrix names this exact combined state
// ("Monaco and Tiptap adjacent controls using shared chrome") for the
// "Scripts / Notes" row. No single existing navigable destination renders
// both editors on one screen: ScriptsWorkspace (Build) and NotesWorkspace
// (Show) are two separate WorkspaceRouter destinations. Mirrors the
// established ?e2e=... fixture-route precedent (DialogFeasibility.tsx,
// DesignSystemGallery.tsx, EmergencyFallbackFixture.tsx, and this plan's
// own DeskOperatorFixture.tsx) rather than inventing a second mechanism --
// both children are the real production workspace components, unmodified;
// only the deterministic Wails seed is supplied by the spec file that
// mounts this route (frontend/e2e/design-system.visual-live-editors.spec.ts).
import ScriptsWorkspace from "../../workspaces/build/ScriptsWorkspace";
import NotesWorkspace from "../../workspaces/show/NotesWorkspace";

export default function ScriptsNotesFixture() {
  return (
    <main
      aria-label="Scripts and Notes fixture"
      style={{ display: "flex", flexDirection: "column", height: "100vh", boxSizing: "border-box", gap: "var(--ds-spacing-space2)", padding: "var(--ds-spacing-space2)" }}
    >
      {/* A compact heading (not the default browser h1 sizing/margins):
          this fixture's captured content needs every available pixel of
          the 720px regression viewport height. */}
      <h1 style={{ margin: 0, fontSize: "var(--ds-typography-font-size-heading)" }}>Scripts / Notes</h1>
      {/* Stacked (not side by side): a two-column split at the 900px
          compact-width regression viewport squeezes NotesWorkspace's own
          list+editor grid (list column defaults to 240px) into an
          unrealistically narrow remaining editor column no real
          navigable destination would ever render at -- mirrors
          DeskOperatorFixture.tsx's identical reasoning/fix. Stacking
          vertically instead gives each real production workspace its own
          full page width, with an explicit equal-share height (flex: 1 on
          a definite-height flex column) so Monaco/Tiptap's own
          `flex: 1; min-height: 0` chain resolves to a real, non-zero
          pixel height rather than collapsing. */}
      <div style={{ flex: 1, minHeight: 0, display: "flex", flexDirection: "column" }}>
        <ScriptsWorkspace />
      </div>
      <div style={{ flex: 1, minHeight: 0, display: "flex", flexDirection: "column" }}>
        <NotesWorkspace />
      </div>
    </main>
  );
}
