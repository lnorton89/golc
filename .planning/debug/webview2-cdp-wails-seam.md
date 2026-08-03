---
status: awaiting_human_verify
trigger: "Packaged WebView2 dialog proof cannot expose CDP because Wails v2 drops the browser-argument environment override."
created: 2026-08-03T00:00:00Z
updated: 2026-08-03T04:12:00Z
---

## Current Focus

hypothesis: Confirmed — the pinned Node/local Playwright CLI and bounded cleanup remove the final harness blockers after CDP discovery.
test: Complete — isolated evidence reports passed, the final proof's user-data and overlay directories are absent, port 19327 has no listener, and no relevant proof process remains.
expecting: The atomic commit contains exactly the six authorized seam files plus this active debug session; all concurrent worktree changes remain uncommitted and untouched.
next_action: Re-stage this session update, assert the index still contains exactly the seven authorized paths, create the atomic fix commit, and inspect the resulting commit and remaining worktree before returning the human-verification checkpoint.
reasoning_checkpoint:
  hypothesis: "The packaged build and CDP seam succeeded, but ambient npx cannot resolve Playwright despite the installed local CLI; immediate profile deletion then throws while WebView2 descendants are still releasing files and masks the primary error."
  confirming_evidence:
    - "The proof build compiled cleanly against the local replacement and progressed beyond the CDP polling loop into the Playwright command."
    - "npx reported 'could not determine executable to run' while frontend/node_modules/@playwright/test/cli.js and the local .bin shims exist."
    - "The finally block failed deleting BrowserMetrics immediately after stopping the host, replacing the primary proof error."
  falsification_test: "If direct pinned Node plus the local Playwright CLI still cannot start, ambient npx resolution is not the remaining test-launch cause."
  fix_rationale: "Using the already bootstrap-pinned Node and exact installed CLI removes host npm/npx ambiguity; bounded cleanup waits for the process tree to release files and never masks the proof result."
  blind_spots: "The Playwright test may reveal a later UI assertion failure after it can launch; that would be a separate proof result."
reasoning_checkpoint:
  hypothesis: "Go rejects replacement targets beneath GOMODCACHE, but accepts overlays against an equivalent local replacement module outside GOMODCACHE."
  confirming_evidence:
    - "The packaged proof fails exactly with Go's explicit GOMODCACHE replacement prohibition before compilation."
    - "The pinned go-webview2 module is small (397 files, 1,351,461 bytes), hash-locked, and has its own go.mod, so a temporary byte-for-byte copy is bounded."
    - "The repository go.mod has no existing replace directives, so a temporary modfile can add one unambiguously."
  falsification_test: "If Go still rejects the copied target or the build resolves the module-cache copy despite the temporary replace directive, the local-replacement design is invalid."
  fix_rationale: "The temporary modfile changes dependency resolution only for the opt-in proof process; the overlay then targets the copied pinned source, avoiding Go's cache-integrity guard while leaving go.mod, go.sum, and ordinary builds untouched."
  blind_spots: "Windows path encoding in the temporary replace directive and overlay JSON must be accepted by Go; the exact packaged build tests both."
reasoning_checkpoint:
  hypothesis: "The checked-in overlay source is treated as a standalone package during ordinary ./... traversal because windows && !native_webview2loader is true, causing its references to go-webview2 edge-private identifiers to fail compilation."
  confirming_evidence:
    - "go test ./internal/command invokes quick vet, which reports undefined iCoreWebView2CreateCoreWebView2EnvironmentCompletedHandler in scripts/ci/webview2-cdp-overlay/create_env_go.go."
    - "The overlay file declares package edge but lives outside the go-webview2 edge package, and its current Windows constraint is active on this host."
  falsification_test: "If ordinary go list/vet still discovers the overlay directory after adding an inactive proof-only tag, the standalone-package discovery mechanism is not the cause."
  fix_rationale: "A dedicated tag keeps the replacement source invisible to all ordinary repository builds while projectBuildGoArguments enables it only for the already opt-in overlay build."
  blind_spots: "Go overlay replacement must honor the replacement file's proof-only build constraint; the end-to-end packaged build is required to confirm selection."
