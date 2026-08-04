# Phase 13 Deferred Items

Out-of-scope discoveries found while executing a plan's own verification loop,
logged here per the executor's SCOPE BOUNDARY rule rather than fixed inline.

## 260803-13-32: Pre-existing top-bar text-overlap regression at 900px width [RESOLVED 2026-08-04]

- **Resolution:** `GlobalFrame`'s header now has the narrow-width strategy
  this entry's own "Recommended follow-up" called for. `SafetyCluster`'s/
  `TempoControls`'/`MidiLearnToggle`'s shared icon-only collapse breakpoint
  moved from 700px to 1024px, restoring `LiveStatusBar` enough room at
  900/960px that Scene/Layers/Bar and the Source/Output chips no longer
  need to squeeze at all. `LiveStatusBar.module.css` also hardened
  `.metricLabel`/`.metricValue`/the new `.statusChip` wrapper so every
  field can genuinely shrink toward 0 (real `overflow:hidden` +
  `min-width:0`, not just an ancestor clip) instead of holding its full
  natural width and geometrically spilling into a neighbor at more extreme
  widths, plus a two-tier gap reduction (`space2` at ≤1024px, `0` at
  ≤700px) that closes the last few px at the 640px supported floor.
  `frontend/e2e/resize.spec.ts` now passes at 640/900/960/1280/1920
  (verified stable across repeated runs); `persistent-shell-*-win32.png`,
  `dialog-layer-*-900-win32.png`, and every other 900px shell/workspace
  baseline that captures the header were re-captured to match. The
  320x240 `Overview` destination's separate, still-open
  `findOverflowingControls` failure (`SafetyCluster`/`MidiLearnToggle`/
  `Diagnose` past the viewport, below the 640px supported floor) is
  unrelated to this entry and reproduces identically on the pre-fix
  baseline -- left open, matching this suite's existing
  `KNOWN_SUB_640PX_OFFENDERS` precedent for other sub-640px destinations.
- **Found during:** Plan 13-32 (Task 1, persistent shell baselines), while
  writing the shell surface's own semantic pre-capture assertions.
- **Symptom:** `helpers.ts`'s `expectTopBarTextToBeReadable(page)` fails at
  exactly 900x720 on the Overview destination with `["Scene" overlaps
  "Layers", "Layers" overlaps "Bar", "live" overlaps "120 BPM"]`.
- **Confirmed pre-existing, not caused by this plan:** reproduces identically
  on the unmodified `master` worktree base via the already-committed,
  previously-green `frontend/e2e/resize.spec.ts`'s own "Test 1: tiling-WM
  aspect-ratio sweep › every destination at 900x720 (regression anchor)"
  test -- run with no changes from this plan applied
  (`npx playwright test e2e/resize.spec.ts --grep "900x720 \(regression
  anchor\)" --project=chromium --workers=1`). This plan's own files
  (`e2e/design-system.visual-shell.spec.ts` and its snapshot PNGs) do not
  touch `GlobalFrame`, `LiveStatusBar`, `TempoControls`, or `SafetyCluster`.
- **Likely root cause:** `.planning/debug/260801-header-narrow-width-
  overflow.md` (resolved 2026-08-02) had verified this exact header
  clean down to 640px width. Since then, Plan 13-15's `SafetyCluster`
  migration deliberately increased each safety control from 28px to
  `--ds-sizing-control-minimum-target` (44px) for accessibility (documented
  in `13-15-SUMMARY.md`'s decisions). That is a real, larger horizontal
  footprint inside `GlobalFrame`'s fixed, single-row, non-wrapping 52px
  header -- plausible enough to have silently pushed the previously-clean
  900px floor back into overlap, though this was not independently
  re-measured against pre-13-15 SHAs as part of this discovery.
- **Why deferred rather than fixed here:** the fix requires touching
  `GlobalFrame.module.css`/`LiveStatusBar.module.css`/
  `TempoControls.module.css`/`SafetyCluster.module.css` -- none of which
  are in Plan 13-32's declared `files_modified` -- and, per the original
  debug doc's own reasoning, a real fix is a narrow-width design decision
  (further icon-only collapse, wrapping, or field truncation tuning), not a
  mechanical one-line correction.
- **Impact on this plan:** Plan 13-32's own shell-baseline test asserts
  UI-SPEC's literal "no control/text/overlay escapes its own box or
  viewport" bullet (`findOverflowingControls`, which does not flag this --
  nothing is clipped past the viewport, elements only visually overlap
  each other) plus safety-cluster/landmark visibility, but does not also
  assert the stricter zero-cross-element-overlap check
  (`expectTopBarTextToBeReadable`) that this regression trips, to avoid
  blocking an unrelated, pre-existing, already-red condition. The four
  accepted `persistent-shell-*-win32.png` baselines therefore visually
  encode this overlap at 900px width, matching the app's real current
  rendering -- not a synthetic clean state.
