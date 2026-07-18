# Pitfalls Research

**Domain:** Cross-platform live-lighting control (Go/Wails, Art-Net, TypeScript scripting, public API, autonomous LLM control)
**Researched:** 2026-07-17
**Confidence:** MEDIUM — current official specifications and project documentation were used; several mitigations are engineering inferences that must be proven against real Art-Net nodes and fixtures.

## Roadmap Phase Vocabulary

This document uses phase topics rather than assuming final roadmap numbering:

| Phase topic | Required outcome |
|-------------|------------------|
| **Foundation — Domain and Command Kernel** | Typed fixture/patch/playback model, single mutation path, state revisions, actor identity, audit primitives |
| **Protocol — Deterministic Playback and Art-Net** | Isolated frame engine, scheduler, Art-Net discovery/transmission, network selection, observability, virtual receiver |
| **Fixtures — Definitions and Patching** | Validated internal fixture schema, authoring/import, provenance, semantic attribute mapping, golden fixture corpus |
| **Workflow — Show Authoring, Playback, Persistence, and Wails UI** | Scenes/chases, deterministic arbitration, crash-safe show storage, fast conventional UI |
| **API — Versioned External Control** | Contract-first public API over the shared command kernel, concurrency and retry semantics |
| **Scripting — TypeScript Runtime** | Compilation/debugging plus capability, time, queue, and resource boundaries |
| **AI — Provider-Neutral LLM Control** | Read/plan/validate/execute loop, stale-state rejection, audit, policy limits, immediate operator override |
| **Release — Native Packaging and System Validation** | Native Windows/macOS/Linux artifacts, installation/signing checks, physical-rig soak and recovery testing |
| **Governance — Repository/Linear Traceability** | Offline-complete repository planning with durable, reconciled Linear links from project inception |

## Release-Blocker Summary

The following are blockers for the first release containing the affected surface, not optional polish:

- The output loop owns immutable frames and continues safely if the UI reloads, a script loops, an API client retries, persistence stalls, or an LLM call hangs.
- ArtDmx packet bytes, sequence behavior, payload length, Port-Address mapping, refresh/keepalive policy, interface selection, discovery, and unicast targeting pass automated and physical-node tests.
- Fixture imports reject unsupported or ambiguous semantics rather than silently flattening them; shows pin the exact validated fixture-definition revision they used.
- Concurrent cues and commands have one documented arbitration policy, one state revision order, and deterministic fake-clock tests.
- Saves and migrations are transactional, backed up through a SQLite-aware mechanism, recoverable after forced termination, and tested with a SQLite version containing the WAL-reset fix if WAL is used.
- Public API, scripts, and AI cannot bypass command validation, directly write DMX, or replay destructive commands accidentally.
- Every autonomous AI action is attributable, bounded, based on a state revision, and interruptible by an operator control that does not depend on the model or UI event queue.
- Each supported OS is built, installed, launched, saved/restored, and connected to a test receiver on that OS.

Later hardening may broaden fixture-format coverage, add optional ArtSync workflows, isolate scripts in a helper process, expose authenticated LAN APIs, and automate bidirectional Linear synchronization. The v1 implementation must explicitly reject unsupported cases; it must not approximate them silently.

## Critical Pitfalls

### Pitfall 1: Treating Wails or the UI as the Playback Engine

**What goes wrong:**
Frame calculation or Art-Net transmission runs in frontend timers, Wails event callbacks, bound UI methods, or a shared queue that the frontend can congest. Rendering, DOM reload, a modal dialog, large state serialization, or a panic then causes visible pauses or stale output.

