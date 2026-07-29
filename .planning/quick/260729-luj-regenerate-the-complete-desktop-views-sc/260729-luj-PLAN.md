---
phase: quick-desktop-views-screenshot-refresh
plan: 260729-luj
type: execute
wave: 1
depends_on: []
files_modified:
  - frontend/e2e/desktop-view-docs.spec.ts
  - site/package.json
  - site/scripts/regenerate-desktop-views.mjs
  - site/public/desktop-views/show-overview.png
  - site/public/desktop-views/show-shows.png
  - site/public/desktop-views/show-save-recovery.png
  - site/public/desktop-views/show-settings.png
  - site/public/desktop-views/build-fixture-library.png
  - site/public/desktop-views/build-patch-pools.png
  - site/public/desktop-views/build-scenes-looks.png
  - site/public/desktop-views/build-scripts.png
  - site/public/desktop-views/operate-operator-surface.png
  - site/public/desktop-views/operate-midi-mapping.png
  - site/public/desktop-views/output-artnet.png
  - site/public/desktop-views/output-diagnostics.png
  - site
autonomous: true
requirements: [DOCS-SCREENSHOTS-01, DOCS-RELEASE-01]
must_haves:
  truths:
    - "All twelve catalog destinations are recaptured from the current frontend source at exactly 1920x1080 with healthy deterministic Wails bindings and no reused stale development server."
    - "Every generated PNG is inspected at original resolution and contains the intended current workspace without stale UI, error banners, overlap, clipping, or jumbled text."
    - "The site repository exposes one maintained npm command that invokes the parent's deterministic capture harness, preserves paths containing spaces, updates site/public/desktop-views, and fails clearly when the expected parent checkout is absent."
    - "Completed capture/layout guards, 274 frontend tests/build, and normal Windows/local site lint, typecheck, build, links, metadata, interaction, and focused visual checks pass without weakening assertions."
    - "Only intended screenshot/tooling changes are committed; site/deno.lock remains untracked and the two protected cmd/golc-desktop resource paths remain exactly in their execution-start state."
    - "The site submodule commit is pushed before the parent gitlink/tooling commit, and the parent is pushed before production deployment."
    - "The pinned npm deploy workflow publishes production only after stale preview processes are stopped, and the live route plus every screenshot asset is verified."
  artifacts:
    - path: frontend/e2e/desktop-view-docs.spec.ts
      provides: "Deterministic 1920x1080 catalog-driven capture and health/layout guards."
      contains: "DOCUMENTATION_VIEWPORT"
    - path: site/public/desktop-views
      provides: "Exactly twelve current desktop workspace PNGs."
    - path: site/package.json
      provides: "Discoverable site-owned npm entrypoint for Desktop Views regeneration."
      contains: "docs:screenshots"
    - path: site/scripts/regenerate-desktop-views.mjs
      provides: "Cross-repository path validation and shell-free delegation to the parent capture harness."
      contains: "GOLC_DESKTOP_VIEWS_PARENT_MISSING"
  key_links:
    - from: frontend/src/shell/desktopViews.json
      to: site/public/desktop-views
      via: "The Playwright capture enumerates the canonical catalog and writes each declared screenshot."
      pattern: "desktopViews.groups.flatMap"
    - from: site/package.json
      to: site/scripts/regenerate-desktop-views.mjs
      via: "The site-owned docs:screenshots npm command calls the maintained wrapper."
      pattern: "regenerate-desktop-views.mjs"
    - from: site/scripts/regenerate-desktop-views.mjs
      to: frontend/e2e/desktop-view-docs.spec.ts
      via: "The wrapper validates the expected parent frontend checkout, then invokes its existing docs:screenshots npm command without shell interpolation."
      pattern: "docs:screenshots"
    - from: site/src/content/desktop-views.json
      to: site/public/desktop-views
      via: "The public guide resolves the same twelve catalog screenshot paths."
      pattern: "/desktop-views/"
    - from: site/package.json
      to: site/netlify.toml
      via: "npm run deploy invokes the pinned Netlify production build and publishes site/out."
      pattern: "netlify deploy --build --prod"
    - from: site
      to: "parent repository gitlink"
      via: "The parent records the exact already-pushed site commit."
      pattern: "160000"
