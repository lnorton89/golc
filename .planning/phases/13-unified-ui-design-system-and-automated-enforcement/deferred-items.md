# Phase 13 Deferred Items

Out-of-scope discoveries found while executing a plan's own verification loop,
logged here per the executor's SCOPE BOUNDARY rule rather than fixed inline.

## 260803-13-32: Pre-existing top-bar text-overlap regression at 900px width

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