tdd_checkpoint: null

## Symptoms

expected: The packaged GOLC desktop application starts WebView2 with CDP enabled so Playwright can connect over a loopback port and verify the shared dialog contract.
actual: Chromium proof passes and the packaged app builds, but WebView2 starts without a debugging listener even when WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS and WEBVIEW2_USER_DATA_FOLDER are set before launch.
errors: GOLC_DIALOG_PROOF_CDP_TIMEOUT after 45 seconds; live inspection confirms msedgewebview2.exe has no remote-debugging argument.
reproduction: Run scripts/ci/run-packaged-dialog-proof.ps1 after mage Build.
started: Phase 13 packaged dialog feasibility work.

## Eliminated

- hypothesis: A public Wails Windows option or GOLC pre-run environment assignment can inject arbitrary WebView2 browser arguments.
  evidence: Wails v2.13.0 exposes user-data and browser executable paths but no additional-browser-arguments field; go-webview2's Go loader unconditionally sets WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS, WEBVIEW2_USER_DATA_FOLDER, and related overrides to empty during init and immediately before environment creation.
  timestamp: 2026-08-03T03:12:00Z
- hypothesis: The existing native_webview2loader build tag preserves external WebView2 environment overrides.
  evidence: webviewloader/native_module.go also calls preventEnvAndRegistryOverrides during init and before environment creation; it rewrites WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS to the explicit string supplied by Wails, which contains no CDP switch.
  timestamp: 2026-08-03T03:18:00Z

## Evidence

- timestamp: 2026-08-03T00:00:00Z
  checked: Live packaged launch and Wails v2 Windows source.
  found: Wails populates only its own browser argument slice and user-data path; its public Windows options expose no additional-browser-arguments setting, so the harness environment override never reaches msedgewebview2.exe.
  implication: Retrying the existing harness cannot satisfy the packaged CDP proof.
- timestamp: 2026-08-03T03:00:00Z
  checked: Repository inventory and worktree state.
  found: The desktop has a single cmd/golc-desktop/main.go Wails entrypoint; the packaged proof is scripts/ci/run-packaged-dialog-proof.ps1; unrelated modifications exist in .planning/STATE.md and Phase 13 planning files and must be preserved.
  implication: Any feasible seam should be localized to the desktop startup/build harness and must avoid broad module or resource changes.
- timestamp: 2026-08-03T03:04:00Z
  checked: Complete desktop entrypoint, Wails config, Mage facade, proof harness, and module declaration.
  found: cmd/golc-desktop/main.go passes no Windows-specific options to wails.Run; the proof sets WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS and WEBVIEW2_USER_DATA_FOLDER only in the child environment; Wails v2.13.0 and github.com/wailsapp/go-webview2 v1.0.22 are pinned in go.mod, which is protected from modification.
  implication: A repository seam must either populate an existing options.App field before wails.Run or build a test-only adapter around the unchanged upstream modules; dependency replacement is out of scope.
- timestamp: 2026-08-03T03:07:00Z
  checked: Symbol search across Wails v2.13.0 and go-webview2 v1.0.22.
  found: Wails' public Windows options contain WebviewBrowserPath but no AdditionalBrowserArguments or UserDataFolder field; go-webview2 exposes WithAdditionalBrowserArguments and WithUserDataFolder internally to its environment creation path.
  implication: The low-level library supports CDP, but Wails v2 does not publicly forward the relevant creation options; feasibility depends on whether Wails has another injectable constructor boundary.
