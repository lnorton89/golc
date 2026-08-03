---
name: golc-site-submodule-workflow
description: How to safely make and land changes in the ./site marketing-site submodule (separate repo, Netlify-deployed). Load before editing anything under site/ or before committing in the main repo when `git status` shows site as modified.
---

<context>
## What `site` is

`./site` is a git submodule (`.gitmodules`) pointing at
`https://github.com/lnorton89/golc-site.git`, a separate repository with its
own history, remote, and Netlify deployment. It is *not* part of the
`golc` module and has no relationship to the Go toolchain, Mage targets, or
`internal/`/`cmd/` pinned-toolchain test gates. `mage <target>` and
`golc-project` commands never touch it.

The main `golc` repo only tracks a single **commit pointer** into
`golc-site`'s history — that's what a ` M site` line in the main repo's
`git status` means: the submodule's checked-out commit no longer matches
the pointer the main repo has recorded (either the submodule has its own
local changes, or it's checked out to a different commit than main
currently points to).
</context>

<workflow>
## Making a change

1. Work inside `site/` as its own repo: `git -C site status`, `git -C site
   diff`, `git -C site log` all operate on `golc-site`'s history, not
   `golc`'s.
2. Commit and push *inside the submodule first*:
   ```bash
   git -C site add <files>
   git -C site commit -m "..."
   git -C site push origin master
   ```
   Netlify deploys off `golc-site`'s own `master` branch (per
   `git -C site branch -vv`) — pushing here is what actually ships the
   site. Nothing in the main `golc` repo triggers or affects that deploy.
3. Only *after* the submodule push succeeds, bump the pointer in the main
   repo:
   ```bash
   git add site
   git commit -m "chore(site): bump submodule to <short-sha> (<what changed>)"
   ```
   This records "the main repo now expects this exact `golc-site` commit" —
   it does not copy any file content, just the pointer.

## Before committing in the main repo

If `git status` in the main repo shows ` M site` and you did *not*
intend to change the site, check `git -C site status`/`log` first — don't
`git add site` reflexively. A stray pointer bump can silently pin the main
repo to an unintended (possibly unpushed, or locally-dirty) submodule
commit. Only stage `site` once you've confirmed the submodule's checked-out
commit is pushed and is the one you actually want recorded.

## Local dev preview

`.claude/launch.json`'s `golc-site-dev` config (`npm --prefix site run
dev`, port 3000) runs the site's own dev server for local preview — separate
from any `golc-desktop` frontend preview port.
</workflow>
