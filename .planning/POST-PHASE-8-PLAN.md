# Post-Phase-8 Plan

**Created:** 2026-07-25 · **Status:** proposed, awaiting execution after Phase 8 completes
**Source:** `golc-state-analysis-2026-07-25.md` (external analysis) + owner decisions below

This document is the answer to the analysis's core finding: the engineering process
is strong, the operator-facing workflow has never been exercised end-to-end, and the
UI has real front-door gaps (Fixture Library stub, no show open/new/switch, no
onboarding) that make any "run a real show" validation premature right now. Phase 8
itself is **not** being trimmed — it runs to completion as currently scoped (all 13
plans, including the CDP debugger and Monaco editor). This plan covers what happens
after Phase 8's verification passes.

## Owner decisions (2026-07-25)

| Question | Decision |
|---|---|
| First Real Show timing/rigor | **Not yet.** The UI isn't close to finished — a validation session is premature until the front-door workflow (patch, open/switch show, onboarding) actually exists on screen. Rigor level (full second-operator/QLC+ comparison vs. lighter solo pass) is an open call to revisit once front-door UI work lands, not decided now. |
| Club vs. cue-stack audience | **Narrow to club/DJ.** Church/school/community language in `PROJECT.md` was aspirational, not validated. No cue-stack/timed-fade work enters the roadmap. Update `PROJECT.md`'s target-audience language and Key Decisions to match the actual scene/BPM/layer model. |
| Launcher/packaging pull-forward | **Not yet.** Dev machine (`mage Run`) is fine for now. Packaging stays scoped to the real Phase 10 (Windows release qualification) — no thin slice pulled forward. |
| Hygiene wave timing | **Own short phase, right after Phase 8.** Fix state/doc drift and the `internal/trace` race + catalog test failures before anything else compounds on top of stale state. |

## Sequencing

```
Phase 8 (current, unchanged, all 13 plans)
   │
   ▼
Phase 8-hygiene  — short phase, doc/state drift + internal/trace fixes
   │
   ▼
Phase 8-front-door — Fixture Library, show open/new/switch, Guided First Show onboarding
   │
   ▼
First Real Show validation — gated on front-door phase; rigor level decided then
   │
   ▼
Phase 9 (Provider-neutral AI autonomy) — proceeds as originally roadmapped,
   informed by First Real Show findings and the now-locked club/DJ scope
   │
   ▼
Phase 10 (Windows release qualification) — unchanged, includes full packaging
   │
   ▼
Phase 11 (Telemetry & crash pipeline) — unchanged
```

(Exact phase numbers get assigned when the phases are actually inserted into
`ROADMAP.md` via `gsd-phase insert` — GSD renumbers 9/10/11 automatically. Not done
yet, on purpose: touching `ROADMAP.md` while Phase 8's wave orchestration is actively
writing to it would race with the running executors.)

---

## 1. Hygiene phase (do first, right after Phase 8)

Low-risk, no product-behavior change, restores trust in the state every future
planning agent reads.

