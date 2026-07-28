# Phase 8: Isolated TypeScript Automation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-25
**Phase:** 8-Isolated TypeScript Automation
**Areas discussed:** Debugging experience, Capability & resource-limit assignment, Termination & safety behavior, Script editor & authoring workflow

---

## Debugging Experience

| Option | Description | Selected |
|--------|-------------|----------|
| Structured logs & diagnostics only (Recommended) | Matches SCRP-05's literal wording; no breakpoint/step debugger; smaller isolation surface | |
| Real interactive breakpoint debugger | Step-through, breakpoints, variable inspection via V8/Node/Deno Inspector Protocol; larger lift, own isolation question | ✓ |

**User's choice:** Real interactive breakpoint debugger
**Notes:** Deliberate departure from the recommended lighter-weight option.

| Option | Description | Selected |
|--------|-------------|----------|
| Full TS-source-mapped stack traces (Recommended) | Source maps translate errors back to original .ts lines | ✓ |
| Summarized diagnostic codes only | GOLC_SCRIPT_*-style codes, no raw stack trace in UI | |

**User's choice:** Full TS-source-mapped stack traces

| Option | Description | Selected |
|--------|-------------|----------|
| Live streaming (Recommended) | Reuses Phase 7 SSE pattern; real-time logs/diagnostics/command outcomes | ✓ |
| Post-run only | Panel populates after script stops | |

**User's choice:** Live streaming

| Option | Description | Selected |
|--------|-------------|----------|
| In the debug panel AND the audit trail (Recommended) | Live per-command feed plus durable API-06 audit record | ✓ |
| Audit trail only | Debug panel shows script-level output only | |

**User's choice:** In the debug panel AND the audit trail

| Option | Description | Selected |
|--------|-------------|----------|
| Debug mode only (Recommended) | Inspector channel only present in explicit Debug launch | ✓ |
| Always available | Every run keeps a live debug connection open | |

**User's choice:** Debug mode only

| Option | Description | Selected |
|--------|-------------|----------|
| Click in the editor gutter (Recommended) | Standard IDE-style breakpoint UX | ✓ |
| `debugger;` statements only | No gutter UI, edit script to change breakpoints | |

**User's choice:** Click in the editor gutter

---

## Capability & Resource-Limit Assignment

| Option | Description | Selected |
|--------|-------------|----------|
| Reuse Phase 7's coarse domain scopes (Recommended) | Same scope enum/enforcement point as internal/api | ✓ |
| New finer-grained script-specific model | Explicit per-command allowlist, separate model | |

**User's choice:** Reuse Phase 7's coarse domain scopes

| Option | Description | Selected |
|--------|-------------|----------|
| Per-script default, editable before each run (Recommended) | Saved profile, pre-filled, adjustable at run time | ✓ |
| Per-run only, no saved defaults | Set fresh every time | |

**User's choice:** Per-script default, editable before each run

| Option | Description | Selected |
|--------|-------------|----------|
| Immediate hard termination, logged (Recommended) | Hard kill the instant a limit is exceeded | ✓ |
| Warning, then grace period before kill | Brief wind-down window first | |

**User's choice:** Immediate hard termination, logged

| Option | Description | Selected |
|--------|-------------|----------|
| Named presets with an Advanced/custom escape hatch (Recommended) | Presets like "Quick action"; custom fields available | ✓ |
| Fully custom numeric fields only | Raw numbers only | |

**User's choice:** Named presets with an Advanced/custom escape hatch

---

## Termination & Safety Behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Separate lightweight per-script Stop (Recommended) | No hold-to-confirm; Revoke Automation stays the global override | ✓ |
| Route through the same hold-to-confirm safety-cluster UX | Every stop uses the deliberate hold gesture | |

**User's choice:** Separate lightweight per-script Stop

| Option | Description | Selected |
|--------|-------------|----------|
| Let in-flight commands finish, then terminate (Recommended) | No new commands after termination begins, but accepted commands complete | ✓ |
| Immediate hard-kill, in-flight commands abandoned | Process killed instantly regardless | |

**User's choice:** Let in-flight commands finish, then terminate

| Option | Description | Selected |
|--------|-------------|----------|
| Keep last-run logs visible until dismissed or re-run (Recommended) | Panel shows "Stopped: [reason]" plus final logs | ✓ |
| Reset to clean idle state immediately | Panel returns to ready state right away | |

**User's choice:** Keep last-run logs visible until dismissed or re-run

| Option | Description | Selected |
|--------|-------------|----------|
| Always explicit, no auto-restart (Recommended) | User always presses Run again deliberately | ✓ |
| Optional per-script auto-restart policy | Restart on crash, up to N times | |

**User's choice:** Always explicit, no auto-restart

---

## Script Editor & Authoring Workflow

| Option | Description | Selected |
|--------|-------------|----------|
| Single-file scripts only (Recommended) | One self-contained .ts file per script | ✓ |
| Multi-file/project-style with local imports | Scripts can import sibling files | |

**User's choice:** Single-file scripts only

| Option | Description | Selected |
|--------|-------------|----------|
| Full live type-checking & autocomplete (Recommended) | Real TS language service (e.g. Monaco) against generated SDK types | ✓ |
| Syntax highlighting only, validate on demand | Lighter editor, click-to-validate | |

**User's choice:** Full live type-checking & autocomplete

| Option | Description | Selected |
|--------|-------------|----------|
| Script library listing every saved script (Recommended) | Dedicated Scripts destination, Build nav group pattern | ✓ |
| Single "current script" scoped to context | No persistent named collection | |

**User's choice:** Script library listing every saved script

| Option | Description | Selected |
|--------|-------------|----------|
| Inside the .golc show file (Recommended) | Scripts are an entity in show.State, travel with the show | ✓ |
| Separate .ts files on disk, referenced by the show | Ordinary text files, external-editor friendly | |

**User's choice:** Inside the .golc show file

---

## Claude's Discretion

None — every gray area discussed converged on an explicit user selection.

## Deferred Ideas

None — discussion stayed within phase scope. Auto-restart policies and multi-file scripts were explicitly considered and declined for this phase (locked-out decisions, not deferred backlog items).
