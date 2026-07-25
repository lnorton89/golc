# Phase 8: Isolated TypeScript Automation - Research

**Researched:** 2026-07-25
**Domain:** Sandboxed TypeScript script execution, process isolation, Windows resource enforcement, and interactive debugging for a Go-owned live-lighting controller
**Confidence:** MEDIUM-HIGH (runtime/toolchain facts verified against current official sources; exact Job Object tuning values and final debug-bridge ergonomics require implementation-time validation)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Debugging Experience**
- **D-01:** v1 ships a **real interactive breakpoint/step debugger**, not structured-logs-only.
- **D-02:** The debug inspector channel is available **only in an explicit "Debug" launch mode** — a normal "Run" never opens or exposes an inspector/debug-protocol connection into the sandboxed process.
- **D-03:** Script errors surface as **full TypeScript-source-mapped stack traces** (via source maps back to the author's original `.ts`), not summarized diagnostic codes only.
- **D-04:** The log/diagnostics panel **streams live** while a script runs, reusing Phase 7's SSE event-stream pattern (`07-CONTEXT.md` D-09..D-12) — not populated only after the script stops.
- **D-05:** Each command outcome (every typed-SDK call) appears **individually in the script's own debug panel in real time, in addition to** being recorded in the Phase 7 API-06 audit trail — not audit-trail-only.

**Capability & Resource-Limit Assignment**
- **D-06:** Scripts **reuse Phase 7's existing coarse API-key domain scopes** (`playback`/`authoring`/`admin`) for capability assignment rather than inventing a separate, finer-grained script-specific capability model.
- **D-07:** Capability/deadline/rate-limit/resource-limit assignments are a **per-script saved default profile**, shown pre-filled and editable in the run dialog before each execution.
- **D-08:** Exceeding a deadline, rate limit, or resource limit is an **immediate hard termination, logged with the reason** — no warning/grace period first.
- **D-09:** Resource/rate limits are set via **named presets** with an **Advanced/custom escape hatch** for raw numeric values.

**Termination & Safety Behavior**
- **D-10:** Per-script Stop is a **separate, lightweight action scoped to that one script** (no hold-to-confirm gesture) — distinct from Phase 6's global Revoke Automation.
- **D-11:** On termination, any command the script already issued and that `internal/command` already accepted is **allowed to finish**; the script stops being able to issue new commands the instant termination begins.
- **D-12:** After a script stops or is terminated, its **last logs/diagnostics/status remain visible** ("Stopped: [reason]") until dismissed or re-run.
- **D-13:** **No auto-restart, ever.** A crashed, blocked, or terminated script always requires an explicit user action.

**Script Editor & Authoring Workflow**
- **D-14:** Scripts are **single self-contained `.ts` files** — no multi-file/project-style scripts with local imports.
- **D-15:** The in-app editor provides **full live type-checking and autocomplete** against the generated GOLC SDK's types (a real TypeScript language service, e.g. Monaco, loading the SDK's `.d.ts`).
- **D-16:** A **dedicated script library view** lists every saved script (name, last-run status, assigned capability profile), following the Build-group workspace library pattern.
- **D-17:** Scripts are **saved inside the `.golc` show file** as another entity in `show.State` — not as separate `.ts` files on disk.

### Claude's Discretion
None — every gray area discussed converged on an explicit user selection. D-01 (real breakpoint debugger) was a deliberate departure from the recommended lighter-weight option; every other decision took the recommended path.

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope. Auto-restart policies (D-13) and multi-file scripts (D-14) were explicitly considered and declined, not deferred — they are locked-out decisions, not backlog items.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SCRP-01 | A user can create, edit, validate, run, stop, and debug TypeScript scripts from the application. | Editor Architecture, Process/IPC Isolation Model, Debugger Integration sections; Validation Architecture maps each verb to a concrete test. |
| SCRP-02 | Scripts use a generated typed GOLC SDK for commands, queries, and events rather than raw DMX access. | Generated TypeScript SDK section — recommends generating from `internal/contracts`'s existing schema registry, not the OpenAPI document, with capability-scoped runtime enforcement. |
| SCRP-03 | Scripts execute outside the playback process with no ambient filesystem, network, environment, subprocess, or native-code permissions. | Deno Runtime Selection, Process/IPC Isolation Model — zero-permission `deno run` per script, all SDK traffic over stdio (never through a Deno `--allow-*` gated API). |
| SCRP-04 | A user can assign script capabilities, deadlines, rate limits, and resource limits before execution. | Capability & Resource-Limit Enforcement, Windows CPU/Memory/Deadline Enforcement sections. |
| SCRP-05 | A user can inspect structured script logs, diagnostics, source locations, command outcomes, and cancellation status. | Live Log/Diagnostics Streaming, Debugger Integration (source-mapped stack traces) sections. |
| SCRP-06 | A runaway, crashed, or blocked script can be terminated without interrupting playback or Art-Net output. | Supervision & Cancellation section — explains why this is structural (separate OS process/Job Object), not conventional. |
</phase_requirements>

## Summary

GOLC's own prior research (`.planning/research/STACK.md`, `.planning/research/SUMMARY.md`, both dated 2026-07-17) already selected **Deno as a supervised, killable sidecar process** for TypeScript execution, ahead of this phase. That choice holds up under deeper verification: Deno's deny-by-default permission model (`--allow-read/-write/-net/-env/-run/-ffi`, all denied unless explicitly granted `[CITED: docs.deno.com/runtime/reference/permissions]`) gives GOLC a script runtime that requires **zero permission flags at all** for its actual v1 use case, because scripts never need raw filesystem/network/env access — every interaction with GOLC happens through a typed SDK that crosses the process boundary over the child's own stdio, which is not gated by any Deno permission flag. Current stable Deno is **2.9.4** (July 23, 2026) `[CITED: github.com/denoland/deno/releases]`.

The single biggest architectural decision this research resolves is the **process/IPC model**: run one **fresh, zero-permission `deno run` OS subprocess per script execution** (not a shared persistent Deno process using Workers, and not an embedded `deno_core` runtime). Deno's own documentation states a Worker's permissions "can't be extended beyond its parent's" `[CITED: docs.deno.com/runtime/fundamentals/workers]` — so a shared long-lived Deno process hosting multiple scripts via Workers would itself need to hold the union of every possible per-script permission grant, which directly undermines the "no ambient access" claim SCRP-03 requires. A fresh, deny-everything subprocess per run keeps every guarantee at the strongest (OS process) boundary and lets each run carry its own Windows Job Object for hard CPU/memory/deadline enforcement — something Deno's own `--v8-flags=--max-old-space-size` does **not** reliably provide at the OS level `[CITED: github.com/denoland/deno issues #18935, #30043]`.

The "persistent script host" the phase context asks about is **not a new standalone binary** — it is a generalization of the exact `internal/trace/transport/process.go` pattern already in the repo, from a one-shot request/response `Call` into a long-lived, bidirectional, multiplexed newline-delimited-JSON session (log lines, command-call/result pairs, cancellation, status) owned by a new `internal/script` package inside the existing `golc-project` daemon process. `internal/artnet/ipc`'s named-pipe transport is a different hop (Wails UI process ↔ headless daemon process) and should **not** be reused for the daemon↔Deno-child hop, which is a direct parent/child `os/exec` stdio relationship exactly like `process.go` already establishes for the Linear adapter.

