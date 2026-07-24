---
phase: quick-packaged-ui-sketch-light-theme
plan: 260723-rym
type: execute
wave: 1
depends_on: []
files_modified:
  - .planning/sketches/themes/default.css
  - .planning/sketches/001-workspace-shell/index.html
  - .planning/sketches/002-programming-workspace/index.html
  - .planning/sketches/003-performance-workspace/index.html
  - .planning/sketches/004-patch-to-play-flow/index.html
  - .kimi-code/skills/sketch-findings-golc/sources/themes/default.css
  - .kimi-code/skills/sketch-findings-golc/sources/001-workspace-shell/index.html
  - .kimi-code/skills/sketch-findings-golc/sources/002-programming-workspace/index.html
  - .kimi-code/skills/sketch-findings-golc/sources/003-performance-workspace/index.html
  - .kimi-code/skills/sketch-findings-golc/sources/004-patch-to-play-flow/index.html
autonomous: true
requirements: [PLAY-01, PLAY-03, PLAY-07, PLAY-10, PLAY-11, PLAY-12]
must_haves:
  truths:
    - "Opening any packaged sketch shows the GOLC Paper light theme by default, using the sibling website's authoritative page, panel, ink, text, line, Signal Blue, status, spectrum, and motion values."
    - "Dark hardcoded chrome, canvas, control, and selected-state colors no longer override the shared light-theme tokens in sketches 001 through 004."
    - "Each sketch retains its existing layouts, content, controls, interactions, variant order, and selected variant: 001 D, 002 A, 003 A, and 004 B."
    - "The canonical `.planning/sketches` assets and packaged `.kimi-code/skills/sketch-findings-golc/sources` assets are byte-for-byte equal for the shared theme and all four sketch HTML files."
  artifacts:
    - path: .planning/sketches/themes/default.css
      provides: "Shared Paper light-theme token contract derived from golc-site"
      contains: "--color-bg: #e4e0d8"
    - path: .planning/sketches/001-workspace-shell/index.html
      provides: "Light-themed workspace-shell variants with D selected"
    - path: .planning/sketches/002-programming-workspace/index.html
      provides: "Light-themed programming variants with A selected"
    - path: .planning/sketches/003-performance-workspace/index.html
      provides: "Light-themed performance variants with A selected"
    - path: .planning/sketches/004-patch-to-play-flow/index.html
      provides: "Light-themed patch-to-play variants with B selected"
    - path: .kimi-code/skills/sketch-findings-golc/sources/themes/default.css
      provides: "Packaged byte mirror of the canonical shared theme"
  key_links:
    - from: C:/Users/Lawrence/Documents/Dev/golc-site/src/app/globals.css
      to: .planning/sketches/themes/default.css
      via: "Authoritative light-theme values mapped to sketch semantic tokens"
      pattern: "#e4e0d8|#f4f1eb|#17181c|#1b44d9"
    - from: .planning/sketches/themes/default.css
      to: .planning/sketches/001-workspace-shell/index.html
      via: "Existing relative theme stylesheet link plus semantic CSS custom properties"
      pattern: "themes/default\\.css|var\\(--color-"
    - from: .planning/sketches
      to: .kimi-code/skills/sketch-findings-golc/sources
      via: "Byte-for-byte theme and HTML mirroring"
      pattern: "default\\.css|index\\.html"
---

<objective>
Restyle the packaged UI sketch assets to the GOLC website's authoritative Paper light theme without changing what the sketches demonstrate.

Purpose: Make the packaged design references visually consistent with the sibling `golc-site` repository while retaining the exact approved workspace and workflow variants.
Output: One canonical light-theme token file, four light-themed sketch HTML files, and exact packaged mirrors of those five assets.
</objective>

<execution_context>
@C:/Users/Lawrence/.codex/gsd-core/workflows/execute-plan.md
@C:/Users/Lawrence/.codex/gsd-core/templates/summary.md
</execution_context>

<context>
@AGENTS.md
@.planning/STATE.md
@.planning/sketches/MANIFEST.md
@.planning/sketches/WRAP-UP-SUMMARY.md
@C:/Users/Lawrence/Documents/Dev/golc-site/src/app/globals.css
@C:/Users/Lawrence/Documents/Dev/golc-site/src/components/ThemeToggle.tsx
@.planning/sketches/themes/default.css
@.planning/sketches/001-workspace-shell/index.html
@.planning/sketches/002-programming-workspace/index.html
@.planning/sketches/003-performance-workspace/index.html
@.planning/sketches/004-patch-to-play-flow/index.html

