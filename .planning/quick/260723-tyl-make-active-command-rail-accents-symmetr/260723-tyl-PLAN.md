---
phase: quick-symmetric-command-rail-accents
plan: 260723-tyl
type: execute
wave: 1
depends_on: []
files_modified:
  - .planning/sketches/001-workspace-shell/index.html
  - .planning/sketches/002-programming-workspace/index.html
  - .planning/sketches/003-performance-workspace/index.html
  - .planning/sketches/004-patch-to-play-flow/index.html
  - .kimi-code/skills/sketch-findings-golc/sources/001-workspace-shell/index.html
  - .kimi-code/skills/sketch-findings-golc/sources/002-programming-workspace/index.html
  - .kimi-code/skills/sketch-findings-golc/sources/003-performance-workspace/index.html
  - .kimi-code/skills/sketch-findings-golc/sources/004-patch-to-play-flow/index.html
autonomous: true
requirements: [PLAY-01, PLAY-03, PLAY-07, PLAY-10, PLAY-11, PLAY-12]
must_haves:
  truths:
    - "Every active command-rail/deck-navigation item in sketches 001 through 004 has equal 2px Signal Blue inset accents on its left and right edges."
    - "The accent correction does not change rail geometry, borders, spacing, content, interactions, scripts, variant order, or the approved initial winners 001 D, 002 A, 003 A, and 004 B."
    - "Each packaged sketch source remains byte-for-byte equal to its canonical `.planning/sketches` counterpart."
  artifacts:
    - path: .planning/sketches/001-workspace-shell/index.html
      provides: "Symmetric active command-rail treatment for workspace-shell variants"
      contains: "inset -2px 0 var(--color-primary)"
    - path: .planning/sketches/002-programming-workspace/index.html
      provides: "Symmetric active command-rail treatment for programming variants"
      contains: "inset -2px 0 var(--color-primary)"
    - path: .planning/sketches/003-performance-workspace/index.html
      provides: "Symmetric active command-rail treatment for performance variants"
      contains: "inset -2px 0 var(--color-primary)"
    - path: .planning/sketches/004-patch-to-play-flow/index.html
      provides: "Symmetric active command-rail treatment for patch-to-play variants"
      contains: "inset -2px 0 var(--color-primary)"
    - path: .kimi-code/skills/sketch-findings-golc/sources
      provides: "Exact packaged mirrors of all four corrected canonical sketches"
  key_links:
    - from: .planning/sketches/001-workspace-shell/index.html
      to: .kimi-code/skills/sketch-findings-golc/sources/001-workspace-shell/index.html
      via: "Complete-byte mirroring after the canonical CSS edit"
      pattern: "inset\\s+-2px\\s+0\\s+var\\(--color-primary\\)"
    - from: .planning/sketches
      to: .kimi-code/skills/sketch-findings-golc/sources
      via: "SHA-256 equality for each of the four corresponding HTML pairs"
      pattern: "001-workspace-shell|002-programming-workspace|003-performance-workspace|004-patch-to-play-flow"
---

<objective>
Make the active command-rail/deck-navigation accent symmetric in all four canonical UI sketches and their packaged mirrors.

Purpose: Apply the user's confirmed direction that the right-side Signal Blue inset must match the existing left-side inset, without altering any layout or behavior.
Output: Four corrected canonical HTML sketches and four exact packaged byte mirrors.
</objective>

<execution_context>
@C:/Users/Lawrence/.codex/gsd-core/workflows/execute-plan.md
@C:/Users/Lawrence/.codex/gsd-core/templates/summary.md
</execution_context>

<context>
@AGENTS.md
@.planning/STATE.md
@.planning/sketches/WRAP-UP-SUMMARY.md
@.planning/sketches/001-workspace-shell/index.html
@.planning/sketches/002-programming-workspace/index.html
@.planning/sketches/003-performance-workspace/index.html
@.planning/sketches/004-patch-to-play-flow/index.html

**Dirty-worktree constraint:** This is a shared dirty worktree. Before editing, inspect `git status --short` and the diff for every owned file. Preserve unrelated work and stop if an owned file contains pre-existing changes that cannot be retained cleanly. Do not modify the shared theme, fonts, scripts, manifests, findings, application source, or any file outside `files_modified`.

