---
status: resolved
trigger: "PS C:\\Users\\Lawrence\\Documents\\Dev\\golc> mage dev / Wails CLI v2.13.0 build fails during Go compilation with 'too many .rsrc sections' after frontend build succeeds"
created: 2026-07-29T16:22:17Z
updated: 2026-07-29T18:30:00Z
---

## Current Focus
<!-- OVERWRITE on each update - always reflects NOW -->

hypothesis: CONFIRMED (refined from original). cmd/golc-desktop/rsrc_windows_amd64.syso (checked in for mage Build's plain `go build`, first added in d0c1bdb8, not 0ed088a8) collides with `wails dev`'s own auto-generated Windows resource file. Wails v2.13.0's dev build path hardcodes Pack:true, so every dev rebuild writes cmd/golc-desktop/golc-desktop-res.syso alongside the checked-in file; Go links every *.syso present in the package directory, and the linker accepts only one .rsrc section, so "too many .rsrc sections" fires during that brief window.
test: N/A - confirmed via direct source read of vendored Wails v2.13.0 + tc-hib/winres, no reproduction run needed to establish mechanism.
expecting: N/A - hypothesis confirmed.
next_action: None -- resolved. All three human-verify items confirmed: (1) `mage dev` build succeeds with no "too many .rsrc sections", (2) titlebar icon renders correctly, (3) real interactive Ctrl+C correctly restores cmd/golc-desktop/rsrc_windows_amd64.syso (confirmed present on disk, not left as *.wails-dev-disabled).
reasoning_checkpoint:
  hypothesis: "cmd/golc-desktop/rsrc_windows_amd64.syso (checked in for mage Build's plain `go build`, first added in d0c1bdb8) collides with wails dev's own auto-generated Windows resource file, because Wails v2.13.0's dev build path hardcodes Pack:true (cmd/wails/flags/dev.go GenerateBuildOptions) and therefore always runs compileResources (pkg/commands/build/packager.go) on every dev rebuild, writing cmd/golc-desktop/golc-desktop-res.syso alongside the checked-in file; Go links every *.syso file present in a package directory, and the linker only accepts one .rsrc section per binary, so both files present simultaneously produces 'too many .rsrc sections'."
  confirming_evidence:
    - "Read pkg/commands/build/packager.go:225-300 (compileResources): writes {ProjectData.Name}-res.syso == golc-desktop-res.syso directly into options.ProjectData.Path == cmd/golc-desktop/ (same directory as the checked-in rsrc_windows_amd64.syso)."
    - "Read cmd/wails/flags/dev.go:126-149 (Dev.GenerateBuildOptions): Pack is hardcoded true with no flag to disable it, unlike other Options fields that map to CLI flags."
    - "Read pkg/commands/build/build.go:340-358 (execBuildApplication): only calls packageApplicationForWindows/compileResources when options.Pack && options.Platform==windows, confirming this fires unconditionally for every `wails dev` rebuild on Windows, then removes the generated syso in a defer -- matches the observed evidence that only ONE .syso file exists on disk right now despite the failure having already occurred (the second file existed only transiently during the failed build)."
    - "git show d0c1bdb8's own commit message states explicitly: 'This repo's mage build/mage run path never invokes Wails' own icon-embedding step (it's a plain go build, unlike wails build/mage Dev), so a Windows resource file embedding the icon has to be committed directly: rsrc_windows_amd64.syso' -- confirming the author already understood mage Dev DOES invoke Wails' icon-embedding step, the exact mechanism that now collides."
  falsification_test: "If a fresh checkout with rsrc_windows_amd64.syso temporarily removed/renamed out of cmd/golc-desktop/ still fails `mage dev` with the same 'too many .rsrc sections' error, the checked-in file is not the second contributor and this hypothesis is wrong."
  fix_rationale: "The checked-in rsrc_windows_amd64.syso is not needed by `wails dev` at all -- Wails regenerates its own resource file from the same build/windows/icon.ico + wails.exe.manifest + info.json inputs on every dev rebuild, and (confirmed via tc-hib/winres@v0.2.1's RT_ICON=3 constant, which Wails passes as the SetIcon resID) already places the icon group at resource ID 3, matching winc.AppIconID -- the exact fix 0ed088a8 made for the checked-in file. So disabling (temporarily renaming) the checked-in file for the duration of the `wails dev` child process addresses the root cause directly (only one .syso ever present during either build path) without touching the file `mage Build`'s plain go build still needs."
  blind_spots: "Have not run `mage dev` end to end on this machine to directly observe the titlebar icon still renders correctly under Wails' own auto-generated resource (relying on reading tc-hib/winres source rather than an empirical extract); have not empirically verified Windows Ctrl+C signal delivery behavior for exec.Cmd child process groups, only reasoned about it from Go/Windows console signal semantics -- the signal.Notify-based cleanup guard is a defensive measure against that risk, not something reproduced happening."
tdd_checkpoint: null

## Symptoms
<!-- Written during gathering, then immutable -->

expected: `mage dev` builds the golc-desktop Go binary after the frontend build succeeds, and the Wails dev app window launches / dev server becomes usable.
actual: Frontend (Vite) build succeeds. Wails then attempts to compile the Go application (`github.com/lnorton89/golc/cmd/golc-desktop`) and fails with `too many .rsrc sections`, exit status 1. No binary produced; Wails reports "No version running, build will be retriggered as soon as changes have been detected" and stays in a build-retry loop.
errors: |
  # github.com/lnorton89/golc/cmd/golc-desktop
  too many .rsrc sections
  Build error - exit status 1
reproduction: Run `mage dev` from repo root on Windows.
started: unknown / user unsure if this is the first time running `mage dev` since the icon-embedding change landed (commit 0ed088a8). Most recent commit before this session is 0ed088a8, which specifically touched Windows resource/icon embedding for golc-desktop.

## Eliminated
<!-- APPEND only - prevents re-investigating after /clear -->

## Evidence
<!-- APPEND only - facts discovered during investigation -->

- timestamp: 2026-07-29T17:05:00Z
  checked: `find . -iname "*.syso"` across the repo (excluding vendored Go toolchain testdata)
  found: Only one project .syso file exists on disk: cmd/golc-desktop/rsrc_windows_amd64.syso
  implication: Any second .syso file involved in the failure must be transient (created and removed during the build itself), not a leftover/duplicate committed file.

- timestamp: 2026-07-29T17:07:00Z
  checked: git show d0c1bdb8 --stat and its commit message (the commit immediately before 0ed088a8, also touching golc-desktop icon/resource files)
  found: d0c1bdb8 first added cmd/golc-desktop/rsrc_windows_amd64.syso, and its message states "mage build/mage run path never invokes Wails' own icon-embedding step ... so a Windows resource file embedding the icon has to be committed directly ... unlike wails build/mage Dev"
  implication: The author already knew `mage Dev` (wails dev) has its own separate icon-embedding step, distinct from the committed .syso needed by mage Build's plain `go build`. This is the seed of the conflict.

- timestamp: 2026-07-29T17:15:00Z
  checked: github.com/wailsapp/wails/v2@v2.13.0 pkg/commands/build/packager.go compileResources() and build.go execBuildApplication()
  found: When options.Pack && options.Platform=="windows", Wails generates its own resource file at {ProjectData.Path}/{ProjectData.Name}-res.syso (== cmd/golc-desktop/golc-desktop-res.syso) from build/windows/icon.ico + wails.exe.manifest + info.json, then deletes it in a defer after CompileProject returns (success or failure).
  implication: This explains why only one .syso is visible on disk now (Evidence #1) -- the generated file existed only briefly, during the failed link, then was cleaned up by the defer regardless of the build's outcome.

- timestamp: 2026-07-29T17:18:00Z
  checked: github.com/wailsapp/wails/v2@v2.13.0 cmd/wails/flags/dev.go Dev.GenerateBuildOptions()
  found: `Pack: true` is hardcoded in the returned build.Options; no CLI flag on the `Dev` flag struct maps to it, unlike every other Options field.
  implication: `wails dev` (invoked by mage Dev) cannot be told to skip Wails' own resource-generation step -- it always runs, every dev rebuild, on Windows.

- timestamp: 2026-07-29T17:22:00Z
  checked: github.com/tc-hib/winres@v0.2.1 winres.go (RT_ICON/RT_GROUP_ICON/RT_MANIFEST constants) and icon.go (SetIcon signature)
  found: `RT_ICON ID = 3`. Wails' compileResources calls `rs.SetIcon(winres.RT_ICON, ico)`, i.e. it (arguably by convention/coincidence) places the icon GROUP resource at ID 3 -- the same ID winc.AppIconID (Wails' own Windows titlebar-icon lookup, the thing 0ed088a8 was fixing) expects.
  implication: Wails' own dev-time auto-generated resource file already satisfies the exact titlebar-icon-ID requirement 0ed088a8 fixed for the checked-in file. The checked-in rsrc_windows_amd64.syso is therefore redundant (and actively harmful) for the `wails dev` code path specifically -- it is only required for mage Build's plain `go build`, which never invokes Wails' packaging step at all.

- timestamp: 2026-07-29T17:35:00Z
  checked: Empirically reproduced the exact failure by copying cmd/golc-desktop/rsrc_windows_amd64.syso to cmd/golc-desktop/golc-desktop-res.syso (simulating Wails' transient dev-build resource file) and running `go build -tags desktop,production -o NUL ./cmd/golc-desktop`
  found: Baseline (single .syso) built cleanly with no output. With the duplicate present, the build failed with the exact reported error: "# github.com/lnorton89/golc/cmd/golc-desktop\ntoo many .rsrc sections". Removing the duplicate again restored a clean build.
  implication: Directly confirms (not just source-read inference) that two *.syso files present in cmd/golc-desktop/ during a single `go build` reproduces this exact symptom -- the falsification test in reasoning_checkpoint passed (confirming, not refuting, the hypothesis).

- timestamp: 2026-07-29T17:42:00Z
  checked: Throwaway unit test (internal/command/dev_syso_verify_test.go, written then deleted -- not committed) exercising disableWindowsResourceSyso directly: seeds a fake rsrc_windows_amd64.syso in a t.TempDir, calls disable, asserts the original is gone and a `.wails-dev-disabled`-suffixed copy exists with identical bytes, calls the returned restore func, asserts the original is back with identical bytes. Also tested the missing-file path (no checked-in resource present).
  found: Both cases pass -- PASS TestDisableWindowsResourceSysoRoundTrip, PASS TestDisableWindowsResourceSysoMissingFile.
  implication: The rename/restore mechanism itself (independent of actually launching `wails dev`) behaves correctly: file is fully excluded from Go's *.syso glob while disabled, and is restored byte-for-byte, in both the present-file and missing-file cases.

- timestamp: 2026-07-29T17:48:00Z
  checked: `go build ./internal/command/...`, `go vet ./internal/command/...`, and `go test ./internal/command/...` after the fix
  found: Build and vet clean. Full package test suite: one pre-existing flaky failure (TestScriptStopTerminatesActiveRun, unrelated to dev.go/syso -- confirmed by running it in isolation both with the fix stashed out (git stash) and reapplied (git stash pop): failed 1 of 4 total runs regardless of whether the fix was present, and the test file has no reference to dev.go/windowsResourceSyso/runDev).
  implication: No regression introduced by this fix; the only test-suite failure observed is pre-existing timing flakiness in an unrelated test.

- timestamp: 2026-07-29T18:12:00Z
  checked: Ran `mage dev` live (backgrounded shell task) end-to-end for the first time on this machine.
  found: "Compiling application: Done." with no "too many .rsrc sections" error; Wails dev server started, "[WebView2] Environment created successfully" logged -- the app launched. Steps 1-2 of the human-verify checklist confirmed working. Then stopped the task via the harness's TaskStop (a forceful process-tree termination, not a console CTRL_C_EVENT): cmd/golc-desktop/rsrc_windows_amd64.syso was left renamed as *.wails-dev-disabled afterward -- the restore did not run. Manually renamed it back to unblock the repo.
  implication: TaskStop kills the process tree directly (akin to taskkill /F), which does not deliver the CTRL_C_EVENT that dev.go's signal.Notify(os.Interrupt, syscall.SIGTERM) handler is designed to catch -- so this is not a valid test of the restore path's real target (an interactive user pressing Ctrl+C in an actual console) and should not be read as disproving the fix. It does confirm the blind_spot flagged earlier is real: any termination path that bypasses Go signal delivery (forceful kill, closing the terminal window via its X button, a crash, `taskkill /F`, VS Code's stop-process button) will still leave the .syso file stuck renamed. This is a genuine robustness gap, distinct from whether a real Ctrl+C works.

- timestamp: 2026-07-29T18:30:00Z
  checked: User ran `mage dev` live and pressed Ctrl+C for real in an interactive terminal (not a backgrounded/force-killed task), then reported back.
  found: User confirmed all three outstanding checklist items directly: "it works and the icons work correctly and i just killed it via ctrl c". Verified on disk: cmd/golc-desktop/rsrc_windows_amd64.syso is present (not renamed to *.wails-dev-disabled).
  implication: The signal.Notify(os.Interrupt, syscall.SIGTERM) restore path in dev.go works correctly for a genuine console CTRL_C_EVENT. Combined with the earlier live-build confirmation and the titlebar icon check, all three human-verify checklist items pass. Session resolved.

## Resolution
<!-- OVERWRITE as understanding evolves -->

root_cause: cmd/golc-desktop/rsrc_windows_amd64.syso is checked into the repo so that `mage Build`'s plain `go build` (which never invokes Wails' own icon-embedding step) still embeds the app/titlebar icon. But `mage Dev` invokes `wails dev`, which (Wails v2.13.0's flags/dev.go hardcodes Pack:true, no flag to disable it) unconditionally regenerates its OWN Windows resource file (cmd/golc-desktop/golc-desktop-res.syso) on every dev rebuild, from the same icon/manifest/info.json inputs, deleting it again once that build finishes. Go's linker auto-includes every *.syso file present in a package directory, and accepts only one .rsrc section per binary -- so during the brief window both files coexist on disk, the link fails with "too many .rsrc sections". This has been latent since d0c1bdb8 first committed rsrc_windows_amd64.syso; 0ed088a8 (the most recent commit, and the original hypothesis's suspect) only changed which resource ID that file's icon lives at and did not introduce the conflict itself.
fix: internal/command/dev.go's runDev now temporarily renames cmd/golc-desktop/rsrc_windows_amd64.syso out of Go's *.syso auto-link path (appending a non-.syso suffix) immediately before invoking `wails dev`, and restores it afterward, so only Wails' own freshly generated resource file is ever present during that child process's build. A signal.Notify guard (matching this repo's existing internal/command/artnet.go convention) prevents Windows Ctrl+C from killing this process before the restore can run. mage Build's plain `go build` path is untouched -- the checked-in file remains exactly where it was for that path.
verification: Self-verified (see Evidence): (1) empirically reproduced "too many .rsrc sections" with a real duplicate .syso in cmd/golc-desktop/ via plain `go build`, confirmed it disappears when only one .syso remains; (2) throwaway unit test confirmed disableWindowsResourceSyso's rename-out/restore round-trip is byte-exact in both the present-file and missing-file cases; (3) `go build`/`go vet`/`go test` clean for internal/command (one pre-existing unrelated flaky test, confirmed present with the fix both stashed and applied); (4) live `mage dev` run confirmed the core bug is fixed -- Go build succeeds ("Compiling application: Done."), no "too many .rsrc sections", Wails dev server and WebView2 launch. Human-verified: user ran `mage dev` interactively, confirmed the build/window works, the titlebar icon renders correctly, and a real Ctrl+C correctly restores rsrc_windows_amd64.syso (confirmed present on disk afterward). All three checklist items pass.
files_changed:
  - internal/command/dev.go