For the real interactive debugger D-01/D-02 requires: Deno's `--inspect-brk=127.0.0.1:<port>` starts a Chrome DevTools/V8 Inspector Protocol WebSocket server **only when that flag is passed** `[CITED: docs.deno.com/runtime/fundamentals/debugging]` — in normal Run mode the flag is simply never passed, so no inspector server exists at all (structural, not conventional, satisfaction of D-02). The Go daemon should be the **sole** CDP client, using `github.com/mafredri/cdp` (Go-native, type-safe CDP bindings; 794 GitHub stars, pushed December 2025, not archived `[VERIFIED: GitHub API]`), translating Monaco gutter-breakpoint UI actions into CDP `Debugger.*` calls and CDP `Debugger.paused`/exception events into GOLC's own typed debug-session events over the existing Phase 7 SSE stream. The browser-hosted Monaco frontend never opens a raw CDP connection itself.

**Primary recommendation:** Pin Deno 2.9.x into `config/toolchain.toml` alongside Go/Node/Mage; spawn one zero-permission `deno run` subprocess per script execution, wrapped in a per-process Windows Job Object (via `golang.org/x/sys/windows`, already a project dependency) for hard CPU/memory/deadline enforcement; carry all SDK/log/debug traffic over a generalized, persistent version of `process.go`'s stdio JSON-lines protocol; bridge the debugger through Deno's built-in `--inspect-brk` and a Go-native CDP client, gated strictly to Debug launch mode; generate the typed SDK from `internal/contracts`'s existing schema registry, not the OpenAPI document.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Script editor (Monaco, live type-check/autocomplete) | Browser/Client | — | Runs entirely in the Wails-hosted React webview against a locally-loaded generated `.d.ts`; no round-trip needed for type-checking. |
| Script library view (D-16) | Browser/Client | API/Backend | Frontend workspace renders a list; backend (`internal/show`) is the source of truth for saved scripts and last-run status. |
| Script storage (`.golc` `show.State` entity, D-17) | Database/Storage | API/Backend | Persisted the same way as every other `show.State` entity (pools, scenes) — autosave/recovery/migration/export come for free. |
| Script execution sandbox (Deno subprocess) | API/Backend | — | Owned and supervised by the Go daemon process (`internal/script`), not the browser/webview — the webview never talks to Deno directly. |
| Capability/scope enforcement (D-06) | API/Backend | — | Reuses Phase 7's `RequireScope` exactly; enforced host-side per SDK call, never trusted to the script process. |
| Resource/CPU/memory/deadline enforcement (D-08/D-09) | API/Backend | — | Windows Job Object + Go `context.WithDeadline`, both owned by the daemon process that spawned the child; the child cannot loosen or observe its own limits. |
| Rate limiting (D-09) | API/Backend | — | `golang.org/x/time/rate`, one limiter per running script instance, identical mechanism to `internal/api/ratelimit.go`. |
| Live log/diagnostics streaming (D-04) | API/Backend | Browser/Client | Daemon relays script stdout/stderr/status as SSE events; frontend is a passive subscriber, exactly like Phase 7's existing "state" event stream. |
| Command outcome audit (D-05) | API/Backend | Database/Storage | Every SDK call is recorded through the same Phase 7 API-06 audit pipeline in addition to the live debug panel. |
| Interactive debugger bridge (D-01/D-02) | API/Backend | — | Go daemon is the sole CDP client against Deno's loopback-only inspector; the browser never opens a raw CDP/inspector connection. |
| Debugger UI (breakpoint gutter, step controls) | Browser/Client | API/Backend | Monaco glyph-margin decorations render breakpoints; backend translates UI actions to/from CDP. |
| Generated TypeScript SDK (SCRP-02) | API/Backend | Browser/Client | Generated from the Go command/schema registry (backend authority); consumed as static types + a stdio-calling runtime shim inside the sandboxed process. |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Deno | **2.9.4** (July 23, 2026) `[CITED: github.com/denoland/deno/releases]` | Sandboxed TypeScript execution runtime for user scripts | Deny-by-default permission model, native TypeScript execution (no separate transpile step), built-in `deno check` type-checking, built-in `--inspect-brk` V8 Inspector Protocol server. Already the project's own prior-research selection (`STACK.md`). |
| `golang.org/x/sys` | **v0.46.0** (already pinned in `go.mod`) `[VERIFIED: go.mod]` | Windows Job Object syscalls (`CreateJobObject`, `SetInformationJobObject`, `AssignProcessToJobObject`) | Official Go team module, already a dependency; the only defensible first-party path to real Windows CPU/memory enforcement (no small third-party wrapper is broadly maintained enough for this safety-critical role — see Package Legitimacy Audit). |
| `golang.org/x/time/rate` | already used in `internal/api/ratelimit.go` `[VERIFIED: internal/api/ratelimit.go]` | Per-script rate limiting (D-09) | Exact mechanism the project already uses for per-API-key rate limiting; reuse directly rather than a second implementation. |
| `github.com/mafredri/cdp` | **v0.35.0** (Sept 2024 tag; repo pushed Dec 2025) `[VERIFIED: proxy.golang.org, GitHub API]` | Go-native Chrome DevTools/V8 Inspector Protocol client for the debugger bridge (D-01/D-02) | Type-safe Go bindings for CDP; 794 stars, not archived, actively pushed. Lets the debugger bridge live entirely in the trusted Go daemon rather than adding a Node/TS intermediary. |
| `github.com/invopop/jsonschema` | **v0.14.0** (already used in `internal/contracts`) `[VERIFIED: internal/contracts/generate.go, proxy.golang.org]` | Reflects Go command/query/event types for the generated TypeScript SDK | Same reflection library `internal/contracts` already uses for JSON-Schema generation; reuse for SDK generation keeps one generation discipline for the whole codebase. |
| `monaco-editor` | 0.56.0 latest, **recommend pinning an older stable minor pending review** `[SUS — see Package Legitimacy Audit]` | In-app TypeScript editor, glyph-margin breakpoint gutter, live type-check/autocomplete (D-15) | The de facto standard embeddable editor with a real TypeScript language service; VS Code's own editor core. No comparable alternative gives equivalent TS IntelliSense in-browser. |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/security.Redact` | already in repo | Bound and redact captured script stdout/stderr before it reaches SSE/audit | Reuse `process.go`'s exact `boundedBuffer` + `security.Redact` pattern for every script log line, not just failure diagnostics. |
| `internal/contracts` schema registry | already in repo | Source-of-truth Go types for the generated SDK | Extend, don't duplicate — add a new descriptor kind or sibling package that emits `.d.ts`/`.ts` instead of JSON Schema from the same registrations. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Deno per-script subprocess | Deno Worker inside one persistent Deno process | Rejected: a Worker's permissions cannot exceed its parent's `[CITED: docs.deno.com/runtime/fundamentals/workers]`, so the persistent parent would need broad ambient permissions to delegate downward — directly contradicts SCRP-03. Also weaker fault isolation (a V8 bug can, in principle, cross Worker boundaries within one process; it cannot cross OS process boundaries). |
| Deno CLI subprocess | Embed `deno_core` (Rust) directly | Rejected: no official Go bindings exist for `deno_core`; would require unsafe cgo/FFI bridging Rust to Go, a much larger and less-audited surface than spawning a pinned, checksum-verified CLI binary the way Go/Node/Mage are already spawned/pinned. |
| Windows Job Object via `golang.org/x/sys/windows` | `github.com/aoldershaw/proclimit` or `github.com/tprasadtp/go-autotune` | Rejected for this specific safety-critical role: `proclimit`'s only tagged release is from 2019 (unmaintained signal); `go-autotune` has no tagged release at all (pseudo-version only) and solves a different problem (GOMAXPROCS/GOMEMLIMIT autotuning for the *current* process, not capping a *child* process). Hand-writing the Job Object struct/syscalls against the officially maintained `x/sys/windows` keeps the defensible-sandbox-claim path first-party and auditable. |
| Go-native CDP client (`mafredri/cdp`) | A Node/TS intermediary using `chrome-remote-interface` | Rejected: would add a second runtime (Node) and a second process hop purely to speak CDP, when the Go daemon can do it directly and is already the trusted process boundary for every other privileged channel (auth, audit, SSE). |
| Direct CDP↔SSE typed-event bridge | Full Debug Adapter Protocol (DAP) server | Rejected: DAP exists to let one editor debug many runtimes interchangeably (VS Code's use case). GOLC only ever debugs one runtime (Deno) behind one specific frontend (its own Monaco instance) — a second protocol translation layer buys no real interoperability benefit here. |
| Generate SDK from `internal/contracts` registry | Generate SDK from Phase 7's OpenAPI document | Rejected as primary source: OpenAPI is HTTP-shaped (paths/methods/bodies), one translation layer removed from the authoritative Go command types `internal/contracts` already reflects byte-stably with drift-checking. Using the same registry for both JSON Schema and the TS SDK keeps one generation discipline instead of two. |

**Installation:**
```bash
# Deno toolchain: add to config/toolchain.toml (mirrors go/node/mage's existing pattern),
# checksum-pinned from https://github.com/denoland/deno/releases, official_host="github.com".
# mage Bootstrap then provisions it into .tools/installs like every other pinned tool.

# Go modules (backend):
go get github.com/mafredri/cdp@v0.35.0
# golang.org/x/sys and golang.org/x/time are already go.mod dependencies — no new install.

# Frontend (Wails webview):
npm install --save-exact monaco-editor@<pinned-stable-minor>   # see Package Legitimacy Audit before pinning 0.56.0
```

**Version verification:** `golang.org/x/sys` and `github.com/invopop/jsonschema` versions were confirmed live against `go.mod` and `proxy.golang.org` respectively. `mafredri/cdp`'s latest tag and repository activity were confirmed against the GitHub API. `monaco-editor`'s published version and publish date were confirmed against the npm registry and flagged by the legitimacy seam — see below.

## Package Legitimacy Audit

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `monaco-editor` | npm | latest tag (0.56.0) published 2026-07-20, 5 days before this research | 7,937,045/wk | `github.com/microsoft/monaco-editor` | **SUS** (`"too-new"` heuristic on the latest version) | Flagged — planner must add `checkpoint:human-verify` before installing. This reads as a heuristic false-positive (Microsoft ships `monaco-editor` on a fast release cadence and it has 7.9M weekly downloads and an official repo, not the profile of a slopsquat), but the recorded verdict is SUS and must be honored: pin an older, previously-stable minor (e.g. the prior tagged release) rather than the bleeding-edge one, and have a human confirm the exact pinned version before first install. |
| `github.com/mafredri/cdp` | Go module proxy | latest tag v0.35.0 (Sept 2024); repo pushed Dec 2025 | 794 GitHub stars, 50 forks | `github.com/mafredri/cdp` | **OK** | Approved. |
| `golang.org/x/sys` | Go module proxy | already pinned v0.46.0 in `go.mod` | official Go team module | `golang.org/x/sys` | **OK** | Approved — existing dependency, no new install. |
| `golang.org/x/time` | Go module proxy | already in use (`internal/api/ratelimit.go`) | official Go team module | `golang.org/x/time` | **OK** | Approved — existing dependency, no new install. |
| `github.com/invopop/jsonschema` | Go module proxy | latest v0.14.0 (2026-04-23) | already in use (`internal/contracts`) | `github.com/invopop/jsonschema` | **OK** | Approved — existing dependency, no new install. |
| `github.com/aoldershaw/proclimit` | Go module proxy | latest tag from 2019 | low adoption signal | `github.com/aoldershaw/proclimit` | Not run through the legitimacy seam (Go modules aren't the seam's primary ecosystem) — assessed manually via `proxy.golang.org` staleness | **REMOVED** — considered and rejected for the safety-critical Job Object role; see Alternatives Considered. |
| `github.com/tprasadtp/go-autotune` | Go module proxy | no tagged release (pseudo-version only, 2024-06-23) | narrower purpose (GOMAXPROCS/GOMEMLIMIT autotuning) | `github.com/tprasadtp/go-autotune` | Assessed manually | **REMOVED** — wrong tool for capping a *child* process; considered and rejected. |

**Packages removed due to [SLOP]-adjacent verdict:** `github.com/aoldershaw/proclimit`, `github.com/tprasadtp/go-autotune` (both manually assessed as unsuitable/under-maintained for this safety-critical role, not run through the automated npm/PyPI-oriented legitimacy seam — Go ecosystem support in that seam is limited; flagged here for the planner's awareness rather than an automated SLOP verdict).
**Packages flagged as suspicious [SUS]:** `monaco-editor` — planner must add a `checkpoint:human-verify` task before the first `npm install monaco-editor` naming the exact pinned version.

## Architecture Patterns

### System Architecture Diagram

```
 ┌────────────────────────────────────────────────────────────────────────┐
 │ Browser/Client (Wails webview, React)                                  │
 │                                                                        │
 │  ScriptsWorkspace (D-16 library) ──┐                                   │
 │  Monaco editor (D-15 typecheck,    │  1. author/edit .ts               │
 │   D-15 autocomplete via .d.ts) ────┤  2. Validate / Run / Stop / Debug │
 │  Debug panel (D-04 live logs,      │     (typed commands over Wails    │
 │   D-05 per-call outcomes,          │      bindings / REST, same as     │
 │   breakpoint gutter clicks)        │      every other GOLC command)    │
 └──────────────┬─────────────────────┴──────────────┬─────────────────────┘
                │ typed command calls                 │ SSE event stream
                ▼                                      │ (D-04 live logs,
 ┌────────────────────────────────────────────────────┼─ D-05 outcomes,
 │ API/Backend — golc-project daemon (Go)              │  debug-session
 │                                                       │  events)
 │  internal/command router  ─── RequireScope (D-06) ──┤
 │       │                                               │
 │       ▼                                               │
 │  internal/script (NEW)                                │
 │    - run/stop/debug lifecycle, one instance per        │
 │      active script RUN                                │
 │    - per-run: context.WithDeadline (D-08),             │
 │      x/time/rate limiter (D-09)                        │
 │    - spawns a FRESH, zero-permission Deno subprocess ──┼──► kill on
 │      per run (os/exec, generalized process.go          │    Stop/deadline/
 │      protocol: persistent, bidirectional, multiplexed  │    limit breach
 │      JSON-lines: cmd-call / cmd-result / log-line /     │    (Job Object
 │      status / cancel)                                  │    close +
 │    - wraps the child in a Windows Job Object            │    taskkill /T /F)
 │      (golang.org/x/sys/windows) for hard CPU/memory     │
 │      caps                                               │
 │    - Debug mode only: spawns with --inspect-brk=        │
 │      127.0.0.1:<port>; Go daemon is the SOLE CDP        │
 │      client (mafredri/cdp) — translates Monaco           │
 │      breakpoint/step UI ⇄ CDP Debugger.* calls           │
 │                                                          │
 │  internal/api (Phase 7): audit pipeline (D-05),          │
 │    SSE event bus (D-04), scope auth — all reused          │
 │    unmodified for scripts                                 │
 └──────────────┬───────────────────────────────────────────┘
                │ stdio JSON-lines (never Deno --allow-net;
                │ this is inherited process stdio, not a
                │ permission-gated channel)
                ▼
 ┌────────────────────────────────────────┐
 │ Isolated OS process: `deno run`          │  no --allow-read/-write/-net/
 │ (or `deno run --inspect-brk=...` in      │  -env/-run/-ffi granted, ever
 │  Debug mode)                             │
 │  - user's single .ts file (D-14)          │
 │  - generated GOLC SDK bound as globals    │
 │    (no import statements at all —         │
 │    zero import surface, zero network      │
 │    module resolution)                     │
 │  - every SDK call marshalled over stdio    │
 │    to the daemon, awaited as a normal      │
 │    async function call                     │
 └────────────────────────────────────────┘

 ┌────────────────────────────────────────┐
 │ Database/Storage — `.golc` SQLite file    │
 │  show.State.Scripts[] (D-17): script       │
 │  source, name, capability/limit profile     │
 │  (D-07), last-run status — same             │
 │  autosave/recovery/migration path as        │
 │  every other State entity                    │
 └────────────────────────────────────────┘
```

### Recommended Project Structure
```
internal/
├── script/                    # NEW: script lifecycle, subprocess supervision, Job Objects
│   ├── router.go              #   "script run"/"script stop"/"script debug" self-registered routes
│   ├── host.go                #   generalized process.go pattern: persistent, bidirectional session
│   ├── jobobject_windows.go   #   Windows Job Object CPU/memory/deadline enforcement
│   ├── jobobject_other.go     #   non-Windows no-op/fallback (Windows is the only v1 platform)
│   ├── debugbridge.go         #   Deno --inspect-brk ⇄ mafredri/cdp ⇄ SSE debug-session events
│   └── capability.go          #   D-06/D-07/D-08/D-09 enforcement, reusing internal/api's RequireScope
├── scriptsdk/                 # NEW: generated typed SDK generator (sibling to internal/contracts)
│   └── generate.go            #   emits .d.ts (Monaco types) + runtime shim (stdio-calling stubs)
├── show/
│   └── scripts.go             # NEW: Script entity in show.State (D-17), following pool.Pool/scene.Scene pattern
├── contracts/                  # EXISTING — reused, not duplicated, as the schema source of truth
├── api/                         # EXISTING — reused unmodified: RequireScope, audit, SSE event bus
└── trace/transport/process.go   # EXISTING — the generalized pattern internal/script/host.go extends

frontend/src/workspaces/build/
└── ScriptsWorkspace.tsx        # NEW: script library (D-16) + Monaco editor + debug panel,
                                 #   structurally following FixtureLibraryWorkspace.tsx
```

### Pattern 1: Zero-Permission Subprocess Per Script Run
**What:** Every script Run/Debug invocation spawns a brand-new `deno run` (or `deno run --inspect-brk=...`) process with **no `--allow-*` flags at all**. All GOLC interaction happens over the process's own stdin/stdout, which Deno's permission system never gates.
**When to use:** Every script execution, without exception — there is no "trusted script" tier in this phase's scope (D-07 profiles govern which *SDK calls* succeed, not which OS permissions the process holds).
**Example:**
```go
// Source: pattern generalized from internal/trace/transport/process.go
// (existing repo precedent), applied to a persistent session instead of
// a one-shot Call.
cmd := exec.CommandContext(ctx, denoExecutable, "run",
    "--no-prompt",         // never interactively prompt for a permission
    scriptTempPath,        // the single .ts file, materialized to a temp path
)
// No --allow-read/-write/-net/-env/-run/-ffi flags: everything the script
// needs flows over cmd.Stdin/cmd.Stdout as newline-delimited JSON.
```

### Pattern 2: Windows Job Object Hard Kill (not `taskkill` alone)
**What:** Every spawned Deno child is immediately assigned to a fresh Windows Job Object (`CreateJobObject` → `SetInformationJobObject` with `JOBOBJECT_EXTENDED_LIMIT_INFORMATION{ProcessMemoryLimit, JOB_OBJECT_LIMIT_JOB_MEMORY}` and `JOBOBJECT_CPU_RATE_CONTROL_INFORMATION` → `AssignProcessToJobObject`), with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` set.
**When to use:** Every script run, before the child can do anything — assignment happens immediately after `cmd.Start()`, before the child has a chance to spawn any descendant of its own (though it should never be granted `--allow-run` to do so in the first place).
**Example:**
```
// Source: Microsoft Learn, Job Objects (Win32) — https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects
// [CITED]. Implement via golang.org/x/sys/windows syscalls (already a
// pinned dependency), not a third-party wrapper (see Package Legitimacy
// Audit). Closing the job handle kills every process still assigned to
// it — a kernel-enforced superset of the existing taskkill /T /F pattern
// process.go already uses for the Linear adapter.
```

### Pattern 3: Debug-Mode-Only Inspector, Host-Mediated CDP
**What:** `--inspect-brk=127.0.0.1:<ephemeral-port>` is passed **only** when the user explicitly launches Debug mode (never for a plain Run). The Go daemon opens the only CDP WebSocket connection to that loopback port; Monaco/the frontend never does.
**When to use:** D-01/D-02's real breakpoint debugger, gated to Debug launch mode.
**Example:**
```go
// Source: pattern derived from Deno's documented --inspect-brk behavior
// (docs.deno.com/runtime/fundamentals/debugging [CITED]) and
// github.com/mafredri/cdp's connection API.
if launchMode == DebugMode {
    port := pickEphemeralLoopbackPort()
    cmd.Args = append(cmd.Args, fmt.Sprintf("--inspect-brk=127.0.0.1:%d", port))
    // after Start(): connect mafredri/cdp to ws://127.0.0.1:<port>,
    // translate Monaco breakpoint/step commands into Debugger.* calls,
    // translate Debugger.paused/console/exception into SSE debug-session
    // events (same event-bus mechanism as internal/api/events.go).
}
```

### Anti-Patterns to Avoid
- **Granting any `--allow-*` flag "just in case":** Every grant weakens the "no ambient access" claim SCRP-03 makes. If a future capability genuinely needs, say, `--allow-net` to a specific host, that must be an explicit, reviewed, per-profile exception — never a default.
- **Using Deno Workers to avoid subprocess-spawn overhead:** Worker permissions cannot exceed the parent's, so this either requires a broadly-permissioned parent (contradicts SCRP-03) or gains nothing over a fresh subprocess.
- **Relying on `--v8-flags=--max-old-space-size` for memory enforcement:** Documented as unreliable for true OS-level memory capping and does not cover native/off-heap memory (`Deno.memoryUsage().external`) at all `[CITED: GitHub issues #18935, #26202, #30043]`. Job Objects are the only defensible mechanism.
- **Letting the frontend hold a direct CDP/inspector WebSocket:** Would leak the loopback inspector port into the browser's attack surface and contradicts D-02's "no inspector connection outside Debug mode, and never exposed to an untrusted surface" intent. The Go daemon must remain the sole CDP client.
- **Treating a capability-scope violation as a soft no-op:** D-08's "immediate hard termination, no grace period" logic should extend to an out-of-scope SDK call, not just deadline/rate/resource overruns — a script that discovers its profile doesn't allow an action should be terminated with a clear reason, not silently ignored (which would look like the SDK "not working" rather than a deliberate boundary).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Windows CPU/memory hard caps on a child process | A custom polling-based "watch process memory, kill if over X" loop | Windows Job Objects (`JOBOBJECT_EXTENDED_LIMIT_INFORMATION` / `JOBOBJECT_CPU_RATE_CONTROL_INFORMATION` via `golang.org/x/sys/windows`) | Polling has a detection lag (the whole point of D-08 is *immediate* hard termination) and can miss a fast memory spike between polls; Job Objects are kernel-enforced and instantaneous. |
| Chrome DevTools Protocol message framing/typing | Hand-rolled WebSocket + raw JSON CDP message construction | `github.com/mafredri/cdp` | CDP has hundreds of typed methods/events across many domains (`Debugger`, `Runtime`, `Console`); a generated, type-safe Go client avoids an entire class of protocol-drift bugs a hand-rolled client would accumulate. |
| TypeScript type-checking/autocomplete in the editor | A custom or partial TS language-service integration | Monaco's built-in TypeScript language service, fed the generated `.d.ts` | Monaco already ships a full TS language service (the same one VS Code uses); re-implementing autocomplete/live diagnostics against generated types would be redundant, lower-quality effort. |
| Rate limiting per script run | A hand-rolled counter+timestamp map | `golang.org/x/time/rate`, one limiter per active script instance | The project already made and validated this exact choice for `internal/api/ratelimit.go` — reuse, don't reinvent. |
| Source-mapped stack traces (D-03) | A custom V8 stack-frame-to-source-map resolver | Deno's built-in source-map support for TypeScript execution (Deno maps stack traces back to original `.ts` natively, since it executes TS directly without a separate user-visible transpile step) | Deno already does this for its own CLI error output; propagate that same mapped trace through the stdio protocol rather than re-deriving positions independently. |
| Process-tree kill on Stop/deadline/crash | Reinventing `process.go`'s `killProcessTree`/`taskkill /T /F` approach from scratch | Reuse `process.go`'s exact pattern, upgraded with a Job Object close (which is a strict superset — kills every process in the job even if the tree-walk `taskkill` performs would miss a reparented descendant) | Don't duplicate a already-proven, already-tested kill path; extend it. |

**Key insight:** Nearly every "don't hand-roll" item in this phase already has a proven precedent living in the repo (`process.go`'s kill/redaction/deadline pattern, `internal/api/ratelimit.go`'s rate limiter, `internal/contracts`'s schema-reflection generator) or an official, narrowly-scoped library (`x/sys/windows`, `mafredri/cdp`). The main risk is reaching for a second-tier third-party module (like the Job Object wrapper candidates rejected above) for a safety-critical path where a first-party, auditable implementation is both available and more defensible when GOLC needs to substantiate a "capability-limited sandbox" claim.

## Common Pitfalls

### Pitfall 1: Treating Deno's Permission Flags as the Capability Boundary
**What goes wrong:** A natural first instinct is to map D-06/D-07's per-script capability profile (playback/authoring/admin + limits) onto Deno `--allow-*` flags — e.g., "authoring scope" grants `--allow-net=127.0.0.1:<api-port>`.
**Why it happens:** Deno's permission model is the most visible security feature of the runtime, so it's tempting to make it do double duty as GOLC's own domain-scope model.
**How to avoid:** Keep the two boundaries separate. Deno permissions should stay at **zero, always** — the script has no legitimate reason to open a socket or read a file itself. GOLC's playback/authoring/admin scopes are enforced **host-side**, on every SDK call arriving over stdio, by the exact same `RequireScope` check Phase 7 already uses. This is also what D-06 literally asks for: reuse of the *existing* scope model, not a Deno-permission-flag re-encoding of it.
**Warning signs:** Any script capability profile that maps to a `--allow-net`/`--allow-read` flag value, or any code path where a script's network/file access is gated by Deno rather than by a Go-side scope check.

### Pitfall 2: Assuming `--inspect-brk` Is Safe to Leave On "Just for Development"
**What goes wrong:** Leaving the inspector flag on by default (e.g., behind an env var that's easy to forget) would put a debug-protocol WebSocket on loopback for every script run, not just Debug-launched ones — quietly reopening the exact channel D-02 explicitly closes.
**Why it happens:** It's convenient during implementation to always attach a debugger.
**How to avoid:** Gate `--inspect-brk` construction at the single call site that builds the Deno command line, driven directly by the launch-mode value the user selected (Run vs Debug) — never by an environment variable, build flag, or "debug build" convention that could silently persist into a shipped build.
**Warning signs:** Any code path that can add `--inspect-brk` to the argument list without threading through the explicit user-selected launch mode.

### Pitfall 3: Windows Job Object CPU Rate Control Semantics
**What goes wrong:** `JOBOBJECT_CPU_RATE_CONTROL_INFORMATION` supports both a "weight-based" scheduling mode and a "hard cap" mode; using the wrong flag combination silently produces *fair scheduling* (the process gets throttled relative to system load) rather than a *hard percentage ceiling* the way D-08's "immediate hard termination" framing implies is needed for the deadline/limit story.
**Why it happens:** The MSDN structure has multiple valid but semantically different configurations and the difference is easy to miss without close reading.
**How to avoid:** Use `JOB_OBJECT_CPU_RATE_CONTROL_HARD_CAP` explicitly (not the weight-based default) when the goal is "this script never exceeds N% of a core," and pair it with the deadline-based hard termination (`context.WithDeadline` + Job Object close) for the actual "immediate kill" behavior D-08 requires — the CPU rate cap throttles, it does not by itself terminate.
**Warning signs:** A script that "hangs" without ever exceeding a memory limit but also never gets terminated — likely means CPU throttling is working as designed but the separate wall-clock deadline enforcement is missing or misconfigured.

### Pitfall 4: Forgetting Deno's Module Resolution Happens Before Permission Checks
**What goes wrong:** Even with zero `--allow-*` flags granted, Deno's *module resolution* (deciding what `import "..."` refers to) is a separate phase from permission-checked *execution*. A script containing a remote or npm import can still trigger a network fetch attempt during resolution, which then fails (denied) — but the attempt itself, and any information leaked by the failure timing/error message, is undesirable in a "genuinely offline, genuinely no ambient network" story.
**Why it happens:** It's easy to assume "no `--allow-net`" means "the process never attempts network I/O of any kind."
**How to avoid:** Enforce **zero import statements** in user scripts at the validation step (SCRP-01's "validate" verb), consistent with D-14's single-file/no-local-imports decision — expose the generated GOLC SDK as global bindings (e.g. `golc.command(...)`) rather than an importable module, so there is no `import` statement for Deno's resolver to ever see, and no legitimate reason for one to exist. If ESM import ergonomics are wanted later, gate that behind an import map pinned to a local, pre-bundled file plus `--cached-only`, never a bare specifier that could resolve remotely.
**Warning signs:** Any generated SDK example or Monaco snippet that uses `import ... from "golc:sdk"` (or similar) rather than a global — a sign the zero-import boundary has been silently reopened.

## Code Examples

### Deno permission model (verified against current docs)
```
// Source: docs.deno.com/runtime/reference/permissions [CITED]
// Deno denies file, network, environment, and subprocess access unless
// explicitly granted via --allow-read/-write/-net/-env/-run/-ffi (and the
// corresponding --deny-* flags take precedence over --allow-* when both
// are present). GOLC's script host should grant NONE of these — the SDK
// boundary is stdio, not any Deno-gated resource.
deno run --no-prompt <script.ts>   // no --allow-* flags at all
```

### Debug-mode-only inspector launch
```
// Source: docs.deno.com/runtime/fundamentals/debugging [CITED]
// --inspect-brk waits for a debugger to connect before executing and
// breaks immediately on connect; --inspect starts execution immediately
// (a race against a short script). GOLC's Debug mode should use
// --inspect-brk specifically, so the daemon's CDP client is guaranteed
// to be attached before any script-authored code runs, and can set every
// UI-configured breakpoint before the first line executes.
deno run --inspect-brk=127.0.0.1:9229 <script.ts>
```

### Offline module policy (relevant only if ESM SDK imports are ever introduced)
```
// Source: docs.deno.com/runtime/packages [CITED], --cached-only confirmed
// current (not deprecated) in Deno 2.8+/2.9 documentation.
deno install --frozen --entrypoint main.ts
deno run --cached-only main.ts
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `--lock-write` for lockfile management | `--frozen` (with `--frozen=false` for the old write-through behavior) | Deprecated ahead of Deno 2 `[CITED: docs.deno.com/runtime/packages]` | Not directly load-bearing for this phase (scripts have no legitimate dependency graph), but relevant if the Deno toolchain's own bootstrap tooling ever needs a lockfile. |
| No minimum-dependency-age protection | `min-release-age` enabled by default (24h) as of Deno 2.9 | 2.9 (2026) `[CITED: docs.deno.com/runtime/packages/supply_chain, deno.com/blog/v2.9]` | Not load-bearing for script execution (zero-import design), but confirms Deno's own supply-chain posture has continued to harden — worth revisiting if D-14 is ever loosened to allow SDK imports. |
| `--no-remote` flag | `--cached-only` (module-resolution-time network denial) + `--frozen` (lockfile enforcement) | Consolidated in recent Deno 2.x releases | If ESM SDK imports are introduced later, `--cached-only` is the correct current flag to hard-deny any uncached module resolution — not the removed `--no-remote`. |

**Deprecated/outdated:**
- `--lock-write`: superseded by `--frozen=false`.
- `--no-remote` as a standalone concept: superseded by `--cached-only` for resolution-time network denial, paired with `--frozen` lockfile enforcement.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Exposing the GOLC SDK as global bindings (not an ES module import) is the right default authoring ergonomic for D-14/D-15, rather than an import-map-based local module. | Architecture Patterns (Pitfall 4), Generated SDK discussion | If the team prefers `import` ergonomics, the SDK generator and the zero-import validation rule both need to change together; low risk since both are internal generator choices, not a runtime/security boundary change. |
| A2 | Out-of-scope SDK calls (a script calling something its D-07 capability profile doesn't grant) should be treated with the same "immediate hard termination" severity as D-08's deadline/rate/resource overruns, rather than a soft per-call error the script could catch and continue past. | Anti-Patterns to Avoid | If wrong, a scope violation might instead surface as a typed runtime error the script can catch/retry — this is a real product-behavior choice CONTEXT.md did not explicitly settle for scope violations specifically (D-08 explicitly covers "deadline, rate limit, or resource limit," not capability-scope). Should be confirmed with the user during planning/discuss if not already implicit. |
| A3 | The Deno toolchain should be pinned in `config/toolchain.toml` following the exact go/node/mage per-platform archive+SHA-256 pattern, sourced from `github.com/denoland/deno/releases`. | Standard Stack, Installation | Low risk — this mirrors an established, working pattern exactly; the only real unknown is whether Deno ships a `.sha256sum` file per asset (confirmed via search) versus a single combined checksums file (Go/Node's pattern) — worth a final check against the actual release assets at implementation time. |
| A4 | A fresh OS subprocess per script *run* (not per script *definition*, and not a shared pool) is acceptable from a startup-latency standpoint for GOLC's use case (single-operator desktop automation, not a high-frequency serverless workload). | Architecture Patterns, Process/IPC Isolation Model | If script start latency turns out to matter more than assumed (e.g., very short, frequently-re-run scripts), a pre-warmed-but-still-zero-permission process pool could be revisited later without changing the security model — but this adds real complexity and was not requested by any locked decision. |

**If this table is empty:** N/A — see entries above.

## Open Questions

1. **Should concurrent script runs be supported in v1, and if so, how many?**
   - What we know: D-16's script library implies multiple saved scripts exist; D-10's "Stop is scoped to that one script" implies at least the *UI* is built assuming more than one script could be relevant at once. PLAY-08's Revoke Automation already speaks of "scripts" plural.
   - What's unclear: Whether SCRP-01 through SCRP-06 require *simultaneous* execution of more than one script, or whether v1 can serialize script runs (one active run at a time, queueing or rejecting a second Run request) while still supporting many *saved* scripts.
   - Recommendation: The subprocess-per-run architecture recommended here supports either answer without redesign (each run is already an independent process/Job Object/IPC session) — the planner should explicitly decide serialized-vs-concurrent as a plan-level scope call rather than defaulting to one silently, since it changes UI affordances (can Stop target "the" running script, or must it target a specific run id).

2. **Exact Deno release asset checksum format for Windows (`.sha256sum` per-asset vs. a combined `SHASUMS256.txt`-style file).**
   - What we know: Search results confirm each Deno release asset has a matching `.sha256sum` file (per-asset), unlike Node's single combined checksums file.
   - What's unclear: The exact filename pattern for the Windows asset's checksum file was not independently fetched and byte-verified in this research session (network fetch tools available in this environment did not retrieve the raw GitHub releases API/asset listing for a specific tag).
   - Recommendation: At implementation time, fetch `https://github.com/denoland/deno/releases/download/v2.9.4/deno-x86_64-pc-windows-msvc.zip.sha256sum` (or the equivalent for whatever exact version is pinned) directly and verify the returned hash before committing it into `config/toolchain.toml`, exactly as the existing Go/Node/Mage entries were presumably verified.

3. **Whether `internal/artnet/ipc`'s named-pipe transport should still be extended to expose script status/control to a *separate* Wails process that is not itself the daemon owning the script subprocess.**
   - What we know: In one topology (per `internal/wails/app.go`), Wails spawns and dials a separate `golc-project.exe artnet serve` daemon over a named pipe; in another, the daemon and the Wails-bound Go host are more tightly coupled.
   - What's unclear: This research assumed the Go process that owns `internal/command` (and therefore `internal/script`) is reachable by the Wails frontend either via direct Wails bindings (same process) or via the existing named-pipe/HTTP+SSE path (separate process) — but did not trace every current deployment topology in exhaustive detail.
   - Recommendation: The planner should confirm which topology Phase 8 targets (script execution always co-located with the daemon that owns `internal/command`, reachable by the Wails frontend through whatever channel already carries other commands) — this affects only *which* existing transport surfaces the new `internal/script` routes, not the Deno-subprocess isolation design itself, which is unaffected either way.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Deno CLI | Script execution sandbox (SCRP-01/02/03) | ✗ (not yet in `config/toolchain.toml` or `.tools/`) | — | None — this is new toolchain provisioning work Phase 8 must add, mirroring the existing Go/Node/Mage bootstrap pattern; no viable fallback runtime meets SCRP-03's permission-denial requirements as cleanly. |
| `golang.org/x/sys` (Windows Job Object syscalls) | Resource enforcement (SCRP-04) | ✓ | v0.46.0 (already in `go.mod`) | — |
| `golang.org/x/time` (rate limiting) | Rate limits (D-09) | ✓ | already in use | — |
| `github.com/mafredri/cdp` | Debugger bridge (D-01/D-02) | ✗ (not yet a `go.mod` dependency) | v0.35.0 latest | None with equivalent Go-native type safety; a hand-rolled CDP client is the only fallback and is not recommended (see Don't Hand-Roll). |
| `monaco-editor` (npm) | Script editor (D-15) | ✗ (not yet a frontend dependency) | 0.56.0 latest (flagged SUS, see audit) | None with equivalent in-browser TypeScript language-service quality. |
| Windows Job Objects (OS feature) | Resource enforcement | ✓ (built into Windows, the only qualified v1 platform) | N/A (OS API) | — |

**Missing dependencies with no fallback:**
- Deno CLI toolchain provisioning — must be added to `config/toolchain.toml` and `internal/bootstrap` as new Phase 8 work.
- `github.com/mafredri/cdp` — must be added as a new `go.mod` dependency.
- `monaco-editor` — must be added as a new frontend dependency, gated by the `checkpoint:human-verify` the SUS verdict requires.

**Missing dependencies with fallback:** None — every new dependency identified is load-bearing for a locked decision (D-01 real debugger, D-15 real TS language service, SCRP-03 sandboxed execution) with no acceptable substitute identified.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework (backend) | Go `testing` package, table-driven tests, `_test.go` suffix convention (existing repo-wide pattern, e.g. `internal/api/*_test.go`, `internal/bootstrap/*_test.go`) |
| Framework (frontend) | Vitest 4.x (already configured; `frontend/package.json`'s `"test": "vitest run"`, wired into the `"build"` script as a runtime-error smoke gate per project convention) |
| Config file | Go: none needed beyond `go.mod`/`mage Test`. Frontend: Vitest config lives inline in `vite.config.ts` (no separate `vitest.config.*` file found) |
| Quick run command | `go test ./internal/script/... ./internal/scriptsdk/... ./internal/show/... -run <TestName>` (backend); `npm --prefix frontend run test -- ScriptsWorkspace` (frontend) |
| Full suite command | `mage Test` (backend, existing project-wide target); `npm --prefix frontend run build` (frontend, includes `tsc --noEmit && vitest run && vite build`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SCRP-01 | Create/edit/validate/run/stop/debug lifecycle for a script | integration (Go) | `go test ./internal/script/... -run TestScriptLifecycle` | ❌ Wave 0 |
| SCRP-01 | Editor create/edit/validate UI round-trip | component (Vitest) | `npm --prefix frontend run test -- ScriptsWorkspace` | ❌ Wave 0 |
| SCRP-02 | Generated SDK is byte-stable and drift-checked | unit (Go) | `go test ./internal/scriptsdk/... -run TestGenerateDrift` | ❌ Wave 0 |
| SCRP-02 | A script cannot reach raw DMX/frame evaluation through the SDK | integration (Go) | `go test ./internal/script/... -run TestSDKNoRawDMXRoute` | ❌ Wave 0 |
| SCRP-03 | Zero Deno permission flags are ever passed for a script run | unit (Go) | `go test ./internal/script/... -run TestDenoCommandLineHasNoAllowFlags` | ❌ Wave 0 |
| SCRP-03 | A script attempting filesystem/network/env/subprocess access is denied at the OS/runtime level | integration (Go, spawns real pinned Deno) | `go test ./internal/script/... -run TestDenoDenialSurface` | ❌ Wave 0 |
| SCRP-04 | Capability/deadline/rate/resource assignment is saved as a per-script profile and pre-filled at run time | integration (Go) | `go test ./internal/show/... -run TestScriptProfilePersistence` | ❌ Wave 0 |
| SCRP-04 | Exceeding a deadline/rate/resource limit produces immediate hard termination with a logged reason (D-08) | integration (Go, Windows-only) | `go test ./internal/script/... -run TestLimitOverrunHardKill` | ❌ Wave 0 |
| SCRP-05 | Structured logs/diagnostics/source-mapped stack traces/command outcomes stream live | integration (Go) | `go test ./internal/script/... -run TestLiveLogStreaming` | ❌ Wave 0 |
| SCRP-06 | A runaway/crashed/blocked script can be terminated without affecting a concurrently running Art-Net frame loop | integration (Go, existing Art-Net test harness) | `go test ./internal/script/... -run TestScriptKillDoesNotBlockArtnet` | ❌ Wave 0 |
| D-01/D-02 | Debug-mode-only inspector: no CDP server exists during a plain Run | unit (Go) | `go test ./internal/script/... -run TestNoInspectorOutsideDebugMode` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** targeted `go test ./internal/script/...` (or the specific changed package) plus `npm --prefix frontend run test -- <changed workspace>` for frontend changes.
- **Per wave merge:** `mage Test` (full backend suite) and `npm --prefix frontend run build` (full frontend suite, including the runtime-error smoke gate).
- **Phase gate:** Full suite green before `/gsd-verify-work`, plus the manual Windows Job Object CPU/memory hard-cap and real-debugger-attach checks noted below (these are the parts least amenable to pure unit testing).

### Wave 0 Gaps
- [ ] `internal/script/` package does not exist yet — every listed test file is new.
- [ ] `internal/scriptsdk/` generator package does not exist yet.
- [ ] `internal/show/scripts.go` (Script entity + persistence) does not exist yet.
- [ ] `config/toolchain.toml` has no `[toolchain.deno]` entry yet — bootstrap tests need a pinned, checksum-verified Deno binary available in CI/test environments before any `TestDenoDenialSurface`-style integration test can run for real (vs. mocked).
- [ ] `frontend/src/workspaces/build/ScriptsWorkspace.tsx` and a Monaco integration do not exist yet; `monaco-editor` is not yet a frontend dependency (blocked on the `checkpoint:human-verify` from the Package Legitimacy Audit).
- [ ] No existing test harness exercises "kill a subprocess while the Art-Net frame loop is running and assert no frame miss" — likely needs a new fixture reusing whatever virtual-clock/deterministic-renderer test infrastructure earlier phases (per `.planning/research/SUMMARY.md`'s "Observability and testkit" component) already established; confirm exact reuse points during planning.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | No | Scripts do not authenticate as end users; they execute inside a trust boundary the daemon itself establishes and supervises (spawn-time, not credential-time trust). |
| V3 Session Management | No | No new session concept is introduced; the debug "session" is a process-lifetime CDP connection owned entirely by the trusted daemon, not a user-facing session. |
| V4 Access Control | Yes | Reuse Phase 7's `RequireScope` (`internal/api/auth.go`) for every SDK call a script makes, keyed off the script's D-06/D-07 capability profile — never a Deno permission flag. |
| V5 Input Validation | Yes | Script source validated pre-execution (SCRP-01 "validate": TypeScript compile/type-check via `deno check`, single-file/no-import structural check per D-14, D-15's Monaco-side live diagnostics as a first pass); every SDK call's arguments validated the same way Phase 7's Huma-driven request validation already works. |
| V6 Cryptography | No new requirement | No new secrets/credentials are introduced by this phase; the CDP/inspector channel is loopback-only and does not need TLS (matches API-05's existing loopback-first policy for the same reason). |

### Known Threat Patterns for This Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Sandbox escape via native code/FFI or unexpected subprocess spawn | Elevation of Privilege | Deno's deny-by-default `--allow-ffi`/`--allow-run` never granted; zero permission flags passed for any script run. |
| Capability/scope bypass (script calling an SDK function outside its granted domain scope) | Elevation of Privilege | Host-side `RequireScope` check on every SDK call arriving over stdio, independent of anything the script process itself claims; treat a violation with the same "immediate hard termination, logged" severity as D-08 (see Assumption A2). |
| Resource exhaustion / DoS against the daemon via a runaway or malicious script | Denial of Service | Windows Job Object CPU-rate hard cap + memory limit, wall-clock deadline (`context.WithDeadline`), and `x/time/rate` per-script rate limiting — all converging on immediate hard kill, no grace period (D-08). |
| Inspector/debug channel exposure enabling arbitrary code execution or state inspection | Information Disclosure / Elevation of Privilege | `--inspect-brk` only ever passed in explicit Debug launch mode (D-02); bound to loopback-only ephemeral port; Go daemon is the sole CDP client, never the browser. |
| Supply-chain injection via script `import` statements (remote or npm) | Tampering | Zero-import script validation (SCRP-01 "validate" step) — the generated SDK is exposed as global bindings, not an importable module, removing the legitimate import surface entirely; the Deno *toolchain* itself remains checksum-pinned offline exactly like Go/Node/Mage. |
| Sensitive data leakage through captured script stdout/stderr (e.g., an error message echoing a secret) | Information Disclosure | Reuse `internal/security.Redact` and `process.go`'s bounded-buffer capture pattern for every line of script output before it reaches the SSE stream or the audit log. |
| A stuck/blocked script's cleanup logic indefinitely delaying playback-relevant Go code | Denial of Service | Structural, not conventional: the script always lives in a separate OS process the daemon's Art-Net/playback goroutines never call into synchronously (mirrors ARTN-04's existing "never backpressured by scripts" guarantee); Stop/deadline/crash all converge on an unconditional Job-Object-close + process-tree kill the script cannot intercept or delay. |

## Sources

### Primary (HIGH confidence)
- `go.mod`, `internal/api/ratelimit.go`, `internal/api/keys.go`, `internal/api/events.go`, `internal/contracts/generate.go`, `internal/trace/transport/process.go`, `internal/wails/app.go`, `internal/show/apikeys.go`, `internal/show/state.go`, `config/toolchain.toml`, `frontend/src/shell/navigation.ts`, `frontend/package.json` — all read directly from the repository this session. `[VERIFIED: local codebase]`
- `proxy.golang.org` module-version lookups for `golang.org/x/sys`, `github.com/invopop/jsonschema`, `github.com/mafredri/cdp`, `github.com/aoldershaw/proclimit`, `github.com/tprasadtp/go-autotune`. `[VERIFIED: proxy.golang.org]`
- GitHub API repository metadata for `mafredri/cdp` (stars, last push, archived status). `[VERIFIED: api.github.com]`
- `npm view monaco-editor version` / package-legitimacy seam output (publish date, weekly downloads, source repo). `[VERIFIED: npm registry via gsd-tools package-legitimacy seam]`

### Secondary (MEDIUM confidence)
- `docs.deno.com/runtime/reference/permissions` — Deno permission flags and deny-by-default model. `[CITED]`
- `docs.deno.com/runtime/fundamentals/debugging` — `--inspect`/`--inspect-brk` behavior. `[CITED]`
- `docs.deno.com/runtime/fundamentals/workers` — Worker permission inheritance constraints. `[CITED]`
- `docs.deno.com/runtime/packages` and `docs.deno.com/runtime/packages/supply_chain` — `--cached-only`, `--frozen`, minimum-dependency-age, vendoring. `[CITED]`
- `github.com/denoland/deno/releases` — current stable version and release cadence (2.9.4, July 23 2026). `[CITED]`
- `learn.microsoft.com/en-us/windows/win32/procthread/job-objects` — Job Object mechanics, basic/extended limits, kill-on-close. `[CITED]`
- GitHub issues `#18935`, `#26202`, `#30043` on `denoland/deno` — documented limitations of V8-flag-based memory limiting for true OS-level enforcement. `[CITED]`
- Monaco Editor TypeDoc (`microsoft.github.io/monaco-editor`) — glyph margin / `IModelDecorationOptions` breakpoint-gutter mechanism. `[CITED]`

### Tertiary (LOW confidence)
- None retained as load-bearing — every claim above was either verified against the local codebase/registries or cited to an official documentation source located this session. Where a specific implementation detail could not be independently confirmed (e.g., the exact Deno release checksum filename for a pinned version), it is called out explicitly in Open Questions rather than stated as fact.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — Deno's continued selection is grounded in the project's own prior research plus this session's re-verification against current official docs; all Go module versions verified live against `proxy.golang.org`/GitHub.
- Architecture (process/IPC/Job Object/debugger model): MEDIUM-HIGH — the subprocess-per-run + Job Object + host-mediated-CDP design follows directly from verified Deno/Windows facts and existing repo precedent (`process.go`, `internal/api`), but the exact generalized stdio protocol shape and Job Object tuning values are implementation-time decisions, not yet built or measured.
- Pitfalls: MEDIUM — grounded in documented Deno/Windows behavior (permission-vs-module-resolution ordering, CPU rate control modes, V8-flag memory-limit unreliability), not yet observed against a real GOLC implementation.

**Research date:** 2026-07-25
**Valid until:** 2026-08-22 (30 days) — Deno ships on a 12-week minor cadence but patch releases (observed weekly in this research: 2.9.1 → 2.9.4 within one month) are frequent; re-verify the exact pinned Deno version and its checksum immediately before implementation regardless of this window.
