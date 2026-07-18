# Project Research Summary

**Project:** GOLC
**Domain:** Cross-platform, Art-Net-first lighting control for small live shows with TypeScript, public API, and bounded autonomous LLM control
**Researched:** 2026-07-17
**Confidence:** MEDIUM

## Executive Summary

GOLC is a safety- and timing-sensitive desktop control system, not primarily a frontend application. The expert approach is a headless, deterministic Go core wrapped by adapters: one owner advances playback state from monotonic time, a pure renderer produces complete universe frames from immutable compiled show plans, and an independent Art-Net worker performs network I/O. Wails, TypeScript, the public API, LLM tools, and Linear are outside that path. **Deterministic playback and Art-Net output must continue without the UI, scripts, API clients, model providers, or Linear.** This is an architectural invariant, not a later optimization. The recommendation is supported by official Go timing behavior, Wails' adapter/lifecycle model, and the Art-Net protocol, while the exact concurrency design remains an architectural inference to prove with load and hardware tests ([Go `time`](https://pkg.go.dev/time), [Wails architecture](https://wails.io/docs/howdoesitwork), [Art-Net 4](https://art-net.org.uk/downloads/art-net.pdf)). **Confidence: MEDIUM.**

Build the conventional small-show workflow first: versioned fixtures and patching, a semantic programmer, groups and palettes, scenes, lightweight cue lists, chases, timing, deterministic HTP/LTP arbitration, structured playback, masters/blackout, durable shows, and observable Art-Net. Then expose the already-proven command model through Wails, a versioned HTTP/SSE API, a process-isolated TypeScript host, and finally bounded LLM autonomy. Use Go 1.26.5, Wails v2.13.0, React/Vite/TypeScript, SQLite, a restricted Deno sidecar, Huma/Chi, and Bifrost Core behind GOLC-owned interfaces. Exact versions were checked against official package/runtime sources; the system composition is a research recommendation ([Go releases](https://go.dev/dl/), [Wails v2.13.0](https://github.com/wailsapp/wails/releases/tag/v2.13.0), [Deno security](https://docs.deno.com/runtime/fundamentals/security/), [SQLite transactions](https://www.sqlite.org/transactional.html)). **Confidence: MEDIUM-HIGH for version facts; MEDIUM for the integrated stack.**

The largest risks are accidental coupling of output to slow adapters, incomplete Art-Net behavior, ambiguous fixture semantics, undefined source arbitration, unsafe persistence, and automation that bypasses validation or manual override. Mitigate them with a shared typed command kernel, mutable authoring compiled into immutable playback plans, bounded/latest-value mailboxes, a dedicated highest-priority local safety lane, strict fixture validation and definition pinning, SQLite-aware backup/migrations, and virtual plus physical receiver tests. Linear is required from inception for delivery traceability, but repository artifacts remain complete offline and synchronize by stable UUID mapping through separate developer tooling; it is never imported into the shipped runtime ([Linear GraphQL](https://linear.app/developers/graphql), [Linear rate limits](https://linear.app/developers/rate-limiting)). **Confidence: MEDIUM.**

## Reconciled Decisions

The four research streams agree on the boundaries but differ on several implementation defaults. Use the following decisions for roadmap creation:

| Decision | Selected default | Reconciliation and status |
|---|---|---|
| TypeScript execution | **Pinned Deno sidecar in a supervised helper process** | Architecture explored esbuild + Goja; stack research found Deno's deny-by-default permissions and direct TypeScript execution a stronger v1 failure boundary. Retain architecture's capability-only IPC, quotas, deadlines, kill/restart, and no renderer access. Open: OS-level resource containment and debugger scope. **MEDIUM.** |
| Public event transport | **REST/JSON commands and queries plus SSE events** | Architecture allowed SSE or WebSocket; stack selected Huma + Chi with OpenAPI 3.1 and SSE/AsyncAPI. WebSockets remain deferred until bidirectional streaming is demonstrated. **HIGH for framework capability; MEDIUM for product fit.** |
| LLM provider layer | **Bifrost Core behind a small GOLC-owned `ModelClient`** | Pitfall research used LangChainGo/Ollama to illustrate risks, not as a final dependency. Bifrost better matches the provider-neutral hosted/local requirement today, but its maturity and provider parity require a phase gate. **MEDIUM.** |
| Linear hierarchy | **One Linear Project for each active release/milestone; roadmap phases as Project Milestones; requirements as issues; tasks as issues/sub-issues** | Stack proposed an Initiative above release Projects, while feature/architecture research used Project -> Project Milestones. Start with the simpler hierarchy; add an Initiative only when multiple release Projects need portfolio grouping. **MEDIUM.** |
| Linear mapping file | **`.planning/linear-map.json`**, credential-free | Research differed only in path casing. Use one lowercase canonical file keyed by stable repository IDs and immutable Linear UUIDs. Authentication material and caches remain outside version control. **HIGH for stable-ID rule; LOW-MEDIUM for filename convention.** |
| Show persistence | **One portable `.golc` SQLite database plus versioned JSON export** | All streams favor SQLite for atomicity, migration, audit, and recovery; JSON is interchange, not authority. **HIGH.** |
| Art-Net implementation | **Narrow internal Art-Net 4 codec and transport** | No current Go library was judged sufficiently proven for the critical path. Community implementations may serve as test oracles only. **MEDIUM until hardware conformance testing.** |
| Desktop runtime | **Wails v2.13.0, not v3 alpha** | v2 is the production line; v3 remains prerelease in the research snapshot. Revisit only after v3 is stable and packaging is proven. **HIGH.** |

## Key Findings

### Recommended Stack

The Go core owns domain rules, command validation, persistence coordination, scheduling, frame rendering, and Art-Net. The frontend is a projection of revisioned Go snapshots; Node is build tooling only. Scripts and model calls run outside the live path, and Linear is development governance only. See [STACK.md](./STACK.md) for exact compatibility notes and install pins.

**Core technologies:**

- **Go 1.26.5:** domain, application kernel, deterministic scheduler, networking, and tests. Use monotonic target-time calculations, race tests, fuzzing, and an injected clock ([official downloads](https://go.dev/dl/)). **HIGH fact confidence.**
- **Wails v2.13.0:** cross-platform desktop shell and narrow generated Go/TypeScript bridge. Never start or drive playback from DOM lifecycle or frontend timers ([release](https://github.com/wailsapp/wails/releases/tag/v2.13.0), [bindings](https://wails.io/docs/howdoesitwork)). **HIGH fact confidence.**
- **React 19.2.7 + Zustand 5.0.14 + Radix UI:** dense operator UI, revisioned snapshot cache, and accessible primitives. Frontend state is non-authoritative. **HIGH version confidence; MEDIUM design confidence.**
- **Vite 8.1.4 + TypeScript 7.0.2 on Node 24.18 LTS:** build, contracts, and editor tooling; ship static embedded assets, not Node ([Node releases](https://nodejs.org/en/about/previous-releases), [Vite](https://www.npmjs.com/package/vite)). **HIGH.**
- **Deno 2.9.3 sidecar:** type-check and execute user TypeScript with no ambient filesystem, network, environment, FFI, or subprocess permission; supervise it as a killable helper ([Deno security model](https://docs.deno.com/runtime/fundamentals/security/)). **HIGH for runtime features; MEDIUM for final sandbox design.**
- **SQLite 3.53.2 through `modernc.org/sqlite` v1.54.0 + goose v3.27.2:** portable show storage, transactional command/audit data, migration, autosave, and recovery. Use Online Backup or `VACUUM INTO`; never copy a live WAL database ([backup API](https://www.sqlite.org/backup.html), [WAL](https://sqlite.org/wal.html)). **HIGH.**
- **Internal Art-Net 4 transport:** implement ArtDmx, ArtPoll/ArtPollReply, per-universe sequence, exact addressing and payload rules, subscriber unicast, and optional tested ArtSync over Go UDP. A 40 Hz default stays below the DMX512 gateway ceiling of 44 Hz; actual limits come from discovered/static targets ([official specification](https://art-net.org.uk/downloads/art-net.pdf)). **MEDIUM until device tests.**
- **Huma v2.39.0 + Chi v5.3.1:** `/api/v1` JSON commands/queries, OpenAPI 3.1, loopback-first policy, and replayable SSE events; generate TypeScript clients and describe events with AsyncAPI ([Huma](https://huma.rocks/), [OpenAPI](https://spec.openapis.org/oas/)). **HIGH for capabilities; MEDIUM for final contract.**
- **Bifrost Core v1.7.2:** provider-neutral hosted/local model adapter with tools, structured output, streaming, and cancellation. GOLC owns orchestration, policies, tools, and audit ([official repository](https://github.com/maximhq/bifrost)). **MEDIUM.**
- **Official Linear TypeScript SDK in `tools/linear-sync`:** explicit, idempotent repository-to-Linear reconciliation only; no runtime dependency ([Linear GraphQL](https://linear.app/developers/graphql)). **HIGH for API/tooling facts.**

### Expected Features

Feature scope is a complete v1, delivered in dependency-ordered vertical slices. Current evidence is official product manuals and specifications, not direct GOLC operator research, so the workflow list is **MEDIUM confidence** ([QLC+ concepts](https://docs.qlcplus.org/v5/basics/glossary-and-concepts), [MagicQ programmer](https://secure.chamsys.co.uk/docs/magicq/programmer/programmer.html), [MagicQ playback](https://secure.chamsys.co.uk/docs/magicq/manual/playback.html)).

**Must have (table stakes):**

- Versioned canonical fixture schema, useful starter library, guided fixture authoring, provenance, validation, and pinned definition revisions.
- Batch fixture patching with stable IDs, mode/footprint inspection, address map, and overlap/range rejection.
- Semantic programmer with fixture/group selection, locate/highlight, touched-attribute visibility, clear/release, undo/redo, and Intensity/Color/Position/Beam palettes.
- Scenes/looks, lightweight GO-driven cue lists, practical chases, fade/delay/hold, tap tempo, and explicit release rules.
- Deterministic HTP intensity and explicit LTP/ownership rules for other attributes, including mid-fade interruption and source release.
- Structured playback banks, keyboard control, group/submasters, Grand Master, blackout, stop/release-all, and source/ownership indication.
- Art-Net interface/target configuration, reliable output, subscriber/node status, final-value/source monitoring, frame health, and errors.
- Atomic save/open/save-as, autosave, rotating recovery, schema migration, backup-before-migrate, and restore diagnostics.
- MIDI Note/CC learn for buttons/faders, soft takeover, and feedback where supported.

**Should have (GOLC differentiators, still v1):**

- Patch-to-playback accelerators: auto-groups/palettes, persistent programmer context, one-step record, inline rename/reorder, and contextual shortcuts.
- One typed command/state/event model shared by Wails, keyboard/MIDI, TypeScript, HTTP, and LLM tools.
- First-class TypeScript editor, generated SDK, diagnostics, cancellation, structured logs, event subscriptions, and bounded execution.
- Stable public API with versioning, dry-run, atomic meaningful batches, idempotency, optimistic revisions, typed errors, audit attribution, and scoped remote enablement.
- Evidence-backed AI fixture drafting through the same import/validate/diff/commit pipeline.
- Provider-neutral show authoring and separately armed live autonomy with allowlists, risk tiers, batch/rate/time limits, stale-state rejection, visible lease, audit, cancellation, and model-independent operator override.
- Explainable command history and reversible authoring; live actions are auditable/compensatable, not falsely described as physically reversible.

**Defer to v1.x after validation:**

- OSC-to-API bridge, fixture morph/remap, reusable parameter effects, advanced MIDI profiles/beat clock/MSC, blind editing, thin remote client, and QLC+ import.

**Defer to v2+:**

- sACN and USB-DMX, professional tracking/move-while-dark/multipart cues, timecode/media/audio-reactive tools, pixel mapping, 2D/3D visualization, multi-console redundancy, multi-user editing, and native/plugin marketplaces.

**Explicit anti-features:** raw DMX writes from scripts/API/AI, blank-canvas-first playback, unbounded autonomy, LLM timing/output, arbitrary online script dependencies, cloud-required operation, multiple output protocols at launch, and silent approximation of unsupported fixture semantics.

### Architecture Approach

Use a modular monolith with hexagonal boundaries. Authoring state is mutable only through serialized commands; compilation produces an immutable `RenderPlan`; the engine adopts only a complete valid plan at a frame boundary; transport receives complete `FrameSet` values through an overwrite-latest mailbox. Durable commands and audit are not lossy, but UI/telemetry events may coalesce and must carry revisions so clients can re-query. This pattern is a reasoned architecture built from documented runtime behavior, not a directly prescribed framework feature. **Confidence: MEDIUM.** See [ARCHITECTURE.md](./ARCHITECTURE.md).

**Non-negotiable runtime invariant:**

```text
UI / scripts / API / LLM / Linear
             |
      typed commands + queries
             v
dispatcher -> authoring state -> compiler -> immutable RenderPlan
                                             |
manual safety lane --------------------------+--> single-owner engine
                                                   |
                                           complete FrameSet
                                                   v
                                      independent Art-Net worker
```

No dependency above the engine may own playback time, mutate frame buffers, write UDP, execute inside a frame, or backpressure output.

**Major components:**

1. **`contracts/v1` and application kernel** — stable DTOs, actor/correlation IDs, revisions, idempotency, permissions, commands, queries, events, and structured errors.
2. **Fixture catalog/import and patch** — canonical semantic definitions, provenance, strict normalization, stable fixture IDs, addressing, and impact reports.
3. **Show authoring and compiler** — programmer, palettes, scenes, cue lists, chases, timing, and all-or-nothing `ShowDocument` to `RenderPlan` compilation.
4. **Live engine and safety controller** — single mutable owner, monotonic schedule, HTP/LTP arbitration, preallocated rendering, missed-frame skip policy, and dedicated highest-priority local override.
5. **Art-Net transport** — sole socket/sequence/topology owner, discovery, subscriber unicast/static targets, latest-frame consumption, ArtSync option, and independent health.
6. **Persistence and audit** — one serialized SQLite writer, transactional state/command/audit commits, migrations, verified backup, integrity checks, and recovery.
7. **Adapters** — thin Wails facade, loopback-first HTTP/SSE API, Deno script IPC, and LLM proposal/tool adapter; all call the same kernel.
8. **Observability and testkit** — virtual clock, deterministic renderer vectors, independent Art-Net node simulator, bounded diagnostics, fault injection, and physical-rig soak.
9. **Traceability tooling** — repository/Linear manifest and reconciliation CLI/CI, isolated from every product package.

### Linear Delivery Traceability

Linear is mandatory from project inception and at every phase transition, but it is a governance gate rather than a console feature or runtime service.

| Repository object | Linear default | Stable linkage |
|---|---|---|
| Active release/milestone | Project | Store its immutable GraphQL UUID; optionally group multiple releases under an Initiative later |
| Roadmap phase | Project Milestone | Store UUID; phase number/title are metadata, not identity |
| `REQ-*` requirement | Issue labeled as a requirement | Store immutable UUID plus the human identifier/URL for display |
| Phase plan or feature slice | Parent issue | Link to the phase milestone and requirements delivered |
| Executable task | Issue/sub-issue | Close only with repository verification evidence |

Commit complete planning intent locally and maintain the credential-free `.planning/linear-map.json` keyed by durable local IDs. Sync is explicit and idempotent, queries by UUID before mutation, handles pagination/partial GraphQL errors/rate limits, and produces dry-run reconciliation for ambiguity. Offline planning, builds, tests, authoring, and playback continue when Linear is unavailable ([Linear pagination](https://linear.app/developers/pagination), [webhooks](https://linear.app/developers/webhooks), [project milestones](https://linear.app/docs/project-milestones)). **Confidence: MEDIUM.**

### Critical Pitfalls

1. **UI or adapter owns playback** — start the headless Go engine independently; UI/events are bounded projections, and DOM reload/hang must not alter receiver cadence ([Wails lifecycle](https://wails.io/docs/howdoesitwork)).
2. **Ticker treated as a real-time guarantee** — derive state from monotonic target time, use an injected clock, skip overdue intermediate frames, measure jitter/deadline misses, and never build a catch-up queue ([Go `time`](https://pkg.go.dev/time)).
3. **Partial Art-Net implementation or wrong interface** — implement exact Port-Address, length, sequence, refresh, discovery, subscriber-unicast, retransmit, and optional ArtSync rules; bind an operator-selected adapter and test VPN/DHCP/unplug cases ([Art-Net 4](https://art-net.org.uk/downloads/art-net.pdf)).
4. **Flat or untrusted fixture definitions** — canonicalize semantic capabilities, fine/switching channels, physical ranges, maintenance ranges, provenance, and definition hashes; reject unsupported ambiguity and smoke-test physical fixtures ([OFL compatibility warning](https://open-fixture-library.org/about/plugins/ofl), [GDTF DMX guidance](https://gdtf-share.com/help/users/gdtf_builder/dmx/index.html)).
5. **Undefined concurrent ownership** — one arbiter defines source priority, HTP/LTP, fade interruption, release, and highest-priority blackout; commands carry IDs, actors, revisions, and deadlines.
6. **Unsafe save/migration assumptions** — use one storage service, transactional migrations, SQLite-aware backup, newer-schema refusal, integrity/foreign-key checks, forced-kill tests, and a pinned SQLite build with applicable WAL fixes ([SQLite corruption guidance](https://www.sqlite.org/howtocorrupt.html)).
7. **Automation bypasses the kernel** — scripts/API/LLM receive capability-scoped typed commands only; enforce rate/resource limits, idempotency, expected revisions, audit, and a local override lane that cannot be defeated by queued automation.
8. **Simulator-only or build-only release evidence** — qualify packet bytes and timing on an independent virtual receiver plus at least two node implementations; install and exercise network/save/restore on every supported OS.
9. **Linear becomes repository truth** — keep complete offline artifacts, map immutable UUIDs, reconcile rather than poll blindly, and never make sync a build or runtime prerequisite.

The detailed failure signs and recovery procedures are in [PITFALLS.md](./PITFALLS.md). Pitfall facts derived from official protocol/runtime documentation are **MEDIUM-HIGH**; prevention strategies and phase placement are **MEDIUM architectural inference** until validated.

## Implications for Roadmap

The roadmap should use the following dependency order. Linear traceability begins before Phase 1 and remains a gate throughout; it is not counted as product runtime work.

### Phase 0: Repository/Linear Traceability (ongoing governance track)

**Rationale:** The project requires delivery traceability from inception, and retrofitting stable mappings creates duplicates and ambiguous identity.

**Delivers:** Stable local milestone/phase/requirement/task IDs, initial Linear Project and Project Milestones, `.planning/linear-map.json`, idempotent sync/reconcile skeleton, and uniqueness/offline-completeness checks.

**Addresses:** Project tracking requirement.

**Avoids:** Linear title identity, online-only planning, duplicate retry creation, and webhook-as-ledger assumptions.

**Research flag:** Standard documented GraphQL/UUID pattern; skip general phase research. Decide the actual workspace/team/status taxonomy during setup.

### Phase 1: Domain and Command Kernel

**Rationale:** Every control surface and safety property depends on shared types, commands, revisions, permissions, and deterministic test seams.

**Delivers:** Typed fixture/address/playback primitives; `contracts/v1`; serialized dispatcher; queries/events; actor, correlation, idempotency, revision and audit primitives; capability model; dedicated manual safety lane; virtual clock; dependency checks.

**Addresses:** Shared control consistency, observable/auditable actions, immediate override foundation.

**Avoids:** Raw integers crossing boundaries, adapter-specific behavior, direct mutable-state races, blackout in a normal backlog, and API-after-UI contract drift.

**Research flag:** Standard patterns; skip broad research. Planning must still settle command granularity, error taxonomy, and the precise safety-lane contract.

### Phase 2: Fixture Definitions and Patch

**Rationale:** Semantic fixtures and stable addressing precede programmer controls, cues, frame compilation, useful scripts, and trustworthy AI.

**Delivers:** Canonical versioned fixture vocabulary, provenance and pinned revisions, OFL/manual/LLM-draft ingestion boundary, validator and golden corpus, starter library, guided editor foundation, fixture instances, modes, batch patching, address map, and impact reports.

**Addresses:** Fixture library/authoring, fast patching, common generic fixtures.

**Avoids:** Flat channel models, silent approximation, mode/fine/switch errors, generic pan/color semantics, slot/universe off-by-one, and library updates changing old shows.

**Research flag:** **Needs phase research** on canonical capability vocabulary, representative fixture corpus, OFL snapshot/licensing, GDTF preservation, hazardous/maintenance attributes, and actual first-user fixtures.

### Phase 3: Show Model and Deterministic Playback Engine

**Rationale:** Playback semantics must be correct headlessly before UI, persistence, or automation can depend on them.

**Delivers:** Programmer state, groups/palettes, scenes, lightweight cue lists, chases, attribute-family timing, HTP/LTP/source arbitration, compiler, immutable plans, single-owner engine, preallocated pure renderer, manual override, and golden frame tests.

**Addresses:** Core conventional authoring semantics, timing, merge, release, masters/blackout foundation.

**Avoids:** One timer per fade, tick-count timing, wall-clock LTP, undefined overlap, catch-up bursts, partial plan adoption, and script/model callbacks in frames.

**Research flag:** **Needs phase research** on measurable jitter/override budgets, cue/release/pause semantics, plan adoption during live edits, scale ceiling, and deterministic effect seeding.

### Phase 4: Art-Net Vertical Slice and Simulation

**Rationale:** Prove patch-to-frame-to-wire behavior before desktop workflow complexity masks protocol faults.

**Delivers:** Internal packet codec, discovery/topology, operator-selected interface, static unicast targets, sequence/rate/address/retransmit rules, latest-frame mailbox, optional ArtSync capability, independent node simulator, output health, packet capture tests, and physical-node smoke gate.

**Addresses:** Reliable observable Art-Net and final DMX/network monitoring foundation.

**Avoids:** Broadcast-everywhere output, wrong NIC, malformed payloads, universe shifts, renderer backpressure, simulator-only confidence, and false claims that UDP send proves fixture output.

**Research flag:** **Needs phase research** against the first users' actual nodes, current subscriber/unicast behavior, static-target UX, compatibility policy, and hardware test matrix.

### Phase 5: Durable Show Storage and Recovery

**Rationale:** Serious UI authoring must operate on a stable, recoverable format; persistence cannot be final polish.

**Delivers:** `.golc` SQLite format, one writer, transactional state/command/audit commits, schema migrations, save/open/save-as, coalesced autosave, verified backups, rotating recovery, integrity/read-only recovery, JSON export, and historical migration corpus.

**Addresses:** Durable show save, autosave, recovery, migration, and explainable command history.

**Avoids:** Copying live WAL files, half migrations, newer-schema rewrites, audit/state divergence, and persistence I/O on the render loop.

**Research flag:** **Targeted phase research** on SQLite durability pragmas, backup retention, portable file/export policy, migration support window, and platform-specific atomic replacement.

### Phase 6: Wails Operator Workflow and MIDI

**Rationale:** Build the product's speed advantage on proven headless commands and persistence, while ensuring UI failure cannot influence output.

**Delivers:** Thin Wails adapter, generated types, revisioned frontend cache, fixture/patch/programmer/cue/chase editors, auto-generated playback banks, masters/blackout, keyboard shortcuts, MIDI Note/CC learn/soft takeover/feedback, diagnostics, and gap/reconnect recovery.

**Addresses:** Modern fast conventional workflow, patch-to-playback accelerators, structured live surface, keyboard/MIDI, monitoring.

**Avoids:** Frontend authority, blank-canvas setup burden, full frame streaming to UI, hidden source ownership, ambiguous adapter state, and edit/live-mode confusion.

**Research flag:** Technical patterns are standard, but **operator validation is required** for screen density, navigation, cue-list importance, MIDI targets, and setup-time improvement versus QLC+. Use UI-specific design/research before implementation.

### Phase 7: Versioned Public API

**Rationale:** Stabilize the public surface after conventional workflows exercise domain semantics, then reuse it as the external contract base for scripts and agents.

**Delivers:** Huma/Chi `/api/v1`, OpenAPI 3.1, REST commands/queries, SSE with replay/gap semantics, loopback default, scoped authenticated remote mode, rate limits, idempotency and revision conflicts, generated TS client, AsyncAPI event contract, and parity tests with Wails.

**Addresses:** First-class documented external control and behavioral parity.

**Avoids:** Handlers reaching repositories/engine directly, retry duplication, last-write races, all-interface unauthenticated binding, API/desktop semantic drift, and WebSocket-first complexity.

**Research flag:** Standard patterns; skip broad research. Planning must define compatibility/deprecation policy, remote-access threat model, and SSE retention limits.

### Phase 8: TypeScript Scripting

**Rationale:** Scripts need stable capabilities, persistence, contracts, observability, and recovery before untrusted code is introduced.

**Delivers:** Monaco editor, generated `golc` SDK/declarations, Deno acceptance checks and execution helper, restricted imports/permissions, capability IPC, deadlines, quotas, per-script queues, logs/source locations, subscriptions, cancellation, process supervision, and script tests.

**Addresses:** First-class TypeScript authoring, execution, debugging foundation, and same-command behavior.

**Avoids:** In-process untrusted runtime failure, arbitrary dependencies, ambient host access, blocking native callbacks, shared runtimes, unbounded event/command queues, and direct DMX writes.

**Research flag:** **Needs phase research** on Deno embedding/distribution, cached-only module policy, memory/CPU enforcement per OS, IPC authentication, debugger scope, and hostile-code claims.

### Phase 9: Provider-Neutral AI and Bounded Autonomy

**Rationale:** AI is the last mutating adapter because it amplifies every gap in commands, fixture validation, permissions, observability, persistence, and recovery.

**Delivers:** Bifrost adapter, provider configuration abstraction, structured typed tools, snapshot/revision observe-plan-validate-execute loop, dry-run/diff, evidence-backed fixture drafts, bounded show authoring, separately armed live leases, allowlists/risk tiers/rate-time-batch caps, redacted audit, cancellation, and model-independent revoke/blackout.

**Addresses:** Hosted/local provider neutrality, fixture assistance, autonomous programming, and intended live control.

**Avoids:** Model as timing engine, invented/stale IDs, retrying destructive tools, direct domain/DMX access, hidden sessions, unsafe maintenance ranges, leaked sensitive data, and stop commands that depend on the model.

**Research flag:** **Needs phase research** on Bifrost maturity/provider parity, structured-output behavior across hosted/local models, context limits, cancellation, local deployment, evaluation corpus, safety policy, and audit redaction. Validate autonomous authoring before exposing live autonomy.

### Phase 10: Cross-Platform Release Qualification

**Rationale:** Build success is not live-show readiness; the architecture's timing and isolation claims require measured evidence on every supported OS and real hardware.

**Delivers:** Native build/install/signing pipelines, clean-machine tests, WebView/dependency checks, suspend/resume and network-change behavior, migration/restore exercises, long fault/soak runs under UI/save/script/API/LLM load, two-node physical Art-Net qualification, representative fixtures, operator runbooks, and release budgets.

**Addresses:** Cross-platform support and reliable small-show release evidence.

**Avoids:** Cross-compile-only claims, average-only frame metrics, single-node compatibility, untested installer/runtime dependencies, and release without recovery drills.

**Research flag:** **Needs targeted research** for the final OS/distribution matrix, signing/notarization, oldest supported environments, measured timer/jitter limits, and physical rig selection.

### Phase Ordering Rationale

- Stable identities, commands, revisions, and safety precede every adapter.
- Fixture semantics precede programmer, cues, compilation, scripts, and AI.
- Deterministic playback precedes Art-Net; the headless Art-Net slice precedes UI complexity.
- Persistence precedes serious operator authoring so schema/recovery are designed into commands.
- Wails validates conventional workflow before freezing the public API.
- API precedes scripts and LLM tools as the documented external contract; scripts and AI still call the in-process kernel through adapters rather than HTTP internally.
- LLM autonomy is last among mutating surfaces and release qualification is last overall, while testing/CI/Linear traceability begin at inception.

### Research Flags Summary

**Use deeper phase research:** Phases 2, 3, 4, 8, 9, and 10; targeted research for Phase 5.

**Use user/workflow validation:** Phase 6, especially first-user scale, cue-list needs, MIDI hardware, and measurable patch-to-playback speed.

**Standard patterns; skip general research:** Phase 0, Phase 1, and Phase 7, while resolving their listed local design questions during planning.

## Confidence Assessment

| Area | Confidence | Notes |
|---|---|---|
| Stack | MEDIUM-HIGH | Versions and framework/runtime capabilities were verified with Context7 and official sources. Bifrost maturity, Deno resource isolation, internal Art-Net ownership, and Linux packaging remain implementation choices requiring validation. |
| Features | MEDIUM | Conventional workflow is supported by official QLC+, MagicQ, ETC, OFL, Art-Net, and MIDI documentation, but no direct GOLC operator interviews or field trials have validated scope/order. |
| Architecture | MEDIUM | Hexagonal boundaries, immutable snapshots, single ownership, and bounded queues fit documented Go/Wails/runtime behavior, but they are architectural inference. Jitter, scale, and failure behavior require benchmarks. |
| Pitfalls | MEDIUM | Protocol/runtime/storage failure modes are grounded in official specifications and documentation; prevention details and recovery costs must be proven on real nodes, fixtures, and OS builds. |
| Linear traceability | MEDIUM | Stable UUID mapping and offline reconciliation follow official Linear APIs. Exact workspace taxonomy, status flow, and whether/when to add an Initiative remain project choices. |

**Overall confidence:** MEDIUM

### Verified Facts vs Architectural Inference

**Verified from primary/Context7 sources:** current researched versions; Wails v2 bindings/lifecycle; Go ticker/monotonic behavior; Art-Net field, rate, address, sequence, discovery/unicast, and sync rules; SQLite WAL/backup/integrity mechanisms; OFL compatibility warning; Linear GraphQL UUID, pagination, webhook, milestone, and rate-limit behavior.

**Architectural inference to validate:** the one-process modular monolith, single engine owner, 40 Hz default, latest-value mailboxes, exact command granularity, Deno as the preferred script host, Huma/SSE contract shape, Bifrost as the provider adapter, phase ordering, and the proposed Linear hierarchy.

### Gaps to Address

- **First-user scale:** Validate universe/fixture/playback/MIDI ceilings before setting performance budgets.
- **Operator workflow:** Test whether lightweight cue lists are essential at launch and measure patch-to-playback time against QLC+.
- **Fixture scope:** Select a representative physical corpus, define the canonical attribute vocabulary, choose OFL snapshot/update/licensing policy, and define explicit unsupported behavior.
- **Art-Net interoperability:** Test current and legacy node behavior, multi-NIC/VPN cases, sequence/retransmit rules, static targets, and optional ArtSync on at least two implementations.
- **Timing budgets:** Set per-OS frame interval, render duration, miss, override-latency, allocation, and soak thresholds from measurement.
- **Persistence policy:** Decide durability pragmas, backup retention, schema support window, export/bundle format, and read-only recovery UX.
- **MIDI targets:** Identify initial controllers and whether generic Note/CC covers feedback without device-specific SysEx.
- **Script boundary:** Prove Deno packaging, offline dependencies, process supervision, memory/CPU limits, debugger behavior, and honest sandbox claims across platforms.
- **AI staging:** Decide whether the first public build exposes live cue triggering or validates autonomous offline authoring first; define hazardous fixture restrictions and evaluation criteria.
- **Provider abstraction:** Verify Bifrost provider parity, structured tools, cancellation, local OpenAI-compatible endpoints, and upgrade policy behind `ModelClient`.
- **Platform matrix:** Name supported OS versions/distributions/architectures and establish native packaging/signing/clean-install CI early.
- **Linear taxonomy:** Confirm workspace/team/status labels and optional Initiative usage without changing the stable local-ID/UUID rule.

## Sources

Detailed provenance and claim-level links are retained in [STACK.md](./STACK.md), [FEATURES.md](./FEATURES.md), [ARCHITECTURE.md](./ARCHITECTURE.md), and [PITFALLS.md](./PITFALLS.md).

### Primary and Context7-Verified

- Context7 `/websites/v3_wails_io`, `/websites/wails_io`, `/wailsapp/wails` — Wails version status, bindings, events, and build model.
- Context7 `/react/react/v19.2.7`, `/vitejs/vite`, `/pmndrs/zustand`, `/microsoft/monaco-editor` — frontend versions and external-store/worker patterns.
- Context7 `/websites/pkg_go_dev_modernc_org_sqlite`, `/pressly/goose` — pure-Go SQLite driver targets and embedded migrations.
- Context7 `/danielgtaylor/huma`, `/websites/openapi-ts_dev`, `/asyncapi/spec` — OpenAPI generation, streaming, generated TypeScript, and event contracts.
- Context7 `/websites/linear_app_developers`, `/linear/linear` — GraphQL/SDK integration approach.
- [Go releases and runtime documentation](https://go.dev/dl/) — compiler version and runtime primitives.
- [Wails documentation](https://wails.io/docs/howdoesitwork) — adapter/binding/lifecycle behavior.
- [Art-Net 4 specification](https://art-net.org.uk/downloads/art-net.pdf) — protocol requirements.
- [Deno security model](https://docs.deno.com/runtime/fundamentals/security/) — permissions and execution boundary.
- [SQLite backup](https://www.sqlite.org/backup.html), [WAL](https://sqlite.org/wal.html), and [integrity guidance](https://www.sqlite.org/howtocorrupt.html) — persistence and recovery.
- [Open Fixture Library format](https://github.com/OpenLightingProject/open-fixture-library/blob/master/docs/fixture-format.md) and [compatibility warning](https://open-fixture-library.org/about/plugins/ofl) — fixture semantics and adapter requirement.
- [Linear GraphQL](https://linear.app/developers/graphql), [milestones](https://linear.app/docs/project-milestones), [webhooks](https://linear.app/developers/webhooks), and [rate limits](https://linear.app/developers/rate-limiting) — traceability mechanics.
- [NIST AI RMF Core](https://airc.nist.gov/airmf-resources/airmf/5-sec-core/) and [OWASP AI Agent Security](https://cheatsheetseries.owasp.org/cheatsheets/AI_Agent_Security_Cheat_Sheet.html) — oversight, least privilege, bounded action, audit, and recovery guidance.

### Product and Workflow References

- [QLC+ concepts](https://docs.qlcplus.org/v5/basics/glossary-and-concepts), [Virtual Console](https://docs.qlcplus.org/v4/virtual-console), and [MIDI](https://docs.qlcplus.org/v5/plugins/midi) — small-show baseline and setup friction reference.
- [MagicQ programmer](https://secure.chamsys.co.uk/docs/magicq/programmer/programmer.html), [cue stacks](https://secure.chamsys.co.uk/docs/magicq/manual/cue_stacks.html), and [playback](https://secure.chamsys.co.uk/docs/magicq/manual/playback.html) — programmer/palette/cue/playback vocabulary.
- [ETC show-file saving](https://www.etcconnect.com/WebDocs/Controls/EosFamilyOnlineHelp/en/Content/05_Show_Files/Saving_Show_Files.htm) — persistence/recovery expectations.
- [MIDI 1.0 specifications](https://midi.org/midi-1-0-core-specifications) — controller baseline.

---
*Research completed: 2026-07-17*
*Ready for roadmap: yes*
