# Milestones: GOLC

## v1.0 — Core Lighting Console (Club/DJ)

**Shipped:** 2026-07-27
**Phases:** 1-8 | **Plans:** 99 | **Timeline:** 2026-07-17 → 2026-07-27 (10 days)
**Closeout type:** verified_closeout (all 8 phases complete and verified; pre-close artifact audit clear)

**Delivered:** The deterministic lighting-console core for club/DJ operators —
offline-safe configuration and Linear traceability, a modular semantic fixture
catalog with reviewed pool/deployment changes, tempo-aware deterministic show
programming and playback, real Art-Net output verified against hardware, durable
SQLite-backed show storage with recovery, a full Wails desktop authoring/operator
surface with generic MIDI control and independent safety overrides, a versioned
audited external API, and isolated capability-limited TypeScript scripting with a
real debugger.

**Key accomplishments:**

1. Offline-first, layered project configuration with a durable local-ID grammar and
   credential-free, idempotent Linear reconciliation (Phase 1).
2. Modular semantic fixture catalog — OFL import plus custom YAML through one
   canonical pipeline, with atomic, reviewed pool/deployment impact analysis and
   capability-based fixture substitution (Phase 2).
3. Deterministic tempo-aware show programming and playback — themes, chases,
   motion presets, bar-loop scenes, and a lock-free engine that adopts staged
   edits only at bar boundaries (Phase 3).
4. Real, verified Art-Net output — byte-exact ArtDMX encoding, a non-blocking 40Hz
   worker behind a Windows daemon, confirmed against Wireshark, an independent OLA
   simulator, and physical hardware (Phase 4).
5. Durable SQLite-backed show storage with in-transaction recovery points,
   verified-backup migrations, and integrity diagnostics (Phase 5).
6. Full Wails desktop authoring/operator workflow — constrained operator surfaces,
   generic MIDI Note/CC learn with soft takeover, and daemon-resident safety
   overrides independent of UI/script/API/LLM completion (Phase 6).
7. Versioned, audited, OpenAPI-documented external HTTP API with revisioned SSE
   events and atomic multi-command batches (Phase 7).
8. Isolated, capability-limited TypeScript scripting — a generated typed SDK,
   Windows Job Object sandboxing, and a real CDP breakpoint/step debugger
   (Phase 8).

**Known gaps (deferred, not blocking):** see `.planning/PROJECT.md`'s "Current
State" section and `.planning/POST-PHASE-8-PLAN.md` — front-door UI (Fixture
Library, show open/new/switch, onboarding), a hygiene pass for doc/state drift and
two known test issues, and `MIDI-HW-02` per-device physical acceptance evidence.

**Scope note:** The original 2026-07-17 requirements defined "v1" as all 11
phases. At close, the owner split LLM autonomy (Phase 9), Windows qualification
(Phase 10), and telemetry (Phase 11) into a later milestone rather than blocking
this release — see `.planning/milestones/v1.0-ROADMAP.md`'s Scope Note.

Full detail: `.planning/milestones/v1.0-ROADMAP.md`, `.planning/milestones/v1.0-REQUIREMENTS.md`