---

<objective>
Regenerate and release the complete Desktop Views screenshot set from the current GUI source.

Purpose: Public desktop documentation must match the GUI now in the worktree, remain visually trustworthy, and reach production through the established submodule and Netlify release contract.
Output: A maintained site-owned regeneration command, twelve reviewed 1920x1080 screenshots, passing local site gates, site-first and parent-second pushed commits, and a verified production deployment.
</objective>

<execution_context>
@C:/Users/Lawrence/.codex/gsd-core/workflows/execute-plan.md
@C:/Users/Lawrence/.codex/gsd-core/templates/summary.md
</execution_context>

<context>
@AGENTS.md
@.planning/STATE.md
@.planning/quick/260729-gq6-add-a-maintainable-desktop-views-documen/260729-gq6-SUMMARY.md
@.planning/quick/260729-ia1-add-a-pinned-npm-deploy-script-for-expli/260729-ia1-SUMMARY.md
@frontend/src/shell/desktopViews.json
@frontend/e2e/desktop-view-docs.spec.ts
@frontend/playwright.config.ts
@frontend/package.json
@site/AGENTS.md
@site/package.json
@site/netlify.toml
@site/src/content/desktop-views.json
@site/tests/visual.spec.ts
@site/tests/metadata.spec.ts
@site/playwright.config.ts

The current GUI edits are the source-of-truth input. Do not revert, restyle, or otherwise modify frontend GUI source to make captures pass. A targeted change to `frontend/e2e/desktop-view-docs.spec.ts` is allowed only when the current GUI requires its deterministic Wails mock/settle/guard tooling to be brought up to date.

The protected parent paths are `cmd/golc-desktop/rsrc_windows_amd64.syso` and `cmd/golc-desktop/rsrc_windows_amd64.syso.wails-dev-disabled`; at review time the tracked `.syso` is clean/present and the `.wails-dev-disabled` path is absent. The site worktree contains unrelated untracked `deno.lock`. Record their exact execution-start existence, Git status, and SHA-256 where present, then never stage, commit, delete, restore, create, or rewrite those paths. Use explicit pathspecs for every commit and require the recorded states/fingerprints to remain unchanged even if the live starting state differs from this review-time snapshot.

`site/` is an independent git repository and submodule. Its screenshot-regeneration command must work when invoked from that repository inside the expected parent checkout, and must stop with a stable, actionable error when the parent frontend is absent. Commit and push the site command/wrapper and refreshed public assets first. Then commit and push the parent capture-tooling change, if any, and the resulting `site` gitlink. Netlify does not auto-build: after both pushes, deploy only from `site/` with the existing `npm run deploy` script and existing linked-site state.

Completed evidence from work already performed before this clarification remains valid and must be carried into the summary: two clean-server capture passes produced hash-identical sets of twelve 1920x1080 PNGs; all 12/12 images passed individual original-resolution visual review; and the frontend passed 274 tests plus its full build. Do not discard, weaken, or relabel this evidence, and do not require a Linux container or Linux pixel-baseline update to repeat it.
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add the site-owned regeneration command and preserve capture evidence</name>
  <files>site/package.json, site/scripts/regenerate-desktop-views.mjs, frontend/e2e/desktop-view-docs.spec.ts, site/public/desktop-views/show-overview.png, site/public/desktop-views/show-shows.png, site/public/desktop-views/show-save-recovery.png, site/public/desktop-views/show-settings.png, site/public/desktop-views/build-fixture-library.png, site/public/desktop-views/build-patch-pools.png, site/public/desktop-views/build-scenes-looks.png, site/public/desktop-views/build-scripts.png, site/public/desktop-views/operate-operator-surface.png, site/public/desktop-views/operate-midi-mapping.png, site/public/desktop-views/output-artnet.png, site/public/desktop-views/output-diagnostics.png</files>
  <action>