- timestamp: 2026-08-03T03:12:00Z
  checked: Wails Windows frontend and go-webview2 Go loader environment creation.
  found: Wails creates edge.NewChromium internally, later passing its AdditionalBrowserArgs to WebView2; the default go-webview2 loader calls preventEnvAndRegistryOverrides during package init and again immediately before creation, explicitly blanking WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS and WEBVIEW2_USER_DATA_FOLDER.
  implication: The observed missing command-line switch is deterministic, and neither setting the variables earlier nor adding a normal options.App helper can work through the default loader.
- timestamp: 2026-08-03T03:18:00Z
  checked: native_webview2loader variant and Wails setupChromium.
  found: The legacy native loader also replaces external overrides with Wails' explicit values; Wails setupChromium only adds SmartScreen, GPU, and renderer-integrity switches and exposes no arbitrary argument hook.
  implication: Changing loader variants cannot expose CDP; a feasible test-only seam must alter the exact go-webview2 environment bridge used by the proof build.
- timestamp: 2026-08-03T03:34:00Z
  checked: Focused tests and PowerShell parse.
  found: support/webview2cdp and internal/command focused tests pass; the packaged proof script parses with no PowerShell syntax errors.
  implication: The validated port/data resolution and explicit build-overlay GOFLAGS path behave as designed; end-to-end WebView2 launch remains to verify.
- timestamp: 2026-08-03T03:37:00Z
  checked: First end-to-end overlay build.
  found: The Go command received the strconv-quoted overlay path as a literal Windows filename and failed before compilation; the proof's generated temporary path itself contains no whitespace.
  implication: The overlay mechanism reached the pinned build exactly as intended, but GOFLAGS must carry this whitespace-free path without quote characters.
- timestamp: 2026-08-03T03:39:00Z
  checked: Complete draft files, scoped worktree diff, untracked support inventory, and whitespace validation.
  found: The seam is confined to the six listed implementation/test files; support contains only webview2cdp; protected paths are untouched; projectBuildGoArguments adds -overlay=<path> directly after build and returns a copy unchanged for non-build or unset inputs; git diff --check is clean.
  implication: The direct-argv adjustment is scoped and ready for focused formatting and test execution without touching unrelated planning edits.
- timestamp: 2026-08-03T03:40:00Z
  checked: Pinned formatter location.
  found: The first formatter lookup omitted the Go distribution's nested go directory; the installed formatter is .tools/toolchains/go/1.26.5/windows-amd64/go/bin/gofmt.exe.
  implication: No source files were changed by the failed lookup; formatting can proceed with the resolved pinned path.
- timestamp: 2026-08-03T03:43:00Z
  checked: Pinned-format and focused package test run.
  found: support/webview2cdp passes, but internal/command's offline acceptance invokes vet ./... and fails in the new scripts/ci/webview2-cdp-overlay package because its replacement source references private edge-package identifiers outside that package. Separate artnet, docs, and scriptsdk failures concern unrelated concurrent work.
  implication: The overlay source needs proof-only build selection before end-to-end verification; unrelated package-test failures must not be altered by this seam.
- timestamp: 2026-08-03T03:49:00Z
  checked: Narrowed regression checks after proof-only tag injection.
  found: TestProjectBuildGoArgumentsDialogProofOverlay and all support/webview2cdp tests pass; ordinary go list ./... excludes the overlay package; PowerShell parsing and git diff whitespace checks pass.
  implication: The seam-owned vet regression is fixed, and the next discriminating test is the packaged WebView2 proof itself.
- timestamp: 2026-08-03T03:52:00Z
  checked: Packaged proof build on port 19327 with isolated temporary evidence.
  found: Go rejects the overlay before compilation with "overlay contains a replacement for ... Files beneath GOMODCACHE ... must not be replaced." The shared Phase 13 evidence was not written.
  implication: Direct argv quoting is fixed but cannot overcome Go's module-cache overlay prohibition; the proof must target a temporary local module replacement or the seam is infeasible.