**Dirty-worktree constraint:** This is a shared dirty worktree. Before editing, inspect `git status --short` and the diff for every owned file. Preserve unrelated work and stop if an owned asset contains pre-existing edits that cannot be cleanly retained. Do not modify application source, sketch README/manifest/research documents, selected findings, or any file outside `files_modified`.

**Discovery:** Level 1 local verification. The authoritative sibling source is `C:/Users/Lawrence/Documents/Dev/golc-site/src/app/globals.css`. Its default is explicitly Paper light mode regardless of OS preference. The governing light values are page `#e4e0d8`, panel/on-accent `#f4f1eb`, ink/blackout `#17181c`, text `#4a4941`, text2 `#57564e`, muted/offline `#8a887f`, line `#d2ccc0`, Signal Blue `#1b44d9`, deep Signal Blue `#1233a8`, frame-lock `#5ac26a`, armed `#c8a24b`, revoked `#e23a2e`, and spectrum `#c0554a`, `#cc8a47`, `#b6a24c`, `#4e9e68`, `#1b44d9`, `#6a50a8`. The website uses Archivo, JetBrains Mono, 120ms tap motion, 200ms settle motion, and a light hover shadow `0 8px 24px rgba(0, 0, 0, 0.08)`.
</context>

<tasks>

<task type="auto">
  <name>Task 1: Apply the authoritative Paper theme to the canonical sketches</name>
  <files>.planning/sketches/themes/default.css, .planning/sketches/001-workspace-shell/index.html, .planning/sketches/002-programming-workspace/index.html, .planning/sketches/003-performance-workspace/index.html, .planning/sketches/004-patch-to-play-flow/index.html</files>
  <read_first>
    - `C:/Users/Lawrence/Documents/Dev/golc-site/src/app/globals.css` - authoritative default light tokens, status/spectrum palette, color-scheme behavior, fonts, motion, and light hover shadow.
    - `.planning/sketches/themes/default.css` - existing sketch token names, typography, spacing, radii, and transition contract to retain.
    - `.planning/sketches/WRAP-UP-SUMMARY.md` - approved selections are 001 D, 002 A, 003 A, and 004 B.
    - `.planning/sketches/001-workspace-shell/index.html` - preserve all four shell variants, D selection, annotations, tool controls, and JavaScript behavior.
    - `.planning/sketches/002-programming-workspace/index.html` - preserve all three programming variants, A selection, editing controls, and JavaScript behavior.
    - `.planning/sketches/003-performance-workspace/index.html` - preserve all three performance variants, A selection, playback controls, and JavaScript behavior.
    - `.planning/sketches/004-patch-to-play-flow/index.html` - preserve all three flow variants, B selection, readiness controls, and JavaScript behavior.
  </read_first>
  <action>First record the four HTML files' script blocks, `data-variant` order, active/selected class markers, and non-color inline styles as structural baselines. Update `default.css` by retaining its public sketch token names and mapping them to the sibling site's light authority: background to page; surfaces to panel; sunken regions to page; borders to line, with muted available for stronger separation; primary and hover to Signal Blue and deep Signal Blue; foregrounds to ink/text/text2/muted; on-primary to on-accent; blackout to ink; and all status/spectrum tokens to their same-named website values. Add semantic tokens for on-primary and the light-mode mark tile/bar where needed. Keep Archivo/JetBrains Mono stacks, spacing, radii, and 120ms/200ms timing. Set light color-scheme behavior explicitly and use the website's light shadow value. Derive translucent selection, warning, danger, and glow treatments only from the authoritative accent/status/spectrum RGB values; do not introduce a second palette.

In each HTML stylesheet and color-bearing inline style, replace dark chrome and arbitrary light-on-dark literals with the shared semantic tokens. Apply page to the outer canvas, panel to raised workspace chrome/cards/controls, page or a token-derived inset treatment to wells/tracks, line to dividers, ink/text/muted to hierarchy, on-primary to selected Signal Blue controls, and authoritative status colors to live/frame-lock/armed/revoked/blackout/offline states. Make the GOLC mark follow the website's light-mode ink tile and panel foreground. Keep status color semantic: red remains reserved for revoke/error, armed remains amber, frame lock remains green, and Signal Blue remains selection/live.

