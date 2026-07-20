# Feature Research

**Domain:** Cross-platform, Art-Net-first lighting control for small live shows
**Researched:** 2026-07-17
**Confidence:** MEDIUM — recommendations are cross-checked against current official software manuals and specifications, but no direct GOLC operator research has yet validated the scope or workflow.

## Feature Landscape

GOLC v1 is credible only if it can run a small show without scripts or AI. The conventional path is the product spine: define and patch fixtures, select fixtures or groups, set semantic attributes in a programmer, record reusable palettes and looks, arrange cues or chases, then play them through a deterministic HTP/LTP engine into Art-Net. TypeScript, the public API, and LLM tools must invoke that same model.

The deliberate v1 boundary is a fast small-show console, not a reduced copy of a touring console. It needs a lightweight programmer and cue-list model, but not full tracking semantics, timecode, media servers, pixel mapping, 3D visualization, multi-console redundancy, or hundreds of playback pages. Official QLC+ documentation establishes the broad small-show baseline, while ChamSys and ETC manuals establish the conventional programmer/palette/cue/playback vocabulary ([QLC+ concepts](https://docs.qlcplus.org/v5/basics/glossary-and-concepts), [MagicQ programmer](https://secure.chamsys.co.uk/docs/magicq/programmer/programmer.html), [MagicQ playback](https://secure.chamsys.co.uk/docs/magicq/manual/playback.html)).

### Table Stakes (Users Expect These)

Missing any P1 item below makes v1 unable to program or safely operate the intended class of show.

| Feature | Why Expected | Complexity | Dependency / v1 Notes | Confidence |
|---------|--------------|------------|-----------------------|------------|
| Versioned fixture model and useful library | Operators think in fixtures, modes, and attributes, not anonymous DMX slots. A usable library avoids authoring every common fixture before the first show. | HIGH | Define GOLC's canonical, versioned schema first. Cover modes, defaults/highlight, semantic capabilities and ranges, HTP/LTP precedence, fade/snap behavior, 8/16-bit channels, and multi-cell basics. Seed a schema-pinned converted library; do not persist OFL's unstable internal format directly. OFL documents these concepts and explicitly warns consumers to use an adapter ([fixture format](https://github.com/OpenLightingProject/open-fixture-library/blob/master/docs/fixture-format.md), [JSON plugin warning](https://open-fixture-library.org/about/plugins/ofl)). | MEDIUM |
| Fast fixture patching | Patch by manufacturer/model/mode, universe/address, quantity and address gap; rename, clone, readdress, unpatch; show footprint and reject overlaps/out-of-range addresses before they corrupt a show. | HIGH | Requires fixture schema and show state. Include generic dimmer/RGB/pan-tilt fallbacks and a visual address map. Batch patch is essential to beat QLC+'s setup friction; QLC+ itself treats fixture management as the core of its fixture-oriented architecture ([fixture manager](https://docs.qlcplus.org/v4/fixture-manager), [add/edit fixtures](https://docs.qlcplus.org/v4/fixture-manager/add-edit-fixtures)). | MEDIUM |
| Fixture-definition authoring and validation | Small-show operators regularly encounter unlisted or budget fixtures; authoring cannot be an external developer task. | HIGH | Provide a guided editor, raw structured view, manual/source attachment, mode/channel reordering, capability-range gap/overlap checks, fine-channel links, defaults, test values, diff, and import/export. Validation must precede patch use. Advanced matrices, wheels, switching channels, and RDM metadata may be editable later, but the canonical schema must preserve them. | MEDIUM |
| Attribute-aware programmer | A console needs a temporary, inspectable place to select fixtures/groups, locate/home or highlight them, change intensity/color/position/beam, see only touched attributes, clear/release, and record the result. | HIGH | Requires patch plus semantic attributes. Programmer data has higher priority than playback, but manual override must be obvious and releasable. Ship selection history, next/previous fixture, odd/even, copy values, and undo/redo; defer command-line syntax and complex fan operations. MagicQ documents the programmer as the cue-building surface and a live override ([programmer](https://secure.chamsys.co.uk/docs/magicq/programmer/programmer.html)). | MEDIUM |
| Groups and palettes/presets | Re-selecting fixtures and re-entering raw values makes programming slow and makes later fixture changes brittle. | MEDIUM | Groups require stable fixture/head IDs and selection. Provide reusable Intensity, Color, Position, and Beam palettes referenced by scenes/cues rather than flattened wherever practical. Auto-create useful groups and fixture-derived palettes after patch. MagicQ uses the same four palette families and records palette references into cues ([palettes](https://secure.chamsys.co.uk/docs/magicq/manual/palletes.html)); QLC+ scenes also accept groups and palettes ([scene editor](https://docs.qlcplus.org/v5/function-manager/scene-editor)). | MEDIUM |
| Looks/scenes and lightweight cue lists | Static looks and ordered GO-driven transitions cover busking, clubs, schools, churches, and simple theatre. | HIGH | Record/update/merge, label, clone, reorder, preview, and delete. Store explicit touched attributes with clear release/default behavior; do not introduce full professional-console tracking in v1. Cue lists need current/next indication, GO, pause/resume, back, jump, and release. ChamSys cue stacks establish ordered cues and current/next playback state ([cue stacks](https://secure.chamsys.co.uk/docs/magicq/manual/cue_stacks.html), [playback](https://secure.chamsys.co.uk/docs/magicq/manual/playback.html)). | MEDIUM |
| Chases with practical timing | Automated repeated looks are core small-show functionality, not an advanced effect. | MEDIUM | A chase references scenes/cues and supports forward, reverse, bounce, random, loop count/single-shot, rate/BPM, tap tempo, and crossfade. Provide per-step hold and a global rate override. QLC+ and MagicQ both document these behaviors ([QLC+ concepts](https://docs.qlcplus.org/v5/basics/glossary-and-concepts), [MagicQ chase options](https://secure.chamsys.co.uk/docs/magicq/manual/cue_stack_settings.html)). | MEDIUM |
| Fade, delay, hold, and release timing | Abrupt transitions make even a correctly programmed show look broken. | HIGH | Use one monotonic engine clock. v1 needs cue/scene fade-in, fade-out/release, delay and hold plus separate timing by attribute family (Intensity, Color, Position, Beam). Defer individual-channel, per-fixture split timing, multipart cues, mark/move-while-dark, and timecode. | MEDIUM |
| Deterministic playback and merge | Several playbacks and the programmer will control overlapping attributes; output must remain predictable. | HIGH | Define HTP for intensity and LTP/most-recent ownership for non-intensity attributes, programmer priority, explicit release, and stable behavior when a source stops. Playback timing must be independent of UI, scripts, API clients, and LLM latency. QLC+ exposes channel HTP/LTP and fade behavior ([channel properties](https://docs.qlcplus.org/v5/fixture-manager/channel-properties)); MagicQ documents programmer and playback priority ([programmer](https://secure.chamsys.co.uk/docs/magicq/programmer/programmer.html), [advanced playback](https://secure.chamsys.co.uk/docs/magicq/manual/advanced_cue_stacks.html)). | MEDIUM |
| Structured live playback surface | An operator needs an immediately usable show screen with buttons/faders, not a design tool they must build before rehearsal. | MEDIUM | Auto-populate a reorderable grid/bank from recorded scenes, cue lists, and chases. Support toggle, momentary flash, GO, pause, release, fader level, names/colors, keyboard focus, and unmistakable running/current/next/fade-progress state. Keep a separate safe edit mode. QLC+'s blank-canvas Virtual Console shows the baseline controls but also the setup burden GOLC should remove ([Virtual Console](https://docs.qlcplus.org/v4/virtual-console), [button](https://docs.qlcplus.org/v5/virtual-console/button), [slider](https://docs.qlcplus.org/v5/virtual-console/slider)). | MEDIUM |
| Masters, blackout, and release-all | The operator needs a hardware-like last-resort action that wins immediately. | MEDIUM | Include Grand Master for intensity, configurable group/submasters, momentary and latched blackout, and stop/release-all. Blackout and manual release must have higher priority than scripts, API calls, or AI and cannot be re-enabled by them. MagicQ documents Grand Master, playbacks, and dead blackout behavior ([playback](https://secure.chamsys.co.uk/docs/magicq/manual/playback.html)); QLC+ exposes blackout and stop-all controls ([button](https://docs.qlcplus.org/v5/virtual-console/button)). | MEDIUM |
| Art-Net setup, reliable output, and live output monitor | A console that cannot prove what it is sending is unsafe to troubleshoot during load-in or a show. | HIGH | Configure interface/IP, Art-Net Net/Sub-Net/Universe or Port-Address, destinations/subscribers, refresh rate, and enable/disable per universe. Monitor final DMX values by fixture/address plus controlling source, node/subscriber state, frame rate, sequence, last-send time, dropped/late frames, and errors. Implement current ArtPoll/ArtPollReply/ArtDmx behavior and track conformance, OEM code, and attribution requirements from the official source ([Art-Net introduction](https://art-net.org.uk/art-net-introduction-and-terminology/), [Art-Net 4 specification](https://art-net.org.uk/downloads/art-net.pdf), [official site](https://art-net.org.uk/)). | MEDIUM |
| Durable show save, autosave, recovery, and migration | Losing a programmed show or reopening into a different state is release-blocking. | HIGH | Save/Open/Save As, dirty indicator, atomic writes, autosave-on-change away from the timing loop, rotating revisions, crash recovery, exportable show bundle, and explicit schema migration with backup-before-migrate. Resume the last recoverable show without automatically enabling output. Established consoles retain autosave and backup archives ([MagicQ system management](https://secure.chamsys.co.uk/docs/magicq/manual/system_management.html), [ETC saving show files](https://www.etcconnect.com/WebDocs/Controls/EosFamilyOnlineHelp/en/Content/05_Show_Files/Saving_Show_Files.htm)). | MEDIUM |
| Keyboard and narrow MIDI 1.0 control | Small-show operators commonly use inexpensive fader/button controllers; mouse-only playback undermines the live workflow. | HIGH | Ship comprehensive keyboard bindings and MIDI Note/Control Change learn for buttons and faders, press/release normalization, soft takeover, and outbound state feedback where supported. Route mapped actions through the same command API. Defer SysEx templates, MIDI Show Control, timecode, and broad device profiles. QLC+ documents these practical inputs and feedback behaviors ([MIDI](https://docs.qlcplus.org/v5/plugins/midi), [input profiles](https://docs.qlcplus.org/v5/input-output/input-profiles)); MIDI.org identifies lighting controllers as a MIDI use case ([MIDI 1.0 core](https://midi.org/midi-1-0-core-specifications)). | MEDIUM |

### Differentiators (Competitive Advantage)

These are differentiators in the market, but they are still P1 for GOLC because they embody the stated product promise.

| Feature | Value Proposition | Complexity | Dependency / v1 Notes | Confidence |
|---------|-------------------|------------|-----------------------|------------|
| Patch-to-playback workflow accelerators | Delivers the core promise: dramatically less setup and navigation than QLC+. | MEDIUM | Batch patch, auto-groups and fixture-derived palettes, persistent selection/programmer side panel, global search/command palette, drag-free one-step Record to new/existing playback, safe inline rename/reorder, and contextual keyboard shortcuts. Preserve operator context instead of bouncing among fixture, function, and virtual-console editors. | MEDIUM |
| One typed command/state/event model for every control surface | UI behavior becomes the tested product API instead of privileged UI-only logic; extensions cannot drift into incomplete or contradictory behavior. | HIGH | Every mutation returns a typed result and revision, validates preconditions, carries actor/correlation IDs, and emits observable state events. Wails v2 generates JS/TS bindings and TypeScript models for bound Go methods, supporting the UI boundary ([Wails method binding](https://wails.io/docs/howdoesitwork), Context7 verified). The timed engine remains inside Go and off the render path. | MEDIUM |
| First-class TypeScript scripting | Users can build actual automation rather than QLC+'s small command-per-line DSL. | HIGH | Provide a versioned typed SDK, editor, formatting/type-checking, run/cancel, breakpoints or step/debug hooks where feasible, structured logs, event subscriptions, deterministic timers, stored dependencies, and script tests. Scripts get capability-scoped commands, quotas, cancellation, and no direct access to the output buffer, Go internals, filesystem, network, or UI DOM by default. QLC+'s documented script editor exposes a limited fixed command set ([QLC+ Script Editor](https://docs.qlcplus.org/v5/function-manager/script-editor)). | MEDIUM |
| Stable public API for software and agents | Enables remote surfaces, show tools, integrations, and alternative agents without UI automation. | HIGH | Version the API deliberately; document schemas and examples; expose read snapshots, validation/dry-run, atomic command batches, idempotency keys, optimistic revisions, typed errors, event subscriptions, actor attribution, and scoped authentication. The API wraps the command model rather than expose raw database or DMX writes. | MEDIUM |
| AI-assisted fixture authoring with evidence | Turns an error-prone blocking task into a guided review while keeping definitions inspectable. | HIGH | Ingest a user-supplied manual, extract manufacturer/model/modes/channels/capability ranges, retain page/source provenance, produce confidence and validation errors, and show a diff before acceptance. Never auto-publish or silently replace an in-use definition. Depends on canonical fixture schema, validator, and audit trail. | MEDIUM |
| Safe autonomous show authoring and operation | Provider-neutral hosted/local LLMs can perform genuine work—patch, program, refine, and play—without becoming a second lighting engine. | HIGH | This is the highest-risk P1 feature. Model proposes only typed domain commands. Enforce session scope, fixture/universe allowlists, risk tiers, command/schema validation, maximum batch/rate/runtime, idempotency, stale-revision rejection, audit log, and immediate cancellation. Authoring may run in preview/offline mode; live output requires a separately armed operator-controlled mode. NIST calls for documented capability scope, human oversight, monitoring, override, incident response, and recovery ([AI RMF Core](https://airc.nist.gov/airmf-resources/airmf/5-sec-core/)); OWASP recommends least privilege, structured validation, action previews, bounded retries, audit trails, and interrupt/rollback ([AI Agent Security](https://cheatsheetseries.owasp.org/cheatsheets/AI_Agent_Security_Cheat_Sheet.html)). | MEDIUM |
| Provider-neutral, local-first AI operation | Users can choose hosted or local models and can still run the show if the network or provider fails. | HIGH | Put model adapters behind a provider-neutral interface; store no provider-specific semantics in show files. Output, playback, saves, manual controls, and scripts remain fully usable with AI disabled or disconnected. Record provider/model/config metadata in each agent audit session. | MEDIUM |
| Explainable command history and reversible authoring | A unified timeline answers “what changed the lights?” and makes experimentation safer for humans, scripts, API clients, and AI. | HIGH | Record actor, command, arguments, affected entities, before/after revision, validation result, execution result, and timestamps. Support undo/redo and transaction rollback for show-authoring commands; live playback actions are auditable and compensatable, not falsely advertised as physically reversible. | MEDIUM |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Blank-canvas console as the primary live workflow | Total layout freedom looks powerful and resembles QLC+. | It forces operators to design a UI after they already programmed the show, duplicates content/control wiring, and creates design-vs-operate failure modes. | Auto-generate structured playback banks from show content; allow limited reordering, labels, colors, sizes, and pages. Consider a freeform layout only after the fast path is proven. |
| Direct raw-DMX access for scripts, API clients, or AI | Appears maximally flexible and is easy to prototype. | It bypasses fixture semantics, HTP/LTP ownership, timing, audit, undo, validation, and manual override; different surfaces will disagree about state. | Offer raw DMX as read-only diagnostics in v1. All writes use typed fixture/programmer/playback commands; add tightly scoped expert escape hatches only with an explicit safety model. |
| Unbounded AI autonomy or approval on every action | One extreme maximizes autonomy; the other appears maximally safe. | Unbounded tools can run away, while constant prompts create approval fatigue and make live operation unusable. Human confirmation alone can also be manipulated or misunderstood. | Risk-tiered autonomy: bounded authoring batches, preview/diff for structural changes, separately armed live scope, clear audit, rate/time limits, and a manual override/blackout that the agent cannot defeat. |
| LLM as a timing or output engine | Natural-language control seems like it could replace console logic. | Inference latency, nondeterminism, network failure, and retries cannot meet frame or fade deadlines. | LLMs schedule validated commands; the deterministic Go engine owns clocks, fades, merge, playback, and Art-Net continuously. |
| Full professional-console tracking model in v1 | Tracking, move-while-dark, multipart cues, priorities, and complex syntax signal sophistication. | These semantics interact deeply with palettes, updates, cue jumps, releases, and fixture replacement; a partial implementation is harder to trust than an explicit small-show model. | Store explicit touched attributes with clear release/default rules. Research tracking only after real users outgrow lightweight cue lists. |
| Full 3D visualizer, media timeline, pixel mapper, or advanced effects engine | They demo well and appear in mature consoles. | Each is effectively a product inside the product and distracts from patch-to-playback speed and live reliability. | Ship numeric/semantic output monitoring and simple fixture tiles. Add reusable parameter effects first; defer 3D, media, timecode, and pixel content to v2+. |
| Every lighting and show-control protocol at launch | Broad hardware compatibility appears to expand the market. | It multiplies cross-platform drivers, failure modes, configuration, and test hardware before the output abstraction is proven. | Art-Net 4 only behind a protocol boundary. Add sACN or USB DMX from measured demand after conformance and field testing. |
| Native OSC as a second product API in v1 | OSC is common in live production and simple to send. | A large OSC address tree would duplicate versioning, validation, authentication, errors, and subscriptions from the public API. OSC itself provides messages, address patterns, and bundles, not product semantics ([OSC 1.0](https://opensoundcontrol.stanford.edu/spec-1_0.html)). | Ship the versioned API first and a documented OSC-to-API bridge in v1.x; only promote stable mappings into native support after usage validates them. |
| Persisting OFL JSON as GOLC's native fixture/show contract | It gives instant access to an open fixture ecosystem. | OFL explicitly says its internal JSON can make backward-incompatible changes; binding saved shows to it creates migration risk. | Use a GOLC-owned versioned fixture schema and import/export adapters with source revision/provenance. |
| Required cloud account, hosted control plane, or one-model lock-in | Simplifies vendor integration and telemetry. | Venues may lack dependable Internet; account/provider failure must not stop playback; it violates the local-model requirement. | Local-first show data and control, optional credentials per provider, portable model adapters, and zero AI dependency in the live output path. |
| Browser/mobile client, multi-user editing, or redundant consoles in v1 | Remote collaboration and failover sound professional. | They introduce distributed state, conflict resolution, authentication, discovery, and synchronization before single-operator correctness is proven. | Keep a single authoritative desktop process. The public API enables later remote clients without prematurely distributing the engine. |
| Safety-interlock replacement or unrestricted autonomous control of hazardous devices | A “controls everything” story is attractive. | Software blackout is not a certified physical safety interlock; lasers, pyrotechnics, machinery, and similar hazards require device- and jurisdiction-specific controls. | Keep physical safety enables external. Treat hazardous capabilities as denied to AI/scripts by default and never market GOLC as replacing certified interlocks. |

## Feature Dependencies

```text
Versioned fixture schema + validation
    -> fixture library/import + fixture authoring
    -> patch and stable fixture/head IDs
    -> semantic attribute graph
    -> programmer + selection + groups
    -> palettes/presets
    -> scenes/looks + cue lists + chases
    -> timing + HTP/LTP merge + release rules
    -> structured playback + masters/blackout
    -> deterministic frame engine
    -> Art-Net output + live output monitor

Versioned show schema
    -> atomic save/autosave + migrations
    -> recovery archive + show bundles

Typed command/state/event model
    -> Wails UI
    -> keyboard/MIDI mappings + feedback
    -> TypeScript SDK/runtime
    -> versioned public API
        -> OSC bridge (v1.x)
        -> LLM tool adapter

Fixture authoring + source provenance + validator
    -> AI fixture-definition assistant

Command validation + capability policy + audit + manual priority
    -> autonomous AI authoring
    -> separately armed autonomous live operation
```

### Dependency Notes

- **Fixture semantics precede every authoring surface:** Palettes, attribute controls, fade/snap choices, HTP/LTP merge, and useful AI commands are impossible to validate against anonymous channel numbers.
- **The programmer precedes cues and chases:** A scene/cue records programmer state. A chase references recorded content; it should not become a second container for raw channel edits.
- **Timing and ownership precede playback UI:** Buttons and faders are safe only after overlap, release, and fade behavior are deterministic and testable without the UI.
- **The command model precedes scripting, API, MIDI, and AI:** Building any control surface early against private internals creates permanent semantic drift.
- **The public API precedes native OSC:** OSC can bridge to stable commands; defining both simultaneously duplicates the contract and weakens version discipline.
- **AI fixture authoring precedes AI patching for unknown devices:** The model may propose a definition, but the validator and operator acceptance establish the fixture semantics that later commands can use.
- **Autonomous authoring precedes autonomous live operation:** Offline/preview show edits exercise validation, auditing, cancellation, and rollback without changing emitted light. Live autonomy adds arming, rate limits, and irrevocable manual priority.
- **Persistence is not a final polish phase:** Stable entity IDs, command revisions, migration, audit, and recovery must be designed with the show model, or later saves will require a rewrite.

## MVP Definition

This is a feature-complete v1 rather than a thin prototype. The correct risk reduction is vertical sequencing—prove each dependency layer with a working slice—not deleting the safety and interoperability commitments that define GOLC.

### Launch With (v1)

- [ ] Versioned fixture/show schemas, converted starter library, guided fixture authoring, and strict validation.
- [ ] Batch patching with overlap/range checks, address map, stable fixture IDs, and common generic fixtures.
- [ ] Attribute-aware programmer, selection tools, groups, Intensity/Color/Position/Beam palettes, and undo/redo.
- [ ] Scenes/looks, lightweight GO-driven cue lists, practical chases, attribute-family fades, delay/hold, tap tempo, and explicit release.
- [ ] Deterministic HTP/LTP merge, structured playback banks, keyboard operation, submasters, Grand Master, blackout, and release-all.
- [ ] Conformant Art-Net output isolated from UI/script/AI timing plus fixture/address/source and network-health monitoring.
- [ ] Atomic save/open/save-as, autosave-on-change, rotating recovery revisions, exportable bundle, and tested schema migration.
- [ ] MIDI Note/CC learn, button/fader mappings, and soft takeover; generic operation does not require device-specific feedback.
- [ ] One typed command/state/event model exposed through Wails, a scoped TypeScript runtime/SDK, and a documented versioned API.
- [ ] Hosted/local provider-neutral LLM integration for evidence-backed fixture authoring and bounded show authoring.
- [ ] Separately armed live AI operation with validation, stale-state protection, allowlists, rate/time limits, audit, cancellation, and manual blackout/release priority.

### Add After Validation (v1.x)

- [ ] Native or reference OSC-to-API bridge — add when real controller mappings reveal a stable address surface.
- [ ] Fixture morph/remap assistant — add when operators need to reuse shows across rented or replacement fixtures.
- [ ] Reusable parameter effects (intensity/color/position wave, fan, phase) — add after timing and grouping prove stable.
- [ ] Device-specific MIDI profiles, feedback, SysEx initialization, beat clock, and MIDI Show Control — add only after independent per-device MIDI-HW-02 evidence passes and EXTN-04 is in scope for the relevant validated hardware or synchronization need.
- [ ] Blind/offline editing while live playback continues — add after the live programmer and command revision model are field-tested.
- [ ] Thin remote web/mobile playback client — add through the public API only after authentication and reconnection behavior are proven.
- [ ] QLC+ show/fixture import tools — add based on migration demand; never compromise the canonical GOLC schema.

### Future Consideration (v2+)

- [ ] sACN and selected USB-DMX outputs — only after Art-Net conformance and the protocol boundary are proven.
- [ ] Advanced console semantics — tracking, move-while-dark, multipart cues, cue-only updates, per-channel timing, and sophisticated playback priorities.
- [ ] Timecode, audio-reactive playback, show timelines, and media-server coordination.
- [ ] Pixel/matrix content tools and 2D/3D visualization.
- [ ] Multi-console redundancy, multi-user editing, distributed playback nodes, and enterprise permissions.
- [ ] Marketplace/plugin host or arbitrary native extensions — only after sandboxing, API stability, and compatibility policy mature.

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Fixture schema, library, authoring, and patch | HIGH | HIGH | P1 |
| Programmer, groups, palettes, undo | HIGH | HIGH | P1 |
| Scenes, cue lists, chases, timing | HIGH | HIGH | P1 |
| Deterministic merge and playback | HIGH | HIGH | P1 |
| Masters, blackout, release-all | HIGH | MEDIUM | P1 |
| Art-Net output and live monitoring | HIGH | HIGH | P1 |
| Persistence, autosave, recovery, migration | HIGH | HIGH | P1 |
| Structured playback workflow accelerators | HIGH | MEDIUM | P1 |
| Keyboard and MIDI Note/CC control | HIGH | HIGH | P1 |
| Typed command model and public API | HIGH | HIGH | P1 |
| TypeScript runtime/SDK/debugging | HIGH | HIGH | P1 |
| AI fixture authoring and show authoring | HIGH | HIGH | P1 |
| Bounded, interruptible live AI operation | HIGH | HIGH | P1 |
| OSC bridge/native surface | MEDIUM | MEDIUM | P2 |
| Fixture morph/remap | MEDIUM | HIGH | P2 |
| Parameter effects/fan/phase engine | MEDIUM | HIGH | P2 |
| Remote mobile/web client | MEDIUM | HIGH | P2 |
| QLC+ import | MEDIUM | HIGH | P2 |
| Additional output protocols | MEDIUM | HIGH | P3 |
| Timecode/media/pixel/3D tools | LOW for initial target | HIGH | P3 |
| Multi-console/multi-user operation | LOW for initial target | HIGH | P3 |

**Priority key:**

- P1: Must have for the promised v1.
- P2: Add after conventional workflow and field reliability are validated.
- P3: Future consideration; explicitly outside the small-show v1 proof point.

## Competitor Feature Analysis

| Feature | QLC+ | Professional-console reference (MagicQ / ETC Eos) | GOLC v1 Approach |
|---------|------|----------------------------------------------------|------------------|
| Fixture patch/definitions | Fixture-oriented manager and external fixture editor; modes, groups, and channel properties. | Large personality libraries and optimized patch syntax/workflows. | Guided batch patch with strict overlap validation, GOLC-owned versioned schema, in-app definition authoring, and evidence-backed AI assistance. |
| Programmer | QLC+ v5 edits scenes through fixture/group/palette components and attribute views. | Explicit programmer with selection, priority, levels, timing, FX, record/update, and live override. | Adopt a compact explicit programmer because it shortens select → adjust → record; omit pro command syntax and complex tracking. |
| Groups and palettes | Supported as scene components in v5. | Central reusable Intensity/Color/Beam/Position records, often referenced by cues. | Auto-create useful groups/palettes after patch, keep them visible beside the programmer, and preserve semantic references. |
| Scenes/cues/chases | Rich function taxonomy (Scene, Chaser, Sequence, EFX, Show, Collection, Script). | Unified cue/cue-stack/playback model with deep timing and tracking. | Fewer first-class concepts: Look/Scene, Cue List, Chase. Shared timing and playback semantics reduce navigation and mental overhead. |
| Live playback surface | Highly customizable blank-canvas Virtual Console with buttons, sliders, frames, and cue lists. | Dedicated playback hardware plus configurable execute windows. | Auto-generated structured banks usable immediately, with constrained customization and a separate safe edit mode. |
| Output and monitoring | Plugin-based universes and DMX monitor; MIDI/OSC feedback. | Extensive output/network status and console diagnostics. | Art-Net-only configuration with final-buffer, source-provenance, subscriber/node, rate, lateness, and error visibility from the start. |
| Scripting | Plain one-command-per-line script editor for starting/stopping functions, setting channels, waiting, and random values ([official editor](https://docs.qlcplus.org/v5/function-manager/script-editor)). | Macros and console-specific automation/remote protocols. | Sandboxed TypeScript with typed SDK, events, cancellation, logs/debugging, tests, quotas, and the exact same commands as UI/API/AI. |
| API and agents | External input plugins and software-specific remote capabilities. | Mature but console-specific remote/control protocols. | Public, versioned, documented command/state/event API designed as a product surface; LLM tools are an adapter over it. |
| AI | No official primary-source evidence found for equivalent autonomous authoring/operation. | No official primary-source evidence found for equivalent provider-neutral autonomous authoring/operation. | Evidence-backed fixture authoring plus scoped autonomous show authoring/live control, validated and auditable with immediate manual override. |
| Persistence/recovery | Save/open project baseline. | Autosave, archives, restore, and show-version handling. | Atomic versioned bundle, autosave-on-change, rotating recovery, migration backup, and audit-linked revisions as launch requirements. |

## Delivery Traceability in Linear (Project Tooling, Not a Console Feature)

Linear is required from project start, but it should not appear in the lighting-console UI or public API. Use it as the authoritative delivery index:

| GOLC Planning Artifact | Linear Object | Stable Linkage Rule |
|------------------------|---------------|---------------------|
| GOLC v1 milestone | One Linear Project | Store the Linear project UUID in milestone metadata; do not key automation by title. |
| Roadmap phase | Project Milestone | Store milestone UUID/link on the phase; milestone order mirrors roadmap order. |
| Requirement / acceptance outcome | Issue | Store both immutable model UUID and human identifier (for example `GOLC-123`) in the requirement record. |
| Phase plan or feature slice | Parent issue | Link it to the project milestone and requirement issues it delivers. |
| Executable implementation task | Sub-issue | Inherit project context; close only with verification evidence linked in the issue. |
| Feature dependency | Blocked-by / blocking issue relation | Mirror only actionable delivery dependencies, not every conceptual relationship. |

Linear documents Projects as outcome-oriented work, Project Milestones as lifecycle stages, sub-issues as implementation decomposition, and blocking relations as dependencies ([Projects](https://linear.app/docs/projects), [Project Milestones](https://linear.app/docs/project-milestones), [parent/sub-issues](https://linear.app/docs/parent-and-sub-issues), [issue relations](https://linear.app/docs/issue-relations)). Its API exposes entity IDs and human issue identifiers; model UUIDs can be copied and used by GraphQL integrations ([GraphQL API](https://linear.app/developers/graphql), [SDK data access](https://linear.app/developers/sdk-fetching-and-modifying-data)). Start with explicit IDs in planning files and manual/CLI updates; add webhook synchronization only when bidirectional automation has a clear owner and conflict policy.

## Sources

All confidence tiers below were assigned through the research confidence seam. Official sources fetched by web search were cross-checked and classify as **MEDIUM**; Context7-classified Wails documentation is **MEDIUM**. No LOW-confidence claim is used as an authoritative requirement.

### Lighting Workflow and Competitors

- [QLC+ v5 Glossary and Concepts](https://docs.qlcplus.org/v5/basics/glossary-and-concepts) — official manual; scenes, chasers, sequences, I/O baseline. **MEDIUM**
- [QLC+ Function Manager](https://docs.qlcplus.org/v5/function-manager) and [Scene Editor](https://docs.qlcplus.org/v5/function-manager/scene-editor) — official manual; authoring taxonomy, fixtures/groups/palettes. **MEDIUM**
- [QLC+ Virtual Console](https://docs.qlcplus.org/v4/virtual-console), [Button](https://docs.qlcplus.org/v5/virtual-console/button), and [Slider](https://docs.qlcplus.org/v5/virtual-console/slider) — official manual; live surface, masters, flash, blackout, stop-all, soft takeover. **MEDIUM**
- [QLC+ Input/Output](https://docs.qlcplus.org/v5/input-output), [Input Profiles](https://docs.qlcplus.org/v5/input-output/input-profiles), [MIDI](https://docs.qlcplus.org/v5/plugins/midi), and [DMX Monitor](https://docs.qlcplus.org/v4/main-window/dmx-monitor) — official manual; external control, feedback, universes, monitoring. **MEDIUM**
- [MagicQ Programmer](https://secure.chamsys.co.uk/docs/magicq/programmer/programmer.html), [Palettes](https://secure.chamsys.co.uk/docs/magicq/manual/palletes.html), [Cue Stacks](https://secure.chamsys.co.uk/docs/magicq/manual/cue_stacks.html), [Playback](https://secure.chamsys.co.uk/docs/magicq/manual/playback.html), and [System Management](https://secure.chamsys.co.uk/docs/magicq/manual/system_management.html) — official manual; conventional console model and recovery. **MEDIUM**
- [ETC Eos Saving Show Files](https://www.etcconnect.com/WebDocs/Controls/EosFamilyOnlineHelp/en/Content/05_Show_Files/Saving_Show_Files.htm) and [Master Configuration](https://www.etcconnect.com/WebDocs/Controls/EosFamilyOnlineHelp/en/Content/04_System_Basics/08_Faders/Master_Configuration.htm) — official manual; persistence, masters, blackout baseline. **MEDIUM**

### Standards, Framework, Safety, and Delivery

- [Art-Net official site](https://art-net.org.uk/), [terminology](https://art-net.org.uk/art-net-introduction-and-terminology/), and [Art-Net 4 specification](https://art-net.org.uk/downloads/art-net.pdf) — official protocol source. **MEDIUM**
- [Open Fixture Library fixture format](https://github.com/OpenLightingProject/open-fixture-library/blob/master/docs/fixture-format.md) and [OFL JSON plugin warning](https://open-fixture-library.org/about/plugins/ofl) — official project documentation; semantic schema concepts and compatibility warning. **MEDIUM**
- [MIDI 1.0 Core Specifications](https://midi.org/midi-1-0-core-specifications) and [OSC 1.0 specification](https://opensoundcontrol.stanford.edu/spec-1_0.html) — official protocol sources. **MEDIUM**
- [Wails v2 How It Works](https://wails.io/docs/howdoesitwork) — official framework documentation retrieved via Context7 after resolving `/websites/wails_io`; generated JS/TS bindings and Go-struct models. **MEDIUM**
- [NIST AI RMF Core](https://airc.nist.gov/airmf-resources/airmf/5-sec-core/) and [NIST AI RMF Playbook](https://airc.nist.gov/docs/AI_RMF_Playbook.pdf) — official risk-management guidance; oversight, monitoring, override, incident response, audit. **MEDIUM**
- [OWASP AI Agent Security Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/AI_Agent_Security_Cheat_Sheet.html) — official project guidance; least privilege, structured validation, bounded execution, audit, interrupt/rollback. **MEDIUM**
- [Linear Projects](https://linear.app/docs/projects), [Project Milestones](https://linear.app/docs/project-milestones), [issue relations](https://linear.app/docs/issue-relations), and [GraphQL API](https://linear.app/developers/graphql) — official product/developer documentation; delivery hierarchy and stable IDs. **MEDIUM**

## Open Validation Questions

- How many universes, fixtures, simultaneous playbacks, and MIDI controls define the actual first-user ceiling? Do not choose pro-console scale without field data.
- Do first users need a GO-driven cue list at launch, or will scenes/chases plus execute banks cover almost all shows? The research recommendation is to keep one lightweight cue-list type because omitting it blocks simple theatre/church workflows.
- Akai MIDImix, Novation Launch Control XL Mk2, and Worlde EasyControl 9 are the selected Phase 6 physical acceptance set; which capabilities can each device independently substantiate through MIDI-HW-02 for its exact hardware revision, firmware, Windows version, and GOLC build beyond generic Note/CC learn and soft takeover?
- Does live AI need direct cue triggering in the first public build, or can launch validation begin with autonomous offline authoring plus operator-started playback? The project intent allows live autonomy, but staged exposure substantially reduces field risk.
- What legal/safety restrictions should apply to fixture capabilities representing lasers, fog/haze, relays, or other non-lighting loads?
- Which OFL snapshot/import policy and licensing notices should ship with the starter fixture library?

---
*Feature research for: GOLC small-show lighting console*
*Researched: 2026-07-17*
