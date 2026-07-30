---
phase: quick-desktop-views-stage-separation
plan: 260729-nfn
type: execute
wave: 1
depends_on: []
files_modified:
  - site/src/components/docs/DesktopViewExplorer.tsx
  - site/tests/visual.spec.ts
  - site
  - .planning/quick/260729-nfn-make-the-desktop-views-screenshot-stage-/260729-nfn-PLAN.md
  - .planning/quick/260729-nfn-make-the-desktop-views-screenshot-stage-/260729-nfn-SUMMARY.md
  - .planning/STATE.md
autonomous: true
requirements: [DOCS-VISUAL-01, DOCS-RELEASE-01]
must_haves:
  truths:
    - "The large Desktop Views screenshot/enlarge stage is visually distinct from the selected view detail beginning with its ID eyebrow and heading in both light and dark themes."
    - "The separation uses one restrained surface, spacing, border, and shadow hierarchy inside the existing master-detail panel rather than introducing another card grid or changing the documentation content."
    - "At desktop and 390px mobile widths, the screenshot, enlarge control, and detail remain ordered, readable, and free of horizontal overflow."
    - "The screenshot lightbox still opens and closes through its existing mouse and keyboard paths and restores focus and scroll behavior."
    - "The exact pushed site revision is recorded by the parent gitlink, deployed once through the pinned npm script, and verified on the canonical production route."
  artifacts:
    - path: site/src/components/docs/DesktopViewExplorer.tsx
      provides: "Theme-token-based screenshot-stage boundary within the existing selected-view panel."
    - path: site/tests/visual.spec.ts
      provides: "Focused structural/style assertions for the boundary at desktop/mobile widths and in both themes."
  key_links:
    - from: site/src/components/docs/DesktopViewExplorer.tsx
      to: "screenshot stage and following detail"
      via: "Stable test IDs and adjacent regions expose the intended surface, spacing, and border boundary without altering tabpanel semantics."
      pattern: "desktop-view-(screenshot-stage|detail)"
    - from: site/tests/visual.spec.ts
      to: site/src/components/docs/DesktopViewExplorer.tsx
      via: "Playwright measures region order, computed theme styles, inset spacing, and page overflow for both themes and viewport classes."
      pattern: "desktop views screenshot stage"
    - from: site/package.json
      to: site/netlify.toml
      via: "The existing pinned deploy script performs the Netlify production build and publish."
      pattern: "netlify deploy --build --prod"
---

<objective>
Make the Desktop Views screenshot stage and the following selected-view detail unmistakably separate in light and dark themes while preserving the single master-detail design, lightbox, and responsive behavior.

Purpose: The current edge-to-edge screenshot/enlarge region and the `SHOW-OVERVIEW` detail surface visually run together, weakening the selected panel's hierarchy.
Output: A restrained theme-coherent stage boundary, focused responsive/theme regression coverage, direct four-mode browser evidence, site-first and parent-gitlink commits, and one verified Netlify production deployment.
</objective>

<execution_context>
@C:/Users/Lawrence/.codex/gsd-core/workflows/execute-plan.md
@C:/Users/Lawrence/.codex/gsd-core/templates/summary.md
</execution_context>

<context>
@AGENTS.md
@.planning/STATE.md
@.planning/quick/260729-hsx-redesign-the-desktop-views-documentation/260729-hsx-SUMMARY.md
@.planning/quick/260729-luj-regenerate-the-complete-desktop-views-sc/260729-luj-SUMMARY.md
@site/AGENTS.md
@site/src/components/docs/DesktopViewExplorer.tsx
@site/src/app/globals.css
@site/tests/visual.spec.ts
@site/playwright.config.ts
@site/package.json
@site/netlify.toml

This is a focused presentation correction inside the existing single tabpanel. Keep the grouped selector, one selected detail, all catalog-driven copy, all twelve views, and the screenshot lightbox behavior unchanged. Do not edit `site/src/content/desktop-views.json`, `site/public/desktop-views/**`, the GUI/frontend source, navigation, or route framing. Add no package and do not modify `site/package.json`, `site/package-lock.json`, `site/netlify.toml`, or `.netlify/state.json`.

Before editing the Next.js component, read the relevant installed styling/component guidance under `site/node_modules/next/dist/docs/` as required by `site/AGENTS.md`. The implementation should primarily use existing Tailwind utilities and the established `bg-page`, `bg-panel`, `border-line`, and text tokens. Prefer an inset screenshot stage with proportionate responsive padding, a visible lower boundary, and a restrained image frame/shadow. Do not wrap the detail in another nested card, add ornamental dividers, or reintroduce repeated cards.