- **Recommended follow-up:** re-run `frontend/e2e/resize.spec.ts`'s "Test 1"
  900x720 case and `expectTopBarTextToBeReadable` in this spec once a future
  pass gives `GlobalFrame`'s header row an explicit narrow-width strategy
  that accounts for the larger 44px safety-control floor; re-capture the
  four `persistent-shell-*-win32.png` baselines afterward since the fix
  will change their pixels.

## 260803-13-33: `GuidedFirstShow`'s Verify-stage tone label crowds its count with no separating space

- **Found during:** Plan 13-33 (Task 3, Guided First Show baselines), while
  visually reviewing the accepted `guided-first-show-*-win32.png` captures.
- **Symptom:** `VerifyStage.tsx`'s `<ul aria-label="Readiness summary">`
  renders `<li><span>Blocker</span><span>{pluralize(...)}</span></li>` --
  two adjacent inline `<span>` elements with no separating space or margin
  between them, so the rendered text reads `Blocker1 blocker` / `Warning1
  warning` / `Evidence4 evidence items` with no visible gap between the
  tone word and its count.
- **Confirmed pre-existing, not caused by this plan:** `VerifyStage.tsx` is
  not in Plan 13-33's declared `files_modified`; the crowding exists
  regardless of which readiness counts are seeded (a copy/spacing gap in
  the component's own markup, not a data artifact of this plan's mocked
  blocker/warning/evidence rollup).
- **Why deferred rather than fixed here:** a one-line CSS/markup fix (e.g.
  a `gap` on the `<li>` or a literal separator between the two spans) is
  low-risk, but `VerifyStage.tsx`/its stylesheet are outside this plan's
  declared scope, and `GuidedFirstShow.test.tsx` may assert the exact
  current text nodes -- a blind fix risks an unrelated test churn this
  plan did not budget for.
- **Impact on this plan:** purely cosmetic; every semantic assertion this
  plan's own test makes (`toContainText("1 blocker")` etc.) still passes
  correctly since `toContainText` matches a substring regardless of
  surrounding whitespace. The accepted `guided-first-show-*-win32.png`
  baselines visually encode this crowding, matching the app's real current
  rendering.
- **Recommended follow-up:** add a small gap/separator between the tone
  label and its count in `VerifyStage.tsx`'s `<li>` markup (or its CSS
  module) in a future Guided First Show polish pass; re-capture the four
  `guided-first-show-*-win32.png` baselines afterward since the fix will
  change their pixels.

## 260804-13-32b: `Overview` destination overflows the viewport at 320x240, below the supported floor

- **Found during:** fixing 260803-13-32 (above) and re-running the full
  `frontend/e2e/resize.spec.ts` sweep to confirm the header fix didn't
  regress anything else.
- **Symptom:** `Test 1`'s 320x240 case fails on the `Overview` destination
  with `findOverflowingControls` (not `expectTopBarTextToBeReadable`):
  `"Stop / Release All"`, `"MIDI Learn"`, and `"Diagnose"` each report a
  right edge past the 320px viewport.