- [ ] `README.md`: fix "Phase 7 … is in progress" → reflect actual state (Phase 8
      complete or executing, per whatever's true when this runs).
- [ ] `.planning/STATE.md` frontmatter: `total_phases: 8` → `11` (matches ROADMAP).
- [ ] `.planning/STATE.md`: fix velocity metrics ("Total execution time: 0 hours",
      missing Phase 6 row, "Last 5 plans: none") — either populate from real data or
      remove the section if it can't be made accurate mechanically.
- [ ] `ROADMAP.md` progress table: Phase 6 row shows "In Progress" with no
      completion date; the checklist above it already marks it complete 2026-07-24.
      Fix the table to match.
- [ ] `.planning/sketches/WORKFLOW-MAP.md`: still describes the pre-restructure IA
      (Patch/Program/Perform/Setup, `Ctrl+1..4`). Update to match the shipped
      Show/Build/Operate/Output rail (`frontend/src/shell/navigation.ts`).
- [ ] `internal/trace/transport/process.go`: fix the `Wait()` vs.
      `safeFailureSummary()` data race flagged in Phase 6's `deferred-items.md`.
- [ ] Fix `TestScopeLinearCatalog`/`TestScopeLinearMap` failures against the real
      catalog (same deferred-items entry).
- [ ] `PROJECT.md`: narrow target-audience language to club/DJ (owner decision
      above) — remove or reframe the church/school/community line, and add a Key
      Decision entry recording that cue-stack/timed-fade playback is explicitly
      out of scope for v1, not an open question.

**Exit gate:** `go test ./...` clean (including the two `internal/trace` failures
above), README/STATE/ROADMAP internally consistent, WORKFLOW-MAP.md matches shipped
navigation, PROJECT.md audience/scope language locked.

---

## 2. Front-door workflow completion phase

This is the phase that makes "First Real Show" a meaningful test instead of a
premature one. All three items already have a design or a backend to build against
— this is UI wiring, not new product design.

- [ ] **Fixture Library workspace**: replace the `ComingSoon` stub in
      `frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx`. Browse, inspect,
      and import fixture definitions (OFL + hand-authored YAML) through the UI —
      the backend `fixture validate`/`fixture inspect`/`fixture import` routes
      already exist (Phase 8's own scriptsdk work classified them). This is the
      *first* box in `WORKFLOW-MAP.md`'s flow and is currently CLI-only.
- [ ] **Show open/new/switch UI**: `SaveRecoveryWorkspace.tsx` currently documents
      that the desktop app resolves exactly one show path at startup. Add UI for
      opening an existing show, creating a new one, and switching between them —
      the CLI flow (`show open`, recovery accept/discard) already exists; this is
      surfacing it. Decide during planning whether this lives in the existing
      Show group or needs a new entry point.
- [ ] **Guided First Show onboarding**: implement Sketch 004-B as approved
      (`grep -ri "guided\|onboard" frontend/src` currently returns 0 hits). This is
      the one artifact in the sketch process specifically designed to answer "how
      does a new operator get from empty app to light on stage" — approved, never
      built.

**Exit gate:** a person who has never used GOLC can go from a fresh checkout to a
patched fixture and a scene on screen using only the UI, with the onboarding flow
as the on-ramp — no CLI required for the happy path.

---

## 3. First Real Show validation (gated, not scheduled yet)

Do not schedule this until the front-door phase above passes its exit gate. When it
does, decide the rigor level then (full session vs. lighter solo pass — this was
left open, not skipped). If/when run:

1. Patch the real rig from OFL imports plus at least one hand-written YAML fixture,
   through the UI, logging any forced CLI drop as a gap.
2. Program a set: scenes, all four layer kinds, blends, tap tempo.
3. Decide then whether to include the second-operator handoff and QLC+ timing
   comparison from the original report (7.2), or run solo first and expand later.
4. Feed every friction point into gap plans, same loop as `06-09`…`06-12`.

---

## 4. Standing practice: process budget (ongoing, not a phase)

From the analysis's §7.7 — apply going forward rather than as a one-time task:

- SUMMARY.md files: keep to the existing template, don't let them grow unbounded.
- Quick-task records: a table row unless the task changed behavior.
- No new `.planning` doc genres without retiring one.

This isn't gated on anything — just a norm to hold to starting now, since the
docs-to-code ratio (1.4× at day 8) only gets heavier from here.

---

## Explicitly deferred / not changing

- **Phase 8 scope** (debugger CDP bridge, Monaco editor, breakpoint UI): unchanged,
  runs to completion. Not revisited by this plan.
- **Launcher/packaging thin slice**: stays in Phase 10, not pulled forward.
- **Cue-stack/timed-fade playback**: out of scope for v1 (see hygiene phase
  PROJECT.md update), not a deferred feature to revisit without a new signal.