**Discovery:** Level 0. The existing CSS contract and exact correction are already established locally. Each of the eight owned HTML files currently contains two active `.deck-nav button` shadow declarations, and every canonical/package pair currently has the same SHA-256 hash.
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add the matching right inset and mirror the corrected sketches</name>
  <files>.planning/sketches/001-workspace-shell/index.html, .planning/sketches/002-programming-workspace/index.html, .planning/sketches/003-performance-workspace/index.html, .planning/sketches/004-patch-to-play-flow/index.html, .kimi-code/skills/sketch-findings-golc/sources/001-workspace-shell/index.html, .kimi-code/skills/sketch-findings-golc/sources/002-programming-workspace/index.html, .kimi-code/skills/sketch-findings-golc/sources/003-performance-workspace/index.html, .kimi-code/skills/sketch-findings-golc/sources/004-patch-to-play-flow/index.html</files>
  <read_first>
    - `.planning/sketches/001-workspace-shell/index.html` - canonical spaced CSS form; retain 001 D and all existing shell behavior.
    - `.planning/sketches/002-programming-workspace/index.html` - canonical compact CSS form; retain 002 A and all authoring behavior.
    - `.planning/sketches/003-performance-workspace/index.html` - canonical compact CSS form; retain 003 A and all performance behavior.
    - `.planning/sketches/004-patch-to-play-flow/index.html` - canonical compact CSS form; retain 004 B and all guided-flow behavior.
    - `.planning/sketches/WRAP-UP-SUMMARY.md` - authority for the approved initial winners D/A/A/B.
  </read_first>
  <action>In each canonical HTML file, update both active deck-navigation `box-shadow` declarations by retaining the existing left 2px Signal Blue inset and appending a second inset with x-offset `-2px`, y-offset `0`, and the same `var(--color-primary)` color. Preserve each file's existing whitespace/minification style so the only canonical byte change is the appended right-side shadow component. Do not add physical borders or change width, padding, radius, background, color, selectors, specificity, layout, markup, visible copy, active/selected classes, variant order, event handlers, or script bytes. In particular, retain the approved initial winners 001 D, 002 A, 003 A, and 004 B.

After all four canonical edits are complete, copy each entire canonical HTML file to the same relative path under `.kimi-code/skills/sketch-findings-golc/sources`. Copy complete bytes rather than independently repeating the CSS edit, preserving encoding and line endings. Do not touch the packaged theme, fonts, skill index, references, or other packaged assets. Run the structural-delta check, occurrence check, pairwise SHA-256 check, and `git diff --check`; because inset shadows do not affect box geometry and the verification proves the edit is isolated, no screenshot artifact is required.</action>
  <verify>
    <automated>pwsh -NoProfile -Command "$rels=@('001-workspace-shell','002-programming-workspace','003-performance-workspace','004-patch-to-play-flow'); $sym='box-shadow\s*:\s*inset\s+2px\s+0\s+var\(--color-primary\)\s*,\s*inset\s+-2px\s+0\s+var\(--color-primary\)'; $right='inset\s+-2px\s+0\s+var\(--color-primary\)'; foreach($rel in $rels){$canonical=&quot;.planning/sketches/$rel/index.html&quot;; $mirror=&quot;.kimi-code/skills/sketch-findings-golc/sources/$rel/index.html&quot;; $now=Get-Content -LiteralPath $canonical -Raw; $symCount=[regex]::Matches($now,$sym).Count; $rightCount=[regex]::Matches($now,$right).Count; if($symCount -ne 2 -or $rightCount -ne 2){throw &quot;$canonical expected exactly two symmetric active-rail declarations; symmetric=$symCount right=$rightCount&quot;}; $head=(git show &quot;HEAD:$canonical&quot;) -join &quot;`n&quot;; if($LASTEXITCODE -ne 0){throw &quot;cannot load tracked baseline for $canonical&quot;}; $collapse={param($s) ([regex]::Replace(($s -replace &quot;`r`n&quot;,&quot;`n&quot;),'\s*,\s*inset\s+-2px\s+0\s+var\(--color-primary\)','')) .TrimEnd()}; if((&amp; $collapse $now) -cne (&amp; $collapse $head)){throw &quot;unexpected change beyond the appended right inset: $canonical&quot;}; $a=(Get-FileHash -LiteralPath $canonical -Algorithm SHA256).Hash; $b=(Get-FileHash -LiteralPath $mirror -Algorithm SHA256).Hash; if($a -cne $b){throw &quot;mirror mismatch: $canonical &lt;&gt; $mirror&quot;}}; $owned=$rels | ForEach-Object {&quot;.planning/sketches/$_/index.html&quot;;&quot;.kimi-code/skills/sketch-findings-golc/sources/$_/index.html&quot;}; git diff --check -- $owned; if($LASTEXITCODE -ne 0){throw 'git diff --check failed'}"</automated>
  </verify>
  <done>All eight HTML files contain exactly two symmetric active-rail shadow declarations, collapsing the newly appended right inset reproduces each tracked canonical baseline exactly, all four canonical/package SHA-256 pairs match, approved winners and scripts are untouched, and `git diff --check` passes.</done>