**Why it happens:**
The UI is the first working control surface, so it becomes the accidental owner of state and time. Wails makes Go methods and cross-runtime events convenient, which can hide the fact that frontend readiness and event delivery are presentation concerns. Wails distinguishes `OnStartup` from `OnDomReady`, and its runtime calls use the application context; this is evidence that lifecycle/UI adapters should not define show timing. [Wails lifecycle and bindings](https://wails.io/docs/howdoesitwork), [Wails runtime introduction](https://wails.io/docs/reference/runtime/intro). **Confidence: MEDIUM.**

**How to avoid:**
Create a headless Go engine during startup. Give one frame loop sole ownership of the current output snapshot and UDP transmitter. UI, scripts, API, and AI submit typed commands to a bounded command processor; they receive snapshots/events but never own the clock or mutable frame buffer. Coalesce presentation updates so a slow frontend cannot backpressure output. Define shutdown, suspend/resume, and output-failure policies explicitly (hold last safe frame, blackout, or stop) and test each.

**Warning signs:**
Art-Net stops when DevTools pauses; `runtime.EventsEmit` appears inside the timing loop; frame state is mutated by several Wails-bound methods; closing/reloading the frontend changes playback; output jitter tracks frontend FPS; queues grow when the UI is hidden.

**Recovery:**
Extract a headless engine interface, replace direct mutations with commands, make UI state a read-only projection, and add a load test that reloads/hangs the DOM while recording receiver timestamps.

**Phase to address:**
**Foundation — Domain and Command Kernel**, enforced by **Protocol — Deterministic Playback and Art-Net**. **Release blocker.**

---

### Pitfall 2: Using a Convenience Ticker as a Real-Time Contract

**What goes wrong:**
Cues drift, chases compress or skip unpredictably, simultaneous universes tear, and a slow computation causes bursts of late frames. A queue of every overdue frame increases latency rather than recovering.

**Why it happens:**
Go provides excellent concurrency but is not a hard-real-time scheduler. The standard ticker may adjust its interval or drop ticks when the receiver is slow, and wall time can change independently from the monotonic clock. [Go `time` documentation](https://pkg.go.dev/time). **Confidence: MEDIUM.**

**How to avoid:**
Use an injected monotonic clock and absolute cue start/deadline calculations. At each frame boundary, compute the state that should exist *now*; do not replay an unbounded backlog of historical frames. Publish immutable per-universe snapshots to the transmitter. Set a deliberate output cadence no higher than the discovered gateway limit, measure calculation/send duration and scheduling lateness, and shed nonessential telemetry before compromising output. Run soak tests under UI churn, fixture import, save/checkpoint, script timeout, API bursts, GC pressure, and slow LLM inference.

**Warning signs:**
Tests use real sleeps; fades are implemented as repeated incremental additions; an unbounded channel stores frames; frame rate is asserted only as an average; no p95/p99/max jitter or missed-deadline metric exists; pausing one consumer makes the entire engine late.

**Recovery:**
Replace incremental transitions with functions of monotonic elapsed time, bound and coalesce queues, add a fake clock, then replay captured command timelines and compare frame hashes and timestamps.

**Phase to address:**
**Protocol — Deterministic Playback and Art-Net**. **Release blocker.**

---

### Pitfall 3: Implementing “UDP with DMX Bytes” Instead of Art-Net 4

**What goes wrong:**
Packets work with one simulator but are ignored, reordered, merged unexpectedly, rate-limited, or held waiting for ArtSync by real nodes. Off-by-one universe mappings or malformed lengths control the wrong output.

**Why it happens:**
The happy-path ArtDmx packet is small, but the protocol contract is precise. The current Art-Net 4 specification defines a 15-bit Port-Address, Sequence values 1–255 with zero disabling resequencing, an even DMX data length from 2–512, gateway-advertised refresh limits (44 Hz for DMX512 gateways), subscriber unicast, and an approximately 800–1000 ms recommended retransmission for unchanged active data. ArtSync moves a node into buffered synchronous operation until another sync or a four-second timeout. [Official Art-Net 4 specification](https://art-net.org.uk/downloads/art-net.pdf). **Confidence: MEDIUM.**

**How to avoid:**
Model packet fields explicitly and maintain sequence state per transmitted stream/universe, wrapping 255 to 1. Validate and pad payload lengths correctly. Discover node subscriptions and refresh capability with ArtPoll/ArtPollReply; send ArtDmx by unicast under the current specification. Keep ArtSync off unless the product deliberately supports and tests multi-universe synchronous output. Treat merge indication or another controller on the same Port-Address as an operator-visible conflict. Build byte-level golden packets and parse captures in tests.

**Warning signs:**
All output targets `255.255.255.255`; Sequence is constant except zero by accident; universe is a single byte; payload can be odd or longer than 512; no ArtPoll state exists; frame rate is hard-coded above 44 Hz; ArtSync packets are emitted without a receiver-state model; tests compare only “lights changed.”

**Recovery:**
Freeze higher-level work, add a protocol encoder/decoder corpus from the specification, capture packets with Wireshark, run the official Art-Net conformance tooling where applicable, and smoke-test at least two node implementations.

**Phase to address:**
**Protocol — Deterministic Playback and Art-Net**. **Release blocker.**

---

### Pitfall 4: Guessing the Network Interface or Broadcast Address

**What goes wrong:**
Output leaves through a VPN, Wi-Fi, Docker/Hyper-V adapter, or loopback interface; discovery works on one adapter while ArtDmx leaves another; DHCP changes or unplug/replug silently strand the show.

**Why it happens:**
Desktop machines commonly have several active interfaces. Go's `InterfaceAddrs` returns unicast addresses without the associated interface, while `Interfaces` plus `Interface.Addrs` retains the relationship and exposes flags such as up, running, broadcast, and loopback. [Go network-interface source/documentation](https://go.dev/src/net/interface.go), [Go `net` package](https://pkg.go.dev/net). The Art-Net specification distinguishes directed broadcast discovery from ArtDmx subscription/unicast; limited broadcast is not a safe catch-all. [Art-Net casting guidance](https://art-net.org.uk/casting/). **Confidence: MEDIUM.**

**How to avoid:**
Represent output endpoints as `{interface ID, local IPv4/prefix, node IP, Port-Address}`. Enumerate up/running non-loopback IPv4 addresses, show the operator adapter name/IP/subnet, and persist a stable preference with a safe fallback prompt. Derive any directed broadcast from that address and mask only for packet types that allow it. Rediscover on interface/address change, expose last successful send and last node reply, set UDP write deadlines, and allow a deliberate manual node target.

**Warning signs:**
Code picks the first non-loopback address; destination is always `255.255.255.255`; socket local address is unspecified with no route verification; adapter identity is absent from saved settings and diagnostics; VPN enablement changes behavior; link changes require an app restart.

**Recovery:**
Stop transmission on ambiguous route changes, retain show playback state, prompt for an interface/node, reopen the socket bound to the selected local address, rediscover, and resume only after an observable receiver handshake or operator override.

**Phase to address:**
**Protocol — Deterministic Playback and Art-Net**. **Release blocker.**

---

### Pitfall 5: Mixing Display Universes, Art-Net Port-Addresses, and DMX Slots

**What goes wrong:**
A fixture patched at “Universe 1, Address 1” emits on Port-Address 0 or slot 2; fixtures crossing slot 512 overlap; an API client and UI refer to different universes; imported shows shift every patch.

**Why it happens:**
Operator-facing conventions are often 1-based, code arrays are 0-based, and Art-Net defines a 15-bit Net/Sub-Net/Universe Port-Address. The current specification deprecates Port-Address zero while many legacy tools historically displayed Art-Net universe zero. [Art-Net glossary](https://art-net.org.uk/art-net-glossary/), [official Art-Net specification](https://art-net.org.uk/downloads/art-net.pdf). **Confidence: MEDIUM.**

**How to avoid:**
Define distinct types for `PortAddress`, displayed universe label, and `DMXSlot` (1–512). Choose one canonical serialized/API representation and convert exactly once at adapters. Reject patches whose footprint exceeds a universe unless the fixture schema explicitly describes multiple DMX breaks. Return both machine value and display label in diagnostics. Test first/last Port-Address, slots 1/512, adjacent fixtures, fine channels, and multi-break rejection.

**Warning signs:**
Raw integers named `universe` cross all layers; scattered `+1`/`-1`; channel 0 appears in API payloads; the same fixture has different start addresses after save/load; boundary fixtures are missing from tests.

**Recovery:**
Version the persisted/API address model, write a one-time migration with before/after patch diff, require operator confirmation when ambiguity exists, and preserve an untouched backup.

**Phase to address:**
**Foundation — Domain and Command Kernel**, verified in **Fixtures — Definitions and Patching** and **Protocol — Deterministic Playback and Art-Net**. **Release blocker.**

---

### Pitfall 6: Treating Fixture Definitions as Flat Channel Name Lists

**What goes wrong:**
Fine channels are reversed or assumed adjacent; firmware-specific modes share the wrong footprint; split/switching channels expose impossible controls; matrices are scrambled; control/reset ranges are faded through; unsupported definitions appear to import successfully but operate incorrectly.

**Why it happens:**
Real fixture formats contain more structure than channel labels. GDTF documents multiple firmware-linked modes, coarse/fine/ultra/uber offsets, defaults/highlights, DMX breaks, logical/channel-function splits, mode-master dependencies, virtual channels, and geometry relationships. [GDTF DMX channel guidance](https://gdtf-share.com/help/users/gdtf_builder/dmx/index.html), [GDTF mode dependencies](https://gdtf-share.com/help/users/gdtf_howto/handle_mode_dependencies/index.html), [GDTF virtual channels](https://gdtf-share.com/help/users/gdtf_howto/virtual_channels/index.html). **Confidence: MEDIUM.**

**How to avoid:**
Design a versioned internal fixture schema before an editor or LLM author. Validate unique modes, footprint, address bounds, complete/nonoverlapping DMX ranges, fine-channel ordering, defaults, discrete versus continuous capabilities, dependencies, and maintenance/reset ranges. Preserve unsupported semantics as explicit errors, not generic channels. Build golden fixtures covering dimmer, RGB(W/A/UV), 16-bit moving head, split channel, mode dependency, multi-cell matrix, nonadjacent fine channel, and multi-break fixture.

**Warning signs:**
The model is `[]Channel{name}`; every capability is linearly interpolated; import logs warnings but still marks a definition usable; fine channel equals coarse+1 by assumption; no firmware/mode identity is saved; reset/control ranges are indistinguishable from looks.

**Recovery:**
Quarantine affected definitions, mark dependent shows “profile review required,” re-import through the versioned validator, and show an operator-readable channel/footprint diff before repatching.

**Phase to address:**
**Fixtures — Definitions and Patching**. **Release blocker for supported definitions; broader format coverage is later hardening.**

---

### Pitfall 7: Collapsing Pan/Tilt and Color Into Generic Percentages

**What goes wrong:**
Moving heads take long-way rotations, point differently after a profile update, snap at wrap boundaries, or use the wrong home. RGBW/amber/UV/CCT fixtures produce inconsistent colors, virtual dimmers fail, and discrete color-wheel slots are interpolated.

**Why it happens:**
DMX slots are transport values, not complete physical semantics. GDTF associates Pan/Tilt with physical ranges/defaults and geometry, supports virtual dimmers and relations, and separates logical/channel functions. [GDTF virtual Pan/Tilt and dimmer guidance](https://gdtf-share.com/help/users/gdtf_howto/virtual_channels/index.html). OFL examples show 16/24-bit fine relationships and physical ranges, underscoring that raw 8-bit percentages are insufficient. [OFL generic fine-channel example](https://open-fixture-library.org/generic/desk-channel). **Confidence: MEDIUM.**

**How to avoid:**
Use typed logical attributes with units and semantics: physical angle/range/home/inversion for position; continuous additive/subtractive emitters, CCT, and discrete wheel slots for color; explicit intensity mastering. Define shortest-path/wrap behavior per attribute and fixture capability. Keep a raw-channel escape hatch clearly separate from semantic programming. Pin the definition hash used by a show so a library update never changes live behavior silently.

**Warning signs:**
All parameters are floats 0–1; Pan and Tilt share generic fade code with gobo or reset channels; RGBW converts by merely copying RGB and leaving W unchanged; no home/default calibration exists; a fixture-library update changes old scene output without a migration diff.

**Recovery:**
Restore the pinned definition, add per-fixture calibration overrides, migrate stored attributes with a previewable scene diff, and require a physical focus/color smoke test before accepting the new profile.

**Phase to address:**
**Fixtures — Definitions and Patching**, exercised in **Workflow — Show Authoring, Playback, Persistence, and Wails UI**. **Release blocker for declared semantic attributes.**

---

### Pitfall 8: Leaving Concurrent Cue and Command Arbitration Undefined

**What goes wrong:**
A chase, scene fade, fader, script, API client, and LLM write the same attribute concurrently. Outcomes depend on goroutine timing; stopping one playback restores an obsolete value; blackout loses to a late queued command.

**Why it happens:**
Single-control demos avoid contention, while the product explicitly has several first-class control surfaces. Data races are runtime-dependent; Go's race detector only finds races in paths actually executed. [Go race detector](https://go.dev/doc/articles/race_detector). **Confidence: MEDIUM.**

**How to avoid:**
Route every mutation through one serialized command processor and one playback arbiter. Define source/layer priority, per-attribute merge policy, fade interruption rules, ownership/release behavior, and a highest-priority operator override/blackout. Commands carry actor, command ID, expected state revision, and deadline. Use immutable snapshots and deterministic fake-clock scenario tests for simultaneous starts/stops, mid-fade edits, chase overlap, stale command rejection, blackout, and resume.

**Warning signs:**
Several goroutines mutate fixture values; `go test -race` is clean only because concurrency scenarios are absent; last-writer-wins means wall-clock arrival order; stopping a cue recomputes from an old captured base; blackout travels through the normal backlog.

**Recovery:**
Disable autonomous sources, cancel queued work, establish an operator-owned safe snapshot, export the audit timeline, then reproduce it through the deterministic arbiter before re-enabling automation.

**Phase to address:**
**Foundation — Domain and Command Kernel** and **Workflow — Show Authoring, Playback, Persistence, and Wails UI**. **Release blocker.**

---

### Pitfall 9: Assuming SQLite Automatically Solves Save, Migration, and Recovery

**What goes wrong:**
A crash leaves a show half-migrated; copying only the main database omits committed WAL content; an older application opens and rewrites a newer schema; corruption is detected only on show day; auto-save overwrites the last recoverable copy.

**Why it happens:**
SQLite transactions are robust only when used correctly. Its WAL file is part of persistent state and must stay with the database; the online backup API or `VACUUM INTO` creates a consistent live snapshot, while raw copying can produce a corrupt backup. `integrity_check` and `foreign_key_check` cover different conditions. Current official documentation also reports a rare WAL-reset corruption race fixed in SQLite 3.51.3 and selected backports. [SQLite corruption guidance](https://www.sqlite.org/howtocorrupt.html), [SQLite backup API](https://www.sqlite.org/backup.html), [SQLite WAL documentation](https://www.sqlite.org/wal.html), [SQLite pragmas](https://sqlite.org/pragma.html). **Confidence: MEDIUM.**

**How to avoid:**
Use one explicit storage service and transactional migrations that update schema/user version in the same transaction. Before migration, make a SQLite-aware timestamped backup and verify it opens. Refuse newer schemas read-write. If WAL is enabled, pin a SQLite build with the WAL-reset fix and control connection/checkpoint ownership. On startup, run quick structural checks; provide a manual full integrity check, backup/export, and recovery mode. Fuzz deserialization and force-kill tests at every save/migration boundary.

**Warning signs:**
Backups use filesystem copy while the DB is open; migration steps run outside a transaction; no schema compatibility policy exists; several subsystems open independent writers/checkpointers; application bundles SQLite 3.51.2 or older unfixed WAL code; save tests only cover graceful close.

**Recovery:**
Open the damaged file read-only, preserve the original and WAL sidecars, try the latest verified backup first, use SQLite recovery/export tooling only on a copy, and import recovered logical show data into a fresh current-schema file. Never “repair” the sole copy in place.

**Phase to address:**
**Workflow — Show Authoring, Playback, Persistence, and Wails UI**. **Release blocker.**

---

### Pitfall 10: Calling an Embedded JavaScript Engine a TypeScript Sandbox

**What goes wrong:**
An infinite loop consumes CPU, allocations exhaust the process, a script blocks in a native Go callback that cannot be interrupted, two goroutines race one runtime, or an accidentally exposed host function reaches filesystem/network/process capabilities. Playback suffers even though the script “has a timeout.”

**Why it happens:**
TypeScript must be compiled to JavaScript, and safe execution is a host responsibility. A goja runtime is not goroutine-safe; the host supplies event-loop/timer behavior; `Interrupt` stops JavaScript but not native Go functions, and runtime reuse after interruption requires synchronized `ClearInterrupt`. [goja package documentation](https://pkg.go.dev/github.com/dop251/goja). **Confidence: MEDIUM.**

**How to avoid:**
Separate compilation/type-checking/source maps from execution. Use one runtime owner per script, expose a small capability API that only submits typed commands, and omit ambient filesystem/network/process access. Put deadlines and cancellation on every host call, bound command/event queues and output size, rate-limit commands, and disable a script after repeated budget violations. Never give a script the mutable frame buffer or UDP socket. Research helper-process isolation before claiming protection from deliberately hostile code or memory exhaustion.

**Warning signs:**
The runtime is stored globally and used by multiple goroutines; TypeScript is passed directly to `RunString`; timeout tests cover only `while(true)` and not blocking Go callbacks; host functions return unbounded data; scripts can call playback internals; no source-map stack trace or deterministic test clock exists.

**Recovery:**
Atomically revoke the script's capabilities, interrupt JavaScript, cancel cooperative host calls, drop its queued commands, preserve its logs/source/version, and keep the frame engine running from the last validated state. Restart a fresh runtime rather than reuse uncertain state.

**Phase to address:**
**Scripting — TypeScript Runtime**, built on the **Foundation** command boundary. **Release blocker for scripting.**

---

### Pitfall 11: Publishing an API That Is Merely Another Race Into Mutable State

**What goes wrong:**
Retries duplicate “start chase,” “add fixture,” or “advance cue”; two clients overwrite each other; API behavior differs from UI/script behavior; later contract cleanup breaks integrations; a network-exposed local API has no authentication boundary.

**Why it happens:**
HTTP does not make arbitrary actions idempotent: PUT/DELETE and safe methods have idempotent semantics, while POST needs an application-level repeatability mechanism. Conditional requests/ETags are established optimistic-concurrency tools. [RFC 9110 HTTP semantics](https://www.rfc-editor.org/rfc/rfc9110.html), [Microsoft API repeatability and concurrency guidance](https://github.com/microsoft/api-guidelines/blob/vNext/azure/Guidelines.md). **Confidence: MEDIUM.**

**How to avoid:**
Version the API before public use and generate documentation/clients from a checked contract. Map every mutation to the shared command model. Require a client command/idempotency UUID, store the result for deduplication, and reject same ID/different payload. Require an expected state/resource revision for conflicting edits and return a structured conflict containing current revision. Serialize accepted commands, return command/result/audit IDs, bind loopback by default, and require explicit authenticated configuration before LAN exposure.

**Warning signs:**
Handlers call repositories or playback objects directly; retrying a timed-out request advances twice; no command ID or expected revision exists; endpoints return success before validation; UI has operations absent from the API command types; binding defaults to all interfaces.

**Recovery:**
Disable remote writes, reconcile audit entries by command ID, expose current authoritative state, provide a migration/deprecation window for clients, and repair duplicated resources through idempotent administrative commands rather than database edits.

**Phase to address:**
**API — Versioned External Control**, with invariants established in **Foundation**. **Release blocker for public API.**

---

### Pitfall 12: Letting the LLM Interpret and Execute in One Step

**What goes wrong:**
The model invents fixture IDs, acts on stale state after inference, repeats a destructive tool after timeout, floods the command queue, triggers maintenance/reset ranges, or continues changing lights after the operator attempts to stop it.

**Why it happens:**
Provider-neutral wrappers expose model-selected tool calls and model-generated arguments; they do not make those arguments true, current, authorized, or idempotent. LangChainGo's tool flow still requires application code to parse and execute arguments, and local-provider compatibility is only partial across state/tool features. [LangChainGo repository](https://github.com/tmc/langchaingo/), [Ollama tool calling](https://docs.ollama.com/capabilities/tool-calling), [Ollama OpenAI-compatibility scope](https://docs.ollama.com/api/openai-compatibility). **Confidence: MEDIUM.**

**How to avoid:**
Split autonomy into inspect → propose → deterministic validate → execute → observe. Give state snapshots revision IDs and reject/replan stale mutations. Put every tool call through the same schema validation, authorization, idempotency, concurrency, and audit path as the API. Define live-mode limits (allowed fixtures/attributes, max delta, max batch, rate, duration, forbidden maintenance ranges) and a dry-run/diff view. Make operator override a high-priority engine command that revokes the AI session and drops pending AI commands without waiting for the model, network, or normal UI queue.

**Warning signs:**
Tools invoke domain services directly; state has no revision; retry policy repeats mutations blindly; the audit stores only natural-language summaries; autonomous mode has no visible owner/session/expiry; “stop AI” sends another model message; prompts or logs contain provider secrets.

**Recovery:**
Revoke the AI capability token/session, cancel provider calls, discard uncommitted AI commands, restore an operator-selected safe scene or checkpoint, preserve a redacted prompt/tool/validation/result audit, and replay the incident against a simulator before re-enabling autonomy.

**Phase to address:**
**AI — Provider-Neutral LLM Control**, after **API** and **Scripting** command boundaries are proven. **Release blocker for autonomous mode.**

---

### Pitfall 13: Trusting Fixture Definitions Because They Came From a Library or an LLM

**What goes wrong:**
A community definition, stale manual, or plausible LLM output maps reset or movement ranges incorrectly. Updating the library silently changes an existing show. The operator cannot determine who authored a mapping or what manual page supports it.

**Why it happens:**
OFL is collaborative and explicitly warns that its internal JSON was not designed as a stable application interchange format and may break compatibility. [OFL JSON plugin warning](https://open-fixture-library.org/about/plugins/ofl). GDTF Share maintains revisions, reinforcing that definition identity is versioned rather than merely manufacturer/model text. [GDTF revision behavior](https://gdtf-share.com/help/developers/gdtf_1_2/changes/version-1.0/index.html). **Confidence: MEDIUM.**

**How to avoid:**
Store source URL/manual, manufacturer/model, firmware/mode, author/generator, imported schema version, retrieval time, license, content hash, validation result, and operator verification status. Require LLM-authored definitions to attach source excerpts/page references and confidence per ambiguous mapping; treat them as drafts until schema validation and a channel-by-channel output test pass. Snapshot/pin the exact normalized definition in each show. Updates require a semantic diff of footprint, defaults, ranges, and attributes.

**Warning signs:**
Definition identity is only a display name; imports auto-update in place; no manual/source field exists; an LLM answer becomes “verified” after JSON-schema validation alone; reset/control capabilities are not flagged; show files depend on a live remote library.

**Recovery:**
Restore the pinned definition, quarantine the suspect revision, compare it with the physical fixture/manual at low-risk output levels, publish a corrected revision with provenance, and migrate affected shows only after an operator-visible diff.

**Phase to address:**
**Fixtures — Definitions and Patching**, extended in **AI — Provider-Neutral LLM Control**. **Release blocker for fixture authoring/import.**

---

### Pitfall 14: Treating Cross-Platform Compilation as Cross-Platform Readiness

**What goes wrong:**
The binary builds but fails to install, launch, render, access network adapters, persist data, resume after sleep, or locate its web runtime on one OS. Signing/notarization or Linux WebKit differences are discovered at release time.

**Why it happens:**
Wails v2 has platform-specific dependencies: Windows uses WebView2, macOS requires native tooling, and Linux requires GTK/WebKit packages with distribution/version differences. Official guidance builds each target on its operating system. [Wails installation requirements](https://wails.io/docs/v2.12.0/gettingstarted/installation/), [Wails cross-platform builds](https://wails.io/docs/guides/crossplatform-build/). **Confidence: MEDIUM.**

**How to avoid:**
Pin the Wails major/minor line; do not mix v2 and v3 instructions. Establish native CI early for supported OS/architectures. On each target, install the produced artifact in a clean environment; launch, select an adapter, transmit to a receiver, save/reopen/migrate a show, exercise suspend/resume where supported, and collect logs. Add signing/notarization, WebView bootstrap, Linux dependency declarations, and upgrade tests before release candidates.

**Warning signs:**
Only the developer OS runs tests; CI cross-compiles without installing; Wails docs/code from different major versions are mixed; packaging has no clean-machine test; paths and line endings leak into show behavior; network tests use only loopback.

**Recovery:**
Keep the prior signed artifact available, isolate platform adapters behind stable interfaces, reproduce on the oldest supported clean OS image, and block release promotion for the failing target rather than claiming partial support.

**Phase to address:**
Native CI begins in **Foundation**; ownership and exit gate are **Release — Native Packaging and System Validation**. **Release blocker.**

---

### Pitfall 15: Building Without a Virtual Receiver and Physical Test Rig

**What goes wrong:**
Unit tests prove interpolation math while packet loss, interface changes, sequence wrap, node disappearance, output jitter, fixture semantics, and blackout priority remain untested. The first realistic integration test happens during a show.

**Why it happens:**
Lighting behavior is observable only at the receiver/fixture boundary. Art-Net's official site provides a conformance tester and recommends protocol diagnostics such as Wireshark, which indicates that packet-level verification is part of implementation, not post-release QA. [Official Art-Net resources](https://art-net.org.uk/). **Confidence: MEDIUM.**

**How to avoid:**
In the protocol phase, create an in-process/loopback virtual Art-Net node that records packet bytes, source/destination, universe, sequence, timing, and freshness; simulate loss, reordering, duplicates, merge, subscription changes, slow receivers, interface loss, and optional ArtSync. Maintain captured golden timelines. Add at least two real node implementations and representative dimmer/RGB/moving-head fixtures to a low-risk physical smoke/soak procedure. Run race tests and fuzz packet, fixture, API, and persistence parsers.

**Warning signs:**
“UDP write returned nil” is the integration assertion; no receiver timestamps are captured; manual tests have no repeatable script; sequence wrap and adapter changes are untested; simulator and encoder share the same parser/assumptions; no real node is in release qualification.

**Recovery:**
Stop feature expansion, add an independent receiver/capture parser, reproduce reported output as a command timeline, convert the incident into a regression capture, and qualify the fix on both virtual and physical rigs.

**Phase to address:**
Starts in **Protocol — Deterministic Playback and Art-Net**, expands through every later surface, and gates **Release**. **Release blocker.**

---

### Pitfall 16: Making Linear Availability Part of Repository Truth

**What goes wrong:**
Planning or builds fail offline; issue renames break links; webhooks are missed; duplicate issues appear after retry; roadmap phase and delivered code drift with no reconciliation path.

**Why it happens:**
Linear's API is cursor-paginated and rate/complexity limited; polling is discouraged. Webhooks retry failed deliveries only three times and may be disabled, so webhook delivery alone is not a durable ledger. Linear exposes delivery UUIDs and entity UUIDs useful for deduplication/stable mapping. [Linear rate limits](https://linear.app/developers/rate-limiting), [Linear pagination](https://linear.app/developers/pagination), [Linear webhooks](https://linear.app/developers/webhooks). **Confidence: MEDIUM.**

**How to avoid:**
Keep complete planning intent and stable local IDs in repository artifacts. Store Linear UUID plus human identifier/URL as metadata, never infer identity from title. Make sync retryable/idempotent and nonblocking; queue changes offline. Deduplicate webhook deliveries, verify signatures if hosting a receiver, and periodically reconcile by `updatedAt` cursor to recover missed events. Add a traceability audit that reports missing, duplicate, closed-too-early, and phase/status mismatches without preventing local planning or builds.

**Warning signs:**
Repository files contain only Linear URLs with no local requirement/phase identity; scripts require network access to read the roadmap; titles are sync keys; every run creates issues; webhook success is assumed permanent; rate-limit responses lose changes.

**Recovery:**
Treat repository artifacts as the recovery source, export the local mapping table, fetch Linear incrementally with backoff, match by stored UUID, produce a dry-run reconciliation report, and require human choice for genuine conflicts before updating either side.

**Phase to address:**
**Governance — Repository/Linear Traceability** from inception and at every phase transition. **Governance gate, not a runtime dependency.**

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| UI calls engine internals directly | Fast demo | Nondeterministic output and divergent control surfaces | Never |
| One raw integer for universe/address/slot | Less type code | Off-by-one patches and incompatible API/save formats | Never |
| Import fixture with warnings and “best effort” flattening | More fixtures appear supported | Dangerous false confidence | Never; reject unsupported semantics |
| Store only raw DMX scene frames | Simple playback | Cannot safely edit semantic attributes or migrate definitions | Only for capture/debug artifacts, not canonical authoring |
| Store only logical attributes without pinned definition | Small show files | Library updates alter output | Never |
| Reuse one goja runtime across scripts/goroutines | Lower setup cost | Races and cross-script contamination | Never |
| Add API after UI by wrapping handlers | Quick exposure | Contract instability and bypassed validation | Never; API adapters may come later, command model may not |
| Raw filesystem copy of live SQLite file | Easy backup | Inconsistent/corrupt backups, especially with WAL | Never while open; use SQLite-aware backup |
| Simulator-only protocol signoff | Cheap CI | Real-node interoperability failures | Acceptable during early development, never for release |
| Linear title as identity | Human readable | Rename/duplicate drift | Never; persist UUID and local ID |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Art-Net nodes | Broadcast ArtDmx everywhere and skip discovery | Discover subscriptions, retain adapter identity, unicast ArtDmx, test legacy-node compatibility explicitly |
| Wails | Start/drive playback from DOM lifecycle or events | Start headless engine in Go; UI is command/snapshot adapter |
| goja/TypeScript | Assume JS interrupt is a full sandbox | Compile/type-check separately; one runtime owner; capability API; cooperative native cancellation; bounded resources |
| LangChainGo/hosted models/Ollama | Normalize provider responses and execute tool args directly | Provider adapter returns proposals; deterministic command validator owns execution and policy |
| SQLite | Multiple uncontrolled writers/checkpointers plus file-copy backups | One storage service, fixed SQLite version, transactional migrations, online backup, integrity/recovery workflow |
| Fixture libraries | Consume OFL internal JSON as a forever-stable runtime format | Normalize through a versioned importer and pin source/schema/content hash |
| Linear | Poll everything or trust webhooks as complete | Offline queue, UUID mapping, webhook dedupe, incremental reconciliation, rate-limit backoff |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Emit full application state to UI every frame | GC spikes and UI lag correlate with Art-Net jitter | Coalesced/delta presentation snapshots at a lower rate | Surprisingly early with dozens of fixtures |
| One goroutine/timer per attribute or fade | Timer/goroutine growth and nondeterministic overlap | One scheduler/arbiter computes all active transitions from absolute time | Complex scenes/chases or repeated start/stop |
| Queue every missed frame | Rising latency after a stall | Replace overdue frames with the current immutable snapshot | Any receiver or calculation pause |
| Synchronous save/import/audit on output owner | Visible freeze during I/O | Snapshot then persist/validate off-loop; commit results through commands | Large shows, slow disks, integrity checks |
| Broadcast or duplicate ArtDmx streams | Network saturation and node merge indicators | Subscription-aware unicast and single transmitter ownership | Multi-universe rigs or multiple adapters/controllers |
| Send every command/event to scripts and LLM | Backlog, token cost, stale actions | Capability-scoped filtered events, coalescing, rate and batch limits | Fast chases/fader motion immediately |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Treat fixture/manual/import data as trusted | Malformed data, unsafe maintenance ranges, path/archive abuse | Size/path limits, schema+semantic validation, quarantine, provenance, safe extraction |
| Expose Go functions broadly to scripts | Filesystem/network/process abuse or uninterruptible native calls | Capability allowlist and cancellable typed command calls only |
| Bind public API to all interfaces without auth | Anyone on venue LAN can control the show | Loopback default; explicit authenticated LAN mode; least-privilege scopes |
| Log provider keys or full sensitive prompts | Credential disclosure | Secret references, redaction, structured audit with protected storage |
| Let AI or scripts write DMX directly | Bypasses validation, audit, override and arbitration | Only the isolated engine/transmitter writes frames/UDP |
| Put blackout/override in normal backlog | Late automation defeats emergency action | Dedicated highest-priority bounded path in the engine |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Hide adapter and target selection | “Output active” while packets leave wrong NIC | Always show adapter/IP, node, universe subscription, last send/reply, and errors |
| Ambiguous universe numbering | Mispatches and wasted setup time | Display convention plus exact Art-Net Port-Address; consistent conversion everywhere |
| Silent fixture approximation | Fixture behaves dangerously despite “valid” badge | Explicit unsupported/ambiguous state with channel-table diff and test mode |
| No live source/ownership indicator | Operator cannot tell whether cue/script/API/AI owns an attribute | Per-source activity, priority, session expiry, and one-click revoke/override |
| Autosave without recovery visibility | Users fear or unknowingly lose shows | Save state, backup timestamp, recovery copies, migration preview, and read-only fallback |
| AI confirmation for everything or nothing | Either unusably slow or unsafe | Policy-based autonomy with visible bounds, dry-run diff, audit, and immediate override |

## "Looks Done But Isn't" Checklist

- [ ] **Art-Net output:** Verify packet bytes, sequence wrap, even payload length, subscriber unicast, 44 Hz ceiling/discovered refresh, unchanged-data retransmit, node loss, and dual-NIC behavior.
- [ ] **Timing:** Verify fake-clock determinism plus receiver-measured jitter under UI reload, save, import, script loop, API burst, and slow LLM calls.
- [ ] **Patching:** Verify slots 1/512, universe boundaries, overlap rejection, fine channels, mode footprint, and unsupported multi-break behavior.
- [ ] **Fixtures:** Verify semantic validation, maintenance-range flags, provenance, definition hash pinning, and physical channel-by-channel smoke test.
- [ ] **Playback:** Verify concurrent cue/source arbitration, mid-fade interruption, stop/release, blackout priority, and deterministic replay.
- [ ] **Persistence:** Verify forced termination during save/migration, consistent backup restore, newer-schema refusal, integrity/foreign-key checks, and WAL-fixed SQLite version.
- [ ] **Wails UI:** Verify output survives DOM reload/hang and UI event consumers can fall behind without backpressuring playback.
- [ ] **API:** Verify versioned contract, idempotent retries, duplicate-ID mismatch rejection, stale revision conflict, auth/bind defaults, and parity with UI commands.
- [ ] **TypeScript:** Verify compile errors/source maps, infinite loop, blocking native callback cancellation, queue/rate budgets, runtime isolation, and capability revocation.
- [ ] **LLM autonomy:** Verify invalid IDs, stale state, provider timeout/retry, duplicate tool call, forbidden action, bounded batch, audit redaction, and model-independent operator stop.
- [ ] **Packaging:** Verify native install/launch/network/save/restore on every supported OS, WebView/dependency handling, suspend/resume, and signing/notarization.
- [ ] **Traceability:** Verify offline repository completeness, stable Linear UUID mapping, rate-limit recovery, duplicate prevention, and dry-run reconciliation.

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| UI/runtime coupled to output | HIGH | Freeze features; extract headless engine and command boundary; add receiver soak test |
| Art-Net packet/address bug | MEDIUM | Stop output; capture packets; correct encoder/mapping; run virtual + two-node regression |
| Wrong interface/route | LOW | Stop ambiguous transmission; rebind selected adapter; rediscover; confirm receiver |
| Bad fixture definition | MEDIUM | Restore pinned version; quarantine revision; diff/manual/physical validation; migrate show explicitly |
| Nondeterministic playback | HIGH | Disable automation; establish operator snapshot; replay audit in deterministic arbiter |
| Corrupt/migrated show | HIGH | Preserve originals/sidecars; restore verified backup; recover into fresh DB; never repair sole copy |
| Runaway script | LOW–MEDIUM | Revoke capability; cancel/interrupt; drop its queue; start fresh runtime; inspect logs |
| Duplicate/stale API actions | MEDIUM | Disable writes; reconcile command IDs/audit; repair through idempotent commands |
| Unsafe AI action | MEDIUM | Engine-level override; revoke session; drop pending commands; restore safe scene; replay incident |
| Broken platform artifact | MEDIUM | Hold release for target; restore prior artifact; reproduce on clean native image |
| Linear drift | LOW | Use repository mappings; incremental fetch; dry-run reconciliation; human-resolve conflicts |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| UI owns playback | Foundation + Protocol | DOM reload/hang does not change receiver cadence or state |
| Ticker treated as contract | Protocol | Fake-clock frame hashes and load-soak jitter/deadline metrics pass |
| Partial Art-Net implementation | Protocol | Golden packets, capture parser, conformance checks, two physical nodes |
| Wrong interface/broadcast | Protocol | Dual-NIC/VPN/DHCP/unplug tests and visible adapter/target diagnostics |
| Universe/address confusion | Foundation + Fixtures + Protocol | Boundary and round-trip serialization/API tests |
| Flat fixture profiles | Fixtures | Golden edge-case corpus and explicit unsupported errors |
| Generic pan/tilt/color | Fixtures + Workflow | Physical range/home/color tests and definition-update diff |
| Undefined playback concurrency | Foundation + Workflow | Deterministic simultaneous-source and override scenarios under `-race` |
| Unsafe persistence/migrations | Workflow | Kill-point migrations, verified backup restore, integrity checks, fixed SQLite version |
| Unsandboxed TypeScript | Scripting | Loop/native-call/resource/capability tests without output interruption |
| Unstable/non-idempotent API | API | Contract tests, retry/dedupe and expected-revision conflicts |
| Unsafe autonomous LLM | AI | Stale/invalid/retry/destructive tests and model-independent stop latency |
| Unproven fixture provenance | Fixtures + AI | Source/hash/review fields and channel-by-channel physical validation |
| Cross-platform build-only support | Foundation + Release | Clean native install and end-to-end smoke test per target |
| No representative test rig | Protocol onward | Virtual fault matrix plus physical soak gates every release |
| Linear/repository drift | Governance | Offline planning works; reconciliation audit has no unexplained gaps/duplicates |

## Sources

### Primary specifications and framework/runtime documentation

- [Art-Net 4 protocol specification, revision 1.4dp](https://art-net.org.uk/downloads/art-net.pdf)
- [Art-Net terminology and casting guidance](https://art-net.org.uk/art-net-introduction-and-terminology/)
- [Wails v2 lifecycle/binding architecture](https://wails.io/docs/howdoesitwork)
- [Wails v2 runtime events](https://wails.io/docs/reference/runtime/events/)
- [Wails v2 installation and platform dependencies](https://wails.io/docs/v2.12.0/gettingstarted/installation/)
- [goja runtime documentation](https://pkg.go.dev/github.com/dop251/goja)
- [Go time package](https://pkg.go.dev/time)
- [Go net package](https://pkg.go.dev/net)
- [Go race detector](https://go.dev/doc/articles/race_detector)
- [SQLite corruption guidance](https://www.sqlite.org/howtocorrupt.html)
- [SQLite backup API](https://www.sqlite.org/backup.html)
- [SQLite WAL documentation](https://www.sqlite.org/wal.html)
- [GDTF DMX channel guidance](https://gdtf-share.com/help/users/gdtf_builder/dmx/index.html)
- [GDTF mode dependencies](https://gdtf-share.com/help/users/gdtf_howto/handle_mode_dependencies/index.html)
- [Open Fixture Library JSON compatibility warning](https://open-fixture-library.org/about/plugins/ofl)
- [RFC 9110: HTTP Semantics](https://www.rfc-editor.org/rfc/rfc9110.html)
- [Linear API rate limiting](https://linear.app/developers/rate-limiting)
- [Linear API webhooks](https://linear.app/developers/webhooks)

### LLM/provider implementation references

- [LangChainGo official repository](https://github.com/tmc/langchaingo/)
- [Ollama tool-calling documentation](https://docs.ollama.com/capabilities/tool-calling)
- [Ollama OpenAI-compatibility documentation](https://docs.ollama.com/api/openai-compatibility)

---
*Pitfalls research for: GOLC*
*Researched: 2026-07-17*
