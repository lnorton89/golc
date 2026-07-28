# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.0 — Core Lighting Console (Club/DJ)

**Shipped:** 2026-07-27
**Phases:** 8 | **Plans:** 99 | **Timeline:** 2026-07-17 → 2026-07-27 (10 days)

### What Was Built

- Offline-first, layered project configuration with a durable local-ID grammar and
  credential-free, idempotent Linear reconciliation (Phase 1).
- A modular semantic fixture catalog (OFL import + custom YAML) with reviewed,
  atomic pool/deployment impact changes and capability-based substitution (Phase 2).
- Deterministic tempo-aware show programming and playback — themes, chases, motion
  presets, bar-loop scenes, next-bar-boundary adoption (Phase 3).
- Real, hardware-verified Art-Net output behind a non-blocking daemon architecture
  (Phase 4).
- Durable SQLite-backed show storage with recovery points and verified-backup
  migrations (Phase 5).
- A full Wails desktop authoring/operator workflow with generic MIDI control and
  daemon-resident safety overrides independent of every other subsystem (Phase 6).
- A versioned, audited, OpenAPI-documented external HTTP API with SSE and atomic
  batches (Phase 7).
- Isolated, capability-limited TypeScript scripting with a generated SDK, Windows
  Job Object sandboxing, and a real CDP debugger (Phase 8).

### What Worked

- The gap-closure pattern (verify → find real gaps → close with numbered follow-up
  plans, e.g. 04-08/04-09, 06-09..06-12, 07-10..07-15) consistently caught genuine
  functional gaps that a single verification pass would have missed, and kept the
  fix scoped and traceable back to the specific finding.
- Reusing existing contracts instead of rebuilding them per phase paid off
  repeatedly: Phase 2's `pool.ImpactPlan`/`ValidatePlanIntegrity`/`Apply` was reused
  verbatim by fixture substitution; Phase 7's ring-buffer SSE pattern was reused
  directly by Phase 8's script event stream.
- Treating the daemon/worker boundary as sacred (UI, scripts, API, and LLM code
  never gets to touch Art-Net timing) held up under real adversarial testing —
  Phase 8's `TestScriptKillDoesNotBlockArtnet` proved a runaway script's kill
  produces no missed frame, and Phase 6's safety overrides route through the
  daemon directly, bypassing the webview entirely.
- The final Phase 8 acceptance pass was run as genuine agent-executed evidence
  gathering against real hardware rather than a rubber-stamped checkpoint, and it
  found two real bugs (a debug-mode CDP hang, a test-hermeticity issue) that a
  paper-only review would have missed.

### What Was Inefficient

- REQUIREMENTS.md checkbox corrections lagged behind actual delivery more than
  once: PLAY-10/11/12 (Phase 6) and SCRP-03/06 (Phase 8) were functionally
  complete and verified but still read "Pending" going into milestone close,
  requiring a dedicated correction pass during archival instead of being fixed at
  the point of delivery.
- STATE.md's velocity-tracking section decayed early (missing Phase 6 row, "Last 5
  plans: none") and was never repaired, so it carried no signal by the time this
  milestone closed.
- The milestone-scope question (is "v1.0" Phases 1-8, or all 11 phases as
  REQUIREMENTS.md's own "v1 requirements: 88" line claimed?) wasn't resolved until
  milestone-close time, even though the owner had already decided it 2026-07-25 in
  `POST-PHASE-8-PLAN.md`. Reconciling ROADMAP.md/REQUIREMENTS.md's "v1" language
  with that decision at the time it was made would have avoided a second pass.

### Patterns Established

- Doc-accuracy corrections discovered during a verification report (e.g.
  `06-VERIFICATION.md` flagging PLAY-10/11/12) need a mechanical follow-up, not
  just a note — the note alone did not get acted on for over a month of elapsed
  project time (06-12 → milestone close).
- When a milestone's real shipped scope diverges from its originally-documented
  scope (as happened here: 8 of 11 originally-"v1" phases), archive only the
  requirements that actually shipped and explicitly carry the rest forward with a
  written Scope Note, rather than either force-closing the unfinished requirements
  or blocking the release on them.

### Key Lessons

1. When a phase's own verification report flags a documentation-accuracy gap
   ("code and tests exist and pass, but REQUIREMENTS.md still says Pending"),
   close it in the same phase rather than deferring it — it otherwise surfaces
   again, at higher stakes, during milestone close.
2. If a milestone's scope is going to be split from what REQUIREMENTS.md
   originally defined, write that decision directly into REQUIREMENTS.md/PROJECT.md
   at decision time (not just in a side planning doc), so the next
   milestone-completion pass doesn't have to re-derive it from git history and
   cross-file comparison.
3. Reusable domain contracts (impact-plan/apply, ring-buffer SSE) are worth
   designing for reuse from the first phase that needs them — every later phase
   that reused one shipped faster and with fewer novel bugs than phases that built
   a parallel mechanism.

### Cost Observations

- Sessions: not tracked in STATE.md's velocity section for this milestone
  (flagged above as inefficiency); not reconstructable from planning artifacts.
- Notable: 936 commits / 200,541 insertions / ~66.5k LOC (Go + TypeScript,
  excluding tests) across 8 phases in 10 calendar days, with 3 phases (1, 6, 7)
  needing dedicated gap-closure rounds after their first verification pass.

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Phases | Key Change |
|-----------|--------|------------|
| v1.0 | 8 | First milestone. Established the gap-closure-round pattern and the daemon/worker-boundary safety discipline that later phases relied on. |

### Top Lessons (Verified Across Milestones)

1. Close documentation-accuracy gaps in the phase that finds them — not yet
   cross-validated against a second milestone, but v1.0's PLAY-10/11/12 and
   SCRP-03/06 corrections cost a dedicated pass at close time that phase-level
   follow-through would have avoided.
