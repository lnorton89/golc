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

## 1. Hygiene phase (do first, right after Phase 8) — DONE 2026-07-27

Low-risk, no product-behavior change, restores trust in the state every future
planning agent reads. Completed as part of the v1.0 milestone close
(`/gsd-complete-milestone`), not as a separately inserted roadmap phase.

- [x] `README.md`: fixed "Phase 8 … is in progress" → v1.0 (Phases 1-8) shipped
      2026-07-27; Roadmap table and tech-stack rows corrected to match.
- [x] `.planning/STATE.md` frontmatter: `total_phases: 8` → `11` (matches ROADMAP).
- [x] `.planning/STATE.md`: velocity metrics section removed — it showed 74 plans
      (should be 99), was missing Phases 06/08 entirely, and had a malformed
      table; no real per-plan duration data was ever captured to populate it
      accurately, so it was removed rather than fabricated.
- [x] `ROADMAP.md` progress table: Phase 6 and Phase 8 rows corrected to
      Complete with their real completion dates.
- [x] `.planning/sketches/WORKFLOW-MAP.md`: rewritten to describe the shipped
      Show/Build/Operate/Output command rail (`frontend/src/shell/navigation.ts`)
      and the real global playback keyboard shortcuts
      (`frontend/src/hooks/useKeyboardWorkflow.ts`); the fictional
      `Ctrl+1..4`/`F6` nav shortcuts were never implemented and are removed.
- [x] `internal/trace/transport/process.go`: fixed the `Wait()` vs.
      `safeFailureSummary()` data race — exit state is now tracked in
      client-owned fields guarded by a dedicated mutex, written once by the
      goroutine that calls `Wait()`. 10 consecutive `-race` runs pass clean.
- [x] `TestScopeLinearCatalog`/`TestScopeLinearMap` now pass cleanly against the
      real repository catalog (resolved incidentally by correcting
      REQUIREMENTS.md's PLAY-10/11/12 and SCRP-03/06 status during milestone
      close — see `.planning/MILESTONES.md`).
- [x] `PROJECT.md`: target-audience language narrowed to club/DJ; a Key
      Decision entry and an Out of Scope line record that cue-stack/timed-fade
      playback is explicitly out of scope for v1, not an open question.

**Exit gate met:** `go test ./...` is clean, README/STATE/ROADMAP are internally
consistent, WORKFLOW-MAP.md matches shipped navigation, PROJECT.md audience/scope
language is locked.

**Note:** ROADMAP.md's Phase 1-8 detail sections were NOT collapsed/archived out
of the live file, unlike GSD's default milestone-archival pattern — this repo's
`internal/trace/catalog` (`BuildCatalog`) requires a live `### Phase N:` section
plus a defined requirement key in `.planning/REQUIREMENTS.md` for every phase
directory still present under `.planning/phases/`. Collapsing them broke the
catalog during this same milestone close and had to be reverted (see git history
around 2026-07-27's `chore: archive v1.0 milestone` / `fix: keep full phase
detail and requirement definitions live` commits). A future change to
`internal/trace/catalog` to also resolve archived phases from
`.planning/milestones/` would be needed before that pattern can be used safely
here.

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