- **Confirmed pre-existing, not caused by 260803-13-32's fix:** reproduces
  identically with `SafetyCluster.module.css`/`TempoControls.module.css`/
  `MidiLearnToggle.module.css`/`LiveStatusBar.module.css` reverted to their
  pre-fix state (`git stash` of those four files, same test, same result).
  320px is below `resize.spec.ts`'s own `HEADER_MIN_SUPPORTED_WIDTH`
  (640px) floor, the same territory `KNOWN_SUB_640PX_OFFENDERS` already
  allowlists per-destination for six other destinations (Settings, Fixture
  Library, Patch & Pools, Scenes & Looks, Operator Surface, Desk) --
  `Overview` simply isn't in that dict yet.
- **Likely root cause:** the same `SafetyCluster` 28px→44px target-size
  growth (13-15) that motivated 260803-13-32 above; even fully collapsed to
  icon-only (which 260803-13-32's breakpoint raise does not change below
  700px), three 44px safety controls plus `MidiLearnToggle` plus
  `LiveStatusBar`'s own 60px floor no longer fit in a 320px-wide header.
- **Why deferred rather than fixed here:** below the documented supported
  floor, this suite's own convention is to allowlist known offenders per
  destination rather than force a narrow-width redesign (see
  `KNOWN_SUB_640PX_OFFENDERS`'s existing six entries) -- `D-13`'s actual
  guarantee (the safety cluster stays available) is separately verified by
  `expectSafetyClusterAvailable` regardless of on-screen position, and this
  is well outside the header-legibility floor 260803-13-32 targeted.
- **Recommended follow-up:** add `Overview: /^"(Stop \/ Release All|MIDI
  Learn|Diagnose)":/` (or similarly scoped pattern) to
  `KNOWN_SUB_640PX_OFFENDERS` in `frontend/e2e/resize.spec.ts`, or -- if a
  future pass wants genuine sub-640px header coverage -- give
  `SafetyCluster`/`MidiLearnToggle` a second, narrower collapse tier below
  700px.

## 260803-13-18: `ROADMAP.md`'s Phase 13 heading uses `**Requirements**:` instead of `**Requirements:**`

- **Found during:** Plan 13-18 (post-task full-suite verification via `mage
  Test`), while confirming no unrelated package regressed.
- **Symptom:** `go test ./internal/trace/catalog/...` fails two subtests
  (`TestScopeLinearCatalog/real_repository_catalog_validates_end_to_end_offline`,
  `TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline`) with
  `GOLC_CATALOG_ROADMAP_REQUIREMENTS_MISSING: phase 13 has no **Requirements:**
  line in ROADMAP.md`.
- **Confirmed pre-existing, not caused by this plan:** `.planning/ROADMAP.md`
  is not in Plan 13-18's declared `files_modified` and was never touched by
  any of this plan's three tasks. `.planning/ROADMAP.md:584` reads
  `**Requirements**: D-01 through D-14 and the approved Phase 13 UI-SPEC
  acceptance contract` -- the bold markers close before the colon, unlike
  every other phase heading in the file (e.g. line 36's
  `**Requirements:** CONF-01, ...`, colon inside the bold markers). This is a
  pure Markdown-formatting drift already present at this worktree's base
  commit (`4bb10d54`), unrelated to any file this plan touches.
- **Why deferred rather than fixed here:** `.planning/ROADMAP.md` is a shared
  orchestrator-owned artifact outside this plan's declared scope; per the
  executor's worktree instructions, `.planning/` documentation files are the
  orchestrator's to update centrally after a wave completes, not an
  individual plan's per-task edit.
- **Impact on this plan:** none of Plan 13-18's own declared verify commands
  (`go test ./internal/command -run DesignSystem`, `go test ./internal/delivery
  ./magefiles -run 'DesignSystem|MageTarget' && mage -l`, `mage CheckOffline
  && git diff --check -- .github/workflows/design-system.yml`) touch
  `internal/trace/catalog`, so this pre-existing failure does not block this
  plan's own success criteria. It does surface in a full, unfiltered `mage
  Test`/`go test ./...` run.
- **Recommended follow-up:** change `.planning/ROADMAP.md:584` from
  `**Requirements**:` to `**Requirements:**` (moving the colon inside the bold
  markers) to match every other phase heading's exact grammar.