Both repositories contain unrelated work. `site/deno.lock` is a pre-existing untracked file and must remain byte-identical, untracked, and unstaged. Preserve all unrelated parent changes, including `.gitattributes`, `internal/command/test.go`, and `internal/wails/svc_safety.go`. Recheck both worktrees before every stage/commit and use explicit pathspecs.

All test, build, preview, and browser work must run directly in the existing Windows/local environment. Do not use Docker, WSL, a Linux container, or any Linux snapshot-baseline workflow, and do not create or update `*-linux.png` baselines.

The site is an independently versioned submodule. Commit and push task-owned site code/tests first. Only then record that exact pushed SHA in the parent gitlink with the quick-task plan and push that parent release commit. Deploy only after those site and parent pushes, execute `npm run deploy` exactly once from `site/`, and do not place the production mutation in a retryable verification command, create a draft deploy, relink Netlify, or deploy through a dashboard. After production verification, finalize and push the summary/STATE evidence in a second parent-only documentation commit. If browser inspection needs a preview process, record the PID at launch and stop only that PID in cleanup; never terminate Node/serve processes by name or broad port sweep.
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Define and implement the screenshot-stage boundary</name>
  <files>site/src/components/docs/DesktopViewExplorer.tsx, site/tests/visual.spec.ts</files>
  <behavior>
    - Test 1: for light and dark themes at 1280x900 and 390x844, the screenshot stage precedes the detail region, has deliberate inset space around the screenshot content, and exposes a nonzero lower border plus a surface distinguishable from the detail.
    - Test 2: the same four theme/viewport combinations have no document or main-content horizontal overflow and keep the detail below the screenshot stage.
    - Test 3: the existing selected-tab, lightbox naming, Escape/backdrop/close handling, focus restoration, and scroll restoration tests continue to pass unchanged.
  </behavior>
  <action>
Add stable `data-testid="desktop-view-screenshot-stage"` and `data-testid="desktop-view-detail"` hooks to the existing screenshot button region and following detail container, then write a focused Playwright test that loops over light/dark and desktop/mobile cases. Use bounding boxes and computed styles to assert source order, responsive inset spacing, a visible lower boundary, distinct stage/detail surfaces, and no horizontal overflow; assert observable properties rather than Tailwind class strings.

After the test fails for the current blended presentation, style the existing screenshot button as one deliberate stage: retain its full-width enlarge hit target, use the page surface against the panel detail, add proportionate mobile/desktop inset spacing, frame the image with the existing line token, and apply only a restrained shadow and lower boundary sufficient to separate the regions in both themes. Keep the `Enlarge screenshot` label inside this stage and ensure the detail still begins with the current ID eyebrow and heading. Do not change component state, tab semantics, image source/alt text, lightbox markup/handlers, selected content, or master-detail grid.
  </action>
  <verify>
    <automated>Push-Location site; try { npx playwright test tests/visual.spec.ts --grep "desktop views (screenshot stage|remain readable|lightbox|expose one grouped selector)" } finally { Pop-Location }</automated>
  </verify>
  <done>The stage/detail boundary is structurally testable and visually deliberate in all four theme/viewport cases, while the selector, selected content, lightbox interactions, and mobile flow remain unchanged.</done>
</task>

<task type="auto">
  <name>Task 2: Review both themes and responsive widths without baseline churn</name>
  <files>site/src/components/docs/DesktopViewExplorer.tsx, site/tests/visual.spec.ts</files>
  <action>
Run only the normal Windows/local lint, typecheck, static build, and focused semantic Playwright coverage. Run directly on Windows: do not invoke Docker, WSL, Linux containers, or Linux CI as a substitute for the requested local evidence. Do not create or update any pixel baseline. If an existing local snapshot assertion unexpectedly runs or fails, stop and report it rather than changing a baseline; rely on the new computed-style, geometry, overflow, and interaction assertions for this task.

Launch one task-owned local static preview if needed and retain its PID. With browser-based inspection at 1280x900 and 390x844, examine light and dark modes, scroll through the screenshot/detail boundary, switch at least Overview and Scripts, and exercise enlarge plus Escape close. Confirm the boundary remains unambiguous, selector/detail ordering stays responsive, and no horizontal overflow appears. Save disposable inspection captures outside the repository if useful; do not add product screenshots or pixel baselines. Stop only the recorded preview PID when inspection finishes.