Record both worktree status listings and execution-start existence/Git-status/SHA-256 fingerprints for the three protected paths exactly as specified. Before invoking the new wrapper, record a filename-to-SHA-256 map of the twelve already-reviewed PNGs. Preserve the GUI and the completed evidence: the current twelve PNGs already passed two hash-identical 1920x1080 captures, 12/12 original-resolution review, and the unchanged capture harness guards.

Add a `docs:screenshots` script to `site/package.json` that runs a small site-owned `scripts/regenerate-desktop-views.mjs` wrapper. In the wrapper, resolve the expected parent `frontend/` directory from `import.meta.url` and `fileURLToPath`, not from the caller's current directory or a hand-concatenated command string. Validate that the parent frontend manifest, canonical desktop catalog, and capture spec exist; otherwise print a stable `GOLC_DESKTOP_VIEWS_PARENT_MISSING` diagnostic naming the expected absolute parent path and exit nonzero before changing any image.

Delegate to the parent's existing `docs:screenshots` npm script with `child_process.spawnSync` using an executable-plus-argument array, `shell: false`, the resolved frontend directory as `cwd`, inherited stdio, and `CI=1`. Prefer the current npm lifecycle's `npm_execpath` invoked through `process.execPath` so Windows command lookup and spaces in the checkout path never require shell quoting. Propagate spawn errors, signals, and nonzero status without conversion. Do not duplicate catalog entries, Playwright logic, Wails bindings, viewport, health assertions, or layout guards in the site wrapper, and add no dependency.

Run `npm run docs:screenshots` from inside `site/`, compare the resulting inventory, dimensions, and hashes to the pre-run reviewed map, and require exact equality. In a GUID-named temporary checkout whose absolute path contains spaces, copy only the wrapper and create minimal disposable site/frontend manifests plus the three required parent files; run the site npm command there and require successful shell-free delegation with `CI=1`. Then remove only that probe's disposable frontend and invoke the copied wrapper again, proving a nonzero exit whose stable `GOLC_DESKTOP_VIEWS_PARENT_MISSING` diagnostic includes the expected absolute frontend path. Resolve and validate both deletion targets beneath the OS temporary directory before recursive cleanup; neither probe may read or modify the real screenshot outputs.
  </action>
  <verify>
    <automated>