- timestamp: 2026-08-03T03:56:00Z
  checked: Pinned go-webview2 module scope and repository module metadata.
  found: The pinned dependency contains 397 files totaling 1,351,461 bytes, its source hash matches the lock, and the repository go.mod has no replace directive.
  implication: A temporary byte-for-byte local module plus one temporary replace directive is bounded and does not need any protected-file edit.
- timestamp: 2026-08-03T04:03:00Z
  checked: Focused checks and second packaged proof on port 19327.
  found: All focused seam checks pass; the packaged build compiles cleanly and reaches the Playwright stage, proving the CDP endpoint was discovered. Ambient npx then fails to resolve an executable even though the local Playwright CLI exists, and immediate profile cleanup throws on a locked BrowserMetrics file.
  implication: The modfile/local-copy overlay mechanism is confirmed through CDP discovery; deterministic Playwright launch and non-masking cleanup are the final harness blockers.
- timestamp: 2026-08-03T04:08:00Z
  checked: Focused regression checks after pinned Node/CLI and bounded cleanup changes.
  found: The exact command argv test and all webview2cdp helper tests pass; the proof script parses; diff whitespace validation passes.
  implication: No focused regression remains, but the final packaged Playwright run has not been repeated, so the seam is intentionally uncommitted.
- timestamp: 2026-08-03T04:10:00Z
  checked: Authorized final packaged dialog proof on CDP port 19327 with isolated evidence.
  found: The script exited 0; the pinned build compiled every project package cleanly; Playwright ran the Chromium dialog feasibility test and reported 1 passed in 2.2 seconds.
  implication: The full packaged WebView2 CDP seam now passes end-to-end; commit authorization is conditioned only on confirming evidence, cleanup, and exact worktree scope.
- timestamp: 2026-08-03T04:11:00Z
  checked: Final evidence JSON, CDP listener/process state, proof resource cleanup, worktree inventory, scoped whitespace, and protected paths.
  found: Evidence status is passed with test exit_code 0 and the pinned local CLI command; port 19327 and relevant processes are absent; the final proof's PID-specific user-data and overlay paths are removed; the seam directories contain exactly the expected files; scoped diff-check is clean; the index is empty; protected paths are unchanged.
  implication: Self-verification and cleanup are complete, and the exact seven authorized paths can be staged without including or reverting concurrent changes.
- timestamp: 2026-08-03T04:12:00Z
  checked: Explicitly staged index and remaining worktree after self-verification.
  found: The index contains exactly the six authorized seam files plus this debug session; cached diff-check is clean. Concurrent changes in planning state, Phase 13 artifacts, and patterns remain unstaged.
  implication: One scoped atomic commit can be created without including or reverting any concurrent work.

## Resolution

root_cause: go-webview2 v1.0.22 deliberately clears WebView2 environment overrides in both loader implementations and Wails v2.13.0 supplies no public arbitrary-browser-argument option, so the proof harness's CDP variables can never reach the browser process.
fix: Added a hash/version-locked Go build overlay used only by the packaged proof, a validated GOLC-only CDP configuration helper, and explicit Mage build support for the proof overlay while preserving ordinary GOFLAGS and runtime behavior.
verification: Focused argument/helper tests, ordinary package exclusion, PowerShell parsing, and whitespace checks pass. The authorized final packaged proof exited 0: the pinned build compiled every project package, CDP discovery succeeded, and Playwright reported the Chromium dialog feasibility test passed (1 passed in 2.2 seconds). Isolated evidence reports status passed and test exit_code 0; the final proof's listener, relevant processes, user-data directory, and overlay workspace are absent after cleanup.
files_changed:
  - internal/command/build.go
  - internal/command/build_overlay_test.go
  - support/webview2cdp/override.go
  - support/webview2cdp/override_test.go
  - scripts/ci/webview2-cdp-overlay/create_env_go.go
  - scripts/ci/run-packaged-dialog-proof.ps1
