<!-- GSD:project-start source:PROJECT.md -->

## Project

**GOLC**

GOLC is a modern lighting-control application for operators of small live shows, built in Go with a Wails desktop interface and a cross-platform architecture. Its first supported release targets Windows. It combines a fast, modular show-authoring workflow with TypeScript scripting, autonomous LLM control, and a well-documented API so people, scripts, external programs, and AI agents can all create and operate fixture patches, scenes, chases, and show playback through the same system.

The first release will output Art-Net and support complete show authoring and playback. Additional lighting protocols and larger-scale console capabilities can be added after the core workflow and extension model are proven.

**Core Value:** An operator can author a modular show once, adapt its fixture pools to different deployments in one or two actions, and hand a simple controller surface to another person for reliable playback.

### Constraints

- **Application stack**: Go with Wails — required by the chosen cross-platform desktop architecture.
- **Initial platform**: Windows only for v1 qualification and support — other desktop platforms are deferred even though portability remains an architectural goal.
- **Scripting**: TypeScript — required for user-authored automation and extensibility.
- **Fixture source format**: Use a strict YAML 1.2 subset with schema validation, duplicate-key rejection, explicit schema versioning, and deterministic normalization — fixture files must remain approachable without introducing ambiguous runtime behavior.
- **Fixture ecosystem**: Support Open Fixture Library import plus first-class custom definitions — imported definitions must pass through GOLC's canonical validation and pinning pipeline.
- **Initial protocol**: Art-Net — all other lighting-output protocols are deferred beyond v1.
- **AI portability**: Use an open-source provider-neutral wrapper that supports common hosted providers and local models — users must not be locked to one LLM vendor.
- **Live reliability**: DMX/Art-Net output and playback timing cannot depend on UI rendering, network-bound LLM inference, or script responsiveness — show output must remain deterministic under load or component failure.
- **Control consistency**: UI, scripts, API calls, and LLM operations must converge on shared domain commands and state — otherwise automation and interoperation will become incomplete or unsafe.
- **Autonomy safety**: Autonomous AI control must remain observable and interruptible by the operator — live lighting changes need a dependable human override even when confirmation is not required for each action.
- **Project tracking**: Use Linear from the start — requirements, roadmap phases, and implementation issues need explicit repository-to-Linear traceability.
- **Developer experience**: Centralize project configuration behind one documented root entrypoint with logically separated subconfiguration — contributors and automation should not need to discover scattered sources of truth.
- **MIDI hardware**: Akai MIDImix, Novation Launch Control XL Mk2, and Worlde EasyControl 9 are the selected Phase 6 physical acceptance set; selection does not establish support. Each device must independently pass MIDI-HW-02 for its exact hardware revision, firmware, Windows version, and GOLC build before a named compatibility or support claim, and device-specific profiles or feedback remain v1.x work under EXTN-04.
- **Safe structural edits**: Pool resizing and fixture substitution default to previewing a deterministic impact plan before commit — modular reuse must not silently corrupt or reinterpret show programming.
- **Musical timing**: Tempo-aware scenes derive timing from a global BPM and explicit bar structure — scene playback must remain deterministic and independent of UI, script, or LLM latency.
- **Automation override**: Revoke Automation must not depend on an AI provider, script runtime, or queued application command completing — it is a local priority control path distinct from blackout.

<!-- GSD:project-end -->

<!-- GSD:stack-start source:research/STACK.md -->