$protectionRecord = Join-Path ([System.IO.Path]::GetTempPath()) 'golc-260729-luj-protected-state.json'
$protectedState = [ordered]@{
  rootStatus = @(git status --short -- 'cmd/golc-desktop/rsrc_windows_amd64.syso' 'cmd/golc-desktop/rsrc_windows_amd64.syso.wails-dev-disabled')
  sysoExists = Test-Path -LiteralPath 'cmd/golc-desktop/rsrc_windows_amd64.syso'
  sysoHash = $(if (Test-Path -LiteralPath 'cmd/golc-desktop/rsrc_windows_amd64.syso') { (Get-FileHash -Algorithm SHA256 -LiteralPath 'cmd/golc-desktop/rsrc_windows_amd64.syso').Hash } else { $null })
  disabledExists = Test-Path -LiteralPath 'cmd/golc-desktop/rsrc_windows_amd64.syso.wails-dev-disabled'
  disabledHash = $(if (Test-Path -LiteralPath 'cmd/golc-desktop/rsrc_windows_amd64.syso.wails-dev-disabled') { (Get-FileHash -Algorithm SHA256 -LiteralPath 'cmd/golc-desktop/rsrc_windows_amd64.syso.wails-dev-disabled').Hash } else { $null })
  denoStatus = @(git -C site status --short -- deno.lock)
  denoHash = $(if (Test-Path -LiteralPath 'site/deno.lock') { (Get-FileHash -Algorithm SHA256 -LiteralPath 'site/deno.lock').Hash } else { $null })
}
$protectedState | ConvertTo-Json -Depth 3 | Set-Content -LiteralPath $protectionRecord -Encoding UTF8
$reviewedHashes = @{}; Get-ChildItem site/public/desktop-views/*.png | ForEach-Object { $reviewedHashes[$_.Name] = (Get-FileHash -Algorithm SHA256 -LiteralPath $_.FullName).Hash }
Push-Location site; try { npm run docs:screenshots; if ($LASTEXITCODE -ne 0) { throw "site docs:screenshots failed with exit $LASTEXITCODE" } } finally { Pop-Location }
$expected = (Get-Content -Raw frontend/src/shell/desktopViews.json | ConvertFrom-Json).groups.views.screenshot | ForEach-Object { Split-Path $_ -Leaf } | Sort-Object
$actual = Get-ChildItem site/public/desktop-views/*.png | Select-Object -ExpandProperty Name | Sort-Object
if (Compare-Object $expected $actual) { throw 'desktop-view asset inventory mismatch' }
Add-Type -AssemblyName System.Drawing
Get-ChildItem site/public/desktop-views/*.png | ForEach-Object {
  $image = [System.Drawing.Image]::FromFile($_.FullName)
  try { if ($image.Width -ne 1920 -or $image.Height -ne 1080) { throw "wrong dimensions: $($_.Name)" } } finally { $image.Dispose() }
  if ($reviewedHashes[$_.Name] -ne (Get-FileHash -Algorithm SHA256 -LiteralPath $_.FullName).Hash) { throw "wrapper changed reviewed screenshot bytes: $($_.Name)" }
}
$tempRoot = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath())
$probeRoot = [System.IO.Path]::GetFullPath((Join-Path $tempRoot ('golc site wrapper probe ' + [guid]::NewGuid())))
$probePrefix = $tempRoot.TrimEnd('\') + '\'
if (-not $probeRoot.StartsWith($probePrefix, [System.StringComparison]::OrdinalIgnoreCase)) { throw 'unsafe wrapper probe root' }
$probeSite = Join-Path $probeRoot 'site'
$probeScriptDir = Join-Path $probeSite 'scripts'
$probeFrontend = Join-Path $probeRoot 'frontend'
New-Item -ItemType Directory -Path $probeScriptDir, (Join-Path $probeFrontend 'src/shell'), (Join-Path $probeFrontend 'e2e') -Force | Out-Null
Copy-Item -LiteralPath site/scripts/regenerate-desktop-views.mjs -Destination $probeScriptDir
Set-Content -LiteralPath (Join-Path $probeSite 'package.json') -Encoding Ascii -Value '{"private":true,"type":"module","scripts":{"docs:screenshots":"node scripts/regenerate-desktop-views.mjs"}}'
Set-Content -LiteralPath (Join-Path $probeFrontend 'package.json') -Encoding Ascii -Value '{"private":true,"type":"module","scripts":{"docs:screenshots":"node verify-delegation.mjs"}}'
Set-Content -LiteralPath (Join-Path $probeFrontend 'src/shell/desktopViews.json') -Encoding Ascii -Value '{}'
Set-Content -LiteralPath (Join-Path $probeFrontend 'e2e/desktop-view-docs.spec.ts') -Encoding Ascii -Value '// wrapper probe'
Set-Content -LiteralPath (Join-Path $probeFrontend 'verify-delegation.mjs') -Encoding Ascii -Value "import { writeFileSync } from 'node:fs'; if (process.env.CI !== '1') process.exit(7); writeFileSync('delegated.ok', 'ok');"
try {
  Push-Location $probeSite; try { npm run docs:screenshots; if ($LASTEXITCODE -ne 0) { throw "space-path delegation failed with exit $LASTEXITCODE" } } finally { Pop-Location }
  if (-not (Test-Path -LiteralPath (Join-Path $probeFrontend 'delegated.ok'))) { throw 'space-path delegation did not reach parent npm script' }
  $probeFrontendFull = [System.IO.Path]::GetFullPath($probeFrontend)
  if (-not $probeFrontendFull.StartsWith(($probeRoot.TrimEnd('\') + '\'), [System.StringComparison]::OrdinalIgnoreCase)) { throw 'unsafe disposable frontend target' }
  Remove-Item -LiteralPath $probeFrontendFull -Recurse -Force
  $probeOutput = &amp; node (Join-Path $probeScriptDir 'regenerate-desktop-views.mjs') 2&gt;&amp;1
  $probeExit = $LASTEXITCODE
  $probeText = $probeOutput -join "`n"
  if ($probeExit -eq 0 -or $probeText -notmatch 'GOLC_DESKTOP_VIEWS_PARENT_MISSING' -or $probeText -notmatch [regex]::Escape($probeFrontendFull)) { throw 'missing-parent diagnostic contract failed' }
} finally {
  if (-not $probeRoot.StartsWith($probePrefix, [System.StringComparison]::OrdinalIgnoreCase)) { throw 'unsafe wrapper probe cleanup target' }
  if (Test-Path -LiteralPath $probeRoot) { Remove-Item -LiteralPath $probeRoot -Recurse -Force }
}
    </automated>
  </verify>
  <done>The site-owned command safely delegates to the unchanged deterministic parent harness, reproduces the already-reviewed twelve-image set from a path containing spaces, fails clearly without the expected parent checkout, and preserves all completed evidence and unrelated files.</done>
</task>

<task type="auto">
  <name>Task 2: Retain completed visual/frontend evidence and pass local site gates</name>
  <files>site/package.json, site/scripts/regenerate-desktop-views.mjs, site/public/desktop-views/show-overview.png, site/public/desktop-views/show-shows.png, site/public/desktop-views/show-save-recovery.png, site/public/desktop-views/show-settings.png, site/public/desktop-views/build-fixture-library.png, site/public/desktop-views/build-patch-pools.png, site/public/desktop-views/build-scenes-looks.png, site/public/desktop-views/build-scripts.png, site/public/desktop-views/operate-operator-surface.png, site/public/desktop-views/operate-midi-mapping.png, site/public/desktop-views/output-artnet.png, site/public/desktop-views/output-diagnostics.png</files>
  <action>
Carry the existing per-file original-resolution inspection notes for all twelve PNGs and the completed frontend result (274 tests and full build) into the summary. Task 1's site-owned wrapper run must leave the same hashes, so these completed observations remain evidence for the delivered assets. If hashes differ, the prior visual evidence no longer applies and release blocks pending a new original-resolution review; do not silently reuse stale observations.

Run normal Windows/local site checks from the committed lock state: lint, typecheck, static build, and link crawl. Run `tests/metadata.spec.ts` and the non-snapshot interaction/layout tests in `tests/visual.spec.ts` with a focused Playwright grep covering mobile overflow, grouped selector/detail, keyboard selection, both lightbox flows, and docs navigation behavior. These browser checks exercise the refreshed images and responsive layout without comparing against or updating `chromium-linux` pixel baselines. Do not run a Linux container, generate platform aliases, update any pixel baseline, increase tolerances, or skip the capture spec's existing top-bar and health/layout guards.
  </action>
  <verify>
    <automated>npm --prefix site ci; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; npm --prefix site run lint; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; npm --prefix site run typecheck; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; npm --prefix site run build; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; npm --prefix site run test:links; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; npm --prefix site exec -- playwright test tests/metadata.spec.ts tests/visual.spec.ts --grep "desktop views social image metadata|mobile menu opens and closes|desktop views remain readable|desktop views expose one grouped selector|desktop views keyboard navigation|desktop views lightbox|resources dropdown|docs navigation"; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }</automated>
    <human-check>Confirm the summary retains the completed 12/12 original-resolution results, two hash-identical 1920x1080 capture passes, and frontend 274-test/build result, and that Task 1 reproduced the same hashes.</human-check>
  </verify>
  <done>The completed capture, 12/12 visual, and frontend evidence remains tied to identical delivered hashes; all Windows/local site build, metadata, interaction, layout, and focused visual checks pass without touching Linux pixel baselines.</done>
</task>

<task type="auto">
  <name>Task 3: Push in submodule order, deploy explicitly, and verify production</name>
  <files>site, site/package.json, site/scripts/regenerate-desktop-views.mjs, frontend/e2e/desktop-view-docs.spec.ts</files>
  <action>
Recheck both worktrees and review binary/path diffs. Compare the two protected-file SHA-256 fingerprints and the tracked-deletion state recorded in Task 1; any difference blocks release. Inside `site/`, stage only `package.json`, `scripts/regenerate-desktop-views.mjs`, and the twelve files under `public/desktop-views/`; explicitly confirm no visual baseline changed and `deno.lock` remains untracked and unstaged. Commit the site changes, push its `master` commit to `origin`, and verify `origin/master` resolves to that exact SHA.

Return to the parent, confirm the `site` gitlink points to the pushed site SHA, and stage only `site` plus `frontend/e2e/desktop-view-docs.spec.ts` if that tooling file changed. Confirm both protected `cmd/golc-desktop` paths remain unstaged, commit, push parent `master`, and verify `origin/master` matches the new parent SHA. Do not include unrelated GUI edits in either commit; they remain the inputs already present in the checkout.

Before deployment, enumerate listening ports and process command lines for local Next, `serve`, and Netlify preview processes, including the known site ports 3000, 4173, and 8888. Resolve the owning PID, executable, command line, and parent process tree against the absolute `site/` and `site/out` paths before termination; stop only processes whose ownership of this checkout is proven, then re-enumerate and require that no proven site preview remains. If ownership is ambiguous, stop and report the blocker instead of killing the process or deploying across a possible output lock. From `site/`, execute the existing `npm run deploy` command exactly once, after the successful build and both pushes. Preserve the existing linked Netlify site; capture the site SHA, parent SHA, deploy ID/log URL, canonical production URL, and command result without exposing credentials.

Verify `https://golc-site.netlify.app/docs/desktop-views` returns 200 and the current guide markup. Derive all twelve screenshot paths from the catalog and require each canonical production asset URL to return 200, `image/png`, a valid PNG signature, 1920x1080 dimensions, and the same SHA-256 as its committed local asset so a stale CDN object cannot pass. In a real browser against production, exercise all twelve tabs and confirm each selected panel shows its corresponding fully loaded current screenshot; also confirm no broken image, overlap, clipping, or horizontal overflow is introduced. Record the browser result per tab in the execution summary. A successful CLI exit or draft URL alone is not release evidence.
  </action>
  <verify>
    <automated>
$siteSha = git -C site rev-parse HEAD
if ((git -C site rev-parse origin/master) -ne $siteSha) { throw 'site origin/master does not match the committed screenshot SHA' }
if ((git ls-tree HEAD site).Split()[2] -ne $siteSha) { throw 'parent gitlink does not match the pushed site SHA' }
if ((git rev-parse origin/master) -ne (git rev-parse HEAD)) { throw 'parent origin/master does not match HEAD' }
$protectionRecord = Join-Path ([System.IO.Path]::GetTempPath()) 'golc-260729-luj-protected-state.json'
if (-not (Test-Path -LiteralPath $protectionRecord)) { throw 'Task 1 protected-state record is missing' }
$protectedState = Get-Content -Raw -LiteralPath $protectionRecord | ConvertFrom-Json
$currentRootStatus = @(git status --short -- 'cmd/golc-desktop/rsrc_windows_amd64.syso' 'cmd/golc-desktop/rsrc_windows_amd64.syso.wails-dev-disabled')
if (Compare-Object @($protectedState.rootStatus) $currentRootStatus) { throw 'protected root resource Git states changed' }
if ([bool]$protectedState.sysoExists -ne (Test-Path -LiteralPath 'cmd/golc-desktop/rsrc_windows_amd64.syso')) { throw 'protected .syso existence changed' }
if ($protectedState.sysoHash -and $protectedState.sysoHash -ne (Get-FileHash -Algorithm SHA256 -LiteralPath 'cmd/golc-desktop/rsrc_windows_amd64.syso').Hash) { throw 'protected .syso bytes changed' }
if ([bool]$protectedState.disabledExists -ne (Test-Path -LiteralPath 'cmd/golc-desktop/rsrc_windows_amd64.syso.wails-dev-disabled')) { throw 'protected disabled-resource existence changed' }
if ($protectedState.disabledHash -and $protectedState.disabledHash -ne (Get-FileHash -Algorithm SHA256 -LiteralPath 'cmd/golc-desktop/rsrc_windows_amd64.syso.wails-dev-disabled').Hash) { throw 'protected disabled-resource bytes changed' }
if (Compare-Object @($protectedState.denoStatus) @(git -C site status --short -- deno.lock)) { throw 'protected site/deno.lock Git state changed' }
if ($protectedState.denoHash -and $protectedState.denoHash -ne (Get-FileHash -Algorithm SHA256 -LiteralPath 'site/deno.lock').Hash) { throw 'protected site/deno.lock bytes changed' }
$response = Invoke-WebRequest -UseBasicParsing https://golc-site.netlify.app/docs/desktop-views
if ($response.StatusCode -ne 200 -or $response.Content -notmatch 'Desktop Views') { throw 'production desktop-views route failed or is stale' }
Add-Type -AssemblyName System.Drawing
$paths = (Get-Content -Raw frontend/src/shell/desktopViews.json | ConvertFrom-Json).groups.views.screenshot
foreach ($path in $paths) {
  $asset = Invoke-WebRequest -UseBasicParsing ("https://golc-site.netlify.app" + $path)
  if ($asset.StatusCode -ne 200 -or $asset.Headers.'Content-Type' -notmatch '^image/png') { throw "production asset failed: $path" }
  $bytes = [byte[]]$asset.Content
  if ($bytes.Length -lt 8 -or $bytes[0] -ne 0x89 -or $bytes[1] -ne 0x50 -or $bytes[2] -ne 0x4E -or $bytes[3] -ne 0x47 -or $bytes[4] -ne 0x0D -or $bytes[5] -ne 0x0A -or $bytes[6] -ne 0x1A -or $bytes[7] -ne 0x0A) { throw "invalid PNG signature: $path" }
  $stream = [System.IO.MemoryStream]::new($bytes)
  try { $image = [System.Drawing.Image]::FromStream($stream); try { if ($image.Width -ne 1920 -or $image.Height -ne 1080) { throw "wrong production dimensions: $path" } } finally { $image.Dispose() } } finally { $stream.Dispose() }
  $localPath = Join-Path 'site/public' $path.TrimStart('/').Replace('/', [System.IO.Path]::DirectorySeparatorChar)
  $sha256 = [System.Security.Cryptography.SHA256]::Create()
  try { $remoteHash = ([System.BitConverter]::ToString($sha256.ComputeHash($bytes))).Replace('-', '') } finally { $sha256.Dispose() }
  if ((Get-FileHash -Algorithm SHA256 -LiteralPath $localPath).Hash -ne $remoteHash) { throw "production asset differs from committed local asset: $path" }
}
Remove-Item -LiteralPath $protectionRecord -Force
    </automated>
    <human-check>Against the canonical production URL, exercise all twelve catalog tabs and record one result per tab confirming the selected panel loaded the matching current screenshot with no broken image, overlap, clipping, or horizontal overflow.</human-check>
  </verify>
  <done>The exact site commit is pushed before the exact parent gitlink/tooling commit, both protected root paths remain exactly in their execution-start state, site/deno.lock remains untracked and unchanged, `npm run deploy` publishes production after stale previews are stopped, and the live route plus all twelve 1920x1080 assets pass HTTP and real-browser verification.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Current GUI process → captured PNGs | A reused or unhealthy frontend server could produce stale or misleading documentation. |
| Site-owned npm command → parent frontend checkout | The independently versioned site delegates capture to files outside its own repository and must resolve them safely. |
| Generated PNGs → public site submodule | Binary assets become public documentation and focused browser-test inputs. |
| Site submodule → parent gitlink | The parent must reference an existing pushed site commit without absorbing unrelated work. |
| Authenticated Netlify CLI → production | Deployment mutates the public production site and uses credentials that must remain secret. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-luj-01 | Spoofing | Playwright/Vite capture server | high | mitigate | Stop port 4790 listeners, force CI-mode fresh server startup, use strict-port configuration, and run two hash-identical captures. |
| T-luj-02 | Tampering | Screenshot inventory/output paths | high | mitigate | Derive names from the canonical catalog and require exact set equality, safe-path capture guards, valid PNGs, and 1920x1080 dimensions. |
| T-luj-07 | Tampering | `site/scripts/regenerate-desktop-views.mjs` parent delegation | high | mitigate | Resolve from `import.meta.url`, validate required parent files, use shell-free argument-array spawning with an absolute cwd, preserve CI-mode capture guards, and fail nonzero with a stable missing-parent diagnostic. |
| T-luj-03 | Repudiation | Visual acceptance | medium | mitigate | Inspect every image at original resolution and record per-file results plus test/deploy evidence in the summary. |
| T-luj-04 | Tampering | Git staging across repositories | high | mitigate | Use explicit pathspecs, push site first, compare the gitlink to the pushed site SHA, and keep the three named unrelated paths unstaged. |
| T-luj-05 | Information Disclosure | Netlify authentication | high | mitigate | Use existing CLI authentication without printing, writing, staging, or summarizing tokens. |
| T-luj-06 | Denial of Service | Production deployment | medium | mitigate | Require successful builds/tests and both pushes, stop only resolved stale preview processes, then use the pinned production command and verify every live asset. |
</threat_model>

<source_audit>
| SOURCE | ID | Feature/Requirement | Task | Status |
|--------|----|---------------------|------|--------|
| GOAL | — | Regenerate, visually verify, release, deploy, and verify complete Desktop Views screenshots | 1-3 | COVERED |
| REQ | DOCS-SCREENSHOTS-01 | Deterministic current-GUI capture and original-resolution review of all twelve 1920x1080 images | 1-2 | COVERED |
| REQ | DOCS-RELEASE-01 | Full gates, submodule-first pushes, explicit Netlify production deployment, live route/asset verification | 2-3 | COVERED |
| CONTEXT | — | Provide a maintainable site-owned npm regeneration command that safely delegates to the expected parent checkout | 1, 3 | COVERED |
| CONTEXT | — | Use Windows/local site checks and focused browser assertions without Linux pixel-baseline updates | 2 | COVERED |
| CONTEXT | — | Retain two hash-identical captures, 12/12 original-resolution passes, and frontend 274-test/build evidence | 1-2 | COVERED |
| CONTEXT | — | Preserve GUI as source and do not revert or independently modify it | 1-3 | COVERED |
| CONTEXT | — | Preserve the execution-start existence/status/fingerprints of both root resource paths and untracked site/deno.lock; never stage them | 1-3 | COVERED |
| CONTEXT | — | Prevent stale capture/preview processes from contaminating capture or deploy | 1, 3 | COVERED |
</source_audit>

<verification>
- The catalog, generated-site content, and public asset names resolve to the same twelve destinations.
- `npm run docs:screenshots` from `site/` delegates shell-free to the expected parent capture harness, reproduces the reviewed hashes, and fails clearly when that parent checkout is absent.
- The completed two-pass deterministic capture, all twelve original-resolution inspections, and frontend 274-test/build evidence remain tied to the delivered hashes.
- Site lint, typecheck, build, links, metadata, interaction, responsive-layout, and focused visual checks pass locally on Windows without modifying Linux baselines.
- Site and parent remote SHAs prove submodule-first push ordering; protected unrelated paths remain unstaged.
- The pinned deploy command succeeds, and production serves the current guide plus all twelve valid PNGs with correct dimensions and browser behavior.
</verification>

<success_criteria>
- Desktop Views documentation shows the complete current GUI without stale, clipped, overlapped, jumbled, broken, or unhealthy captures.
- A maintainer can regenerate it from the site repository with one npm command that safely locates the expected parent app even when the checkout path contains spaces.
- Completed deterministic capture, original-resolution review, and frontend test/build evidence is preserved, while remaining site checks run locally without Linux pixel-baseline churn.
- The release is reproducible and preserves unrelated work across both repositories.
- Production is explicitly deployed and verified rather than inferred from local build or CLI exit alone.
</success_criteria>

<output>
Create `.planning/quick/260729-luj-regenerate-the-complete-desktop-views-sc/260729-luj-SUMMARY.md` when done.
</output>