</task>

</tasks>

<source_audit>

| Source | ID | Required outcome | Task | Status |
|---|---|---|---:|---|
| GOAL | quick-task | Make active command-rail/deck-navigation accents symmetric in all four canonical sketches and packaged mirrors | 1 | COVERED |
| REQ | PLAY-01 | Preserve the complete on-screen authoring/playback sketch workflow | 1 | COVERED |
| REQ | PLAY-03 | Preserve the constrained operator-surface sketch and controls | 1 | COVERED |
| REQ | PLAY-07 | Preserve visible live state while correcting only the selected-navigation accent | 1 | COVERED |
| REQ | PLAY-10 | Preserve patch/deployment workflow content and interaction | 1 | COVERED |
| REQ | PLAY-11 | Preserve deployment/output readiness evidence | 1 | COVERED |
| REQ | PLAY-12 | Preserve scene/look programming content and interaction | 1 | COVERED |
| RESEARCH | local CSS | Reuse the existing active-state selector, 2px inset geometry, and primary token; add no dependency or new styling system | 1 | COVERED |
| CONTEXT | user-confirmed direction | The right-side accent matches the left-side accent | 1 | COVERED |
| CONTEXT | scope fence | Change no layout, other borders, interactions, winners, or scripts | 1 | COVERED |
| CONTEXT | mirror equality | Packaged sources remain exact byte mirrors of canonical sketches | 1 | COVERED |

</source_audit>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|---|---|
| Canonical sketches -> packaged skill sources | Duplicate design references can drift if independently edited. |
| CSS presentation -> preserved sketch behavior | The active-state rule shares HTML files with approved markup and scripts. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|---|---|---|---|---|---|
| T-Q-TYL-01 | Tampering | canonical sketch HTML | medium | mitigate | Collapse only the appended right inset and require byte equality with each tracked baseline, proving no other canonical content changed. |
| T-Q-TYL-02 | Tampering | packaged mirrors | medium | mitigate | Copy complete canonical bytes and require SHA-256 equality for all four corresponding pairs. |
| T-Q-TYL-03 | Denial of Service | active navigation layout | low | mitigate | Use inset shadows only; do not alter border, size, padding, layout, selector behavior, or interaction code. |
| T-Q-TYL-04 | Information Disclosure | static sketch assets | low | accept | The owned files contain repository-owned static UI examples with no credentials, personal data, persistence, or network calls. |
</threat_model>

<verification>
- Each of the eight owned HTML files has exactly two active navigation declarations containing both the left and matching right 2px Signal Blue insets.
- Removing only the appended right inset from each canonical file reproduces its tracked baseline byte content after line-ending normalization.
- SHA-256 hashes match for all four canonical/package HTML pairs.
- `git diff --check` passes across the eight owned files.
- Optional visual spot-check: open `.planning/sketches/001-workspace-shell/index.html` at desktop width and confirm the active command-rail item shows equal blue inset bars on both edges with no size or position shift.
</verification>

<success_criteria>
- Active command-rail/deck-navigation accents are visibly symmetric in sketches 001 through 004.
- Only the matching right inset component is added; layout, other borders, content, behavior, scripts, variant order, and D/A/A/B initial winners remain unchanged.
- Canonical and packaged versions of each sketch are exact byte mirrors.
- No theme, font, reference, application, or unrelated dirty-worktree file is modified.
</success_criteria>

<output>
Create `.planning/quick/260723-tyl-make-active-command-rail-accents-symmetr/260723-tyl-SUMMARY.md` when done.
</output>
