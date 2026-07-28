# Phase 8: Isolated TypeScript Automation - Context

**Gathered:** 2026-07-25
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can create, edit, validate, run, stop, and debug capability-limited TypeScript scripts against a generated typed GOLC SDK, executing in a supervised process that has no ambient filesystem, network, environment, subprocess, or native-code access and can never own or delay deterministic playback or Art-Net output.

This phase clarifies HOW to deliver the debugging experience, capability/resource-limit assignment, termination/safety behavior, and script editor/authoring workflow. It does not add new domain capabilities beyond SCRP-01 through SCRP-06.

Requirements: SCRP-01 through SCRP-06.

</domain>

<decisions>
## Implementation Decisions

### Debugging Experience
- **D-01:** v1 ships a **real interactive breakpoint/step debugger**, not structured-logs-only. This directly resolves the ROADMAP's open "debugger scope" research question in favor of a full debugger.
- **D-02:** The debug inspector channel is available **only in an explicit "Debug" launch mode** — a normal "Run" never opens or exposes an inspector/debug-protocol connection into the sandboxed process. This keeps the default execution path free of the extra privileged channel a real debugger requires.
- **D-03:** Script errors surface as **full TypeScript-source-mapped stack traces** (via source maps back to the author's original `.ts`), not summarized diagnostic codes only.
- **D-04:** The log/diagnostics panel **streams live** while a script runs, reusing Phase 7's SSE event-stream pattern (`07-CONTEXT.md` D-09..D-12) — not populated only after the script stops.
- **D-05:** Each command outcome (every typed-SDK call) appears **individually in the script's own debug panel in real time, in addition to** being recorded in the Phase 7 API-06 audit trail — not audit-trail-only.

### Capability & Resource-Limit Assignment
- **D-06:** Scripts **reuse Phase 7's existing coarse API-key domain scopes** (`playback`/`authoring`/`admin`, `07-CONTEXT.md` D-08) for capability assignment rather than inventing a separate, finer-grained script-specific capability model.
- **D-07:** Capability/deadline/rate-limit/resource-limit assignments are a **per-script saved default profile**, shown pre-filled and editable in the run dialog before each execution — not re-entered from scratch every run, and not silently reused without review.
- **D-08:** Exceeding a deadline, rate limit, or resource limit is an **immediate hard termination, logged with the reason** — no warning/grace period first.
- **D-09:** Resource/rate limits are set via **named presets** (e.g. "Quick action," "Long-running automation") **with an Advanced/custom escape hatch** for raw numeric values — not raw numeric fields as the primary interface.

### Termination & Safety Behavior
- **D-10:** Per-script Stop is a **separate, lightweight action scoped to that one script** (no hold-to-confirm gesture) — distinct from Phase 6's Revoke Automation, which remains the global override blocking all scripts + AI at once. Stop targets one script; Revoke Automation targets everything.
- **D-11:** On termination (Stop, deadline, crash), any command the script already issued and that `internal/command` already accepted is **allowed to finish**; the script stops being able to issue new commands the instant termination begins, but in-flight work is not abandoned mid-mutation.
- **D-12:** After a script stops or is terminated, its **last logs/diagnostics/status remain visible** in the panel ("Stopped: [reason]") until the user dismisses them or re-runs — the panel does not silently reset to a clean idle state.
- **D-13:** **No auto-restart, ever.** A crashed, blocked, or terminated script always requires an explicit user action (pressing Run again). This is a deliberate hard boundary against Phase 8 growing autonomous-looping behavior that belongs to Phase 9's bounded-autonomy model.

### Script Editor & Authoring Workflow
- **D-14:** Scripts are **single self-contained `.ts` files** — no multi-file/project-style scripts with local imports in this phase. One file is one validation/capability/sandbox execution unit.
- **D-15:** The in-app editor provides **full live type-checking and autocomplete** against the generated GOLC SDK's types (a real TypeScript language service, e.g. Monaco, loading the SDK's `.d.ts`) — not syntax-highlighting-only with validate-on-demand.
- **D-16:** A **dedicated script library view** lists every saved script in the show (name, last-run status, assigned capability profile), following the same library pattern as the existing Build-group workspaces (Fixture Library, Patch & Pools) — scripting is not scoped to just "whatever's currently open."
- **D-17:** Scripts are **saved inside the `.golc` show file** as another entity in `show.State` — not as separate `.ts` files on disk — so they get the same autosave/recovery/migration/export treatment as the rest of the show and travel with it when shared (per Phase 5's single-portable-file model, SHOW-01).

### Claude's Discretion
None — every gray area discussed converged on an explicit user selection. Note that D-01 (real breakpoint debugger) was a deliberate departure from the recommended lighter-weight option; every other decision took the recommended path.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements this phase must satisfy
- `.planning/REQUIREMENTS.md` §"TypeScript Scripting" (SCRP-01 through SCRP-06) — the six locked requirements this phase's decisions were made to satisfy.
- `.planning/ROADMAP.md` §"Phase 8: Isolated TypeScript Automation" — goal, dependencies (Phase 7), success criteria, and the research note flagging Deno distribution, offline dependency policy, process/IPC isolation, Windows CPU/memory enforcement, debugger scope (now resolved to "real debugger" by D-01/D-02), supervision, cancellation, and defensible sandbox claims as open research areas.

### Project-level constraints
- `.planning/PROJECT.md` §Constraints — "Live reliability" (DMX/Art-Net output and playback timing cannot depend on... script responsiveness) and §Context — "TypeScript is a first-class automation surface, not an incidental plugin format. Scripts should use the same domain capabilities available to the UI and API" — governs D-06's decision to reuse Phase 7's domain scopes rather than inventing a parallel model.
- `.planning/STATE.md` §Accumulated Context → Decisions — "UI, persistence, scripts, API, LLM, and Linear never own or block deterministic playback or Art-Net timing" — the governing constraint behind D-11's "in-flight commands finish, but the script cannot issue new ones" termination design.

### Existing command/API/translation authority this phase must reuse, not duplicate
- `internal/command/router.go` — `Request`/`Result`/`CommandRegistry.Execute`, the single execution authority the generated SDK (SCRP-02) and every script command call must ultimately reach, the same way Phase 7's REST layer does.
- `internal/api` (Phase 7) — `translate.go`, `auth.go`, `keys.go` (coarse domain scopes, `07-CONTEXT.md` D-08), `ratelimit.go`, `batch.go`, `dryrun.go`, `idempotency.go`, `audit.go` — the existing translation-layer and scope-enforcement pattern D-06 reuses directly for script capability checks, and the existing audit pipeline D-05 writes command outcomes into.
- `internal/api/events.go`, `internal/api/observer.go` — the existing SSE event-stream implementation (Phase 7 D-09..D-12: one global stream, monotonic revision, `Last-Event-ID` gap recovery) that D-04's live log/diagnostics streaming should follow rather than building a second streaming mechanism.
- `internal/contracts/generate.go` — the deterministic Go-to-JSON-Schema generation pattern (self-registering `SchemaDescriptor`s, byte-stable output, drift checking) — the closest existing precedent for generating the typed TypeScript SDK (SCRP-02); research should determine whether the SDK generates from this same schema registry or from the OpenAPI contract Phase 7 already produces.

### Existing subprocess/isolation precedent this phase builds on
- `internal/trace/transport/process.go` — `ProcessClient`/`ProcessConfig`: a pinned-executable subprocess launcher over a strict newline-delimited JSON protocol, with per-call deadline enforcement, full process-tree kill on timeout/cancellation (Windows `taskkill /T /F`), and redaction-scanned bounded stderr capture. This is the closest existing precedent in the repo for spawning and supervising the isolated script-execution process (D-02's debug-mode-gated inspector channel, D-08's hard-kill-on-limit-overrun, D-13's no-auto-restart) — research should determine what changes are needed for a persistent (not one-shot Call) script host versus this existing one-shot RPC client.
- `internal/artnet/ipc` — existing cross-platform IPC transport (Windows named pipe / Unix socket) built during the PowerShell-removal work; a candidate transport for the script host's command/event channel back to the daemon, alongside or instead of a new mechanism.
- `internal/wails/app.go` — the existing supervised-daemon-child spawn/dial/orphan-cleanup lifecycle (Phase 6/7 precedent) the script-execution process's own start/stop/supervision should likely slot into.

### Frontend/UI precedent this phase builds on
- `frontend/src/shell/navigation.ts` — the locked `Show/Build/Operate/Output` rail; D-16's script library destination is added the same mechanical way as existing entries (one `NavGroup` entry + one `WorkspaceRouter.tsx` case), likely under `Build` alongside Fixture Library/Patch & Pools/Scenes & Looks.
- `frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx` (and sibling `PatchPoolsWorkspace.tsx`, `ScenesLooksWorkspace.tsx`) — existing library-workspace pattern D-16's script library should follow structurally.
- `.planning/sketches/SKILL.md` and its `references/` — validated GOLC design system (Paper/Ink palette, Signal Blue for selection/live/manual control, compact panel/inspector conventions). No sketch specifically covers a script editor/debugger UI, so this phase's editor/debugger panel is new visual territory that should still follow the locked shell/panel/inspector primitives rather than inventing a new visual language.

No user-referenced ADRs/specs beyond the project's own planning docs and the codebase precedents above came up during discussion — no additional canonical docs to add.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/trace/transport.ProcessClient` — pinned-executable launcher, deadline enforcement, process-tree kill, redacted stderr — the closest structural precedent for the sandboxed script host process, though it's currently a one-shot RPC client, not a persistent host; expect real changes, not a drop-in reuse.
- `internal/api`'s translation/auth/scope/audit/SSE stack (Phase 7) — reused directly for D-06 (capability scopes), D-05 (command outcomes to audit), and D-04 (live streaming pattern).
- `internal/contracts`'s schema-generation registry — closest precedent for D-15's generated SDK type definitions.
- `frontend/src/workspaces/build/*` — existing library-workspace components as the structural template for D-16's script library.

### Established Patterns
- `{DOMAIN}_{CONDITION}` diagnostic code convention (e.g. `GOLC_TRANSPORT_ADAPTER_MISSING`, `GOLC_SHOW_STATE_INVALID`) — new Phase 8 diagnostics (capability denial, limit overrun, debug-channel errors) should follow the same naming convention.
- Self-registering routes/schemas via `MustDeclareRoute`/`MustRegisterSchema` — the established pattern for any new internal/command routes or generated-SDK schema entries this phase adds.
- Supervised daemon-child process lifecycle (Phase 6/7) — the pattern the script-execution process's own supervision should follow rather than inventing a new process-ownership model.

### Integration Points
- No `internal/script` (or equivalent) package exists yet — this is new, greenfield code for script storage, execution supervision, capability enforcement, and SDK generation.
- No script entity exists yet in `internal/show`'s `show.State` — D-17's "scripts live inside the .golc file" requires new fields/types extending `State`, following the established Phase 2/3/5 pattern of extending the single revisioned document.
- No script-editor or debugger frontend component exists yet — D-15/D-16's editor and library are new frontend surfaces, likely a new `frontend/src/workspaces/build/ScriptsWorkspace.tsx` (or similar) plus a Monaco (or equivalent) editor integration, itself a new frontend dependency not yet in the repo.

</code_context>

<specifics>
## Specific Ideas

- The debugging decision (D-01) was a deliberate departure from the recommended lighter-weight option — the user explicitly wants a real breakpoint/step debugger, not just structured logs. The debug-mode-gated inspector channel (D-02) exists specifically to make that safe to build without weakening the default sandbox posture.
- No particular external tool/library references or "I want it like X" examples were given — all decisions were concrete answers to concrete implementation-choice questions.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. No scope-creep suggestions arose during the 4 discussed areas. Auto-restart policies (D-13) and multi-file scripts (D-14) were explicitly considered and declined for this phase rather than deferred as separate ideas — they are locked-out decisions, not backlog items.

</deferred>

---

*Phase: 8-Isolated TypeScript Automation*
*Context gathered: 2026-07-25*