## Technology Stack

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go | **1.26.5** | Domain model, command validation, playback scheduler, networking, persistence | Current stable patch on 2026-07-17. Goroutines, monotonic `time.Time`, `net.UDPConn`, cancellation, and the race detector fit a deterministic, observable playback engine. Pin `go 1.26.5` in CI/tooling. [Official downloads](https://go.dev/dl/) **[HIGH]** |
| Wails | **v2.13.0** | Windows/macOS/Linux desktop shell and Go-to-TypeScript bridge | v2 is the production line; v3 is still explicitly alpha. v2 embeds the compiled frontend, uses native WebViews, and generates TS definitions for bound Go types. [v2.13.0 release](https://github.com/wailsapp/wails/releases/tag/v2.13.0), [version status](https://github.com/wailsapp/wails#readme) **[HIGH]** |
| Node.js | **24.18.0 LTS** | Frontend build/test toolchain only | Use the active LTS line, never ship Node in the application. Vite 8 accepts Node 22.12+; Node 24 is the current production LTS while 26 is Current. [Official release table](https://nodejs.org/en/about/previous-releases) **[HIGH]** |
| React / React DOM | **19.2.7** | Dense programming/playback UI | The broad component/testing ecosystem and external-store support are a better fit than inventing a custom widget stack. React is a projection of Go state, not the show-state authority. [Official npm package](https://www.npmjs.com/package/react) **[HIGH]** |
| Vite | **8.1.4** | Wails frontend dev server and production asset build | Current stable Vite, fast HMR, modern TS/worker support, and static output that Wails can embed. Do not use Wails v2 dynamic `AssetsHandler` tricks; compile static assets. [Official npm package](https://www.npmjs.com/package/vite), [Wails asset guidance](https://wails.io/docs/guides/application-development/) **[HIGH]** |
| TypeScript | **7.0.2** | Application frontend and user-script authoring surface | Current stable compiler/tooling release. Generate the GOLC scripting declarations from the same API/domain schemas used by other clients. [Official npm package](https://www.npmjs.com/package/typescript) **[HIGH]** |
| Deno sidecar | **2.9.3** | Type-check and execute user TypeScript outside the lighting process | Deno runs TS directly and denies filesystem, network, environment, FFI, and subprocess access by default. Run scripts in a separate, killable `golc-script-host` process with `--no-prompt`, frozen/cached-only dependencies, a reduced-permission Worker, hard deadlines, and JSON-RPC over stdio. [Latest release marker](https://dl.deno.land/release-latest.txt), [security model](https://docs.deno.com/runtime/fundamentals/security/), [`deno check`](https://docs.deno.com/runtime/reference/cli/check/) **[HIGH]** |
| SQLite through `modernc.org/sqlite` | **driver v1.54.0 / SQLite 3.53.2** | Portable show file, fixture data, command audit, recovery | CGo-free `database/sql` driver with current Windows/macOS/Linux targets. SQLite gives crash-safe transactions without a service. Use one `.golc` SQLite database as the canonical portable format. [Driver docs](https://pkg.go.dev/modernc.org/sqlite), [SQLite transaction guarantees](https://www.sqlite.org/transactional.html) **[HIGH]** |
| Art-Net | **Art-Net 4 Protocol Release 1.4, document revision 1.4 (2025-10-23)** | v1 network lighting output | Implement the narrow v1 codec (`ArtDmx`, `ArtPoll`, `ArtPollReply`, sequence, addressing) over Go `net.UDPConn`; no current Go library has enough adoption/completeness to justify owning the critical output path. Art-Net uses UDP port `0x1936`. Test byte-for-byte against official vectors/pcaps. [Official specification](https://art-net.org.uk/downloads/art-net.pdf), [official port note](https://art-net.org.uk/port/) **[MEDIUM]** |
| Huma + Chi | **Huma v2.39.0 / Chi v5.3.1** | Versioned public JSON command/query API and SSE event feed | Go structs drive validation and OpenAPI 3.1 generation; Huma has response streaming/SSE support and Chi is a small standard-HTTP router. Both adapters call the same Go command bus as Wails. [Huma docs](https://huma.rocks/), [Huma package](https://pkg.go.dev/github.com/danielgtaylor/huma/v2), [Chi package](https://pkg.go.dev/github.com/go-chi/chi/v5) **[HIGH]** |
| Bifrost Core | **v1.7.2** | Provider-neutral hosted/local LLM client | Current Apache-2.0 Go core supports hosted providers and OpenAI-compatible/local providers such as Ollama, with streaming, tools, structured output, routing, and cancellation. Embed only the Go core behind GOLC's small `ModelClient` interface; do not ship its gateway UI. [Official repository](https://github.com/maximhq/bifrost), [Go package](https://pkg.go.dev/github.com/maximhq/bifrost/core) **[MEDIUM]** |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `zustand` | **5.0.14** | UI projection/cache | Store latest Go snapshot and editor-local state. Use selectors and `subscribe` for transient meter updates; never put authoritative cues or playback timing here. [Official package](https://www.npmjs.com/package/zustand) **[HIGH]** |
| Radix UI Primitives | **exact per-package pins; e.g. `@radix-ui/react-dialog` 1.1.19** | Accessible dialogs, popovers, menus, sliders, tabs | Use unstyled primitives under a GOLC design system. Avoid a theme framework that dictates console density. [Official package](https://www.npmjs.com/package/@radix-ui/react-dialog) **[HIGH]** |
| Monaco Editor | **0.55.1** | Script editor, diagnostics, completion, source locations | Load only in the scripting workspace. Use ESM workers through Vite and inject generated `golc.d.ts`; dispose models on close. AMD integration is deprecated. [Official package](https://www.npmjs.com/package/monaco-editor), [official Vite sample](https://github.com/microsoft/monaco-editor/tree/main/samples/browser-esm-vite-react) **[HIGH]** |
| `github.com/pressly/goose/v3` | **v3.27.2** | Embedded SQLite schema migrations | Embed forward migrations with `go:embed`; open older shows through explicit, tested migrations and preserve an untouched backup before upgrading. [Official package](https://pkg.go.dev/github.com/pressly/goose/v3) **[HIGH]** |
| `github.com/google/uuid` | **v1.6.0** | Stable show/entity/command IDs | Generate UUIDv7 IDs for fixtures, groups, scenes, chases, scripts, commands, and events. IDs must survive rename/reorder and become API idempotency/audit keys. [Official package](https://pkg.go.dev/github.com/google/uuid) **[HIGH]** |
| `golang.org/x/net` | **v0.57.0** | Network-interface and IPv4 control helpers | Use only where the standard UDP API does not expose required interface/control-message behavior; keep Art-Net packet encoding in an internal package. [Official package](https://pkg.go.dev/golang.org/x/net) **[HIGH]** |
| `golang.org/x/sync` | **v0.22.0** | Bounded worker groups and cancellation | Use outside the frame loop for discovery, persistence, API, and AI work. Prefer fixed queues and backpressure to unbounded goroutine creation. [Official package](https://pkg.go.dev/golang.org/x/sync) **[HIGH]** |
| `openapi-typescript` / `openapi-fetch` | **7.13.0 / 0.17.0** | Generated frontend/external TS API types and small typed client | Generate from the checked-in OpenAPI snapshot and fail CI on drift. The Wails adapter may reuse the generated DTO types without routing local UI traffic through HTTP. [Generator](https://www.npmjs.com/package/openapi-typescript), [client](https://www.npmjs.com/package/openapi-fetch) **[HIGH]** |
| AsyncAPI / `@asyncapi/cli` | **spec 3.1.0 / CLI 6.0.2** | Contract for SSE event names/payloads/replay semantics | Keep `api/asyncapi.yaml` beside OpenAPI, reference the same JSON schemas, and validate it in CI. It is documentation/contract tooling, not an in-process broker. [Specification](https://github.com/asyncapi/spec), [CLI](https://www.npmjs.com/package/@asyncapi/cli) **[HIGH]** |
| `log/slog` | **Go 1.26 standard library** | Structured operational logs | JSON logs in releases, human handler in development. Add `show_id`, `command_id`, `source`, `universe`, `frame_seq`, and latency fields. Keep the immutable command audit in SQLite; logs are diagnostics, not audit truth. **[HIGH]** |

### Development and Delivery Tools

| Tool | Version | Purpose | Notes |
|------|---------|---------|-------|
| Go `testing`, race detector, fuzzing | Go **1.26.5** | Domain, scheduler, packet, migration, and concurrency tests | Use an injected clock and deterministic step runner. Run `go test -race ./...`; fuzz every packet decoder and public command decoder. |
| `pgregory.net/rapid` | **v1.3.0** | Property/state-machine tests | Assert intensity ranges, merge rules, cue transitions, UUID stability, and save/load round trips across generated shows. [Package](https://pkg.go.dev/pgregory.net/rapid) |
| `testify` | **v1.11.1** | Concise assertions only | Use `require`; do not adopt suite-heavy abstractions that hide concurrency cleanup. [Package](https://pkg.go.dev/github.com/stretchr/testify) |
| Vitest | **4.1.10** | TS unit and browser component tests | Use Browser Mode for interaction-sensitive controls and fake only the typed Go adapter. [Official package](https://www.npmjs.com/package/vitest) |
| Playwright | **1.61.1** | Browser E2E and Windows WebView2 smoke tests | Test most flows against the built frontend/API in normal browsers. Add a smaller Windows-native suite by starting Wails with WebView2 remote debugging and connecting over CDP. [Official package](https://www.npmjs.com/package/@playwright/test), [WebView2 guidance](https://playwright.dev/docs/webview2) |
| Wails CLI | **v2.13.0** | Desktop build/package | Pin the CLI, do not install `@latest` in CI. Windows: `wails build -nsis`; include WebView2 bootstrap strategy and sign both EXE and installer. [NSIS guide](https://wails.io/docs/guides/windows-installer/), [WebView2 guidance](https://wails.io/docs/guides/windows/) |
| Platform signing tools | OS-supplied/current CI image | Trusted distribution | Build/sign on each target OS: Windows SignTool + timestamping, Apple `codesign`/`notarytool`, Linux AppImage plus DEB. Treat Linux WebKitGTK baselines as a packaging-phase test matrix, not a promise inferred from a cross-compile. [Wails signing guide](https://wails.io/docs/guides/signing) |
| Linear TypeScript SDK | **`@linear/sdk` 88.1.0** | Day-one planning/delivery synchronization | Workflow tool only; keep it under `tools/linear-sync`, not the GOLC runtime. Use Node 24, exact lockfile, GraphQL pagination, and `LINEAR_API_KEY` from CI secrets. [Official SDK package](https://www.npmjs.com/package/@linear/sdk), [official GraphQL docs](https://linear.app/developers/graphql) **[HIGH]** |

## Persistent Show Format

## Command, Event, and API Contracts

- Define immutable Go command envelopes: `id`, `type`, `schema_version`, `source`, `expected_revision`, `requested_at`, payload, and optional scheduling metadata.
- The command handler validates authorization, revision, safety limits, and domain invariants, commits state/audit transactionally, then publishes a typed event. UI, scripts, HTTP, and LLM tools all call this handler.
- Wails binds a narrow typed adapter for local UI calls. Huma exposes `/api/v1` JSON commands/queries and `/api/v1/events` SSE for external clients. SSE is enough for a one-way state/event stream and is easier to observe/replay than a WebSocket protocol in v1.
- Generate OpenAPI 3.1 from Huma/Go types; generate TS with `openapi-typescript`. Describe event names, payload schemas, ordering, `Last-Event-ID`, heartbeats, and disconnect recovery in AsyncAPI 3.1. CI compares generated artifacts and runs compatibility tests.
- Do not put frame-level DMX values on the public event bus. Publish throttled snapshots/telemetry; the frame loop writes Art-Net independently.

## TypeScript Runtime Boundary

## LLM Boundary

## Linear from Day One

| GSD artifact | Linear object | Stable linkage |
|--------------|---------------|----------------|
| Milestone/release (for example v1) | Project | Store GraphQL UUID as `linear_project_id`; optionally group multiple releases under an Initiative later |
| `ROADMAP.md` phase | Project milestone | Store UUID as `linear_project_milestone_id`; phase number and title are metadata, not identity |
| Phase `PLAN.md` / feature slice | Parent issue | Link to the phase milestone and requirements delivered |
| `REQ-*` requirement | Issue labeled `requirement` | Store immutable issue UUID plus human key such as `GOLC-123` |
| Executable plan task | Issue or sub-issue | Store immutable issue UUID; link to its plan parent and requirement, and use dependencies/relations for ordering |

## Installation

# Go core (exact versions recorded in go.mod/go.sum)

# Frontend runtime (save exact versions; commit lockfile)

# Frontend/contract/test tooling

# Linear sync tool only, isolated under tools/linear-sync

# Download Deno v2.9.3 from the official release CDN in packaging jobs,

# verify the published checksum, and bundle the target-specific binary.

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| Wails v2.13.0 | Wails v3 alpha | Re-evaluate after v3 reaches stable and has proven packaging/migration support; do not start a live-control v1 on an alpha ABI. |
| React 19 + Zustand | Svelte 5 | Reasonable for a smaller team already expert in Svelte, but React currently offers the safer editor/testing/accessibility ecosystem for this dense application. |
| SQLite show file | Versioned JSON bundle | Use JSON only for import/export, fixtures, examples, and source-control-friendly interchange. It is not enough for atomic multi-entity edits, audit queries, and migrations. |
| Deno sidecar | Sobek/goja in-process | Consider only for trusted macros with a deliberately small ES feature set; it is not a security boundary and process failure would share fate with playback. |
| Deno sidecar | QuickJS Go | Revisit after the binding declares production readiness and its cross-platform C toolchain is proven; its own README currently warns against production use. [Official warning](https://github.com/buke/quickjs-go) |
| Huma/OpenAPI + SSE/AsyncAPI | Protobuf/Connect/gRPC | Add for a later high-throughput remote-control ecosystem with generated clients in many languages. JSON/OpenAPI is easier for operators, scripts, and LLM tools in v1. |
| Bifrost Core behind a GOLC interface | LangChainGo | Use LangChainGo if GOLC later needs its chain/vector-store ecosystem. Do not use a general agent executor to bypass GOLC command validation. |
| Internal Art-Net codec | `go-artnet`/small community libraries | A library can be used as a test oracle, never as the only evidence of protocol correctness. Reconsider if a well-maintained, conformance-tested implementation emerges. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| Wails v3 alpha/nightly | Prerelease API and packaging churn are unacceptable for a cross-platform live controller. | Wails v2.13.0 |
| UI `setInterval`, animation frames, or JS state as playback clock | WebView stalls, background throttling, and rendering load would alter output timing. | Go monotonic scheduler and immutable frame snapshots |
| Direct script/LLM access to DMX buffers, UDP sockets, or database | Bypasses validation, audit, rate limiting, and manual override. | Typed commands through the Go command bus |
| In-process execution of untrusted JS/TS | Panics, runaway CPU/memory, and engine bugs share fate with playback; language sandboxes are not OS isolation. | Restricted, killable Deno child process |
| Node `vm` as a sandbox | Node documents it as a context mechanism, not a security boundary, and it would require shipping Node. | Deno permissions plus process isolation |
| Arbitrary npm/URL imports in show scripts | Supply-chain and reproducibility risk; shows may fail offline. | Generated `golc` module plus curated, locked imports |
| ORM for show-state mutations | Hides transaction boundaries and schema/query costs that matter for migrations and audit. | `database/sql`, explicit SQL, goose migrations |
| JSON files as authoritative persistence | Multi-file atomicity, migrations, referential integrity, recovery, and concurrent inspection become custom infrastructure. | Single SQLite `.golc` database plus JSON export |
| Redis/PostgreSQL/message broker in v1 | Adds deployment services without solving a single-operator desktop need. | In-process queues + SQLite + SSE |
| WebSocket-first public API | Creates a second bidirectional command protocol and harder reconnect semantics before it is needed. | HTTP JSON commands plus replayable SSE events |
| Full Bifrost gateway/admin UI or LangChain-style autonomous executor | Excess surface area and hidden agent loops; neither may become a second authority over the show. | Bifrost Core as provider adapter + GOLC-owned orchestration |
| Community Linear CLI or title-based sync | Titles/phase numbers change and duplicate; community tools add another unpinned interpretation layer. | Official `@linear/sdk`, GraphQL UUID map, explicit sync |
| Floating `latest`, uncommitted lockfiles, or cross-platform build from one host only | Produces non-reproducible installers and misses native WebView/signing problems. | Exact pins, checksums, lockfiles, native OS CI jobs |

## Stack Patterns by Variant

- Run Go playback and Art-Net continuously even if the WebView reloads, Deno exits, the API client disconnects, or an LLM times out.
- Send UI snapshots at a throttled rate; retain full-rate frames only in the Go output pipeline and diagnostic counters.
- Start the same Go application services without Wails, use an injected clock and loopback UDP receiver, and exercise Huma/OpenAPI contracts.
- This is a test/build variant, not a separate implementation.
- Deno uses only bundled/locked code; Bifrost local providers are optional; hosted AI failure never blocks authoring or playback.
- Linear and package registries are developer tooling only and are absent from runtime startup.

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| Wails v2.13.0 | Go 1.25+; recommend Go 1.26.5 | v2.13.0 module metadata declares Go 1.25. Build/test each native OS. |
| Vite 8.1.4 | Node 20.19+ or 22.12+; recommend Node 24.18 LTS | Vite is build-time only. Static `dist` is embedded by Wails. |
| React 19.2.7 | Zustand 5.0.14 | Zustand uses React external-store semantics; use selectors/transient subscriptions for fast meters. |
| modernc SQLite v1.54.0 | Go 1.25+; SQLite 3.53.2 | Keep the exact `modernc.org/libc` version selected by the driver's `go.mod`; do not override it independently. |
| goose v3.27.2 | Go 1.25.7+; `database/sql` SQLite | Embed migrations; migration tests must open copies of every supported historical fixture. |
| Huma v2.39.0 | Go 1.25+; Chi v5.3.1 | OpenAPI 3.1 is authoritative; also emit 3.0 only if a client tool requires it. |
| Vitest 4.1.10 | Vite 6.4–8; Node 22.12+ | Use Node 24 LTS. Browser Mode is preferred for complex controls. |
| Deno 2.9.3 | Standalone sidecar per OS/arch | Never let user code inherit host permissions; use `--no-prompt`, locked/cached-only dependency policy, and parent-enforced deadlines. |
| Bifrost Core v1.7.2 | Go 1.26.4+ | Aligns with Go 1.26.5. Keep an internal adapter because provider feature parity changes faster than domain code. |
| `@linear/sdk` 88.1.0 | Node 18+; recommend Node 24 LTS | Tooling workspace only. Pin generated SDK changes and review sync diffs before mutation. |

## Sources

- `/websites/v3_wails_io`, `/websites/wails_io`, `/wailsapp/wails` — v2/v3 status, bindings, events, build model
- `/react/react/v19.2.7`, `/vitejs/vite`, `/pmndrs/zustand`, `/microsoft/monaco-editor` — frontend versions and external-store/worker patterns
- `/websites/pkg_go_dev_modernc_org_sqlite`, `/pressly/goose` — pure-Go SQLite targets and embedded migrations
- `/evanw/esbuild`, `/tmc/langchaingo` — evaluated embedded TS/LLM alternatives
- `/danielgtaylor/huma`, `/websites/openapi-ts_dev`, `/asyncapi/spec` — OpenAPI generation, streaming, generated TS, AsyncAPI 3.1
- `/websites/linear_app_developers`, `/linear/linear` — official GraphQL/SDK approach

<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->

## Conventions

Conventions not yet established. Will populate as patterns emerge during development.
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->

## Architecture

Architecture not yet mapped. Follow existing patterns found in the codebase.
<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->

## Project Skills

No project skills found. Add skills to any of: `.claude/skills/`, `.agents/skills/`, `.cursor/skills/`, `.github/skills/`, or `.codex/skills/` with a `SKILL.md` index file.
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->

## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:

- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->

<!-- GSD:profile-start -->

## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