Change presentation colors only. Do not change element order, dimensions, grids, spacing, typography sizes, labels, data attributes, classes used by scripts, initial `active`/`selected` markers, visible variant selection, event handlers, tool controls, annotations, or script blocks. Retain 001 D, 002 A, 003 A, and 004 B exactly as documented.</action>
  <verify>
    <automated>pwsh -NoProfile -Command "$allowed=@('#e4e0d8','#f4f1eb','#17181c','#4a4941','#57564e','#8a887f','#d2ccc0','#1b44d9','#1233a8','#5ac26a','#c8a24b','#e23a2e','#c0554a','#cc8a47','#b6a24c','#4e9e68','#6a50a8'); $files=@('.planning/sketches/themes/default.css','.planning/sketches/001-workspace-shell/index.html','.planning/sketches/002-programming-workspace/index.html','.planning/sketches/003-performance-workspace/index.html','.planning/sketches/004-patch-to-play-flow/index.html'); foreach($f in $files){$raw=Get-Content -LiteralPath $f -Raw; $hex=[regex]::Matches($raw,'(?i)#[0-9a-f]{3,8}') | ForEach-Object {$_.Value.ToLowerInvariant()} | Sort-Object -Unique; $unexpected=@($hex | Where-Object {$_ -notin $allowed}); if($unexpected){throw ""$f has non-authoritative hex colors: $($unexpected -join ', ')""}}; $css=Get-Content -LiteralPath $files[0] -Raw; foreach($token in @('--color-bg: #e4e0d8','--color-surface: #f4f1eb','--color-text: #17181c','--color-primary: #1b44d9','--color-primary-hover: #1233a8','--color-border: #d2ccc0')){if(-not $css.Contains($token)){throw ""missing light token $token""}}; foreach($f in $files[1..4]){$path=$f.Replace('\','/'); $base=git show ""HEAD:$path""; if($LASTEXITCODE -ne 0){throw ""cannot load baseline $path""}; $now=Get-Content -LiteralPath $f -Raw; $script='(?s)<script>(.*?)</script>'; if(([regex]::Match($now,$script).Groups[1].Value) -cne ([regex]::Match(($base -join ""`n""),$script).Groups[1].Value)){throw ""script behavior changed: $f""}; $strip={param($s) $s=[regex]::Replace($s,'(?s)<style>.*?</style>','<style/>'); $s=[regex]::Replace($s,'(?i)(background(?:-color)?|color|border-color)\s*:\s*[^;""'']+;?',''); return (($s -replace ""`r`n"",""`n"") -replace ""`r"",""`n"").Trim()}; if((&amp; $strip $now) -cne (&amp; $strip ($base -join ""`n""))){throw ""non-color structure or selection changed: $f""}}; git diff --check -- $files"</automated>
  </verify>
  <done>The canonical theme contains the website's exact Paper light palette, all four sketches use it without dark hardcoded overrides, and automated baseline checks show that scripts, markup structure, interactions, and selected variants are unchanged.</done>
</task>

<task type="auto">
  <name>Task 2: Rebuild the packaged sources as exact mirrors</name>
  <files>.kimi-code/skills/sketch-findings-golc/sources/themes/default.css, .kimi-code/skills/sketch-findings-golc/sources/001-workspace-shell/index.html, .kimi-code/skills/sketch-findings-golc/sources/002-programming-workspace/index.html, .kimi-code/skills/sketch-findings-golc/sources/003-performance-workspace/index.html, .kimi-code/skills/sketch-findings-golc/sources/004-patch-to-play-flow/index.html</files>
  <read_first>
    - `.planning/sketches/themes/default.css` - completed canonical theme source.
    - `.planning/sketches/001-workspace-shell/index.html` - completed canonical 001 source.
    - `.planning/sketches/002-programming-workspace/index.html` - completed canonical 002 source.
    - `.planning/sketches/003-performance-workspace/index.html` - completed canonical 003 source.
    - `.planning/sketches/004-patch-to-play-flow/index.html` - completed canonical 004 source.
    - `.kimi-code/skills/sketch-findings-golc/sources/` - packaged mirror root; preserve directory names and do not touch other skill content.
  </read_first>
  <action>Synchronize each completed canonical asset to its same relative path under `.kimi-code/skills/sketch-findings-golc/sources`: the shared theme plus sketches 001 through 004. Copy the complete bytes rather than independently reapplying color edits so the package cannot drift from the canonical design references. Preserve UTF-8 content and line endings consistently across each pair. Do not modify the packaged skill index, findings documents, README files, or any additional source.</action>
  <verify>
    <automated>pwsh -NoProfile -Command "$pairs=@(@('.planning/sketches/themes/default.css','.kimi-code/skills/sketch-findings-golc/sources/themes/default.css'),@('.planning/sketches/001-workspace-shell/index.html','.kimi-code/skills/sketch-findings-golc/sources/001-workspace-shell/index.html'),@('.planning/sketches/002-programming-workspace/index.html','.kimi-code/skills/sketch-findings-golc/sources/002-programming-workspace/index.html'),@('.planning/sketches/003-performance-workspace/index.html','.kimi-code/skills/sketch-findings-golc/sources/003-performance-workspace/index.html'),@('.planning/sketches/004-patch-to-play-flow/index.html','.kimi-code/skills/sketch-findings-golc/sources/004-patch-to-play-flow/index.html')); foreach($pair in $pairs){$a=(Get-FileHash -LiteralPath $pair[0] -Algorithm SHA256).Hash; $b=(Get-FileHash -LiteralPath $pair[1] -Algorithm SHA256).Hash; if($a -cne $b){throw ""mirror mismatch: $($pair[0]) <> $($pair[1])""}}; git diff --check -- ($pairs | ForEach-Object {$_[0];$_[1]})"</automated>
  </verify>
  <done>Every packaged theme/sketch source has the same SHA-256 hash as its canonical `.planning/sketches` counterpart, with no unrelated packaged skill file changed.</done>