Before and after all npm/test commands, hash `site/deno.lock` without staging it and confirm it remains byte-identical. Do not invoke snapshot-update mode, loosen screenshot tolerance, edit screenshots, or create platform-specific baseline files.
  </action>
  <verify>
    <automated>npm --prefix site run lint; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; npm --prefix site run typecheck; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; npm --prefix site run build; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; Push-Location site; try { npx playwright test tests/visual.spec.ts --grep "desktop views" } finally { Pop-Location }</automated>
    <human-check>Inspect desktop and 390px mobile in light and dark using the browser. The stage/detail break is clear but restrained, and both screenshot enlargement and responsive stacking still behave normally.</human-check>
  </verify>
  <done>All normal Windows/local code and browser gates pass, the four requested visual cases are inspected directly, no pixel baseline changes, and `site/deno.lock` plus every screenshot/GUI/unrelated path is preserved.</done>
</task>

<task type="auto">
  <name>Task 3: Push in submodule order, deploy once, and verify production</name>
  <files>site, .planning/quick/260729-nfn-make-the-desktop-views-screenshot-stage-/260729-nfn-PLAN.md, .planning/quick/260729-nfn-make-the-desktop-views-screenshot-stage-/260729-nfn-SUMMARY.md, .planning/STATE.md</files>
  <action>
Inspect site and parent diffs immediately before staging. In `site/`, stage only `src/components/docs/DesktopViewExplorer.tsx` and `tests/visual.spec.ts`; explicitly confirm `deno.lock`, pixel baselines, and screenshots are not staged. Commit, push the site `master` commit to `origin`, capture its full SHA, and confirm the remote branch contains that exact revision. If the remote advanced, fetch and integrate only after confirming the incoming changes do not overlap task-owned files; never discard or overwrite unrelated work.

Return to the parent repository, verify the `site` gitlink equals the pushed site SHA, and commit only the gitlink plus this quick-task plan with explicit pathspecs. Push that parent release commit and confirm its remote contains the exact gitlink. Preserve all pre-existing parent modifications unstaged. This parent push, and the site push before it, are mandatory preconditions for deployment.

Immediately before deployment, require the site worktree's tracked files to be clean, require `HEAD` to equal the confirmed `origin/master` SHA, and require the only permitted untracked entry to be the unchanged `deno.lock`; also confirm the parent `HEAD` is pushed and its `site` gitlink equals that same site SHA. If any precondition fails, stop before publishing. Then run `npm run deploy` exactly once from `site/` as the sole production mutation in this plan. Let authentication or build failure surface; do not retry the production mutation without reporting the failed attempt. Capture the deploy ID, production URL, and logs.

