---
description: Reconcile identityAliases that no longer describe the file/flow
allowed-tools: Bash(node:*)
---

!`node C:\Users\Lawrence\AppData\Local\nvm\v22.19.0\node_modules\@cstart\coldstart\dist\index.js kb repair-aliases --root .`

It prints a PAGINATED worklist (10 notes at a time) — each note you review must end with a `kb write` carrying `aliasesVerified:true` on it, even when you kept every alias unchanged, or it reappears next run. Once a batch is marked, re-run the exact same command again (no `--offset`) — reviewed notes drop out on their own, so it keeps surfacing whatever is still outstanding. Stop when it prints "No file/flow notes to reconcile."