</task>

</tasks>

<source_audit>

| Source | ID | Required outcome | Task | Status |
|---|---|---|---:|---|
| GOAL | quick-task | Match the packaged UI sketches to the sibling GOLC website's Paper light theme | 1-2 | COVERED |
| REQ | PLAY-01 | Preserve the complete on-screen playback sketch workflow while restyling | 1 | COVERED |
| REQ | PLAY-03 | Preserve constrained operator-surface content and controls while restyling | 1 | COVERED |
| REQ | PLAY-07 | Preserve visible live state and semantic status distinctions while restyling | 1 | COVERED |
| REQ | PLAY-10 | Preserve patch/deployment workflow content and interactions while restyling | 1 | COVERED |
| REQ | PLAY-11 | Preserve deployment and Art-Net configuration workflow content while restyling | 1 | COVERED |
| REQ | PLAY-12 | Preserve scene/look programming workflow content and interactions while restyling | 1 | COVERED |
| RESEARCH | golc-site globals | Use the exact authoritative light palette, fonts, motion values, color-scheme default, and light shadow | 1 | COVERED |
| CONTEXT | selected variants | Preserve 001 D, 002 A, 003 A, and 004 B | 1 | COVERED |
| CONTEXT | scope fence | Change theme colors only; preserve layouts/interactions and modify no assets outside the ten named canonical/mirror files | 1-2 | COVERED |
| CONTEXT | mirror equality | Keep canonical and packaged sources byte-for-byte equal | 2 | COVERED |

</source_audit>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|---|---|
| Sibling website repository -> canonical sketch tokens | A separate repository is the visual authority copied into GOLC planning assets. |
| Canonical sketches -> packaged skill sources | Design references are duplicated for packaging and can silently diverge. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|---|---|---|---|---|---|
| T-Q-SKETCH-01 | Tampering | light-theme palette | medium | mitigate | Allow only the explicit authoritative website hex palette in the five canonical assets and fail verification on any additional hex literal. |
| T-Q-SKETCH-02 | Tampering | sketch behavior and selections | medium | mitigate | Compare script blocks and normalized non-color HTML against tracked baselines so presentation work cannot alter interactions, structure, or active variants. |
| T-Q-SKETCH-03 | Tampering | packaged mirrors | medium | mitigate | Require SHA-256 equality for the theme and each of the four canonical/package pairs. |
| T-Q-SKETCH-04 | Information Disclosure | static sketch assets | low | accept | The assets contain only repository-owned static UI examples and no credentials, user data, runtime state, or network calls. |
</threat_model>

<verification>
- The five canonical assets use only the authoritative sibling website hex palette; translucent treatments derive from those same accent/status/spectrum values.
- The theme token assertions prove Paper page/panel/ink/line and Signal Blue/deep Signal Blue are installed under the existing sketch contract.
- Each HTML script block and normalized non-color structure equals its tracked baseline, preserving layout, behavior, content, and selected variants.
- SHA-256 equality passes for all five canonical/package pairs.
- `git diff --check` passes for all ten owned files.
- <human-check>Open each canonical HTML file in a browser at desktop width, confirm the Paper light theme appears immediately, switch through every variant, exercise buttons/tool controls, and confirm 001 D, 002 A, 003 A, and 004 B remain the initially selected findings with readable text and unmistakable blue/live, green/frame-lock, amber/armed, and red/revoke states.</human-check>
</verification>

<success_criteria>
- All four sketches render in the website's Paper light visual language with no dark chrome overriding shared tokens.
- Layouts, content, interaction behavior, and approved variant selections are unchanged.
- Semantic state colors remain legible and retain their documented meanings.
- Canonical and packaged assets are exact byte mirrors across the theme and sketches 001 through 004.
- No application source or unrelated planning/package asset changes.
</success_criteria>

<output>
Create `.planning/quick/260723-rym-update-packaged-ui-sketch-assets-to-matc/260723-rym-SUMMARY.md` when done.
</output>