Verify `https://golc-site.netlify.app/docs/desktop-views` returns successfully and use a real browser against that canonical URL at desktop/mobile and light/dark to confirm the new stage/detail boundary, Overview/Scripts tab switching, no horizontal overflow, and lightbox open/Escape/focus restoration. After that evidence exists, create the quick-task summary, update STATE through the GSD quick completion workflow, and record the exact site SHA, pre-deploy parent release SHA, deploy ID, and production browser evidence without secrets. Commit only SUMMARY.md and STATE.md as a second parent-only evidence commit, push it, and confirm `origin/master` contains both the release gitlink commit and the final evidence. If the parent remote advances at either push, apply the same inspect-before-integrate rule and never overwrite unrelated work.
  </action>
  <verify>
    <automated>$siteHead = git -C site rev-parse HEAD; $siteRemote = git -C site rev-parse origin/master; if ($siteHead -ne $siteRemote) { throw "site HEAD is not pushed" }; $gitlink = (git ls-tree HEAD site).Split()[2]; if ($gitlink -ne $siteHead) { throw "parent gitlink does not match pushed site SHA" }; $parentHead = git rev-parse HEAD; $parentRemote = git rev-parse origin/master; if ($parentHead -ne $parentRemote) { throw "final parent evidence commit is not pushed" }; $status = (Invoke-WebRequest -UseBasicParsing https://golc-site.netlify.app/docs/desktop-views).StatusCode; if ($status -ne 200) { throw "production route returned $status" }; $status</automated>
  </verify>
  <done>The site revision is committed and pushed before the parent gitlink release commit, both are pushed before the sole production deploy invocation, the final summary/STATE evidence commit is pushed afterward, and the canonical live route passes four-mode visual/responsive checks plus preserved lightbox behavior.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|---|---|
| Browser assertions → visual acceptance | Computed layout/style and screenshots must prove the intended boundary rather than merely detect class names. |
| Site submodule → parent repository | The parent gitlink must reference the exact pushed site revision without absorbing unrelated parent work. |
| Authenticated local CLI → Netlify production | One command mutates the public site linked by the existing Netlify state. |
| Netlify result → canonical public route | CLI success is not proof that the intended revision and responsive behavior are live. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|---|---|---|---|---|---|
| T-nfn-01 | Tampering | Visual evidence | medium | mitigate | Prefer computed-style/layout assertions and direct browser captures; prohibit snapshot-update mode and keep all pixel baselines unchanged unless an existing local assertion proves a change is unavoidable and the executor reports it first. |
| T-nfn-02 | Tampering | User work and `site/deno.lock` | high | mitigate | Hash/preserve the lock, use explicit staging pathspecs, and inspect both worktrees before each commit. |
| T-nfn-03 | Denial of Service | Local preview cleanup | medium | mitigate | Record the task-owned preview PID and stop only that process; prohibit name-wide or broad port termination. |
| T-nfn-04 | Spoofing | Netlify target/revision | high | mitigate | Preserve the existing link, deploy only after both pushes, record exact SHAs/deploy ID, and verify the canonical production URL. |
| T-nfn-05 | Information Disclosure | Git/Netlify authentication | high | mitigate | Use existing authenticated contexts and never print, stage, or summarize tokens or credentials. |
| T-nfn-06 | Repudiation | Production release | medium | mitigate | Record site SHA, parent SHA/gitlink, deploy ID, canonical status, and browser results in the summary. |
</threat_model>

<verification>
- Focused Playwright assertions prove stage/detail order, inset spacing, boundary/surface distinction, and no overflow at 1280x900 and 390x844 in light and dark.
- Existing selector and lightbox semantic tests pass without weakened assertions.
- Normal Windows/local lint, typecheck, static build, focused semantic Playwright tests, and four-mode direct browser inspection pass.
- Original-resolution and live-browser reviews find a restrained but unambiguous separation in all four visual cases.
- Git diffs contain only the component, focused test, parent gitlink, and quick-task artifacts; pixel baselines, screenshots, GUI source, `site/deno.lock`, and unrelated root changes remain untouched.
- Site and parent release pushes precede the single successful, non-retryable `npm run deploy`; a final parent-only evidence commit records the deploy afterward, and the live canonical route serves and behaves as the deployed revision.
</verification>

<success_criteria>
- The screenshot/enlarge stage no longer blends into the `SHOW-OVERVIEW` / `Show overview` detail in either theme.
- The page remains one master-detail experience, not a collection of nested or repeated cards.
- Desktop/mobile layout, tab selection, lightbox behavior, focus restoration, and overflow constraints remain green.
- Only task-owned site and parent artifacts are committed, pushed, deployed, and traceably verified.
</success_criteria>

<source_coverage_audit>

| Source | Item | Status | Plan coverage |
|---|---|---|---|
| GOAL | Clearly separate the Desktop Views screenshot stage from following detail in light/dark, verify responsive behavior, push, and deploy | COVERED | Tasks 1-3 implement, verify, release, and inspect production. |
| REQ | DOCS-VISUAL-01 theme-coherent responsive stage/detail separation | COVERED | Tasks 1-2 add the boundary, computed regression assertions, normal local gates, and four-mode direct browser review without baseline churn. |
| REQ | DOCS-RELEASE-01 site-first push, parent gitlink, one pinned deployment, production verification | COVERED | Task 3 enforces and records the complete release order. |
| RESEARCH | Existing theme tokens and master-detail/lightbox patterns | COVERED | Task 1 reuses current tokens and markup, with no new dependency or architecture. |
| CONTEXT | Preserve single master-detail design and avoid card-heavy treatment | COVERED | Task 1 constrains the change to one inset stage inside the current tabpanel. |
| CONTEXT | Keep lightbox and responsive behavior intact | COVERED | Tasks 1-2 retain handlers/semantics and exercise existing plus new viewport/theme assertions. |
| CONTEXT | Browser-verify desktop/mobile and light/dark | COVERED | Tasks 2 and 3 require local and production browser inspection in all four cases. |
| CONTEXT | Preserve `site/deno.lock`, screenshots, GUI source, and unrelated changes | COVERED | All tasks restrict file ownership, hashes, staging, and cleanup. |
| CONTEXT | Use Windows/local non-snapshot evidence; do not use Linux containers or update Linux baselines | COVERED | Task 2 explicitly prohibits Docker/WSL/Linux substitution and all Linux baseline creation or updates. |
| CONTEXT | Commit/push site first, then parent gitlink; deploy once with `npm run deploy` | COVERED | Task 3 specifies site push, parent release/gitlink push, one non-retryable deploy, production verification, then a parent-only evidence push. |
</source_coverage_audit>

<output>
Create `.planning/quick/260729-nfn-make-the-desktop-views-screenshot-stage-/260729-nfn-SUMMARY.md` when done.
</output>
